package generic

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"
)

func TestAlignCollectionByIdentity(t *testing.T) {
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
			prior:           identityElements(identityElement("A", "one"), identityElement("A", "two")),
			remote:          identityElements(identityElement("A", "one")),
			identifierPaths: []string{"/Name"},
			wantError:       "duplicate identity",
		},
		"R7 missing identity rejected": {
			prior:           identityElements(identityElement("A", "one")),
			remote:          identityElements(map[string]interface{}{"Value": "one"}),
			identifierPaths: []string{"/Name"},
			wantError:       "missing or null",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			aligned, err := alignCollectionByIdentity(test.prior, test.remote, test.identifierPaths)
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
			wantFinalJSON: `{"Labels":[{"Name":"B","Value":"two"},{"Name":"A","Value":"one"}]}`,
		},
		"U2 update same identity": {
			current:       `{"Labels":[{"Name":"A","Value":"old"},{"Name":"B","Value":"two"}]}`,
			planned:       `{"Labels":[{"Name":"A","Value":"new"},{"Name":"B","Value":"two"}]}`,
			wantFinalJSON: `{"Labels":[{"Name":"A","Value":"new"},{"Name":"B","Value":"two"}]}`,
		},
		"U3 reverse order and update correct identity": {
			current:       `{"Labels":[{"Name":"A","Value":"old"},{"Name":"B","Value":"two"}]}`,
			planned:       `{"Labels":[{"Name":"B","Value":"two"},{"Name":"A","Value":"new"}]}`,
			wantFinalJSON: `{"Labels":[{"Name":"B","Value":"two"},{"Name":"A","Value":"new"}]}`,
		},
		"U4 add identity": {
			current:       `{"Labels":[{"Name":"A","Value":"one"},{"Name":"B","Value":"two"}]}`,
			planned:       `{"Labels":[{"Name":"A","Value":"one"},{"Name":"C","Value":"three"},{"Name":"B","Value":"two"}]}`,
			wantFinalJSON: `{"Labels":[{"Name":"A","Value":"one"},{"Name":"C","Value":"three"},{"Name":"B","Value":"two"}]}`,
		},
		"U5 remove identity": {
			current:       `{"Labels":[{"Name":"A","Value":"one"},{"Name":"B","Value":"two"},{"Name":"C","Value":"three"}]}`,
			planned:       `{"Labels":[{"Name":"C","Value":"three"},{"Name":"A","Value":"one"}]}`,
			wantFinalJSON: `{"Labels":[{"Name":"C","Value":"three"},{"Name":"A","Value":"one"}]}`,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			normalizedCurrent, normalizedPlanned, err := normalizeIdentityCollections(test.current, test.planned, identity)
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
			if name == "U3 reverse order and update correct identity" && !strings.Contains(patch, `/Labels/1/Value`) {
				t.Fatalf("patch does not target reordered A value: %s", patch)
			}
		})
	}
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
