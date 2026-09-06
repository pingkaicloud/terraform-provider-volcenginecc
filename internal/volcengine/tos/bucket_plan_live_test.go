package tos

import (
	"context"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	frameworkprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	providerschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestLiveTOSBucketPlanIgnoresPolicyCosmetics(t *testing.T) {
	t.Run("internal", func(t *testing.T) {
		replayLiveTOSBucketPlan(t, "/tmp/tos-live-config.json", "/tmp/tos-live-prior.json")
	})
	t.Run("data", func(t *testing.T) {
		replayLiveTOSBucketPlan(t, "/tmp/tos-live-data-config.json", "/tmp/tos-live-data-prior.json")
	})
}

func replayLiveTOSBucketPlan(t *testing.T, configPath, priorPath string) {
	t.Helper()
	configJSON, err := os.ReadFile(configPath)
	if err != nil {
		t.Skip(err)
	}
	priorJSON, err := os.ReadFile(priorPath)
	if err != nil {
		t.Skip(err)
	}

	ctx := context.Background()
	res, err := bucketResource(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var schemaResp resource.SchemaResponse
	res.Schema(ctx, resource.SchemaRequest{}, &schemaResp)
	resourceType := schemaResp.Schema.Type().TerraformType(ctx)

	config := mustTFValueFromJSON(t, resourceType, configJSON)
	prior := mustTFValueFromJSON(t, resourceType, priorJSON)

	before := filteredKnownDiffPaths(t, config, prior)
	t.Logf("config vs prior filtered diffs (%d): %v", len(before), before)
	if len(before) == 0 {
		t.Fatal("config vs prior should still have a cosmetic diff")
	}

	server := providerserver.NewProtocol6(liveTOSBucketPlanProvider{})()
	response, err := server.PlanResourceChange(ctx, &tfprotov6.PlanResourceChangeRequest{
		TypeName:         "volcenginecc_tos_bucket",
		Config:           mustDynamicValue(t, resourceType, config),
		PriorState:       mustDynamicValue(t, resourceType, prior),
		ProposedNewState: mustDynamicValue(t, resourceType, config),
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
	after := filteredKnownDiffPaths(t, planned, prior)
	t.Logf("planned vs prior filtered diffs (%d): %v", len(after), after)
	if len(after) != 0 {
		t.Fatalf("planned filtered diffs = %v, want none", after)
	}
}

type liveTOSBucketPlanProvider struct{}

func (liveTOSBucketPlanProvider) Metadata(_ context.Context, _ frameworkprovider.MetadataRequest, response *frameworkprovider.MetadataResponse) {
	response.TypeName = "volcenginecc"
}

func (liveTOSBucketPlanProvider) Schema(_ context.Context, _ frameworkprovider.SchemaRequest, response *frameworkprovider.SchemaResponse) {
	response.Schema = providerschema.Schema{}
}

func (liveTOSBucketPlanProvider) Configure(context.Context, frameworkprovider.ConfigureRequest, *frameworkprovider.ConfigureResponse) {
}

func (liveTOSBucketPlanProvider) DataSources(context.Context) []func() datasource.DataSource {
	return nil
}

func (liveTOSBucketPlanProvider) Resources(context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		func() resource.Resource {
			res, err := bucketResource(context.Background())
			if err != nil {
				panic(err)
			}
			return res
		},
	}
}

func mustTFValueFromJSON(t *testing.T, resourceType tftypes.Type, raw []byte) tftypes.Value {
	t.Helper()
	value, err := tftypes.ValueFromJSONWithOpts(raw, resourceType, tftypes.ValueFromJSONOpts{IgnoreUndefinedAttributes: true})
	if err != nil {
		t.Fatalf("ValueFromJSONWithOpts() error = %v", err)
	}
	return value
}

func mustDynamicValue(t *testing.T, resourceType tftypes.Type, value tftypes.Value) *tfprotov6.DynamicValue {
	t.Helper()
	dynamic, err := tfprotov6.NewDynamicValue(resourceType, value)
	if err != nil {
		t.Fatalf("NewDynamicValue() error = %v", err)
	}
	return &dynamic
}

func filteredKnownDiffPaths(t *testing.T, planned, prior tftypes.Value) []string {
	t.Helper()
	diffs, err := planned.Diff(prior)
	if err != nil {
		t.Fatalf("Diff() error = %v", err)
	}
	var paths []string
	for _, diff := range diffs {
		if diff.Value1 != nil && diff.Value1.IsKnown() && !diff.Value1.IsNull() {
			paths = append(paths, diff.Path.String())
		}
	}
	return paths
}
