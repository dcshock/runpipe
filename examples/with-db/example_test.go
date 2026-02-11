package main

import (
	"context"
	"testing"
	"time"

	"github.com/dcshock/runpipe/observer"
	"github.com/dcshock/runpipe/observer/repository"
	"github.com/dcshock/runpipe/pipeline"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

// TestParkAndResume runs the demo pipeline against a testcontainers Postgres:
// run with observer -> park -> wait -> resume -> assert result.
func TestParkAndResume(t *testing.T) {
	ctx := context.Background()

	postgresContainer, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}
	t.Cleanup(func() {
		if err := postgresContainer.Terminate(ctx); err != nil {
			t.Logf("terminate container: %v", err)
		}
	})

	connStr, err := postgresContainer.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	if err := runMigration(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	queries := repository.New(pool)
	obs := observer.NewDBObserver(queries)
	parkDelay := 100 * time.Millisecond
	pl := DemoPipeline(queries, parkDelay)
	lookup := func(name string) *pipeline.Pipeline {
		if name == PipelineName() {
			return pl
		}
		return nil
	}
	resumer := observer.NewResumer(queries, lookup)

	input := []string{"hello", "world"}
	opts := &pipeline.RunOptions{Observer: obs}

	result, err := pl.RunWithInput(ctx, input, opts)
	if err == nil {
		t.Fatalf("expected parked error, got result %v", result)
	}
	if !pipeline.IsParked(err) {
		t.Fatalf("expected parked error, got %v", err)
	}

	time.Sleep(parkDelay + 50*time.Millisecond)

	if err := resumer.RunDue(ctx, obs); err != nil {
		t.Fatalf("resume: %v", err)
	}

	// After resume there should be no parked runs left.
	runs, err := queries.GetPipelineParkedRunsDueForResume(ctx)
	if err != nil {
		t.Fatalf("get parked: %v", err)
	}
	if len(runs) > 0 {
		t.Errorf("expected no parked runs after resume, got %d", len(runs))
	}
}
