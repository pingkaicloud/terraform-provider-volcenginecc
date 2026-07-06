//go:build ignore

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/volcengine/terraform-provider-volcenginecc/internal/cloudcontrol"
	servicecloudcontrol "github.com/volcengine/terraform-provider-volcenginecc/internal/service/cloudcontrol"
	"github.com/volcengine/terraform-provider-volcenginecc/internal/util"
	"github.com/volcengine/volcengine-go-sdk/volcengine"
	"github.com/volcengine/volcengine-go-sdk/volcengine/credentials/clicreds"
	"github.com/volcengine/volcengine-go-sdk/volcengine/session"
)

// main reverses ScalingConfiguration Volumes through Cloud Control so the
// subsequent GetResource call can verify whether remote order is preserved.
func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: go run reorder_scaling_configuration_volumes.go <scaling-configuration-id>")
		os.Exit(2)
	}
	ctx := context.Background()
	config := volcengine.NewConfig().
		WithRegion("cn-beijing").
		WithEndpoint("cloudcontrol.cn-beijing.volcengineapi.com").
		WithCredentials(clicreds.NewCliCredentials("/Users/bytedance/.volcengine/config.json", "default"))
	sess, err := session.NewSession(config)
	if err != nil {
		panic(err)
	}
	client := cloudcontrol.New(sess)
	input := &cloudcontrol.GetResourceInput{
		TypeName:   util.StringPtr("Volcengine::AutoScaling::ScalingConfiguration"),
		RegionID:   util.StringPtr("cn-beijing"),
		Identifier: util.StringPtr(os.Args[1]),
	}
	got, err := client.GetResourceWithContext(ctx, input)
	if err != nil {
		panic(err)
	}
	var properties map[string]any
	if err := json.Unmarshal([]byte(*got.ResourceDescription.Properties), &properties); err != nil {
		panic(err)
	}
	volumes := properties["Volumes"].([]any)
	if len(volumes) != 2 {
		panic(fmt.Sprintf("expected two volumes, got %d", len(volumes)))
	}
	volumes[0], volumes[1] = volumes[1], volumes[0]
	output, err := client.UpdateResourceWithContext(ctx, &cloudcontrol.UpdateResourceInput{
		TypeName:      input.TypeName,
		RegionID:      input.RegionID,
		Identifier:    input.Identifier,
		ClientToken:   util.StringPtr(util.GenerateToken(32)),
		PatchDocument: []any{map[string]any{"op": "replace", "path": "/Volumes", "value": volumes}},
	})
	if err != nil {
		panic(err)
	}
	if output.TaskID != nil && *output.TaskID != "" {
		if _, _, err := servicecloudcontrol.AwaitTask(ctx, client, *output.TaskID); err != nil {
			panic(err)
		}
	}
	verified, err := client.GetResourceWithContext(ctx, input)
	if err != nil {
		panic(err)
	}
	if err := json.Unmarshal([]byte(*verified.ResourceDescription.Properties), &properties); err != nil {
		panic(err)
	}
	for i, volume := range properties["Volumes"].([]any) {
		fmt.Printf("%d: size=%v\n", i, volume.(map[string]any)["Size"])
	}
}
