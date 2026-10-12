package vke

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/volcengine/terraform-provider-volcenginecc/internal/cloudcontrol"
	tfcloudcontrol "github.com/volcengine/terraform-provider-volcenginecc/internal/service/cloudcontrol"
	"github.com/volcengine/volcengine-go-sdk/volcengine"
	"github.com/volcengine/volcengine-go-sdk/volcengine/credentials"
	"github.com/volcengine/volcengine-go-sdk/volcengine/session"
)

func TestUpgradeWaitsForCloudTask(t *testing.T) {
	for _, phase := range []string{"Succeeded", "Failed", "Paused", "Approving", ""} {
		t.Run(phase, func(t *testing.T) {
			submissions, polls := 0, 0
			c := &upgradeClient{accountID: 123, call: func(_ context.Context, action string, input map[string]any, out any) error {
				var response string
				switch action {
				case "ListClusters":
					response = `{"Items":[{"Id":"cluster","KubernetesVersion":"v1.34.6-vke.12","Status":{"Phase":"Running"}}]}`
				case "UpgradeNodePools":
					submissions++
					if input["KubernetesVersion"] != "v1.34.6-vke.12" || input["Operation"] != "Upgrade" || input["IgnorePrecheckWarning"] != false {
						t.Fatal("upgrade must use full observed VKE version and retain prechecks")
					}
					response = `{"TaskId":"task"}`
				case "ListTasks":
					polls++
					if input["AccountId"] != int64(123) {
						t.Fatal("missing account ID")
					}
					response = `{"Items":[]}`
					if polls == 2 {
						response = `{"Items":[{"TaskId":"task","Status":{"Phase":"Running"}}]}`
					} else if polls == 3 {
						response = fmt.Sprintf(`{"Items":[{"TaskId":"task","Status":{"Phase":%q}}]}`, phase)
					}
				default:
					t.Fatalf("unexpected action %s", action)
				}
				return json.Unmarshal([]byte(response), out)
			}}
			err := c.upgrade(context.Background(), "cluster", "pool", "1.34")
			if (err == nil) != (phase == "Succeeded") || submissions != 1 || polls != 3 {
				t.Fatalf("upgrade: error=%v submissions=%d polls=%d", err, submissions, polls)
			}
		})
	}
}

type upgradeTransport func(*http.Request) (*http.Response, error)

func (f upgradeTransport) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

type upgradeProvider struct{ client *cloudcontrol.CloudControl }

func (p upgradeProvider) CloudControlAPIClient(context.Context) *cloudcontrol.CloudControl {
	return p.client
}
func (upgradeProvider) Region(context.Context) string                      { return "cn-beijing" }
func (upgradeProvider) CreateIdentity(context.Context) string              { return "" }
func (upgradeProvider) RegisterLogger(ctx context.Context) context.Context { return ctx }

func TestUpgradeUsesSDKTransportAndJSONContract(t *testing.T) {
	var actions []string
	httpClient := &http.Client{Transport: upgradeTransport(func(req *http.Request) (*http.Response, error) {
		action := req.URL.Query().Get("Action")
		actions = append(actions, action)
		result := `{"AccountId":123}`
		if action != "GetCallerIdentity" {
			if !strings.HasPrefix(req.Header.Get("Content-Type"), "application/json") || req.URL.Query().Get("Version") != "2022-05-12" {
				t.Fatal("native VKE APIs require JSON and the VKE API version")
			}
			var input map[string]any
			if err := json.NewDecoder(req.Body).Decode(&input); err != nil {
				t.Fatal(err)
			}
			switch action {
			case "UpdateClusterVersion":
				if input["Id"] != "cluster" || input["KubernetesVersion"] != "1.34" || input["Operation"] != "Upgrade" || input["ClientToken"] == "" {
					t.Fatal("invalid native control plane request")
				}
				result = `{"TaskId":"task"}`
			case "ListTasks":
				if input["AccountId"] != float64(123) {
					t.Fatal("task account must come from STS")
				}
				result = `{"Items":[{"TaskId":"task","Status":{"Phase":"Succeeded"}}]}`
			default:
				t.Fatalf("unexpected API %s", action)
			}
		}
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}},
			Body: io.NopCloser(strings.NewReader(`{"ResponseMetadata":{},"Result":` + result + `}`))}, nil
	})}
	sess, err := session.NewSession(&volcengine.Config{Region: volcengine.String("cn-beijing"), HTTPClient: httpClient,
		Credentials: credentials.NewStaticCredentials("test-access", "test-secret", "test-session")})
	if err != nil {
		t.Fatal(err)
	}
	c, err := newUpgradeClient(context.Background(), upgradeProvider{cloudcontrol.New(sess)})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.upgrade(context.Background(), "cluster", "", "1.34"); err != nil {
		t.Fatal(err)
	}
	if strings.Join(actions, ",") != "GetCallerIdentity,UpdateClusterVersion,ListTasks" {
		t.Fatal(actions)
	}
}

