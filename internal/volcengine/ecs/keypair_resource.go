// Copyright (c) 2025 Beijing Volcano Engine Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

// Hand-written customization for `volcenginecc_ecs_keypair`.
//
// The Volcengine Cloud Control API only returns `PrivateKey` from
// CreateResource. GetResource never returns it. The default flow in
// `internal/generic/resource.go` therefore loses the private key immediately
// after creation.
//
// This file registers a custom override that:
//   - During Create, captures `PrivateKey` from the CreateResource event's
//     ResourceModel and writes it into state.
//   - During Read, restores `private_key` from prior state because the
//     auto-generated Read overwrites it with null from GetResource.
//
// Update/Delete/Metadata/Schema/Configure/ImportState/ConfigValidators all
// delegate to the auto-generated inner resource. The override is registered via
// `customresources.Register` so the auto-generated `keypair_resource_gen.go`
// stays untouched and survives `make resources`.

package ecs

import (
	"context"
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/volcengine/terraform-provider-volcenginecc/internal/customresources"
	"github.com/volcengine/terraform-provider-volcenginecc/internal/generic"
	tfcloudcontrol "github.com/volcengine/terraform-provider-volcenginecc/internal/service/cloudcontrol"
	"github.com/volcengine/terraform-provider-volcenginecc/internal/util"
)

const (
	keypairTerraformTypeName        = "volcenginecc_ecs_keypair"
	keypairCloudControlTypeName     = "Volcengine::ECS::Keypair"
	keypairPrivateKeyTerraformField = "private_key"
)

// Mirror of the WithAttributeNameMap call in the auto-generated
// keypair_resource_gen.go. Duplicated here so this override stays
// self-contained; if the upstream schema changes, this mapping must be
// updated alongside. The "id" → "ID" entry is intentionally omitted because
// the generic framework synthesizes it for the reserved top-level identifier
// (see resourceWithAttributeNameMap in internal/generic/resource.go); the
// translator never sees a known "id" value during Create either way.
var keypairTfToCcNameMap = map[string]string{
	"created_time":  "CreatedTime",
	"description":   "Description",
	"finger_print":  "FingerPrint",
	"instance_ids":  "InstanceIds",
	"key":           "Key",
	"key_pair_id":   "KeyPairId",
	"key_pair_name": "KeyPairName",
	"private_key":   "PrivateKey",
	"project_name":  "ProjectName",
	"public_key":    "PublicKey",
	"tags":          "Tags",
	"updated_time":  "UpdatedTime",
	"value":         "Value",
}

var keypairCcToTfNameMap = func() map[string]string {
	m := make(map[string]string, len(keypairTfToCcNameMap))
	for tf, cc := range keypairTfToCcNameMap {
		m[cc] = tf
	}
	return m
}()

func init() {
	customresources.Register(keypairTerraformTypeName, func(_ context.Context, inner resource.Resource) (resource.Resource, error) {
		return &keypairResourceWithPrivateKey{Resource: inner}, nil
	})
}

// keypairResourceWithPrivateKey embeds the auto-generated inner resource so that
// Metadata/Schema/Update/Delete (declared on resource.Resource) forward
// automatically. Configure/Read/Create are overridden; ImportState and
// ConfigValidators are explicitly forwarded because they belong to optional
// interfaces the framework probes via type assertion on the outer type.
type keypairResourceWithPrivateKey struct {
	resource.Resource
	provider tfcloudcontrol.Provider
}

func (r *keypairResourceWithPrivateKey) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if c, ok := r.Resource.(resource.ResourceWithConfigure); ok {
		c.Configure(ctx, req, resp)
	}
	if v := req.ProviderData; v != nil {
		if p, ok := v.(tfcloudcontrol.Provider); ok {
			r.provider = p
		}
	}
}

// Read delegates to the inner generic Read, then restores private_key from
// prior state. The Volcengine ECS API's GetResource never returns PrivateKey,
// so without this post-processing the value would be clobbered to null on every
// refresh.
func (r *keypairResourceWithPrivateKey) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var priorPrivateKey types.String
	priorDiags := req.State.GetAttribute(ctx, path.Root(keypairPrivateKeyTerraformField), &priorPrivateKey)

	r.Resource.Read(ctx, req, resp)
	if resp.Diagnostics.HasError() {
		return
	}

	if !priorDiags.HasError() && !priorPrivateKey.IsNull() && !priorPrivateKey.IsUnknown() && priorPrivateKey.ValueString() != "" {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(keypairPrivateKeyTerraformField), priorPrivateKey)...)
	}
}

