// Copyright (c) 2025 Beijing Volcano Engine Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package cloudcontrol

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/volcengine/volcengine-go-sdk/volcengine/request"

	"github.com/volcengine/terraform-provider-volcenginecc/internal/base"
	"github.com/volcengine/terraform-provider-volcenginecc/internal/cloudcontrol"
	"github.com/volcengine/terraform-provider-volcenginecc/internal/tfresource"
	"github.com/volcengine/terraform-provider-volcenginecc/internal/util"
)

// API is the minimal Cloud Control surface used by the operation executor.
// *cloudcontrol.CloudControl satisfies it; tests inject a fake.
type API interface {
	CreateResourceWithContext(ctx context.Context, input *cloudcontrol.CreateResourceInput, opts ...request.Option) (*cloudcontrol.CreateResourceOutput, error)
	UpdateResourceWithContext(ctx context.Context, input *cloudcontrol.UpdateResourceInput, opts ...request.Option) (*cloudcontrol.UpdateResourceOutput, error)
	DeleteResourceWithContext(ctx context.Context, input *cloudcontrol.DeleteResourceInput, opts ...request.Option) (*cloudcontrol.DeleteResourceOutput, error)
	GetTaskWithContext(ctx context.Context, input *cloudcontrol.GetTaskInput, opts ...request.Option) (*cloudcontrol.GetTaskOutput, error)
}

// OperationKind identifies which Cloud Control mutation an Operation performs.
type OperationKind string

const (
	OperationCreate OperationKind = "create"
	OperationUpdate OperationKind = "update"
	OperationDelete OperationKind = "delete"
)

// Operation describes one logical Cloud Control mutation.
//
// StableToken is the deterministic ClientToken derived from the resource
// identity. It is only meant to protect an attempt whose outcome is unknown
// (timeout, provider restart while the task is still running). Cloud Control
// replays the original task for a repeated token, including a task that has
// already FAILED, so once a FAILED verdict is observed the executor moves to a
// fresh AttemptToken instead of replaying the same failure forever.
type Operation struct {
	Kind          OperationKind
	TypeName      string
	Region        string
	Identifier    string          // update / delete
	TargetState   *map[string]any // create
	PatchDocument []any           // update
	StableToken   string
	// Timeout bounds the whole operation including in-operation retries.
	// Zero means DefaultOperationTimeout.
	Timeout time.Duration
	// Timing overrides poll and backoff intervals; nil means defaults.
	Timing *Timing
}

// Timing controls waiting behaviour. Tests shorten it.
type Timing struct {
	InitialPoll    time.Duration
	PollInterval   time.Duration
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	// MaxAttempts is the maximum number of Cloud Control submissions per
	// operation, counting the first stable-token submission.
	MaxAttempts int
	// ReplaySkew tolerates clock drift between the provider and Cloud Control
	// when deciding whether a FAILED event is a replay of an older task.
	ReplaySkew time.Duration
	// MaxTaskPollFailures is the number of consecutive GetTask transport
	// failures tolerated before the operation gives up.
	MaxTaskPollFailures int
}

// DefaultOperationTimeout bounds an operation without an explicit timeout.
const DefaultOperationTimeout = 120 * time.Minute

var defaultTiming = Timing{
	InitialPoll:         time.Second,
	PollInterval:        10 * time.Second,
	InitialBackoff:      5 * time.Second,
	MaxBackoff:          60 * time.Second,
	MaxAttempts:         6,
	ReplaySkew:          60 * time.Second,
	MaxTaskPollFailures: 5,
}

// Result is the terminal outcome of a successful operation.
type Result struct {
	Event    *cloudcontrol.ProgressEvent
	TaskID   string
	Attempts int
}

// OperationError carries the last Cloud Control verdict for a failed
// operation. It intentionally excludes request payloads.
type OperationError struct {
	Kind          OperationKind
	TypeName      string
	Identifier    string
	TaskID        string
	RequestID     string
	ErrorCode     string
	StatusMessage string
	EventTime     time.Time
	Attempts      int
	Class         FailureClass
}

