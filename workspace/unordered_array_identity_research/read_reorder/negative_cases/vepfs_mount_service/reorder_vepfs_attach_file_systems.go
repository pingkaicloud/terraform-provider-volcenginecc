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

// main reverses VEPFS attached file systems through Cloud Control and compares
// the submitted order with the order returned by GetResource.
func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: go run reorder_vepfs_attach_file_systems.go <mount-service-id>")
		os.Exit(2)
	}
	ctx := context.Background()
	config := volcengine.NewConfig().
		WithRegion("cn-beijing").
		WithEndpoint("cloudcontrol.cn-beijing.volcengineapi.com").
		WithCredentials(clicreds.NewCliCredentials("/Users/bytedance/.volcengine/config.json", "volc-eps-ccapi-test"))
	sess, err := session.NewSession(config)
	if err != nil {
		panic(err)
	}
	client := cloudcontrol.New(sess)
	input := &cloudcontrol.GetResourceInput{
		TypeName:   util.StringPtr("Volcengine::VEPFS::MountService"),
		RegionID:   util.StringPtr("cn-beijing"),
		Identifier: util.StringPtr(os.Args[1]),
	}
	properties := getMountServiceProperties(ctx, client, input)
	attachFileSystems, ok := properties["AttachFileSystems"].([]any)
	if !ok {
		panic("AttachFileSystems is missing or not an array")
	}
	if len(attachFileSystems) != 2 {
		panic(fmt.Sprintf("expected two attach_file_systems elements, got %d", len(attachFileSystems)))
	}
	fmt.Println("before:")
	printAttachFileSystems(attachFileSystems)

	attachFileSystems[0], attachFileSystems[1] = attachFileSystems[1], attachFileSystems[0]
	output, err := client.UpdateResourceWithContext(ctx, &cloudcontrol.UpdateResourceInput{
		TypeName:      input.TypeName,
		RegionID:      input.RegionID,
		Identifier:    input.Identifier,
		ClientToken:   util.StringPtr(util.GenerateToken(32)),
		PatchDocument: []any{map[string]any{"op": "replace", "path": "/AttachFileSystems", "value": attachFileSystems}},
	})
	if err != nil {
		panic(err)
	}
	if output.TaskID != nil && *output.TaskID != "" {
		if _, _, err := servicecloudcontrol.AwaitTask(ctx, client, *output.TaskID); err != nil {
			panic(err)
		}
	}
	verified := getMountServiceProperties(ctx, client, input)
	verifiedAttachFileSystems, ok := verified["AttachFileSystems"].([]any)
	if !ok {
		panic("verified AttachFileSystems is missing or not an array")
	}
	fmt.Println("after:")
	printAttachFileSystems(verifiedAttachFileSystems)
}

// getMountServiceProperties retrieves and decodes the mount-service properties.
func getMountServiceProperties(ctx context.Context, client *cloudcontrol.CloudControl, input *cloudcontrol.GetResourceInput) map[string]any {
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

// printAttachFileSystems prints attached file-system identities in array order.
func printAttachFileSystems(attachFileSystems []any) {
	for i, item := range attachFileSystems {
		fileSystem := item.(map[string]any)
		fmt.Printf("%d: path=%v file_system_id=%v file_system_name=%v status=%v\n",
			i,
			fileSystem["CustomerPath"],
			fileSystem["FileSystemId"],
			fileSystem["FileSystemName"],
			fileSystem["Status"],
		)
	}
}
