// Copyright (c) 2025 Beijing Volcano Engine Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package cloudcontrol

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/volcengine/volcengine-go-sdk/volcengine/request"
	"github.com/volcengine/volcengine-go-sdk/volcengine/response"
	"github.com/volcengine/volcengine-go-sdk/volcengine/volcengineerr"

	"github.com/volcengine/terraform-provider-volcenginecc/internal/base"
	"github.com/volcengine/terraform-provider-volcenginecc/internal/cloudcontrol"
	"github.com/volcengine/terraform-provider-volcenginecc/internal/tfresource"
)

// fakeCC simulates Cloud Control ClientToken semantics: a token seen before
// replays the task it originally produced, whatever its terminal status.
type fakeCC struct {
	mu sync.Mutex

	// verdicts are consumed in order by *new* tokens.
	verdicts []cloudcontrol.ProgressEvent
	// submitErrs are consumed in order before verdicts; nil means no error.
	submitErrs []error
	// tasks maps task ID to a sequence of GetTask events; the last is repeated.
	tasks map[string][]cloudcontrol.ProgressEvent

	byToken   map[string]cloudcontrol.ProgressEvent
	tokens    []string
	getTasks  []string
	submits   int
	getErrs   []error
	nextTask  int
	requestID int
}

func newFakeCC() *fakeCC {
	return &fakeCC{byToken: map[string]cloudcontrol.ProgressEvent{}, tasks: map[string][]cloudcontrol.ProgressEvent{}}
}

func (f *fakeCC) submit(token string) (cloudcontrol.ProgressEvent, *response.ResponseMetadata, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.submits++
	f.requestID++
	meta := &response.ResponseMetadata{RequestId: "req-" + itoa(f.requestID)}
	if len(f.submitErrs) > 0 {
		err := f.submitErrs[0]
		f.submitErrs = f.submitErrs[1:]
		if err != nil {
			return cloudcontrol.ProgressEvent{}, meta, err
		}
	}
	if ev, ok := f.byToken[token]; ok {
		return ev, meta, nil
	}
	if len(f.verdicts) == 0 {
		panic("fakeCC: no verdict left for new token")
	}
	ev := f.verdicts[0]
	f.verdicts = f.verdicts[1:]
	f.byToken[token] = ev
	f.tokens = append(f.tokens, token)
	return ev, meta, nil
}

func (f *fakeCC) CreateResourceWithContext(_ context.Context, in *cloudcontrol.CreateResourceInput, _ ...request.Option) (*cloudcontrol.CreateResourceOutput, error) {
	ev, meta, err := f.submit(*in.ClientToken)
	if err != nil {
		return nil, err
	}
	return &cloudcontrol.CreateResourceOutput{Metadata: meta, ProgressEvent: ev}, nil
}

func (f *fakeCC) UpdateResourceWithContext(_ context.Context, in *cloudcontrol.UpdateResourceInput, _ ...request.Option) (*cloudcontrol.UpdateResourceOutput, error) {
	ev, meta, err := f.submit(*in.ClientToken)
	if err != nil {
		return nil, err
	}
	return &cloudcontrol.UpdateResourceOutput{Metadata: meta, ProgressEvent: ev}, nil
}

func (f *fakeCC) DeleteResourceWithContext(_ context.Context, in *cloudcontrol.DeleteResourceInput, _ ...request.Option) (*cloudcontrol.DeleteResourceOutput, error) {
	ev, meta, err := f.submit(*in.ClientToken)
	if err != nil {
		return nil, err
	}
	return &cloudcontrol.DeleteResourceOutput{Metadata: meta, ProgressEvent: ev}, nil
}

func (f *fakeCC) GetTaskWithContext(_ context.Context, in *cloudcontrol.GetTaskInput, _ ...request.Option) (*cloudcontrol.GetTaskOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.getTasks = append(f.getTasks, *in.TaskID)
	if len(f.getErrs) > 0 {
		err := f.getErrs[0]
		f.getErrs = f.getErrs[1:]
		if err != nil {
			return nil, err
		}
	}
	seq := f.tasks[*in.TaskID]
	if len(seq) == 0 {
		panic("fakeCC: no GetTask sequence for " + *in.TaskID)
	}
	ev := seq[0]
	if len(seq) > 1 {
		f.tasks[*in.TaskID] = seq[1:]
	}
	return &cloudcontrol.GetTaskOutput{ProgressEvent: ev}, nil
}

