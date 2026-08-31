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
