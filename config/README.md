# config

Stage registry and human-readable pipeline configuration for runpipe.

## Stage registry

Register stages by name, then reference them in pipeline configs:

```go
reg := config.NewRegistry()
reg.Register("fetch", fetchStage)
reg.Register("parse", parseStage)
reg.Register("validate", pipeline.Validate[Payload](isValid, "invalid"))
```

## Pipeline config (YAML)

Define pipelines with a list of stage names; any stage can have optional modifiers:

```yaml
name: my-pipeline
stages:
  - fetch
  - name: parse
    retry: exponential
    timeout: 60s
    initial: 5s
    max_attempts: 5
  - validate
```

- **Plain name**: `- fetch` — use the registered stage as-is.
- **Name + options**:
  - `retry: exponential` — wrap with exponential backoff retry (requires `BuildOptions.RetryPersist`).
  - `retry: fixed` — wrap with fixed backoff retry.
  - `timeout: 60s` — wrap with `WithTimeout(60s)`.
  - `initial`, `multiplier`, `cap`, `max_attempts` — for exponential retry.

Durations use Go duration strings (`60s`, `5m`, etc.).

### Optional source

You can reference a **source** by name so the built pipeline gets an initial input when you call `Run(ctx, opts)`:

```yaml
name: ingest
source: queue-poller
stages: [fetch, parse]
```

Register the source in a **SourceRegistry** and pass it in **BuildOptions.SourceRegistry**:

```go
sources := config.NewSourceRegistry()
sources.Register("queue-poller", myQueuePollerFunc)
opts := &config.BuildOptions{SourceRegistry: sources}
p, err := config.BuildPipeline(reg, cfg, opts)
// p.Source is set to myQueuePollerFunc
```

### Multi-pipeline YAML

One file can define multiple pipelines under a top-level `pipelines` map:

```yaml
pipelines:
  ingest:
    name: ingest
    stages: [fetch, parse]
  notify:
    stages: [validate, send]
```

Use **ParseMultiPipelineConfig** and **BuildAllPipelines**:

```go
multi, err := config.ParseMultiPipelineConfig(yamlBytes)
if err != nil { ... }
pipelines, err := config.BuildAllPipelines(reg, multi, opts)
if err != nil { ... }
// pipelines["ingest"], pipelines["notify"] are *pipeline.Pipeline
```

If a pipeline entry omits `name`, the map key is used as the pipeline name.

### Sequences

A **sequence** runs multiple pipelines in order with the same payload (each pipeline receives the same input). Add a top-level `sequences` map: each entry has a `name` and an ordered list of pipeline names (`pipelines`) that must exist in the `pipelines` map.

```yaml
pipelines:
  ingest:
    name: ingest
    stages: [fetch, parse]
  notify:
    name: notify
    stages: [validate, send]

sequences:
  ingest-then-notify:
    name: ingest-then-notify
    pipelines: [ingest, notify]
```

Build pipelines first, then build sequences from the built pipeline map:

```go
multi, err := config.ParseMultiPipelineConfig(yamlBytes)
if err != nil { ... }
pipelines, err := config.BuildAllPipelines(reg, multi, opts)
if err != nil { ... }
sequences, err := config.BuildAllSequences(multi, pipelines)
if err != nil { ... }
// sequences["ingest-then-notify"] is a *pipeline.Sequence
result, err := sequences["ingest-then-notify"].Run(ctx, payload, opts)
```

## Building a pipeline

```go
cfg, err := config.ParsePipelineConfig(yamlBytes)
if err != nil { ... }

opts := &config.BuildOptions{
    RetryPersist:      observerStore.PersistFunc(),  // required if any stage has retry
    RetryAttemptStore: pipeline.NewMemoryAttemptStore(), // optional for exponential
}
p, err := config.BuildPipeline(reg, cfg, opts)
if err != nil { ... }
// p is a *pipeline.Pipeline
```

See [example.yaml](example.yaml) for a full sample.

## Enabling RetryPersist

Any stage with `retry: exponential` or `retry: fixed` needs **RetryPersist** so the pipeline can park the run and resume it later. Set `BuildOptions.RetryPersist` when building.

### With the observer package (Postgres)

If you use [github.com/dcshock/runpipe/observer](https://pkg.go.dev/github.com/dcshock/runpipe/observer), create a **ParkedRunStore** from your DB queries and use its **PersistFunc**:

```go
import (
    "github.com/dcshock/runpipe/observer"
    "github.com/dcshock/runpipe/observer/repository"
)

// queries from repository.New(pool) or similar
store := observer.NewParkedRunStore(queries)
opts := &config.BuildOptions{
    RetryPersist:      store.PersistFunc(),
    RetryAttemptStore: pipeline.NewMemoryAttemptStore(), // or a DB-backed AttemptStore
}
p, err := config.BuildPipeline(reg, cfg, opts)
```

Persisted runs are stored in `pipeline_parked_run`. Your resume job (e.g. **Resumer.RunDue**) will pick them up when `resume_at <= now`. Run the pipeline with **RunOptions{Observer: obs}** and a **RunID** so retry can persist.

### Without a database (tests or single-process)

For tests or in-memory use, you can provide a persist function that stores **ParkedRun** in memory (e.g. a slice or map). The pipeline still returns **ErrParked**; a test or loop can re-run the pipeline later with the saved input. Retry only works if the pipeline is run with an **Observer** and **RunID** (or generated ID).
