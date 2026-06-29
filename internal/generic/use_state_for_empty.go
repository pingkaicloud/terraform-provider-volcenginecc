package generic

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
)

type useStateForEmptyModifier struct{}

// UseStateForEmpty returns a string plan modifier that preserves a known prior
// value when an API-specific empty configuration means the value is unspecified.
func UseStateForEmpty() planmodifier.String {
	return useStateForEmptyModifier{}
}

// Description describes the empty-string state preservation behavior.
func (useStateForEmptyModifier) Description(context.Context) string {
	return "Uses a known prior state value when the configured string is empty."
}

// MarkdownDescription describes the empty-string state preservation behavior.
func (modifier useStateForEmptyModifier) MarkdownDescription(ctx context.Context) string {
	return modifier.Description(ctx)
}

// PlanModifyString preserves known prior state only for a known empty config;
// creation and null, unknown, or non-empty configurations retain their plan.
func (useStateForEmptyModifier) PlanModifyString(_ context.Context, request planmodifier.StringRequest, response *planmodifier.StringResponse) {
	if request.ConfigValue.IsNull() || request.ConfigValue.IsUnknown() || request.ConfigValue.ValueString() != "" {
		return
	}
	if request.StateValue.IsNull() || request.StateValue.IsUnknown() {
		return
	}
	response.PlanValue = request.StateValue
}
