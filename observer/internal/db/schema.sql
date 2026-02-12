-- Pipeline run: one row per pipeline execution (standalone or from a sequence).
CREATE TABLE pipeline_run (
    run_id   TEXT PRIMARY KEY,
    name     TEXT NOT NULL,
    payload  JSONB,
    status   TEXT NOT NULL DEFAULT 'running',  -- running | success | failed | parked
    result   JSONB,
    error    TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- One row per stage execution within a run. Updated in AfterStage.
CREATE TABLE pipeline_run_stage (
    pipeline_run_id TEXT NOT NULL REFERENCES pipeline_run(run_id) ON DELETE CASCADE,
    stage_index     INT  NOT NULL,
    input_json      JSONB,
    output_json     JSONB,
    status          TEXT NOT NULL DEFAULT 'running',  -- running | success | failed | parked
    error           TEXT,
    duration_ms     BIGINT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (pipeline_run_id, stage_index)
);

-- Parked runs: persisted by ParkStageAfter / Retry; resumed when resume_at <= now.
-- claimed_by/claimed_at: set when a resumer picks the run so only one pod processes it; cleared on re-park.
CREATE TABLE pipeline_parked_run (
    run_id              TEXT PRIMARY KEY,
    pipeline_name       TEXT NOT NULL,
    next_stage_index    INT  NOT NULL,
    input_for_next_stage JSONB NOT NULL,
    resume_at           TIMESTAMPTZ NOT NULL,
    claimed_by          TEXT,
    claimed_at          TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_pipeline_parked_run_resume_at ON pipeline_parked_run (resume_at);

-- Retry attempt count per run_id for ExponentialBackoffPersist (max attempts enforcement).
CREATE TABLE pipeline_retry_attempt (
    run_id        TEXT PRIMARY KEY,
    attempt_count INT NOT NULL DEFAULT 0,
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
