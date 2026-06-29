package generic

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	frameworkprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	providerschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestPlanResourceChangeMergesIdentityCollectionComputedUnknown(t *testing.T) {
	t.Parallel()

	elementType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"identity": tftypes.String,
		"computed": tftypes.String,
		"writable": tftypes.String,
	}}
	resourceType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"id":    tftypes.String,
		"items": tftypes.Set{ElementType: elementType},
	}}
	collectionType := tftypes.Set{ElementType: elementType}

	config := tftypes.NewValue(resourceType, map[string]tftypes.Value{
		"id": tftypes.NewValue(tftypes.String, nil),
		"items": tftypes.NewValue(collectionType, []tftypes.Value{
			terraformPlanElement(elementType, "second", nil, "new-second"),
			terraformPlanElement(elementType, "first", nil, "new-first"),
		}),
	})
	prior := tftypes.NewValue(resourceType, map[string]tftypes.Value{
		"id": tftypes.NewValue(tftypes.String, "resource-id"),
		"items": tftypes.NewValue(collectionType, []tftypes.Value{
			terraformPlanElement(elementType, "first", nil, "old-first"),
			terraformPlanElement(elementType, "second", "readback", "old-second"),
		}),
	})
	proposed := tftypes.NewValue(resourceType, map[string]tftypes.Value{
		"id": tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
		"items": tftypes.NewValue(collectionType, []tftypes.Value{
			terraformPlanElement(elementType, "second", tftypes.UnknownValue, "new-second"),
			terraformPlanElement(elementType, "first", tftypes.UnknownValue, "new-first"),
		}),
	})

	server := providerserver.NewProtocol6(identityPlanTestProvider{})()
	response, err := server.PlanResourceChange(context.Background(), &tfprotov6.PlanResourceChangeRequest{
		TypeName:         "test_identity_resource",
		Config:           identityPlanDynamicValue(t, resourceType, config),
		PriorState:       identityPlanDynamicValue(t, resourceType, prior),
		ProposedNewState: identityPlanDynamicValue(t, resourceType, proposed),
	})
	if err != nil {
		t.Fatalf("PlanResourceChange() error = %v", err)
	}
	for _, diagnostic := range response.Diagnostics {
		if diagnostic.Severity == tfprotov6.DiagnosticSeverityError {
			t.Fatalf("PlanResourceChange() diagnostic = %s: %s", diagnostic.Summary, diagnostic.Detail)
		}
	}

	got, err := response.PlannedState.Unmarshal(resourceType)
	if err != nil {
		t.Fatalf("unmarshalling planned state: %v", err)
	}
	want := tftypes.NewValue(resourceType, map[string]tftypes.Value{
		"id": tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
		"items": tftypes.NewValue(collectionType, []tftypes.Value{
			terraformPlanElement(elementType, "second", "readback", "new-second"),
			terraformPlanElement(elementType, "first", nil, "new-first"),
		}),
	})
	if !got.Equal(want) {
		t.Fatalf("PlanResourceChange() planned state = %s, want %s", got, want)
	}
}

func TestPlanResourceChangePreservesConfiguredCollectionChanges(t *testing.T) {
	t.Parallel()

	elementType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"identity": tftypes.String,
		"computed": tftypes.String,
		"writable": tftypes.String,
	}}
	collectionType := tftypes.Set{ElementType: elementType}
	resourceType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"id":    tftypes.String,
		"items": collectionType,
	}}

	testCases := map[string]struct {
		configElement  tftypes.Value
		priorElement   tftypes.Value
		plannedElement tftypes.Value
		wantElement    tftypes.Value
	}{
		"ordinary writable field": {
			configElement:  terraformPlanElement(elementType, "same", nil, "new"),
			priorElement:   terraformPlanElement(elementType, "same", "readback", "old"),
			plannedElement: terraformPlanElement(elementType, "same", tftypes.UnknownValue, "new"),
			wantElement:    terraformPlanElement(elementType, "same", "readback", "new"),
		},
		"explicit optional computed field": {
			configElement:  terraformPlanElement(elementType, "same", "explicit", "same"),
			priorElement:   terraformPlanElement(elementType, "same", "readback", "same"),
			plannedElement: terraformPlanElement(elementType, "same", "explicit", "same"),
			wantElement:    terraformPlanElement(elementType, "same", "explicit", "same"),
		},
		"identity field": {
			configElement:  terraformPlanElement(elementType, "new", nil, "same"),
			priorElement:   terraformPlanElement(elementType, "old", "readback", "same"),
			plannedElement: terraformPlanElement(elementType, "new", tftypes.UnknownValue, "same"),
			wantElement:    terraformPlanElement(elementType, "new", "readback", "same"),
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := identityPlanProtocolCollection(t, "test_identity_resource", resourceType, collectionType,
				[]tftypes.Value{testCase.configElement},
				[]tftypes.Value{testCase.priorElement},
				[]tftypes.Value{testCase.plannedElement},
			)
			want := tftypes.NewValue(collectionType, []tftypes.Value{testCase.wantElement})
			if !got.Equal(want) {
				t.Fatalf("PlanResourceChange() items = %s, want %s", got, want)
			}
		})
	}
}

