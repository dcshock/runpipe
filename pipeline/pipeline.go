package pipeline

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ConvertFunc converts value of type A to type B. Used by Transform to build a stage.
type ConvertFunc[A, B any] func(ctx context.Context, a A) (B, error)

// Transform returns a stage that converts the previous stage's output (type A) to type B.
// Use it between stages: stage1 | Transform(convert) | stage2, where stage1 outputs A
// and stage2 expects B.
func Transform[A, B any](convert ConvertFunc[A, B]) Stage {
	return func(ctx context.Context, input interface{}) (interface{}, error) {
		a, ok := input.(A)
		if !ok {
			var zero A
			return nil, fmt.Errorf("transform: expected %T, got %T", zero, input)
		}
		return convert(ctx, a)
	}
}

// RetryPolicy configures how Retry retries a stage via park/resume. Backoff is the
// delay before the run is resumed (ResumeAt = now + Backoff). If ShouldRetry is
// non-nil, only errors for which it returns true are retried; otherwise all errors
// are retried. Use RetryableErr(err) in a stage to mark an error as retryable.
// MaxAttempts is not enforced by the pipeline; your persist callback can track
// attempt count per run_id and refuse to persist (or mark failed) after N parks.
type RetryPolicy struct {
	MaxAttempts int           // optional; enforce in persist (e.g. don't persist if attempt >= N)
	Backoff     time.Duration // delay before resume (ResumeAt = now + Backoff)
	ShouldRetry func(err error) bool
}

// Retryable marks err as retryable. Use with RetryPolicy.ShouldRetry so only
// these errors trigger a retry (e.g. transient failures), not permanent ones.
type Retryable struct{ Err error }

func (e *Retryable) Error() string { return e.Err.Error() }
func (e *Retryable) Unwrap() error { return e.Err }
func RetryableErr(err error) error { return &Retryable{Err: err} }
func IsRetryable(err error) bool   { return errors.As(err, new(*Retryable)) }

// ErrParked is returned by ParkStage (and by Retry on retryable failure) after
// successfully persisting run state for later execution. Callers should treat it
// as "pipeline paused, not failed": do not retry the run; the resume job will
// run it when due.
var ErrParked = errors.New("pipeline parked for later execution")

func IsParked(err error) bool { return errors.Is(err, ErrParked) }

// Retry wraps a stage so that on retryable failure it persists a ParkedRun and
// returns ErrParked instead of blocking. The same run is resumed later by your
// periodic resume job (when resume_at <= now); the resumed run re-executes this
// stage. Requires RunOptions.Observer and RunID. Enforce MaxAttempts in your
// persist callback (e.g. track attempt per run_id and do not persist after N).
func Retry(inner Stage, policy RetryPolicy, persist ParkPersistWithTime) Stage {
	return func(ctx context.Context, input interface{}) (interface{}, error) {
		out, err := inner(ctx, input)
		if err == nil {
			return out, nil
		}
		if policy.ShouldRetry != nil && !policy.ShouldRetry(err) {
			return nil, err
		}
		meta, ok := runMetaFromContext(ctx)
		if !ok {
			return nil, fmt.Errorf("retry: pipeline must be run with Observer and RunID")
		}
		// Re-run this stage (same index), not the next one.
		parked := ParkedRun{
			RunState: RunState{
				RunID:             meta.RunID,
				PipelineName:      meta.PipelineName,
				NextStageIndex:    meta.StageIndex,
				InputForNextStage: input,
			},
			ResumeAt: time.Now().Add(policy.Backoff),
		}
		if err := persist(ctx, parked); err != nil {
			return nil, fmt.Errorf("retry: persist: %w", err)
		}
		return nil, ErrParked
	}
}

