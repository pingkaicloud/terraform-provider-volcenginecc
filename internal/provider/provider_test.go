// Copyright (c) HashiCorp, Inc.
// Copyright (c) 2025 Beijing Volcano Engine Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	frameworkprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	providerschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/volcengine/terraform-provider-volcenginecc/internal/cloudcontrol"
	"github.com/volcengine/volcengine-go-sdk/volcengine"
)

const testProxyAuthorization = "Basic WlRJX3Rlc3Q6dGVzdC10b2tlbg=="

func TestProviderProxyAuthorizationSchema(t *testing.T) {
	var response frameworkprovider.SchemaResponse
	(&VolcengineCCProvider{}).Schema(context.Background(), frameworkprovider.SchemaRequest{}, &response)

	attribute, ok := response.Schema.Attributes["proxy_authorization"].(providerschema.StringAttribute)
	if !ok {
		t.Fatal("proxy_authorization is not a string attribute")
	}
	if !attribute.Optional {
		t.Error("proxy_authorization must be optional")
	}
	if !attribute.Sensitive {
		t.Error("proxy_authorization must be sensitive")
	}

	noProxyAttribute, ok := response.Schema.Attributes["no_proxy"].(providerschema.StringAttribute)
	if !ok {
		t.Fatal("no_proxy is not a string attribute")
	}
	if !noProxyAttribute.Optional {
		t.Error("no_proxy must be optional")
	}

	includeAttribute, ok := response.Schema.Attributes["proxy_include_domains"].(providerschema.SetAttribute)
	if !ok {
		t.Fatal("proxy_include_domains is not a set attribute")
	}
	if !includeAttribute.Optional {
		t.Error("proxy_include_domains must be optional")
	}
	if !includeAttribute.ElementType.Equal(types.StringType) {
		t.Fatalf("proxy_include_domains element type = %T, want string", includeAttribute.ElementType)
	}
}

