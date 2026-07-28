package generic

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"
)

func TestCanonicalizeCollectionByIdentity(t *testing.T) {
	tests := map[string]struct {
		prior           []interface{}
		remote          []interface{}
		identifierPaths []string
		wantNames       []string
		wantError       string
	}{
		"R1 remote order changed": {
			prior:           identityElements(identityElement("A", "old"), identityElement("B", "old")),
			remote:          identityElements(identityElement("B", "old"), identityElement("A", "old")),
			identifierPaths: []string{"/Name"},
			wantNames:       []string{"A", "B"},
		},
		"R2 remote backfill and order changed": {
			prior: identityElements(
				map[string]interface{}{"Name": "A", "Value": "old", "Status": nil},
				map[string]interface{}{"Name": "B", "Value": "old", "Status": nil},
			),
			remote: identityElements(
				map[string]interface{}{"Name": "B", "Value": "old", "Status": "active"},
				map[string]interface{}{"Name": "A", "Value": "old", "Status": "active"},
			),
			identifierPaths: []string{"/Name"},
			wantNames:       []string{"A", "B"},
		},
		"R3 remote addition appended": {
			prior:           identityElements(identityElement("A", "old"), identityElement("B", "old")),
			remote:          identityElements(identityElement("C", "new"), identityElement("B", "old"), identityElement("A", "old")),
			identifierPaths: []string{"/Name"},
			wantNames:       []string{"A", "B", "C"},
		},
		"R4 remote deletion omitted": {
			prior:           identityElements(identityElement("A", "old"), identityElement("B", "old")),
			remote:          identityElements(identityElement("B", "old")),
			identifierPaths: []string{"/Name"},
			wantNames:       []string{"B"},
		},
		"R5 composite identity": {
			prior: identityElements(
				map[string]interface{}{"ZoneId": "z1", "NodeType": "Primary", "Value": "one"},
				map[string]interface{}{"ZoneId": "z1", "NodeType": "Readonly", "Value": "two"},
			),
			remote: identityElements(
				map[string]interface{}{"ZoneId": "z1", "NodeType": "Readonly", "Value": "two"},
				map[string]interface{}{"ZoneId": "z1", "NodeType": "Primary", "Value": "one"},
			),
			identifierPaths: []string{"/ZoneId", "/NodeType"},
			wantNames:       []string{"Primary", "Readonly"},
		},
		"R6 duplicate identity rejected": {
			prior:           identityElements(identityElement("A", "one")),
			remote:          identityElements(identityElement("A", "one"), identityElement("A", "two")),
			identifierPaths: []string{"/Name"},
			wantError:       "duplicate identity",
		},
		"R7 missing identity rejected": {
			prior:           identityElements(identityElement("A", "one")),
			remote:          identityElements(map[string]interface{}{"Value": "one"}),
			identifierPaths: []string{"/Name"},
			wantError:       "is missing",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			aligned, err := canonicalizeCollectionByIdentity(test.remote, test.identifierPaths)
			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("expected error containing %q, got %v", test.wantError, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got := make([]string, 0, len(aligned))
			field := "Name"
			if len(test.identifierPaths) == 2 {
				field = "NodeType"
			}
			for _, element := range aligned {
				got = append(got, element.(map[string]interface{})[field].(string))
			}
			assertStringSliceEqual(t, got, test.wantNames)
		})
	}
}

