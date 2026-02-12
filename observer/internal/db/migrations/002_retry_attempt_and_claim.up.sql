-- Retry attempt tracking and single-run concurrency (claim) for parked runs.
-- Apply after 001_pipeline_tables.up.sql

-- Allow only one resumer to process a parked run at a time (e.g. across k8s pods).
ALTER TABLE pipeline_parked_run
ADD COLUMN IF NOT EXISTS claimed_by TEXT,
ADD COLUMN IF NOT EXISTS claimed_at TIMESTAMPTZ;

-- Store attempt count per run_id for max retries (ExponentialBackoffPersist).
CREATE TABLE IF NOT EXISTS pipeline_retry_attempt (
    run_id        TEXT PRIMARY KEY,
    attempt_count INT NOT NULL DEFAULT 0,
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
