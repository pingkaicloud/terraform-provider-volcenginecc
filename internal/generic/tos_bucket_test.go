package generic

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

const (
	tosTestPlannedPolicy = `{"Version":"1.0","Statement":[{"Effect":"Allow","Principal":["trn:iam::2121942738:role/dev-cn-beijing-ng-dfs-workload-cluster"],"Action":["tos:AbortMultipartUpload","tos:DeleteObject","tos:GetObject","tos:HeadBucket","tos:ListBucket","tos:ListObjects","tos:PutObject"],"Resource":["trn:tos:::bucket","trn:tos:::bucket/*"]}]}`
	tosTestCurrentPolicy = `{"Version":"1.0","Statement":[{"Sid":"unnamed-1788184247581693564","Effect":"Allow","Principal":["2121942738:role/dev-cn-beijing-ng-dfs-workload-cluster"],"Action":["tos:AbortMultipartUpload","tos:DeleteObject","tos:GetObject","tos:HeadBucket","tos:ListBucket","tos:ListObjects","tos:PutObject"],"Resource":["trn:tos:::bucket","trn:tos:::bucket/*"]}]}`
)

func TestTOSPolicyEquivalentLiveShape(t *testing.T) {
	t.Parallel()
	if !tosPolicyEquivalent(tosTestPlannedPolicy, tosTestCurrentPolicy) {
		t.Fatal("live TOS Sid + principal rewrite should be equivalent")
	}
	changed := `{"Version":"1.0","Statement":[{"Effect":"Allow","Principal":["trn:iam::2121942738:role/other"],"Action":["tos:GetObject"],"Resource":["trn:tos:::bucket/*"]}]}`
	if tosPolicyEquivalent(changed, tosTestCurrentPolicy) {
		t.Fatal("a real principal/action change must not be equivalent")
	}
}

func TestNormalizeTOSBucketPolicyPlan(t *testing.T) {
	t.Parallel()

	resourceType := tosBucketTestType()
	prior := tosBucketTestValue(resourceType, tosTestCurrentPolicy, lifecycleRule{id: "abort", prefix: nil, status: "Enabled"})
	planned := tosBucketTestValue(resourceType, tosTestPlannedPolicy, lifecycleRule{id: "abort", prefix: nil, status: "Enabled"})

	before := filteredPlanDiffPaths(t, planned, prior)
	if len(before) != 1 || !strings.Contains(before[0], tosBucketPolicyAttr) {
		t.Fatalf("pre-normalize filtered diff paths = %v, want only %s", before, tosBucketPolicyAttr)
	}

	normalized, err := normalizeTOSBucketPolicyPlan(prior, planned)
	if err != nil {
		t.Fatalf("normalizeTOSBucketPolicyPlan() error = %v", err)
	}
	after := filteredPlanDiffPaths(t, normalized, prior)
	if len(after) != 0 {
		t.Fatalf("post-normalize filtered diff paths = %v, want none", after)
	}

	got, _, ok := tosBucketStringAttr(normalized, tosBucketPolicyAttr)
	if !ok || got != tosTestCurrentPolicy {
		t.Fatalf("normalized policy = %q, want prior policy", got)
	}
}

func TestNormalizeTOSBucketPolicyPlanKeepsRealChange(t *testing.T) {
	t.Parallel()

	resourceType := tosBucketTestType()
	changed := `{"Version":"1.0","Statement":[{"Effect":"Deny","Principal":["trn:iam::2121942738:role/dev-cn-beijing-ng-dfs-workload-cluster"],"Action":["tos:GetObject"],"Resource":["trn:tos:::bucket/*"]}]}`
	prior := tosBucketTestValue(resourceType, tosTestCurrentPolicy)
	planned := tosBucketTestValue(resourceType, changed)

	normalized, err := normalizeTOSBucketPolicyPlan(prior, planned)
	if err != nil {
		t.Fatalf("normalizeTOSBucketPolicyPlan() error = %v", err)
	}
	if !normalized.Equal(planned) {
		t.Fatal("real policy change must keep the planned value")
	}
}

