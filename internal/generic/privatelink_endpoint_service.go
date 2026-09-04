package generic

import (
	"encoding/json"
	"strings"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

const (
	volcenginePrivateLinkEndpointServiceType = "Volcengine::PrivateLink::EndpointService"

	plEndpointServiceZoneIDsAttr    = "zone_ids"
	plEndpointServiceResourcesAttr  = "resources"
	plEndpointServiceResourceIDAttr = "resource_id"
	plEndpointServiceCCZoneIDs      = "ZoneIds"
	plEndpointServiceCCResources    = "Resources"
	plEndpointServiceCCResourceID   = "ResourceId"
)

// expandPrivateLinkEndpointServiceZoneIDsJSON splits Cloud Control ZoneIds
// readback. Privatelink Describe returns "cn-beijing-a,cn-beijing-c,cn-beijing-d"
// (or a one-element array of that string) while the schema and DPM spec use
// ["cn-beijing-a","cn-beijing-c","cn-beijing-d"]. Observe must emit the array
// or Crossplane treats it as drift and calls UpdateResource.
func expandPrivateLinkEndpointServiceZoneIDsJSON(properties string) (string, bool, error) {
	if strings.TrimSpace(properties) == "" {
		return properties, false, nil
	}
	document := map[string]any{}
	if err := json.Unmarshal([]byte(properties), &document); err != nil {
		return "", false, err
	}
	if !expandPrivateLinkEndpointServiceZoneIDsDocument(document) {
		return properties, false, nil
	}
	out, err := json.Marshal(document)
	if err != nil {
		return "", false, err
	}
	return string(out), true, nil
}

func expandPrivateLinkEndpointServiceZoneIDsDocument(document map[string]any) bool {
	changed := expandPrivateLinkZoneIDsField(document, plEndpointServiceCCZoneIDs)
	resources, ok := document[plEndpointServiceCCResources].([]any)
	if !ok {
		return changed
	}
	for _, item := range resources {
		resource, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if expandPrivateLinkZoneIDsField(resource, plEndpointServiceCCZoneIDs) {
			changed = true
		}
	}
	return changed
}

func expandPrivateLinkZoneIDsField(object map[string]any, field string) bool {
	raw, ok := object[field]
	if !ok || raw == nil {
		return false
	}
	expanded, changed := expandPrivateLinkZoneIDsValue(raw)
	if !changed {
		return false
	}
	object[field] = expanded
	return true
}

func expandPrivateLinkZoneIDsValue(raw any) (any, bool) {
	switch v := raw.(type) {
	case string:
		ids := splitPrivateLinkZoneIDs(v)
		out := make([]any, len(ids))
		for i, id := range ids {
			out[i] = id
		}
		return out, true
	case []any:
		expanded := make([]any, 0, len(v))
		changed := false
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				expanded = append(expanded, item)
				continue
			}
			parts := splitPrivateLinkZoneIDs(s)
			if len(parts) != 1 || parts[0] != s {
				changed = true
			}
			for _, part := range parts {
				expanded = append(expanded, part)
			}
		}
		expanded = dedupePrivateLinkZoneIDs(expanded)
		if !changed && privateLinkZoneIDSlicesEqual(v, expanded) {
			return v, false
		}
		return expanded, true
	default:
		return raw, false
	}
}

func splitPrivateLinkZoneIDs(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	ids := make([]string, 0, len(parts))
	for _, part := range parts {
		id := strings.TrimSpace(part)
		if id == "" {
			continue
		}
		ids = append(ids, id)
	}
	return ids
}

