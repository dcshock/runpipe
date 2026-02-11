# runpipe

Runpipe is a Go library for building **single-value pipelines**: linear chains of stages where each stage’s output is the next stage’s input. It supports optional persistence (observer), **park/resume** (pause a run and continue later), and **retry** with fixed or exponential backoff.

## Features

- **Pipeline & Stage**: Compose stages `(ctx, input) -> (output, error)` in order; run with `Run(ctx)` or `RunWithInput(ctx, payload, opts)`.
- **Observer**: Optional before/after hooks for the pipeline and each stage (persistence, logging, resume).
- **Park & Resume**: Stages can “park” a run (persist state, return `ErrParked`); a job later resumes from the next stage (or same stage for retry).
- **Retry**: Wrap a stage so retryable failures park and are re-run later; use **exponential backoff** via `ExponentialBackoffPolicy` and `ExponentialBackoffPersist`.
- **Sequence**: Run multiple pipelines in order with the same payload (and semantics).
- **Stdlib stages**: `Identity`, `Tap`, `Validate`, `Constant`, `WithTimeout`, `MapSlice`, `FilterSlice`, `Transform`.

## Installation

```bash
go get github.com/dcshock/runpipe
```

Optional packages:

```bash
go get github.com/dcshock/runpipe/observer   # Postgres observer, park, resume
go get github.com/dcshock/runpipe/config     # Stage registry + YAML pipeline config
```

## Quick example

```go
package main

import (
    "context"
    "github.com/dcshock/runpipe/pipeline"
)

func main() {
    ctx := context.Background()
    p := &pipeline.Pipeline{
        Name:   "demo",
        Stages: []pipeline.Stage{
            pipeline.Validate[int](func(n int) bool { return n > 0 }, "positive"),
            pipeline.Transform(func(ctx context.Context, n int) (int, error) {
                return n * 2, nil
            }),
        },
    }
    result, _ := p.RunWithInput(ctx, 21, nil)
    // result == 42
}
```

## Documentation

- **[Implementation guide (docs/GUIDE.md)](docs/GUIDE.md)** — How to implement pipelines, stages, observers, park/resume, retry, sequences, and stdlib stages, with examples.
- **Package docs**: [pipeline](https://pkg.go.dev/github.com/dcshock/runpipe/pipeline), [observer](https://pkg.go.dev/github.com/dcshock/runpipe/observer) (optional).

## Project layout

- **pipeline/** — Core pipeline, stages, retry, park; no DB dependency.
- **observer/** — Optional Postgres observer (DBObserver, ParkedRunStore, Resumer); uses sqlc and pgx.
- **config/** — Optional stage registry and human-readable pipeline config (YAML); stages by name with `retry: exponential`, `timeout: 60s`, etc.
- **examples/with-db/** — Example pipeline with stdlib stages, DBObserver, park, and resume; tests use testcontainers Postgres.

## Tests

```bash
go test ./...
```

Observer and example tests require Docker (testcontainers).

## License

See [LICENSE](LICENSE).
