//go:build ignore

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/volcengine/terraform-provider-volcenginecc/internal/cloudcontrol"
	"github.com/volcengine/terraform-provider-volcenginecc/internal/util"
	"github.com/volcengine/volcengine-go-sdk/volcengine"
	"github.com/volcengine/volcengine-go-sdk/volcengine/credentials/clicreds"
	"github.com/volcengine/volcengine-go-sdk/volcengine/session"
)

// main lists VPN-related Cloud Control resources across all result pages to
// locate identifiers for the reorder candidate.
func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: go run list_resources.go <cloud-control-type-name>")
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
	var nextToken *string
	for {
		out, err := client.ListResourceWithContext(ctx, &cloudcontrol.ListResourceInput{
			TypeName:   util.StringPtr(os.Args[1]),
			RegionID:   util.StringPtr("cn-beijing"),
			MaxResults: 100,
			NextToken:  nextToken,
		})
		if err != nil {
			panic(err)
		}
		for _, description := range out.ResourceDescriptions {
			var properties map[string]any
			if err := json.Unmarshal([]byte(*description.Properties), &properties); err != nil {
				panic(err)
			}
			b, err := json.Marshal(properties)
			if err != nil {
				panic(err)
			}
			fmt.Println(string(b))
		}
		if out.NextToken == nil || *out.NextToken == "" {
			break
		}
		nextToken = out.NextToken
	}
}
