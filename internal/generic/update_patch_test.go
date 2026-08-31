package generic

import (
	"encoding/json"
	"testing"
)

func TestSuppressVKENodePoolDesiredReplicas(t *testing.T) {
	tests := map[string]struct {
		current       string
		planned       string
		changed       bool
		expectRemoved bool
	}{
		"enabled current": {
			current:       `{"AutoScaling":{"Enabled":true,"DesiredReplicas":1}}`,
			planned:       `{"AutoScaling":{"Enabled":true,"DesiredReplicas":0,"MinReplicas":0}}`,
			changed:       true,
			expectRemoved: true,
		},
		"enabled planned": {
			current:       `{"AutoScaling":{"Enabled":false,"DesiredReplicas":1}}`,
			planned:       `{"AutoScaling":{"Enabled":true,"DesiredReplicas":2}}`,
			changed:       true,
			expectRemoved: true,
		},
		"disabled": {
			current:       `{"AutoScaling":{"Enabled":false,"DesiredReplicas":1}}`,
			planned:       `{"AutoScaling":{"Enabled":false,"DesiredReplicas":2}}`,
			changed:       false,
			expectRemoved: false,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			current, planned, changed, err := suppressVKENodePoolDesiredReplicas(tt.current, tt.planned)
			if err != nil {
				t.Fatalf("suppressVKENodePoolDesiredReplicas() error = %v", err)
			}
			if changed != tt.changed {
				t.Fatalf("changed = %v, want %v", changed, tt.changed)
			}

			for _, documentJSON := range []string{current, planned} {
				var document map[string]any
				if err := json.Unmarshal([]byte(documentJSON), &document); err != nil {
					t.Fatal(err)
				}
				autoScaling := document["AutoScaling"].(map[string]any)
				_, hasDesired := autoScaling["DesiredReplicas"]
				if hasDesired == tt.expectRemoved {
					t.Fatalf("DesiredReplicas presence = %v, want %v", hasDesired, !tt.expectRemoved)
				}
			}
		})
	}
}

func TestSuppressVKENodePoolDesiredReplicasEmptyPatch(t *testing.T) {
	current, planned, _, err := suppressVKENodePoolDesiredReplicas(
		`{"AutoScaling":{"Enabled":true,"DesiredReplicas":1}}`,
		`{"AutoScaling":{"Enabled":true,"DesiredReplicas":2}}`,
	)
	if err != nil {
		t.Fatalf("suppressVKENodePoolDesiredReplicas() error = %v", err)
	}

	patch, err := patchDocument(current, planned)
	if err != nil {
		t.Fatalf("patchDocument() error = %v", err)
	}
	if patch != "[]" {
		t.Fatalf("patchDocument() = %s, want []", patch)
	}
}
