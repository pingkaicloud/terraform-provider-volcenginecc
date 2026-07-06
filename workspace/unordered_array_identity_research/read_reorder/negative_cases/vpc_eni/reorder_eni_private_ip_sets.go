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

// main reverses ENI private IP sets through Cloud Control and compares the
// submitted order with the order returned by GetResource.
func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: go run reorder_eni_private_ip_sets.go <network-interface-id>")
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
		TypeName:   util.StringPtr("Volcengine::VPC::ENI"),
		RegionID:   util.StringPtr("cn-beijing"),
		Identifier: util.StringPtr(os.Args[1]),
	}
	properties := getENIProperties(ctx, client, input)
	privateIPSets, ok := properties["PrivateIpSets"].([]any)
	if !ok {
		panic("PrivateIpSets is missing or not an array")
	}
	if len(privateIPSets) != 2 {
		panic(fmt.Sprintf("expected two private IP sets, got %d", len(privateIPSets)))
	}
	fmt.Println("before:")
	printPrivateIPSets(privateIPSets)

	privateIPSets[0], privateIPSets[1] = privateIPSets[1], privateIPSets[0]
	output, err := client.UpdateResourceWithContext(ctx, &cloudcontrol.UpdateResourceInput{
		TypeName:      input.TypeName,
		RegionID:      input.RegionID,
		Identifier:    input.Identifier,
		ClientToken:   util.StringPtr(util.GenerateToken(32)),
		PatchDocument: []any{map[string]any{"op": "replace", "path": "/PrivateIpSets", "value": privateIPSets}},
	})
	if err != nil {
		panic(err)
	}
	if output.TaskID != nil && *output.TaskID != "" {
		if _, _, err := servicecloudcontrol.AwaitTask(ctx, client, *output.TaskID); err != nil {
			panic(err)
		}
	}
	verified := getENIProperties(ctx, client, input)
	verifiedPrivateIPSets, ok := verified["PrivateIpSets"].([]any)
	if !ok {
		panic("verified PrivateIpSets is missing or not an array")
	}
	fmt.Println("after:")
	printPrivateIPSets(verifiedPrivateIPSets)
}

// getENIProperties retrieves and decodes the current ENI properties.
func getENIProperties(ctx context.Context, client *cloudcontrol.CloudControl, input *cloudcontrol.GetResourceInput) map[string]any {
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

// printPrivateIPSets prints private IP identities in their current array order.
func printPrivateIPSets(privateIPSets []any) {
	for i, item := range privateIPSets {
		privateIPSet := item.(map[string]any)
		fmt.Printf("%d: private_ip=%v primary=%v\n",
			i,
			privateIPSet["PrivateIpAddress"],
			privateIPSet["Primary"],
		)
	}
}
