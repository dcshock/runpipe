# Runpipe: orientation for AI agents

This file helps AI assistants work in the runpipe repo. Read it first when editing code or answering questions.

## What this repo is

- **runpipe** is a Go library for single-value pipelines: linear stages `(ctx, input) -> (output, error)`, with optional observers, park/resume, and retry.
- **Root module**: `github.com/dcshock/runpipe` (see `go.mod`). Core types live in **pipeline/** (same module).
- **Optional modules** (each has its own `go.mod` and `replace github.com/dcshock/runpipe => ..`):
  - **config/** — Stage registry + YAML pipeline config; `BuildPipeline(reg, cfg, opts)`, `ParsePipelineConfig`, sequences, observers list.
  - **observer/** — Postgres observer, ParkedRunStore, Resumer; sqlc in `internal/db` and `repository/`.
  - **httpstages/** — Stages: Get, Fetch, ParseJSON, ParseJSONTo, Expect, ExpectEqual.
  - **examples/with-db/** — Example app using pipeline + observer; testcontainers in tests.

## Where to look

| Goal | Where |
|------|--------|
| Stage type, Pipeline, Run, RunWithInput, Observer, Park, Retry | `pipeline/pipeline.go`, `pipeline/doc.go` |
| Stdlib stages (Identity, Tap, Validate, MapSlice, …) | `pipeline/stages.go`, `pipeline/pipeline.go` (Transform) |
| MultiObserver | `pipeline/pipeline.go` (search "MultiObserver") |
| Config: Registry, PipelineConfig, BuildPipeline, sequences, observers | `config/config.go`, `config/build.go`, `config/registry.go` |
| YAML format and options | `config/README.md`, `config/example.yaml` |
| Observer DB, park, resume | `observer/` (db_observer.go, parked.go, resumer.go) |
| HTTP stages | `httpstages/get.go`, `httpstages/json.go`, `httpstages/expect.go` |
| Full implementation guide | `docs/GUIDE.md` |
| Production notes | `docs/GUIDE.md` § Production considerations |

## Conventions

- **Stage signature**: `pipeline.Stage` is `func(ctx context.Context, input interface{}) (interface{}, error)`. Stages receive the previous stage’s output (or initial payload); return value goes to the next stage or error stops the pipeline.
- **Run(ctx, opts)** and **RunWithInput(ctx, input, opts)** — `opts` can be `nil`; use `RunOptions{Observer, RunID, StageOffset}` for observer/park/resume.
- **Config**: Stages are registered by **name** in a `config.Registry`. YAML references those names; URL/predicate etc. are set in Go when registering. See `httpstages/README.md` § Using with config.
- **Tests**: Unit tests next to code (`*_test.go`). Run `go test ./...` from repo root (root module only); for config/observer/httpstages run tests from that directory or use `go test ./path/...` if the root go.mod includes the module. Observer and examples/with-db tests need Docker (testcontainers).
- **New optional module**: Add a directory with its own `go.mod` (require runpipe, replace => ..). Depend on `github.com/dcshock/runpipe` and optionally `github.com/dcshock/runpipe/pipeline` for stages.

## Adding things

- **New stdlib stage**: Add in `pipeline/stages.go` (or pipeline.go if it needs to live with core types). Add test in `pipeline/stages_test.go`. Document in `docs/GUIDE.md` § Stdlib stages.
- **New config option**: Extend structs in `config/config.go` (e.g. PipelineConfig, StageRef), handle in `config/build.go`. Update `config/README.md` and `config/example.yaml` if user-facing.
- **New stage package (like httpstages)**: New dir, own go.mod, implement `pipeline.Stage`; add README and tests. Optionally document “using with config” (register by name, YAML lists names).

## Docs to keep in sync

- **README.md** — Features, install, quick example, project layout, link to GUIDE.
- **docs/GUIDE.md** — Full implementation guide; table of contents and sections.
- **config/README.md** — Registry, YAML, retry, source, observers, sequences.
- **httpstages/README.md** — Stages, example, “Using with config”.
- **AGENTS.md** (this file) — Update layout and “where to look” when adding modules or moving code.
