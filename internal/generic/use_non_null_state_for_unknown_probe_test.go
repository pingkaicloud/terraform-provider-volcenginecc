package generic

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// TestUseNonNullStateForUnknownPriorNullBehavior locks down the v1.17.0 plan
// modifier behavior that the generator relies on for nested computed children.
func TestUseNonNullStateForUnknownPriorNullBehavior(t *testing.T) {
	t.Parallel()

	t.Run("string", testStringPlanModifiers)
	t.Run("bool", testBoolPlanModifiers)
	t.Run("list", testListPlanModifiers)
	t.Run("set", testSetPlanModifiers)
	t.Run("object", testObjectPlanModifiers)
}

func testStringPlanModifiers(t *testing.T) {
	t.Parallel()

	resp := planmodifier.StringResponse{PlanValue: types.StringUnknown()}
	stringplanmodifier.UseStateForUnknown().PlanModifyString(context.Background(), planmodifier.StringRequest{
		State:       existingResourceState(),
		ConfigValue: types.StringNull(),
		PlanValue:   types.StringUnknown(),
		StateValue:  types.StringNull(),
	}, &resp)
	assertNullPlanValue(t, resp.PlanValue)

	resp = planmodifier.StringResponse{PlanValue: types.StringUnknown()}
	stringplanmodifier.UseNonNullStateForUnknown().PlanModifyString(context.Background(), planmodifier.StringRequest{
		State:       existingResourceState(),
		ConfigValue: types.StringNull(),
		PlanValue:   types.StringUnknown(),
		StateValue:  types.StringNull(),
	}, &resp)
	assertUnknownPlanValue(t, resp.PlanValue)

	resp = planmodifier.StringResponse{PlanValue: types.StringUnknown()}
	stringplanmodifier.UseNonNullStateForUnknown().PlanModifyString(context.Background(), planmodifier.StringRequest{
		State:       existingResourceState(),
		ConfigValue: types.StringNull(),
		PlanValue:   types.StringUnknown(),
		StateValue:  types.StringValue("server-filled"),
	}, &resp)
	if got, want := resp.PlanValue.ValueString(), "server-filled"; got != want {
		t.Fatalf("plan value = %q, want %q", got, want)
	}
}

func testBoolPlanModifiers(t *testing.T) {
	t.Parallel()

	resp := planmodifier.BoolResponse{PlanValue: types.BoolUnknown()}
	boolplanmodifier.UseStateForUnknown().PlanModifyBool(context.Background(), planmodifier.BoolRequest{
		State:       existingResourceState(),
		ConfigValue: types.BoolNull(),
		PlanValue:   types.BoolUnknown(),
		StateValue:  types.BoolNull(),
	}, &resp)
	assertNullPlanValue(t, resp.PlanValue)

	resp = planmodifier.BoolResponse{PlanValue: types.BoolUnknown()}
	boolplanmodifier.UseNonNullStateForUnknown().PlanModifyBool(context.Background(), planmodifier.BoolRequest{
		State:       existingResourceState(),
		ConfigValue: types.BoolNull(),
		PlanValue:   types.BoolUnknown(),
		StateValue:  types.BoolNull(),
	}, &resp)
	assertUnknownPlanValue(t, resp.PlanValue)

	resp = planmodifier.BoolResponse{PlanValue: types.BoolUnknown()}
	boolplanmodifier.UseNonNullStateForUnknown().PlanModifyBool(context.Background(), planmodifier.BoolRequest{
		State:       existingResourceState(),
		ConfigValue: types.BoolNull(),
		PlanValue:   types.BoolUnknown(),
		StateValue:  types.BoolValue(true),
	}, &resp)
	if !resp.PlanValue.ValueBool() {
		t.Fatalf("plan value = false, want true")
	}
}

func testListPlanModifiers(t *testing.T) {
	t.Parallel()

	resp := planmodifier.ListResponse{PlanValue: types.ListUnknown(types.StringType)}
	listplanmodifier.UseStateForUnknown().PlanModifyList(context.Background(), planmodifier.ListRequest{
		State:       existingResourceState(),
		ConfigValue: types.ListNull(types.StringType),
		PlanValue:   types.ListUnknown(types.StringType),
		StateValue:  types.ListNull(types.StringType),
	}, &resp)
	assertNullPlanValue(t, resp.PlanValue)

	resp = planmodifier.ListResponse{PlanValue: types.ListUnknown(types.StringType)}
	listplanmodifier.UseNonNullStateForUnknown().PlanModifyList(context.Background(), planmodifier.ListRequest{
		State:       existingResourceState(),
		ConfigValue: types.ListNull(types.StringType),
		PlanValue:   types.ListUnknown(types.StringType),
		StateValue:  types.ListNull(types.StringType),
	}, &resp)
	assertUnknownPlanValue(t, resp.PlanValue)

	priorValue, diags := types.ListValue(types.StringType, []attr.Value{types.StringValue("server-filled")})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %s", diags)
	}
	resp = planmodifier.ListResponse{PlanValue: types.ListUnknown(types.StringType)}
	listplanmodifier.UseNonNullStateForUnknown().PlanModifyList(context.Background(), planmodifier.ListRequest{
		State:       existingResourceState(),
		ConfigValue: types.ListNull(types.StringType),
		PlanValue:   types.ListUnknown(types.StringType),
		StateValue:  priorValue,
	}, &resp)
	if !resp.PlanValue.Equal(priorValue) {
		t.Fatalf("plan value = %s, want %s", resp.PlanValue, priorValue)
	}
}

