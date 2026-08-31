package generic

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	frameworkprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	providerschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestPlanResourceChangeNormalizesAutoscaledDesiredReplicas(t *testing.T) {
	t.Parallel()

	resourceType := vkeNodePoolTestType()
	server := providerserver.NewProtocol6(vkeNodePoolPlanTestProvider{})()

	tests := map[string]struct {
		config        tftypes.Value
		prior         tftypes.Value
		proposed      tftypes.Value
		wantDesired   *int64
		wantUnchanged bool
	}{
		"autoscaling enabled omitted desired": {
			config:      vkeNodePoolTestValue(resourceType, "pool-id", true, nil, int64Ptr(0)),
			prior:       vkeNodePoolTestValue(resourceType, "pool-id", true, int64Ptr(1), int64Ptr(0)),
			proposed:    vkeNodePoolTestValue(resourceType, "pool-id", true, int64Ptr(0), int64Ptr(0)),
			wantDesired: int64Ptr(1),
		},
		"autoscaling enabled explicit desired": {
			config:      vkeNodePoolTestValue(resourceType, "pool-id", true, int64Ptr(3), int64Ptr(0)),
			prior:       vkeNodePoolTestValue(resourceType, "pool-id", true, int64Ptr(1), int64Ptr(0)),
			proposed:    vkeNodePoolTestValue(resourceType, "pool-id", true, int64Ptr(3), int64Ptr(0)),
			wantDesired: int64Ptr(1),
		},
		"autoscaling disabled keeps planned desired": {
			config:        vkeNodePoolTestValue(resourceType, "pool-id", false, int64Ptr(2), int64Ptr(0)),
			prior:         vkeNodePoolTestValue(resourceType, "pool-id", false, int64Ptr(1), int64Ptr(0)),
			proposed:      vkeNodePoolTestValue(resourceType, "pool-id", false, int64Ptr(2), int64Ptr(0)),
			wantUnchanged: true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			planned := planVKENodePoolChange(t, server, resourceType, tt.config, tt.prior, tt.proposed)
			if tt.wantUnchanged {
				desired, ok := vkeNodePoolObjectAttr(planned, vkeNodePoolAutoScalingAttr, vkeNodePoolDesiredReplicasAttr)
				if !ok {
					t.Fatal("planned state missing desired_replicas")
				}
				got := tftypesNumberAsInt64(t, desired)
				want := tftypesNumberAsInt64(t, mustVKENodePoolDesired(t, tt.proposed))
				if got != want {
					t.Fatalf("desired_replicas = %d, want planned %d", got, want)
				}
				return
			}
			desired, ok := vkeNodePoolObjectAttr(planned, vkeNodePoolAutoScalingAttr, vkeNodePoolDesiredReplicasAttr)
			if !ok {
				t.Fatal("planned state missing desired_replicas")
			}
			if tftypesNumberAsInt64(t, desired) != *tt.wantDesired {
				t.Fatalf("desired_replicas = %d, want %d", tftypesNumberAsInt64(t, desired), *tt.wantDesired)
			}
			paths := filteredPlanDiffPaths(t, planned, tt.prior)
			if len(paths) != 0 {
				t.Fatalf("filtered diff paths = %v, want none", paths)
			}
		})
	}
}

func TestPlanResourceChangeReportsDesiredReplicasDiffPath(t *testing.T) {
	t.Parallel()

	resourceType := vkeNodePoolTestType()
	prior := vkeNodePoolTestValue(resourceType, "pool-id", true, int64Ptr(1), int64Ptr(0))
	proposed := vkeNodePoolTestValue(resourceType, "pool-id", true, int64Ptr(0), int64Ptr(0))
	before := filteredPlanDiffPaths(t, proposed, prior)
	if len(before) != 1 || !strings.Contains(before[0], vkeNodePoolDesiredReplicasAttr) {
		t.Fatalf("proposed filtered diff paths = %v, want only %s", before, vkeNodePoolDesiredReplicasAttr)
	}

	planned := planVKENodePoolChange(t, providerserver.NewProtocol6(vkeNodePoolPlanTestProvider{})(), resourceType,
		vkeNodePoolTestValue(resourceType, "pool-id", true, nil, int64Ptr(0)),
		prior,
		proposed,
	)
	after := filteredPlanDiffPaths(t, planned, prior)
	if len(after) != 0 {
		t.Fatalf("planned filtered diff paths = %v, want none", after)
	}
}

func TestPlanResourceChangeKeepsInjectedSecurityGroups(t *testing.T) {
	t.Parallel()

	resourceType := vkeNodePoolTestType()
	prior := vkeNodePoolTestValue(resourceType, "pool-id", true, int64Ptr(0), int64Ptr(0), "sg-node", "sg-cluster")
	proposed := vkeNodePoolTestValue(resourceType, "pool-id", true, int64Ptr(0), int64Ptr(0), "sg-node")
	before := filteredPlanDiffPaths(t, proposed, prior)
	if len(before) != 1 || !strings.Contains(before[0], vkeNodePoolSecurityGroupIDsAttr) {
		t.Fatalf("proposed filtered diff paths = %v, want only %s", before, vkeNodePoolSecurityGroupIDsAttr)
	}

	planned := planVKENodePoolChange(t, providerserver.NewProtocol6(vkeNodePoolPlanTestProvider{})(), resourceType,
		vkeNodePoolTestValue(resourceType, "pool-id", true, int64Ptr(0), int64Ptr(0), "sg-node"),
		prior,
		proposed,
	)
	ids, _, ok := vkeNodePoolSecurityGroupIDSet(planned)
	if !ok || !stringSetEqual(ids, []string{"sg-node", "sg-cluster"}) {
		t.Fatalf("planned security_group_ids = %v, want injected cluster sg kept", ids)
	}
	after := filteredPlanDiffPaths(t, planned, prior)
	if len(after) != 0 {
		t.Fatalf("planned filtered diff paths = %v, want none", after)
	}
}

