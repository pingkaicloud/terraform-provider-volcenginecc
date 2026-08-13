package vpc

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	frameworkprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	providerschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestENIPlanResourceChangePreservesNullAssociatedElasticIPByIdentity(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	eni, err := eNIResource(ctx)
	if err != nil {
		t.Fatalf("eNIResource() error = %v", err)
	}
	var schemaResponse resource.SchemaResponse
	eni.Schema(ctx, resource.SchemaRequest{}, &schemaResponse)
	resourceType := schemaResponse.Schema.Type().TerraformType(ctx)
	resourceObjectType, ok := resourceType.(tftypes.Object)
	if !ok {
		t.Fatalf("ENI Terraform type = %T, want tftypes.Object", resourceType)
	}
	privateIPSetsType := resourceObjectType.AttributeTypes["private_ip_sets"]
	privateIPSets, ok := privateIPSetsType.(tftypes.Set)
	if !ok {
		t.Fatalf("private_ip_sets type = %T, want tftypes.Set", privateIPSetsType)
	}
	privateIPElement, ok := privateIPSets.ElementType.(tftypes.Object)
	if !ok {
		t.Fatalf("private_ip_sets element type = %T, want tftypes.Object", privateIPSets.ElementType)
	}

	config := eniRootValue(resourceObjectType)
	prior := eniRootValue(resourceObjectType)
	proposed := eniRootValue(resourceObjectType)
	config["private_ip_sets"] = tftypes.NewValue(privateIPSets, []tftypes.Value{
		eniPrivateIPElement(privateIPElement, "192.168.0.220", nil),
		eniPrivateIPElement(privateIPElement, "192.168.0.221", nil),
	})
	prior["private_ip_sets"] = tftypes.NewValue(privateIPSets, []tftypes.Value{
		eniPrivateIPElement(privateIPElement, "192.168.0.221", nil),
		eniPrivateIPElement(privateIPElement, "192.168.0.220", nil),
	})
	proposed["private_ip_sets"] = tftypes.NewValue(privateIPSets, []tftypes.Value{
		eniPrivateIPElement(privateIPElement, "192.168.0.220", tftypes.UnknownValue),
		eniPrivateIPElement(privateIPElement, "192.168.0.221", tftypes.UnknownValue),
	})
	config["description"] = tftypes.NewValue(tftypes.String, "new description")
	prior["description"] = tftypes.NewValue(tftypes.String, "old description")
	proposed["description"] = tftypes.NewValue(tftypes.String, "new description")

	server := providerserver.NewProtocol6(eniIdentityPlanTestProvider{})()
	response, err := server.PlanResourceChange(ctx, &tfprotov6.PlanResourceChangeRequest{
		TypeName:         "volcenginecc_vpc_eni",
		Config:           eniDynamicValue(t, resourceType, tftypes.NewValue(resourceType, config)),
		PriorState:       eniDynamicValue(t, resourceType, tftypes.NewValue(resourceType, prior)),
		ProposedNewState: eniDynamicValue(t, resourceType, tftypes.NewValue(resourceType, proposed)),
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
	want := tftypes.NewValue(privateIPSets, []tftypes.Value{
		eniPrivateIPElement(privateIPElement, "192.168.0.220", nil),
		eniPrivateIPElement(privateIPElement, "192.168.0.221", nil),
	})
	if !plannedRoot["private_ip_sets"].Equal(want) {
		t.Fatalf("planned private_ip_sets = %s, want %s", plannedRoot["private_ip_sets"], want)
	}
}

// eniIdentityPlanTestProvider exposes the generated ENI resource through the
// protocol server without configuring a cloud client.
type eniIdentityPlanTestProvider struct{}

func (eniIdentityPlanTestProvider) Metadata(_ context.Context, _ frameworkprovider.MetadataRequest, response *frameworkprovider.MetadataResponse) {
	response.TypeName = "volcenginecc"
}

func (eniIdentityPlanTestProvider) Schema(_ context.Context, _ frameworkprovider.SchemaRequest, response *frameworkprovider.SchemaResponse) {
	response.Schema = providerschema.Schema{}
}

func (eniIdentityPlanTestProvider) Configure(context.Context, frameworkprovider.ConfigureRequest, *frameworkprovider.ConfigureResponse) {
}

func (eniIdentityPlanTestProvider) DataSources(context.Context) []func() datasource.DataSource {
	return nil
}

func (eniIdentityPlanTestProvider) Resources(context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		func() resource.Resource {
			value, err := eNIResource(context.Background())
			if err != nil {
				panic(err)
			}
			return value
		},
	}
}

// eniRootValue creates a typed null baseline for every generated ENI
// attribute so the fixture stays aligned when unrelated attributes are added.
func eniRootValue(resourceType tftypes.Object) map[string]tftypes.Value {
	result := make(map[string]tftypes.Value, len(resourceType.AttributeTypes))
	for name, attributeType := range resourceType.AttributeTypes {
		result[name] = tftypes.NewValue(attributeType, nil)
	}
	return result
}

// eniPrivateIPElement creates the exact generated PrivateIpSets element shape.
func eniPrivateIPElement(elementType tftypes.Object, privateIPAddress string, associatedElasticIP interface{}) tftypes.Value {
	return tftypes.NewValue(elementType, map[string]tftypes.Value{
		"associated_elastic_ip": tftypes.NewValue(elementType.AttributeTypes["associated_elastic_ip"], associatedElasticIP),
		"private_ip_address":    tftypes.NewValue(tftypes.String, privateIPAddress),
	})
}

// eniDynamicValue encodes a typed protocol value for the ENI fixture.
func eniDynamicValue(t *testing.T, valueType tftypes.Type, value tftypes.Value) *tfprotov6.DynamicValue {
	t.Helper()
	result, err := tfprotov6.NewDynamicValue(valueType, value)
	if err != nil {
		t.Fatalf("NewDynamicValue() error = %v", err)
	}
	return &result
}
