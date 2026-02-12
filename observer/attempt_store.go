package observer

import (
	"context"
	"errors"

	"github.com/dcshock/runpipe/observer/repository"
	"github.com/dcshock/runpipe/pipeline"
	"github.com/jackc/pgx/v5"
)

// DBAttemptStore implements pipeline.AttemptStore using the observer's database.
// Use it with pipeline.ExponentialBackoffPersist so retry attempt count is stored in the DB
// and max attempts are enforced across restarts and multiple resumer pods.
func NewDBAttemptStore(queries *repository.Queries) *DBAttemptStore {
	return &DBAttemptStore{queries: queries}
}

// DBAttemptStore stores retry attempt count per run_id in pipeline_retry_attempt.
type DBAttemptStore struct {
	queries *repository.Queries
}

// GetAttempt implements pipeline.AttemptStore. Returns 0 if the run has no row yet.
func (s *DBAttemptStore) GetAttempt(ctx context.Context, runID string) (int, error) {
	n, err := s.queries.GetRetryAttempt(ctx, runID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil
		}
		return 0, err
	}
	return int(n), nil
}

// SetAttempt implements pipeline.AttemptStore.
func (s *DBAttemptStore) SetAttempt(ctx context.Context, runID string, attempt int) error {
	return s.queries.UpsertRetryAttempt(ctx, repository.UpsertRetryAttemptParams{
		RunID:        runID,
		AttemptCount: int32(attempt),
	})
}

// Ensure DBAttemptStore implements pipeline.AttemptStore.
var _ pipeline.AttemptStore = (*DBAttemptStore)(nil)
