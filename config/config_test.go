package config

import (
	"context"
	"testing"

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
