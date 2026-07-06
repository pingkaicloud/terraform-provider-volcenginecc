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

// main reverses security-group ingress permissions through Cloud Control to
// test whether the service preserves an order different from Terraform state.
func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: go run reorder_security_group_ingress.go <security-group-id>")
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
		TypeName:   util.StringPtr("Volcengine::VPC::SecurityGroup"),
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
	permissions, ok := properties["IngressPermissions"].([]any)
	if !ok {
		panic("IngressPermissions is missing or not an array")
	}
	if len(permissions) != 2 {
		panic(fmt.Sprintf("expected two ingress permissions, got %d", len(permissions)))
	}
	fmt.Println("before:")
	printPermissions(permissions)

	permissions[0], permissions[1] = permissions[1], permissions[0]
	output, err := client.UpdateResourceWithContext(ctx, &cloudcontrol.UpdateResourceInput{
		TypeName:      input.TypeName,
		RegionID:      input.RegionID,
		Identifier:    input.Identifier,
		ClientToken:   util.StringPtr(util.GenerateToken(32)),
		PatchDocument: []any{map[string]any{"op": "replace", "path": "/IngressPermissions", "value": permissions}},
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
	permissions, ok = properties["IngressPermissions"].([]any)
	if !ok {
		panic("verified IngressPermissions is missing or not an array")
	}
	fmt.Println("after:")
	printPermissions(permissions)
}

// printPermissions prints ingress identity fields in their current array order.
func printPermissions(permissions []any) {
	for i, permission := range permissions {
		item := permission.(map[string]any)
		fmt.Printf("%d: description=%v priority=%v port=%v-%v cidr=%v\n",
			i,
			item["Description"],
			item["Priority"],
			item["PortStart"],
			item["PortEnd"],
			item["CidrIp"],
		)
	}
}
