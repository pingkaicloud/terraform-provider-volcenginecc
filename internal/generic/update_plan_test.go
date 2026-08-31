package generic

import (
	"math/big"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestNormalizeVKENodePoolDesiredReplicasPlan(t *testing.T) {
	t.Parallel()

	resourceType := vkeNodePoolTestType()
	tests := map[string]struct {
		state         tftypes.Value
		plan          tftypes.Value
		wantDesired   *int64
		wantUnchanged bool
	}{
		"autoscaling enabled omitted desired uses prior": {
			state:       vkeNodePoolTestValue(resourceType, "pool-id", true, int64Ptr(1), int64Ptr(0)),
			plan:        vkeNodePoolTestValue(resourceType, "pool-id", true, int64Ptr(0), int64Ptr(0)),
			wantDesired: int64Ptr(1),
		},
		"autoscaling enabled explicit desired still uses prior": {
			state:       vkeNodePoolTestValue(resourceType, "pool-id", true, int64Ptr(1), int64Ptr(0)),
			plan:        vkeNodePoolTestValue(resourceType, "pool-id", true, int64Ptr(3), int64Ptr(0)),
			wantDesired: int64Ptr(1),
		},
		"autoscaling disabled keeps planned desired": {
			state:         vkeNodePoolTestValue(resourceType, "pool-id", false, int64Ptr(1), int64Ptr(0)),
			plan:          vkeNodePoolTestValue(resourceType, "pool-id", false, int64Ptr(2), int64Ptr(0)),
			wantUnchanged: true,
		},
		"create without prior keeps planned desired": {
			state:         tftypes.NewValue(resourceType, nil),
			plan:          vkeNodePoolTestValue(resourceType, nil, true, int64Ptr(0), int64Ptr(0)),
			wantUnchanged: true,
		},
		"prior without desired keeps planned desired": {
			state:         vkeNodePoolTestValue(resourceType, "pool-id", true, nil, int64Ptr(0)),
			plan:          vkeNodePoolTestValue(resourceType, "pool-id", true, int64Ptr(0), int64Ptr(0)),
			wantUnchanged: true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := normalizeVKENodePoolDesiredReplicasPlan(tt.state, tt.plan)
			if err != nil {
				t.Fatalf("normalizeVKENodePoolDesiredReplicasPlan() error = %v", err)
			}
			if tt.wantUnchanged {
				if !got.Equal(tt.plan) {
					t.Fatalf("planned state = %s, want unchanged %s", got, tt.plan)
				}
				return
			}
			desired, ok := vkeNodePoolObjectAttr(got, vkeNodePoolAutoScalingAttr, vkeNodePoolDesiredReplicasAttr)
			if !ok {
				t.Fatal("planned state missing desired_replicas")
			}
			gotDesired := tftypesNumberAsInt64(t, desired)
			if gotDesired != *tt.wantDesired {
				t.Fatalf("desired_replicas = %d, want %d", gotDesired, *tt.wantDesired)
			}
		})
	}
}

func TestNormalizeVKENodePoolSecurityGroupIDsPlan(t *testing.T) {
	t.Parallel()

	resourceType := vkeNodePoolTestType()
	nodeSG := "sg-node"
	clusterSG := "sg-cluster"
	tests := map[string]struct {
		state         tftypes.Value
		plan          tftypes.Value
		wantIDs       []string
		wantUnchanged bool
	}{
		"keeps injected cluster sg when config is subset": {
			state:   vkeNodePoolTestValue(resourceType, "pool-id", true, int64Ptr(1), int64Ptr(0), nodeSG, clusterSG),
			plan:    vkeNodePoolTestValue(resourceType, "pool-id", true, int64Ptr(1), int64Ptr(0), nodeSG),
			wantIDs: []string{nodeSG, clusterSG},
		},
		"keeps planned set when adding a new sg": {
			state:         vkeNodePoolTestValue(resourceType, "pool-id", true, int64Ptr(1), int64Ptr(0), nodeSG, clusterSG),
			plan:          vkeNodePoolTestValue(resourceType, "pool-id", true, int64Ptr(1), int64Ptr(0), nodeSG, "sg-new"),
			wantUnchanged: true,
		},
		"create without prior keeps planned set": {
			state:         tftypes.NewValue(resourceType, nil),
			plan:          vkeNodePoolTestValue(resourceType, nil, true, int64Ptr(0), int64Ptr(0), nodeSG),
			wantUnchanged: true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := normalizeVKENodePoolSecurityGroupIDsPlan(tt.state, tt.plan)
			if err != nil {
				t.Fatalf("normalizeVKENodePoolSecurityGroupIDsPlan() error = %v", err)
			}
			if tt.wantUnchanged {
				if !got.Equal(tt.plan) {
					t.Fatalf("planned state = %s, want unchanged %s", got, tt.plan)
				}
				return
			}
			ids, _, ok := vkeNodePoolSecurityGroupIDSet(got)
			if !ok {
				t.Fatal("planned state missing security_group_ids")
			}
			if !stringSetEqual(ids, tt.wantIDs) {
				t.Fatalf("security_group_ids = %v, want %v", ids, tt.wantIDs)
			}
		})
	}
}

