package cloudcontrol

import (
	"encoding/json"
	"strings"
	"testing"

	ccsdk "github.com/volcengine/terraform-provider-volcenginecc/internal/cloudcontrol"
	"github.com/volcengine/terraform-provider-volcenginecc/internal/tfresource"
	"github.com/volcengine/volcengine-go-sdk/volcengine"
)

func TestNormalizeResourceDescriptionPreservesTagFields(t *testing.T) {
	desc := &ccsdk.ResourceDescriptionForGetResourceOutput{
		Properties: volcengine.String(`{
			"Tags": [
				{"Key": "env", "Type": "CUSTOM", "Value": "test"},
				{"Key": "sys:owner", "Type": "SYSTEM", "Value": "platform"}
			]
		}`),
	}

	if err := NormalizeResourceDescription(desc); err != nil {
		t.Fatalf("NormalizeResourceDescription() error = %v", err)
	}

	var got map[string][]map[string]string
	if err := json.Unmarshal([]byte(*desc.Properties), &got); err != nil {
		t.Fatalf("unmarshal normalized properties: %v", err)
	}

	tags := got["Tags"]
	if len(tags) != 1 {
		t.Fatalf("len(Tags) = %d, want 1; Tags = %#v", len(tags), tags)
	}
	if tags[0]["Key"] != "env" || tags[0]["Value"] != "test" || tags[0]["Type"] != "CUSTOM" {
		t.Fatalf("Tags[0] = %#v, want Key/Value/Type preserved", tags[0])
	}
}

// TestNormalizeResourceDescriptionMalformedPropertiesIsNotNotFound verifies
// malformed Properties remain a normalization error rather than NotFound.
func TestNormalizeResourceDescriptionMalformedPropertiesIsNotNotFound(t *testing.T) {
	t.Parallel()

	properties := "{"
	description := &ccsdk.ResourceDescriptionForGetResourceOutput{Properties: &properties}

	err := NormalizeResourceDescription(description)
	if err == nil {
		t.Fatal("NormalizeResourceDescription() returned nil, want an error")
	}
	if tfresource.NotFound(err) {
		t.Fatalf("NormalizeResourceDescription() error must not be NotFound: %v", err)
	}
	if !strings.Contains(err.Error(), "failed to unmarshal ResourceDescription.Properties") {
		t.Fatalf("NormalizeResourceDescription() error = %q, want unmarshal context", err)
	}
}
