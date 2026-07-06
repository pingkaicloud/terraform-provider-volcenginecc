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

// main reverses VPN tunnel options through Cloud Control and compares the
// submitted order with the order returned by GetResource.
func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: go run reorder_vpn_tunnel_options.go <vpn-connection-id>")
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
		TypeName:   util.StringPtr("Volcengine::VPN::VPNConnection"),
		RegionID:   util.StringPtr("cn-beijing"),
		Identifier: util.StringPtr(os.Args[1]),
	}
	properties := getVPNProperties(ctx, client, input)
	tunnelOptions, ok := properties["TunnelOptions"].([]any)
	if !ok {
		panic("TunnelOptions is missing or not an array")
	}
	if len(tunnelOptions) != 2 {
		panic(fmt.Sprintf("expected two tunnel options, got %d", len(tunnelOptions)))
	}
	fmt.Println("before:")
	printTunnelOptions(tunnelOptions)

	tunnelOptions[0], tunnelOptions[1] = tunnelOptions[1], tunnelOptions[0]
	output, err := client.UpdateResourceWithContext(ctx, &cloudcontrol.UpdateResourceInput{
		TypeName:      input.TypeName,
		RegionID:      input.RegionID,
		Identifier:    input.Identifier,
		ClientToken:   util.StringPtr(util.GenerateToken(32)),
		PatchDocument: []any{map[string]any{"op": "replace", "path": "/TunnelOptions", "value": tunnelOptions}},
	})
	if err != nil {
		panic(err)
	}
	if output.TaskID != nil && *output.TaskID != "" {
		if _, _, err := servicecloudcontrol.AwaitTask(ctx, client, *output.TaskID); err != nil {
			panic(err)
		}
	}
	verified := getVPNProperties(ctx, client, input)
	verifiedTunnelOptions, ok := verified["TunnelOptions"].([]any)
	if !ok {
		panic("verified TunnelOptions is missing or not an array")
	}
	fmt.Println("after:")
	printTunnelOptions(verifiedTunnelOptions)
}

// getVPNProperties retrieves and decodes the current VPN connection properties.
func getVPNProperties(ctx context.Context, client *cloudcontrol.CloudControl, input *cloudcontrol.GetResourceInput) map[string]any {
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

// printTunnelOptions prints tunnel identities in their current array order.
func printTunnelOptions(tunnelOptions []any) {
	for i, item := range tunnelOptions {
		tunnelOption := item.(map[string]any)
		fmt.Printf("%d: role=%v customer_gateway_id=%v tunnel_id=%v connect_status=%v\n",
			i,
			tunnelOption["Role"],
			tunnelOption["CustomerGatewayId"],
			tunnelOption["TunnelId"],
			tunnelOption["ConnectStatus"],
		)
	}
}