func TestUpgradeResumesAttemptAfterReplayedFailure(t *testing.T) {
	var tokens []string
	for range 2 {
		attempt := 0
		c := &upgradeClient{call: func(_ context.Context, action string, input map[string]any, out any) error {
			if action == "UpdateClusterVersion" {
				attempt++
				tokens = append(tokens, input["ClientToken"].(string))
				return json.Unmarshal([]byte(fmt.Sprintf(`{"TaskId":"task%d"}`, attempt)), out)
			}
			result := `{"Items":[{"TaskId":"task2","Status":{"Phase":"Succeeded"}}]}`
			if attempt == 1 {
				result = `{"Items":[{"TaskId":"task1","Status":{"Phase":"Failed"},"EndTime":"2026-01-01T00:00:00Z"}]}`
			}
			return json.Unmarshal([]byte(result), out)
		}}
		if err := c.upgrade(context.Background(), "cluster", "", "1.34"); err != nil {
			t.Fatal(err)
		}
	}
	if len(tokens) != 4 || tokens[0] == tokens[1] || tokens[0] != tokens[2] || tokens[1] != tokens[3] {
		t.Fatal("a restart must replay the same initial and retry attempts")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c := &upgradeClient{pollInterval: time.Second, call: func(context.Context, string, map[string]any, any) error { return nil }}
	if _, err := c.waitTask(ctx, "missing"); err == nil {
		t.Fatal("an absent task must time out, not complete")
	}
	if err := c.upgrade(context.Background(), "cluster", "", "1.34"); err == nil || !strings.Contains(err.Error(), "no task ID") {
		t.Fatalf("an accepted response without a task ID is not completion: %v", err)
	}
}

// This fake checks the boundary with the unmodified Cloud Control resource.
type upgradeInner struct{ resource.Resource }

func (*upgradeInner) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema.Attributes = map[string]schema.Attribute{
		"id": schema.StringAttribute{Computed: true}, "cluster_id": schema.StringAttribute{Optional: true},
		"node_statistics": schema.SingleNestedAttribute{Computed: true, Attributes: map[string]schema.Attribute{
			"total_count": schema.Int64Attribute{Computed: true},
		}},
	}
}

func (*upgradeInner) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	for _, field := range []string{"kubernetes_version", appliedVersionField} {
		var value types.String
		resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root(field), &value)...)
		if !value.IsNull() {
			resp.Diagnostics.AddError("unexpected Cloud Control property", field)
		}
	}
	resp.State = tfsdk.State{Schema: req.Plan.Schema, Raw: req.Plan.Raw}
}

func (*upgradeInner) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	resp.State = req.State
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("kubernetes_version"), types.StringNull())...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(appliedVersionField), types.StringNull())...)
}

func TestNodePoolVersionRequiresCompletedApply(t *testing.T) {
	ctx := context.Background()
	r := &upgradeResource{Resource: &upgradeInner{}, nodePool: true}
	var sr resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &sr)
	tfType := sr.Schema.Type().TerraformType(ctx)
	for _, applied := range []string{`null`, `"1.32"`} {
		prior := tfsdk.State{Schema: sr.Schema, Raw: mustTFValueFromJSON(t, tfType,
			`{"id":"pool","cluster_id":"cluster","kubernetes_version":"1.34","applied_kubernetes_version":`+applied+`}`)}
		var rr resource.ReadResponse
		r.Read(ctx, resource.ReadRequest{State: prior}, &rr)
		if rr.Diagnostics.HasError() {
			t.Fatal(rr.Diagnostics)
		}
		var observed types.String
		rr.State.GetAttribute(ctx, path.Root("kubernetes_version"), &observed)
		if observed.ValueString() != strings.Trim(applied, `"`) && !(applied == "null" && observed.IsNull()) {
			t.Fatal("read must use completed state, never the desired version")
		}
	}
	prior := tfsdk.State{Schema: sr.Schema, Raw: mustTFValueFromJSON(t, tfType,
		`{"id":"pool","cluster_id":"cluster","kubernetes_version":"1.32","applied_kubernetes_version":"1.32"}`)}
	plan := tfsdk.Plan{Schema: sr.Schema, Raw: prior.Raw}
	plan.SetAttribute(ctx, path.Root("kubernetes_version"), "1.34")
	// No version property may leak to Cloud Control, even if the native call fails.
	r.newClient = func(context.Context, tfcloudcontrol.Provider) (*upgradeClient, error) {
		return nil, fmt.Errorf("simulated native API failure")
	}
	var ur resource.UpdateResponse
	r.Update(ctx, resource.UpdateRequest{State: prior, Plan: plan}, &ur)
	if len(ur.Diagnostics) != 1 || ur.Diagnostics[0].Summary() != "VKE Kubernetes upgrade" {
		t.Fatal(ur.Diagnostics)
	}
	var applied types.String
	ur.State.GetAttribute(ctx, path.Root(appliedVersionField), &applied)
	if applied.ValueString() != "1.32" {
		t.Fatal("failed upgrade must retain the previous completed version")
	}
	plan.SetAttribute(ctx, path.Root("node_statistics").AtName("total_count"), int64(0))
	r.newClient = func(context.Context, tfcloudcontrol.Provider) (*upgradeClient, error) {
		return &upgradeClient{call: func(_ context.Context, action string, _ map[string]any, out any) error {
			if action != "ListClusters" {
				t.Fatalf("empty pools only need control plane verification, got %s", action)
			}
			return json.Unmarshal([]byte(`{"Items":[{"Id":"cluster","KubernetesVersion":"v1.34.6-vke.12","Status":{"Phase":"Running"}}]}`), out)
		}}, nil
	}
	ur = resource.UpdateResponse{}
	r.Update(ctx, resource.UpdateRequest{State: prior, Plan: plan}, &ur)
	if ur.Diagnostics.HasError() {
		t.Fatal(ur.Diagnostics)
	}
	ur.State.GetAttribute(ctx, path.Root(appliedVersionField), &applied)
	if applied.ValueString() != "1.34" {
		t.Fatal("empty pool should inherit the verified control plane version")
	}
}
