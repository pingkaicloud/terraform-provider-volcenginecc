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
// Control treats a repeated token as the same operation, which makes retries
// after a provider restart safe.
func OperationToken(operation, typeName, identifier, payload string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{operation, typeName, identifier, payload}, "\x00")))
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
