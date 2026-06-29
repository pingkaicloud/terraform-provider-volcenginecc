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
			resourceSchema := schema.Schema{Attributes: map[string]schema.Attribute{
				"id": schema.StringAttribute{Computed: true},
				"items": schema.SetNestedAttribute{
					Optional: true,
					Computed: true,
					NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
						"identity": schema.StringAttribute{Optional: true, Computed: true},
						"computed": schema.StringAttribute{
							Optional: true,
							Computed: true,
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
							},
						},
						"writable": schema.StringAttribute{Optional: true},
					}},
					PlanModifiers: []planmodifier.Set{},
				},
			}}
			value, err := NewResource(context.Background(),
				resourceWithCloudControlTypeName("Test::Identity::Resource"),
				resourceWithTerraformTypeName("test_identity_resource"),
				resourceWithTerraformSchema(resourceSchema),
				resourceWithAttributeNameMap(map[string]string{
					"id":       "Id",
					"items":    "Items",
					"identity": "Identity",
					"computed": "Computed",
					"writable": "Writable",
				}),
				resourceWithCollectionIdentities([]CollectionIdentity{{
					PropertyPath:    "/Items",
					IdentifierPaths: []string{"/Identity"},
					UniqueItems:     true,
				}}),
			)
			if err != nil {
				panic(err)
			}
			return value
		},
	}
}

func identityPlanDynamicValue(t *testing.T, valueType tftypes.Type, value tftypes.Value) *tfprotov6.DynamicValue {
	t.Helper()
	result, err := tfprotov6.NewDynamicValue(valueType, value)
	if err != nil {
		t.Fatalf("NewDynamicValue() error = %v", err)
	}
	return &result
}
