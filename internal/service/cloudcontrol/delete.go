// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package cloudcontrol

import (
	"context"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// DeleteResource deletes the Cloud Control resource identified by typeName/id
// and waits for the verdict. A resource that is already gone is reported as
// tfresource.NotFoundError. Transient failures (locks, conflicts) are retried
// with fresh client tokens; see Run.
func DeleteResource(ctx context.Context, api API, region, roleARN, typeName, id string) error {
	return DeleteResourceWithTimeout(ctx, api, region, typeName, id, 0)
}

// DeleteResourceWithTimeout is DeleteResource with an explicit overall budget.
func DeleteResourceWithTimeout(ctx context.Context, api API, region, typeName, id string, timeout time.Duration) error {
	tflog.Debug(ctx, "DeleteResource", map[string]interface{}{
		"cfTypeName": typeName,
		"id":         id,
	})

	_, err := Run(ctx, api, Operation{
		Kind:        OperationDelete,
		TypeName:    typeName,
		Region:      region,
		Identifier:  id,
		StableToken: DeleteOperationToken(typeName, id),
		Timeout:     timeout,
	})
	return err
}
