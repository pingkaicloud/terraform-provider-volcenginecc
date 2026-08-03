// Copyright (c) HashiCorp, Inc.
// Copyright (c) 2025 Beijing Volcano Engine Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const profileModeRamRoleArn = "ramrolearn"

type profileModeConfig struct {
	Current  string                        `json:"current"`
	Profiles map[string]*profileModeRecord `json:"profiles"`
}

type profileModeRecord struct {
	Mode string `json:"mode"`
}

// validateProfileAssumeRoleMode rejects a Profile that already performs RAM role
// assumption. V1 supports one Provider-level AssumeRole hop and leaves role chaining
// to the dedicated follow-up requirement.
func validateProfileAssumeRoleMode(configPath, profileName string) error {
	resolvedPath, err := resolveProfileConfigPath(configPath)
	if err != nil {
		return err
	}

	content, err := os.ReadFile(resolvedPath)
	if err != nil {
		return fmt.Errorf("read profile configuration %s: %w", resolvedPath, err)
	}

	var config profileModeConfig
	if err := json.Unmarshal(content, &config); err != nil {
		return fmt.Errorf("parse profile configuration %s: %w", resolvedPath, err)
	}

	resolvedProfile := profileName
	if resolvedProfile == "" {
		resolvedProfile = os.Getenv("VOLCENGINE_PROFILE")
	}
	if resolvedProfile == "" {
		resolvedProfile = os.Getenv("VOLCSTACK_PROFILE")
	}
	if resolvedProfile == "" {
		resolvedProfile = config.Current
	}
	if resolvedProfile == "" {
		resolvedProfile = "default"
	}

	profile, ok := config.Profiles[resolvedProfile]
	if !ok || profile == nil {
		return fmt.Errorf("profile %q was not found in %s", resolvedProfile, resolvedPath)
	}
	if strings.EqualFold(strings.TrimSpace(profile.Mode), profileModeRamRoleArn) {
		return fmt.Errorf("profile %q already contains a RAM role chain; V1 does not support another AssumeRole hop", resolvedProfile)
	}
	return nil
}

// resolveProfileConfigPath applies the same default-path rules as the SDK CLI
// credentials provider so validation and credential retrieval inspect the same file.
func resolveProfileConfigPath(configPath string) (string, error) {
	if configPath != "" {
		return configPath, nil
	}
	if envPath := os.Getenv("VOLCENGINE_CLI_CONFIG_FILE"); envPath != "" {
		return envPath, nil
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home for profile configuration: %w", err)
	}
	return filepath.Join(homeDir, ".volcengine", "config.json"), nil
}
