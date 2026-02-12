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
//
// Retries and max attempts:
//
// Use DBAttemptStore with pipeline.ExponentialBackoffPersist and set
// ExponentialBackoffPolicy.MaxAttempts so retry attempt count is stored in
// pipeline_retry_attempt and runs stop retrying after the limit (e.g. across
// restarts and multiple pods).
//
// Single-run concurrency (e.g. multiple k8s pods):
//
// Use Resumer.RunDueWithClaim instead of RunDue. Pass a unique claimID per
// pod (e.g. os.Hostname() or the pod name). The implementation claims due
// rows with FOR UPDATE SKIP LOCKED so each run is processed by only one pod;
// stale claims older than 5 minutes are treated as unclaimed.

package observer
