//go:build ignore

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/volcengine/terraform-provider-volcenginecc/internal/registry"
	_ "github.com/volcengine/terraform-provider-volcenginecc/internal/volcengine/autoscaling"
)

type decodedValue struct {
	File  string `json:"file"`
	Index int    `json:"index"`
	Known bool   `json:"known"`
	Null  bool   `json:"null"`
	Value string `json:"value"`
}

// main decodes captured PlanResourceChange values at Volumes[*].DeleteWithInstance.
func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: go run decode_plan_resource_change.go <msgpack>...")
		os.Exit(2)
	}

	ctx := context.Background()
	factory := registry.ResourceFactories()["volcenginecc_autoscaling_scaling_configuration"]
	if factory == nil {
		panic("scaling configuration resource factory is not registered")
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

	var output []decodedValue
	for _, filename := range os.Args[1:] {
		raw, err := os.ReadFile(filename)
		if err != nil {
			panic(err)
		}
		root, err := tftypes.ValueFromMsgPack(raw, tfType)
		if err != nil {
			panic(err)
		}
		for index := 0; index < 2; index++ {
			path := tftypes.NewAttributePath().
				WithAttributeName("volumes").
				WithElementKeyInt(index).
				WithAttributeName("delete_with_instance")
			walked, _, err := tftypes.WalkAttributePath(root, path)
			if err != nil {
				panic(err)
			}
			value, ok := walked.(tftypes.Value)
			if !ok {
				panic(fmt.Sprintf("unexpected value type %T", walked))
			}
			output = append(output, decodedValue{
				File:  filename,
				Index: index,
				Known: value.IsKnown(),
				Null:  value.IsNull(),
				Value: value.String(),
			})
		}
	}

	encoded, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		panic(err)
	}
	fmt.Println(string(encoded))
}
