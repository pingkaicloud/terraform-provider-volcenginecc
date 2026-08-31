package vke

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

func TestLiveNodePoolPlanKeepsInjectedSecurityGroups(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	res, err := nodePoolResource(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var schemaResp resource.SchemaResponse
	res.Schema(ctx, resource.SchemaRequest{}, &schemaResp)
	resourceType := schemaResp.Schema.Type().TerraformType(ctx)

	config := mustTFValueFromJSON(t, resourceType, liveNodePoolConfigJSON)
	prior := mustTFValueFromJSON(t, resourceType, liveNodePoolPriorJSON)
	proposed := mustTFValueFromJSON(t, resourceType, liveNodePoolProposedJSON)

	before := filteredKnownDiffPaths(t, proposed, prior)
	if len(before) == 0 {
		t.Fatal("proposed vs prior should still have a security group diff before ModifyPlan")
	}
	t.Logf("pre-plan filtered diffs: %v", before)

	server := providerserver.NewProtocol6(liveNodePoolPlanProvider{})()
	response, err := server.PlanResourceChange(ctx, &tfprotov6.PlanResourceChangeRequest{
		TypeName:         "volcenginecc_vke_node_pool",
		Config:           mustDynamicValue(t, resourceType, config),
		PriorState:       mustDynamicValue(t, resourceType, prior),
		ProposedNewState: mustDynamicValue(t, resourceType, proposed),
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
	if len(after) != 0 {
		t.Fatalf("planned filtered diffs = %v, want none", after)
	}
}

type liveNodePoolPlanProvider struct{}

func (liveNodePoolPlanProvider) Metadata(_ context.Context, _ frameworkprovider.MetadataRequest, response *frameworkprovider.MetadataResponse) {
	response.TypeName = "volcenginecc"
}

func (liveNodePoolPlanProvider) Schema(_ context.Context, _ frameworkprovider.SchemaRequest, response *frameworkprovider.SchemaResponse) {
	response.Schema = providerschema.Schema{}
}

func (liveNodePoolPlanProvider) Configure(context.Context, frameworkprovider.ConfigureRequest, *frameworkprovider.ConfigureResponse) {
}

func (liveNodePoolPlanProvider) DataSources(context.Context) []func() datasource.DataSource {
	return nil
}

func (liveNodePoolPlanProvider) Resources(context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		func() resource.Resource {
			res, err := nodePoolResource(context.Background())
			if err != nil {
				panic(err)
			}
			return res
		},
	}
}

func mustTFValueFromJSON(t *testing.T, resourceType tftypes.Type, raw string) tftypes.Value {
	t.Helper()
	value, err := tftypes.ValueFromJSONWithOpts([]byte(raw), resourceType, tftypes.ValueFromJSONOpts{IgnoreUndefinedAttributes: true})
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

func TestLiveNodePoolPlanWithSparseProposed(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	res, err := nodePoolResource(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var schemaResp resource.SchemaResponse
	res.Schema(ctx, resource.SchemaRequest{}, &schemaResp)
	resourceType := schemaResp.Schema.Type().TerraformType(ctx)

	config := mustTFValueFromJSON(t, resourceType, liveNodePoolConfigJSON)
	prior := mustTFValueFromJSON(t, resourceType, liveNodePoolPriorJSON)

	server := providerserver.NewProtocol6(liveNodePoolPlanProvider{})()
	response, err := server.PlanResourceChange(ctx, &tfprotov6.PlanResourceChangeRequest{
		TypeName:         "volcenginecc_vke_node_pool",
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
	t.Logf("sparse-proposed filtered diffs (%d): %v", len(after), after)
	if len(after) != 0 {
		t.Fatalf("planned filtered diffs = %v, want none", after)
	}
}

// Live Seed worker pool: DPM writes one node SG; VKE adds a cluster default SG.
const liveNodePoolConfigJSON = `{
  "id": "cda8fkhu3mdjrrh2itidg|pda8fpihc8ff7rbq9h1jg",
  "cluster_id": "cda8fkhu3mdjrrh2itidg",
  "name": "tidbcloud-tidb-worker-ecs-r3a-4xlarge-27639c3e-cn-beijing-a",
  "auto_scaling": {"enabled": true, "max_replicas": 20, "min_replicas": 0, "priority": 5, "subnet_policy": "Priority"},
  "kubernetes_config": {
    "labels": [{"key": "node-spec", "value": "16C128G"}, {"key": "tidbcloud.com/node-pool-k8s-version", "value": "v1.34.6-vke.12"}],
    "taints": [{"effect": "NoSchedule", "key": "dedicated", "value": "worker"}]
  },
  "management": {"remedy_config": {}},
  "node_config": {
    "image_id": "image-ybqi99s7yq8rx7mnk44b",
    "instance_charge_type": "PostPaid",
    "instance_type_ids": ["ecs.r3a.4xlarge"],
    "project_name": "pingkai-premium-infra-dev-cn-beijing",
    "public_access_enabled": false,
    "security": {
      "login": {"ssh_key_pair_name": "dev-seed-cn-beijing-ng-db26df53-operator"},
      "security_group_ids": ["sg-iimzx00a7thc74o8cuc8498m"],
      "security_strategies": ["Hids"]
    },
    "spot_strategy": "NoSpot",
    "subnet_ids": ["subnet-1jobp4ebpn4e81n7amqafc8fz"],
    "system_volume": {"size": 50, "type": "ESSD_PL0"}
  },
  "tags": [
    {"key": "created-by", "value": "ali-align"},
    {"key": "dataplane", "value": "dp-ali-align-20260828"},
    {"key": "environment", "value": "dev"},
    {"key": "servicetype", "value": "dedicated"},
    {"key": "tenant", "value": "6"},
    {"key": "tidbcloud.com/availableCpu", "value": "15"},
    {"key": "tidbcloud.com/availableMemoryMi", "value": "119991"},
    {"key": "tidbcloud.com/cpu", "value": "16"},
    {"key": "tidbcloud.com/memeoryGi", "value": "128"},
    {"key": "usedby", "value": "dbaas-control-plane-seed"}
  ]
}`

const liveNodePoolPriorJSON = `{
  "id": "cda8fkhu3mdjrrh2itidg|pda8fpihc8ff7rbq9h1jg",
  "cluster_id": "cda8fkhu3mdjrrh2itidg",
  "name": "tidbcloud-tidb-worker-ecs-r3a-4xlarge-27639c3e-cn-beijing-a",
  "auto_scaling": {"desired_replicas": 0, "enabled": true, "max_replicas": 20, "min_replicas": 0, "priority": 5, "subnet_policy": "Priority"},
  "kubernetes_config": {
    "auto_sync_disabled": false,
    "cordon": false,
    "labels": [{"key": "node-spec", "value": "16C128G"}, {"key": "tidbcloud.com/node-pool-k8s-version", "value": "v1.34.6-vke.12"}],
    "name_prefix": "",
    "name_suffix": "",
    "name_use_hostname": false,
    "taints": [{"effect": "NoSchedule", "key": "dedicated", "value": "worker"}]
  },
  "management": {"enabled": false, "remedy_config": {"enabled": false}},
  "node_config": {
    "additional_container_storage_enabled": false,
    "auto_renew": false,
    "auto_renew_period": 0,
    "deployment_set_group_number": 0,
    "deployment_set_id": "",
    "image_id": "image-ybqi99s7yq8rx7mnk44b",
    "initialize_script": "",
    "instance_charge_type": "PostPaid",
    "instance_type_ids": ["ecs.r3a.4xlarge"],
    "name_prefix": "",
    "period": 0,
    "project_name": "pingkai-premium-infra-dev-cn-beijing",
    "public_access_config": {"bandwidth": 100, "billing_type": 3, "isp": "BGP"},
    "public_access_enabled": false,
    "security": {
      "login": {"ssh_key_pair_name": "dev-seed-cn-beijing-ng-db26df53-operator", "type": "SshKeyPair"},
      "security_group_ids": ["sg-iimzx00a7thc74o8cuc8498m", "sg-bt05ndi90hs05h0b2uq3ri6i"],
      "security_strategies": ["Hids"],
      "security_strategy_enabled": true
    },
    "spot_strategy": "NoSpot",
    "subnet_ids": ["subnet-1jobp4ebpn4e81n7amqafc8fz"],
    "system_volume": {"size": 50, "type": "ESSD_PL0"}
  },
  "created_time": "2026-08-28T11:13:15+08:00",
  "node_pool_id": "pda8fpihc8ff7rbq9h1jg",
  "tags": [
    {"key": "created-by", "value": "ali-align"},
    {"key": "dataplane", "value": "dp-ali-align-20260828"},
    {"key": "environment", "value": "dev"},
    {"key": "servicetype", "value": "dedicated"},
    {"key": "tenant", "value": "6"},
    {"key": "tidbcloud.com/availableCpu", "value": "15"},
    {"key": "tidbcloud.com/availableMemoryMi", "value": "119991"},
    {"key": "tidbcloud.com/cpu", "value": "16"},
    {"key": "tidbcloud.com/memeoryGi", "value": "128"},
    {"key": "usedby", "value": "dbaas-control-plane-seed"}
  ],
  "updated_time": "2026-08-31T11:16:34+08:00"
}`

// proposed matches Upjet after optional+computed nulls take prior, but
// configured security_group_ids stays the single DPM SG.
const liveNodePoolProposedJSON = `{
  "id": "cda8fkhu3mdjrrh2itidg|pda8fpihc8ff7rbq9h1jg",
  "cluster_id": "cda8fkhu3mdjrrh2itidg",
  "name": "tidbcloud-tidb-worker-ecs-r3a-4xlarge-27639c3e-cn-beijing-a",
  "auto_scaling": {"desired_replicas": 0, "enabled": true, "max_replicas": 20, "min_replicas": 0, "priority": 5, "subnet_policy": "Priority"},
  "kubernetes_config": {
    "auto_sync_disabled": false,
    "cordon": false,
    "labels": [{"key": "node-spec", "value": "16C128G"}, {"key": "tidbcloud.com/node-pool-k8s-version", "value": "v1.34.6-vke.12"}],
    "name_prefix": "",
    "name_suffix": "",
    "name_use_hostname": false,
    "taints": [{"effect": "NoSchedule", "key": "dedicated", "value": "worker"}]
  },
  "management": {"enabled": false, "remedy_config": {"enabled": false}},
  "node_config": {
    "additional_container_storage_enabled": false,
    "auto_renew": false,
    "auto_renew_period": 0,
    "deployment_set_group_number": 0,
    "deployment_set_id": "",
    "image_id": "image-ybqi99s7yq8rx7mnk44b",
    "initialize_script": "",
    "instance_charge_type": "PostPaid",
    "instance_type_ids": ["ecs.r3a.4xlarge"],
    "name_prefix": "",
    "period": 0,
    "project_name": "pingkai-premium-infra-dev-cn-beijing",
    "public_access_config": {"bandwidth": 100, "billing_type": 3, "isp": "BGP"},
    "public_access_enabled": false,
    "security": {
      "login": {"ssh_key_pair_name": "dev-seed-cn-beijing-ng-db26df53-operator", "type": "SshKeyPair"},
      "security_group_ids": ["sg-iimzx00a7thc74o8cuc8498m"],
      "security_strategies": ["Hids"],
      "security_strategy_enabled": true
    },
    "spot_strategy": "NoSpot",
    "subnet_ids": ["subnet-1jobp4ebpn4e81n7amqafc8fz"],
    "system_volume": {"size": 50, "type": "ESSD_PL0"}
  },
  "created_time": "2026-08-28T11:13:15+08:00",
  "node_pool_id": "pda8fpihc8ff7rbq9h1jg",
  "tags": [
    {"key": "created-by", "value": "ali-align"},
    {"key": "dataplane", "value": "dp-ali-align-20260828"},
    {"key": "environment", "value": "dev"},
    {"key": "servicetype", "value": "dedicated"},
    {"key": "tenant", "value": "6"},
    {"key": "tidbcloud.com/availableCpu", "value": "15"},
    {"key": "tidbcloud.com/availableMemoryMi", "value": "119991"},
    {"key": "tidbcloud.com/cpu", "value": "16"},
    {"key": "tidbcloud.com/memeoryGi", "value": "128"},
    {"key": "usedby", "value": "dbaas-control-plane-seed"}
  ],
  "updated_time": "2026-08-31T11:16:34+08:00"
}`
