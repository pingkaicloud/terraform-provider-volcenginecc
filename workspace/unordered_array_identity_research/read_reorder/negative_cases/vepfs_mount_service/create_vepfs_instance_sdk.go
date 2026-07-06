//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/volcengine/volcengine-go-sdk/service/vepfs"
	"github.com/volcengine/volcengine-go-sdk/volcengine"
	"github.com/volcengine/volcengine-go-sdk/volcengine/credentials/clicreds"
	"github.com/volcengine/volcengine-go-sdk/volcengine/session"
	"github.com/volcengine/volcengine-go-sdk/volcengine/volcengineerr"
)

// main creates the temporary VEPFS instance required by the mount-service
// reorder candidate and prints the identifiers needed by the Terraform fixture.
func main() {
	profile := os.Getenv("VOLCENGINE_PROFILE")
	if profile == "" {
		profile = "default"
	}
	config := volcengine.NewConfig().
		WithRegion("cn-beijing").
		WithCredentials(clicreds.NewCliCredentials("/Users/bytedance/.volcengine/config.json", profile))
	sess, err := session.NewSession(config)
	if err != nil {
		fatal("new session", err)
	}
	client := vepfs.New(sess)

	probeOut, err := client.DescribeFileSystems(&vepfs.DescribeFileSystemsInput{
		PageNumber: int32Ptr(1),
		PageSize:   int32Ptr(10),
	})
	if err != nil {
		fatal("describe probe", err)
	}
	fmt.Printf("describe probe ok: request_id=%s count=%d\n", probeOut.Metadata.RequestId, len(probeOut.FileSystems))
	for _, fileSystem := range probeOut.FileSystems {
		fmt.Printf(
			"file_system name=%s id=%s status=%s version=%s\n",
			stringValue(fileSystem.FileSystemName),
			stringValue(fileSystem.FileSystemId),
			stringValue(fileSystem.Status),
			stringValue(fileSystem.VersionNumber),
		)
	}
	if os.Getenv("VEPFS_PROBE_ONLY") == "1" {
		return
	}

	capacity := int32(8)
	if raw := os.Getenv("VEPFS_CAPACITY"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 32)
		if err != nil {
			fatal("parse VEPFS_CAPACITY", err)
		}
		capacity = int32(parsed)
	}

	input := &vepfs.CreateFileSystemInput{
		FileSystemName: stringPtr(fmt.Sprintf("tf-vepfs-sdk-check-%s", time.Now().Format("20060102150405"))),
		ZoneId:         stringPtr("cn-beijing-a"),
		ChargeType:     stringPtr(vepfs.EnumOfChargeTypeForCreateFileSystemInputPayAsYouGo),
		FileSystemType: stringPtr(vepfs.EnumOfFileSystemTypeForCreateFileSystemInputVePfs),
		ProtocolType:   stringPtr(vepfs.EnumOfProtocolTypeForCreateFileSystemInputVePfs),
		StoreType:      stringPtr("Advance_100"),
		Project:        stringPtr("default"),
		Capacity:       int32Ptr(capacity),
		VpcId:          stringPtr(envOrDefault("VEPFS_VPC_ID", "vpc-rrco37ovjq4gv0x58zft8ul")),
		SubnetId:       stringPtr(envOrDefault("VEPFS_SUBNET_ID", "subnet-rrwqhg3qzxfkv0x57g3edcq")),
	}
	if version := os.Getenv("VEPFS_VERSION"); version != "" {
		input.VersionNumber = stringPtr(version)
	}
	printJSON("create input", input)

	createOut, err := client.CreateFileSystem(input)
	if err != nil {
		fatal("create file system", err)
	}
	printJSON("create output", createOut)

	if createOut.FileSystemId == nil || *createOut.FileSystemId == "" {
		fmt.Fprintln(os.Stderr, "create succeeded without FileSystemId; skip cleanup")
		return
	}

	describeOut, err := client.DescribeFileSystems(&vepfs.DescribeFileSystemsInput{
		FileSystemIds: createOut.FileSystemId,
		PageNumber:    int32Ptr(1),
		PageSize:      int32Ptr(10),
	})
	if err != nil {
		printError("describe created file system", err)
	} else {
		printJSON("describe created output", describeOut)
	}

	deleteOut, err := client.DeleteFileSystem(&vepfs.DeleteFileSystemInput{FileSystemId: createOut.FileSystemId})
	if err != nil {
		printError("delete file system", err)
		os.Exit(1)
	}
	printJSON("delete output", deleteOut)
}

// fatal prints a structured SDK error for the failed step and terminates.
func fatal(step string, err error) {
	printError(step, err)
	os.Exit(1)
}

// printError reports both the generic error and any structured SDK response.
func printError(step string, err error) {
	fmt.Fprintf(os.Stderr, "%s failed: %v\n", step, err)
	if requestFailure, ok := err.(volcengineerr.RequestFailure); ok {
		fmt.Fprintf(os.Stderr, "status_code=%d request_id=%s code=%s message=%s\n", requestFailure.StatusCode(), requestFailure.RequestID(), requestFailure.Code(), requestFailure.Message())
		return
	}
	if sdkErr, ok := err.(volcengineerr.Error); ok {
		fmt.Fprintf(os.Stderr, "code=%s message=%s\n", sdkErr.Code(), sdkErr.Message())
	}
}

// printJSON writes a labeled, indented representation of an SDK value.
func printJSON(label string, value any) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		fmt.Printf("%s: %#v\n", label, value)
		return
	}
	fmt.Printf("%s:\n%s\n", label, string(data))
}

// stringPtr returns a pointer to value for SDK request fields.
func stringPtr(value string) *string {
	return &value
}

// int32Ptr returns a pointer to value for SDK request fields.
func int32Ptr(value int32) *int32 {
	return &value
}

// stringValue safely formats an optional SDK string field.
func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// envOrDefault returns an environment override or the existing repro default.
func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
