package generic

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestCanonicalizeTerraformCollectionState(t *testing.T) {
	t.Parallel()

	elementType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"identity": tftypes.String,
		"zone":     tftypes.String,
		"readback": tftypes.String,
	}}
	collectionType := tftypes.List{ElementType: elementType}
	identifiers := [][]string{{"identity"}}

	testCases := map[string]struct {
		remote []tftypes.Value
		want   []tftypes.Value
	}{
		"remote order follows canonical identity order": {
			remote: []tftypes.Value{
				terraformReadElement(elementType, "second", "a", "new-second"),
				terraformReadElement(elementType, "first", "a", "new-first"),
			},
			want: []tftypes.Value{
				terraformReadElement(elementType, "first", "a", "new-first"),
				terraformReadElement(elementType, "second", "a", "new-second"),
			},
		},
		"remote new elements use canonical position": {
			remote: []tftypes.Value{
				terraformReadElement(elementType, "new", "a", "new-readback"),
				terraformReadElement(elementType, "first", "a", "new-first"),
			},
			want: []tftypes.Value{
				terraformReadElement(elementType, "first", "a", "new-first"),
				terraformReadElement(elementType, "new", "a", "new-readback"),
			},
		},
		"single remote element is preserved": {
			remote: []tftypes.Value{
				terraformReadElement(elementType, "first", "a", "new-first"),
			},
			want: []tftypes.Value{
				terraformReadElement(elementType, "first", "a", "new-first"),
			},
		},
		"compound identity uses declared component order": {
			remote: []tftypes.Value{
				terraformReadElement(elementType, "same", "b", "new-b"),
				terraformReadElement(elementType, "same", "a", "new-a"),
			},
			want: []tftypes.Value{
				terraformReadElement(elementType, "same", "a", "new-a"),
				terraformReadElement(elementType, "same", "b", "new-b"),
			},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			testIdentifiers := identifiers
			if name == "compound identity uses declared component order" {
				testIdentifiers = [][]string{{"identity"}, {"zone"}}
			}
			got, err := canonicalizeTerraformCollectionState(
				tftypes.NewValue(collectionType, testCase.remote),
				testIdentifiers,
			)
			if err != nil {
				t.Fatalf("canonicalizeTerraformCollectionState() error = %v", err)
			}
			want := tftypes.NewValue(collectionType, testCase.want)
			if !got.Equal(want) {
				t.Fatalf("canonicalizeTerraformCollectionState() = %s, want %s", got, want)
			}
		})
	}
}

func TestCanonicalizeTerraformCollectionStateRejectsUnsafeIdentity(t *testing.T) {
	t.Parallel()

	elementType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"identity": tftypes.String,
		"zone":     tftypes.String,
		"readback": tftypes.String,
	}}
	collectionType := tftypes.List{ElementType: elementType}
	duplicateRemote := tftypes.NewValue(collectionType, []tftypes.Value{
		terraformReadElement(elementType, "same", "a", "first"),
		terraformReadElement(elementType, "same", "a", "second"),
	})

	_, err := canonicalizeTerraformCollectionState(duplicateRemote, [][]string{{"identity"}})
	if err == nil {
		t.Fatal("canonicalizeTerraformCollectionState() expected duplicate identity error")
	}
}

func TestCanonicalizeTerraformCollectionStateLeavesNullAndUnknownCollections(t *testing.T) {
	t.Parallel()

	elementType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"identity": tftypes.String,
		"zone":     tftypes.String,
		"readback": tftypes.String,
	}}
	collectionType := tftypes.List{ElementType: elementType}
	for name, remote := range map[string]tftypes.Value{
		"null remote":    tftypes.NewValue(collectionType, nil),
		"unknown remote": tftypes.NewValue(collectionType, tftypes.UnknownValue),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := canonicalizeTerraformCollectionState(remote, [][]string{{"identity"}})
			if err != nil {
				t.Fatalf("canonicalizeTerraformCollectionState() error = %v", err)
			}
			if !got.Equal(remote) {
				t.Fatalf("canonicalizeTerraformCollectionState() = %s, want remote %s", got, remote)
			}
		})
	}
}

func TestCanonicalizeTerraformCollectionStateScalarTypes(t *testing.T) {
	t.Parallel()

	elementType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"number":  tftypes.Number,
		"enabled": tftypes.Bool,
	}}
	collectionType := tftypes.List{ElementType: elementType}
	element := func(number int, enabled bool) tftypes.Value {
		return tftypes.NewValue(elementType, map[string]tftypes.Value{
			"number":  tftypes.NewValue(tftypes.Number, number),
			"enabled": tftypes.NewValue(tftypes.Bool, enabled),
		})
	}
	remote := tftypes.NewValue(collectionType, []tftypes.Value{
		element(10, false),
		element(2, true),
		element(2, false),
	})
	got, err := canonicalizeTerraformCollectionState(remote, [][]string{{"number"}, {"enabled"}})
	if err != nil {
		t.Fatalf("canonicalizeTerraformCollectionState() error = %v", err)
	}
	want := tftypes.NewValue(collectionType, []tftypes.Value{
		element(2, false),
		element(2, true),
		element(10, false),
	})
	if !got.Equal(want) {
		t.Fatalf("canonicalizeTerraformCollectionState() = %s, want %s", got, want)
	}
}

func terraformReadElement(elementType tftypes.Object, identity string, zone string, readback string) tftypes.Value {
	return tftypes.NewValue(elementType, map[string]tftypes.Value{
		"identity": tftypes.NewValue(tftypes.String, identity),
		"zone":     tftypes.NewValue(tftypes.String, zone),
		"readback": tftypes.NewValue(tftypes.String, readback),
	})
}