func TestNormalizeTOSBucketLifecyclePlan(t *testing.T) {
	t.Parallel()

	resourceType := tosBucketTestType()
	empty := ""
	prior := tosBucketTestValue(resourceType, tosTestCurrentPolicy, lifecycleRule{id: "abort", prefix: nil, status: "Enabled"})
	planned := tosBucketTestValue(resourceType, tosTestCurrentPolicy, lifecycleRule{id: "abort", prefix: &empty, status: "Enabled"})

	before := filteredPlanDiffPaths(t, planned, prior)
	if len(before) == 0 {
		t.Fatal("pre-normalize filtered diff paths = [], want a lifecycle prefix drift")
	}

	normalized, err := normalizeTOSBucketLifecyclePlan(prior, planned)
	if err != nil {
		t.Fatalf("normalizeTOSBucketLifecyclePlan() error = %v", err)
	}
	after := filteredPlanDiffPaths(t, normalized, prior)
	if len(after) != 0 {
		t.Fatalf("post-normalize filtered diff paths = %v, want none", after)
	}
}

func TestTOSLifecycleEquivalentJSONNumbers(t *testing.T) {
	t.Parallel()

	resourceType := tftypes.Set{ElementType: tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"lifecycle_rule_id": tftypes.String,
		"status":            tftypes.String,
		"expiration": tftypes.Object{AttributeTypes: map[string]tftypes.Type{
			"days": tftypes.Number,
		}},
	}}}
	raw := []byte(`[{"lifecycle_rule_id":"expire","status":"Enabled","expiration":{"days":7}}]`)
	fromJSON, err := tftypes.ValueFromJSONWithOpts(raw, resourceType, tftypes.ValueFromJSONOpts{IgnoreUndefinedAttributes: true})
	if err != nil {
		t.Fatalf("ValueFromJSONWithOpts() error = %v", err)
	}
	fromNative := tftypes.NewValue(resourceType, []tftypes.Value{
		tftypes.NewValue(resourceType.ElementType, map[string]tftypes.Value{
			"lifecycle_rule_id": tftypes.NewValue(tftypes.String, "expire"),
			"status":            tftypes.NewValue(tftypes.String, "Enabled"),
			"expiration": tftypes.NewValue(resourceType.ElementType.(tftypes.Object).AttributeTypes["expiration"], map[string]tftypes.Value{
				"days": tftypes.NewValue(tftypes.Number, 7),
			}),
		}),
	})
	if !tosLifecycleValuesEquivalent(fromJSON, fromNative) {
		t.Fatal("JSON-decoded lifecycle days must compare equal to native numbers")
	}
}

func TestNormalizeTOSBucketLifecyclePlanIgnoresUnknownComputed(t *testing.T) {
	t.Parallel()

	resourceType := tosBucketLifecycleComputedTestType()
	prior := tosBucketLifecycleComputedValue(resourceType, "abort", nil, "Enabled", false)
	empty := ""
	planned := tosBucketLifecycleComputedValue(resourceType, "abort", &empty, "Enabled", true)

	normalized, err := normalizeTOSBucketLifecyclePlan(prior, planned)
	if err != nil {
		t.Fatalf("normalizeTOSBucketLifecyclePlan() error = %v", err)
	}
	after := filteredPlanDiffPaths(t, normalized, prior)
	if len(after) != 0 {
		t.Fatalf("post-normalize filtered diff paths = %v, want none", after)
	}
}