func itoa(i int) string {
	const digits = "0123456789"
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{digits[i%10]}, b...)
		i /= 10
	}
	return string(b)
}

func str(s string) *string { return &s }

func event(status, taskID string, t time.Time) cloudcontrol.ProgressEvent {
	return cloudcontrol.ProgressEvent{OperationStatus: str(status), TaskID: str(taskID), EventTime: t}
}

func failed(taskID, code, message string, t time.Time) cloudcontrol.ProgressEvent {
	ev := event(base.FAILED, taskID, t)
	ev.ErrorCode = str(code)
	ev.StatusMessage = str(message)
	return ev
}

func fastTiming() *Timing {
	return &Timing{
		InitialPoll:         time.Millisecond,
		PollInterval:        time.Millisecond,
		InitialBackoff:      time.Millisecond,
		MaxBackoff:          2 * time.Millisecond,
		MaxAttempts:         4,
		ReplaySkew:          time.Second,
		MaxTaskPollFailures: 3,
	}
}

const (
	testType = "Volcengine::PrivateLink::EndpointService"
	testID   = "epsvc-1tdv9zkxndce8ohcb2yuz2ru"
)

func deleteOp(timing *Timing) Operation {
	return Operation{
		Kind:        OperationDelete,
		TypeName:    testType,
		Region:      "cn-beijing",
		Identifier:  testID,
		StableToken: DeleteOperationToken(testType, testID),
		Timing:      timing,
	}
}

// TestRunDeleteReplayedFailureUsesNewToken reproduces the production case: the
// stable token replays a FAILED InvalidLock task from an earlier reconcile, so
// the executor must submit again with a different token.
func TestRunDeleteReplayedFailureUsesNewToken(t *testing.T) {
	t.Parallel()
	f := newFakeCC()
	old := time.Now().Add(-10 * time.Minute)
	stable := DeleteOperationToken(testType, testID)
	f.byToken[stable] = failed("task-old", "InvalidRequest", "InvalidRequest: InvalidEndpointService.InvalidLock: The operation is refused by the endpoint service.", old)
	f.verdicts = []cloudcontrol.ProgressEvent{event(base.SUCCESS, "task-new", time.Now())}

	res, err := Run(context.Background(), f, deleteOp(fastTiming()))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if res.Attempts != 2 || res.TaskID != "task-new" {
		t.Fatalf("attempts=%d task=%q, want 2 / task-new", res.Attempts, res.TaskID)
	}
	if f.submits != 2 {
		t.Fatalf("submits = %d, want 2", f.submits)
	}
	if len(f.tokens) != 1 || f.tokens[0] == stable {
		t.Fatalf("second submission must use a fresh token, got %v", f.tokens)
	}
}

// TestRunDeleteReplayedTerminalCodeRetriesOnce verifies that a replayed verdict
// is re-established with a new token even when its error code looks terminal,
// and that a fresh terminal verdict then stops the operation.
func TestRunDeleteReplayedTerminalCodeRetriesOnce(t *testing.T) {
	t.Parallel()
	f := newFakeCC()
	old := time.Now().Add(-time.Hour)
	f.byToken[DeleteOperationToken(testType, testID)] = failed("task-old", "AccessDenied", "AccessDenied", old)
	f.verdicts = []cloudcontrol.ProgressEvent{failed("task-fresh", "AccessDenied", "AccessDenied", time.Now())}

	_, err := Run(context.Background(), f, deleteOp(fastTiming()))
	var opErr *OperationError
	if !errors.As(err, &opErr) {
		t.Fatalf("error = %v, want *OperationError", err)
	}
	if opErr.Class != FailureTerminal || opErr.TaskID != "task-fresh" || opErr.Attempts != 2 {
		t.Fatalf("unexpected verdict: %+v", opErr)
	}
	if f.submits != 2 {
		t.Fatalf("submits = %d, want 2", f.submits)
	}
}