func TestSetProxyDefaultsFromEnvironment(t *testing.T) {
	t.Run("environment values are used when configuration is absent", func(t *testing.T) {
		clearProxyEnvironment(t)
		t.Setenv("VOLCENGINE_PROXY_URL", "http://proxy-from-env.example:8080")
		t.Setenv("VOLCENGINE_PROXY_AUTHORIZATION", "Basic from-environment")
		t.Setenv("VOLCENGINE_NO_PROXY", "localhost,.internal.example")
		t.Setenv("NO_PROXY", "ignored.example")
		config := &configModel{
			ProxyURL:            types.StringNull(),
			ProxyAuthorization:  types.StringNull(),
			NoProxy:             types.StringNull(),
			ProxyIncludeDomains: types.SetNull(types.StringType),
		}

		setProxyDefaultsFromEnvironment(config)

		if got, want := config.ProxyURL.ValueString(), "http://proxy-from-env.example:8080"; got != want {
			t.Fatalf("proxy URL = %q, want %q", got, want)
		}
		if got, want := config.ProxyAuthorization.ValueString(), "Basic from-environment"; got != want {
			t.Fatalf("proxy authorization = %q, want %q", got, want)
		}
		if got, want := config.NoProxy.ValueString(), "localhost,.internal.example"; got != want {
			t.Fatalf("no_proxy = %q, want %q", got, want)
		}
	})

	t.Run("standard NO_PROXY is used as a fallback", func(t *testing.T) {
		clearProxyEnvironment(t)
		t.Setenv("NO_PROXY", ".standard.example")
		t.Setenv("no_proxy", ".lowercase.example")
		config := &configModel{
			ProxyURL:            types.StringNull(),
			ProxyAuthorization:  types.StringNull(),
			NoProxy:             types.StringNull(),
			ProxyIncludeDomains: types.SetNull(types.StringType),
		}

		setProxyDefaultsFromEnvironment(config)

		if got, want := config.NoProxy.ValueString(), ".standard.example"; got != want {
			t.Fatalf("no_proxy = %q, want %q", got, want)
		}
	})

	t.Run("proxy include domains suppress inherited NO_PROXY", func(t *testing.T) {
		clearProxyEnvironment(t)
		t.Setenv("VOLCENGINE_PROXY_INCLUDE_DOMAINS", " cloudcontrol.cn-beijing.volcengineapi.com,api.example.com,api.example.com ")
		t.Setenv("NO_PROXY", ".inherited.example")
		config := &configModel{
			ProxyURL:            types.StringNull(),
			ProxyAuthorization:  types.StringNull(),
			NoProxy:             types.StringNull(),
			ProxyIncludeDomains: types.SetNull(types.StringType),
		}

		setProxyDefaultsFromEnvironment(config)

		got, err := proxyIncludeDomainsFromConfig(config.ProxyIncludeDomains)
		if err != nil {
			t.Fatal(err)
		}
		if fmt.Sprint(got) != "[cloudcontrol.cn-beijing.volcengineapi.com api.example.com]" {
			t.Fatalf("proxy include domains = %v", got)
		}
		if !config.NoProxy.IsNull() {
			t.Fatalf("inherited NO_PROXY must not conflict with proxy_include_domains: %q", config.NoProxy.ValueString())
		}
	})

	t.Run("explicit values take precedence", func(t *testing.T) {
		clearProxyEnvironment(t)
		t.Setenv("VOLCENGINE_PROXY_URL", "http://proxy-from-env.example:8080")
		t.Setenv("VOLCENGINE_PROXY_AUTHORIZATION", "Basic from-environment")
		t.Setenv("VOLCENGINE_NO_PROXY", ".environment.example")
		config := &configModel{
			ProxyURL:            types.StringValue("http://proxy-from-config.example:8080"),
			ProxyAuthorization:  types.StringValue("Basic from-config"),
			NoProxy:             types.StringValue(".configuration.example"),
			ProxyIncludeDomains: types.SetNull(types.StringType),
		}

		setProxyDefaultsFromEnvironment(config)

		if got, want := config.ProxyURL.ValueString(), "http://proxy-from-config.example:8080"; got != want {
			t.Fatalf("proxy URL = %q, want %q", got, want)
		}
		if got, want := config.ProxyAuthorization.ValueString(), "Basic from-config"; got != want {
			t.Fatalf("proxy authorization = %q, want %q", got, want)
		}
		if got, want := config.NoProxy.ValueString(), ".configuration.example"; got != want {
			t.Fatalf("no_proxy = %q, want %q", got, want)
		}
	})

	t.Run("explicit empty values disable environment values", func(t *testing.T) {
		clearProxyEnvironment(t)
		t.Setenv("VOLCENGINE_PROXY_URL", "http://proxy-from-env.example:8080")
		t.Setenv("VOLCENGINE_PROXY_AUTHORIZATION", "Basic from-environment")
		t.Setenv("VOLCENGINE_NO_PROXY", ".environment.example")
		t.Setenv("VOLCENGINE_PROXY_INCLUDE_DOMAINS", ".included.example")
		config := &configModel{
			ProxyURL:            types.StringValue(""),
			ProxyAuthorization:  types.StringValue(""),
			NoProxy:             types.StringValue(""),
			ProxyIncludeDomains: proxyStringSet(),
		}

		setProxyDefaultsFromEnvironment(config)

		if config.ProxyURL.ValueString() != "" || config.ProxyAuthorization.ValueString() != "" || config.NoProxy.ValueString() != "" || len(config.ProxyIncludeDomains.Elements()) != 0 {
			t.Fatal("explicit empty proxy configuration must take precedence over environment values")
		}
	})
}

func clearProxyEnvironment(t *testing.T) {
	t.Helper()
	for _, name := range []string{"VOLCENGINE_PROXY_URL", "VOLCENGINE_PROXY_AUTHORIZATION", "VOLCENGINE_NO_PROXY", "VOLCENGINE_PROXY_INCLUDE_DOMAINS", "NO_PROXY", "no_proxy"} {
		t.Setenv(name, "")
	}
}

