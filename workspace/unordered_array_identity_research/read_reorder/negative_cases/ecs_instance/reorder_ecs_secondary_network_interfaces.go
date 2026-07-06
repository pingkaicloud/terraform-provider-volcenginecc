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

// main reverses ECS secondary network interfaces through Cloud Control and
// compares the submitted order with the order returned by GetResource.
func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: go run reorder_ecs_secondary_network_interfaces.go <instance-id>")
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
		TypeName:   util.StringPtr("Volcengine::ECS::Instance"),
		RegionID:   util.StringPtr("cn-beijing"),
		Identifier: util.StringPtr(os.Args[1]),
	}
	properties := getProperties(ctx, client, input)
	interfaces, ok := properties["SecondaryNetworkInterfaces"].([]any)
	if !ok {
		panic("SecondaryNetworkInterfaces is missing or not an array")
	}
	if len(interfaces) != 2 {
		panic(fmt.Sprintf("expected two secondary network interfaces, got %d", len(interfaces)))
	}
	fmt.Println("before:")
	printSecondaryNetworkInterfaces(interfaces)

	interfaces[0], interfaces[1] = interfaces[1], interfaces[0]
	output, err := client.UpdateResourceWithContext(ctx, &cloudcontrol.UpdateResourceInput{
		TypeName:      input.TypeName,
		RegionID:      input.RegionID,
		Identifier:    input.Identifier,
		ClientToken:   util.StringPtr(util.GenerateToken(32)),
		PatchDocument: []any{map[string]any{"op": "replace", "path": "/SecondaryNetworkInterfaces", "value": interfaces}},
	})
	if err != nil {
		panic(err)
	}
	if output.TaskID != nil && *output.TaskID != "" {
		if _, _, err := servicecloudcontrol.AwaitTask(ctx, client, *output.TaskID); err != nil {
			panic(err)
		}
	}
	verified := getProperties(ctx, client, input)
	verifiedInterfaces, ok := verified["SecondaryNetworkInterfaces"].([]any)
	if !ok {
		panic("verified SecondaryNetworkInterfaces is missing or not an array")
	}
	fmt.Println("after:")
	printSecondaryNetworkInterfaces(verifiedInterfaces)
}

// getProperties retrieves and decodes the current ECS Cloud Control properties.
func getProperties(ctx context.Context, client *cloudcontrol.CloudControl, input *cloudcontrol.GetResourceInput) map[string]any {
	got, err := client.GetResourceWithContext(ctx, input)
	if err != nil {
		panic(err)
	}
	var properties map[string]any
	if err := json.Unmarshal([]byte(*got.ResourceDescription.Properties), &properties); err != nil {
		panic(err)
	}
	return properties
}

// printSecondaryNetworkInterfaces prints the identity-relevant fields in their
// current array order so service-side canonicalization is visible.
func printSecondaryNetworkInterfaces(interfaces []any) {
	for i, item := range interfaces {
		networkInterface := item.(map[string]any)
		fmt.Printf("%d: subnet=%v primary_ip=%v eni=%v\n",
			i,
			networkInterface["SubnetId"],
			networkInterface["PrimaryIpAddress"],
			networkInterface["NetworkInterfaceId"],
		)
	}
}
