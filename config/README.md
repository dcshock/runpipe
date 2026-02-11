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
