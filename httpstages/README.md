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

## Installation

```bash
go get github.com/dcshock/runpipe/httpstages
```
