//go:build ignore

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/volcengine/terraform-provider-volcenginecc/internal/registry"
	_ "github.com/volcengine/terraform-provider-volcenginecc/internal/volcengine/ecs"
)

// main decodes an ECS PlanResourceChange capture and prints all differences.
func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: go run decode_plan_diff.go <prior-state.msgpack> <proposed-new-state.msgpack>")
		os.Exit(2)
	}

	ctx := context.Background()
	factory := registry.ResourceFactories()["volcenginecc_ecs_instance"]
	if factory == nil {
		panic("ECS instance resource factory is not registered")
	}
	res, err := factory(ctx)
	if err != nil {
		panic(err)
	}
	var schemaResp resource.SchemaResponse
	res.Schema(ctx, resource.SchemaRequest{}, &schemaResp)
	if schemaResp.Diagnostics.HasError() {
		panic(schemaResp.Diagnostics.Errors())
	}
	tfType := schemaResp.Schema.Type().TerraformType(ctx)

	values := make([]tftypes.Value, 0, 2)
	for _, filename := range os.Args[1:] {
		raw, err := os.ReadFile(filename)
		if err != nil {
			panic(err)
		}
		value, err := tftypes.ValueFromMsgPack(raw, tfType)
		if err != nil {
			panic(err)
		}
		values = append(values, value)
	}

	diffs, err := values[0].Diff(values[1])
	if err != nil {
		panic(err)
	}
	fmt.Printf("equal=%t diff_count=%d\n", values[0].Equal(values[1]), len(diffs))
	for _, diff := range diffs {
		fmt.Println(diff.String())
	}
}