func TestRunDeleteInProgressTaskIsPolledNotResubmitted(t *testing.T) {
	t.Parallel()
	f := newFakeCC()
	f.byToken[DeleteOperationToken(testType, testID)] = event(base.IN_PROGRESS, "task-running", time.Now().Add(-5*time.Minute))
	f.tasks["task-running"] = []cloudcontrol.ProgressEvent{
		event(base.IN_PROGRESS, "task-running", time.Now()),
		event(base.SUCCESS, "task-running", time.Now()),
	}

	res, err := Run(context.Background(), f, deleteOp(fastTiming()))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if f.submits != 1 || res.TaskID != "task-running" || len(f.getTasks) != 2 {
		t.Fatalf("submits=%d task=%q getTasks=%d", f.submits, res.TaskID, len(f.getTasks))
	}
}

func TestRunDeleteNotFoundStopsWithoutResubmit(t *testing.T) {
	t.Parallel()
	for name, ev := range map[string]cloudcontrol.ProgressEvent{
		"generic code":   failed("task-nf", "NotFound", "", time.Now()),
		"service code":   failed("task-nf", "InvalidEndpointService.NotFound", "", time.Now()),
		"handler marker": failed("task-nf", "InvalidRequest", "handler failed (HandlerErrorCode: NotFound)", time.Now()),
	} {
		t.Run(name, func(t *testing.T) {
			f := newFakeCC()
			f.verdicts = []cloudcontrol.ProgressEvent{ev}
			_, err := Run(context.Background(), f, deleteOp(fastTiming()))
			if !tfresource.NotFound(err) {
				t.Fatalf("error = %v, want NotFoundError", err)
			}
			if f.submits != 1 {
				t.Fatalf("submits = %d, want 1", f.submits)
			}
		})
	}
}

func TestRunDeleteAsyncNotFoundStops(t *testing.T) {
	t.Parallel()
	f := newFakeCC()
	f.verdicts = []cloudcontrol.ProgressEvent{event(base.IN_PROGRESS, "task-a", time.Now())}
	f.tasks["task-a"] = []cloudcontrol.ProgressEvent{failed("task-a", "InvalidNatGateway.NotFound", "", time.Now())}

	_, err := Run(context.Background(), f, deleteOp(fastTiming()))
	if !tfresource.NotFound(err) {
		t.Fatalf("error = %v, want NotFoundError", err)
	}
}

func TestRunDeleteFreshTerminalFailureDoesNotRetry(t *testing.T) {
	t.Parallel()
	f := newFakeCC()
	f.verdicts = []cloudcontrol.ProgressEvent{failed("task-a", "AccessDenied", "AccessDenied: no permission", time.Now())}

	_, err := Run(context.Background(), f, deleteOp(fastTiming()))
	var opErr *OperationError
	if !errors.As(err, &opErr) || opErr.Class != FailureTerminal {
		t.Fatalf("error = %v, want terminal OperationError", err)
	}
	if f.submits != 1 {
		t.Fatalf("submits = %d, want 1", f.submits)
	}
}

func TestRunDeleteRetryableExhaustsBudget(t *testing.T) {
	t.Parallel()
	timing := fastTiming()
	f := newFakeCC()
	for i := 0; i < timing.MaxAttempts; i++ {
		f.verdicts = append(f.verdicts, failed("task-"+itoa(i), "ResourceConflict", "ResourceConflict", time.Now()))
	}

	_, err := Run(context.Background(), f, deleteOp(timing))
	var opErr *OperationError
	if !errors.As(err, &opErr) || opErr.Class != FailureRetryable {
		t.Fatalf("error = %v, want retryable OperationError", err)
	}
	if f.submits != timing.MaxAttempts || opErr.Attempts != timing.MaxAttempts {
		t.Fatalf("submits=%d attempts=%d, want %d", f.submits, opErr.Attempts, timing.MaxAttempts)
	}
	seen := map[string]bool{}
	for _, tok := range f.tokens {
		if seen[tok] {
			t.Fatalf("token %q reused", tok)
		}
		seen[tok] = true
	}
}

