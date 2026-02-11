package observer

import (
	"context"
	"fmt"

	"github.com/dcshock/runpipe/dbobserver/internal/db/repository"
	"github.com/dcshock/runpipe/pipeline"
)

// ParkedRunStore persists and queries parked pipeline runs (for ParkStageAfter / Retry).
type ParkedRunStore struct {
	queries *repository.Queries
}

// NewParkedRunStore returns a store that uses the given Queries.
func NewParkedRunStore(queries *repository.Queries) *ParkedRunStore {
	return &ParkedRunStore{queries: queries}
}

// Save persists a ParkedRun (insert or replace by run_id). Use for ParkStageAfter and Retry persist callbacks.
func (s *ParkedRunStore) Save(ctx context.Context, parked pipeline.ParkedRun) error {
	inputJSON, err := marshalOptional(parked.InputForNextStage)
	if err != nil {
		return fmt.Errorf("marshal input_for_next_stage: %w", err)
	}
	if inputJSON == nil {
		inputJSON = []byte("null")
	}
	return s.queries.UpsertPipelineParkedRun(ctx, repository.UpsertPipelineParkedRunParams{
		RunID:             parked.RunID,
		PipelineName:      parked.PipelineName,
		NextStageIndex:    int32(parked.NextStageIndex),
		InputForNextStage: inputJSON,
		ResumeAt:          parked.ResumeAt,
	})
}

// PersistFunc returns a pipeline.ParkPersistWithTime that saves to this store. Use with ParkStageAfter or Retry.
func (s *ParkedRunStore) PersistFunc() pipeline.ParkPersistWithTime {
	return func(ctx context.Context, parked pipeline.ParkedRun) error {
		return s.Save(ctx, parked)
	}
}
