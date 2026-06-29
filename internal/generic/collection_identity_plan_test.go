package generic

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestMergeTerraformCollectionPlan(t *testing.T) {
	t.Parallel()

	elementType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"identity": tftypes.String,
		"computed": tftypes.String,
		"writable": tftypes.String,
	}}
	identifiers := [][]string{{"identity"}}
	computedFields := map[string]bool{"computed": true}

	testCases := map[string]struct {
		collectionType tftypes.Type
		config         []tftypes.Value
		prior          []tftypes.Value
		planned        []tftypes.Value
		want           []tftypes.Value
	}{
		"set restores omitted unknown computed field by identity": {
			collectionType: tftypes.Set{ElementType: elementType},
			config: []tftypes.Value{
				terraformPlanElement(elementType, "second", nil, "config-second"),
				terraformPlanElement(elementType, "first", nil, "config-first"),
			},
			prior: []tftypes.Value{
				terraformPlanElement(elementType, "first", nil, "old-first"),
				terraformPlanElement(elementType, "second", "readback", "old-second"),
			},
			planned: []tftypes.Value{
				terraformPlanElement(elementType, "second", tftypes.UnknownValue, "new-second"),
				terraformPlanElement(elementType, "first", tftypes.UnknownValue, "new-first"),
			},
			want: []tftypes.Value{
				terraformPlanElement(elementType, "second", "readback", "new-second"),
				terraformPlanElement(elementType, "first", nil, "new-first"),
			},
		},
		"multiset list preserves explicit and ordinary writable changes": {
			collectionType: tftypes.List{ElementType: elementType},
			config: []tftypes.Value{
				terraformPlanElement(elementType, "first", "explicit", "new"),
			},
			prior: []tftypes.Value{
				terraformPlanElement(elementType, "first", "prior", "old"),
			},
			planned: []tftypes.Value{
				terraformPlanElement(elementType, "first", "explicit", "new"),
			},
			want: []tftypes.Value{
				terraformPlanElement(elementType, "first", "explicit", "new"),
			},
		},
		"multiset list can read config omission when create-only identity is read back": {
			collectionType: tftypes.List{ElementType: elementType},
			config: []tftypes.Value{
				terraformPlanElement(elementType, nil, nil, "config"),
			},
			prior: []tftypes.Value{
				terraformPlanElement(elementType, "same", nil, "old"),
			},
			planned: []tftypes.Value{
				terraformPlanElement(elementType, "same", tftypes.UnknownValue, "config"),
			},
			want: []tftypes.Value{
				terraformPlanElement(elementType, "same", nil, "config"),
			},
		},
		"identity change remains a removal and addition": {
			collectionType: tftypes.Set{ElementType: elementType},
			config: []tftypes.Value{
				terraformPlanElement(elementType, "new", nil, "value"),
			},
			prior: []tftypes.Value{
				terraformPlanElement(elementType, "old", "prior", "value"),
			},
			planned: []tftypes.Value{
				terraformPlanElement(elementType, "new", tftypes.UnknownValue, "value"),
			},
			want: []tftypes.Value{
				terraformPlanElement(elementType, "new", tftypes.UnknownValue, "value"),
			},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := mergeTerraformCollectionPlan(
				tftypes.NewValue(testCase.collectionType, testCase.config),
				tftypes.NewValue(testCase.collectionType, testCase.prior),
				tftypes.NewValue(testCase.collectionType, testCase.planned),
				identifiers,
				computedFields,
			)
			if err != nil {
				t.Fatalf("mergeTerraformCollectionPlan() error = %v", err)
			}
			want := tftypes.NewValue(testCase.collectionType, testCase.want)
			if !got.Equal(want) {
				t.Fatalf("mergeTerraformCollectionPlan() = %s, want %s", got, want)
			}
		})
	}
}

func TestMergeTerraformCollectionPlanUnsafeIdentityPreservesPlan(t *testing.T) {
	t.Parallel()

	elementType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"identity": tftypes.String,
		"computed": tftypes.String,
	}}
	collectionType := tftypes.Set{ElementType: elementType}
	element := tftypes.NewValue(elementType, map[string]tftypes.Value{
		"identity": tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
		"computed": tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
	})
	planned := tftypes.NewValue(collectionType, []tftypes.Value{element})

	got, err := mergeTerraformCollectionPlan(planned, planned, planned, [][]string{{"identity"}}, map[string]bool{"computed": true})
	if err == nil {
		t.Fatal("mergeTerraformCollectionPlan() expected unsafe identity error")
	}
	if !got.Equal(planned) {
		t.Fatalf("mergeTerraformCollectionPlan() changed unsafe plan: %s", got)
	}
}

func terraformPlanElement(elementType tftypes.Object, identity interface{}, computed, writable interface{}) tftypes.Value {
	return tftypes.NewValue(elementType, map[string]tftypes.Value{
		"identity": tftypes.NewValue(tftypes.String, identity),
		"computed": tftypes.NewValue(tftypes.String, computed),
		"writable": tftypes.NewValue(tftypes.String, writable),
	})
}
