package vke

import (
	"context"
	"fmt"
	"maps"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/volcengine/terraform-provider-volcenginecc/internal/customresources"
	tfcloudcontrol "github.com/volcengine/terraform-provider-volcenginecc/internal/service/cloudcontrol"
)

const appliedVersionField = "applied_kubernetes_version"

func init() {
	for _, name := range []string{"volcenginecc_vke_cluster", "volcenginecc_vke_node_pool"} {
		customresources.Register(name, func(_ context.Context, inner resource.Resource) (resource.Resource, error) {
			return &upgradeResource{Resource: inner, nodePool: name == "volcenginecc_vke_node_pool"}, nil
		})
	}
}

// Keep native upgrade handling outside the generated Cloud Control resources.
// In particular, neither version field below is sent in a CC update patch.
type upgradeResource struct {
	resource.Resource
	nodePool  bool
	provider  tfcloudcontrol.Provider
	newClient func(context.Context, tfcloudcontrol.Provider) (*upgradeClient, error)
}

func (r *upgradeResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	r.Resource.Schema(ctx, req, resp)
	resp.Schema.Attributes = maps.Clone(resp.Schema.Attributes)
	description := "Kubernetes major.minor version (for example 1.34). Changes upgrade the control plane in place through the native VKE API; kubernetes_version_create remains create-only."
	if r.nodePool {
		description = "Kubernetes major.minor version (for example 1.34). Changes upgrade this pool's nodes in place after the control plane reaches that version. State reflects the last successful provider create/upgrade, not a per-node kubelet query."
		resp.Schema.Attributes[appliedVersionField] = schema.StringAttribute{
			Computed:    true,
			Description: "Last Kubernetes version successfully applied by this provider. Kept separately from configuration so imported resources or reconstructed state cannot mistake a desired version for a completed upgrade.",
		}
	}
	resp.Schema.Attributes["kubernetes_version"] = schema.StringAttribute{
		Optional: true, Computed: true, Description: description,
		Validators:    []validator.String{stringvalidator.RegexMatches(regexp.MustCompile(`^[0-9]+\.[0-9]+$`), "must be a major.minor version, for example 1.34")},
		PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
	}
}

func (r *upgradeResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if inner, ok := r.Resource.(resource.ResourceWithConfigure); ok {
		inner.Configure(ctx, req, resp)
	}
	if req.ProviderData != nil {
		r.provider = req.ProviderData.(tfcloudcontrol.Provider)
	}
	if r.newClient == nil {
		r.newClient = newUpgradeClient
	}
}

func (r *upgradeResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	if inner, ok := r.Resource.(resource.ResourceWithConfigValidators); ok {
		return inner.ConfigValidators(ctx)
	}
	return nil
}

func (r *upgradeResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if inner, ok := r.Resource.(resource.ResourceWithImportState); ok {
		inner.ImportState(ctx, req, resp)
	}
}

func (r *upgradeResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	// Preserve the existing normalization of autoscaler replicas and VKE's
	// injected security group; the wrapper must forward optional interfaces.
	if inner, ok := r.Resource.(resource.ResourceWithModifyPlan); ok {
		inner.ModifyPlan(ctx, req, resp)
	}
}

func (r *upgradeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var applied types.String
	if r.nodePool {
		resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root(appliedVersionField), &applied)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}
	r.Resource.Read(ctx, req, resp)
	if !r.nodePool || resp.Diagnostics.HasError() || resp.State.Raw.IsNull() {
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(appliedVersionField), applied)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("kubernetes_version"), applied)...)
}

