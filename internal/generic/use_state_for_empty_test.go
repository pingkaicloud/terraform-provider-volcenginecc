package generic

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestUseStateForEmpty(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		config types.String
		state  types.String
		plan   types.String
		want   types.String
	}{
		"empty config uses known state": {
			config: types.StringValue(""),
			state:  types.StringValue("192.168.0.10"),
			plan:   types.StringValue(""),
			want:   types.StringValue("192.168.0.10"),
		},
		"creation keeps empty config": {
			config: types.StringValue(""),
			state:  types.StringNull(),
			plan:   types.StringValue(""),
			want:   types.StringValue(""),
		},
		"non-empty config keeps plan": {
			config: types.StringValue("192.168.0.20"),
			state:  types.StringValue("192.168.0.10"),
			plan:   types.StringValue("192.168.0.20"),
			want:   types.StringValue("192.168.0.20"),
		},
		"null config keeps plan": {
			config: types.StringNull(),
			state:  types.StringValue("192.168.0.10"),
			plan:   types.StringUnknown(),
			want:   types.StringUnknown(),
		},
		"unknown state keeps plan": {
			config: types.StringValue(""),
			state:  types.StringUnknown(),
			plan:   types.StringValue(""),
			want:   types.StringValue(""),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			request := planmodifier.StringRequest{
				ConfigValue: test.config,
				StateValue:  test.state,
				PlanValue:   test.plan,
			}
			response := planmodifier.StringResponse{PlanValue: test.plan}
			UseStateForEmpty().PlanModifyString(context.Background(), request, &response)
			if !response.PlanValue.Equal(test.want) {
				t.Fatalf("PlanValue = %s, want %s", response.PlanValue, test.want)
			}
		})
	}
}