func TestPlanResourceChangeUnsafeIdentityAndNoMetadataDoNotGuess(t *testing.T) {
	t.Parallel()

	elementType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"identity": tftypes.String,
		"computed": tftypes.String,
		"writable": tftypes.String,
	}}
	collectionType := tftypes.Set{ElementType: elementType}
	resourceType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"id":    tftypes.String,
		"items": collectionType,
	}}
	unknownIdentity := terraformPlanElement(elementType, "", tftypes.UnknownValue, "planned")
	var unknownObject map[string]tftypes.Value
	if err := unknownIdentity.As(&unknownObject); err != nil {
		t.Fatalf("decoding fixture element: %v", err)
	}
	unknownObject["identity"] = tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
	unknownIdentity = tftypes.NewValue(elementType, unknownObject)

	testCases := map[string]struct {
		config  []tftypes.Value
		prior   []tftypes.Value
		planned []tftypes.Value
	}{
		"unknown": {
			config:  []tftypes.Value{unknownIdentity},
			prior:   []tftypes.Value{terraformPlanElement(elementType, "prior", "prior", "prior")},
			planned: []tftypes.Value{unknownIdentity},
		},
		"duplicate": {
			config: []tftypes.Value{
				terraformPlanElement(elementType, "duplicate", nil, "first"),
				terraformPlanElement(elementType, "duplicate", nil, "second"),
			},
			prior: []tftypes.Value{
				terraformPlanElement(elementType, "duplicate", "prior-first", "first"),
				terraformPlanElement(elementType, "duplicate", "prior-second", "second"),
			},
			planned: []tftypes.Value{
				terraformPlanElement(elementType, "duplicate", tftypes.UnknownValue, "first"),
				terraformPlanElement(elementType, "duplicate", tftypes.UnknownValue, "second"),
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			withMetadata := identityPlanProtocolCollection(t, "test_identity_resource", resourceType, collectionType,
				testCase.config, testCase.prior, testCase.planned)
			withoutMetadata := identityPlanProtocolCollection(t, "test_no_identity_resource", resourceType, collectionType,
				testCase.config, testCase.prior, testCase.planned)
			if !withMetadata.Equal(withoutMetadata) {
				t.Fatalf("unsafe identity plan = %s, want unchanged framework behavior %s", withMetadata, withoutMetadata)
			}
		})
	}
}

func TestPlanResourceChangeMergesMultisetAfterAttributeModifier(t *testing.T) {
	t.Parallel()

	elementType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"identity": tftypes.String,
		"computed": tftypes.String,
		"writable": tftypes.String,
	}}
	collectionType := tftypes.List{ElementType: elementType}
	resourceType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"id":    tftypes.String,
		"items": collectionType,
	}}
	got := identityPlanProtocolCollection(t, "test_identity_multiset_resource", resourceType, collectionType,
		[]tftypes.Value{
			terraformPlanElement(elementType, "second", nil, "new-second"),
			terraformPlanElement(elementType, "first", nil, "new-first"),
		},
		[]tftypes.Value{
			terraformPlanElement(elementType, "first", nil, "old-first"),
			terraformPlanElement(elementType, "second", "readback", "old-second"),
		},
		[]tftypes.Value{
			terraformPlanElement(elementType, "second", tftypes.UnknownValue, "new-second"),
			terraformPlanElement(elementType, "first", tftypes.UnknownValue, "new-first"),
		},
	)
	want := tftypes.NewValue(collectionType, []tftypes.Value{
		terraformPlanElement(elementType, "second", "readback", "new-second"),
		terraformPlanElement(elementType, "first", nil, "new-first"),
	})
	if !got.Equal(want) {
		t.Fatalf("PlanResourceChange() multiset = %s, want %s", got, want)
	}
}

// identityPlanTestProvider exposes one generic resource through the real
// protocol server so tests exercise the full PlanResourceChange pipeline.
type identityPlanTestProvider struct{}

func (identityPlanTestProvider) Metadata(_ context.Context, _ frameworkprovider.MetadataRequest, response *frameworkprovider.MetadataResponse) {
	response.TypeName = "test"
}

func (identityPlanTestProvider) Schema(_ context.Context, _ frameworkprovider.SchemaRequest, response *frameworkprovider.SchemaResponse) {
	response.Schema = providerschema.Schema{}
}

