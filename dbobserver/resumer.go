package observer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dcshock/runpipe/dbobserver/repository"
	"github.com/dcshock/runpipe/pipeline"
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

// RunDue finds all parked runs with resume_at <= now, runs each pipeline from the saved stage, then
// removes the parked row on success (or when the run completes without parking again). Uses the
// given observer for the resumed run. If a pipeline is not found by name, that run is skipped and
// the parked row is left for inspection. All due runs are attempted; the last error is returned if any.
func (r *Resumer) RunDue(ctx context.Context, obs pipeline.Observer) error {
	due, err := r.queries.GetPipelineParkedRunsDueForResume(ctx)
	if err != nil {
		return fmt.Errorf("get parked runs due: %w", err)
	}
	var lastErr error
	for _, row := range due {
		if err := r.resumeOne(ctx, row, obs); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

func (r *Resumer) resumeOne(ctx context.Context, row repository.GetPipelineParkedRunsDueForResumeRow, obs pipeline.Observer) error {
	pl := r.lookup(row.PipelineName)
	if pl == nil {
		return fmt.Errorf("pipeline %q not found for run_id %s", row.PipelineName, row.RunID)
	}
	if int(row.NextStageIndex) >= len(pl.Stages) {
		_ = r.queries.DeletePipelineParkedRun(ctx, row.RunID)
		return nil
	}
	var input interface{}
	if len(row.InputForNextStage) > 0 {
		if err := json.Unmarshal(row.InputForNextStage, &input); err != nil {
			return fmt.Errorf("unmarshal input for run_id %s: %w", row.RunID, err)
		}
	}
	remaining := &pipeline.Pipeline{
		Name:   pl.Name,
		Stages: pl.Stages[row.NextStageIndex:],
	}
	opts := &pipeline.RunOptions{
		RunID:       row.RunID,
		Observer:    obs,
		StageOffset: int(row.NextStageIndex),
	}
	_, err := remaining.RunWithInput(ctx, input, opts)
	if err != nil && pipeline.IsParked(err) {
		return nil
	}
	if err == nil {
		return r.queries.DeletePipelineParkedRun(ctx, row.RunID)
	}
	return err
}