func TestNewProxyHTTPClientValidation(t *testing.T) {
	testCases := map[string]struct {
		proxyURL           string
		proxyAuthorization string
		noProxy            string
		includeDomains     []string
	}{
		"malformed URL":                  {proxyURL: "http://%zz"},
		"missing scheme":                 {proxyURL: "proxy.example:8080"},
		"unsupported scheme":             {proxyURL: "ftp://proxy.example:8080"},
		"missing host":                   {proxyURL: "http:///proxy"},
		"invalid authorization":          {proxyURL: "http://proxy.example:8080", proxyAuthorization: "Basic secret\r\nX-Test: value"},
		"NUL in authorization":           {proxyURL: "http://proxy.example:8080", proxyAuthorization: "Basic secret\x00"},
		"DEL in authorization":           {proxyURL: "http://proxy.example:8080", proxyAuthorization: "Basic secret\x7f"},
		"authorization with SOCKS":       {proxyURL: "socks5://proxy.example:1080", proxyAuthorization: testProxyAuthorization},
		"userinfo and authorization set": {proxyURL: "http://user:password@proxy.example:8080", proxyAuthorization: testProxyAuthorization},
		"selection modes combined":       {proxyURL: "http://proxy.example:8080", noProxy: ".direct.example", includeDomains: []string{"proxied.example"}},
		"empty include domain":           {proxyURL: "http://proxy.example:8080", includeDomains: []string{""}},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			_, _, err := newProxyHTTPClient(testCase.proxyURL, testCase.proxyAuthorization, testCase.noProxy, testCase.includeDomains)
			if err == nil {
				t.Fatal("expected proxy configuration error")
			}
			if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), testProxyAuthorization) {
				t.Fatalf("proxy error leaked authorization: %v", err)
			}
		})
	}
}

func TestNewProxyHTTPClientAllowsSOCKSWithoutAuthorization(t *testing.T) {
	client, normalizedProxyURL, err := newProxyHTTPClient("socks5h://proxy.example:1080", "", "", nil)
	if err != nil {
		t.Fatalf("creating SOCKS proxy client: %v", err)
	}
	if normalizedProxyURL != "socks5h://proxy.example:1080" {
		t.Fatalf("normalized proxy URL = %q", normalizedProxyURL)
	}

	request, err := http.NewRequest(http.MethodGet, "https://cloudcontrol.cn-beijing.volcengineapi.com", nil)
	if err != nil {
		t.Fatal(err)
	}
	proxyURL, err := baseTransport(t, client).Proxy(request)
	if err != nil {
		t.Fatalf("resolving SOCKS proxy: %v", err)
	}
	if proxyURL == nil || proxyURL.String() != normalizedProxyURL {
		t.Fatalf("proxy URL = %v, want %q", proxyURL, normalizedProxyURL)
	}
}

func TestNewProxyHTTPClientNoProxySelection(t *testing.T) {
	client, proxyURL, err := newProxyHTTPClient(
		"http://proxy.example:8080",
		"",
		"volcengineapi.com,.internal.example,10.0.0.0/8",
		nil,
	)
	if err != nil {
		t.Fatalf("creating proxy client: %v", err)
	}

	testCases := map[string]struct {
		requestURL string
		wantProxy  bool
	}{
		"exact domain bypasses":          {requestURL: "https://volcengineapi.com", wantProxy: false},
		"domain suffix bypasses":         {requestURL: "https://cloudcontrol.cn-beijing.volcengineapi.com", wantProxy: false},
		"leading dot bypasses subdomain": {requestURL: "https://api.internal.example", wantProxy: false},
		"leading dot keeps root proxied": {requestURL: "https://internal.example", wantProxy: true},
		"CIDR bypasses":                  {requestURL: "http://10.10.20.30", wantProxy: false},
		"unmatched domain uses proxy":    {requestURL: "https://example.com", wantProxy: true},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			assertProxySelection(t, client, testCase.requestURL, proxyURL, testCase.wantProxy)
		})
	}
}

func TestNewProxyHTTPClientIncludeDomainSelection(t *testing.T) {
	client, proxyURL, err := newProxyHTTPClient(
		"http://proxy.example:8080",
		"",
		"",
		[]string{"cloudcontrol.cn-beijing.volcengineapi.com", ".included.example"},
	)
	if err != nil {
		t.Fatalf("creating proxy client: %v", err)
	}

	testCases := map[string]struct {
		requestURL string
		wantProxy  bool
	}{
		"included domain uses proxy":       {requestURL: "https://cloudcontrol.cn-beijing.volcengineapi.com", wantProxy: true},
		"included suffix uses proxy":       {requestURL: "https://api.included.example", wantProxy: true},
		"leading dot keeps root direct":    {requestURL: "https://included.example", wantProxy: false},
		"unlisted Cloud Control is direct": {requestURL: "https://cloudcontrol.cn-shanghai.volcengineapi.com", wantProxy: false},
		"unlisted domain is direct":        {requestURL: "https://example.com", wantProxy: false},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			assertProxySelection(t, client, testCase.requestURL, proxyURL, testCase.wantProxy)
		})
	}
}