func TestCanonicalizeCollectionByIdentityComparator(t *testing.T) {
	tests := map[string]struct {
		collection      []interface{}
		identifierPaths []string
		field           string
		want            []string
	}{
		"single string field": {
			collection: identityElements(
				identityElement("k2", "two"),
				identityElement("Name", "name"),
				identityElement("k1", "one"),
			),
			identifierPaths: []string{"/Name"},
			field:           "Name",
			want:            []string{"Name", "k1", "k2"},
		},
		"composite fields": {
			collection: identityElements(
				map[string]interface{}{"ZoneId": "b", "NodeType": "primary"},
				map[string]interface{}{"ZoneId": "a", "NodeType": "secondary"},
				map[string]interface{}{"ZoneId": "a", "NodeType": "primary"},
			),
			identifierPaths: []string{"/ZoneId", "/NodeType"},
			field:           "NodeType",
			want:            []string{"primary", "secondary", "primary"},
		},
		"nested field": {
			collection: identityElements(
				map[string]interface{}{"Scope": map[string]interface{}{"Name": "z"}},
				map[string]interface{}{"Scope": map[string]interface{}{"Name": "a"}},
			),
			identifierPaths: []string{"/Scope/Name"},
			field:           "Scope.Name",
			want:            []string{"a", "z"},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			canonical, err := canonicalizeCollectionByIdentity(test.collection, test.identifierPaths)
			if err != nil {
				t.Fatalf("canonicalizeCollectionByIdentity() error = %v", err)
			}
			got := make([]string, 0, len(canonical))
			for _, raw := range canonical {
				element := raw.(map[string]interface{})
				if test.field == "Scope.Name" {
					got = append(got, element["Scope"].(map[string]interface{})["Name"].(string))
					continue
				}
				got = append(got, element[test.field].(string))
			}
			assertStringSliceEqual(t, got, test.want)
		})
	}
}

func TestCanonicalizeIdentityDesiredStateScalarTypes(t *testing.T) {
	tests := map[string]struct {
		state      string
		identifier string
		want       string
	}{
		"number uses numeric order": {
			state:      `{"Items":[{"Id":10},{"Id":2},{"Id":-1}]}`,
			identifier: "/Id",
			want:       `{"Items":[{"Id":-1},{"Id":2},{"Id":10}]}`,
		},
		"boolean orders false before true": {
			state:      `{"Items":[{"Enabled":true},{"Enabled":false}]}`,
			identifier: "/Enabled",
			want:       `{"Items":[{"Enabled":false},{"Enabled":true}]}`,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := canonicalizeIdentityDesiredState(test.state, []CollectionIdentity{{
				PropertyPath:    "/Items",
				IdentifierPaths: []string{test.identifier},
			}})
			if err != nil {
				t.Fatalf("canonicalizeIdentityDesiredState() error = %v", err)
			}
			assertJSONEqual(t, got, test.want)
		})
	}
}

func TestCanonicalizeIdentityDesiredStateRejectsEquivalentNumericIdentity(t *testing.T) {
	_, err := canonicalizeIdentityDesiredState(
		`{"Items":[{"Id":1},{"Id":1.0}]}`,
		[]CollectionIdentity{{PropertyPath: "/Items", IdentifierPaths: []string{"/Id"}}},
	)
	if err == nil || !strings.Contains(err.Error(), "duplicate identity") {
		t.Fatalf("canonicalizeIdentityDesiredState() error = %v, want duplicate identity", err)
	}
}