func TestNormalizeTOSBucketLifecyclePlanKeepsRealChange(t *testing.T) {
	t.Parallel()

	resourceType := tosBucketTestType()
	prior := tosBucketTestValue(resourceType, tosTestCurrentPolicy, lifecycleRule{id: "abort", prefix: nil, status: "Enabled"})
	planned := tosBucketTestValue(resourceType, tosTestCurrentPolicy, lifecycleRule{id: "abort", prefix: nil, status: "Disabled"})

	normalized, err := normalizeTOSBucketLifecyclePlan(prior, planned)
	if err != nil {
		t.Fatalf("normalizeTOSBucketLifecyclePlan() error = %v", err)
	}
	if !normalized.Equal(planned) {
		t.Fatal("a real lifecycle status change must keep the planned value")
	}
}

func TestSuppressTOSBucketInjectedPolicyEmptyPatch(t *testing.T) {
	t.Parallel()

	current := `{"Policy":` + jsonQuote(tosTestCurrentPolicy) + `}`
	planned := `{"Policy":` + jsonQuote(tosTestPlannedPolicy) + `}`
	gotCurrent, gotPlanned, changed, err := suppressTOSBucketInjectedPolicy(current, planned)
	if err != nil {
		t.Fatalf("suppressTOSBucketInjectedPolicy() error = %v", err)
	}
	if !changed {
		t.Fatal("expected policy cosmetics to be suppressed")
	}
	patch, err := patchDocument(gotCurrent, gotPlanned)
	if err != nil {
		t.Fatalf("patchDocument() error = %v", err)
	}
	if patch != "[]" {
		t.Fatalf("patchDocument() = %s, want []", patch)
	}
}

func TestNormalizeTOSBucketACLPlan(t *testing.T) {
	t.Parallel()

	resourceType := tosBucketACLTestType()
	prior := tosBucketACLTestValue(resourceType, "2121942738", "CanonicalUser", "FULL_CONTROL", false)
	planned := tosBucketACLTestValue(resourceType, "2121942738", "CanonicalUser", "FULL_CONTROL", true)

	before := filteredPlanDiffPaths(t, planned, prior)
	if len(before) == 0 {
		t.Fatal("pre-normalize filtered diff paths = [], want ACL grant set drift")
	}

	normalized, err := normalizeTOSBucketACLPlan(prior, planned)
	if err != nil {
		t.Fatalf("normalizeTOSBucketACLPlan() error = %v", err)
	}
	after := filteredPlanDiffPaths(t, normalized, prior)
	if len(after) != 0 {
		t.Fatalf("post-normalize filtered diff paths = %v, want none", after)
	}
}

func TestNormalizeTOSBucketACLPlanKeepsRealChange(t *testing.T) {
	t.Parallel()

	resourceType := tosBucketACLTestType()
	prior := tosBucketACLTestValue(resourceType, "2121942738", "CanonicalUser", "FULL_CONTROL", false)
	planned := tosBucketACLTestValue(resourceType, "2121942738", "CanonicalUser", "READ", false)

	normalized, err := normalizeTOSBucketACLPlan(prior, planned)
	if err != nil {
		t.Fatalf("normalizeTOSBucketACLPlan() error = %v", err)
	}
	if !normalized.Equal(planned) {
		t.Fatal("a real ACL permission change must keep the planned value")
	}
}

func TestSuppressTOSBucketInjectedACLEmptyPatch(t *testing.T) {
	t.Parallel()

	current := `{"ACL":{"Grants":[{"Grantee":{"GranteeId":"2121942738","Type":"CanonicalUser","DisplayName":"owner"},"Permission":"FULL_CONTROL"}],"Owner":{"OwnerId":"2121942738"}}}`
	planned := `{"ACL":{"Grants":[{"Grantee":{"GranteeId":"2121942738","Type":"CanonicalUser"},"Permission":"FULL_CONTROL"}],"Owner":{"OwnerId":"2121942738"}}}`
	gotCurrent, gotPlanned, changed, err := suppressTOSBucketInjectedACL(current, planned)
	if err != nil {
		t.Fatalf("suppressTOSBucketInjectedACL() error = %v", err)
	}
	if !changed {
		t.Fatal("expected ACL cosmetics to be suppressed")
	}
	patch, err := patchDocument(gotCurrent, gotPlanned)
	if err != nil {
		t.Fatalf("patchDocument() error = %v", err)
	}
	if patch != "[]" {
		t.Fatalf("patchDocument() = %s, want []", patch)
	}
}