// Observer provides pre/post hooks for pipeline and stage execution so you can
// persist run state (e.g. to a DB) for monitoring and restart. BeforePipeline is
// called before any stage runs (write the run record here). BeforeStage/AfterStage
// are called around each stage (log start/end, input/output, duration). AfterPipeline
// is called when the pipeline finishes (success or error).
type Observer interface {
	BeforePipeline(ctx context.Context, runID, name string, payload interface{}) error
	AfterPipeline(ctx context.Context, runID string, result interface{}, err error) error
	BeforeStage(ctx context.Context, runID string, stageIndex int, input interface{}) error
	AfterStage(ctx context.Context, runID string, stageIndex int, input, output interface{}, stageErr error, duration time.Duration) error
}

// RunOptions is optional and used to attach an Observer and optional RunID.
// If Observer is set and RunID is empty, a new UUID is generated for the run.
// StageOffset is added to each stage index when calling the Observer (use when
// resuming: run a pipeline with only the remaining stages and set StageOffset to
// the index of the first stage being run so observer sees correct global indices).
type RunOptions struct {
	Observer    Observer
	RunID       string
	StageOffset int
}

// RunState is the minimal state to persist so a pipeline run can be resumed after
// shutdown. Your Observer should store these (e.g. in AfterStage on success, or when
// the run is interrupted). When resuming, load RunState from the DB, get the
// original Pipeline by name, then run remaining stages with RunWithInput and
// StageOffset as described in the package doc.
type RunState struct {
	RunID             string      // same RunID used when the run started
	PipelineName      string      // name of the Pipeline (to look up Stages)
	NextStageIndex    int         // 0-based index of the first stage to run when resuming
	InputForNextStage interface{} // value to pass as input to that stage (must be serializable for DB)
}

// context keys for run metadata (injected when Observer is set; used by ParkStage)
type runMetaKey struct{}

type runMeta struct {
	RunID, PipelineName string
	StageIndex          int
}

func runMetaFromContext(ctx context.Context) (runMeta, bool) {
	m, ok := ctx.Value(runMetaKey{}).(runMeta)
	return m, ok
}

// ParkPersist is called by ParkStage to save run state for later execution (e.g. to a DB).
// The state should be serializable so it can be resumed after a delay or restart.
type ParkPersist func(ctx context.Context, state RunState) error

// ParkedRun extends RunState with an optional time when the run should be resumed.
// Persist it (e.g. to a DB with a resume_at column) and run a periodic job that
// selects parked runs where resume_at <= now and resumes them.
type ParkedRun struct {
	RunState
	ResumeAt time.Time // when to resume (zero = no time; caller decides when)
}

// ParkPersistWithTime is used by ParkStageAfter to save run state and a resume time.
type ParkPersistWithTime func(ctx context.Context, parked ParkedRun) error

// ParkStage is a stage that persists the current run state and returns ErrParked so the
// pipeline stops without failing. The run can be resumed later by loading RunState and
// calling RunWithInput with remaining stages and StageOffset (see package doc).
// Park only works when the pipeline is run with RunOptions.Observer and a RunID (or
// generated one); otherwise the stage returns an error.
func ParkStage(persist ParkPersist) Stage {
	return func(ctx context.Context, input interface{}) (interface{}, error) {
		meta, ok := runMetaFromContext(ctx)
		if !ok {
			return nil, fmt.Errorf("park: pipeline must be run with Observer and RunID")
		}
		state := RunState{
			RunID:             meta.RunID,
			PipelineName:      meta.PipelineName,
			NextStageIndex:    meta.StageIndex + 1,
			InputForNextStage: input,
		}
		if err := persist(ctx, state); err != nil {
			return nil, fmt.Errorf("park: persist: %w", err)
		}
		return nil, ErrParked
	}
}

