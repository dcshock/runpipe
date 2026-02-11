// Package pipeline provides single-value pipeline and sequence types. A Pipeline
// runs stages in order (optionally with its own Source for standalone use); each
// stage's output is the next stage's input. A Sequence runs multiple pipelines
// in order with the same payload and semantics: the caller's payload is passed to every pipeline
// (payload → pipeline1, payload → pipeline2, …), not piped between pipelines.
//
// Optional pre/post hooks (Observer) let you persist run state to a DB for monitoring
// and restart: BeforePipeline (e.g. write run record), BeforeStage/AfterStage (log
// start/end, input/output, duration), AfterPipeline (log result or error). Pass
// RunOptions{Observer: myObserver} to RunWithInput or Sequence.Run.
//
// For stages that can fail transiently, wrap them with Retry(stage, policy, persist).
// Retry uses the same park/resume mechanism: on retryable error it persists a
// ParkedRun (same stage, ResumeAt = now + Backoff) and returns ErrParked; your
// resume job re-runs the stage when due. Use RetryableErr(err) and
// policy.ShouldRetry (e.g. IsRetryable) to retry only marked errors. Enforce
// MaxAttempts in your persist callback (e.g. track attempt per run_id).
//
// To pause a run for later execution (e.g. wait for an external event or a delay), use
// ParkStage(persist) or ParkStageAfter(delay, persist). The stage persists RunState (or
// ParkedRun with ResumeAt) and returns ErrParked. Treat ErrParked as "paused, not failed".
// Park only works when the pipeline is run with Observer and RunID (injected into context).
//
// When to resume: For a fixed delay (e.g. "park for 3 minutes"), use ParkStageAfter(3*time.Minute, persist).
// Persist ParkedRun to a table with a resume_at column. Run a periodic job (e.g. every minute)
// that selects parked runs where resume_at <= now, loads RunState, and resumes each via
// RunWithInput with remaining stages and StageOffset as in the Resuming section.
//
// # Resuming after shutdown
//
// With a DB-persisted Observer you can resume a pipeline mid-run after a restart.
//
//  1. Persist in your Observer: runID, pipeline name, and in AfterStage (on success)
//     the next stage index (stageIndex+1) and the output of the completed stage (that
//     output is the input to the next stage). Persist in a form you can serialize
//     (e.g. JSON for the input). On failure you may persist stageIndex and input so
//     you can retry that stage.
//
//  2. On startup (or a "resume runs" job), load incomplete runs from the DB. For each
//     run you need: RunID, PipelineName, NextStageIndex (0-based index of the first
//     stage to run), and InputForNextStage (the value to pass into that stage).
//
//  3. Resume by running only the remaining stages with the same runID and a stage
//     offset so observer indices match the original pipeline:
//
//     remaining := &Pipeline{
//       Name:   saved.PipelineName,
//       Stages: originalPipeline.Stages[saved.NextStageIndex:],
//     }
//     result, err := remaining.RunWithInput(ctx, saved.InputForNextStage, &RunOptions{
//       RunID:       saved.RunID,
//       Observer:    yourObserver,
//       StageOffset: saved.NextStageIndex,
//     })
//
// RunState is a convenience struct for the values to persist and load; use it or
// your own schema.
package pipeline
