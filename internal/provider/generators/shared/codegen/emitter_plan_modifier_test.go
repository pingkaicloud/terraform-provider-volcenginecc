package codegen

import (
	"strings"
	"testing"

	"github.com/hashicorp/cli"
	"github.com/volcengine/terraform-provider-volcenginecc/internal/ccschema"
)

func TestEmitRootPropertiesSchemaUsesNonNullStateForNestedChildren(t *testing.T) {
	t.Parallel()

	out := &strings.Builder{}
	features, err := Emitter{
		CfResource: syntheticPlanModifierResource(),
		Ui:         cli.NewMockUi(),
		Writer:     out,
	}.EmitRootPropertiesSchema("volcenginecc_test_resource", map[string]string{})
	if err != nil {
		t.Fatalf("EmitRootPropertiesSchema() error = %s", err)
	}

	if got, want := strings.Count(out.String(), "UseStateForUnknown()"), 4; got != want {
		t.Fatalf("UseStateForUnknown count = %d, want %d\n%s", got, want, out.String())
	}
	if got, want := strings.Count(out.String(), "UseNonNullStateForUnknown()"), 5; got != want {
		t.Fatalf("UseNonNullStateForUnknown count = %d, want %d\n%s", got, want, out.String())
	}
	assertContains(t, out.String(), `"top_computed":schema.StringAttribute`)
	assertContains(t, out.String(), "stringplanmodifier.UseStateForUnknown()")
	assertContains(t, out.String(), `"items":schema.ListNestedAttribute`)
	assertContains(t, out.String(), "listplanmodifier.UseStateForUnknown()")
	assertContains(t, out.String(), `"set_items":schema.SetNestedAttribute`)
	assertContains(t, out.String(), "setplanmodifier.UseStateForUnknown()")
	assertContains(t, out.String(), "boolplanmodifier.UseNonNullStateForUnknown()")
	assertContains(t, out.String(), `"single":schema.SingleNestedAttribute`)
	assertContains(t, out.String(), "objectplanmodifier.UseStateForUnknown()")
	assertContains(t, out.String(), "listplanmodifier.UseNonNullStateForUnknown()")

	if !slicesContain(features.FrameworkPlanModifierPackages, "stringplanmodifier") {
		t.Fatalf("expected stringplanmodifier package in features: %#v", features.FrameworkPlanModifierPackages)
	}
}

func TestEmitRootPropertiesSchemaDataSourceOmitsPlanModifiers(t *testing.T) {
	t.Parallel()

	out := &strings.Builder{}
	_, err := Emitter{
		CfResource:   syntheticPlanModifierResource(),
		IsDataSource: true,
		Ui:           cli.NewMockUi(),
		Writer:       out,
	}.EmitRootPropertiesSchema("volcenginecc_test_resource", map[string]string{})
	if err != nil {
		t.Fatalf("EmitRootPropertiesSchema() error = %s", err)
	}

	if strings.Contains(out.String(), "UseStateForUnknown()") {
		t.Fatalf("data source schema unexpectedly emitted UseStateForUnknown():\n%s", out.String())
	}
	if strings.Contains(out.String(), "UseNonNullStateForUnknown()") {
		t.Fatalf("data source schema unexpectedly emitted UseNonNullStateForUnknown():\n%s", out.String())
	}
}

func syntheticPlanModifierResource() *ccschema.Resource {
	stringType := ccschema.Type(ccschema.PropertyTypeString)
	boolType := ccschema.Type(ccschema.PropertyTypeBoolean)
	arrayType := ccschema.Type(ccschema.PropertyTypeArray)
	objectType := ccschema.Type(ccschema.PropertyTypeObject)
	uniqueItems := true
	insertionOrder := false

	return &ccschema.Resource{
		Properties: map[string]*ccschema.Property{
			"TopComputed": {
				Type: &stringType,
			},
			"Items": {
				Type:        &arrayType,
				UniqueItems: &uniqueItems,
				Items: &ccschema.Property{
					Type: &objectType,
					Properties: map[string]*ccschema.Property{
						"ChildComputed": {
							Type: &stringType,
						},
						"ChildFlag": {
							Type: &boolType,
						},
					},
				},
			},
			"SetItems": {
				Type:           &arrayType,
				UniqueItems:    &uniqueItems,
				InsertionOrder: &insertionOrder,
				Items: &ccschema.Property{
					Type: &objectType,
					Properties: map[string]*ccschema.Property{
						"SetChildComputed": {
							Type: &stringType,
						},
					},
				},
			},
			"Single": {
				Type: &objectType,
				Properties: map[string]*ccschema.Property{
					"InnerList": {
						Type: &arrayType,
						Items: &ccschema.Property{
							Type: &stringType,
						},
					},
					"InnerName": {
						Type: &stringType,
					},
				},
			},
		},
	}
}

func assertContains(t *testing.T, s, substr string) {
	t.Helper()

	if !strings.Contains(s, substr) {
		t.Fatalf("expected output to contain %q:\n%s", substr, s)
	}
}

func slicesContain(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}