func TestNormalizeVKENodePoolSecurityGroupIDsPlanDiffPath(t *testing.T) {
	t.Parallel()

	resourceType := vkeNodePoolTestType()
	prior := vkeNodePoolTestValue(resourceType, "pool-id", true, int64Ptr(1), int64Ptr(0), "sg-node", "sg-cluster")
	planned := vkeNodePoolTestValue(resourceType, "pool-id", true, int64Ptr(1), int64Ptr(0), "sg-node")

	before := filteredPlanDiffPaths(t, planned, prior)
	if len(before) != 1 || !strings.Contains(before[0], vkeNodePoolSecurityGroupIDsAttr) {
		t.Fatalf("pre-normalize filtered diff paths = %v, want only %s", before, vkeNodePoolSecurityGroupIDsAttr)
	}

	normalized, err := normalizeVKENodePoolSecurityGroupIDsPlan(prior, planned)
	if err != nil {
		t.Fatalf("normalizeVKENodePoolSecurityGroupIDsPlan() error = %v", err)
	}
	after := filteredPlanDiffPaths(t, normalized, prior)
	if len(after) != 0 {
		t.Fatalf("post-normalize filtered diff paths = %v, want none", after)
	}
}

func TestNormalizeVKENodePoolDesiredReplicasPlanDiffPath(t *testing.T) {
	t.Parallel()

	resourceType := vkeNodePoolTestType()
	prior := vkeNodePoolTestValue(resourceType, "pool-id", true, int64Ptr(1), int64Ptr(0))
	planned := vkeNodePoolTestValue(resourceType, "pool-id", true, int64Ptr(0), int64Ptr(0))

	before := filteredPlanDiffPaths(t, planned, prior)
	if len(before) != 1 || !strings.Contains(before[0], vkeNodePoolDesiredReplicasAttr) {
		t.Fatalf("pre-normalize filtered diff paths = %v, want only %s", before, vkeNodePoolDesiredReplicasAttr)
	}

	normalized, err := normalizeVKENodePoolDesiredReplicasPlan(prior, planned)
	if err != nil {
		t.Fatalf("normalizeVKENodePoolDesiredReplicasPlan() error = %v", err)
	}
	after := filteredPlanDiffPaths(t, normalized, prior)
	if len(after) != 0 {
		t.Fatalf("post-normalize filtered diff paths = %v, want none", after)
	}
}

func vkeNodePoolTestType() tftypes.Object {
	return tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"id": tftypes.String,
		"auto_scaling": tftypes.Object{AttributeTypes: map[string]tftypes.Type{
			"enabled":          tftypes.Bool,
			"desired_replicas": tftypes.Number,
			"min_replicas":     tftypes.Number,
		}},
		"node_config": tftypes.Object{AttributeTypes: map[string]tftypes.Type{
			"security": tftypes.Object{AttributeTypes: map[string]tftypes.Type{
				"security_group_ids": tftypes.Set{ElementType: tftypes.String},
			}},
		}},
	}}
}

func vkeNodePoolTestValue(resourceType tftypes.Type, id any, enabled bool, desired *int64, min *int64, securityGroupIDs ...string) tftypes.Value {
	objectType := resourceType.(tftypes.Object)
	nodeConfigType := objectType.AttributeTypes["node_config"].(tftypes.Object)
	securityType := nodeConfigType.AttributeTypes["security"].(tftypes.Object)
	return tftypes.NewValue(resourceType, map[string]tftypes.Value{
		"id": tftypes.NewValue(tftypes.String, id),
		"auto_scaling": tftypes.NewValue(objectType.AttributeTypes["auto_scaling"], map[string]tftypes.Value{
			"enabled":          tftypes.NewValue(tftypes.Bool, enabled),
			"desired_replicas": vkeNodePoolNumberValue(desired),
			"min_replicas":     vkeNodePoolNumberValue(min),
		}),
		"node_config": tftypes.NewValue(nodeConfigType, map[string]tftypes.Value{
			"security": tftypes.NewValue(securityType, map[string]tftypes.Value{
				"security_group_ids": vkeNodePoolSecurityGroupIDValue(securityType.AttributeTypes["security_group_ids"], securityGroupIDs),
			}),
		}),
	})
}

func vkeNodePoolSecurityGroupIDValue(setType tftypes.Type, ids []string) tftypes.Value {
	if ids == nil {
		return tftypes.NewValue(setType, nil)
	}
	values := make([]tftypes.Value, 0, len(ids))
	for _, id := range ids {
		values = append(values, tftypes.NewValue(tftypes.String, id))
	}
	return tftypes.NewValue(setType, values)
}

func vkeNodePoolNumberValue(value *int64) tftypes.Value {
	if value == nil {
		return tftypes.NewValue(tftypes.Number, nil)
	}
	return tftypes.NewValue(tftypes.Number, new(big.Float).SetInt64(*value))
}

func int64Ptr(value int64) *int64 { return &value }

func tftypesNumberAsInt64(t *testing.T, value tftypes.Value) int64 {
	t.Helper()
	var number big.Float
	if err := value.As(&number); err != nil {
		t.Fatalf("decoding number: %v", err)
	}
	got, _ := number.Int64()
	return got
}

// filteredPlanDiffPaths mirrors upjet's filteredDiffExists: only planned
// values that are known and non-null count as a Crossplane update trigger.
func filteredPlanDiffPaths(t *testing.T, planned, prior tftypes.Value) []string {
	t.Helper()
	diffs, err := planned.Diff(prior)
	if err != nil {
		t.Fatalf("Diff() error = %v", err)
	}
	var paths []string
	for _, diff := range diffs {
		if diff.Value1 != nil && diff.Value1.IsKnown() && !diff.Value1.IsNull() {
			paths = append(paths, diff.Path.String())
		}
	}
	return paths
}