func (identityPlanTestProvider) Configure(context.Context, frameworkprovider.ConfigureRequest, *frameworkprovider.ConfigureResponse) {
}

func (identityPlanTestProvider) DataSources(context.Context) []func() datasource.DataSource {
	return nil
}

func (identityPlanTestProvider) Resources(context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		func() resource.Resource {
			return identityPlanTestResource("test_identity_resource", true)
		},
		func() resource.Resource {
			return identityPlanTestResource("test_no_identity_resource", false)
		},
		func() resource.Resource {
			return identityPlanTestResource("test_identity_multiset_resource", true)
		},
	}
}

// identityPlanTestResource builds protocol fixtures with or without identity
// metadata so tests can prove the feature is opt-in.
func identityPlanTestResource(typeName string, withMetadata bool) resource.Resource {
	nestedObject := schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
		"identity": schema.StringAttribute{Optional: true, Computed: true},
		"computed": schema.StringAttribute{
			Optional: true,
			Computed: true,
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		},
		"writable": schema.StringAttribute{Optional: true},
	}}
	var collectionAttribute schema.Attribute
	if typeName == "test_identity_multiset_resource" {
		collectionAttribute = schema.ListNestedAttribute{
			Optional:     true,
			Computed:     true,
			NestedObject: nestedObject,
			PlanModifiers: []planmodifier.List{
				Multiset(),
			},
		}
	} else {
		collectionAttribute = schema.SetNestedAttribute{
			Optional:      true,
			Computed:      true,
			NestedObject:  nestedObject,
			PlanModifiers: []planmodifier.Set{},
		}
	}
	resourceSchema := schema.Schema{Attributes: map[string]schema.Attribute{
		"id":    schema.StringAttribute{Computed: true},
		"items": collectionAttribute,
	}}
	options := []ResourceOptionsFunc{
		resourceWithCloudControlTypeName("Test::Identity::Resource"),
		resourceWithTerraformTypeName(typeName),
		resourceWithTerraformSchema(resourceSchema),
		resourceWithAttributeNameMap(map[string]string{
			"id":       "Id",
			"items":    "Items",
			"identity": "Identity",
			"computed": "Computed",
			"writable": "Writable",
		}),
	}
	if withMetadata {
		options = append(options, resourceWithCollectionIdentities([]CollectionIdentity{{
			PropertyPath:    "/Items",
			IdentifierPaths: []string{"/Identity"},
			UniqueItems:     true,
		}}))
	}
	value, err := NewResource(context.Background(), options...)
	if err != nil {
		panic(err)
	}
	return value
}

// identityPlanProtocolCollection executes one collection fixture through the
// protocol server and returns the final planned collection.
func identityPlanProtocolCollection(
	t *testing.T,
	typeName string,
	resourceType tftypes.Type,
	collectionType tftypes.Type,
	configElements []tftypes.Value,
	priorElements []tftypes.Value,
	plannedElements []tftypes.Value,
) tftypes.Value {
	t.Helper()
	root := func(elements []tftypes.Value) tftypes.Value {
		return tftypes.NewValue(resourceType, map[string]tftypes.Value{
			"id":    tftypes.NewValue(tftypes.String, nil),
			"items": tftypes.NewValue(collectionType, elements),
		})
	}
	server := providerserver.NewProtocol6(identityPlanTestProvider{})()
	response, err := server.PlanResourceChange(context.Background(), &tfprotov6.PlanResourceChangeRequest{
		TypeName:         typeName,
		Config:           identityPlanDynamicValue(t, resourceType, root(configElements)),
		PriorState:       identityPlanDynamicValue(t, resourceType, root(priorElements)),
		ProposedNewState: identityPlanDynamicValue(t, resourceType, root(plannedElements)),
	})
	if err != nil {
		t.Fatalf("PlanResourceChange() error = %v", err)
	}
	for _, diagnostic := range response.Diagnostics {
		if diagnostic.Severity == tfprotov6.DiagnosticSeverityError {
			t.Fatalf("PlanResourceChange() diagnostic = %s: %s", diagnostic.Summary, diagnostic.Detail)
		}
	}
	planned, err := response.PlannedState.Unmarshal(resourceType)
	if err != nil {
		t.Fatalf("unmarshalling planned state: %v", err)
	}
	var plannedRoot map[string]tftypes.Value
	if err := planned.As(&plannedRoot); err != nil {
		t.Fatalf("decoding planned root: %v", err)
	}
	return plannedRoot["items"]
}

func identityPlanDynamicValue(t *testing.T, valueType tftypes.Type, value tftypes.Value) *tfprotov6.DynamicValue {
	t.Helper()
	result, err := tfprotov6.NewDynamicValue(valueType, value)
	if err != nil {
		t.Fatalf("NewDynamicValue() error = %v", err)
	}
	return &result
}
