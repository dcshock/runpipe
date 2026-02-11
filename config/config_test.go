package config

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dcshock/runpipe/pipeline"
	"gopkg.in/yaml.v3"
)

func TestRegistry_RegisterGet(t *testing.T) {
	reg := NewRegistry()
	stage := pipeline.Identity()
	reg.Register("id", stage)
	s, ok := reg.Get("id")
	if !ok || s == nil {
		t.Fatal("Get(id) should return stage")
	}
	_, ok = reg.Get("missing")
	if ok {
		t.Error("Get(missing) should return false")
	}
}

func TestRegistry_MustGet_Panic(t *testing.T) {
	reg := NewRegistry()
	defer func() {
		if r := recover(); r == nil {
			t.Error("MustGet missing should panic")
		}
	}()
	reg.MustGet("nope")
}

func TestParsePipelineConfig_Simple(t *testing.T) {
	yaml := `
name: test-pipeline
stages:
  - fetch
  - parse
  - validate
`
	cfg, err := ParsePipelineConfig([]byte(yaml))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Name != "test-pipeline" {
		t.Errorf("name: got %q", cfg.Name)
	}
	if len(cfg.Stages) != 3 {
		t.Fatalf("stages: got %d", len(cfg.Stages))
	}
	if cfg.Stages[0].Name != "fetch" || cfg.Stages[1].Name != "parse" || cfg.Stages[2].Name != "validate" {
		t.Errorf("stage names: %v", cfg.Stages)
	}
}

func TestParsePipelineConfig_WithOptions(t *testing.T) {
	yaml := `
name: with-retry
stages:
  - fetch
  - name: parse
    retry: exponential
    timeout: 60s
    initial: 5s
    max_attempts: 5
  - validate
`
	cfg, err := ParsePipelineConfig([]byte(yaml))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Stages) != 3 {
		t.Fatalf("stages: got %d", len(cfg.Stages))
	}
	s1 := cfg.Stages[1]
	if s1.Name != "parse" || s1.Retry != "exponential" || s1.MaxAttempts != 5 {
		t.Errorf("stage 1: %+v", s1)
	}
	if s1.Timeout.Duration() != 60*1e9 || s1.Initial.Duration() != 5*1e9 {
		t.Errorf("timeout/initial: %v %v", s1.Timeout, s1.Initial)
	}
}

