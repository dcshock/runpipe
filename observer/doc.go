// Package observer provides pipeline.Observer implementations and parked-run
// persistence for the pipeline package.
//
//   - DBObserver: persists each pipeline run and its stages to Postgres
//     (pipeline_run, pipeline_run_stage) for monitoring and resume support.
//   - ParkedRunStore: persists ParkedRun (from ParkStageAfter / Retry) to
//     pipeline_parked_run. Use PersistFunc() with ParkStageAfter or Retry.
//   - Resumer: queries parked runs where resume_at <= now and runs their
//     remaining stages. Register pipelines by name via PipelineLookup and call
//     RunDue periodically (e.g. from a cron job).

package observer
