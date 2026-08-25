// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package cloudcontrol

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/volcengine/terraform-provider-volcenginecc/internal/base"
	"github.com/volcengine/terraform-provider-volcenginecc/internal/cloudcontrol"
	"github.com/volcengine/terraform-provider-volcenginecc/internal/tfresource"
	"github.com/volcengine/terraform-provider-volcenginecc/internal/util"
)

func DeleteResource(ctx context.Context, cloudControlClient *cloudcontrol.CloudControl, region, roleARN, typeName, id string) error {
	tflog.Debug(ctx, "DeleteResource", map[string]interface{}{
		"cfTypeName": typeName,
		"id":         id,
	})

	resp, err := cloudControlClient.DeleteResourceWithContext(ctx, &cloudcontrol.DeleteResourceInput{
		TypeName:    util.StringPtr(typeName),
		RegionID:    util.StringPtr(region),
		Identifier:  util.StringPtr(id),
		ClientToken: util.StringPtr(OperationToken("delete", typeName, id, "")),
	})
	if err != nil {
		return wrapCloudControlNotFound(err)
	}
	if resp == nil || resp.OperationStatus == nil {
		return fmt.Errorf("DeleteResource returned an empty operation status")
	}
	taskId := ""
	status := *resp.OperationStatus
	if status == base.FAILED {
		err := fmt.Errorf("DeleteResource failed: status=%q request_id=%q task_id=%q error_code=%q",
			status, resp.GetRequestId(), util.ToString(resp.TaskID), util.ToString(resp.ErrorCode))
		if isCloudControlNotFoundProgressEvent(&resp.ProgressEvent) {
			return &tfresource.NotFoundError{LastError: err}
		}
		return err
	} else if status == base.SUCCESS {
		return nil
	} else if status == base.IN_PROGRESS || status == base.PENDING {
		if resp.TaskID == nil || *resp.TaskID == "" {
			return fmt.Errorf("call DeleteResource returned %s without a task ID", status)
		}
		taskId = *resp.TaskID
	} else {
		return fmt.Errorf("DeleteResource returned an unexpected status: status=%q request_id=%q task_id=%q",
			status, resp.GetRequestId(), util.ToString(resp.TaskID))
	}
	tflog.Info(ctx, "Cloud Control API DeleteResource waiting task ...... ", map[string]interface{}{
		"TaskID":    hclog.Fmt("%v", taskId),
		"RequestID": hclog.Fmt("%v", resp.GetRequestId()),
	})
	_, _, err = AwaitTask(ctx, cloudControlClient, taskId)
	if err != nil {
		return err
	}
	return nil
}

func InvokeGetTask(ctx context.Context, client *cloudcontrol.CloudControl, taskId string) (*cloudcontrol.ProgressEvent, error) {
	input := &cloudcontrol.GetTaskInput{
		TaskID: &taskId,
	}
	output, err := client.GetTaskWithContext(ctx, input)

	if err != nil {
		return nil, err
	}
	if output == nil {
		return nil, fmt.Errorf("GetTask returned an empty response")
	}

	return &output.ProgressEvent, nil
}

func AwaitTask(ctx context.Context, client *cloudcontrol.CloudControl, taskId string) (*cloudcontrol.ProgressEvent, bool, error) {
	deadline := time.NewTimer(24 * time.Hour)
	defer deadline.Stop()
	initialPoll := time.NewTimer(time.Second)
	select {
	case <-ctx.Done():
		if !initialPoll.Stop() {
			select {
			case <-initialPoll.C:
			default:
			}
		}
		return nil, false, ctx.Err()
	case <-deadline.C:
		if !initialPoll.Stop() {
			select {
			case <-initialPoll.C:
			default:
			}
		}
		return nil, false, fmt.Errorf("await task timeout")
	case <-initialPoll.C:
	}

	poll := time.NewTicker(10 * time.Second)
	defer poll.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, false, ctx.Err()
		case <-deadline.C:
			return nil, false, fmt.Errorf("await task timeout")
		case <-poll.C:
		}
		output, err := InvokeGetTask(ctx, client, taskId)
		if err != nil {
			return nil, false, fmt.Errorf("invoke get task: %w", err)
		}
		if output == nil || output.OperationStatus == nil {
			return nil, false, fmt.Errorf("invoke get task returned an empty operation status")
		}

		status := *output.OperationStatus
		if status == base.FAILED {
			err := fmt.Errorf("get task failed: status=%q task_id=%q type=%q identifier=%q error_code=%q",
				status, util.ToString(output.TaskID), util.ToString(output.TypeName), util.ToString(output.Identifier), util.ToString(output.ErrorCode))
			if isCloudControlNotFoundProgressEvent(output) {
				return nil, false, &tfresource.NotFoundError{LastError: err}
			}
			return nil, false, err
		} else if status == base.SUCCESS {
			return output, true, nil
		} else if status == base.IN_PROGRESS || status == base.PENDING {
			continue
		} else {
			return nil, false, fmt.Errorf("get task returned an unexpected status: status=%q task_id=%q type=%q identifier=%q",
				status, util.ToString(output.TaskID), util.ToString(output.TypeName), util.ToString(output.Identifier))
		}

	}
}
