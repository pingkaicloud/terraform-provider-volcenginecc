package generic

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// liveObserveJoinedZoneIDs is the 2026-09-04 EndpointService Observe shape:
// Cloud Control returned resources[0].zoneIds as one comma-joined string.
const liveObserveJoinedZoneIDs = `{"Resources":[{"ResourceId":"nlb-2wf8mbkriiy2oz9cquhe870e","ZoneIds":["cn-beijing-a,cn-beijing-c,cn-beijing-d"]}],"ZoneIds":["cn-beijing-a,cn-beijing-c,cn-beijing-d"]}`

const liveSpecSplitZoneIDs = `{"Resources":[{"ResourceId":"nlb-2wf8mbkriiy2oz9cquhe870e","ZoneIds":["cn-beijing-a","cn-beijing-c","cn-beijing-d"]}],"ZoneIds":["cn-beijing-a","cn-beijing-c","cn-beijing-d"]}`

func TestExpandPrivateLinkEndpointServiceZoneIDsJSONLiveObserve(t *testing.T) {
	t.Parallel()

	got, changed, err := expandPrivateLinkEndpointServiceZoneIDsJSON(liveObserveJoinedZoneIDs)
	if err != nil {
		t.Fatalf("expandPrivateLinkEndpointServiceZoneIDsJSON() error = %v", err)
	}
	if !changed {
		t.Fatal("expandPrivateLinkEndpointServiceZoneIDsJSON() changed = false, want true")
	}

	var document map[string]any
	if err := json.Unmarshal([]byte(got), &document); err != nil {
		t.Fatal(err)
	}
	want := []string{"cn-beijing-a", "cn-beijing-c", "cn-beijing-d"}
	if ids, ok := privateLinkZoneIDsFromJSON(document["ZoneIds"]); !ok || !stringSetEqual(ids, want) {
		t.Fatalf("ZoneIds = %v, want %v", ids, want)
	}
	resources := document["Resources"].([]any)
	resource := resources[0].(map[string]any)
	if ids, ok := privateLinkZoneIDsFromJSON(resource["ZoneIds"]); !ok || !stringSetEqual(ids, want) {
		t.Fatalf("Resources[0].ZoneIds = %v, want %v", ids, want)
	}
}

func TestExpandPrivateLinkEndpointServiceZoneIDsJSONStringReadback(t *testing.T) {
	t.Parallel()

	got, changed, err := expandPrivateLinkEndpointServiceZoneIDsJSON(`{"ZoneIds":"cn-beijing-a,cn-beijing-c"}`)
	if err != nil {
		t.Fatalf("expandPrivateLinkEndpointServiceZoneIDsJSON() error = %v", err)
	}
	if !changed {
		t.Fatal("expected string ZoneIds to expand")
	}
	var document map[string]any
	if err := json.Unmarshal([]byte(got), &document); err != nil {
		t.Fatal(err)
	}
	ids, ok := privateLinkZoneIDsFromJSON(document["ZoneIds"])
	if !ok || !stringSetEqual(ids, []string{"cn-beijing-a", "cn-beijing-c"}) {
		t.Fatalf("ZoneIds = %v", ids)
	}
}

func TestExpandPrivateLinkEndpointServiceZoneIDsJSONIdempotent(t *testing.T) {
	t.Parallel()

	got, changed, err := expandPrivateLinkEndpointServiceZoneIDsJSON(liveSpecSplitZoneIDs)
	if err != nil {
		t.Fatalf("expandPrivateLinkEndpointServiceZoneIDsJSON() error = %v", err)
	}
	if changed {
		t.Fatalf("already-split ZoneIds changed: %s", got)
	}
}

func TestSuppressPrivateLinkEndpointServiceZoneIDsLiveObserveEmptyPatch(t *testing.T) {
	t.Parallel()

	current, planned, _, err := suppressPrivateLinkEndpointServiceZoneIDs(liveObserveJoinedZoneIDs, liveSpecSplitZoneIDs)
	if err != nil {
		t.Fatalf("suppressPrivateLinkEndpointServiceZoneIDs() error = %v", err)
	}
	patch, err := patchDocument(current, planned)
	if err != nil {
		t.Fatalf("patchDocument() error = %v", err)
	}
	if patch != "[]" {
		t.Fatalf("patchDocument() = %s, want []", patch)
	}
}

func TestSuppressPrivateLinkEndpointServiceZoneIDsKeepsRealAZChange(t *testing.T) {
	t.Parallel()

	planned := `{"Resources":[{"ResourceId":"nlb-2wf8mbkriiy2oz9cquhe870e","ZoneIds":["cn-beijing-a","cn-beijing-c"]}],"ZoneIds":["cn-beijing-a","cn-beijing-c"]}`
	current, plannedOut, _, err := suppressPrivateLinkEndpointServiceZoneIDs(liveObserveJoinedZoneIDs, planned)
	if err != nil {
		t.Fatalf("suppressPrivateLinkEndpointServiceZoneIDs() error = %v", err)
	}
	patch, err := patchDocument(current, plannedOut)
	if err != nil {
		t.Fatalf("patchDocument() error = %v", err)
	}
	if patch == "[]" {
		t.Fatal("real AZ shrink produced an empty patch")
	}
}