// ParkStageAfter is like ParkStage but sets ResumeAt to now + delay before calling
// persist. Persist the ParkedRun (including ResumeAt) to storage; a periodic job
// should select parked runs where resume_at <= now and resume them (see package doc).
func ParkStageAfter(delay time.Duration, persist ParkPersistWithTime) Stage {
	return func(ctx context.Context, input interface{}) (interface{}, error) {
		meta, ok := runMetaFromContext(ctx)
		if !ok {
			return nil, fmt.Errorf("park: pipeline must be run with Observer and RunID")
		}
		parked := ParkedRun{
			RunState: RunState{
				RunID:             meta.RunID,
				PipelineName:      meta.PipelineName,
				NextStageIndex:    meta.StageIndex + 1,
				InputForNextStage: input,
			},
			ResumeAt: time.Now().Add(delay),
		}
		if err := persist(ctx, parked); err != nil {
			return nil, fmt.Errorf("park: persist: %w", err)
		}
		return nil, ErrParked
	}
}

// Stage is a single step in a pipeline. It receives the output of the previous
// stage (or the source) and returns the input for the next stage.
type Stage func(ctx context.Context, input interface{}) (interface{}, error)

// Pipeline runs a linear chain of stages (stage1 | stage2 | ...). Optionally
// Source can be set for standalone Run(ctx); when used inside a Sequence,
// the payload is piped in via RunWithInput and Source is ignored.
type Pipeline struct {
	Name   string
	Source func(ctx context.Context) (interface{}, error) // optional; used only by Run(ctx)
	Stages []Stage
}

// Run executes the pipeline: runs the source (if non-nil), then runs each stage in order.
// Returns the last stage's output or the first error. Use RunWithInput when the initial
// input is supplied by a Sequence.
func (p *Pipeline) Run(ctx context.Context) (interface{}, error) {
	var out interface{}
	var err error
	if p.Source != nil {
		out, err = p.Source(ctx)
		if err != nil {
			return nil, err
		}
	}
	return p.RunWithInput(ctx, out, nil)
}

// RunWithInput runs the pipeline's stages starting with the given input. The payload
// is piped to the first stage; each stage's output is the next stage's input.
// Returns the last stage's output or the first error.
// If opts is non-nil and opts.Observer is set, pre/post hooks are called for the
// pipeline and each stage (for DB persistence, logging, and restart support).
func (p *Pipeline) RunWithInput(ctx context.Context, input interface{}, opts *RunOptions) (interface{}, error) {
	if opts == nil || opts.Observer == nil {
		return p.runStages(ctx, input, nil, "", 0)
	}
	runID := opts.RunID
	if runID == "" {
		runID = uuid.New().String()
	}
	stageOffset := opts.StageOffset
	if err := opts.Observer.BeforePipeline(ctx, runID, p.Name, input); err != nil {
		return nil, fmt.Errorf("before pipeline: %w", err)
	}
	result, err := p.runStages(ctx, input, opts.Observer, runID, stageOffset)
	if postErr := opts.Observer.AfterPipeline(ctx, runID, result, err); postErr != nil {
		// Don't mask pipeline error
		if err == nil {
			err = fmt.Errorf("after pipeline: %w", postErr)
		}
	}
	return result, err
}

// runStages runs stages with optional observer hooks. stageOffset is added to
// the stage index when calling the observer (for resume: run remaining stages with offset).
func (p *Pipeline) runStages(ctx context.Context, input interface{}, obs Observer, runID string, stageOffset int) (interface{}, error) {
	out := input
	for i, stage := range p.Stages {
		globalIdx := i + stageOffset
		if obs != nil {
			if err := obs.BeforeStage(ctx, runID, globalIdx, out); err != nil {
				return nil, fmt.Errorf("before stage %d: %w", globalIdx, err)
			}
		}
		start := time.Now()
		stageCtx := ctx
		if obs != nil {
			// StageIndex is local to this pipeline so resume runs stages[i+1:]; globalIdx is for observer only.
			stageCtx = context.WithValue(ctx, runMetaKey{}, runMeta{RunID: runID, PipelineName: p.Name, StageIndex: i})
		}
		next, stageErr := stage(stageCtx, out)
		duration := time.Since(start)
		if obs != nil {
			if postErr := obs.AfterStage(ctx, runID, globalIdx, out, next, stageErr, duration); postErr != nil {
				if stageErr == nil {
					stageErr = fmt.Errorf("after stage: %w", postErr)
				}
			}
		}
		if stageErr != nil {
			return nil, fmt.Errorf("stage %d: %w", globalIdx, stageErr)
		}
		out = next
	}
	return out, nil
}

