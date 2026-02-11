package httpstages

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dcshock/runpipe/pipeline"
)

// TestPipeline_GET_ParseJSON_Expect runs a full pipeline: GET -> ParseJSON -> Expect (pass).
func TestPipeline_GET_ParseJSON_Expect(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok","version":1}`))
	}))
	defer ts.Close()

	p := &pipeline.Pipeline{
		Name: "http-check",
		Stages: []pipeline.Stage{
			Get(nil, ts.URL),
			ParseJSON(),
			Expect(func(v interface{}) error {
				m, ok := v.(map[string]interface{})
				if !ok {
					return fmt.Errorf("expected map")
				}
				if m["status"] != "ok" {
					return fmt.Errorf("status is %v", m["status"])
				}
				return nil
			}),
		},
	}
	out, err := p.RunWithInput(context.Background(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	m := out.(map[string]interface{})
	if m["status"] != "ok" || m["version"].(float64) != 1 {
		t.Errorf("unexpected result: %v", out)
	}
}

// TestPipeline_GET_ParseJSON_Expect_Fail verifies the pipeline errors when Expect fails.
func TestPipeline_GET_ParseJSON_Expect_Fail(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"error"}`))
	}))
	defer ts.Close()

	p := &pipeline.Pipeline{
		Name: "http-check",
		Stages: []pipeline.Stage{
			Get(nil, ts.URL),
			ParseJSON(),
			Expect(func(v interface{}) error {
				m, ok := v.(map[string]interface{})
				if !ok {
					return fmt.Errorf("expected map")
				}
				if s, _ := m["status"].(string); s != "ok" {
					return fmt.Errorf("unexpected status: %v", m["status"])
				}
				return nil
			}),
		},
	}
	_, err := p.RunWithInput(context.Background(), nil, nil)
	if err == nil {
		t.Fatal("expected pipeline to fail when Expect returns error")
	}
}
