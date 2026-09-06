package generic

import (
	"encoding/json"
	"math/big"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

const (
	volcengineTOSBucketType   = "Volcengine::TOS::Bucket"
	tosBucketPolicyAttr       = "policy"
	tosBucketLifecycleAttr    = "lifecycle_config"
	tosBucketACLAttr          = "acl"
	tosIAMPrincipalPrefix     = "trn:iam::"
	tosDefaultPolicyVersion   = "1.0"
	tosLegacyPolicyVersion    = "1"
	tosCCPolicyField    = "Policy"
	tosCCLifecycleField = "Lifecycle"
	tosCCACLField       = "ACL"
)

// normalizeTOSBucketPolicyPlan copies prior policy into the plan when the only
// differences are TOS-injected cosmetics: Sid, default Version, and rewriting
// trn:iam::<account>:role/name to <account>:role/name. Ali/Tencent keep bucket
// policy off the Bucket CR (ignore_changes / separate BucketPolicy); VKE embeds
// policy on the bucket, so the provider must ignore those injections or every
// poll sends UpdateResource.
func normalizeTOSBucketPolicyPlan(state, plan tftypes.Value) (tftypes.Value, error) {
	if state.IsNull() || !state.IsKnown() || plan.IsNull() || !plan.IsKnown() {
		return plan, nil
	}
	statePolicy, stateValue, ok := tosBucketStringAttr(state, tosBucketPolicyAttr)
	if !ok || stateValue.IsNull() || !stateValue.IsKnown() {
		return plan, nil
	}
	planPolicy, planValue, ok := tosBucketStringAttr(plan, tosBucketPolicyAttr)
	if !ok || !planValue.IsKnown() || planValue.IsNull() || strings.TrimSpace(planPolicy) == "" {
		return plan, nil
	}
	if statePolicy == planPolicy || !tosPolicyEquivalent(planPolicy, statePolicy) {
		return plan, nil
	}
	return setTOSBucketAttr(plan, tosBucketPolicyAttr, stateValue)
}

// normalizeTOSBucketLifecyclePlan copies prior lifecycle rules into the plan
// when the only difference is an empty Prefix (DPM writes "") versus TOS
// omitting the field.
func normalizeTOSBucketLifecyclePlan(state, plan tftypes.Value) (tftypes.Value, error) {
	if state.IsNull() || !state.IsKnown() || plan.IsNull() || !plan.IsKnown() {
		return plan, nil
	}
	stateValue, ok := tosBucketAttr(state, tosBucketLifecycleAttr)
	if !ok || !stateValue.IsKnown() {
		return plan, nil
	}
	planValue, ok := tosBucketAttr(plan, tosBucketLifecycleAttr)
	if !ok || !planValue.IsKnown() {
		return plan, nil
	}
	if stateValue.Equal(planValue) {
		return plan, nil
	}
	if !tosLifecycleValuesEquivalent(planValue, stateValue) {
		return plan, nil
	}
	return setTOSBucketAttr(plan, tosBucketLifecycleAttr, stateValue)
}

// normalizeTOSBucketACLPlan copies prior ACL into the plan when the only
// difference is TOS/TF Plugin Framework filling computed grant fields
// (canned, display_name) as unknown. DPM does not manage TOS ACL; LateInitialize
// copies the owner FULL_CONTROL grant into spec. A SetNested grant with
// unknown computed children is a different set key than the prior grant, so
// Upjet keeps calling Update. Real grant/permission/owner-id changes stay.
func normalizeTOSBucketACLPlan(state, plan tftypes.Value) (tftypes.Value, error) {
	if state.IsNull() || !state.IsKnown() || plan.IsNull() || !plan.IsKnown() {
		return plan, nil
	}
	stateValue, ok := tosBucketAttr(state, tosBucketACLAttr)
	if !ok || !stateValue.IsKnown() || stateValue.IsNull() {
		return plan, nil
	}
	planValue, ok := tosBucketAttr(plan, tosBucketACLAttr)
	if !ok || !planValue.IsKnown() || planValue.IsNull() {
		return plan, nil
	}
	if stateValue.Equal(planValue) {
		return plan, nil
	}
	if !tosACLValuesEquivalent(planValue, stateValue) {
		return plan, nil
	}
	return setTOSBucketAttr(plan, tosBucketACLAttr, stateValue)
}

// suppressTOSBucketInjectedPolicy keeps TOS-injected policy cosmetics out of
// the Cloud Control JSON Patch. Real statement changes still patch.
func suppressTOSBucketInjectedPolicy(current, planned string) (string, string, bool, error) {
	currentDocument := map[string]any{}
	if err := json.Unmarshal([]byte(current), &currentDocument); err != nil {
		return "", "", false, err
	}
	plannedDocument := map[string]any{}
	if err := json.Unmarshal([]byte(planned), &plannedDocument); err != nil {
		return "", "", false, err
	}
	currentPolicy, ok := tosJSONString(currentDocument[tosCCPolicyField])
	if !ok {
		return current, planned, false, nil
	}
	plannedPolicy, ok := tosJSONString(plannedDocument[tosCCPolicyField])
	if !ok || strings.TrimSpace(plannedPolicy) == "" {
		return current, planned, false, nil
	}
	if currentPolicy == plannedPolicy || !tosPolicyEquivalent(plannedPolicy, currentPolicy) {
		return current, planned, false, nil
	}
	plannedDocument[tosCCPolicyField] = currentDocument[tosCCPolicyField]
	plannedBytes, err := json.Marshal(plannedDocument)
	if err != nil {
		return "", "", false, err
	}
	return current, string(plannedBytes), true, nil
}

// suppressTOSBucketEmptyLifecyclePrefix keeps empty Prefix out of the patch
// when TOS omitted the field.
func suppressTOSBucketEmptyLifecyclePrefix(current, planned string) (string, string, bool, error) {
	currentDocument := map[string]any{}
	if err := json.Unmarshal([]byte(current), &currentDocument); err != nil {
		return "", "", false, err
	}
	plannedDocument := map[string]any{}
	if err := json.Unmarshal([]byte(planned), &plannedDocument); err != nil {
		return "", "", false, err
	}
	if !tosLifecycleJSONEquivalent(plannedDocument[tosCCLifecycleField], currentDocument[tosCCLifecycleField]) {
		return current, planned, false, nil
	}
	if tosRawJSONEqual(plannedDocument[tosCCLifecycleField], currentDocument[tosCCLifecycleField]) {
		return current, planned, false, nil
	}
	plannedDocument[tosCCLifecycleField] = currentDocument[tosCCLifecycleField]
	plannedBytes, err := json.Marshal(plannedDocument)
	if err != nil {
		return "", "", false, err
	}
	return current, string(plannedBytes), true, nil
}

// suppressTOSBucketInjectedACL keeps computed grant cosmetics (Canned,
// DisplayName) out of the Cloud Control JSON Patch.
func suppressTOSBucketInjectedACL(current, planned string) (string, string, bool, error) {
	currentDocument := map[string]any{}
	if err := json.Unmarshal([]byte(current), &currentDocument); err != nil {
		return "", "", false, err
	}
	plannedDocument := map[string]any{}
	if err := json.Unmarshal([]byte(planned), &plannedDocument); err != nil {
		return "", "", false, err
	}
	if !tosACLJSONEquivalent(plannedDocument[tosCCACLField], currentDocument[tosCCACLField]) {
		return current, planned, false, nil
	}
	if tosRawJSONEqual(plannedDocument[tosCCACLField], currentDocument[tosCCACLField]) {
		return current, planned, false, nil
	}
	plannedDocument[tosCCACLField] = currentDocument[tosCCACLField]
	plannedBytes, err := json.Marshal(plannedDocument)
	if err != nil {
		return "", "", false, err
	}
	return current, string(plannedBytes), true, nil
}

func tosPolicyEquivalent(planned, current string) bool {
	left, ok := tosCanonicalPolicy(planned)
	if !ok {
		return false
	}
	right, ok := tosCanonicalPolicy(current)
	if !ok {
		return false
	}
	return left == right
}

func tosCanonicalPolicy(raw string) (string, bool) {
	var document map[string]any
	if err := json.Unmarshal([]byte(raw), &document); err != nil {
		return "", false
	}
	if version, ok := document["Version"].(string); ok && (version == tosDefaultPolicyVersion || version == tosLegacyPolicyVersion) {
		delete(document, "Version")
	}
	statements, ok := document["Statement"].([]any)
	if !ok {
		return "", false
	}
	canonical := make([]string, 0, len(statements))
	for _, item := range statements {
		statement, ok := item.(map[string]any)
		if !ok {
			return "", false
		}
		delete(statement, "Sid")
		if principals, exists := statement["Principal"]; exists {
			statement["Principal"] = tosNormalizePrincipals(principals)
		}
		if actions, exists := statement["Action"]; exists {
			statement["Action"] = tosSortedStringValues(actions)
		}
		if resources, exists := statement["Resource"]; exists {
			statement["Resource"] = tosSortedStringValues(resources)
		}
		encoded, err := json.Marshal(statement)
		if err != nil {
			return "", false
		}
		canonical = append(canonical, string(encoded))
	}
	sort.Strings(canonical)
	document["Statement"] = json.RawMessage("[" + strings.Join(canonical, ",") + "]")
	encoded, err := json.Marshal(document)
	if err != nil {
		return "", false
	}
	return string(encoded), true
}

func tosNormalizePrincipals(value any) any {
	switch typed := value.(type) {
	case string:
		return strings.TrimPrefix(typed, tosIAMPrincipalPrefix)
	case []any:
		normalized := make([]string, 0, len(typed))
		for _, item := range typed {
			text, ok := item.(string)
			if !ok {
				return value
			}
			normalized = append(normalized, strings.TrimPrefix(text, tosIAMPrincipalPrefix))
		}
		sort.Strings(normalized)
		copied := make([]any, len(normalized))
		for i, item := range normalized {
			copied[i] = item
		}
		return copied
	default:
		return value
	}
}

func tosSortedStringValues(value any) any {
	switch typed := value.(type) {
	case string:
		return typed
	case []any:
		items := make([]string, 0, len(typed))
		for _, item := range typed {
			text, ok := item.(string)
			if !ok {
				return value
			}
			items = append(items, text)
		}
		sort.Strings(items)
		copied := make([]any, len(items))
		for i, item := range items {
			copied[i] = item
		}
		return copied
	default:
		return value
	}
}

func tosLifecycleValuesEquivalent(plan, state tftypes.Value) bool {
	planRules, ok := tosComparableKnownValue(plan)
	if !ok {
		return false
	}
	stateRules, ok := tosComparableKnownValue(state)
	if !ok {
		return false
	}
	return tosRawJSONEqual(tosOmitEmpty(planRules), tosOmitEmpty(stateRules))
}

func tosLifecycleJSONEquivalent(plan, state any) bool {
	return tosRawJSONEqual(tosOmitEmpty(plan), tosOmitEmpty(state))
}

func tosACLValuesEquivalent(plan, state tftypes.Value) bool {
	planGrants, planOwner, planDelivered, ok := tosACLIdentity(plan)
	if !ok {
		return false
	}
	stateGrants, stateOwner, stateDelivered, ok := tosACLIdentity(state)
	if !ok {
		return false
	}
	if !stringSetEqual(planGrants, stateGrants) {
		return false
	}
	if planOwner != "" && stateOwner != "" && planOwner != stateOwner {
		return false
	}
	if planDelivered != "" && stateDelivered != "" && planDelivered != stateDelivered {
		return false
	}
	return true
}

func tosACLJSONEquivalent(plan, state any) bool {
	planGrants, planOwner, planDelivered, ok := tosACLJSONIdentity(plan)
	if !ok {
		return false
	}
	stateGrants, stateOwner, stateDelivered, ok := tosACLJSONIdentity(state)
	if !ok {
		return false
	}
	if !stringSetEqual(planGrants, stateGrants) {
		return false
	}
	if planOwner != "" && stateOwner != "" && planOwner != stateOwner {
		return false
	}
	if planDelivered != "" && stateDelivered != "" && planDelivered != stateDelivered {
		return false
	}
	return true
}

func tosACLIdentity(acl tftypes.Value) ([]string, string, string, bool) {
	if acl.IsNull() || !acl.IsKnown() {
		return nil, "", "", false
	}
	var object map[string]tftypes.Value
	if err := acl.As(&object); err != nil {
		return nil, "", "", false
	}
	grants, ok := tosACLGrantIdentities(object["grants"])
	if !ok {
		return nil, "", "", false
	}
	return grants, tosKnownString(object["owner"], "owner_id"), tosKnownBoolString(object["bucket_acl_delivered"]), true
}

func tosACLGrantIdentities(grants tftypes.Value) ([]string, bool) {
	if grants.IsNull() {
		return nil, true
	}
	if !grants.IsKnown() {
		return nil, false
	}
	var items []tftypes.Value
	if err := grants.As(&items); err != nil {
		return nil, false
	}
	keys := make([]string, 0, len(items))
	for _, item := range items {
		key, ok := tosACLGrantIdentity(item)
		if !ok {
			return nil, false
		}
		keys = append(keys, key)
	}
	return keys, true
}

func tosACLGrantIdentity(grant tftypes.Value) (string, bool) {
	if grant.IsNull() || !grant.IsKnown() {
		return "", false
	}
	var object map[string]tftypes.Value
	if err := grant.As(&object); err != nil {
		return "", false
	}
	permission := tosKnownStringValue(object["permission"])
	if permission == "" {
		return "", false
	}
	var grantee map[string]tftypes.Value
	if value, ok := object["grantee"]; ok && !value.IsNull() && value.IsKnown() {
		if err := value.As(&grantee); err != nil {
			return "", false
		}
	}
	typ := tosKnownStringValue(grantee["type"])
	id := tosKnownStringValue(grantee["grantee_id"])
	canned := tosKnownStringValue(grantee["canned"])
	if id == "" {
		id = canned
	}
	if typ == "" || id == "" {
		return "", false
	}
	return typ + "|" + id + "|" + permission, true
}

func tosKnownString(object tftypes.Value, name string) string {
	if object.IsNull() || !object.IsKnown() {
		return ""
	}
	var nested map[string]tftypes.Value
	if err := object.As(&nested); err != nil {
		return ""
	}
	return tosKnownStringValue(nested[name])
}

func tosKnownStringValue(value tftypes.Value) string {
	if value.IsNull() || !value.IsKnown() {
		return ""
	}
	var text string
	if err := value.As(&text); err != nil {
		return ""
	}
	return text
}

func tosKnownBoolString(value tftypes.Value) string {
	if value.IsNull() || !value.IsKnown() {
		return ""
	}
	var flag bool
	if err := value.As(&flag); err != nil {
		return ""
	}
	if flag {
		return "true"
	}
	return "false"
}

func tosACLJSONIdentity(value any) ([]string, string, string, bool) {
	object, ok := value.(map[string]any)
	if !ok {
		return nil, "", "", value == nil
	}
	grants, ok := tosACLJSONGrantIdentities(tosJSONField(object, "Grants", "grants"))
	if !ok {
		return nil, "", "", false
	}
	ownerID := ""
	if owner, ok := tosJSONField(object, "Owner", "owner").(map[string]any); ok {
		ownerID = tosJSONText(tosJSONField(owner, "OwnerId", "owner_id"))
	}
	return grants, ownerID, tosJSONBoolString(tosJSONField(object, "BucketACLDelivered", "bucket_acl_delivered")), true
}

func tosACLJSONGrantIdentities(value any) ([]string, bool) {
	if value == nil {
		return nil, true
	}
	items, ok := value.([]any)
	if !ok {
		return nil, false
	}
	keys := make([]string, 0, len(items))
	for _, item := range items {
		grant, ok := item.(map[string]any)
		if !ok {
			return nil, false
		}
		permission := tosJSONText(tosJSONField(grant, "Permission", "permission"))
		grantee, _ := tosJSONField(grant, "Grantee", "grantee").(map[string]any)
		typ := tosJSONText(tosJSONField(grantee, "Type", "type"))
		id := tosJSONText(tosJSONField(grantee, "GranteeId", "grantee_id"))
		if id == "" {
			id = tosJSONText(tosJSONField(grantee, "Canned", "canned"))
		}
		if permission == "" || typ == "" || id == "" {
			return nil, false
		}
		keys = append(keys, typ+"|"+id+"|"+permission)
	}
	return keys, true
}

func tosJSONField(object map[string]any, names ...string) any {
	if object == nil {
		return nil
	}
	for _, name := range names {
		if value, ok := object[name]; ok {
			return value
		}
	}
	return nil
}

func tosJSONText(value any) string {
	text, _ := value.(string)
	return text
}

func tosJSONBoolString(value any) string {
	switch typed := value.(type) {
	case bool:
		if typed {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

func tosComparableValue(value tftypes.Value) (any, bool) {
	return tosComparableKnownValue(value)
}

// tosComparableKnownValue walks a Terraform value but treats unknown
// computed fields as absent. TOS lifecycle/ACL SetNested objects fill
// unused children as unknown after PlanResourceChange; those must not
// fail semantic comparison or every poll keeps a filtered plan diff.
func tosComparableKnownValue(value tftypes.Value) (any, bool) {
	if value.IsNull() {
		return nil, true
	}
	if !value.IsKnown() {
		return nil, true
	}
	switch {
	case value.Type().Is(tftypes.String):
		var text string
		if err := value.As(&text); err != nil {
			return nil, false
		}
		if text == "" {
			return nil, true
		}
		return text, true
	case value.Type().Is(tftypes.Bool):
		var flag bool
		if err := value.As(&flag); err != nil {
			return nil, false
		}
		return flag, true
	case value.Type().Is(tftypes.Number):
		number := new(big.Float)
		if err := value.As(&number); err != nil {
			return nil, false
		}
		converted, _ := number.Float64()
		return converted, true
	case value.Type().Is(tftypes.Object{}):
		var object map[string]tftypes.Value
		if err := value.As(&object); err != nil {
			return nil, false
		}
		out := map[string]any{}
		for key, child := range object {
			converted, ok := tosComparableValue(child)
			if !ok {
				return nil, false
			}
			if converted != nil {
				out[key] = converted
			}
		}
		if len(out) == 0 {
			return nil, true
		}
		return out, true
	case value.Type().Is(tftypes.Set{}), value.Type().Is(tftypes.List{}), value.Type().Is(tftypes.Tuple{}):
		var items []tftypes.Value
		if err := value.As(&items); err != nil {
			return nil, false
		}
		out := make([]any, 0, len(items))
		for _, item := range items {
			if !item.IsKnown() {
				return nil, false
			}
			converted, ok := tosComparableKnownValue(item)
			if !ok {
				return nil, false
			}
			if converted != nil {
				out = append(out, converted)
			}
		}
		encoded := make([]string, 0, len(out))
		for _, item := range out {
			raw, err := json.Marshal(item)
			if err != nil {
				return nil, false
			}
			encoded = append(encoded, string(raw))
		}
		sort.Strings(encoded)
		sorted := make([]any, 0, len(encoded))
		for _, raw := range encoded {
			var item any
			if err := json.Unmarshal([]byte(raw), &item); err != nil {
				return nil, false
			}
			sorted = append(sorted, item)
		}
		return sorted, true
	default:
		return nil, false
	}
}

func tosOmitEmpty(value any) any {
	switch typed := value.(type) {
	case string:
		if typed == "" {
			return nil
		}
		return typed
	case map[string]any:
		out := map[string]any{}
		for key, child := range typed {
			converted := tosOmitEmpty(child)
			if converted != nil {
				out[key] = converted
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	case []any:
		out := make([]any, 0, len(typed))
		for _, child := range typed {
			converted := tosOmitEmpty(child)
			if converted != nil {
				out = append(out, converted)
			}
		}
		if len(out) == 0 {
			return nil
		}
		encoded := make([]string, 0, len(out))
		for _, item := range out {
			raw, err := json.Marshal(item)
			if err != nil {
				return value
			}
			encoded = append(encoded, string(raw))
		}
		sort.Strings(encoded)
		sorted := make([]any, 0, len(encoded))
		for _, raw := range encoded {
			var item any
			if err := json.Unmarshal([]byte(raw), &item); err != nil {
				return value
			}
			sorted = append(sorted, item)
		}
		return sorted
	default:
		return value
	}
}

func tosRawJSONEqual(left, right any) bool {
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

func tosJSONString(value any) (string, bool) {
	text, ok := value.(string)
	return text, ok
}

func tosBucketAttr(root tftypes.Value, name string) (tftypes.Value, bool) {
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

func tosBucketStringAttr(root tftypes.Value, name string) (string, tftypes.Value, bool) {
	value, ok := tosBucketAttr(root, name)
	if !ok {
		return "", tftypes.Value{}, false
	}
	if value.IsNull() || !value.IsKnown() {
		return "", value, true
	}
	var text string
	if err := value.As(&text); err != nil {
		return "", tftypes.Value{}, false
	}
	return text, value, true
}

func setTOSBucketAttr(root tftypes.Value, name string, value tftypes.Value) (tftypes.Value, error) {
	var object map[string]tftypes.Value
	if err := root.As(&object); err != nil {
		return root, err
	}
	object[name] = value
	return tftypes.NewValue(root.Type(), object), nil
}