func TestPairCollectionsByIdentity(t *testing.T) {
	tests := map[string]struct {
		left            []interface{}
		right           []interface{}
		identifierPaths []string
		uniqueItems     bool
		wantMatched     [][2]int
		wantLeftOnly    []int
		wantRightOnly   []int
		wantError       string
		wantDegraded    string
	}{
		"same order and identity": {
			left:            identityElements(identityElement("A", "old"), identityElement("B", "old")),
			right:           identityElements(identityElement("A", "old"), identityElement("B", "old")),
			identifierPaths: []string{"/Name"},
			uniqueItems:     true,
			wantMatched:     [][2]int{{0, 0}, {1, 1}},
		},
		"reverse order and identity": {
			left:            identityElements(identityElement("A", "old"), identityElement("B", "old")),
			right:           identityElements(identityElement("B", "old"), identityElement("A", "old")),
			identifierPaths: []string{"/Name"},
			uniqueItems:     true,
			wantMatched:     [][2]int{{0, 1}, {1, 0}},
		},
		"non identity field differs": {
			left:            identityElements(identityElement("A", "old")),
			right:           identityElements(identityElement("A", "new")),
			identifierPaths: []string{"/Name"},
			uniqueItems:     true,
			wantMatched:     [][2]int{{0, 0}},
		},
		"composite identity": {
			left: identityElements(
				map[string]interface{}{"Zone": "z1", "Role": "primary"},
				map[string]interface{}{"Zone": "z1", "Role": "readonly"},
			),
			right: identityElements(
				map[string]interface{}{"Zone": "z1", "Role": "readonly"},
				map[string]interface{}{"Zone": "z1", "Role": "primary"},
			),
			identifierPaths: []string{"/Zone", "/Role"},
			wantMatched:     [][2]int{{0, 1}, {1, 0}},
		},
		"nested identity": {
			left: identityElements(
				map[string]interface{}{"Scope": map[string]interface{}{"Region": "cn-beijing"}},
			),
			right: identityElements(
				map[string]interface{}{"Scope": map[string]interface{}{"Region": "cn-beijing"}},
			),
			identifierPaths: []string{"/Scope/Region"},
			wantMatched:     [][2]int{{0, 0}},
		},
		"identity changed": {
			left:            identityElements(identityElement("A", "same")),
			right:           identityElements(identityElement("B", "same")),
			identifierPaths: []string{"/Name"},
			wantLeftOnly:    []int{0},
			wantRightOnly:   []int{0},
		},
		"element added": {
			left:            identityElements(identityElement("A", "one")),
			right:           identityElements(identityElement("A", "one"), identityElement("B", "two")),
			identifierPaths: []string{"/Name"},
			wantMatched:     [][2]int{{0, 0}},
			wantRightOnly:   []int{1},
		},
		"element removed": {
			left:            identityElements(identityElement("A", "one"), identityElement("B", "two")),
			right:           identityElements(identityElement("B", "two")),
			identifierPaths: []string{"/Name"},
			wantMatched:     [][2]int{{1, 0}},
			wantLeftOnly:    []int{0},
		},
		"missing identity": {
			left:            identityElements(map[string]interface{}{"Value": "one"}),
			right:           identityElements(identityElement("A", "one")),
			identifierPaths: []string{"/Name"},
			wantError:       `identifier path "/Name" is missing`,
		},
		"null identity": {
			left:            identityElements(map[string]interface{}{"Name": nil}),
			right:           identityElements(identityElement("A", "one")),
			identifierPaths: []string{"/Name"},
			wantError:       `identifier path "/Name" is null`,
		},
		"unknown identity": {
			left:            identityElements(map[string]interface{}{"Name": collectionIdentityUnknown}),
			right:           identityElements(identityElement("A", "one")),
			identifierPaths: []string{"/Name"},
			wantError:       `identifier path "/Name" is unknown`,
		},
		"duplicate identity on left": {
			left:            identityElements(identityElement("A", "one"), identityElement("A", "two")),
			right:           identityElements(identityElement("A", "one")),
			identifierPaths: []string{"/Name"},
			wantError:       "indexing left collection: duplicate identity",
		},
		"duplicate identity on right": {
			left:            identityElements(identityElement("A", "one")),
			right:           identityElements(identityElement("A", "one"), identityElement("A", "two")),
			identifierPaths: []string{"/Name"},
			wantError:       "indexing right collection: duplicate identity",
		},
		"typed values do not collide": {
			left:            identityElements(map[string]interface{}{"Name": 1}),
			right:           identityElements(map[string]interface{}{"Name": "1"}),
			identifierPaths: []string{"/Name"},
			wantLeftOnly:    []int{0},
			wantRightOnly:   []int{0},
		},
		"set and multiset use same configured identity core": {
			left:            identityElements(identityElement("A", "old")),
			right:           identityElements(identityElement("A", "new")),
			identifierPaths: []string{"/Name"},
			uniqueItems:     false,
			wantMatched:     [][2]int{{0, 0}},
		},
		"missing identity metadata degrades without index fallback": {
			left:         identityElements(identityElement("A", "one")),
			right:        identityElements(identityElement("A", "one")),
			uniqueItems:  true,
			wantDegraded: "no elementIdentifier is configured",
		},
		"multiset duplicate without safe identity degrades": {
			left: identityElements(
				identityElement("A", "same"),
				identityElement("A", "same"),
			),
			right: identityElements(
				identityElement("A", "same"),
				identityElement("A", "same"),
			),
			identifierPaths: []string{"/Name"},
			uniqueItems:     false,
			wantDegraded:    "multiset contains complete duplicate elements without a safe identity",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			pairing, err := pairCollectionsByIdentity(
				test.left,
				test.right,
				test.identifierPaths,
				test.uniqueItems,
			)
			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("expected error containing %q, got %v", test.wantError, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if test.wantDegraded != "" {
				if !pairing.Degraded || pairing.Reason != test.wantDegraded {
					t.Fatalf("got degraded=%t reason=%q, want reason=%q", pairing.Degraded, pairing.Reason, test.wantDegraded)
				}
				return
			}
			if pairing.Degraded {
				t.Fatalf("unexpected degradation: %s", pairing.Reason)
			}
			assertCollectionPairs(t, pairing.Matched, test.wantMatched)
			assertCollectionReferences(t, pairing.LeftOnly, test.wantLeftOnly)
			assertCollectionReferences(t, pairing.RightOnly, test.wantRightOnly)
		})
	}
}

