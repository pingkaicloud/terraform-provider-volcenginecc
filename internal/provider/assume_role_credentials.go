// Copyright (c) HashiCorp, Inc.
// Copyright (c) 2025 Beijing Volcano Engine Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"time"

	"github.com/volcengine/volcengine-go-sdk/volcengine/credentials"
)

type assumeRoleRetriever func(credentials.StsValue) (credentials.Value, error)

type sourceAssumeRoleProvider struct {
	credentials.Expiry
	source   *credentials.Credentials
	stsValue credentials.StsValue
	retrieve assumeRoleRetriever
	validate sourceCredentialsValidator
}

// newAssumeRoleCredentials creates refreshable target-role credentials backed by source.
// Every target refresh obtains the latest source AK/SK/session token before calling STS.
func newAssumeRoleCredentials(source *credentials.Credentials, stsValue credentials.StsValue, validate sourceCredentialsValidator) *credentials.Credentials {
	return newAssumeRoleCredentialsWithRetriever(source, stsValue, validate, retrieveAssumeRoleCredentials)
}

// newAssumeRoleCredentialsWithRetriever creates source-aware AssumeRole credentials with
// an injectable STS retrieval function for deterministic tests.
func newAssumeRoleCredentialsWithRetriever(source *credentials.Credentials, stsValue credentials.StsValue, validate sourceCredentialsValidator, retrieve assumeRoleRetriever) *credentials.Credentials {
	return credentials.NewExpireAbleCredentials(&sourceAssumeRoleProvider{
		source:   source,
		stsValue: stsValue,
		retrieve: retrieve,
		validate: validate,
	})
}

// Retrieve refreshes the source credentials first, then uses those exact values to obtain
// target-role credentials. This preserves refresh behavior for temporary Profile credentials.
func (p *sourceAssumeRoleProvider) Retrieve() (credentials.Value, error) {
	if p.validate != nil {
		if err := p.validate(); err != nil {
			return credentials.Value{ProviderName: "SourceAssumeRoleProvider"}, fmt.Errorf("validate AssumeRole source credentials: %w", err)
		}
	}

	sourceValue, err := p.source.Get()
	if err != nil {
		return credentials.Value{ProviderName: "SourceAssumeRoleProvider"}, fmt.Errorf("retrieve AssumeRole source credentials: %w", err)
	}
	if !sourceValue.HasKeys() {
		return credentials.Value{ProviderName: "SourceAssumeRoleProvider"}, fmt.Errorf("retrieve AssumeRole source credentials: access key or secret key is empty")
	}

	stsValue := p.stsValue
	stsValue.AccessKey = sourceValue.AccessKeyID
	stsValue.SecurityKey = sourceValue.SecretAccessKey
	stsValue.SessionToken = sourceValue.SessionToken

	targetValue, err := p.retrieve(stsValue)
	if err != nil {
		return credentials.Value{ProviderName: "SourceAssumeRoleProvider"}, fmt.Errorf("assume target role: %w", err)
	}
	if !targetValue.HasKeys() {
		return credentials.Value{ProviderName: "SourceAssumeRoleProvider"}, fmt.Errorf("assume target role: STS returned empty credentials")
	}

	p.SetExpiration(time.Now().Add(time.Duration(stsValue.DurationSeconds)*time.Second), time.Minute)
	targetValue.ProviderName = "SourceAssumeRoleProvider"
	return targetValue, nil
}

// retrieveAssumeRoleCredentials obtains one set of target-role credentials through the
// Volcengine SDK. The surrounding provider owns caching and refresh timing.
func retrieveAssumeRoleCredentials(stsValue credentials.StsValue) (credentials.Value, error) {
	targetCredentials := credentials.NewStsCredentials(stsValue)
	return targetCredentials.Get()
}
