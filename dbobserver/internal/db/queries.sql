-- name: InsertPipelineRun :exec
INSERT INTO pipeline_run (run_id, name, payload, status)
VALUES ($1, $2, $3, 'running');

-- name: UpsertPipelineRun :exec
INSERT INTO pipeline_run (run_id, name, payload, status)
VALUES ($1, $2, $3, 'running')
ON CONFLICT (run_id) DO UPDATE SET status = 'running', updated_at = now();

-- name: UpdatePipelineRunComplete :exec
UPDATE pipeline_run
SET status = $2, result = $3, error = $4, updated_at = now()
WHERE run_id = $1;

-- name: InsertPipelineRunStage :exec
INSERT INTO pipeline_run_stage (pipeline_run_id, stage_index, input_json, status)
VALUES ($1, $2, $3, 'running');

-- name: UpdatePipelineRunStage :exec
UPDATE pipeline_run_stage
SET output_json = $3, status = $4, error = $5, duration_ms = $6, updated_at = now()
WHERE pipeline_run_id = $1 AND stage_index = $2;

-- name: UpsertPipelineParkedRun :exec
INSERT INTO pipeline_parked_run (run_id, pipeline_name, next_stage_index, input_for_next_stage, resume_at, updated_at)
VALUES ($1, $2, $3, $4, $5, now())
ON CONFLICT (run_id) DO UPDATE SET
    pipeline_name = EXCLUDED.pipeline_name,
    next_stage_index = EXCLUDED.next_stage_index,
    input_for_next_stage = EXCLUDED.input_for_next_stage,
    resume_at = EXCLUDED.resume_at,
    updated_at = now();

-- name: GetPipelineParkedRunsDueForResume :many
SELECT run_id, pipeline_name, next_stage_index, input_for_next_stage, resume_at
FROM pipeline_parked_run
WHERE resume_at <= now()
ORDER BY resume_at;

-- name: DeletePipelineParkedRun :exec
DELETE FROM pipeline_parked_run WHERE run_id = $1;
