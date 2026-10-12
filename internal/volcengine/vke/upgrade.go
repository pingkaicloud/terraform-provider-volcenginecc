package vke

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	tfcloudcontrol "github.com/volcengine/terraform-provider-volcenginecc/internal/service/cloudcontrol"
	"github.com/volcengine/volcengine-go-sdk/service/sts"
	sdkvke "github.com/volcengine/volcengine-go-sdk/service/vke"
	"github.com/volcengine/volcengine-go-sdk/volcengine/request"
	"github.com/volcengine/volcengine-go-sdk/volcengine/session"
)

var minorVersionPattern = regexp.MustCompile(`^v?([0-9]+\.[0-9]+)(?:\.|$)`)

func minorVersion(version string) (string, error) {
	matches := minorVersionPattern.FindStringSubmatch(version)
	if len(matches) != 2 {
		return "", fmt.Errorf("invalid VKE Kubernetes version %q", version)
	}
	return matches[1], nil
}

// Cloud Control only exposes the create-time cluster version. The native VKE
// upgrade APIs also return a TaskId: acceptance alone is not upgrade success.
// https://docs.volcengine.com/docs/vke/UpdateClusterVersion
// https://docs.volcengine.com/docs/vke/UpgradeNodePools
type upgradeClient struct {
	call         func(context.Context, string, map[string]any, any) error
	accountID    int64
	pollInterval time.Duration
}

func newUpgradeClient(ctx context.Context, provider tfcloudcontrol.Provider) (*upgradeClient, error) {
	// Reuse credentials (including session tokens), transport and proxy settings.
	// The Cloud Control endpoint must not override the VKE/STS service endpoints.
	cfg := provider.CloudControlAPIClient(ctx).Config.Copy()
	cfg.Endpoint = nil
	sess, err := session.NewSession(cfg)
	if err != nil {
		return nil, fmt.Errorf("create VKE session: %w", err)
	}
	identity, err := sts.New(sess).GetCallerIdentityWithContext(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return nil, fmt.Errorf("get account for VKE upgrade tasks: %w", err)
	}
	if identity == nil || identity.AccountId == nil || *identity.AccountId == 0 {
		return nil, fmt.Errorf("STS GetCallerIdentity returned no account ID")
	}
	client := sdkvke.New(sess)
	return &upgradeClient{
		accountID:    *identity.AccountId,
		pollInterval: 10 * time.Second,
		call: func(ctx context.Context, action string, input map[string]any, output any) error {
			// The pinned SDK does not yet generate the upgrade/task methods. Its
			// common request path still supplies signing, retries and cancellation.
			var body map[string]any
			req := client.NewRequest(&request.Operation{Name: action, HTTPMethod: "POST", HTTPPath: "/"}, &input, &body)
			req.HTTPRequest.Header.Set("Content-Type", "application/json")
			req.SetContext(ctx)
			if err := req.Send(); err != nil {
				return fmt.Errorf("VKE %s: %w", action, err)
			}
			result, ok := body["Result"]
			if !ok || result == nil {
				return fmt.Errorf("VKE %s returned no result", action)
			}
			data, err := json.Marshal(result)
			if err != nil {
				return fmt.Errorf("encode VKE %s result: %w", action, err)
			}
			if err := json.Unmarshal(data, output); err != nil {
				return fmt.Errorf("decode VKE %s result: %w", action, err)
			}
			return nil
		},
	}, nil
}

func (c *upgradeClient) clusterVersion(ctx context.Context, id string) (string, error) {
	var result struct {
		Items []struct {
			ID                string `json:"Id"`
			KubernetesVersion string
			Status            struct{ Phase string }
		}
	}
	if err := c.call(ctx, "ListClusters", map[string]any{"Filter": map[string]any{"Ids": []string{id}}}, &result); err != nil {
		return "", err
	}
	if len(result.Items) != 1 || result.Items[0].ID != id || result.Items[0].KubernetesVersion == "" {
		return "", fmt.Errorf("VKE cluster %s has no observed Kubernetes version", id)
	}
	if result.Items[0].Status.Phase != "Running" {
		return "", fmt.Errorf("VKE cluster %s is %s, waiting for Running", id, result.Items[0].Status.Phase)
	}
	return result.Items[0].KubernetesVersion, nil
}

