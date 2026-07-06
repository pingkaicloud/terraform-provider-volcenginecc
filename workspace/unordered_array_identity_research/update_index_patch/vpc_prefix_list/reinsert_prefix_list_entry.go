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

// main removes and reinserts a PrefixList entry so remote order becomes [B, A]
// before the Terraform update verifies identity-aware patch indexes.
func main() {
	if len(os.Args) != 3 || (os.Args[1] != "read" && os.Args[1] != "reinsert" && os.Args[1] != "reverse") {
		fmt.Fprintln(os.Stderr, "usage: go run reinsert_prefix_list_entry.go <read|reinsert|reverse> <prefix-list-id>")
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
	typeName := util.StringPtr("Volcengine::VPC::PrefixList")
	regionID := util.StringPtr("cn-beijing")
	identifier := util.StringPtr(os.Args[2])
	properties := getProperties(ctx, client, typeName, regionID, identifier)
	entries := properties["PrefixListEntries"].([]any)
	printEntries("before", entries)
	if os.Args[1] == "read" {
		return
	}
	if os.Args[1] == "reverse" {
		reversed := make([]any, len(entries))
		for i := range entries {
			reversed[len(entries)-1-i] = entries[i]
		}
		update(ctx, client, typeName, regionID, identifier, []any{
			map[string]any{"op": "replace", "path": "/PrefixListEntries", "value": reversed},
		})
		properties = getProperties(ctx, client, typeName, regionID, identifier)
		printEntries("after", properties["PrefixListEntries"].([]any))
		return
	}

	index := -1
	var target map[string]any
	for i, value := range entries {
		entry := value.(map[string]any)
		if entry["Cidr"] == "10.252.0.128/25" {
			index = i
			target = entry
			break
		}
	}
	if index < 0 {
		panic("target prefix list entry not found")
	}
	update(ctx, client, typeName, regionID, identifier, []any{
		map[string]any{"op": "remove", "path": fmt.Sprintf("/PrefixListEntries/%d", index)},
	})
	update(ctx, client, typeName, regionID, identifier, []any{
		map[string]any{"op": "add", "path": "/PrefixListEntries/-", "value": target},
	})
	properties = getProperties(ctx, client, typeName, regionID, identifier)
	printEntries("after", properties["PrefixListEntries"].([]any))
}

// getProperties returns the current Cloud Control resource properties.
func getProperties(ctx context.Context, client *cloudcontrol.CloudControl, typeName, regionID, identifier *string) map[string]any {
	output, err := client.GetResourceWithContext(ctx, &cloudcontrol.GetResourceInput{
		TypeName:   typeName,
		RegionID:   regionID,
		Identifier: identifier,
	})
	if err != nil {
		panic(err)
	}
	var properties map[string]any
	if err := json.Unmarshal([]byte(*output.ResourceDescription.Properties), &properties); err != nil {
		panic(err)
	}
	return properties
}

// update applies a patch and waits until Cloud Control finishes it.
func update(ctx context.Context, client *cloudcontrol.CloudControl, typeName, regionID, identifier *string, patch []any) {
	fmt.Printf("patch: %v\n", patch)
	output, err := client.UpdateResourceWithContext(ctx, &cloudcontrol.UpdateResourceInput{
		TypeName:      typeName,
		RegionID:      regionID,
		Identifier:    identifier,
		ClientToken:   util.StringPtr(util.GenerateToken(32)),
		PatchDocument: patch,
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

// printEntries prints the remote array order used by the stage F assertion.
func printEntries(label string, entries []any) {
	fmt.Println(label + ":")
	for i, value := range entries {
		entry := value.(map[string]any)
		fmt.Printf("%d: Cidr=%v Description=%v\n", i, entry["Cidr"], entry["Description"])
	}
}