func TestBuildPipeline_NoRetry(t *testing.T) {
	reg := NewRegistry()
	double := pipeline.Transform(func(ctx context.Context, n int) (int, error) { return n * 2, nil })
	reg.Register("double", double)
	reg.Register("id", pipeline.Identity())

	cfg := &PipelineConfig{
		Name:   "math",
		Stages: []StageRef{{Name: "id"}, {Name: "double"}},
	}
	p, err := BuildPipeline(reg, cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "math" || len(p.Stages) != 2 {
		t.Fatalf("pipeline: %+v", p)
	}
	ctx := context.Background()
	out, err := p.RunWithInput(ctx, 21, nil)
	if err != nil {
		t.Fatal(err)
	}
	if out != 42 {
		t.Errorf("expected 42, got %v", out)
	}
}

func TestBuildPipeline_UnknownStage(t *testing.T) {
	reg := NewRegistry()
	reg.Register("a", pipeline.Identity())
	cfg := &PipelineConfig{Name: "x", Stages: []StageRef{{Name: "a"}, {Name: "not-registered"}}}
	_, err := BuildPipeline(reg, cfg, nil)
	if err == nil {
		t.Fatal("expected error for unknown stage")
	}
}

func TestBuildPipeline_RetryWithoutPersist(t *testing.T) {
	reg := NewRegistry()
	reg.Register("flaky", pipeline.Identity())
	cfg := &PipelineConfig{
		Name:   "x",
		Stages: []StageRef{{Name: "flaky", Retry: "exponential", Initial: Duration(100)}},
	}
	_, err := BuildPipeline(reg, cfg, nil)
	if err == nil {
		t.Fatal("expected error when retry without RetryPersist")
	}
}

func TestDuration_Unmarshal(t *testing.T) {
	data := []byte("initial: 30s")
	var s struct {
		Initial Duration `yaml:"initial"`
	}
	if err := yaml.Unmarshal(data, &s); err != nil {
		t.Fatal(err)
	}
	if s.Initial.Duration() != 30*1e9 {
		t.Errorf("got %v", s.Initial.Duration())
	}
}

func TestParseMultiPipelineConfig(t *testing.T) {
	yaml := `
pipelines:
  ingest:
    name: ingest
    stages: [fetch, parse]
  notify:
    stages: [validate, send]
`
	multi, err := ParseMultiPipelineConfig([]byte(yaml))
	if err != nil {
		t.Fatal(err)
	}
	if len(multi.Pipelines) != 2 {
		t.Fatalf("pipelines: got %d", len(multi.Pipelines))
	}
	if multi.Pipelines["ingest"].Name != "ingest" || len(multi.Pipelines["ingest"].Stages) != 2 {
		t.Errorf("ingest: %+v", multi.Pipelines["ingest"])
	}
	if multi.Pipelines["notify"].Name != "" {
		t.Errorf("notify name should be empty in raw config: %q", multi.Pipelines["notify"].Name)
	}
	if len(multi.Pipelines["notify"].Stages) != 2 {
		t.Errorf("notify stages: %v", multi.Pipelines["notify"].Stages)
	}
}

func TestBuildAllPipelines(t *testing.T) {
	reg := NewRegistry()
	reg.Register("id", pipeline.Identity())
	double := pipeline.Transform(func(ctx context.Context, n int) (int, error) { return n * 2, nil })
	reg.Register("double", double)

	yaml := `
pipelines:
  math:
    name: math
    stages: [id, double]
  copy:
    stages: [id]
`
	multi, err := ParseMultiPipelineConfig([]byte(yaml))
	if err != nil {
		t.Fatal(err)
	}
	pipelines, err := BuildAllPipelines(reg, multi, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(pipelines) != 2 {
		t.Fatalf("got %d pipelines", len(pipelines))
	}
	// "copy" had no name in YAML; BuildAllPipelines uses map key
	if p := pipelines["copy"]; p == nil || p.Name != "copy" {
		t.Errorf("copy pipeline: %+v", p)
	}
	out, err := pipelines["math"].RunWithInput(context.Background(), 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	if out != 20 {
		t.Errorf("math pipeline: expected 20, got %v", out)
	}
}

func TestBuildPipeline_WithSource(t *testing.T) {
	reg := NewRegistry()
	reg.Register("id", pipeline.Identity())
	sources := NewSourceRegistry()
	sources.Register("ten", func(ctx context.Context) (interface{}, error) { return 10, nil })

	cfg := &PipelineConfig{
		Name:   "with-source",
		Source: "ten",
		Stages: []StageRef{{Name: "id"}},
	}
	opts := &BuildOptions{SourceRegistry: sources}
	p, err := BuildPipeline(reg, cfg, opts)
	if err != nil {
		t.Fatal(err)
	}
	if p.Source == nil {
		t.Fatal("pipeline Source should be set")
	}
	out, err := p.Run(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if out != 10 {
		t.Errorf("expected 10, got %v", out)
	}
}

// TestBuildPipeline_Retry_EndToEnd builds a pipeline from config with retry, runs it with a stage
// that returns RetryableErr, and asserts the run parks and the mock persist is called with expected state.
func TestBuildPipeline_Retry_EndToEnd(t *testing.T) {
	var recorded pipeline.ParkedRun
	persist := func(ctx context.Context, p pipeline.ParkedRun) error {
		recorded = p
		return nil
	}

	reg := NewRegistry()
	flaky := func(ctx context.Context, in interface{}) (interface{}, error) {
		return nil, pipeline.RetryableErr(errors.New("transient"))
	}
	reg.Register("flaky", flaky)
	reg.Register("id", pipeline.Identity())

	cfg := &PipelineConfig{
		Name:   "retry-pipeline",
		Stages: []StageRef{{Name: "id"}, {Name: "flaky", Retry: "fixed", Initial: Duration(time.Second)}},
	}
	opts := &BuildOptions{RetryPersist: persist}
	p, err := BuildPipeline(reg, cfg, opts)
	if err != nil {
		t.Fatal(err)
	}

	obs := &mockObserver{}
	_, err = p.RunWithInput(context.Background(), 42, &pipeline.RunOptions{Observer: obs})
	if err == nil || !pipeline.IsParked(err) {
		t.Fatalf("expected ErrParked, got %v", err)
	}
	if recorded.RunID == "" {
		t.Error("ParkedRun should have RunID")
	}
	if recorded.PipelineName != "retry-pipeline" {
		t.Errorf("PipelineName: got %q", recorded.PipelineName)
	}
	// Retry parks to re-run the same stage, so NextStageIndex is the flaky stage index (1)
	if recorded.NextStageIndex != 1 {
		t.Errorf("NextStageIndex: got %d (expected 1 for re-run same stage)", recorded.NextStageIndex)
	}
	if recorded.InputForNextStage != 42 {
		t.Errorf("InputForNextStage: got %v", recorded.InputForNextStage)
	}
	if recorded.ResumeAt.IsZero() {
		t.Error("ResumeAt should be set for retry")
	}
}

// mockObserver implements pipeline.Observer so RunWithInput has RunID and can park.
type mockObserver struct{}

func (m *mockObserver) BeforePipeline(ctx context.Context, runID, name string, payload interface{}) error {
	return nil
}
func (m *mockObserver) AfterPipeline(ctx context.Context, runID string, result interface{}, err error) error {
	return nil
}
func (m *mockObserver) BeforeStage(ctx context.Context, runID string, stageIndex int, input interface{}) error {
	return nil
}
func (m *mockObserver) AfterStage(ctx context.Context, runID string, stageIndex int, input, output interface{}, stageErr error, duration time.Duration) error {
	return nil
}
