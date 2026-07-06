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

// main changes one ingress permission description and then restores it to
// exercise the real Cloud Control readback path without changing element identity.
func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: go run change_and_restore_ingress.go <security-group-id>")
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

	before := getIngressPermissions(ctx, client, input)
	if len(before) != 2 {
		panic(fmt.Sprintf("expected two ingress permissions, got %d", len(before)))
	}
	fmt.Println("before:")
	printIngressPermissions(before)

	first := before[0].(map[string]any)
	originalDescription, ok := first["Description"].(string)
	if !ok {
		panic("first ingress permission Description is missing or not a string")
	}

	updateIngressDescription(ctx, client, input, originalDescription+"-temporary")

	changedReadback := getIngressPermissions(ctx, client, input)
	fmt.Println("after temporary change:")
	printIngressPermissions(changedReadback)

	updateIngressDescription(ctx, client, input, originalDescription)

	after := getIngressPermissions(ctx, client, input)
	fmt.Println("after restore:")
	printIngressPermissions(after)
}

// getIngressPermissions reads the real Cloud Control resource and returns its current ingress array.
func getIngressPermissions(ctx context.Context, client *cloudcontrol.CloudControl, input *cloudcontrol.GetResourceInput) []any {
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
	return permissions
}

// updateIngressDescription changes only the first rule description and waits for the real Cloud Control task.
func updateIngressDescription(ctx context.Context, client *cloudcontrol.CloudControl, input *cloudcontrol.GetResourceInput, description string) {
	output, err := client.UpdateResourceWithContext(ctx, &cloudcontrol.UpdateResourceInput{
		TypeName:      input.TypeName,
		RegionID:      input.RegionID,
		Identifier:    input.Identifier,
		ClientToken:   util.StringPtr(util.GenerateToken(32)),
		PatchDocument: []any{map[string]any{"op": "replace", "path": "/IngressPermissions/0/Description", "value": description}},
	})
	if err != nil {
		panic(err)
	}
	if output.TaskID != nil && *output.TaskID != "" {
		if _, _, err := servicecloudcontrol.AwaitTask(ctx, client, *output.TaskID); err != nil {
			panic(err)
		}
	}
}

// printIngressPermissions prints identity-like fields and nested read-only timestamps for comparison.
func printIngressPermissions(permissions []any) {
	for i, permission := range permissions {
		item := permission.(map[string]any)
		fmt.Printf("%d: description=%v priority=%v port=%v-%v cidr=%v creation_time=%v update_time=%v\n",
			i,
			item["Description"],
			item["Priority"],
			item["PortStart"],
			item["PortEnd"],
			item["CidrIp"],
			item["CreationTime"],
			item["UpdateTime"],
		)
	}
}
