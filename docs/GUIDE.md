# Runpipe: Implementation Guide

This guide explains how to implement pipelines, stages, observers, park/resume, and retry in runpipe.

---

## Table of contents

1. [Concepts](#concepts)
2. [Implementing stages](#implementing-stages)
3. [Building and running pipelines](#building-and-running-pipelines)
4. [Observer and RunOptions](#observer-and-runoptions)
5. [Park and resume](#park-and-resume)
6. [Retry and exponential backoff](#retry-and-exponential-backoff)
7. [Sequences](#sequences)
8. [Stdlib stages](#stdlib-stages)
9. [Complete examples](#complete-examples)

---

## Concepts

- **Pipeline**: A linear chain of **stages**. Input flows through each stage in order; each stage’s output is the next stage’s input. Optionally, a pipeline has a **Source** that produces the initial input when you call `Run(ctx)`.
- **Stage**: A function `(ctx, input) -> (output, error)`. Stages can transform data, validate it, call external services, or pause the run (park) for later.
- **Observer**: Optional hooks (before/after pipeline, before/after each stage) for persistence, logging, and resume. Required for **Park** and **Retry**.
- **Park**: A stage can “park” the run: persist state and return `ErrParked`. A separate job later **resumes** the run from the next stage (or the same stage for retry).
- **Retry**: Wraps a stage so that on retryable failure it parks and is re-run later (with fixed or exponential backoff).
- **Sequence**: Runs multiple pipelines in order; each pipeline receives the **same** payload (and semantics), not the previous pipeline’s output.

---

## Implementing stages

### Stage signature

A **Stage** is:

```go
type Stage func(ctx context.Context, input interface{}) (interface{}, error)
```

- **input**: Output of the previous stage (or the initial payload when using `RunWithInput`).
- **return**: Value for the next stage, or an error to stop the pipeline.

Example:

```go
func double(ctx context.Context, input interface{}) (interface{}, error) {
    n, ok := input.(int)
    if !ok {
        return nil, fmt.Errorf("expected int, got %T", input)
    }
    return n * 2, nil
}
```

### Type-safe stages with Transform

Use **Transform** to connect typed stages without manual type assertions:

```go
// ConvertFunc[A, B]: (ctx, A) -> (B, error)
stage := pipeline.Transform(func(ctx context.Context, r RawResult) (ProcessedResult, error) {
    return ProcessedResult{Count: len(r.Lines)}, nil
})
```

The pipeline still passes `interface{}` between stages; Transform does the assertion and calls your typed function.

### Using stdlib stages

The `pipeline` package provides reusable stages (see [Stdlib stages](#stdlib-stages)): **Identity**, **Tap**, **Validate**, **Constant**, **WithTimeout**, **MapSlice**, **FilterSlice**. Compose them with your own stages:

```go
stages := []pipeline.Stage{
    pipeline.Validate[[]string](func(s []string) bool { return len(s) > 0 }, "non-empty"),
    pipeline.Tap(logInput),
    myCustomStage,
    pipeline.MapSlice(strings.ToUpper),
}
```

---

## Building and running pipelines

### Pipeline struct

```go
type Pipeline struct {
    Name   string
    Source func(ctx context.Context) (interface{}, error) // optional
    Stages []Stage
}
```

- **Name**: Identifies the pipeline (for observers and resume).
- **Source**: Optional. Used only when you call `Run(ctx)` with no input; ignored when using `RunWithInput` or when the pipeline is part of a Sequence.
- **Stages**: Ordered list of stages.

### Run vs RunWithInput

- **Run(ctx)**  
  Runs the **Source** (if set), then runs all stages with that output. Use for standalone pipelines that produce their own input.

- **RunWithInput(ctx, input, opts)**  
  Skips Source and runs stages starting with `input`. Use when:
  - You already have a payload (e.g. from an API or a Sequence).
  - You are **resuming** a run (input = saved `InputForNextStage`, opts = `RunOptions{RunID, Observer, StageOffset}`).

Example:

```go
p := &pipeline.Pipeline{
    Name:   "my-pipeline",
    Stages: []pipeline.Stage{stage1, stage2, stage3},
}

// With initial input (no observer)
result, err := p.RunWithInput(ctx, payload, nil)

// With observer (for persistence / park / retry)
result, err := p.RunWithInput(ctx, payload, &pipeline.RunOptions{
    Observer: myObserver,
    RunID:    "run-123", // optional; generated if empty
})
```

---

## Observer and RunOptions

**RunOptions**:

```go
type RunOptions struct {
    Observer    Observer  // required for Park and Retry
    RunID       string    // optional; UUID generated if empty
    StageOffset int       // for resume: offset added to stage indices in observer calls
}
```

**Observer** interface:

```go
type Observer interface {
    BeforePipeline(ctx, runID, name string, payload interface{}) error
    AfterPipeline(ctx, runID string, result interface{}, err error) error
    BeforeStage(ctx, runID string, stageIndex int, input interface{}) error
    AfterStage(ctx, runID string, stageIndex int, input, output interface{}, stageErr error, duration time.Duration) error
}
```

- **BeforePipeline**: Called once per run; use it to create a run record (e.g. in DB).
- **AfterPipeline**: Called when the pipeline finishes (success or error).
- **BeforeStage** / **AfterStage**: Called for each stage. Use them to persist progress (e.g. stage index and output) so you can resume after a restart.

When **resuming**, run only the remaining stages and set **StageOffset** to the index of the first stage you’re running, so observer stage indices match the original pipeline.

---

## Park and resume

### Why park

- **Delay**: Run a stage later (e.g. “wait 5 minutes” with `ParkStageAfter`).
- **External event**: Persist state and resume when an external system signals (e.g. webhook, queue).
- **Retry**: On transient failure, park and re-run the same stage later (see [Retry](#retry-and-exponential-backoff)).

### ParkStage and ParkStageAfter

- **ParkStage(persist)**  
  Persists **RunState** (run ID, pipeline name, **next** stage index, input for that stage). No resume time; your job decides when to resume.

- **ParkStageAfter(delay, persist)**  
  Same as above but sets **ResumeAt = now + delay**. Persist the **ParkedRun** (including `ResumeAt`); a periodic job selects runs where `resume_at <= now` and resumes them.

Both require **Observer** and **RunID** (or generated ID). They return **ErrParked** after a successful persist; treat that as “paused”, not failure.

### Persist types

- **ParkPersist**: `func(ctx, RunState) error` — for `ParkStage`.
- **ParkPersistWithTime**: `func(ctx, ParkedRun) error` — for `ParkStageAfter` and **Retry**. `ParkedRun` has `ResumeAt`.

### Resuming

1. **Load** saved state: `RunID`, `PipelineName`, `NextStageIndex`, `InputForNextStage`.
2. **Build** the remaining pipeline:  
   `Stages: originalPipeline.Stages[saved.NextStageIndex:]`
3. **Run** with the same RunID and observer, and set **StageOffset**:

```go
remaining := &pipeline.Pipeline{
    Name:   saved.PipelineName,
    Stages: originalPipeline.Stages[saved.NextStageIndex:],
}
result, err := remaining.RunWithInput(ctx, saved.InputForNextStage, &pipeline.RunOptions{
    RunID:       saved.RunID,
    Observer:    yourObserver,
    StageOffset: saved.NextStageIndex,
})
```

The optional **observer** package provides **DBObserver**, **ParkedRunStore**, and **Resumer** for Postgres-backed park and resume.

---

## Retry and exponential backoff

### Retry

**Retry(stage, policy, persist)** wraps a stage. When the stage returns an error that the policy considers retryable, it persists a **ParkedRun** (same stage index, so the stage is re-run on resume) and returns **ErrParked**. Your resume job runs the pipeline again when due.

- **RetryPolicy**: `Backoff` (delay until resume), `ShouldRetry(err)`, optional `MaxAttempts` (enforced in your persist).
- Mark errors as retryable with **RetryableErr(err)**; filter with **IsRetryable(err)** in `ShouldRetry`.

Example:

```go
policy := pipeline.RetryPolicy{
    Backoff:     2 * time.Minute,
    ShouldRetry: pipeline.IsRetryable,
}
stage := pipeline.Retry(myStage, policy, parkedRunStore.PersistFunc())
```

### Exponential backoff

Use **ExponentialBackoffPolicy** and **ExponentialBackoffPersist** so delay = `Initial * Multiplier^attempt`, with optional **Cap** and **MaxAttempts**:

```go
policy := pipeline.ExponentialBackoffPolicy{
    Initial:     time.Second,
    Multiplier:  2,
    Cap:         5 * time.Minute,
    MaxAttempts: 5,
    ShouldRetry: pipeline.IsRetryable,
}
store := pipeline.NewMemoryAttemptStore() // or DB-backed AttemptStore
persist := pipeline.ExponentialBackoffPersist(policy, store, basePersist)
retryPolicy := pipeline.RetryPolicyFromExponential(policy)
stage := pipeline.Retry(myStage, retryPolicy, persist)
```

**AttemptStore** must track attempt count per `run_id`; **MemoryAttemptStore** is for tests or single-process use.

---

## Sequences

A **Sequence** runs several pipelines one after another. Each pipeline receives the **same** payload (and semantics); the result of the sequence is the last pipeline’s result.

```go
seq := &pipeline.Sequence{
    Name:      "ingest-then-notify",
    Pipelines: []*pipeline.Pipeline{p1, p2, p3},
}
result, err := seq.Run(ctx, payload, opts)
```

With an observer, stage indices are **global** across all pipelines (0, 1, 2, … for all stages in order).

---

## Stdlib stages

| Stage | Purpose |
|-------|--------|
| **Identity()** | Pass-through; no-op. |
| **Tap(fn)** | Call `fn(ctx, input)` then pass input through. |
| **Validate\[T\](predicate, errMsg)** | Pass through only if `predicate(v)`; else error. |
| **Constant(value)** | Ignore input; always output `value`. |
| **WithTimeout(inner, timeout)** | Run inner with a deadline. |
| **MapSlice(convert)** | `[]T` → `[]U` via `ConvertFunc[T,U]` per element. |
| **FilterSlice(keep)** | Keep elements where `keep(v)` is true. |

See **pipeline/stages.go** and **pipeline/stages_test.go** for details.

---

## Human-readable config (config package)

The optional **config** module provides a **stage registry** and **YAML pipeline config** so you can define pipelines by stage name and options instead of Go code:

```yaml
name: my-pipeline
stages:
  - fetch
  - name: parse
    retry: exponential
    timeout: 60s
    initial: 5s
    max_attempts: 5
  - validate
```

- Register stages with `registry.Register("fetch", fetchStage)` etc.
- Parse YAML with `config.ParsePipelineConfig(data)`.
- Build with `config.BuildPipeline(registry, cfg, opts)`; set `opts.RetryPersist` when any stage has `retry`.

See **config/README.md** and **config/example.yaml**.

---

## Complete examples

### Minimal pipeline (no observer)

```go
p := &pipeline.Pipeline{
    Name:   "echo",
    Stages: []pipeline.Stage{pipeline.Identity()},
}
out, err := p.RunWithInput(ctx, 42, nil)
// out == 42
```

### Pipeline with observer

```go
obs := myObserver // implements pipeline.Observer
opts := &pipeline.RunOptions{Observer: obs}
result, err := p.RunWithInput(ctx, payload, opts)
```

### Park and resume (with observer package)

```go
// Run
store := observer.NewParkedRunStore(queries)
pl := &pipeline.Pipeline{
    Name:   "workflow",
    Stages: []pipeline.Stage{stage1, pipeline.ParkStageAfter(5*time.Minute, store.PersistFunc()), stage2},
}
_, err := pl.RunWithInput(ctx, input, &pipeline.RunOptions{Observer: obs})
if err != nil && pipeline.IsParked(err) {
    // Run parked; resume job will continue later
}

// Resume (e.g. in a cron job)
resumer := observer.NewResumer(queries, pipelineLookup)
_ = resumer.RunDue(ctx, obs)
```

### Retry with exponential backoff

```go
policy := pipeline.ExponentialBackoffPolicy{
    Initial:     time.Second,
    Multiplier:  2,
    Cap:         time.Minute,
    MaxAttempts: 5,
    ShouldRetry: pipeline.IsRetryable,
}
attemptStore := pipeline.NewMemoryAttemptStore()
persist := pipeline.ExponentialBackoffPersist(policy, attemptStore, observerStore.PersistFunc())
stage := pipeline.Retry(flakyStage, pipeline.RetryPolicyFromExponential(policy), persist)
```

### Resume after shutdown

Persist run state in your Observer (e.g. in **AfterStage**). On startup, load incomplete runs and for each:

```go
remaining := &pipeline.Pipeline{
    Name:   saved.PipelineName,
    Stages: fullPipeline.Stages[saved.NextStageIndex:],
}
result, err := remaining.RunWithInput(ctx, saved.InputForNextStage, &pipeline.RunOptions{
    RunID:       saved.RunID,
    Observer:    obs,
    StageOffset: saved.NextStageIndex,
})
```

---

## Testing

- **pipeline_test.go**: Pipeline, Sequence, observer hooks, StageOffset, park, retry, resume.
- **stages_test.go**: Identity, Tap, Validate, Constant, WithTimeout, MapSlice, FilterSlice.
- **retry_test.go**: ExponentialBackoffPersist, cap, MaxAttempts, MemoryAttemptStore.
- **example_test.go**: Example pipelines (ls/grep, transform).
- **examples/with-db**: Full example with DBObserver, ParkStageAfter, Resumer, and testcontainers Postgres.

Run all tests:

```bash
go test ./...
```
