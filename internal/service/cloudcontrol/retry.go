// Copyright (c) 2025 Beijing Volcano Engine Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package cloudcontrol

import (
	"context"
	"errors"
	"math/rand"
	"net"
	"strings"
	"time"

	"github.com/volcengine/volcengine-go-sdk/volcengine/volcengineerr"

	"github.com/volcengine/terraform-provider-volcenginecc/internal/cloudcontrol"
)

// FailureClass describes how the executor should treat a FAILED verdict.
type FailureClass string

const (
	// FailureNotFound means the target resource does not exist.
	FailureNotFound FailureClass = "not_found"
	// FailureRetryable means the handler reported a transient condition such
	// as a lock or conflict; a fresh attempt is expected to make progress.
	FailureRetryable FailureClass = "retryable"
	// FailureReplayed means Cloud Control returned an older task for a reused
	// client token. The verdict is stale and must be re-established with a new
	// token regardless of its error code.
	FailureReplayed FailureClass = "replayed"
	// FailureTerminal means the verdict is fresh and not expected to change
	// without an input change (permissions, validation, quota).
	FailureTerminal FailureClass = "terminal"
)

// retryableMarkers are matched case-insensitively against the ErrorCode and
// the handler code embedded in StatusMessage. Cloud Control frequently returns
// a generic ErrorCode ("InvalidRequest") and puts the service code only in
// StatusMessage, e.g. "InvalidRequest: InvalidEndpointService.InvalidLock: ...".
var retryableMarkers = []string{
	"invalidlock",
	"resourceconflict",
	".conflict",
	"conflict.",
	"inprogress",
	"in_progress",
	"operationdenied.locked",
	"dependencyviolation",
	"throttling",
	"flowlimitexceeded",
	"toomanyrequests",
	"requestlimitexceeded",
	"internalerror",
	"internalservererror",
	"serviceunavailable",
	"requesttimeout",
	"tryagain",
	"tryagainlater",
	"resourcebusy",
	"resourceinuse",
	"eventualconsistency",
}

// classifyFailure decides how a FAILED ProgressEvent should be treated.
func classifyFailure(event *cloudcontrol.ProgressEvent) FailureClass {
	if event == nil {
		return FailureTerminal
	}
	if isCloudControlNotFoundProgressEvent(event) {
		return FailureNotFound
	}
	if event.ErrorCode != nil && isRetryableCode(*event.ErrorCode) {
		return FailureRetryable
	}
	if event.StatusMessage != nil && isRetryableCode(*event.StatusMessage) {
		return FailureRetryable
	}
	return FailureTerminal
}

func isRetryableCode(text string) bool {
	lower := strings.ToLower(text)
	if lower == "" {
		return false
	}
	for _, marker := range retryableMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

// isReplay reports whether a FAILED event predates the attempt that received
// it, i.e. Cloud Control replayed an earlier task for a reused client token.
// A zero EventTime cannot be judged and is treated as fresh.
func isReplay(eventTime, attemptStart time.Time, skew time.Duration) bool {
	if eventTime.IsZero() {
		return false
	}
	return eventTime.Before(attemptStart.Add(-skew))
}

// isRetryableTransportError classifies SDK-level errors (before Cloud Control
// produced a ProgressEvent). Only conditions where the request may be retried
// with the same token qualify.
func isRetryableTransportError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var rf volcengineerr.RequestFailure
	if errors.As(err, &rf) {
		if rf.StatusCode() >= 500 {
			return true
		}
		if rf.StatusCode() == 429 {
			return true
		}
		return isRetryableCode(rf.Code())
	}
	var ve volcengineerr.Error
	if errors.As(err, &ve) {
		if isRetryableCode(ve.Code()) {
			return true
		}
		// SDK wraps dial/read failures as RequestError with the original error.
		if ve.Code() == "RequestError" || ve.Code() == "SerializationError" {
			return isNetworkError(ve.OrigErr()) || isNetworkError(err)
		}
		return false
	}
	return isNetworkError(err)
}

func isNetworkError(err error) bool {
	if err == nil {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "eof") ||
		strings.Contains(msg, "timeout")
}

// nextBackoff doubles the delay up to max and adds up to 20% jitter.
func nextBackoff(current, max time.Duration) time.Duration {
	next := current * 2
	if next > max {
		next = max
	}
	if next <= 0 {
		return max
	}
	jitter := time.Duration(rand.Int63n(int64(next) / 5)) //nolint:gosec // jitter only
	return next + jitter
}
