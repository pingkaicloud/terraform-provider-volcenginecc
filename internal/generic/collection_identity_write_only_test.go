package generic

import (
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestRestoreTerraformCollectionWriteOnlyValues(t *testing.T) {
	t.Parallel()

	elementType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"identity": tftypes.String,
		"secret":   tftypes.String,
		"value":    tftypes.String,
	}}
	collectionType := tftypes.List{ElementType: elementType}
	identifiers := [][]string{{"identity"}}
	writeOnlyPaths := [][]string{{"secret"}}

	testCases := map[string]struct {
		prior  []tftypes.Value
		remote []tftypes.Value
		want   []tftypes.Value
	}{
		"remote order copies by identity": {
			prior: []tftypes.Value{
				terraformWriteOnlyElement(elementType, "first", "secret-first", "old-first"),
				terraformWriteOnlyElement(elementType, "second", "secret-second", "old-second"),
			},
			remote: []tftypes.Value{
				terraformWriteOnlyElement(elementType, "second", nil, "new-second"),
				terraformWriteOnlyElement(elementType, "first", nil, "new-first"),
			},
			want: []tftypes.Value{
				terraformWriteOnlyElement(elementType, "first", "secret-first", "new-first"),
				terraformWriteOnlyElement(elementType, "second", "secret-second", "new-second"),
			},
		},
		"remote new element does not inherit prior secret": {
			prior: []tftypes.Value{
				terraformWriteOnlyElement(elementType, "first", "secret-first", "old-first"),
			},
			remote: []tftypes.Value{
				terraformWriteOnlyElement(elementType, "first", nil, "new-first"),
				terraformWriteOnlyElement(elementType, "new", nil, "new-value"),
			},
			want: []tftypes.Value{
				terraformWriteOnlyElement(elementType, "first", "secret-first", "new-first"),
				terraformWriteOnlyElement(elementType, "new", nil, "new-value"),
			},
		},
		"remote missing prior element leaves no residual secret": {
			prior: []tftypes.Value{
				terraformWriteOnlyElement(elementType, "deleted", "secret-deleted", "old-deleted"),
				terraformWriteOnlyElement(elementType, "first", "secret-first", "old-first"),
			},
			remote: []tftypes.Value{
				terraformWriteOnlyElement(elementType, "first", nil, "new-first"),
			},
			want: []tftypes.Value{
				terraformWriteOnlyElement(elementType, "first", "secret-first", "new-first"),
			},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := restoreTerraformCollectionWriteOnlyValues(
				tftypes.NewValue(collectionType, testCase.prior),
				tftypes.NewValue(collectionType, testCase.remote),
				identifiers,
				writeOnlyPaths,
			)
			if err != nil {
				t.Fatalf("restoreTerraformCollectionWriteOnlyValues() error = %v", err)
			}
			want := tftypes.NewValue(collectionType, testCase.want)
			if !got.Equal(want) {
				t.Fatalf("restoreTerraformCollectionWriteOnlyValues() = %s, want %s", got, want)
			}
		})
	}
}

func TestRestoreTerraformCollectionWriteOnlyValuesRejectsDuplicateIdentity(t *testing.T) {
	t.Parallel()

	elementType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"identity": tftypes.String,
		"secret":   tftypes.String,
		"value":    tftypes.String,
	}}
	collectionType := tftypes.List{ElementType: elementType}
	duplicate := tftypes.NewValue(collectionType, []tftypes.Value{
		terraformWriteOnlyElement(elementType, "same", nil, "first"),
		terraformWriteOnlyElement(elementType, "same", nil, "second"),
	})

	_, err := restoreTerraformCollectionWriteOnlyValues(duplicate, duplicate, [][]string{{"identity"}}, [][]string{{"secret"}})
	if err == nil {
		t.Fatal("restoreTerraformCollectionWriteOnlyValues() expected duplicate identity error")
	}
}