func assertProxySelection(t *testing.T, client *http.Client, requestURL, wantProxyURL string, wantProxy bool) {
	t.Helper()
	request, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	proxyURL, err := baseTransport(t, client).Proxy(request)
	if err != nil {
		t.Fatalf("resolving proxy: %v", err)
	}
	if !wantProxy {
		if proxyURL != nil {
			t.Fatalf("proxy URL = %v, want direct connection", proxyURL)
		}
		return
	}
	if proxyURL == nil || proxyURL.String() != wantProxyURL {
		t.Fatalf("proxy URL = %v, want %q", proxyURL, wantProxyURL)
	}
}

func TestProviderDataRejectsAuthorizationWithoutProxyURL(t *testing.T) {
	config := newTestConfig("", "", testProxyAuthorization, false)

	providerData, diagnostics := newProviderData(context.Background(), config)
	if providerData != nil {
		t.Fatal("provider data must be nil for invalid proxy configuration")
	}
	if !diagnostics.HasError() {
		t.Fatal("expected proxy configuration diagnostic")
	}
	if strings.Contains(fmt.Sprint(diagnostics), testProxyAuthorization) {
		t.Fatal("proxy diagnostic leaked authorization")
	}
}

func TestProviderDataRejectsIncludeDomainsWithoutProxyURL(t *testing.T) {
	config := newTestConfig("", "", "", false)
	config.ProxyIncludeDomains = proxyStringSet("cloudcontrol.cn-beijing.volcengineapi.com")

	providerData, diagnostics := newProviderData(context.Background(), config)
	if providerData != nil {
		t.Fatal("provider data must be nil for invalid proxy configuration")
	}
	if !diagnostics.HasError() {
		t.Fatal("expected proxy configuration diagnostic")
	}
}

func TestProviderDataRejectsConflictingProxySelection(t *testing.T) {
	config := newTestConfig("", "http://proxy.example:8080", "", false)
	config.NoProxy = types.StringValue(".direct.example")
	config.ProxyIncludeDomains = proxyStringSet("cloudcontrol.cn-beijing.volcengineapi.com")

	providerData, diagnostics := newProviderData(context.Background(), config)
	if providerData != nil {
		t.Fatal("provider data must be nil for invalid proxy configuration")
	}
	if !diagnostics.HasError() {
		t.Fatal("expected proxy configuration diagnostic")
	}
}

func TestProviderDataPreservesProxyIncludeDomains(t *testing.T) {
	config := newTestConfig("", "http://proxy.example:8080", testProxyAuthorization, false)
	config.ProxyIncludeDomains = proxyStringSet("cloudcontrol.cn-beijing.volcengineapi.com")

	providerData, diagnostics := newProviderData(context.Background(), config)
	if diagnostics.HasError() {
		t.Fatalf("creating provider: %v", diagnostics)
	}
	client := providerData.ccAPIClient.Config.HTTPClient
	assertProxySelection(t, client, "https://cloudcontrol.cn-beijing.volcengineapi.com", "http://proxy.example:8080", true)
	assertProxySelection(t, client, "https://sts.volcengineapi.com", "http://proxy.example:8080", false)
}

