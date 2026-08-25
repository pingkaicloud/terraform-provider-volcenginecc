// Copyright (c) 2025 Beijing Volcano Engine Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

// Package provider exposes the Terraform Plugin Framework provider for
// consumers that embed the official provider in another Go process.
package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/provider"
	internalprovider "github.com/volcengine/terraform-provider-volcenginecc/internal/provider"
)

// NewProvider returns a fresh Volcengine Terraform Plugin Framework provider.
func NewProvider() provider.Provider {
	return internalprovider.New()
}

// New returns a fresh Volcengine Terraform Plugin Framework provider.
func New() provider.Provider {
	return NewProvider()
}