func (e *OperationError) Error() string {
	eventTime := "-"
	if !e.EventTime.IsZero() {
		eventTime = e.EventTime.Format(time.RFC3339)
	}
	return fmt.Sprintf("Cloud Control %s failed: type=%q identifier=%q task_id=%q request_id=%q error_code=%q status_message=%q event_time=%s attempts=%d class=%s",
		e.Kind, e.TypeName, e.Identifier, e.TaskID, e.RequestID, e.ErrorCode, e.StatusMessage, eventTime, e.Attempts, e.Class)
}

// submission is the normalized view of a Create/Update/Delete response.
type submission struct {
	event     cloudcontrol.ProgressEvent
	requestID string
}

// Run executes op until it reaches a terminal verdict.
//
// Flow per attempt:
//
//	submit(token)
//	  SUCCESS               -> done
//	  IN_PROGRESS | PENDING -> poll GetTask until terminal
//	  FAILED NotFound       -> tfresource.NotFoundError
//	  FAILED replay/retry   -> new AttemptToken, backoff, next attempt
//	  FAILED terminal       -> OperationError
//
// The first attempt always uses op.StableToken so an in-flight task from a
// previous process is resumed rather than duplicated.
func Run(ctx context.Context, api API, op Operation) (*Result, error) {
	if err := op.validate(); err != nil {
		return nil, err
	}
	timing := op.timing()
	timeout := op.Timeout
	if timeout <= 0 {
		timeout = DefaultOperationTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	token := op.StableToken
	backoff := timing.InitialBackoff
	var lastErr error

	for attempt := 1; attempt <= timing.MaxAttempts; attempt++ {
		attemptStart := time.Now()
		sub, err := op.submit(ctx, api, token)
		if err != nil {
			if tfresource.NotFound(err) {
				return nil, err
			}
			if !isRetryableTransportError(err) {
				return nil, err
			}
			// The request may or may not have been accepted; keep the same token so
			// a resubmission is idempotent against the server.
			lastErr = err
			tflog.Warn(ctx, "Cloud Control submission failed, retrying with the same token", map[string]interface{}{
				"operation": string(op.Kind),
				"type":      op.TypeName,
				"id":        op.Identifier,
				"attempt":   attempt,
				"error":     err.Error(),
			})
			if err := sleep(ctx, backoff); err != nil {
				return nil, wrapLast(err, lastErr)
			}
			backoff = nextBackoff(backoff, timing.MaxBackoff)
			continue
		}

		event := sub.event
		status := util.ToString(event.OperationStatus)
		taskID := util.ToString(event.TaskID)
		tflog.Info(ctx, "Cloud Control operation submitted", map[string]interface{}{
			"operation":  string(op.Kind),
			"type":       op.TypeName,
			"id":         op.Identifier,
			"attempt":    attempt,
			"status":     status,
			"task_id":    taskID,
			"request_id": sub.requestID,
		})

		switch status {
		case base.SUCCESS:
			return &Result{Event: &event, TaskID: taskID, Attempts: attempt}, nil
		case base.IN_PROGRESS, base.PENDING:
			if taskID == "" {
				return nil, fmt.Errorf("Cloud Control %s returned %s without a task ID (request_id=%q)", op.Kind, status, sub.requestID)
			}
			polled, err := awaitTask(ctx, api, taskID, timing)
			if err != nil {
				return nil, err
			}
			if util.ToString(polled.OperationStatus) == base.SUCCESS {
				return &Result{Event: polled, TaskID: taskID, Attempts: attempt}, nil
			}
			event = *polled
		case base.FAILED:
			// handled below
		default:
			return nil, fmt.Errorf("Cloud Control %s returned an unexpected status: status=%q request_id=%q task_id=%q", op.Kind, status, sub.requestID, taskID)
		}

		// FAILED verdict, either synchronous or after polling.
		opErr := &OperationError{
			Kind:          op.Kind,
			TypeName:      op.TypeName,
			Identifier:    op.Identifier,
			TaskID:        util.ToString(event.TaskID),
			RequestID:     sub.requestID,
			ErrorCode:     util.ToString(event.ErrorCode),
			StatusMessage: util.ToString(event.StatusMessage),
			EventTime:     event.EventTime,
			Attempts:      attempt,
		}
		if isCloudControlNotFoundProgressEvent(&event) {
			opErr.Class = FailureNotFound
			return nil, &tfresource.NotFoundError{LastError: opErr}
		}
		if op.Kind == OperationCreate && util.ToString(event.Identifier) != "" {
			// A failed create that already produced an identifier may have left a
			// partial resource behind. Re-submitting would risk a duplicate.
			opErr.Class = FailureTerminal
			opErr.Identifier = util.ToString(event.Identifier)
			return nil, opErr
		}
		replay := isReplay(event.EventTime, attemptStart, timing.ReplaySkew)
		opErr.Class = classifyFailure(&event)
		if replay && opErr.Class != FailureRetryable {
			opErr.Class = FailureReplayed
		}
		lastErr = opErr
		if opErr.Class == FailureTerminal {
			return nil, opErr
		}
		if attempt == timing.MaxAttempts {
			break
		}
		tflog.Warn(ctx, "Cloud Control operation failed, retrying with a new client token", map[string]interface{}{
			"operation":  string(op.Kind),
			"type":       op.TypeName,
			"id":         op.Identifier,
			"attempt":    attempt,
			"task_id":    opErr.TaskID,
			"error_code": opErr.ErrorCode,
			"replay":     replay,
			"class":      string(opErr.Class),
		})
		token = AttemptToken(op.StableToken)
		if err := sleep(ctx, backoff); err != nil {
			return nil, wrapLast(err, lastErr)
		}
		backoff = nextBackoff(backoff, timing.MaxBackoff)
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("Cloud Control %s exhausted %d attempts", op.Kind, timing.MaxAttempts)
	}
	return nil, lastErr
}

func (op Operation) validate() error {
	if op.TypeName == "" {
		return errors.New("Cloud Control operation requires a type name")
	}
	if op.StableToken == "" {
		return errors.New("Cloud Control operation requires a stable client token")
	}
	switch op.Kind {
	case OperationCreate:
		if op.TargetState == nil {
			return errors.New("Cloud Control create requires a target state")
		}
	case OperationUpdate:
		if op.Identifier == "" {
			return errors.New("Cloud Control update requires an identifier")
		}
	case OperationDelete:
		if op.Identifier == "" {
			return errors.New("Cloud Control delete requires an identifier")
		}
	default:
		return fmt.Errorf("unsupported Cloud Control operation %q", op.Kind)
	}
	return nil
}

func (op Operation) timing() Timing {
	if op.Timing == nil {
		return defaultTiming
	}
	t := *op.Timing
	if t.InitialPoll <= 0 {
		t.InitialPoll = defaultTiming.InitialPoll
	}
	if t.PollInterval <= 0 {
		t.PollInterval = defaultTiming.PollInterval
	}
	if t.InitialBackoff <= 0 {
		t.InitialBackoff = defaultTiming.InitialBackoff
	}
	if t.MaxBackoff <= 0 {
		t.MaxBackoff = defaultTiming.MaxBackoff
	}
	if t.MaxAttempts <= 0 {
		t.MaxAttempts = defaultTiming.MaxAttempts
	}
	if t.ReplaySkew < 0 {
		t.ReplaySkew = defaultTiming.ReplaySkew
	}
	if t.MaxTaskPollFailures <= 0 {
		t.MaxTaskPollFailures = defaultTiming.MaxTaskPollFailures
	}
	return t
}

func (op Operation) submit(ctx context.Context, api API, token string) (*submission, error) {
	switch op.Kind {
	case OperationCreate:
		out, err := api.CreateResourceWithContext(ctx, &cloudcontrol.CreateResourceInput{
			TypeName:    util.StringPtr(op.TypeName),
			RegionID:    op.Region,
			ClientToken: util.StringPtr(token),
			TargetState: op.TargetState,
		})
		if err != nil {
			return nil, err
		}
		if out == nil || out.OperationStatus == nil {
			return nil, errors.New("Cloud Control CreateResource returned an empty operation status")
		}
		return &submission{event: out.ProgressEvent, requestID: out.GetRequestId()}, nil
	case OperationUpdate:
		out, err := api.UpdateResourceWithContext(ctx, &cloudcontrol.UpdateResourceInput{
			TypeName:      util.StringPtr(op.TypeName),
			RegionID:      util.StringPtr(op.Region),
			Identifier:    util.StringPtr(op.Identifier),
			ClientToken:   util.StringPtr(token),
			PatchDocument: op.PatchDocument,
		})
		if err != nil {
			return nil, wrapCloudControlNotFound(err)
		}
		if out == nil || out.OperationStatus == nil {
			return nil, errors.New("Cloud Control UpdateResource returned an empty operation status")
		}
		return &submission{event: out.ProgressEvent, requestID: out.GetRequestId()}, nil
	case OperationDelete:
		out, err := api.DeleteResourceWithContext(ctx, &cloudcontrol.DeleteResourceInput{
			TypeName:    util.StringPtr(op.TypeName),
			RegionID:    util.StringPtr(op.Region),
			Identifier:  util.StringPtr(op.Identifier),
			ClientToken: util.StringPtr(token),
		})
		if err != nil {
			return nil, wrapCloudControlNotFound(err)
		}
		if out == nil || out.OperationStatus == nil {
			return nil, errors.New("Cloud Control DeleteResource returned an empty operation status")
		}
		return &submission{event: out.ProgressEvent, requestID: out.GetRequestId()}, nil
	}
	return nil, fmt.Errorf("unsupported Cloud Control operation %q", op.Kind)
}

// awaitTask polls GetTask until the task leaves IN_PROGRESS/PENDING. It
// returns the terminal event for both SUCCESS and FAILED so the caller can
// classify FAILED consistently with synchronous failures.
func awaitTask(ctx context.Context, api API, taskID string, timing Timing) (*cloudcontrol.ProgressEvent, error) {
	if err := sleep(ctx, timing.InitialPoll); err != nil {
		return nil, err
	}
	failures := 0
	for {
		out, err := api.GetTaskWithContext(ctx, &cloudcontrol.GetTaskInput{TaskID: util.StringPtr(taskID)})
		if err != nil {
			if !isRetryableTransportError(err) {
				return nil, fmt.Errorf("invoke get task %q: %w", taskID, err)
			}
			failures++
			if failures >= timing.MaxTaskPollFailures {
				return nil, fmt.Errorf("invoke get task %q failed %d times: %w", taskID, failures, err)
			}
		} else {
			failures = 0
			if out == nil || out.OperationStatus == nil {
				return nil, fmt.Errorf("get task %q returned an empty operation status", taskID)
			}
			switch util.ToString(out.OperationStatus) {
			case base.SUCCESS, base.FAILED:
				event := out.ProgressEvent
				return &event, nil
			case base.IN_PROGRESS, base.PENDING:
			default:
				return nil, fmt.Errorf("get task returned an unexpected status: status=%q task_id=%q type=%q identifier=%q",
					util.ToString(out.OperationStatus), util.ToString(out.TaskID), util.ToString(out.TypeName), util.ToString(out.Identifier))
			}
		}
		if err := sleep(ctx, timing.PollInterval); err != nil {
			return nil, err
		}
	}
}

func sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func wrapLast(ctxErr, last error) error {
	if last == nil {
		return ctxErr
	}
	return fmt.Errorf("%w (last Cloud Control verdict: %v)", ctxErr, last)
}
