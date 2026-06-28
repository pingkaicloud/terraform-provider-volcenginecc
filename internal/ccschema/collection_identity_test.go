package ccschema

import (
	"strings"
	"testing"
)

func TestResourceJsonSchemaElementIdentifierParsing(t *testing.T) {
	t.Parallel()

	schema, err := NewResourceJsonSchemaDocument(`{
		"typeName":"Volcengine::Test::Identity",
		"properties":{
			"Rules":{
				"type":"array",
				"insertionOrder":false,
				"elementIdentifier":["/Name","/Scope/Region"],
				"items":{"type":"object","properties":{
					"Name":{"type":"string"},
					"Scope":{"type":"object","properties":{"Region":{"type":"string"}}}
				}}
			},
			"Legacy":{"type":"array","items":{"type":"string"}}
		}
	}`)
	if err != nil {
		t.Fatalf("NewResourceJsonSchemaDocument() error = %v", err)
	}

	resource, err := schema.Resource()
	if err != nil {
		t.Fatalf("Resource() error = %v", err)
	}

	got := resource.Properties["Rules"].ElementIdentifier
	if len(got) != 2 || got[0] != "/Name" || got[1] != "/Scope/Region" {
		t.Fatalf("ElementIdentifier = %#v", got)
	}
	if resource.Properties["Legacy"].ElementIdentifier != nil {
		t.Fatalf("legacy ElementIdentifier = %#v, want nil", resource.Properties["Legacy"].ElementIdentifier)
	}
}

func TestResourceValidateCollectionIdentities(t *testing.T) {
	t.Parallel()

	falseValue := false
	trueValue := true
	arrayType := Type(PropertyTypeArray)
	objectType := Type(PropertyTypeObject)
	stringType := Type(PropertyTypeString)

	validProperty := func() *Property {
		return &Property{
			Type:              &arrayType,
			InsertionOrder:    &falseValue,
			ElementIdentifier: []string{"/Name"},
			Items: &Property{
				Type: &objectType,
				Properties: map[string]*Property{
					"Name": {Type: &stringType},
				},
			},
		}
	}

	tests := map[string]struct {
		mutate  func(*Resource, *Property)
		wantErr string
	}{
		"valid single identity": {},
		"valid composite identity": {
			mutate: func(_ *Resource, property *Property) {
				property.ElementIdentifier = []string{"/Name", "/Scope/Region"}
				property.Items.Properties["Scope"] = &Property{
					Type: &objectType,
					Properties: map[string]*Property{
						"Region": {Type: &stringType},
					},
				}
			},
		},
		"non array": {
			mutate:  func(_ *Resource, property *Property) { property.Type = &objectType },
			wantErr: "only valid for type=array",
		},
		"ordered array": {
			mutate:  func(_ *Resource, property *Property) { property.InsertionOrder = &trueValue },
			wantErr: "requires insertionOrder=false",
		},
		"non object items": {
			mutate:  func(_ *Resource, property *Property) { property.Items.Type = &stringType },
			wantErr: "requires items to resolve to type=object",
		},
		"missing pointer": {
			mutate:  func(_ *Resource, property *Property) { property.ElementIdentifier = []string{"/Missing"} },
			wantErr: `does not resolve at segment "Missing"`,
		},
		"invalid pointer": {
			mutate:  func(_ *Resource, property *Property) { property.ElementIdentifier = []string{"Name"} },
			wantErr: "must be a non-empty JSON Pointer",
		},
		"duplicate pointer": {
			mutate:  func(_ *Resource, property *Property) { property.ElementIdentifier = []string{"/Name", "/Name"} },
			wantErr: "is duplicated",
		},
		"read only identity": {
			mutate: func(resource *Resource, _ *Property) {
				resource.ReadOnlyProperties = PropertyJsonPointers{"/properties/Rules/*/Name"}
			},
			wantErr: "must not reference a readOnly property",
		},
		"write only identity": {
			mutate: func(resource *Resource, _ *Property) {
				resource.WriteOnlyProperties = PropertyJsonPointers{"/properties/Rules/*/Name"}
			},
			wantErr: "must not reference a writeOnly property",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			typeName := "Volcengine::Test::Identity"
			property := validProperty()
			resource := &Resource{
				TypeName:   &typeName,
				Properties: map[string]*Property{"Rules": property},
			}
			if test.mutate != nil {
				test.mutate(resource, property)
			}

			err := resource.ValidateCollectionIdentities()
			if test.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateCollectionIdentities() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("ValidateCollectionIdentities() error = %v, want containing %q", err, test.wantErr)
			}
			if !strings.Contains(err.Error(), typeName) || !strings.Contains(err.Error(), "/properties/Rules") {
				t.Fatalf("ValidateCollectionIdentities() error lacks resource/path context: %v", err)
			}
		})
	}
}
