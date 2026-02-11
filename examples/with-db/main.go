// Demo runs a pipeline with DBObserver, parks, and resume.
// Requires DATABASE_URL (e.g. postgres://user:pass@localhost:5432/dbname).
// Run: go run .   or  DATABASE_URL=... go run .
package main

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/dcshock/runpipe/observer"
	"github.com/dcshock/runpipe/observer/repository"
	"github.com/dcshock/runpipe/pipeline"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migration.sql
var migrationSQL string

func main() {
	ctx := context.Background()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is required (e.g. postgres://user:pass@localhost:5432/dbname)")
	}

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	if err := runMigration(ctx, pool); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	queries := repository.New(pool)
	obs := observer.NewDBObserver(queries)
	pl := DemoPipeline(queries, 2*time.Second)
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
	if err != nil {
		if pipeline.IsParked(err) {
			fmt.Println("Pipeline parked; waiting 2s then resuming...")
			time.Sleep(2 * time.Second)
			if err := resumer.RunDue(ctx, obs); err != nil {
				log.Fatalf("resume: %v", err)
			}
			fmt.Println("Resume completed.")
			return
		}
		log.Fatalf("run: %v", err)
	}

	fmt.Printf("Result: %v\n", result)
}

func runMigration(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, migrationSQL)
	return err
}
