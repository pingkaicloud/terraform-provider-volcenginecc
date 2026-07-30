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

func TestCanonicalizeTerraformCollectionsAtNestedArrayPath(t *testing.T) {
	t.Parallel()

	memberType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"id": tftypes.String,
	}}
	membersType := tftypes.List{ElementType: memberType}
	groupType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"name":    tftypes.String,
		"members": membersType,
	}}
	groupsType := tftypes.List{ElementType: groupType}
	rootType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"groups": groupsType,
	}}
	member := func(id string) tftypes.Value {
		return tftypes.NewValue(memberType, map[string]tftypes.Value{
			"id": tftypes.NewValue(tftypes.String, id),
		})
	}
	group := func(name string, members ...tftypes.Value) tftypes.Value {
		return tftypes.NewValue(groupType, map[string]tftypes.Value{
			"name":    tftypes.NewValue(tftypes.String, name),
			"members": tftypes.NewValue(membersType, members),
		})
	}
	root := tftypes.NewValue(rootType, map[string]tftypes.Value{
		"groups": tftypes.NewValue(groupsType, []tftypes.Value{
			group("z", member("2"), member("1")),
			group("a", member("4"), member("3")),
		}),
	})

	resource := genericResource{
		ccToTfNameMap: map[string]string{
			"Groups":  "groups",
			"Name":    "name",
			"Members": "members",
			"Id":      "id",
		},
		collectionIdentities: []CollectionIdentity{
			{PropertyPath: "/Groups", IdentifierPaths: []string{"/Name"}},
			{PropertyPath: "/Groups/*/Members", IdentifierPaths: []string{"/Id"}},
		},
	}
	got, err := resource.canonicalizeIdentityCollectionState(root)
	if err != nil {
		t.Fatalf("canonicalizeIdentityCollectionState() error = %v", err)
	}
	want := tftypes.NewValue(rootType, map[string]tftypes.Value{
		"groups": tftypes.NewValue(groupsType, []tftypes.Value{
			group("a", member("3"), member("4")),
			group("z", member("1"), member("2")),
		}),
	})
	if !got.Equal(want) {
		t.Fatalf("canonicalizeIdentityCollectionState() = %s, want %s", got, want)
	}
}

func TestCanonicalizeTerraformNestedPlanDegradesOnlyUnsafeLeaf(t *testing.T) {
	t.Parallel()

	memberType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{"id": tftypes.String}}
	membersType := tftypes.List{ElementType: memberType}
	groupType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{"members": membersType}}
	groupsType := tftypes.List{ElementType: groupType}
	rootType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{"groups": groupsType}}
	member := func(id interface{}) tftypes.Value {
		return tftypes.NewValue(memberType, map[string]tftypes.Value{
			"id": tftypes.NewValue(tftypes.String, id),
		})
	}
	group := func(members ...tftypes.Value) tftypes.Value {
		return tftypes.NewValue(groupType, map[string]tftypes.Value{
			"members": tftypes.NewValue(membersType, members),
		})
	}
	root := tftypes.NewValue(rootType, map[string]tftypes.Value{
		"groups": tftypes.NewValue(groupsType, []tftypes.Value{
			group(member(tftypes.UnknownValue), member("a")),
			group(member("d"), member("c")),
		}),
	})

	got, err := canonicalizeTerraformCollectionsAtPath(
		root,
		[]string{"groups", "*", "members"},
		[][]string{{"id"}},
		true,
	)
	if err != nil {
		t.Fatalf("canonicalizeTerraformCollectionsAtPath() error = %v", err)
	}
	want := tftypes.NewValue(rootType, map[string]tftypes.Value{
		"groups": tftypes.NewValue(groupsType, []tftypes.Value{
			group(member(tftypes.UnknownValue), member("a")),
			group(member("c"), member("d")),
		}),
	})
	if !got.Equal(want) {
		t.Fatalf("canonicalizeTerraformCollectionsAtPath() = %s, want %s", got, want)
	}
}

func terraformReadElement(elementType tftypes.Object, identity string, zone string, readback string) tftypes.Value {
	return tftypes.NewValue(elementType, map[string]tftypes.Value{
		"identity": tftypes.NewValue(tftypes.String, identity),
		"zone":     tftypes.NewValue(tftypes.String, zone),
		"readback": tftypes.NewValue(tftypes.String, readback),
	})
}
