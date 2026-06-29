package generic

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestAlignTerraformCollectionState(t *testing.T) {
	t.Parallel()

	elementType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"identity": tftypes.String,
		"zone":     tftypes.String,
		"readback": tftypes.String,
	}}
	collectionType := tftypes.List{ElementType: elementType}
	identifiers := [][]string{{"identity"}}

	testCases := map[string]struct {
		prior  []tftypes.Value
		remote []tftypes.Value
		want   []tftypes.Value
	}{
		"remote order follows prior identity order": {
			prior: []tftypes.Value{
				terraformReadElement(elementType, "first", "a", "old-first"),
				terraformReadElement(elementType, "second", "a", "old-second"),
			},
			remote: []tftypes.Value{
				terraformReadElement(elementType, "second", "a", "new-second"),
				terraformReadElement(elementType, "first", "a", "new-first"),
			},
			want: []tftypes.Value{
				terraformReadElement(elementType, "first", "a", "new-first"),
				terraformReadElement(elementType, "second", "a", "new-second"),
			},
		},
		"remote new elements are appended": {
			prior: []tftypes.Value{
				terraformReadElement(elementType, "first", "a", "old-first"),
			},
			remote: []tftypes.Value{
				terraformReadElement(elementType, "new", "a", "new-readback"),
				terraformReadElement(elementType, "first", "a", "new-first"),
			},
			want: []tftypes.Value{
				terraformReadElement(elementType, "first", "a", "new-first"),
				terraformReadElement(elementType, "new", "a", "new-readback"),
			},
		},
		"remote missing prior elements are omitted": {
			prior: []tftypes.Value{
				terraformReadElement(elementType, "deleted", "a", "old-deleted"),
				terraformReadElement(elementType, "first", "a", "old-first"),
			},
			remote: []tftypes.Value{
				terraformReadElement(elementType, "first", "a", "new-first"),
			},
			want: []tftypes.Value{
				terraformReadElement(elementType, "first", "a", "new-first"),
			},
		},
		"compound identity order is preserved": {
			prior: []tftypes.Value{
				terraformReadElement(elementType, "same", "a", "old-a"),
				terraformReadElement(elementType, "same", "b", "old-b"),
			},
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
			if name == "compound identity order is preserved" {
				testIdentifiers = [][]string{{"identity"}, {"zone"}}
			}
			got, err := alignTerraformCollectionState(
				tftypes.NewValue(collectionType, testCase.prior),
				tftypes.NewValue(collectionType, testCase.remote),
				testIdentifiers,
			)
			if err != nil {
				t.Fatalf("alignTerraformCollectionState() error = %v", err)
			}
			want := tftypes.NewValue(collectionType, testCase.want)
			if !got.Equal(want) {
				t.Fatalf("alignTerraformCollectionState() = %s, want %s", got, want)
			}
		})
	}
}

func TestAlignTerraformCollectionStateRejectsUnsafeIdentity(t *testing.T) {
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

	_, err := alignTerraformCollectionState(duplicateRemote, duplicateRemote, [][]string{{"identity"}})
	if err == nil {
		t.Fatal("alignTerraformCollectionState() expected duplicate identity error")
	}
}

func TestAlignTerraformCollectionStateLeavesNullAndUnknownCollections(t *testing.T) {
	t.Parallel()

	elementType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"identity": tftypes.String,
		"zone":     tftypes.String,
		"readback": tftypes.String,
	}}
	collectionType := tftypes.List{ElementType: elementType}
	remote := tftypes.NewValue(collectionType, []tftypes.Value{
		terraformReadElement(elementType, "first", "a", "new-first"),
	})
	for name, prior := range map[string]tftypes.Value{
		"null prior":    tftypes.NewValue(collectionType, nil),
		"unknown prior": tftypes.NewValue(collectionType, tftypes.UnknownValue),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := alignTerraformCollectionState(prior, remote, [][]string{{"identity"}})
			if err != nil {
				t.Fatalf("alignTerraformCollectionState() error = %v", err)
			}
			if !got.Equal(remote) {
				t.Fatalf("alignTerraformCollectionState() = %s, want remote %s", got, remote)
			}
		})
	}
}

func terraformReadElement(elementType tftypes.Object, identity string, zone string, readback string) tftypes.Value {
	return tftypes.NewValue(elementType, map[string]tftypes.Value{
		"identity": tftypes.NewValue(tftypes.String, identity),
		"zone":     tftypes.NewValue(tftypes.String, zone),
		"readback": tftypes.NewValue(tftypes.String, readback),
	})
}
