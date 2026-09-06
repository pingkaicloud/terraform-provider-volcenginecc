// Copyright (c) 2025 Beijing Volcano Engine Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package cloudcontrol

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/volcengine/terraform-provider-volcenginecc/internal/util"
)

// OperationToken returns a deterministic Cloud Control client token. Cloud
// Control treats a repeated token as the same operation, which makes resuming
// an in-flight task after a provider restart safe.
//
// The same token also replays a task that already FAILED, so callers must not
// reuse it for a fresh attempt after a failure; see AttemptToken and Run.
func OperationToken(operation, typeName, identifier, payload string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{operation, typeName, identifier, payload}, "\x00")))
	return hex.EncodeToString(sum[:])[:32]
}

// UpdateOperationToken is the stable token for applying patchDocument to id.
func UpdateOperationToken(typeName, id, patchDocument string) string {
	return OperationToken("update", typeName, id, patchDocument)
}

// DeleteOperationToken is the stable token for deleting id.
func DeleteOperationToken(typeName, id string) string {
	return OperationToken("delete", typeName, id, "")
}

// AttemptToken derives a fresh token for a new attempt of the operation that
// stableToken identifies. It is unique per call so Cloud Control starts a new
// task instead of replaying a FAILED one.
func AttemptToken(stableToken string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{stableToken, util.GenerateToken(32)}, "\x00")))
	return hex.EncodeToString(sum[:])[:32]
}

// CreateOperationToken returns a token for a logical resource create. Embedded
// controller usage supplies a stable resource identity (the Crossplane UID),
// while direct Terraform usage has no resource address available at this
// layer and therefore uses a unique token for each create request.
func CreateOperationToken(typeName, resourceIdentity string) string {
	if resourceIdentity != "" {
		return OperationToken("create", typeName, resourceIdentity, "")
	}
	return util.GenerateToken(32)
}