func TestSuppressTOSBucketEmptyLifecyclePrefixEmptyPatch(t *testing.T) {
	t.Parallel()

	current := `{"Lifecycle":[{"LifecycleRuleId":"abort","Status":"Enabled"}]}`
	planned := `{"Lifecycle":[{"LifecycleRuleId":"abort","Status":"Enabled","Prefix":""}]}`
	gotCurrent, gotPlanned, changed, err := suppressTOSBucketEmptyLifecyclePrefix(current, planned)
	if err != nil {
		t.Fatalf("suppressTOSBucketEmptyLifecyclePrefix() error = %v", err)
	}
	if !changed {
		t.Fatal("expected empty Prefix to be suppressed")
	}
	patch, err := patchDocument(gotCurrent, gotPlanned)
	if err != nil {
		t.Fatalf("patchDocument() error = %v", err)
	}
	if patch != "[]" {
		t.Fatalf("patchDocument() = %s, want []", patch)
	}
}

type lifecycleRule struct {
	id     string
	prefix *string
	status string
}

func tosBucketTestType() tftypes.Object {
	return tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		tosBucketPolicyAttr: tftypes.String,
		tosBucketLifecycleAttr: tftypes.Set{ElementType: tftypes.Object{AttributeTypes: map[string]tftypes.Type{
			"lifecycle_rule_id": tftypes.String,
			"prefix":            tftypes.String,
			"status":            tftypes.String,
		}}},
	}}
}

func tosBucketTestValue(resourceType tftypes.Object, policy string, rules ...lifecycleRule) tftypes.Value {
	items := make([]tftypes.Value, 0, len(rules))
	ruleType := resourceType.AttributeTypes[tosBucketLifecycleAttr].(tftypes.Set).ElementType
	for _, rule := range rules {
		prefix := tftypes.NewValue(tftypes.String, nil)
		if rule.prefix != nil {
			prefix = tftypes.NewValue(tftypes.String, *rule.prefix)
		}
		items = append(items, tftypes.NewValue(ruleType, map[string]tftypes.Value{
			"lifecycle_rule_id": tftypes.NewValue(tftypes.String, rule.id),
			"prefix":            prefix,
			"status":            tftypes.NewValue(tftypes.String, rule.status),
		}))
	}
	lifecycle := tftypes.NewValue(resourceType.AttributeTypes[tosBucketLifecycleAttr], nil)
	if len(items) > 0 {
		lifecycle = tftypes.NewValue(resourceType.AttributeTypes[tosBucketLifecycleAttr], items)
	}
	return tftypes.NewValue(resourceType, map[string]tftypes.Value{
		tosBucketPolicyAttr:    tftypes.NewValue(tftypes.String, policy),
		tosBucketLifecycleAttr: lifecycle,
	})
}

func tosBucketLifecycleComputedTestType() tftypes.Object {
	return tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		tosBucketLifecycleAttr: tftypes.Set{ElementType: tftypes.Object{AttributeTypes: map[string]tftypes.Type{
			"lifecycle_rule_id": tftypes.String,
			"prefix":            tftypes.String,
			"status":            tftypes.String,
			"expiration": tftypes.Object{AttributeTypes: map[string]tftypes.Type{
				"date": tftypes.String,
				"days": tftypes.Number,
			}},
			"filter": tftypes.Object{AttributeTypes: map[string]tftypes.Type{
				"object_size_greater_than": tftypes.Number,
			}},
		}}},
	}}
}

