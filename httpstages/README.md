# httpstages

Pipeline stages for HTTP GET requests and JSON response handling. Use with [runpipe](https://pkg.go.dev/github.com/dcshock/runpipe/pipeline).

## Stages

- **Get(client, url)** — Performs a GET to a fixed URL. Input is ignored; output is the response body `[]byte`. Uses pipeline context for timeout/cancellation. Non-2xx responses return an error.
- **Fetch(client)** — Performs a GET to the URL from the previous stage's output (input must be a string). Output is `[]byte`.
- **ParseJSON()** — Unmarshals input (`[]byte` or `string`) from JSON. Output is the decoded value (e.g. `map[string]interface{}`).
- **ParseJSONTo[T]()** — Unmarshals into a typed value `*T`.
- **Expect(predicate)** — Runs `predicate(input)`; if it returns an error, the pipeline fails. Otherwise passes input through.
- **ExpectEqual(expected)** — Fails if input is not equal to `expected` (uses `reflect.DeepEqual`).

## Example: GET → Parse JSON → Verify

```go
p := &pipeline.Pipeline{
    Name: "check-api",
    Stages: []pipeline.Stage{
        httpstages.Get(nil, "https://api.example.com/status"),
        httpstages.ParseJSON(),
        httpstages.Expect(func(v interface{}) error {
            m, ok := v.(map[string]interface{})
            if !ok {
                return fmt.Errorf("expected JSON object")
            }
            if m["status"] != "ok" {
                return fmt.Errorf("status is %v", m["status"])
            }
            return nil
        }),
    },
}
out, err := p.RunWithInput(ctx, nil, nil)
```

## Using with config (YAML)

The [config](https://pkg.go.dev/github.com/dcshock/runpipe/config) package builds pipelines from YAML by **stage name**. Register concrete httpstages (with URL and predicate set in Go) under names, then reference those names in YAML.

**1. Register stages by name**

URL and Expect predicate are fixed when you register; the registry holds the built stages.

```go
import (
    "fmt"
    "github.com/dcshock/runpipe/config"
    "github.com/dcshock/runpipe/httpstages"
)

reg := config.NewRegistry()
// Fixed URL and predicate; register under names
reg.Register("get-status", httpstages.Get(nil, "https://api.example.com/status"))
reg.Register("parse-json", httpstages.ParseJSON())
reg.Register("expect-ok", httpstages.Expect(func(v interface{}) error {
    m, ok := v.(map[string]interface{})
    if !ok {
        return fmt.Errorf("expected JSON object")
    }
    if m["status"] != "ok" {
        return fmt.Errorf("status is %v", m["status"])
    }
    return nil
}))
```

**2. Define the pipeline in YAML**

```yaml
name: check-api
stages:
  - get-status
  - parse-json
  - expect-ok
```

You can add config options (e.g. retry, timeout) to any stage; see config package docs.

**3. Build and run**

```go
cfg, err := config.ParsePipelineConfig(yamlBytes)
if err != nil { ... }
p, err := config.BuildPipeline(reg, cfg, nil)
if err != nil { ... }
out, err := p.RunWithInput(ctx, nil, nil)
```

To use different URLs or predicates, register multiple named stages (e.g. `get-status`, `get-health`, `expect-ok`, `expect-version`) and reference the right names in your YAML pipelines.

## Installation

```bash
go get github.com/dcshock/runpipe/httpstages
```