func dedupePrivateLinkZoneIDs(items []any) []any {
	seen := make(map[string]struct{}, len(items))
	out := make([]any, 0, len(items))
	for _, item := range items {
		s, ok := item.(string)
		if !ok {
			out = append(out, item)
			continue
		}
		if _, exists := seen[s]; exists {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func privateLinkZoneIDSlicesEqual(left, right []any) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

// suppressPrivateLinkEndpointServiceZoneIDs treats equivalent AZ sets as no
// change so a comma-joined Cloud Control readback does not call UpdateResource.
func suppressPrivateLinkEndpointServiceZoneIDs(current, planned string) (string, string, bool, error) {
	currentDocument := map[string]any{}
	if err := json.Unmarshal([]byte(current), &currentDocument); err != nil {
		return "", "", false, err
	}
	plannedDocument := map[string]any{}
	if err := json.Unmarshal([]byte(planned), &plannedDocument); err != nil {
		return "", "", false, err
	}

	changed := expandPrivateLinkEndpointServiceZoneIDsDocument(currentDocument)
	if expandPrivateLinkEndpointServiceZoneIDsDocument(plannedDocument) {
		changed = true
	}
	if copyEquivalentPrivateLinkZoneIDsJSON(currentDocument, plannedDocument) {
		changed = true
	}
	if copyEquivalentPrivateLinkResourceZoneIDsJSON(currentDocument, plannedDocument) {
		changed = true
	}
	if !changed {
		return current, planned, false, nil
	}

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

func copyEquivalentPrivateLinkZoneIDsJSON(current, planned map[string]any) bool {
	currentIDs, currentOK := privateLinkZoneIDsFromJSON(current[plEndpointServiceCCZoneIDs])
	plannedIDs, plannedOK := privateLinkZoneIDsFromJSON(planned[plEndpointServiceCCZoneIDs])
	if !currentOK || !plannedOK || !stringSetEqual(currentIDs, plannedIDs) {
		return false
	}
	if privateLinkJSONValuesEqual(current[plEndpointServiceCCZoneIDs], planned[plEndpointServiceCCZoneIDs]) {
		return false
	}
	planned[plEndpointServiceCCZoneIDs] = current[plEndpointServiceCCZoneIDs]
	return true
}

func copyEquivalentPrivateLinkResourceZoneIDsJSON(current, planned map[string]any) bool {
	currentResources, currentOK := current[plEndpointServiceCCResources].([]any)
	plannedResources, plannedOK := planned[plEndpointServiceCCResources].([]any)
	if !currentOK || !plannedOK {
		return false
	}
	currentByID := privateLinkResourcesByID(currentResources)
	changed := false
	for _, item := range plannedResources {
		resource, ok := item.(map[string]any)
		if !ok {
			continue
		}
		id, _ := resource[plEndpointServiceCCResourceID].(string)
		if id == "" {
			continue
		}
		currentResource, ok := currentByID[id]
		if !ok {
			continue
		}
		currentIDs, currentOK := privateLinkZoneIDsFromJSON(currentResource[plEndpointServiceCCZoneIDs])
		plannedIDs, plannedOK := privateLinkZoneIDsFromJSON(resource[plEndpointServiceCCZoneIDs])
		if !currentOK || !plannedOK || !stringSetEqual(currentIDs, plannedIDs) {
			continue
		}
		if privateLinkJSONValuesEqual(currentResource[plEndpointServiceCCZoneIDs], resource[plEndpointServiceCCZoneIDs]) {
			continue
		}
		resource[plEndpointServiceCCZoneIDs] = currentResource[plEndpointServiceCCZoneIDs]
		changed = true
	}
	return changed
}

func privateLinkResourcesByID(resources []any) map[string]map[string]any {
	out := make(map[string]map[string]any, len(resources))
	for _, item := range resources {
		resource, ok := item.(map[string]any)
		if !ok {
			continue
		}
		id, _ := resource[plEndpointServiceCCResourceID].(string)
		if id == "" {
			continue
		}
		out[id] = resource
	}
	return out
}

func privateLinkZoneIDsFromJSON(raw any) ([]string, bool) {
	expanded, _ := expandPrivateLinkZoneIDsValue(raw)
	items, ok := expanded.([]any)
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

func privateLinkJSONValuesEqual(left, right any) bool {
	leftBytes, err := json.Marshal(left)
	if err != nil {
		return false
	}
	rightBytes, err := json.Marshal(right)
	if err != nil {
		return false
	}
	return string(leftBytes) == string(rightBytes)
}

// normalizePrivateLinkEndpointServiceZoneIDsPlan copies prior zone_ids into the
// plan when the only difference is comma-join / order of the same AZ set.
func normalizePrivateLinkEndpointServiceZoneIDsPlan(state, plan tftypes.Value) (tftypes.Value, error) {
	if state.IsNull() || !state.IsKnown() || plan.IsNull() || !plan.IsKnown() {
		return plan, nil
	}
	normalized, err := copyEquivalentPrivateLinkZoneIDsPlan(state, plan)
	if err != nil {
		return plan, err
	}
	return copyEquivalentPrivateLinkResourceZoneIDsPlan(state, normalized)
}

func copyEquivalentPrivateLinkZoneIDsPlan(state, plan tftypes.Value) (tftypes.Value, error) {
	stateIDs, stateValue, ok := privateLinkZoneIDsAttr(state, plEndpointServiceZoneIDsAttr)
	if !ok {
		return plan, nil
	}
	planIDs, planValue, ok := privateLinkZoneIDsAttr(plan, plEndpointServiceZoneIDsAttr)
	if !ok {
		return plan, nil
	}
	if !stringSetEqual(stateIDs, planIDs) || stateValue.Equal(planValue) {
		return plan, nil
	}
	return setPrivateLinkRootAttr(plan, plEndpointServiceZoneIDsAttr, stateValue)
}

func copyEquivalentPrivateLinkResourceZoneIDsPlan(state, plan tftypes.Value) (tftypes.Value, error) {
	stateResources, ok := privateLinkResourcesValue(state)
	if !ok {
		return plan, nil
	}
	planResources, ok := privateLinkResourcesValue(plan)
	if !ok {
		return plan, nil
	}
	stateByID := make(map[string]tftypes.Value, len(stateResources))
	for _, resource := range stateResources {
		id, ok := privateLinkResourceID(resource)
		if !ok {
			continue
		}
		stateByID[id] = resource
	}
	changed := false
	updated := make([]tftypes.Value, 0, len(planResources))
	for _, resource := range planResources {
		id, ok := privateLinkResourceID(resource)
		if !ok {
			updated = append(updated, resource)
			continue
		}
		stateResource, ok := stateByID[id]
		if !ok {
			updated = append(updated, resource)
			continue
		}
		stateIDs, stateValue, ok := privateLinkZoneIDsAttr(stateResource, plEndpointServiceZoneIDsAttr)
		if !ok {
			updated = append(updated, resource)
			continue
		}
		planIDs, planValue, ok := privateLinkZoneIDsAttr(resource, plEndpointServiceZoneIDsAttr)
		if !ok || !stringSetEqual(stateIDs, planIDs) || stateValue.Equal(planValue) {
			updated = append(updated, resource)
			continue
		}
		replaced, err := setPrivateLinkRootAttr(resource, plEndpointServiceZoneIDsAttr, stateValue)
		if err != nil {
			return plan, err
		}
		updated = append(updated, replaced)
		changed = true
	}
	if !changed {
		return plan, nil
	}
	return setPrivateLinkRootAttr(plan, plEndpointServiceResourcesAttr, tftypes.NewValue(plan.Type().(tftypes.Object).AttributeTypes[plEndpointServiceResourcesAttr], updated))
}

func privateLinkResourcesValue(root tftypes.Value) ([]tftypes.Value, bool) {
	value, ok := privateLinkObjectAttr(root, plEndpointServiceResourcesAttr)
	if !ok || value.IsNull() || !value.IsKnown() {
		return nil, false
	}
	var items []tftypes.Value
	if err := value.As(&items); err != nil {
		return nil, false
	}
	return items, true
}

func privateLinkResourceID(resource tftypes.Value) (string, bool) {
	value, ok := privateLinkObjectAttr(resource, plEndpointServiceResourceIDAttr)
	if !ok || value.IsNull() || !value.IsKnown() {
		return "", false
	}
	var id string
	if err := value.As(&id); err != nil || id == "" {
		return "", false
	}
	return id, true
}

func privateLinkZoneIDsAttr(root tftypes.Value, name string) ([]string, tftypes.Value, bool) {
	value, ok := privateLinkObjectAttr(root, name)
	if !ok || value.IsNull() || !value.IsKnown() {
		return nil, tftypes.Value{}, false
	}
	var items []tftypes.Value
	if err := value.As(&items); err != nil {
		return nil, tftypes.Value{}, false
	}
	ids := make([]string, 0, len(items))
	for _, item := range items {
		if item.IsNull() || !item.IsKnown() {
			return nil, tftypes.Value{}, false
		}
		var id string
		if err := item.As(&id); err != nil {
			return nil, tftypes.Value{}, false
		}
		ids = append(ids, splitPrivateLinkZoneIDs(id)...)
	}
	return ids, value, true
}

func privateLinkObjectAttr(root tftypes.Value, name string) (tftypes.Value, bool) {
	if root.IsNull() || !root.IsKnown() {
		return tftypes.Value{}, false
	}
	var object map[string]tftypes.Value
	if err := root.As(&object); err != nil {
		return tftypes.Value{}, false
	}
	value, ok := object[name]
	return value, ok
}

func setPrivateLinkRootAttr(root tftypes.Value, name string, value tftypes.Value) (tftypes.Value, error) {
	var object map[string]tftypes.Value
	if err := root.As(&object); err != nil {
		return root, err
	}
	object[name] = value
	return tftypes.NewValue(root.Type(), object), nil
}
