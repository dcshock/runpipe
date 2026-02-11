package observer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dcshock/runpipe/observer/repository"
	"github.com/dcshock/runpipe/pipeline"
	"github.com/jackc/pgx/v5/pgtype"
)

// DBObserver persists pipeline and stage execution to Postgres (pipeline_run,
// pipeline_run_stage) so runs can be monitored and resumed.
type DBObserver struct {
	queries *repository.Queries
}

// NewDBObserver returns an Observer that writes to the given Queries (e.g. from
// repository.New(db)).
func NewDBObserver(queries *repository.Queries) *DBObserver {
	return &DBObserver{queries: queries}
}

// BeforePipeline implements pipeline.Observer. Inserts or updates a pipeline_run row with status 'running'.
// Uses upsert so the same run can be observed when resuming (same run_id).
func (o *DBObserver) BeforePipeline(ctx context.Context, runID, name string, payload interface{}) error {
	payloadJSON, err := marshalOptional(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	return o.queries.UpsertPipelineRun(ctx, repository.UpsertPipelineRunParams{
		RunID:   runID,
		Name:    name,
		Payload: payloadJSON,
	})
}

// AfterPipeline implements pipeline.Observer. Updates pipeline_run with status (success/failed), result, and error.
func (o *DBObserver) AfterPipeline(ctx context.Context, runID string, result interface{}, err error) error {
	status := "success"
	if err != nil {
		if pipeline.IsParked(err) {
			status = "parked"
		} else {
			status = "failed"
		}
	}
	resultJSON, _ := marshalOptional(result)
	errText := pgtype.Text{}
	if err != nil {
		errText.String = err.Error()
		errText.Valid = true
	}
	return o.queries.UpdatePipelineRunComplete(ctx, repository.UpdatePipelineRunCompleteParams{
		RunID:   runID,
		Status:  status,
		Result:  resultJSON,
		Error:   errText,
	})
}

// BeforeStage implements pipeline.Observer. Inserts a pipeline_run_stage row with status 'running'.
func (o *DBObserver) BeforeStage(ctx context.Context, runID string, stageIndex int, input interface{}) error {
	inputJSON, err := marshalOptional(input)
	if err != nil {
		return fmt.Errorf("marshal stage input: %w", err)
	}
	return o.queries.InsertPipelineRunStage(ctx, repository.InsertPipelineRunStageParams{
		PipelineRunID: runID,
		StageIndex:    int32(stageIndex),
		InputJson:     inputJSON,
	})
}

// AfterStage implements pipeline.Observer. Updates pipeline_run_stage with output, status, error, duration.
func (o *DBObserver) AfterStage(ctx context.Context, runID string, stageIndex int, input, output interface{}, stageErr error, duration time.Duration) error {
	status := "success"
	if stageErr != nil {
		if pipeline.IsParked(stageErr) {
			status = "parked"
		} else {
			status = "failed"
		}
	}
	outputJSON, _ := marshalOptional(output)
	errText := pgtype.Text{}
	if stageErr != nil {
		errText.String = stageErr.Error()
		errText.Valid = true
	}
	durationMs := pgtype.Int8{}
	durationMs.Int64 = duration.Milliseconds()
	durationMs.Valid = true
	return o.queries.UpdatePipelineRunStage(ctx, repository.UpdatePipelineRunStageParams{
		PipelineRunID: runID,
		StageIndex:    int32(stageIndex),
		OutputJson:    outputJSON,
		Status:        status,
		Error:         errText,
		DurationMs:    durationMs,
	})
}

func marshalOptional(v interface{}) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return json.Marshal(v)
}
