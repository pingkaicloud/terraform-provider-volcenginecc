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

// main reverses RDS PostgreSQL node information through Cloud Control and
// compares the submitted order with the order returned by GetResource.
func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: go run reorder_rdspostgresql_node_info.go <instance-id>")
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
		TypeName:   util.StringPtr("Volcengine::RDSPostgreSQL::Instance"),
		RegionID:   util.StringPtr("cn-beijing"),
		Identifier: util.StringPtr(os.Args[1]),
	}
	properties := getPostgreSQLProperties(ctx, client, input)
	nodeInfo, ok := properties["NodeInfo"].([]any)
	if !ok {
		panic("NodeInfo is missing or not an array")
	}
	if len(nodeInfo) != 2 {
		panic(fmt.Sprintf("expected two node_info elements, got %d", len(nodeInfo)))
	}
	fmt.Println("before:")
	printNodeInfo(nodeInfo)

	nodeInfo[0], nodeInfo[1] = nodeInfo[1], nodeInfo[0]
	output, err := client.UpdateResourceWithContext(ctx, &cloudcontrol.UpdateResourceInput{
		TypeName:      input.TypeName,
		RegionID:      input.RegionID,
		Identifier:    input.Identifier,
		ClientToken:   util.StringPtr(util.GenerateToken(32)),
		PatchDocument: []any{map[string]any{"op": "replace", "path": "/NodeInfo", "value": nodeInfo}},
	})
	if err != nil {
		panic(err)
	}
	if output.TaskID != nil && *output.TaskID != "" {
		if _, _, err := servicecloudcontrol.AwaitTask(ctx, client, *output.TaskID); err != nil {
			panic(err)
		}
	}
	verified := getPostgreSQLProperties(ctx, client, input)
	verifiedNodeInfo, ok := verified["NodeInfo"].([]any)
	if !ok {
		panic("verified NodeInfo is missing or not an array")
	}
	fmt.Println("after:")
	printNodeInfo(verifiedNodeInfo)
}

// getPostgreSQLProperties retrieves and decodes the current instance properties.
func getPostgreSQLProperties(ctx context.Context, client *cloudcontrol.CloudControl, input *cloudcontrol.GetResourceInput) map[string]any {
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

// printNodeInfo prints the identity-relevant node fields in their current order.
func printNodeInfo(nodeInfo []any) {
	for i, item := range nodeInfo {
		node := item.(map[string]any)
		fmt.Printf("%d: zone=%v type=%v spec=%v node_id=%v status=%v\n",
			i,
			node["ZoneId"],
			node["NodeType"],
			node["NodeSpec"],
			node["NodeId"],
			node["NodeStatus"],
		)
	}
}
