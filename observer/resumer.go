package observer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dcshock/runpipe/observer/repository"
	"github.com/dcshock/runpipe/pipeline"
	"github.com/jackc/pgx/v5/pgtype"
)

// PipelineLookup returns the pipeline for the given name, or nil if not found.
// The caller must register pipelines by name so the resumer can run remaining stages.
type PipelineLookup func(name string) *pipeline.Pipeline

// Resumer queries for parked runs due for resume and runs them (remaining stages) with the given observer.
type Resumer struct {
	queries *repository.Queries
	lookup  PipelineLookup
}

// NewResumer returns a resumer that uses the given Queries and pipeline lookup.
func NewResumer(queries *repository.Queries, lookup PipelineLookup) *Resumer {
	return &Resumer{queries: queries, lookup: lookup}
}

// RunDue finds all parked runs with resume_at <= now(), runs each pipeline from the saved stage, then
// removes the parked row on success (or when the run completes without parking again). Uses the
// given observer for the resumed run. If a pipeline is not found by name, that run is skipped and
// the parked row is left for inspection. All due runs are attempted; the last error is returned if any.
// For single-process use only; with multiple pods use RunDueWithClaim so each run is processed by one pod.
func (r *Resumer) RunDue(ctx context.Context, obs pipeline.Observer) error {
	due, err := r.queries.GetPipelineParkedRunsDueForResume(ctx)
	if err != nil {
		return fmt.Errorf("get parked runs due: %w", err)
	}
	var lastErr error
	for _, row := range due {
		if err := r.resumeOneRow(ctx, runRow{runID: row.RunID, pipelineName: row.PipelineName, nextStageIndex: row.NextStageIndex, inputForNextStage: row.InputForNextStage}, obs); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// RunDueWithClaim claims up to limit due parked runs (using FOR UPDATE SKIP LOCKED) so that only one
// resumer processes each run. Use a unique claimID per pod (e.g. os.Hostname(), or pod name in k8s).
// Stale claims (older than 5 minutes) are treated as unclaimed. Use this when running multiple resumer
// pods to avoid concurrent execution of the same run.
func (r *Resumer) RunDueWithClaim(ctx context.Context, obs pipeline.Observer, claimID string, limit int) error {
	if limit <= 0 {
		limit = 20
	}
	claimed, err := r.queries.ClaimAndGetPipelineParkedRunsDue(ctx, repository.ClaimAndGetPipelineParkedRunsDueParams{
		ClaimedBy: pgtype.Text{String: claimID, Valid: true},
		Limit:     int32(limit),
	})
	if err != nil {
		return fmt.Errorf("claim parked runs due: %w", err)
	}
	var lastErr error
	for _, row := range claimed {
		if err := r.resumeOneRow(ctx, runRow{runID: row.RunID, pipelineName: row.PipelineName, nextStageIndex: row.NextStageIndex, inputForNextStage: row.InputForNextStage}, obs); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

type runRow struct {
	runID             string
	pipelineName      string
	nextStageIndex    int32
	inputForNextStage []byte
}

func (r *Resumer) resumeOneRow(ctx context.Context, row runRow, obs pipeline.Observer) error {
	pl := r.lookup(row.pipelineName)
	if pl == nil {
		return fmt.Errorf("pipeline %q not found for run_id %s", row.pipelineName, row.runID)
	}
	if int(row.nextStageIndex) >= len(pl.Stages) {
		_ = r.queries.DeletePipelineParkedRun(ctx, row.runID)
		return nil
	}
	var input interface{}
	if len(row.inputForNextStage) > 0 {
		if err := json.Unmarshal(row.inputForNextStage, &input); err != nil {
			return fmt.Errorf("unmarshal input for run_id %s: %w", row.runID, err)
		}
	}
	remaining := &pipeline.Pipeline{
		Name:   pl.Name,
		Stages: pl.Stages[row.nextStageIndex:],
	}
	opts := &pipeline.RunOptions{
		RunID:       row.runID,
		Observer:    obs,
		StageOffset: int(row.nextStageIndex),
	}
	_, err := remaining.RunWithInput(ctx, input, opts)
	if err != nil && pipeline.IsParked(err) {
		return nil
	}
	if err == nil {
		return r.queries.DeletePipelineParkedRun(ctx, row.runID)
	}
	return err
}
