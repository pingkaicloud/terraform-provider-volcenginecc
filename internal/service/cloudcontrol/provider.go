// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package cloudcontrol

import (
	"context"

	"github.com/volcengine/terraform-provider-volcenginecc/internal/cloudcontrol"
)

type Provider interface {
	CloudControlAPIClient(context.Context) *cloudcontrol.CloudControl

	Region(ctx context.Context) string

	// CreateIdentity returns the stable identity of the logical resource being
	// created, when the provider is embedded by a controller such as Upjet.
	CreateIdentity(ctx context.Context) string

	RegisterLogger(ctx context.Context) context.Context
}