func (r *upgradeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var target types.String
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("kubernetes_version"), &target)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.nodePool {
		if target.ValueString() != "" {
			var clusterID string
			resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("cluster_id"), &clusterID)...)
			if resp.Diagnostics.HasError() {
				return
			}
			client, err := r.newClient(ctx, r.provider)
			if err == nil {
				err = checkClusterVersion(ctx, client, clusterID, target.ValueString())
			}
			if err != nil {
				resp.Diagnostics.AddError("VKE node pool version", err.Error())
				return
			}
		}
		resp.Diagnostics.Append(req.Plan.SetAttribute(ctx, path.Root(appliedVersionField), types.StringNull())...)
	} else if target.ValueString() != "" {
		var createVersion types.String
		resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("kubernetes_version_create"), &createVersion)...)
		if createVersion.ValueString() != "" && !createVersion.Equal(target) {
			resp.Diagnostics.AddError("VKE cluster version", "kubernetes_version and kubernetes_version_create must match at creation")
			return
		}
		resp.Diagnostics.Append(req.Plan.SetAttribute(ctx, path.Root("kubernetes_version_create"), target)...)
	}
	// A Cloud Control create only accepts KubernetesVersionCreate; the node
	// pool's version is inherited from its control plane at creation.
	version := types.StringUnknown()
	if r.nodePool {
		version = types.StringNull()
	}
	resp.Diagnostics.Append(req.Plan.SetAttribute(ctx, path.Root("kubernetes_version"), version)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.Resource.Create(ctx, req, resp)
	if r.nodePool && !resp.Diagnostics.HasError() {
		if target.IsUnknown() {
			target = types.StringNull()
		}
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("kubernetes_version"), target)...)
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(appliedVersionField), target)...)
	}
}

func (r *upgradeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var target, prior types.String
	var id string
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("kubernetes_version"), &target)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("kubernetes_version"), &prior)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("id"), &id)...)
	var clusterID string
	if r.nodePool {
		resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("cluster_id"), &clusterID)...)
		resp.Diagnostics.Append(req.Plan.SetAttribute(ctx, path.Root(appliedVersionField), types.StringNull())...)
		resp.Diagnostics.Append(req.State.SetAttribute(ctx, path.Root(appliedVersionField), types.StringNull())...)
		resp.Diagnostics.Append(req.State.SetAttribute(ctx, path.Root("kubernetes_version"), types.StringNull())...)
		resp.Diagnostics.Append(req.Plan.SetAttribute(ctx, path.Root("kubernetes_version"), types.StringNull())...)
	} else {
		clusterID = id
		resp.Diagnostics.Append(req.Plan.SetAttribute(ctx, path.Root("kubernetes_version"), prior)...)
	}
	if resp.Diagnostics.HasError() {
		return
	}
	r.Resource.Update(ctx, req, resp)
	if resp.Diagnostics.HasError() {
		return
	}
	// Preserve the old completion marker until the native task succeeds.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("kubernetes_version"), prior)...)
	if r.nodePool {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(appliedVersionField), prior)...)
	}
	if target.ValueString() == "" || target.Equal(prior) {
		return
	}
	client, err := r.newClient(ctx, r.provider)
	if err != nil {
		resp.Diagnostics.AddError("VKE Kubernetes upgrade", err.Error())
		return
	}
	poolID := ""
	emptyPool := false
	if r.nodePool {
		poolID = id
		var count types.Int64
		resp.Diagnostics.Append(resp.State.GetAttribute(ctx, path.Root("node_statistics").AtName("total_count"), &count)...)
		if resp.Diagnostics.HasError() {
			return
		}
		emptyPool = !count.IsNull() && !count.IsUnknown() && count.ValueInt64() == 0
	}
	// Empty pools have no nodes to roll. Future nodes inherit the verified
	// control plane version; UpgradeNodePools need not receive an empty batch.
	if !emptyPool {
		if err := client.upgrade(ctx, clusterID, poolID, target.ValueString()); err != nil {
			resp.Diagnostics.AddError("VKE Kubernetes upgrade", err.Error())
			return
		}
	}
	if err := checkClusterVersion(ctx, client, clusterID, target.ValueString()); err != nil {
		resp.Diagnostics.AddError("VKE Kubernetes upgrade", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("kubernetes_version"), target)...)
	if r.nodePool {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(appliedVersionField), target)...)
	}
}

func checkClusterVersion(ctx context.Context, client *upgradeClient, id, target string) error {
	version, err := client.clusterVersion(ctx, id)
	if err != nil {
		return err
	}
	minor, err := minorVersion(version)
	if err != nil {
		return err
	}
	if minor != target {
		return fmt.Errorf("VKE cluster %s is at %s, expected %s", id, version, target)
	}
	return nil
}