func (c *upgradeClient) upgrade(ctx context.Context, clusterID, nodePoolID, target string) error {
	ctx, cancel := context.WithTimeout(ctx, time.Hour)
	defer cancel()
	action := "UpdateClusterVersion"
	input := map[string]any{"Id": clusterID, "KubernetesVersion": target, "Operation": "Upgrade", "IgnorePrecheckWarning": false}
	if nodePoolID != "" {
		version, err := c.clusterVersion(ctx, clusterID)
		if err != nil {
			return err
		}
		minor, err := minorVersion(version)
		if err != nil {
			return err
		}
		if minor != target {
			return fmt.Errorf("VKE cluster %s is at %s, node pool upgrade requires %s", clusterID, version, target)
		}
		action = "UpgradeNodePools"
		input = map[string]any{
			"ClusterId": clusterID, "KubernetesVersion": version, "Operation": "Upgrade", "IgnorePrecheckWarning": false,
			"NodePools":     []map[string]any{{"NodePoolId": nodePoolID}},
			"UpgradePolicy": map[string]any{"MaxUnavailable": 1, "MaxFailed": 0},
		}
	}

	token := tfcloudcontrol.OperationToken("upgrade", action, clusterID+"/"+nodePoolID, target)
	started := time.Now()
	previousFailedTask := ""
	for {
		input["ClientToken"] = token
		var result struct {
			TaskID string `json:"TaskId"`
		}
		if err := c.call(ctx, action, input, &result); err != nil {
			return err
		}
		if result.TaskID == "" {
			return fmt.Errorf("VKE %s returned no task ID", action)
		}
		task, err := c.waitTask(ctx, result.TaskID)
		if err != nil {
			return err
		}
		if task.Status.Phase == "Succeeded" {
			return nil
		}
		ended, parseErr := time.Parse(time.RFC3339, task.EndTime)
		if (task.Status.Phase != "Failed" && task.Status.Phase != "Aborted") || parseErr != nil || !ended.Before(started.Add(-time.Minute)) {
			return fmt.Errorf("VKE upgrade task %s is %s; inspect its precheck and task details", task.TaskID, task.Status.Phase)
		}
		if task.TaskID == previousFailedTask {
			return fmt.Errorf("VKE upgrade replay did not advance past failed task %s", task.TaskID)
		}
		previousFailedTask = task.TaskID
		// A replayed failed task needs a new attempt. Derive its token from the
		// failed task, so a restart resumes that same attempt rather than starting
		// a concurrent upgrade. Only a failure predating this invocation advances
		// the chain; a fresh failure is returned to Terraform/Crossplane.
		token = tfcloudcontrol.OperationToken("retry", action, task.TaskID, target)
		if err := ctx.Err(); err != nil {
			return err
		}
	}
}

type upgradeTask struct {
	TaskID     string `json:"TaskId"`
	Status     struct{ Phase string }
	EndTime    string
	Exceptions []struct{ Message string }
}

func (c *upgradeClient) waitTask(ctx context.Context, id string) (upgradeTask, error) {
	for {
		var result struct{ Items []upgradeTask }
		err := c.call(ctx, "ListTasks", map[string]any{
			"AccountId": c.accountID, "Filter": map[string]any{"Ids": []string{id}}, "PageSize": 100,
		}, &result)
		if err != nil {
			return upgradeTask{}, err
		}
		if len(result.Items) > 1 {
			return upgradeTask{}, fmt.Errorf("VKE ListTasks returned multiple tasks for %s", id)
		}
		if len(result.Items) == 1 {
			task := result.Items[0]
			if task.TaskID != id {
				return upgradeTask{}, fmt.Errorf("VKE ListTasks returned an unexpected task for %s", id)
			}
			switch task.Status.Phase {
			case "Succeeded":
				if len(task.Exceptions) != 0 {
					return task, fmt.Errorf("VKE upgrade task %s completed with exceptions", id)
				}
				return task, nil
			case "Failed", "Aborted", "Paused", "Approving":
				return task, nil
			case "Running", "Pending", "Waiting":
			default:
				return task, fmt.Errorf("VKE upgrade task %s has unknown phase %q", id, task.Status.Phase)
			}
		}
		// An accepted task may not be visible immediately. Never interpret an
		// empty list as completion; the caller's timeout bounds the wait.
		timer := time.NewTimer(c.pollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return upgradeTask{}, fmt.Errorf("wait for VKE upgrade task %s: %w", id, ctx.Err())
		case <-timer.C:
		}
	}
}
