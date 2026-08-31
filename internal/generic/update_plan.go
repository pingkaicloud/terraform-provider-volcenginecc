package generic

import "github.com/hashicorp/terraform-plugin-go/tftypes"

const (
	vkeNodePoolAutoScalingAttr     = "auto_scaling"
	vkeNodePoolEnabledAttr         = "enabled"
	vkeNodePoolDesiredReplicasAttr = "desired_replicas"
)

// normalizeVKENodePoolDesiredReplicasPlan copies prior desired_replicas into
// the plan when autoscaling owns that field. Create and import (no prior
// state) keep the planned value so the first apply can still send desired.
func normalizeVKENodePoolDesiredReplicasPlan(state, plan tftypes.Value) (tftypes.Value, error) {
	if state.IsNull() || !state.IsKnown() || plan.IsNull() || !plan.IsKnown() {
		return plan, nil
	}
	if !vkeNodePoolAutoscalingEnabledValue(plan) && !vkeNodePoolAutoscalingEnabledValue(state) {
		return plan, nil
	}
	desired, ok := vkeNodePoolObjectAttr(state, vkeNodePoolAutoScalingAttr, vkeNodePoolDesiredReplicasAttr)
	if !ok || desired.IsNull() || !desired.IsKnown() {
		return plan, nil
	}
	return setVKENodePoolDesiredReplicas(plan, desired)
}

func vkeNodePoolAutoscalingEnabledValue(root tftypes.Value) bool {
	enabled, ok := vkeNodePoolObjectAttr(root, vkeNodePoolAutoScalingAttr, vkeNodePoolEnabledAttr)
	if !ok || enabled.IsNull() || !enabled.IsKnown() {
		return false
	}
	var value bool
	if err := enabled.As(&value); err != nil {
		return false
	}
	return value
}

func vkeNodePoolObjectAttr(root tftypes.Value, names ...string) (tftypes.Value, bool) {
	current := root
	for _, name := range names {
		if current.IsNull() || !current.IsKnown() {
			return tftypes.Value{}, false
		}
		var object map[string]tftypes.Value
		if err := current.As(&object); err != nil {
			return tftypes.Value{}, false
		}
		next, ok := object[name]
		if !ok {
			return tftypes.Value{}, false
		}
		current = next
	}
	return current, true
}

func setVKENodePoolDesiredReplicas(plan tftypes.Value, desired tftypes.Value) (tftypes.Value, error) {
	var root map[string]tftypes.Value
	if err := plan.As(&root); err != nil {
		return plan, err
	}
	autoScaling, ok := root[vkeNodePoolAutoScalingAttr]
	if !ok || autoScaling.IsNull() || !autoScaling.IsKnown() {
		return plan, nil
	}
	var autoScalingObject map[string]tftypes.Value
	if err := autoScaling.As(&autoScalingObject); err != nil {
		return plan, err
	}
	autoScalingObject[vkeNodePoolDesiredReplicasAttr] = desired
	root[vkeNodePoolAutoScalingAttr] = tftypes.NewValue(autoScaling.Type(), autoScalingObject)
	return tftypes.NewValue(plan.Type(), root), nil
}