func assertCollectionPairs(t *testing.T, got []collectionElementPair, want [][2]int) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got pairs %#v, want indexes %#v", got, want)
	}
	for index := range want {
		if got[index].LeftIndex != want[index][0] || got[index].RightIndex != want[index][1] {
			t.Fatalf("got pairs %#v, want indexes %#v", got, want)
		}
	}
}

func assertCollectionReferences(t *testing.T, got []collectionElementReference, want []int) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got references %#v, want indexes %#v", got, want)
	}
	for index := range want {
		if got[index].Index != want[index] {
			t.Fatalf("got references %#v, want indexes %#v", got, want)
		}
	}
}

func TestNormalizeIdentityCollectionsForPatch(t *testing.T) {
	identity := []CollectionIdentity{{
		PropertyPath:    "/Labels",
		IdentifierPaths: []string{"/Name"},
	}}

	tests := map[string]struct {
		current       string
		planned       string
		wantPatch     string
		wantFinalJSON string
	}{
		"U1 order only": {
			current:       `{"Labels":[{"Name":"A","Value":"one"},{"Name":"B","Value":"two"}]}`,
			planned:       `{"Labels":[{"Name":"B","Value":"two"},{"Name":"A","Value":"one"}]}`,
			wantPatch:     `[]`,
			wantFinalJSON: `{"Labels":[{"Name":"A","Value":"one"},{"Name":"B","Value":"two"}]}`,
		},
		"U2 update same identity": {
			current:       `{"Labels":[{"Name":"A","Value":"old"},{"Name":"B","Value":"two"}]}`,
			planned:       `{"Labels":[{"Name":"A","Value":"new"},{"Name":"B","Value":"two"}]}`,
			wantFinalJSON: `{"Labels":[{"Name":"A","Value":"new"},{"Name":"B","Value":"two"}]}`,
		},
		"U3 reverse order and update correct identity": {
			current:       `{"Labels":[{"Name":"A","Value":"old"},{"Name":"B","Value":"two"}]}`,
			planned:       `{"Labels":[{"Name":"B","Value":"two"},{"Name":"A","Value":"new"}]}`,
			wantFinalJSON: `{"Labels":[{"Name":"A","Value":"new"},{"Name":"B","Value":"two"}]}`,
		},
		"U4 add identity": {
			current:       `{"Labels":[{"Name":"A","Value":"one"},{"Name":"B","Value":"two"}]}`,
			planned:       `{"Labels":[{"Name":"A","Value":"one"},{"Name":"C","Value":"three"},{"Name":"B","Value":"two"}]}`,
			wantFinalJSON: `{"Labels":[{"Name":"A","Value":"one"},{"Name":"B","Value":"two"},{"Name":"C","Value":"three"}]}`,
		},
		"U5 remove identity": {
			current:       `{"Labels":[{"Name":"A","Value":"one"},{"Name":"B","Value":"two"},{"Name":"C","Value":"three"}]}`,
			planned:       `{"Labels":[{"Name":"C","Value":"three"},{"Name":"A","Value":"one"}]}`,
			wantFinalJSON: `{"Labels":[{"Name":"A","Value":"one"},{"Name":"C","Value":"three"}]}`,
		},
		"U6 missing planned collection is left to whole-field patch": {
			current:       `{"Labels":[{"Name":"A","Value":"one"}]}`,
			planned:       `{}`,
			wantFinalJSON: `{}`,
		},
		"U7 null current collection is left to whole-field patch": {
			current:       `{"Labels":null}`,
			planned:       `{"Labels":[{"Name":"A","Value":"one"}]}`,
			wantFinalJSON: `{"Labels":[{"Name":"A","Value":"one"}]}`,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			normalizedCurrent, normalizedPlanned, err := canonicalizeIdentityCollections(test.current, test.planned, identity)
			if err != nil {
				t.Fatalf("unexpected normalization error: %v", err)
			}

			patch, err := patchDocument(normalizedCurrent, normalizedPlanned)
			if err != nil {
				t.Fatalf("unexpected patch error: %v", err)
			}
			if test.wantPatch != "" && patch != test.wantPatch {
				t.Fatalf("got patch %s, want %s", patch, test.wantPatch)
			}

			applied, err := applyTestJSONPatch(normalizedCurrent, patch)
			if err != nil {
				t.Fatalf("applying patch %s to %s: %v", patch, normalizedCurrent, err)
			}
			assertJSONEqual(t, applied, test.wantFinalJSON)

			if name == "U2 update same identity" && !strings.Contains(patch, `/Labels/0/Value`) {
				t.Fatalf("patch does not target A value: %s", patch)
			}
			if name == "U3 reverse order and update correct identity" && !strings.Contains(patch, `/Labels/0/Value`) {
				t.Fatalf("patch does not target reordered A value: %s", patch)
			}
		})
	}
}