func TestProviderProxyClientsAreIsolated(t *testing.T) {
	defaultClientTransport := http.DefaultClient.Transport
	defaultTransport := http.DefaultTransport

	first, firstDiagnostics := newProviderData(context.Background(), newTestConfig("", "http://proxy-one.example:8080", "Basic first", false))
	if firstDiagnostics.HasError() {
		t.Fatalf("creating first provider: %v", firstDiagnostics)
	}
	second, secondDiagnostics := newProviderData(context.Background(), newTestConfig("", "http://proxy-two.example:8080", "Basic second", false))
	if secondDiagnostics.HasError() {
		t.Fatalf("creating second provider: %v", secondDiagnostics)
	}
	withoutProxy, noProxyDiagnostics := newProviderData(context.Background(), newTestConfig("", "", "", false))
	if noProxyDiagnostics.HasError() {
		t.Fatalf("creating provider without proxy: %v", noProxyDiagnostics)
	}

	if http.DefaultClient.Transport != defaultClientTransport {
		t.Fatal("provider configuration mutated http.DefaultClient.Transport")
	}
	if http.DefaultTransport != defaultTransport {
		t.Fatal("provider configuration mutated http.DefaultTransport")
	}
	if withoutProxy.ccAPIClient.Config.HTTPClient == http.DefaultClient {
		t.Fatal("provider without proxy must still use a private HTTP client")
	}

	request, err := http.NewRequest(http.MethodGet, "https://cloudcontrol.cn-beijing.volcengineapi.com", nil)
	if err != nil {
		t.Fatal(err)
	}
	assertProxyTransport(t, first.ccAPIClient.Config.HTTPClient, request, "http://proxy-one.example:8080", "Basic first")
	assertProxyTransport(t, second.ccAPIClient.Config.HTTPClient, request, "http://proxy-two.example:8080", "Basic second")
}

func TestCloudControlHTTPSProxyAuthorization(t *testing.T) {
	testCloudControlHTTPSProxy(t, testProxyAuthorization)
}

func TestCloudControlHTTPSProxyWithoutAuthorization(t *testing.T) {
	testCloudControlHTTPSProxy(t, "")
}

func testCloudControlHTTPSProxy(t *testing.T, proxyAuthorization string) {
	t.Helper()
	originAuthorization := make(chan string, 1)
	origin := httptest.NewTLSServer(cloudControlHandler(originAuthorization))
	defer origin.Close()

	proxy, observations := newAuthenticatedConnectProxy(t, proxyAuthorization)
	defer proxy.Close()

	providerData, diagnostics := newProviderData(
		context.Background(),
		newTestConfig(origin.URL, proxy.URL, proxyAuthorization, false),
	)
	if diagnostics.HasError() {
		t.Fatalf("configuring provider: %v", diagnostics)
	}
	defer providerData.ccAPIClient.Config.HTTPClient.CloseIdleConnections()

	transport := baseTransport(t, providerData.ccAPIClient.Config.HTTPClient)
	targetTransport := origin.Client().Transport.(*http.Transport)
	transport.TLSClientConfig = targetTransport.TLSClientConfig.Clone()

	callCloudControl(t, providerData.ccAPIClient)

	observation := receiveProxyObservation(t, observations)
	if observation.method != http.MethodConnect {
		t.Fatalf("proxy method = %q, want CONNECT", observation.method)
	}
	if observation.authorization != proxyAuthorization {
		t.Fatalf("proxy authorization = %q, want %q", observation.authorization, proxyAuthorization)
	}
	if got := receiveString(t, originAuthorization, "origin request"); got != "" {
		t.Fatalf("origin received Proxy-Authorization header %q", got)
	}
}

func TestCloudControlHTTPProxyAuthorization(t *testing.T) {
	originAuthorization := make(chan string, 1)
	origin := httptest.NewServer(cloudControlHandler(originAuthorization))
	defer origin.Close()

	proxy, observations := newAuthenticatedForwardProxy(t, testProxyAuthorization)
	defer proxy.Close()

	providerData, diagnostics := newProviderData(
		context.Background(),
		newTestConfig(origin.URL, proxy.URL, testProxyAuthorization, true),
	)
	if diagnostics.HasError() {
		t.Fatalf("configuring provider: %v", diagnostics)
	}
	defer providerData.ccAPIClient.Config.HTTPClient.CloseIdleConnections()

	callCloudControl(t, providerData.ccAPIClient)

	observation := receiveProxyObservation(t, observations)
	if observation.method != http.MethodPost {
		t.Fatalf("proxy method = %q, want POST", observation.method)
	}
	if observation.authorization != testProxyAuthorization {
		t.Fatalf("proxy authorization = %q, want %q", observation.authorization, testProxyAuthorization)
	}
	if got := receiveString(t, originAuthorization, "origin request"); got != "" {
		t.Fatalf("origin received Proxy-Authorization header %q", got)
	}
}