func TestRunDeleteAsyncRetryableFailureRetries(t *testing.T) {
	t.Parallel()
	f := newFakeCC()
	f.verdicts = []cloudcontrol.ProgressEvent{
		event(base.IN_PROGRESS, "task-a", time.Now()),
		event(base.SUCCESS, "task-b", time.Now()),
	}
	f.tasks["task-a"] = []cloudcontrol.ProgressEvent{failed("task-a", "InvalidRequest", "InvalidRequest: InvalidEndpointService.InvalidLock: locked", time.Now())}

	res, err := Run(context.Background(), f, deleteOp(fastTiming()))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if res.TaskID != "task-b" || res.Attempts != 2 {
		t.Fatalf("task=%q attempts=%d", res.TaskID, res.Attempts)
	}
}

func TestRunTransportErrorRetriesWithSameToken(t *testing.T) {
	t.Parallel()
	f := newFakeCC()
	f.submitErrs = []error{volcengineerr.NewRequestFailure(volcengineerr.New("ServiceUnavailable", "try later", nil), 503, "req"), nil}
	f.verdicts = []cloudcontrol.ProgressEvent{event(base.SUCCESS, "task-a", time.Now())}

	_, err := Run(context.Background(), f, deleteOp(fastTiming()))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if f.submits != 2 || len(f.tokens) != 1 || f.tokens[0] != DeleteOperationToken(testType, testID) {
		t.Fatalf("expected the same stable token to be resubmitted, submits=%d tokens=%v", f.submits, f.tokens)
	}
}

func TestRunTransportNotFoundStops(t *testing.T) {
	t.Parallel()
	f := newFakeCC()
	f.submitErrs = []error{volcengineerr.New("InvalidEndpointService.NotFound", "gone", nil)}

	_, err := Run(context.Background(), f, deleteOp(fastTiming()))
	if !tfresource.NotFound(err) {
		t.Fatalf("error = %v, want NotFoundError", err)
	}
}