func TestRestoreTerraformCollectionWriteOnlyValuesRejectsIncompleteIdentity(t *testing.T) {
	t.Parallel()

	elementType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"identity": tftypes.String,
		"secret":   tftypes.String,
		"value":    tftypes.String,
	}}
	collectionType := tftypes.List{ElementType: elementType}

	tests := map[string]struct {
		element   tftypes.Value
		wantError string
	}{
		"null identity": {
			element:   terraformWriteOnlyElement(elementType, "", nil, "value"),
			wantError: "identity attribute is null or unknown",
		},
		"unknown identity": {
			element: tftypes.NewValue(elementType, map[string]tftypes.Value{
				"identity": tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
				"secret":   tftypes.NewValue(tftypes.String, nil),
				"value":    tftypes.NewValue(tftypes.String, "value"),
			}),
			wantError: "identity attribute is null or unknown",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			collection := tftypes.NewValue(collectionType, []tftypes.Value{test.element})
			_, err := restoreTerraformCollectionWriteOnlyValues(collection, collection, [][]string{{"identity"}}, [][]string{{"secret"}})
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("restoreTerraformCollectionWriteOnlyValues() error = %v, want containing %q", err, test.wantError)
			}
		})
	}
}

func TestRestoreTerraformCollectionWriteOnlyValuesRejectsMissingIdentity(t *testing.T) {
	t.Parallel()

	elementType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"secret": tftypes.String,
		"value":  tftypes.String,
	}}
	collectionType := tftypes.List{ElementType: elementType}
	collection := tftypes.NewValue(collectionType, []tftypes.Value{
		tftypes.NewValue(elementType, map[string]tftypes.Value{
			"secret": tftypes.NewValue(tftypes.String, nil),
			"value":  tftypes.NewValue(tftypes.String, "value"),
		}),
	})

	_, err := restoreTerraformCollectionWriteOnlyValues(collection, collection, [][]string{{"identity"}}, [][]string{{"secret"}})
	if err == nil || !strings.Contains(err.Error(), `identity attribute "identity" is missing`) {
		t.Fatalf("restoreTerraformCollectionWriteOnlyValues() error = %v, want missing identity", err)
	}
}

func TestIdentityCollectionWriteOnlyAttributeNames(t *testing.T) {
	t.Parallel()

	resource := &genericResource{
		ccToTfNameMap: map[string]string{
			"Items":  "items",
			"Secret": "secret",
			"Value":  "value",
		},
		writeOnlyPropertyPaths: []string{
			"/properties/Items/*/Secret",
			"/properties/Value",
		},
		collectionIdentities: []CollectionIdentity{{
			PropertyPath:    "/Items",
			IdentifierPaths: []string{"/Value"},
		}},
	}

	if !resource.writeOnlyPathBelongsToIdentityCollection("/properties/Items/*/Secret") {
		t.Fatal("writeOnlyPathBelongsToIdentityCollection() = false, want true")
	}
	if resource.writeOnlyPathBelongsToIdentityCollection("/properties/Value") {
		t.Fatal("writeOnlyPathBelongsToIdentityCollection() = true, want false")
	}

	got, err := resource.identityCollectionWriteOnlyAttributeNames("/Items")
	if err != nil {
		t.Fatalf("identityCollectionWriteOnlyAttributeNames() error = %v", err)
	}
	want := [][]string{{"secret"}}
	if len(got) != len(want) || len(got[0]) != len(want[0]) || got[0][0] != want[0][0] {
		t.Fatalf("identityCollectionWriteOnlyAttributeNames() = %#v, want %#v", got, want)
	}
}

func terraformWriteOnlyElement(elementType tftypes.Object, identity string, secret interface{}, value string) tftypes.Value {
	return tftypes.NewValue(elementType, map[string]tftypes.Value{
		"identity": tftypes.NewValue(tftypes.String, identityValue(identity)),
		"secret":   tftypes.NewValue(tftypes.String, secret),
		"value":    tftypes.NewValue(tftypes.String, value),
	})
}

func identityValue(identity string) interface{} {
	if identity == "" {
		return nil
	}
	return identity
}