func TestCanonicalizeIdentityCollectionsRejectsUnsafeIdentity(t *testing.T) {
	identity := []CollectionIdentity{{
		PropertyPath:    "/Labels",
		IdentifierPaths: []string{"/Name"},
	}}

	tests := map[string]struct {
		current   string
		planned   string
		wantError string
	}{
		"missing identity": {
			current:   `{"Labels":[{"Value":"one"}]}`,
			planned:   `{"Labels":[{"Name":"A","Value":"one"}]}`,
			wantError: `identifier path "/Name" is missing`,
		},
		"duplicate current identity": {
			current:   `{"Labels":[{"Name":"A","Value":"one"},{"Name":"A","Value":"two"}]}`,
			planned:   `{"Labels":[{"Name":"A","Value":"new"}]}`,
			wantError: "duplicate identity",
		},
		"duplicate planned identity": {
			current:   `{"Labels":[{"Name":"A","Value":"one"}]}`,
			planned:   `{"Labels":[{"Name":"A","Value":"one"},{"Name":"A","Value":"two"}]}`,
			wantError: "duplicate identity",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			_, _, err := canonicalizeIdentityCollections(test.current, test.planned, identity)
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("canonicalizeIdentityCollections() error = %v, want containing %q", err, test.wantError)
			}
		})
	}
}

func TestCanonicalizeIdentityCollectionsForSetAndMultisetMetadata(t *testing.T) {
	tests := map[string]CollectionIdentity{
		"set metadata": {
			PropertyPath:    "/Labels",
			IdentifierPaths: []string{"/Name"},
			UniqueItems:     true,
		},
		"multiset metadata": {
			PropertyPath:    "/Labels",
			IdentifierPaths: []string{"/Name"},
			UniqueItems:     false,
		},
	}

	for name, identity := range tests {
		t.Run(name, func(t *testing.T) {
			normalizedCurrent, normalizedPlanned, err := canonicalizeIdentityCollections(
				`{"Labels":[{"Name":"A","Value":"old"},{"Name":"B","Value":"same"}]}`,
				`{"Labels":[{"Name":"B","Value":"same"},{"Name":"A","Value":"new"}]}`,
				[]CollectionIdentity{identity},
			)
			if err != nil {
				t.Fatalf("canonicalizeIdentityCollections() error = %v", err)
			}

			patch, err := patchDocument(normalizedCurrent, normalizedPlanned)
			if err != nil {
				t.Fatalf("patchDocument() error = %v", err)
			}
			if !strings.Contains(patch, `/Labels/0/Value`) {
				t.Fatalf("patch does not target reordered A value: %s", patch)
			}

			applied, err := applyTestJSONPatch(normalizedCurrent, patch)
			if err != nil {
				t.Fatalf("applying patch %s to %s: %v", patch, normalizedCurrent, err)
			}
			assertJSONEqual(t, applied, `{"Labels":[{"Name":"A","Value":"new"},{"Name":"B","Value":"same"}]}`)
		})
	}
}

