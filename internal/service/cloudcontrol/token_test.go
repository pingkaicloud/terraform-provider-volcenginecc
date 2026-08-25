// Copyright (c) 2025 Beijing Volcano Engine Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package cloudcontrol

import "testing"

func TestCreateOperationToken(t *testing.T) {
	t.Parallel()

	typeName := "Volcengine::VKE::Kubeconfig"
	if got, want := CreateOperationToken(typeName, "uid-a"), OperationToken("create", typeName, "uid-a", ""); got != want {
		t.Fatalf("UID token = %q, want %q", got, want)
	}
	if got, want := CreateOperationToken(typeName, "uid-a"), CreateOperationToken(typeName, "uid-a"); got != want {
		t.Fatalf("same UID did not produce a stable token: %q != %q", got, want)
	}
	if got, want := CreateOperationToken(typeName, "uid-a"), CreateOperationToken(typeName, "uid-b"); got == want {
		t.Fatalf("different UIDs produced the same token: %q", got)
	}

	first := CreateOperationToken(typeName, "")
	second := CreateOperationToken(typeName, "")
	if len(first) != 32 || len(second) != 32 {
		t.Fatalf("direct Terraform tokens have lengths %d and %d, want 32", len(first), len(second))
	}
	if first == second {
		t.Fatalf("direct Terraform creates produced the same random token %q", first)
	}
}