func TestCloudControlNoProxyBypassesProxyAndAuthorization(t *testing.T) {
	originAuthorization := make(chan string, 1)
	origin := httptest.NewServer(cloudControlHandler(originAuthorization))
	defer origin.Close()

	proxy, observations := newAuthenticatedForwardProxy(t, testProxyAuthorization)
	defer proxy.Close()

	config := newTestConfig(origin.URL, proxy.URL, testProxyAuthorization, true)
	config.NoProxy = types.StringValue("*")
	providerData, diagnostics := newProviderData(context.Background(), config)
	if diagnostics.HasError() {
		t.Fatalf("configuring provider: %v", diagnostics)
	}
	defer providerData.ccAPIClient.Config.HTTPClient.CloseIdleConnections()

	callCloudControl(t, providerData.ccAPIClient)

	if got := receiveString(t, originAuthorization, "origin request"); got != "" {
		t.Fatalf("direct origin received Proxy-Authorization header %q", got)
	}
	select {
	case observation := <-observations:
		t.Fatalf("no_proxy request unexpectedly used proxy: %#v", observation)
	default:
	}
}

type proxyObservation struct {
	method        string
	authorization string
}

func newTestConfig(endpoint, proxyURL, proxyAuthorization string, disableSSL bool) *configModel {
	proxyURLValue := types.StringNull()
	if proxyURL != "" {
		proxyURLValue = types.StringValue(proxyURL)
	}
	proxyAuthorizationValue := types.StringNull()
	if proxyAuthorization != "" {
		proxyAuthorizationValue = types.StringValue(proxyAuthorization)
	}
	var endpoints *endpointData
	if endpoint != "" {
		endpoints = &endpointData{
			CloudControlAPI: types.StringValue(endpoint),
			STS:             types.StringNull(),
		}
	}

	return &configModel{
		AccessKey:           types.StringValue("test-access-key"),
		SecretKey:           types.StringValue("test-secret-key"),
		SessionToken:        types.StringNull(),
		Region:              types.StringValue("cn-beijing"),
		DisableSSL:          types.BoolValue(disableSSL),
		CustomerHeaders:     types.StringNull(),
		ProxyURL:            proxyURLValue,
		ProxyAuthorization:  proxyAuthorizationValue,
		NoProxy:             types.StringNull(),
		ProxyIncludeDomains: types.SetNull(types.StringType),
		Endpoints:           endpoints,
		Profile:             types.StringNull(),
		FilePath:            types.StringNull(),
		terraformVersion:    "1.0.0",
	}
}

func proxyStringSet(values ...string) types.Set {
	elements := make([]attr.Value, 0, len(values))
	for _, value := range values {
		elements = append(elements, types.StringValue(value))
	}
	return types.SetValueMust(types.StringType, elements)
}

func assertProxyTransport(t *testing.T, client *http.Client, request *http.Request, wantURL, wantAuthorization string) {
	t.Helper()
	transport := baseTransport(t, client)
	proxyURL, err := transport.Proxy(request)
	if err != nil {
		t.Fatalf("resolving proxy: %v", err)
	}
	if proxyURL == nil || proxyURL.String() != wantURL {
		t.Fatalf("proxy URL = %v, want %q", proxyURL, wantURL)
	}
	if got := transport.ProxyConnectHeader.Get("Proxy-Authorization"); got != wantAuthorization {
		t.Fatalf("CONNECT proxy authorization = %q, want %q", got, wantAuthorization)
	}
}

func baseTransport(t *testing.T, client *http.Client) *http.Transport {
	t.Helper()
	transport := client.Transport
	if authorizationTransport, ok := transport.(*proxyAuthorizationRoundTripper); ok {
		transport = authorizationTransport.transport
	}
	base, ok := transport.(*http.Transport)
	if !ok {
		t.Fatalf("HTTP transport has type %T, want *http.Transport", transport)
	}
	return base
}