func TestPlanResourceChangeNormalizesLiveDesiredAndSecurityGroups(t *testing.T) {
	t.Parallel()

	resourceType := vkeNodePoolTestType()
	prior := vkeNodePoolTestValue(resourceType, "pool-id", true, int64Ptr(1), int64Ptr(0), "sg-node", "sg-cluster")
	proposed := vkeNodePoolTestValue(resourceType, "pool-id", true, int64Ptr(0), int64Ptr(0), "sg-node")

	planned := planVKENodePoolChange(t, providerserver.NewProtocol6(vkeNodePoolPlanTestProvider{})(), resourceType,
		vkeNodePoolTestValue(resourceType, "pool-id", true, nil, int64Ptr(0), "sg-node"),
		prior,
		proposed,
	)
	after := filteredPlanDiffPaths(t, planned, prior)
	if len(after) != 0 {
		t.Fatalf("planned filtered diff paths = %v, want none", after)
	}
}

func planVKENodePoolChange(t *testing.T, server tfprotov6.ProviderServer, resourceType tftypes.Type, config, prior, proposed tftypes.Value) tftypes.Value {
	t.Helper()
	response, err := server.PlanResourceChange(context.Background(), &tfprotov6.PlanResourceChangeRequest{
		TypeName:         "test_vke_node_pool",
		Config:           identityPlanDynamicValue(t, resourceType, config),
		PriorState:       identityPlanDynamicValue(t, resourceType, prior),
		ProposedNewState: identityPlanDynamicValue(t, resourceType, proposed),
	})
	if err != nil {
		t.Fatalf("PlanResourceChange() error = %v", err)
	}
	for _, diagnostic := range response.Diagnostics {
		if diagnostic.Severity == tfprotov6.DiagnosticSeverityError {
			t.Fatalf("PlanResourceChange() diagnostic = %s: %s", diagnostic.Summary, diagnostic.Detail)
		}
	}
	planned, err := response.PlannedState.Unmarshal(resourceType)
	if err != nil {
		t.Fatalf("unmarshalling planned state: %v", err)
	}
	return planned
}

func mustVKENodePoolDesired(t *testing.T, root tftypes.Value) tftypes.Value {
	t.Helper()
	desired, ok := vkeNodePoolObjectAttr(root, vkeNodePoolAutoScalingAttr, vkeNodePoolDesiredReplicasAttr)
	if !ok {
		t.Fatal("missing desired_replicas")
	}
	return desired
}

type vkeNodePoolPlanTestProvider struct{}

func (vkeNodePoolPlanTestProvider) Metadata(_ context.Context, _ frameworkprovider.MetadataRequest, response *frameworkprovider.MetadataResponse) {
	response.TypeName = "test"
}

func (vkeNodePoolPlanTestProvider) Schema(_ context.Context, _ frameworkprovider.SchemaRequest, response *frameworkprovider.SchemaResponse) {
	response.Schema = providerschema.Schema{}
}

func (vkeNodePoolPlanTestProvider) Configure(context.Context, frameworkprovider.ConfigureRequest, *frameworkprovider.ConfigureResponse) {
}

func (vkeNodePoolPlanTestProvider) DataSources(context.Context) []func() datasource.DataSource {
	return nil
}

func (vkeNodePoolPlanTestProvider) Resources(context.Context) []func() resource.Resource {
	return []func() resource.Resource{vkeNodePoolPlanTestResource}
}

func vkeNodePoolPlanTestResource() resource.Resource {
	resourceSchema := schema.Schema{Attributes: map[string]schema.Attribute{
		"id": schema.StringAttribute{Computed: true},
		"auto_scaling": schema.SingleNestedAttribute{
			Optional: true,
			Computed: true,
			Attributes: map[string]schema.Attribute{
				"enabled":          schema.BoolAttribute{Optional: true, Computed: true},
				"desired_replicas": schema.Int64Attribute{Optional: true, Computed: true},
				"min_replicas":     schema.Int64Attribute{Optional: true, Computed: true},
			},
		},
		"node_config": schema.SingleNestedAttribute{
			Optional: true,
			Computed: true,
			Attributes: map[string]schema.Attribute{
				"security": schema.SingleNestedAttribute{
					Optional: true,
					Computed: true,
					Attributes: map[string]schema.Attribute{
						"security_group_ids": schema.SetAttribute{
							ElementType: types.StringType,
							Optional:    true,
							Computed:    true,
						},
					},
				},
			},
		},
	}}
	value, err := NewResource(context.Background(),
		resourceWithCloudControlTypeName(volcengineVKENodePoolType),
		resourceWithTerraformTypeName("test_vke_node_pool"),
		resourceWithTerraformSchema(resourceSchema),
		resourceWithAttributeNameMap(map[string]string{
			"id":                 "ID",
			"auto_scaling":       "AutoScaling",
			"enabled":            "Enabled",
			"desired_replicas":   "DesiredReplicas",
			"min_replicas":       "MinReplicas",
			"node_config":        "NodeConfig",
			"security":           "Security",
			"security_group_ids": "SecurityGroupIds",
		}),
	)
	if err != nil {
		panic(err)
	}
	return value
}