func TestCanonicalizeIdentityCollectionsUsesObjectiveOrderForPatchIndexes(t *testing.T) {
	identity := []CollectionIdentity{{
		PropertyPath:    "/PrefixListEntries",
		IdentifierPaths: []string{"/Cidr"},
		UniqueItems:     true,
	}}
	current := `{"PrefixListEntries":[{"Cidr":"A","Description":"a"},{"Cidr":"B","Description":"b"}]}`
	planned := `{"PrefixListEntries":[{"Cidr":"A","Description":"a"},{"Cidr":"B","Description":"b-updated"}]}`
	remote := `{"PrefixListEntries":[{"Cidr":"B","Description":"b"},{"Cidr":"A","Description":"a"}]}`

	normalizedCurrent, normalizedPlanned, err := canonicalizeIdentityCollections(current, planned, identity)
	if err != nil {
		t.Fatalf("canonicalizeIdentityCollections() error = %v", err)
	}
	patch, err := patchDocument(normalizedCurrent, normalizedPlanned)
	if err != nil {
		t.Fatalf("patchDocument() error = %v", err)
	}
	if !strings.Contains(patch, `/PrefixListEntries/1/Description`) {
		t.Fatalf("patch does not target canonical B index: %s", patch)
	}

	canonicalRemote, err := canonicalizeIdentityDesiredState(remote, identity)
	if err != nil {
		t.Fatalf("canonicalizeIdentityDesiredState() error = %v", err)
	}
	applied, err := applyTestJSONPatch(canonicalRemote, patch)
	if err != nil {
		t.Fatalf("applying patch %s to canonical remote %s: %v", patch, canonicalRemote, err)
	}
	assertJSONEqual(t, applied, `{"PrefixListEntries":[{"Cidr":"A","Description":"a"},{"Cidr":"B","Description":"b-updated"}]}`)
}

func TestCanonicalizeIdentityCollectionsSurvivesHandlerReadOrderChange(t *testing.T) {
	identity := []CollectionIdentity{{
		PropertyPath:    "/Tags",
		IdentifierPaths: []string{"/Key"},
		UniqueItems:     true,
	}}
	providerCurrent := `{"Tags":[{"Key":"k1","Value":"v2"},{"Key":"Name","Value":"tt-rabbitmq-instance"}]}`
	planned := `{"Tags":[{"Key":"Name","Value":"tt-rabbitmq-instance"},{"Key":"k1","Value":"v1"},{"Key":"k2","Value":"v3"}]}`
	handlerCurrent := `{"Tags":[{"Key":"Name","Value":"tt-rabbitmq-instance"},{"Key":"k1","Value":"v2"}]}`

	canonicalCurrent, canonicalPlanned, err := canonicalizeIdentityCollections(providerCurrent, planned, identity)
	if err != nil {
		t.Fatalf("canonicalizeIdentityCollections() error = %v", err)
	}
	patch, err := patchDocument(canonicalCurrent, canonicalPlanned)
	if err != nil {
		t.Fatalf("patchDocument() error = %v", err)
	}
	if !strings.Contains(patch, `/Tags/1`) || strings.Contains(patch, `/Tags/0`) {
		t.Fatalf("patch does not target canonical k1 index: %s", patch)
	}

	canonicalHandlerCurrent, err := canonicalizeIdentityDesiredState(handlerCurrent, identity)
	if err != nil {
		t.Fatalf("canonicalizeIdentityDesiredState() error = %v", err)
	}
	applied, err := applyTestJSONPatch(canonicalHandlerCurrent, patch)
	if err != nil {
		t.Fatalf("applying patch %s to handler state %s: %v", patch, canonicalHandlerCurrent, err)
	}
	assertJSONEqual(t, applied, planned)
}