// Sequence runs multiple pipelines in order. The caller supplies a payload that
// is passed to every pipeline (and semantics): each pipeline receives the same
// source payload, not the previous pipeline's output. Stops on first error (like shell &&).
type Sequence struct {
	Name      string
	Pipelines []*Pipeline
}

// Run executes the sequence with the given payload. The payload is passed to
// each pipeline as its input (and semantics: payload → pipeline1, payload → pipeline2, …).
// Returns the last pipeline's result when all succeed, or the first error.
// If opts is non-nil and opts.Observer is set, hooks are called for the sequence as
// a whole and for each stage (stage indices are global across all pipelines).
func (s *Sequence) Run(ctx context.Context, payload interface{}, opts *RunOptions) (interface{}, error) {
	if opts == nil || opts.Observer == nil {
		return s.runPipelines(ctx, payload, nil, "")
	}
	runID := opts.RunID
	if runID == "" {
		runID = uuid.New().String()
	}
	if err := opts.Observer.BeforePipeline(ctx, runID, s.Name, payload); err != nil {
		return nil, fmt.Errorf("before pipeline: %w", err)
	}
	result, err := s.runPipelines(ctx, payload, opts.Observer, runID)
	if postErr := opts.Observer.AfterPipeline(ctx, runID, result, err); postErr != nil {
		if err == nil {
			err = fmt.Errorf("after pipeline: %w", postErr)
		}
	}
	return result, err
}

// runPipelines runs each pipeline in order; each receives the same payload (and semantics).
func (s *Sequence) runPipelines(ctx context.Context, payload interface{}, obs Observer, runID string) (interface{}, error) {
	var last interface{}
	globalStageIndex := 0
	for i, p := range s.Pipelines {
		if obs != nil {
			result, err := s.runPipelineWithObserver(ctx, p, payload, obs, runID, &globalStageIndex)
			if err != nil {
				return nil, fmt.Errorf("pipeline %d (%s): %w", i, p.Name, err)
			}
			last = result
		} else {
			result, err := p.RunWithInput(ctx, payload, nil)
			if err != nil {
				return nil, fmt.Errorf("pipeline %d (%s): %w", i, p.Name, err)
			}
			last = result
		}
	}
	return last, nil
}

// runPipelineWithObserver runs one pipeline's stages, calling observer for each stage with global indices.
func (s *Sequence) runPipelineWithObserver(ctx context.Context, p *Pipeline, input interface{}, obs Observer, runID string, globalIndex *int) (interface{}, error) {
	out := input
	for localIdx, stage := range p.Stages {
		idx := *globalIndex
		*globalIndex++
		if err := obs.BeforeStage(ctx, runID, idx, out); err != nil {
			return nil, fmt.Errorf("before stage %d: %w", idx, err)
		}
		start := time.Now()
		stageCtx := context.WithValue(ctx, runMetaKey{}, runMeta{RunID: runID, PipelineName: p.Name, StageIndex: localIdx})
		next, stageErr := stage(stageCtx, out)
		duration := time.Since(start)
		if postErr := obs.AfterStage(ctx, runID, idx, out, next, stageErr, duration); postErr != nil {
			if stageErr == nil {
				stageErr = fmt.Errorf("after stage: %w", postErr)
			}
		}
		if stageErr != nil {
			return nil, fmt.Errorf("stage %d: %w", idx, stageErr)
		}
		out = next
	}
	return out, nil
}
