-- Pipeline observer tables for runpipe (DBObserver, ParkedRunStore, Resumer).
-- Apply with: psql $DATABASE_URL -f dbobserver/internal/db/migrations/001_pipeline_tables.up.sql

-- Pipeline run: one row per pipeline execution (standalone or from a sequence).
CREATE TABLE IF NOT EXISTS pipeline_run (
    run_id   TEXT PRIMARY KEY,
    name     TEXT NOT NULL,
    payload  JSONB,
    status   TEXT NOT NULL DEFAULT 'running',
    result   JSONB,
    error    TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS pipeline_run_stage (
    pipeline_run_id TEXT NOT NULL REFERENCES pipeline_run(run_id) ON DELETE CASCADE,
    stage_index     INT  NOT NULL,
    input_json      JSONB,
    output_json     JSONB,
    status          TEXT NOT NULL DEFAULT 'running',
    error           TEXT,
    duration_ms     BIGINT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (pipeline_run_id, stage_index)
);

CREATE TABLE IF NOT EXISTS pipeline_parked_run (
    run_id               TEXT PRIMARY KEY,
    pipeline_name        TEXT NOT NULL,
    next_stage_index      INT  NOT NULL,
    input_for_next_stage  JSONB NOT NULL,
    resume_at             TIMESTAMPTZ NOT NULL,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_pipeline_parked_run_resume_at ON pipeline_parked_run (resume_at);
