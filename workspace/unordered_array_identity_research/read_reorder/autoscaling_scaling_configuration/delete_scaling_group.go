//go:build ignore

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/volcengine/terraform-provider-volcenginecc/internal/cloudcontrol"
	servicecloudcontrol "github.com/volcengine/terraform-provider-volcenginecc/internal/service/cloudcontrol"
	"github.com/volcengine/terraform-provider-volcenginecc/internal/util"
	"github.com/volcengine/volcengine-go-sdk/volcengine"
	"github.com/volcengine/volcengine-go-sdk/volcengine/credentials/clicreds"
	"github.com/volcengine/volcengine-go-sdk/volcengine/session"
)

// main deletes the temporary scaling group used by the AutoScaling identity
// reproduction after Terraform can no longer remove it in dependency order.
func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: go run delete_scaling_group.go <scaling-group-id>")
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
	output, err := client.DeleteResourceWithContext(ctx, &cloudcontrol.DeleteResourceInput{
		TypeName:    util.StringPtr("Volcengine::AutoScaling::ScalingGroup"),
		RegionID:    util.StringPtr("cn-beijing"),
		Identifier:  util.StringPtr(os.Args[1]),
		ClientToken: util.StringPtr(util.GenerateToken(32)),
	})
	if err != nil {
		panic(err)
	}
	if output.TaskID != nil && *output.TaskID != "" {
		if _, _, err := servicecloudcontrol.AwaitTask(ctx, client, *output.TaskID); err != nil {
			panic(err)
		}
	}
	fmt.Println("deleted scaling group", os.Args[1])
}