func cloudControlHandler(proxyAuthorization chan<- string) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		proxyAuthorization <- request.Header.Get("Proxy-Authorization")
		response.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(response, `{"ResponseMetadata":{"RequestId":"test-request"},"Result":{"TypeName":"Volcengine::Test::Resource","ResourceDescription":{"Identifier":"test-id","Properties":"{}"}}}`)
	}
}

func callCloudControl(t *testing.T, client *cloudcontrol.CloudControl) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	output, err := client.GetResourceWithContext(ctx, &cloudcontrol.GetResourceInput{
		TypeName:   volcengine.String("Volcengine::Test::Resource"),
		Identifier: volcengine.String("test-id"),
	})
	if err != nil {
		t.Fatalf("Cloud Control request failed: %v", err)
	}
	if output.TypeName == nil || *output.TypeName != "Volcengine::Test::Resource" {
		t.Fatalf("unexpected Cloud Control response: %#v", output)
	}
}

func newAuthenticatedConnectProxy(t *testing.T, authorization string) (*httptest.Server, <-chan proxyObservation) {
	t.Helper()
	observations := make(chan proxyObservation, 1)
	proxy := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		observations <- proxyObservation{method: request.Method, authorization: request.Header.Get("Proxy-Authorization")}
		if request.Method != http.MethodConnect || request.Header.Get("Proxy-Authorization") != authorization {
			response.Header().Set("Proxy-Authenticate", "Basic")
			response.WriteHeader(http.StatusProxyAuthRequired)
			return
		}

		upstream, err := net.DialTimeout("tcp", request.Host, 2*time.Second)
		if err != nil {
			http.Error(response, err.Error(), http.StatusBadGateway)
			return
		}
		hijacker, ok := response.(http.Hijacker)
		if !ok {
			upstream.Close()
			http.Error(response, "hijacking unsupported", http.StatusInternalServerError)
			return
		}
		clientConnection, readWriter, err := hijacker.Hijack()
		if err != nil {
			upstream.Close()
			return
		}
		if _, err = readWriter.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
			clientConnection.Close()
			upstream.Close()
			return
		}
		if err = readWriter.Flush(); err != nil {
			clientConnection.Close()
			upstream.Close()
			return
		}

		go copyAndClose(upstream, clientConnection)
		go copyAndClose(clientConnection, upstream)
	}))
	return proxy, observations
}

func newAuthenticatedForwardProxy(t *testing.T, authorization string) (*httptest.Server, <-chan proxyObservation) {
	t.Helper()
	observations := make(chan proxyObservation, 1)
	directTransport := http.DefaultTransport.(*http.Transport).Clone()
	directTransport.Proxy = nil
	t.Cleanup(directTransport.CloseIdleConnections)

	proxy := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		observations <- proxyObservation{method: request.Method, authorization: request.Header.Get("Proxy-Authorization")}
		if request.Header.Get("Proxy-Authorization") != authorization {
			response.Header().Set("Proxy-Authenticate", "Basic")
			response.WriteHeader(http.StatusProxyAuthRequired)
			return
		}

		forwardRequest := request.Clone(request.Context())
		forwardRequest.RequestURI = ""
		forwardRequest.Header = request.Header.Clone()
		forwardRequest.Header.Del("Proxy-Authorization")
		upstreamResponse, err := directTransport.RoundTrip(forwardRequest)
		if err != nil {
			http.Error(response, err.Error(), http.StatusBadGateway)
			return
		}
		defer upstreamResponse.Body.Close()
		for key, values := range upstreamResponse.Header {
			for _, value := range values {
				response.Header().Add(key, value)
			}
		}
		response.WriteHeader(upstreamResponse.StatusCode)
		_, _ = io.Copy(response, upstreamResponse.Body)
	}))
	return proxy, observations
}

func copyAndClose(destination, source net.Conn) {
	_, _ = io.Copy(destination, source)
	_ = destination.Close()
	_ = source.Close()
}

func receiveProxyObservation(t *testing.T, observations <-chan proxyObservation) proxyObservation {
	t.Helper()
	select {
	case observation := <-observations:
		return observation
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for proxy request")
		return proxyObservation{}
	}
}

func receiveString(t *testing.T, values <-chan string, description string) string {
	t.Helper()
	select {
	case value := <-values:
		return value
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for %s", description)
		return ""
	}
}
