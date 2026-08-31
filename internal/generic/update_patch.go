package generic

import "encoding/json"

const volcengineVKENodePoolType = "Volcengine::VKE::NodePool"

// suppressVKENodePoolDesiredReplicas removes the autoscaler-owned desired
// replica count from NodePool updates. The create path still sends it.
func suppressVKENodePoolDesiredReplicas(current, planned string) (string, string, bool, error) {
	currentDocument := map[string]any{}
	if err := json.Unmarshal([]byte(current), &currentDocument); err != nil {
		return "", "", false, err
	}
	plannedDocument := map[string]any{}
	if err := json.Unmarshal([]byte(planned), &plannedDocument); err != nil {
		return "", "", false, err
	}

	if !vkeNodePoolAutoscalingEnabled(currentDocument) && !vkeNodePoolAutoscalingEnabled(plannedDocument) {
		return current, planned, false, nil
	}

	deleteVKENodePoolDesiredReplicas(currentDocument)
	deleteVKENodePoolDesiredReplicas(plannedDocument)

	currentBytes, err := json.Marshal(currentDocument)
	if err != nil {
		return "", "", false, err
	}
	plannedBytes, err := json.Marshal(plannedDocument)
	if err != nil {
		return "", "", false, err
	}
	return string(currentBytes), string(plannedBytes), true, nil
}

// suppressVKENodePoolInjectedSecurityGroups keeps extra remote security groups
// when the planned set is a subset of current. VKE injects a cluster default
// SG that DPM does not write.
func suppressVKENodePoolInjectedSecurityGroups(current, planned string) (string, string, bool, error) {
	currentDocument := map[string]any{}
	if err := json.Unmarshal([]byte(current), &currentDocument); err != nil {
		return "", "", false, err
	}
	plannedDocument := map[string]any{}
	if err := json.Unmarshal([]byte(planned), &plannedDocument); err != nil {
		return "", "", false, err
	}

	currentIDs, ok := vkeNodePoolSecurityGroupIDsJSON(currentDocument)
	if !ok {
		return current, planned, false, nil
	}
	plannedIDs, ok := vkeNodePoolSecurityGroupIDsJSON(plannedDocument)
	if !ok {
		return current, planned, false, nil
	}
	if !stringSetSubset(plannedIDs, currentIDs) || stringSetEqual(plannedIDs, currentIDs) {
		return current, planned, false, nil
	}
	if !setVKENodePoolSecurityGroupIDsJSON(plannedDocument, currentIDs) {
		return current, planned, false, nil
	}

	plannedBytes, err := json.Marshal(plannedDocument)
	if err != nil {
		return "", "", false, err
	}
	return current, string(plannedBytes), true, nil
}

func vkeNodePoolSecurityGroupIDsJSON(document map[string]any) ([]string, bool) {
	nodeConfig, ok := document["NodeConfig"].(map[string]any)
	if !ok {
		return nil, false
	}
	security, ok := nodeConfig["Security"].(map[string]any)
	if !ok {
		return nil, false
	}
	raw, ok := security["SecurityGroupIds"]
	if !ok || raw == nil {
		return nil, false
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, false
	}
	ids := make([]string, 0, len(items))
	for _, item := range items {
		id, ok := item.(string)
		if !ok {
			return nil, false
		}
		ids = append(ids, id)
	}
	return ids, true
}

func setVKENodePoolSecurityGroupIDsJSON(document map[string]any, ids []string) bool {
	nodeConfig, ok := document["NodeConfig"].(map[string]any)
	if !ok {
		return false
	}
	security, ok := nodeConfig["Security"].(map[string]any)
	if !ok {
		return false
	}
	copied := make([]any, len(ids))
	for i, id := range ids {
		copied[i] = id
	}
	security["SecurityGroupIds"] = copied
	return true
}

func vkeNodePoolAutoscalingEnabled(document map[string]any) bool {
	autoScaling, ok := document["AutoScaling"].(map[string]any)
	if !ok {
		return false
	}
	enabled, ok := autoScaling["Enabled"].(bool)
	return ok && enabled
}

func deleteVKENodePoolDesiredReplicas(document map[string]any) {
	autoScaling, ok := document["AutoScaling"].(map[string]any)
	if ok {
		delete(autoScaling, "DesiredReplicas")
	}
}