func testSetPlanModifiers(t *testing.T) {
	t.Parallel()

	resp := planmodifier.SetResponse{PlanValue: types.SetUnknown(types.StringType)}
	setplanmodifier.UseStateForUnknown().PlanModifySet(context.Background(), planmodifier.SetRequest{
		State:       existingResourceState(),
		ConfigValue: types.SetNull(types.StringType),
		PlanValue:   types.SetUnknown(types.StringType),
		StateValue:  types.SetNull(types.StringType),
	}, &resp)
	assertNullPlanValue(t, resp.PlanValue)

	resp = planmodifier.SetResponse{PlanValue: types.SetUnknown(types.StringType)}
	setplanmodifier.UseNonNullStateForUnknown().PlanModifySet(context.Background(), planmodifier.SetRequest{
		State:       existingResourceState(),
		ConfigValue: types.SetNull(types.StringType),
		PlanValue:   types.SetUnknown(types.StringType),
		StateValue:  types.SetNull(types.StringType),
	}, &resp)
	assertUnknownPlanValue(t, resp.PlanValue)

	priorValue, diags := types.SetValue(types.StringType, []attr.Value{types.StringValue("server-filled")})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %s", diags)
	}
	resp = planmodifier.SetResponse{PlanValue: types.SetUnknown(types.StringType)}
	setplanmodifier.UseNonNullStateForUnknown().PlanModifySet(context.Background(), planmodifier.SetRequest{
		State:       existingResourceState(),
		ConfigValue: types.SetNull(types.StringType),
		PlanValue:   types.SetUnknown(types.StringType),
		StateValue:  priorValue,
	}, &resp)
	if !resp.PlanValue.Equal(priorValue) {
		t.Fatalf("plan value = %s, want %s", resp.PlanValue, priorValue)
	}
}

func testObjectPlanModifiers(t *testing.T) {
	t.Parallel()

	attrTypes := map[string]attr.Type{"name": types.StringType}
	resp := planmodifier.ObjectResponse{PlanValue: types.ObjectUnknown(attrTypes)}
	objectplanmodifier.UseStateForUnknown().PlanModifyObject(context.Background(), planmodifier.ObjectRequest{
		State:       existingResourceState(),
		ConfigValue: types.ObjectNull(attrTypes),
		PlanValue:   types.ObjectUnknown(attrTypes),
		StateValue:  types.ObjectNull(attrTypes),
	}, &resp)
	assertNullPlanValue(t, resp.PlanValue)

	resp = planmodifier.ObjectResponse{PlanValue: types.ObjectUnknown(attrTypes)}
	objectplanmodifier.UseNonNullStateForUnknown().PlanModifyObject(context.Background(), planmodifier.ObjectRequest{
		State:       existingResourceState(),
		ConfigValue: types.ObjectNull(attrTypes),
		PlanValue:   types.ObjectUnknown(attrTypes),
		StateValue:  types.ObjectNull(attrTypes),
	}, &resp)
	assertUnknownPlanValue(t, resp.PlanValue)

	priorValue, diags := types.ObjectValue(attrTypes, map[string]attr.Value{"name": types.StringValue("server-filled")})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %s", diags)
	}
	resp = planmodifier.ObjectResponse{PlanValue: types.ObjectUnknown(attrTypes)}
	objectplanmodifier.UseNonNullStateForUnknown().PlanModifyObject(context.Background(), planmodifier.ObjectRequest{
		State:       existingResourceState(),
		ConfigValue: types.ObjectNull(attrTypes),
		PlanValue:   types.ObjectUnknown(attrTypes),
		StateValue:  priorValue,
	}, &resp)
	if !resp.PlanValue.Equal(priorValue) {
		t.Fatalf("plan value = %s, want %s", resp.PlanValue, priorValue)
	}
}

func assertNullPlanValue(t *testing.T, value attr.Value) {
	t.Helper()

	if !value.IsNull() {
		t.Fatalf("plan value null = false, want true")
	}
	if value.IsUnknown() {
		t.Fatalf("plan value unknown = true, want false")
	}
}

func assertUnknownPlanValue(t *testing.T, value attr.Value) {
	t.Helper()

	if value.IsNull() {
		t.Fatalf("plan value null = true, want false")
	}
	if !value.IsUnknown() {
		t.Fatalf("plan value unknown = false, want true")
	}
}

// existingResourceState returns a non-null resource state so plan modifiers can
// exercise attribute-level prior null and non-null behavior.
func existingResourceState() tfsdk.State {
	return tfsdk.State{
		Raw: tftypes.NewValue(
			tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"id": tftypes.String,
				},
			},
			map[string]tftypes.Value{
				"id": tftypes.NewValue(tftypes.String, "existing-resource"),
			},
		),
	}
}
