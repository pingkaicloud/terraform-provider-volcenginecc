package generic

import "github.com/hashicorp/terraform-plugin-go/tftypes"

const (
	vkeNodePoolAutoScalingAttr      = "auto_scaling"
	vkeNodePoolEnabledAttr          = "enabled"
	vkeNodePoolDesiredReplicasAttr  = "desired_replicas"
	vkeNodePoolNodeConfigAttr       = "node_config"
	vkeNodePoolSecurityAttr         = "security"
	vkeNodePoolSecurityGroupIDsAttr = "security_group_ids"
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

// normalizeVKENodePoolSecurityGroupIDsPlan keeps VKE-injected security groups
// that are present in prior state when every configured ID is already there.
// DPM only writes the node SG it owns; VKE adds a cluster default SG. Create
// and a real add/replace (config not a subset of state) keep the planned set.
func normalizeVKENodePoolSecurityGroupIDsPlan(state, plan tftypes.Value) (tftypes.Value, error) {
	if state.IsNull() || !state.IsKnown() || plan.IsNull() || !plan.IsKnown() {
		return plan, nil
	}
	stateIDs, stateSet, ok := vkeNodePoolSecurityGroupIDSet(state)
	if !ok || stateSet.IsNull() || !stateSet.IsKnown() {
		return plan, nil
	}
	planIDs, planSet, ok := vkeNodePoolSecurityGroupIDSet(plan)
	if !ok || !planSet.IsKnown() {
		return plan, nil
	}
	if !planSet.IsNull() && !stringSetSubset(planIDs, stateIDs) {
		return plan, nil
	}
	if !planSet.IsNull() && stringSetEqual(planIDs, stateIDs) {
		return plan, nil
	}
	return setVKENodePoolNestedValue(plan, stateSet, vkeNodePoolNodeConfigAttr, vkeNodePoolSecurityAttr, vkeNodePoolSecurityGroupIDsAttr)
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

func vkeNodePoolSecurityGroupIDSet(root tftypes.Value) ([]string, tftypes.Value, bool) {
	value, ok := vkeNodePoolObjectAttr(root, vkeNodePoolNodeConfigAttr, vkeNodePoolSecurityAttr, vkeNodePoolSecurityGroupIDsAttr)
	if !ok {
		return nil, tftypes.Value{}, false
	}
	if value.IsNull() || !value.IsKnown() {
		return nil, value, true
	}
	var items []tftypes.Value
	if err := value.As(&items); err != nil {
		return nil, tftypes.Value{}, false
	}
	ids := make([]string, 0, len(items))
	for _, item := range items {
		if item.IsNull() || !item.IsKnown() {
			return nil, value, false
		}
		var id string
		if err := item.As(&id); err != nil {
			return nil, tftypes.Value{}, false
		}
		ids = append(ids, id)
	}
	return ids, value, true
}

func setVKENodePoolNestedValue(root tftypes.Value, leaf tftypes.Value, names ...string) (tftypes.Value, error) {
	if len(names) == 0 {
		return leaf, nil
	}
	var object map[string]tftypes.Value
	if err := root.As(&object); err != nil {
		return root, err
	}
	child, ok := object[names[0]]
	if !ok || child.IsNull() || !child.IsKnown() {
		return root, nil
	}
	updated, err := setVKENodePoolNestedValue(child, leaf, names[1:]...)
	if err != nil {
		return root, err
	}
	object[names[0]] = updated
	return tftypes.NewValue(root.Type(), object), nil
}

func stringSetSubset(subset, universe []string) bool {
	have := make(map[string]struct{}, len(universe))
	for _, id := range universe {
		have[id] = struct{}{}
	}
	for _, id := range subset {
		if _, ok := have[id]; !ok {
			return false
		}
	}
	return true
}

func stringSetEqual(left, right []string) bool {
	return stringSetSubset(left, right) && stringSetSubset(right, left)
}
