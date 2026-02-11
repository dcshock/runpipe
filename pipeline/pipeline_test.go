package pipeline

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

// --- Pipeline: RunWithInput, Run, observer, errors ---

func TestPipeline_RunWithInput_NoObserver(t *testing.T) {
	ctx := context.Background()
	p := &Pipeline{
		Name: "simple",
		Stages: []Stage{
			Transform(func(ctx context.Context, n int) (int, error) { return n * 2, nil }),
			Transform(func(ctx context.Context, n int) (int, error) { return n + 1, nil }),
		},
	}
	out, err := p.RunWithInput(ctx, 5, nil)
	if err != nil {
		t.Fatal(err)
	}
	if out != 11 {
		t.Errorf("expected 11, got %v", out)
	}
}

func TestPipeline_RunWithInput_WithObserver(t *testing.T) {
	ctx := context.Background()
	var runIDSeen string
	var order []string
	obs := &hookObserver{
		beforePipeline: func(ctx context.Context, runID, name string, payload interface{}) error {
			runIDSeen = runID
			order = append(order, "BeforePipeline:"+name)
			return nil
		},
		afterPipeline: func(ctx context.Context, runID string, result interface{}, err error) error {
			order = append(order, "AfterPipeline")
			return nil
		},
		beforeStage: func(ctx context.Context, runID string, stageIndex int, input interface{}) error {
			order = append(order, fmt.Sprintf("BeforeStage:%d", stageIndex))
			return nil
		},
		afterStage: func(ctx context.Context, runID string, stageIndex int, input, output interface{}, stageErr error, d time.Duration) error {
			order = append(order, fmt.Sprintf("AfterStage:%d", stageIndex))
			return nil
		},
	}

	p := &Pipeline{
		Name: "observed",
		Stages: []Stage{
			Identity(),
			Identity(),
		},
	}
	_, err := p.RunWithInput(ctx, 42, &RunOptions{Observer: obs})
	if err != nil {
		t.Fatal(err)
	}
	if runIDSeen == "" {
		t.Error("expected runID to be generated")
	}
	want := []string{"BeforePipeline:observed", "BeforeStage:0", "AfterStage:0", "BeforeStage:1", "AfterStage:1", "AfterPipeline"}
	if len(order) != len(want) {
		t.Fatalf("order: got %d hooks, want %d: %v", len(order), len(want), order)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Errorf("order[%d]: got %q, want %q", i, order[i], want[i])
		}
	}
}

func TestPipeline_RunWithInput_WithObserverAndRunID(t *testing.T) {
	ctx := context.Background()
	var runIDSeen string
	obs := &hookObserver{
		beforePipeline: func(ctx context.Context, runID, name string, payload interface{}) error {
			runIDSeen = runID
			return nil
		},
	}
	p := &Pipeline{Name: "x", Stages: []Stage{Identity()}}
	opts := &RunOptions{Observer: obs, RunID: "my-run-123"}
	_, err := p.RunWithInput(ctx, nil, opts)
	if err != nil {
		t.Fatal(err)
	}
	if runIDSeen != "my-run-123" {
		t.Errorf("runID: got %q, want my-run-123", runIDSeen)
	}
}

func TestPipeline_Run_WithSource(t *testing.T) {
	ctx := context.Background()
	called := false
	p := &Pipeline{
		Name: "with-source",
		Source: func(ctx context.Context) (interface{}, error) {
			called = true
			return 100, nil
		},
		Stages: []Stage{
			Transform(func(ctx context.Context, n int) (int, error) { return n + 1, nil }),
		},
	}
	out, err := p.Run(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Error("Source was not called")
	}
	if out != 101 {
		t.Errorf("expected 101, got %v", out)
	}
}

func TestPipeline_StageError(t *testing.T) {
	ctx := context.Background()
	errFail := errors.New("stage failed")
	p := &Pipeline{
		Name: "fail",
		Stages: []Stage{
			Identity(),
			func(ctx context.Context, input interface{}) (interface{}, error) {
				return nil, errFail
			},
			Identity(),
		},
	}
	_, err := p.RunWithInput(ctx, nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, errFail) && !errors.Is(err, fmt.Errorf("stage 1: %w", errFail)) {
		t.Errorf("expected wrapped stage error, got %v", err)
	}
}

func TestPipeline_EmptyStages(t *testing.T) {
	ctx := context.Background()
	p := &Pipeline{Name: "empty", Stages: nil}
	out, err := p.RunWithInput(ctx, 7, nil)
	if err != nil {
		t.Fatal(err)
	}
	if out != 7 {
		t.Errorf("expected 7, got %v", out)
	}
}