func TestRunGetTaskTransientErrorsTolerated(t *testing.T) {
	t.Parallel()
	f := newFakeCC()
	f.verdicts = []cloudcontrol.ProgressEvent{event(base.IN_PROGRESS, "task-a", time.Now())}
	f.getErrs = []error{volcengineerr.NewRequestFailure(volcengineerr.New("InternalError", "", nil), 500, "r"), nil}
	f.tasks["task-a"] = []cloudcontrol.ProgressEvent{event(base.SUCCESS, "task-a", time.Now())}

	if _, err := Run(context.Background(), f, deleteOp(fastTiming())); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestRunCreateReplayedFailureUsesNewTokenAndReturnsIdentifier(t *testing.T) {
	t.Parallel()
	f := newFakeCC()
	stable := CreateOperationToken(testType, "uid-a")
	f.byToken[stable] = failed("task-old", "InternalError", "InternalError", time.Now().Add(-time.Hour))
	ok := event(base.IN_PROGRESS, "task-new", time.Now())
	f.verdicts = []cloudcontrol.ProgressEvent{ok}
	done := event(base.SUCCESS, "task-new", time.Now())
	done.Identifier = str("epsvc-created")
	f.tasks["task-new"] = []cloudcontrol.ProgressEvent{done}

	state := map[string]any{"ServiceName": "x"}
	res, err := Run(context.Background(), f, Operation{
		Kind:        OperationCreate,
		TypeName:    testType,
		Region:      "cn-beijing",
		TargetState: &state,
		StableToken: stable,
		Timing:      fastTiming(),
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if res.Event == nil || res.Event.Identifier == nil || *res.Event.Identifier != "epsvc-created" {
		t.Fatalf("identifier not propagated: %+v", res.Event)
	}
	if len(f.tokens) != 1 || f.tokens[0] == stable {
		t.Fatalf("create must retry with a fresh token, got %v", f.tokens)
	}
}

func TestRunCreateFailedWithIdentifierDoesNotResubmit(t *testing.T) {
	t.Parallel()
	f := newFakeCC()
	ev := failed("task-a", "InternalError", "partial", time.Now())
	ev.Identifier = str("epsvc-partial")
	f.verdicts = []cloudcontrol.ProgressEvent{ev}

	state := map[string]any{}
	_, err := Run(context.Background(), f, Operation{
		Kind:        OperationCreate,
		TypeName:    testType,
		TargetState: &state,
		StableToken: CreateOperationToken(testType, "uid-b"),
		Timing:      fastTiming(),
	})
	var opErr *OperationError
	if !errors.As(err, &opErr) || opErr.Identifier != "epsvc-partial" || opErr.Class != FailureTerminal {
		t.Fatalf("error = %v, want terminal OperationError carrying the identifier", err)
	}
	if f.submits != 1 {
		t.Fatalf("submits = %d, want 1", f.submits)
	}
}

func TestRunUpdateReplayedConflictRetries(t *testing.T) {
	t.Parallel()
	f := newFakeCC()
	patch := `[{"op":"replace","path":"/Description","value":"x"}]`
	stable := UpdateOperationToken(testType, testID, patch)
	f.byToken[stable] = failed("task-old", "ResourceConflict", "ResourceConflict", time.Now().Add(-time.Hour))
	f.verdicts = []cloudcontrol.ProgressEvent{event(base.SUCCESS, "task-new", time.Now())}

	res, err := Run(context.Background(), f, Operation{
		Kind:          OperationUpdate,
		TypeName:      testType,
		Region:        "cn-beijing",
		Identifier:    testID,
		PatchDocument: []any{map[string]any{"op": "replace", "path": "/Description", "value": "x"}},
		StableToken:   stable,
		Timing:        fastTiming(),
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if res.TaskID != "task-new" || f.submits != 2 {
		t.Fatalf("task=%q submits=%d", res.TaskID, f.submits)
	}
}

func TestRunUpdateNotFoundSurfaces(t *testing.T) {
	t.Parallel()
	f := newFakeCC()
	f.verdicts = []cloudcontrol.ProgressEvent{failed("task-a", "InvalidNatGateway.NotFound", "", time.Now())}
	_, err := Run(context.Background(), f, Operation{
		Kind:          OperationUpdate,
		TypeName:      testType,
		Identifier:    testID,
		PatchDocument: []any{},
		StableToken:   UpdateOperationToken(testType, testID, "[]"),
		Timing:        fastTiming(),
	})
	if !tfresource.NotFound(err) {
		t.Fatalf("error = %v, want NotFoundError", err)
	}
}

func TestRunContextCancelledDuringBackoff(t *testing.T) {
	t.Parallel()
	f := newFakeCC()
	f.verdicts = []cloudcontrol.ProgressEvent{failed("task-a", "ResourceConflict", "", time.Now())}
	timing := fastTiming()
	timing.InitialBackoff = time.Hour
	timing.MaxBackoff = time.Hour
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	_, err := Run(ctx, f, deleteOp(timing))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if f.submits != 1 {
		t.Fatalf("submits = %d, want 1", f.submits)
	}
}

func TestRunValidation(t *testing.T) {
	t.Parallel()
	f := newFakeCC()
	for name, op := range map[string]Operation{
		"missing type":   {Kind: OperationDelete, Identifier: "x", StableToken: "t"},
		"missing token":  {Kind: OperationDelete, TypeName: "T", Identifier: "x"},
		"delete no id":   {Kind: OperationDelete, TypeName: "T", StableToken: "t"},
		"create no body": {Kind: OperationCreate, TypeName: "T", StableToken: "t"},
		"unknown kind":   {Kind: "drop", TypeName: "T", StableToken: "t"},
	} {
		if _, err := Run(context.Background(), f, op); err == nil {
			t.Fatalf("%s: expected validation error", name)
		}
	}
	if f.submits != 0 {
		t.Fatalf("validation must not reach the API, submits=%d", f.submits)
	}
}

func TestDeleteResourceKeepsLegacySignature(t *testing.T) {
	t.Parallel()
	f := newFakeCC()
	f.verdicts = []cloudcontrol.ProgressEvent{event(base.SUCCESS, "task-a", time.Now())}
	if err := DeleteResource(context.Background(), f, "cn-beijing", "", testType, testID); err != nil {
		t.Fatalf("DeleteResource() error = %v", err)
	}
	if f.tokens[0] != DeleteOperationToken(testType, testID) {
		t.Fatalf("first attempt must use the stable delete token")
	}
}

func TestClassifyFailure(t *testing.T) {
	t.Parallel()
	now := time.Now()
	cases := map[string]struct {
		ev   cloudcontrol.ProgressEvent
		want FailureClass
	}{
		"invalid lock in status message only": {failed("t", "InvalidRequest", "InvalidRequest: InvalidEndpointService.InvalidLock: refused", now), FailureRetryable},
		"resource conflict code":              {failed("t", "ResourceConflict", "", now), FailureRetryable},
		"flow limit":                          {failed("t", "AccountFlowLimitExceeded", "", now), FailureRetryable},
		"internal error":                      {failed("t", "InternalError", "", now), FailureRetryable},
		"access denied":                       {failed("t", "AccessDenied", "AccessDenied", now), FailureTerminal},
		"invalid parameter":                   {failed("t", "InvalidParameter", "InvalidParameter: bad", now), FailureTerminal},
		"quota":                               {failed("t", "QuotaExceed.EndpointEachVpc", "", now), FailureTerminal},
		"not found":                           {failed("t", "InvalidVpc.NotFound", "", now), FailureNotFound},
		"empty":                               {event(base.FAILED, "t", now), FailureTerminal},
	}
	for name, c := range cases {
		if got := classifyFailure(&c.ev); got != c.want {
			t.Errorf("%s: classifyFailure = %s, want %s", name, got, c.want)
		}
	}
}

func TestIsReplay(t *testing.T) {
	t.Parallel()
	start := time.Now()
	if !isReplay(start.Add(-10*time.Minute), start, time.Minute) {
		t.Fatal("ten-minute-old event must be a replay")
	}
	if isReplay(start.Add(-10*time.Second), start, time.Minute) {
		t.Fatal("event within skew must not be a replay")
	}
	if isReplay(time.Time{}, start, time.Minute) {
		t.Fatal("zero event time cannot be judged as replay")
	}
}

func TestAttemptTokenIsUniqueAnd32Chars(t *testing.T) {
	t.Parallel()
	stable := DeleteOperationToken(testType, testID)
	a, b := AttemptToken(stable), AttemptToken(stable)
	if len(a) != 32 || len(b) != 32 {
		t.Fatalf("lengths %d/%d, want 32", len(a), len(b))
	}
	if a == b || a == stable {
		t.Fatalf("attempt tokens must differ from each other and from the stable token")
	}
	if UpdateOperationToken(testType, testID, "p") != OperationToken("update", testType, testID, "p") ||
		DeleteOperationToken(testType, testID) != OperationToken("delete", testType, testID, "") {
		t.Fatal("stable token helpers must match the historical OperationToken algorithm")
	}
}

func TestIsRetryableTransportError(t *testing.T) {
	t.Parallel()
	if !isRetryableTransportError(volcengineerr.NewRequestFailure(volcengineerr.New("X", "", nil), 502, "r")) {
		t.Fatal("5xx must be retryable")
	}
	if !isRetryableTransportError(volcengineerr.NewRequestFailure(volcengineerr.New("X", "", nil), 429, "r")) {
		t.Fatal("429 must be retryable")
	}
	if isRetryableTransportError(volcengineerr.NewRequestFailure(volcengineerr.New("AccessDenied", "", nil), 403, "r")) {
		t.Fatal("403 AccessDenied must not be retryable")
	}
	if isRetryableTransportError(context.Canceled) {
		t.Fatal("cancellation must not be retried")
	}
	if !isRetryableTransportError(volcengineerr.New("RequestError", "send request failed", errors.New("read tcp: connection reset by peer"))) {
		t.Fatal("network reset must be retryable")
	}
}