func tosBucketLifecycleComputedValue(resourceType tftypes.Object, id string, prefix *string, status string, unknownComputed bool) tftypes.Value {
	ruleType := resourceType.AttributeTypes[tosBucketLifecycleAttr].(tftypes.Set).ElementType.(tftypes.Object)
	expirationType := ruleType.AttributeTypes["expiration"].(tftypes.Object)
	filterType := ruleType.AttributeTypes["filter"].(tftypes.Object)
	prefixValue := tftypes.NewValue(tftypes.String, nil)
	if prefix != nil {
		prefixValue = tftypes.NewValue(tftypes.String, *prefix)
	}
	expiration := tftypes.NewValue(expirationType, nil)
	filter := tftypes.NewValue(filterType, nil)
	if unknownComputed {
		expiration = tftypes.NewValue(expirationType, tftypes.UnknownValue)
		filter = tftypes.NewValue(filterType, tftypes.UnknownValue)
	}
	rule := tftypes.NewValue(ruleType, map[string]tftypes.Value{
		"lifecycle_rule_id": tftypes.NewValue(tftypes.String, id),
		"prefix":            prefixValue,
		"status":            tftypes.NewValue(tftypes.String, status),
		"expiration":        expiration,
		"filter":            filter,
	})
	return tftypes.NewValue(resourceType, map[string]tftypes.Value{
		tosBucketLifecycleAttr: tftypes.NewValue(resourceType.AttributeTypes[tosBucketLifecycleAttr], []tftypes.Value{rule}),
	})
}

func tosBucketACLTestType() tftypes.Object {
	grantee := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"canned":       tftypes.String,
		"display_name": tftypes.String,
		"grantee_id":   tftypes.String,
		"type":         tftypes.String,
	}}
	grant := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"grantee":    grantee,
		"permission": tftypes.String,
	}}
	return tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		tosBucketACLAttr: tftypes.Object{AttributeTypes: map[string]tftypes.Type{
			"bucket_acl_delivered": tftypes.Bool,
			"grants":               tftypes.Set{ElementType: grant},
			"owner": tftypes.Object{AttributeTypes: map[string]tftypes.Type{
				"display_name": tftypes.String,
				"owner_id":     tftypes.String,
			}},
		}},
	}}
}

func tosBucketACLTestValue(resourceType tftypes.Object, granteeID, typ, permission string, unknownComputed bool) tftypes.Value {
	aclType := resourceType.AttributeTypes[tosBucketACLAttr].(tftypes.Object)
	grantType := aclType.AttributeTypes["grants"].(tftypes.Set).ElementType.(tftypes.Object)
	granteeType := grantType.AttributeTypes["grantee"].(tftypes.Object)
	canned := tftypes.NewValue(tftypes.String, nil)
	displayName := tftypes.NewValue(tftypes.String, nil)
	if unknownComputed {
		canned = tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
		displayName = tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
	}
	grant := tftypes.NewValue(grantType, map[string]tftypes.Value{
		"grantee": tftypes.NewValue(granteeType, map[string]tftypes.Value{
			"canned":       canned,
			"display_name": displayName,
			"grantee_id":   tftypes.NewValue(tftypes.String, granteeID),
			"type":         tftypes.NewValue(tftypes.String, typ),
		}),
		"permission": tftypes.NewValue(tftypes.String, permission),
	})
	acl := tftypes.NewValue(aclType, map[string]tftypes.Value{
		"bucket_acl_delivered": tftypes.NewValue(tftypes.Bool, nil),
		"grants":               tftypes.NewValue(aclType.AttributeTypes["grants"], []tftypes.Value{grant}),
		"owner": tftypes.NewValue(aclType.AttributeTypes["owner"], map[string]tftypes.Value{
			"display_name": tftypes.NewValue(tftypes.String, nil),
			"owner_id":     tftypes.NewValue(tftypes.String, granteeID),
		}),
	})
	return tftypes.NewValue(resourceType, map[string]tftypes.Value{
		tosBucketACLAttr: acl,
	})
}

func jsonQuote(value string) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(encoded)
}