func identityElement(name, value string) map[string]interface{} {
	return map[string]interface{}{"Name": name, "Value": value}
}

func identityElements(elements ...map[string]interface{}) []interface{} {
	result := make([]interface{}, len(elements))
	for index, element := range elements {
		result[index] = element
	}
	return result
}

func assertStringSliceEqual(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func assertJSONEqual(t *testing.T, got, want string) {
	t.Helper()
	var gotValue interface{}
	if err := json.Unmarshal([]byte(got), &gotValue); err != nil {
		t.Fatalf("invalid got JSON: %v", err)
	}
	var wantValue interface{}
	if err := json.Unmarshal([]byte(want), &wantValue); err != nil {
		t.Fatalf("invalid want JSON: %v", err)
	}
	gotJSON, _ := json.Marshal(gotValue)
	wantJSON, _ := json.Marshal(wantValue)
	if string(gotJSON) != string(wantJSON) {
		t.Fatalf("got %s, want %s", gotJSON, wantJSON)
	}
}

type testJSONPatchOperation struct {
	Operation string      `json:"op"`
	Path      string      `json:"path"`
	Value     interface{} `json:"value"`
}

func applyTestJSONPatch(document, patch string) (string, error) {
	var root interface{}
	if err := json.Unmarshal([]byte(document), &root); err != nil {
		return "", err
	}
	var operations []testJSONPatchOperation
	if err := json.Unmarshal([]byte(patch), &operations); err != nil {
		return "", err
	}

	for _, operation := range operations {
		segments, err := jsonPointerSegments(operation.Path)
		if err != nil {
			return "", err
		}
		root, err = applyTestJSONPatchOperation(root, segments, operation)
		if err != nil {
			return "", err
		}
	}

	result, err := json.Marshal(root)
	if err != nil {
		return "", err
	}
	return string(result), nil
}

func applyTestJSONPatchOperation(current interface{}, segments []string, operation testJSONPatchOperation) (interface{}, error) {
	if len(segments) == 0 {
		switch operation.Operation {
		case "add", "replace":
			return operation.Value, nil
		case "remove":
			return nil, nil
		default:
			return nil, fmt.Errorf("unsupported operation %q", operation.Operation)
		}
	}

	segment := segments[0]
	switch value := current.(type) {
	case map[string]interface{}:
		if len(segments) == 1 {
			switch operation.Operation {
			case "add", "replace":
				value[segment] = operation.Value
			case "remove":
				delete(value, segment)
			default:
				return nil, fmt.Errorf("unsupported operation %q", operation.Operation)
			}
			return value, nil
		}
		child, ok := value[segment]
		if !ok {
			return nil, fmt.Errorf("object segment %q not found", segment)
		}
		updated, err := applyTestJSONPatchOperation(child, segments[1:], operation)
		if err != nil {
			return nil, err
		}
		value[segment] = updated
		return value, nil

	case []interface{}:
		index, err := strconv.Atoi(segment)
		if err != nil {
			return nil, fmt.Errorf("invalid array index %q", segment)
		}
		if len(segments) == 1 {
			switch operation.Operation {
			case "add":
				if index < 0 || index > len(value) {
					return nil, fmt.Errorf("array add index %d out of range", index)
				}
				value = append(value, nil)
				copy(value[index+1:], value[index:])
				value[index] = operation.Value
			case "replace":
				if index < 0 || index >= len(value) {
					return nil, fmt.Errorf("array replace index %d out of range", index)
				}
				value[index] = operation.Value
			case "remove":
				if index < 0 || index >= len(value) {
					return nil, fmt.Errorf("array remove index %d out of range", index)
				}
				value = append(value[:index], value[index+1:]...)
			default:
				return nil, fmt.Errorf("unsupported operation %q", operation.Operation)
			}
			return value, nil
		}
		if index < 0 || index >= len(value) {
			return nil, fmt.Errorf("array index %d out of range", index)
		}
		updated, err := applyTestJSONPatchOperation(value[index], segments[1:], operation)
		if err != nil {
			return nil, err
		}
		value[index] = updated
		return value, nil

	default:
		return nil, fmt.Errorf("cannot traverse %T", current)
	}
}
