package shared

import (
	"reflect"
	"testing"

	"github.com/volcengine/terraform-provider-volcenginecc/internal/ccschema"
)

func TestCollectCollectionIdentities(t *testing.T) {
	t.Parallel()

	arrayType := ccschema.Type(ccschema.PropertyTypeArray)
	objectType := ccschema.Type(ccschema.PropertyTypeObject)
	falseValue := false
	trueValue := true

	properties := map[string]*ccschema.Property{
		"Rules": {
			Type:              &arrayType,
			InsertionOrder:    &falseValue,
			UniqueItems:       &trueValue,
			ElementIdentifier: []string{"/Name", "/Scope"},
			Items:             &ccschema.Property{Type: &objectType},
		},
		"Parent": {
			Type: &objectType,
			Properties: map[string]*ccschema.Property{
				"Children": {
					Type:              &arrayType,
					InsertionOrder:    &falseValue,
					ElementIdentifier: []string{"/Id"},
					Items:             &ccschema.Property{Type: &objectType},
				},
			},
		},
		"Legacy": {
			Type:  &arrayType,
			Items: &ccschema.Property{Type: &objectType},
		},
		"Groups": {
			Type:              &arrayType,
			ElementIdentifier: []string{"/Name"},
			Items: &ccschema.Property{
				Type: &objectType,
				Properties: map[string]*ccschema.Property{
					"Members": {
						Type:              &arrayType,
						ElementIdentifier: []string{"/Id"},
						Items:             &ccschema.Property{Type: &objectType},
					},
				},
			},
		},
	}

	got := collectCollectionIdentities(properties, nil)
	want := []CollectionIdentity{
		{
			PropertyPath:    "/Groups/*/Members",
			IdentifierPaths: []string{"/Id"},
			UniqueItems:     false,
		},
		{
			PropertyPath:    "/Groups",
			IdentifierPaths: []string{"/Name"},
			UniqueItems:     false,
		},
		{
			PropertyPath:    "/Parent/Children",
			IdentifierPaths: []string{"/Id"},
			UniqueItems:     false,
		},
		{
			PropertyPath:    "/Rules",
			IdentifierPaths: []string{"/Name", "/Scope"},
			UniqueItems:     true,
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("collectCollectionIdentities() = %#v, want %#v", got, want)
	}
}