func (r *keypairResourceWithPrivateKey) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if i, ok := r.Resource.(resource.ResourceWithImportState); ok {
		i.ImportState(ctx, req, resp)
	}
}

func (r *keypairResourceWithPrivateKey) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	if c, ok := r.Resource.(resource.ResourceWithConfigValidators); ok {
		return c.ConfigValidators(ctx)
	}
	return nil
}

// Create reproduces the generic Create flow but captures the CreateResource
// event's ResourceModel so it can extract PrivateKey, which the Volcengine ECS
// API only returns at creation time.
func (r *keypairResourceWithPrivateKey) Create(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	cloudControlClient := r.provider.CloudControlAPIClient(ctx)

	desiredState, err := generic.ToCloudControlString(ctx, request.Plan.Schema, request.Plan.Raw, keypairTfToCcNameMap)
	if err != nil {
		response.Diagnostics.Append(generic.DesiredStateErrorDiag("Plan", err))
		return
	}

	targetState := make(map[string]any)
	if err := json.Unmarshal([]byte(desiredState), &targetState); err != nil {
		response.Diagnostics.Append(generic.DesiredStateErrorDiag("Plan", err))
		return
	}

	result, err := tfcloudcontrol.Run(ctx, cloudControlClient, tfcloudcontrol.Operation{
		Kind:        tfcloudcontrol.OperationCreate,
		TypeName:    keypairCloudControlTypeName,
		Region:      r.provider.Region(ctx),
		TargetState: &targetState,
		StableToken: tfcloudcontrol.CreateOperationToken(keypairCloudControlTypeName, r.provider.CreateIdentity(ctx)),
	})
	if err != nil {
		response.Diagnostics.Append(generic.ServiceOperationErrorDiag("Cloud Control API", "CreateResource", err))
		return
	}

	// The create event is the only response that carries the private key.
	event := result.Event
	if event == nil || event.Identifier == nil {
		response.Diagnostics.Append(generic.ServiceOperationEmptyResultDiag("Cloud Control API", "CreateResource"))
		return
	}
	id := *event.Identifier

	response.State.Raw = request.Plan.Raw
	if diags := response.State.SetAttribute(ctx, path.Root("id"), id); diags.HasError() {
		response.Diagnostics.Append(diags...)
		return
	}

	// Fill all unknown attributes (computed timestamps, KeyPairId, FingerPrint,
	// and crucially PrivateKey) from a single resource-model JSON. Prefer the
	// CreateResource event's ResourceModel because it's the only response that
	// ever carries PrivateKey; fall back to GetResource if the event didn't
	// include a model.
	unknowns, err := generic.UnknownValuePaths(ctx, response.State.Raw)
	if err != nil {
		response.Diagnostics.AddError("Creation Of Terraform State Unsuccessful", err.Error())
		return
	}
	if len(unknowns) > 0 {
		var resourceModel string
		if event.ResourceModel != nil && *event.ResourceModel != "" {
			resourceModel = *event.ResourceModel
		} else {
			description, err := tfcloudcontrol.FindResourceByTypeNameAndID(ctx, cloudControlClient, r.provider.Region(ctx), keypairCloudControlTypeName, id)
			if err != nil {
				response.Diagnostics.Append(generic.ServiceOperationErrorDiag("Cloud Control API", "GetResource", err))
				return
			}
			if description == nil {
				response.Diagnostics.Append(generic.ServiceOperationEmptyResultDiag("Cloud Control API", "GetResource"))
				return
			}
			resourceModel = util.ToString(description.ResourceDescription.Properties)
		}
		if err := generic.SetUnknownValuesFromResourceModel(ctx, &response.State, unknowns, resourceModel, keypairCcToTfNameMap); err != nil {
			response.Diagnostics.AddError("Creation Of Terraform State Unsuccessful", err.Error())
			return
		}
	}
}