func TestNormalizePrivateLinkEndpointServiceZoneIDsPlanLiveObserve(t *testing.T) {
	t.Parallel()

	resourceType := privateLinkEndpointServiceTestType()
	nlb := "nlb-2wf8mbkriiy2oz9cquhe870e"
	prior := privateLinkEndpointServiceTestValue(resourceType, nlb, []string{"cn-beijing-a,cn-beijing-c,cn-beijing-d"})
	planned := privateLinkEndpointServiceTestValue(resourceType, nlb, []string{"cn-beijing-a", "cn-beijing-c", "cn-beijing-d"})

	before := filteredPlanDiffPaths(t, planned, prior)
	if len(before) == 0 {
		t.Fatal("pre-normalize filtered diff is empty, live Observe shape should drift")
	}

	normalized, err := normalizePrivateLinkEndpointServiceZoneIDsPlan(prior, planned)
	if err != nil {
		t.Fatalf("normalizePrivateLinkEndpointServiceZoneIDsPlan() error = %v", err)
	}
	after := filteredPlanDiffPaths(t, normalized, prior)
	if len(after) != 0 {
		t.Fatalf("post-normalize filtered diff paths = %v, want none", after)
	}
}

func TestNormalizePrivateLinkEndpointServiceZoneIDsPlanKeepsRealAZChange(t *testing.T) {
	t.Parallel()

	resourceType := privateLinkEndpointServiceTestType()
	nlb := "nlb-2wf8mbkriiy2oz9cquhe870e"
	prior := privateLinkEndpointServiceTestValue(resourceType, nlb, []string{"cn-beijing-a", "cn-beijing-c", "cn-beijing-d"})
	planned := privateLinkEndpointServiceTestValue(resourceType, nlb, []string{"cn-beijing-a", "cn-beijing-c"})

	got, err := normalizePrivateLinkEndpointServiceZoneIDsPlan(prior, planned)
	if err != nil {
		t.Fatalf("normalizePrivateLinkEndpointServiceZoneIDsPlan() error = %v", err)
	}
	if !got.Equal(planned) {
		t.Fatal("real AZ change was rewritten")
	}
	if diffs := filteredPlanDiffPaths(t, got, prior); len(diffs) == 0 {
		t.Fatal("real AZ change produced no filtered diff")
	}
}

func TestNormalizePrivateLinkEndpointServiceZoneIDsPlanOrderOnly(t *testing.T) {
	t.Parallel()

	resourceType := privateLinkEndpointServiceTestType()
	nlb := "nlb-2wf8mbkriiy2oz9cquhe870e"
	prior := privateLinkEndpointServiceTestValue(resourceType, nlb, []string{"cn-beijing-c", "cn-beijing-a", "cn-beijing-d"})
	planned := privateLinkEndpointServiceTestValue(resourceType, nlb, []string{"cn-beijing-a", "cn-beijing-c", "cn-beijing-d"})

	normalized, err := normalizePrivateLinkEndpointServiceZoneIDsPlan(prior, planned)
	if err != nil {
		t.Fatalf("normalizePrivateLinkEndpointServiceZoneIDsPlan() error = %v", err)
	}
	after := filteredPlanDiffPaths(t, normalized, prior)
	if len(after) != 0 {
		t.Fatalf("order-only filtered diff paths = %v, want none", after)
	}
}

func privateLinkEndpointServiceTestType() tftypes.Object {
	resourceType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"resource_id": tftypes.String,
		"zone_ids":    tftypes.List{ElementType: tftypes.String},
	}}
	return tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"zone_ids":  tftypes.Set{ElementType: tftypes.String},
		"resources": tftypes.Set{ElementType: resourceType},
	}}
}

func privateLinkEndpointServiceTestValue(resourceType tftypes.Type, resourceID string, zoneIDs []string) tftypes.Value {
	objectType := resourceType.(tftypes.Object)
	resourceObjectType := objectType.AttributeTypes["resources"].(tftypes.Set).ElementType.(tftypes.Object)
	return tftypes.NewValue(resourceType, map[string]tftypes.Value{
		"zone_ids": privateLinkZoneIDCollectionValue(objectType.AttributeTypes["zone_ids"], zoneIDs),
		"resources": tftypes.NewValue(objectType.AttributeTypes["resources"], []tftypes.Value{
			tftypes.NewValue(resourceObjectType, map[string]tftypes.Value{
				"resource_id": tftypes.NewValue(tftypes.String, resourceID),
				"zone_ids":    privateLinkZoneIDCollectionValue(resourceObjectType.AttributeTypes["zone_ids"], zoneIDs),
			}),
		}),
	})
}

func privateLinkZoneIDCollectionValue(collectionType tftypes.Type, ids []string) tftypes.Value {
	values := make([]tftypes.Value, 0, len(ids))
	for _, id := range ids {
		values = append(values, tftypes.NewValue(tftypes.String, id))
	}
	return tftypes.NewValue(collectionType, values)
}

func TestExpandPrivateLinkEndpointServiceZoneIDsJSONUsesLiveCommaJoin(t *testing.T) {
	t.Parallel()

	if !strings.Contains(liveObserveJoinedZoneIDs, "cn-beijing-a,cn-beijing-c,cn-beijing-d") {
		t.Fatal("live fixture lost the comma-joined Observe string")
	}
}