func TestPipeline_StageOffset(t *testing.T) {
	ctx := context.Background()
	var indices []int
	obs := &hookObserver{
		beforeStage: func(ctx context.Context, runID string, stageIndex int, input interface{}) error {
			indices = append(indices, stageIndex)
			return nil
		},
	}
	p := &Pipeline{
		Name:   "offset",
		Stages: []Stage{Identity(), Identity()},
	}
	_, err := p.RunWithInput(ctx, nil, &RunOptions{Observer: obs, StageOffset: 10})
	if err != nil {
		t.Fatal(err)
	}
	want := []int{10, 11}
	if len(indices) != 2 || indices[0] != 10 || indices[1] != 11 {
		t.Errorf("stage indices with offset: got %v, want %v", indices, want)
	}
}

// --- Sequence ---

func TestSequence_Run_SamePayload(t *testing.T) {
	ctx := context.Background()
	var inputs []interface{}
	p1 := &Pipeline{
		Name: "first",
		Stages: []Stage{
			Tap(func(ctx context.Context, v interface{}) { inputs = append(inputs, v) }),
			Transform(func(ctx context.Context, s string) (int, error) { return len(s), nil }),
		},
	}
	p2 := &Pipeline{
		Name: "second",
		Stages: []Stage{
			Tap(func(ctx context.Context, v interface{}) { inputs = append(inputs, v) }),
			Identity(),
		},
	}
	seq := &Sequence{Name: "seq", Pipelines: []*Pipeline{p1, p2}}
	payload := "hello"
	out, err := seq.Run(ctx, payload, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Each pipeline receives the same payload
	if len(inputs) != 2 || inputs[0] != payload || inputs[1] != payload {
		t.Errorf("inputs: got %v", inputs)
	}
	// Result is last pipeline's output (identity of payload)
	if out != payload {
		t.Errorf("result: got %v", out)
	}
}

func TestSequence_WithObserver_GlobalIndices(t *testing.T) {
	ctx := context.Background()
	var indices []int
	obs := &hookObserver{
		beforeStage: func(ctx context.Context, runID string, stageIndex int, input interface{}) error {
			indices = append(indices, stageIndex)
			return nil
		},
	}
	p1 := &Pipeline{Name: "a", Stages: []Stage{Identity(), Identity()}}
	p2 := &Pipeline{Name: "b", Stages: []Stage{Identity()}}
	seq := &Sequence{Name: "seq", Pipelines: []*Pipeline{p1, p2}}
	_, err := seq.Run(ctx, nil, &RunOptions{Observer: obs})
	if err != nil {
		t.Fatal(err)
	}
	want := []int{0, 1, 2}
	if len(indices) != 3 || indices[0] != 0 || indices[1] != 1 || indices[2] != 2 {
		t.Errorf("global stage indices: got %v, want %v", indices, want)
	}
}

func TestSequence_FirstPipelineError(t *testing.T) {
	ctx := context.Background()
	errFail := errors.New("fail")
	p1 := &Pipeline{
		Name:   "fail",
		Stages: []Stage{func(ctx context.Context, in interface{}) (interface{}, error) { return nil, errFail }},
	}
	p2 := &Pipeline{Name: "ok", Stages: []Stage{Identity()}}
	seq := &Sequence{Name: "seq", Pipelines: []*Pipeline{p1, p2}}
	_, err := seq.Run(ctx, nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- Park ---

func TestParkStage_RequiresObserver(t *testing.T) {
	ctx := context.Background()
	persist := func(ctx context.Context, state RunState) error { return nil }
	p := &Pipeline{
		Name:   "park",
		Stages: []Stage{ParkStage(persist)},
	}
	_, err := p.RunWithInput(ctx, 1, nil)
	if err == nil {
		t.Fatal("expected error when parking without observer")
	}
}

func TestParkStageAfter_SetsResumeAt(t *testing.T) {
	ctx := context.Background()
	var saved ParkedRun
	persist := func(ctx context.Context, parked ParkedRun) error {
		saved = parked
		return nil
	}
	obs := &hookObserver{}
	p := &Pipeline{
		Name:   "park-after",
		Stages: []Stage{ParkStageAfter(5*time.Second, persist)},
	}
	_, err := p.RunWithInput(ctx, "input", &RunOptions{Observer: obs})
	if err == nil || !IsParked(err) {
		t.Fatalf("expected ErrParked, got %v", err)
	}
	if saved.ResumeAt.IsZero() {
		t.Error("ResumeAt should be set")
	}
	delay := time.Until(saved.ResumeAt)
	if delay < 4*time.Second || delay > 6*time.Second {
		t.Errorf("ResumeAt delay: got %v", delay)
	}
	if saved.NextStageIndex != 1 {
		t.Errorf("NextStageIndex: got %d, want 1", saved.NextStageIndex)
	}
	if saved.InputForNextStage != "input" {
		t.Errorf("InputForNextStage: got %v", saved.InputForNextStage)
	}
}

// --- Retry ---

func TestRetry_RetryableErr_Parks(t *testing.T) {
	ctx := context.Background()
	var parked ParkedRun
	persist := func(ctx context.Context, p ParkedRun) error {
		parked = p
		return nil
	}
	inner := func(ctx context.Context, in interface{}) (interface{}, error) {
		return nil, RetryableErr(errors.New("transient"))
	}
	policy := RetryPolicy{Backoff: time.Minute, ShouldRetry: IsRetryable}
	stage := Retry(inner, policy, persist)
	obs := &hookObserver{}
	p := &Pipeline{Name: "retry", Stages: []Stage{stage}}
	_, err := p.RunWithInput(ctx, 1, &RunOptions{Observer: obs})
	if err == nil || !IsParked(err) {
		t.Fatalf("expected ErrParked, got %v", err)
	}
	if parked.NextStageIndex != 0 {
		t.Errorf("NextStageIndex should be 0 (re-run same stage), got %d", parked.NextStageIndex)
	}
	if parked.InputForNextStage != 1 {
		t.Errorf("InputForNextStage: got %v", parked.InputForNextStage)
	}
}

func TestRetry_NonRetryable_Propagates(t *testing.T) {
	ctx := context.Background()
	persistCalled := false
	persist := func(ctx context.Context, p ParkedRun) error {
		persistCalled = true
		return nil
	}
	inner := func(ctx context.Context, in interface{}) (interface{}, error) {
		return nil, errors.New("permanent")
	}
	policy := RetryPolicy{Backoff: time.Minute, ShouldRetry: IsRetryable}
	stage := Retry(inner, policy, persist)
	p := &Pipeline{Name: "retry", Stages: []Stage{stage}}
	_, err := p.RunWithInput(ctx, 1, &RunOptions{Observer: &hookObserver{}})
	if err == nil {
		t.Fatal("expected error")
	}
	if persistCalled {
		t.Error("persist should not be called for non-retryable error")
	}
}

func TestRetry_RequiresObserver(t *testing.T) {
	ctx := context.Background()
	inner := func(ctx context.Context, in interface{}) (interface{}, error) {
		return nil, RetryableErr(errors.New("x"))
	}
	persist := func(ctx context.Context, p ParkedRun) error { return nil }
	stage := Retry(inner, RetryPolicy{Backoff: time.Second, ShouldRetry: IsRetryable}, persist)
	p := &Pipeline{Name: "retry", Stages: []Stage{stage}}
	_, err := p.RunWithInput(ctx, 1, nil)
	if err == nil {
		t.Fatal("expected error when retry without observer")
	}
}

// --- Resume (remaining stages with StageOffset) ---

func TestResume_RemainingStages(t *testing.T) {
	ctx := context.Background()
	var runOrder []int
	mkStage := func(id int) Stage {
		return func(ctx context.Context, in interface{}) (interface{}, error) {
			runOrder = append(runOrder, id)
			return in, nil
		}
	}
	full := &Pipeline{
		Name:   "full",
		Stages: []Stage{mkStage(0), mkStage(1), mkStage(2), mkStage(3)},
	}
	// Simulate: we "parked" after stage 1; resume from stage 2 and 3
	remaining := &Pipeline{
		Name:   full.Name,
		Stages: full.Stages[2:],
	}
	inputForStage2 := 100
	opts := &RunOptions{Observer: &hookObserver{}, RunID: "resume-1", StageOffset: 2}
	out, err := remaining.RunWithInput(ctx, inputForStage2, opts)
	if err != nil {
		t.Fatal(err)
	}
	if out != inputForStage2 {
		t.Errorf("output: got %v", out)
	}
	if len(runOrder) != 2 || runOrder[0] != 2 || runOrder[1] != 3 {
		t.Errorf("runOrder: got %v, want [2, 3]", runOrder)
	}
}

// --- Observer helpers ---

type hookObserver struct {
	beforePipeline func(context.Context, string, string, interface{}) error
	afterPipeline  func(context.Context, string, interface{}, error) error
	beforeStage    func(context.Context, string, int, interface{}) error
	afterStage     func(context.Context, string, int, interface{}, interface{}, error, time.Duration) error
}

func (h *hookObserver) BeforePipeline(ctx context.Context, runID, name string, payload interface{}) error {
	if h.beforePipeline != nil {
		return h.beforePipeline(ctx, runID, name, payload)
	}
	return nil
}

func (h *hookObserver) AfterPipeline(ctx context.Context, runID string, result interface{}, err error) error {
	if h.afterPipeline != nil {
		return h.afterPipeline(ctx, runID, result, err)
	}
	return nil
}

func (h *hookObserver) BeforeStage(ctx context.Context, runID string, stageIndex int, input interface{}) error {
	if h.beforeStage != nil {
		return h.beforeStage(ctx, runID, stageIndex, input)
	}
	return nil
}

func (h *hookObserver) AfterStage(ctx context.Context, runID string, stageIndex int, input, output interface{}, stageErr error, d time.Duration) error {
	if h.afterStage != nil {
		return h.afterStage(ctx, runID, stageIndex, input, output, stageErr, d)
	}
	return nil
}
