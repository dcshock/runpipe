package pipeline_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/provenance-io/hastra-pulse/pipeline"
)

// Example: simulate `ls` | `grep pattern`
// Payload is the "directory" (string); ls stage lists fake files; grep stage filters by pattern.

// lsStage simulates `ls dir`: input is a directory path (string), output is a list of filenames ([]string).
func lsStage(ctx context.Context, input interface{}) (interface{}, error) {
	dir, ok := input.(string)
	if !ok {
		return nil, nil // or return error
	}
	_ = dir // in a real impl, read directory
	// Simulated listing (like ls output)
	return []string{"main.go", "pipeline.go", "doc.go", "README.md", "go.sum", "go.mod"}, nil
}

// grepStage returns a stage that filters []string lines by substring (like grep pattern).
func grepStage(pattern string) pipeline.Stage {
	return func(ctx context.Context, input interface{}) (interface{}, error) {
		lines, ok := input.([]string)
		if !ok {
			return nil, nil
		}
		var out []string
		for _, line := range lines {
			if strings.Contains(line, pattern) {
				out = append(out, line)
			}
		}
		return out, nil
	}
}

func TestExampleLSGrepPipeline(t *testing.T) {
	ctx := context.Background()

	// Pipeline: payload (dir) -> ls -> grep "go"
	// Equivalent to: ls . | grep go
	p := &pipeline.Pipeline{
		Name:   "ls-grep",
		Stages: []pipeline.Stage{lsStage, grepStage("go")},
	}

	// Payload is the "directory" to list (e.g. ".")
	payload := "."
	result, err := p.RunWithInput(ctx, payload, nil)
	if err != nil {
		t.Fatalf("pipeline failed: %v", err)
	}

	files, ok := result.([]string)
	if !ok {
		t.Fatalf("expected []string, got %T", result)
	}

	// We expect only names containing "go": main.go, pipeline.go, doc.go, go.sum, go.mod
	if len(files) != 5 {
		t.Errorf("expected 5 files matching 'go', got %d: %v", len(files), files)
	}
	for _, f := range files {
		if !strings.Contains(f, "go") {
			t.Errorf("file %q should contain 'go'", f)
		}
	}
	t.Logf("ls . | grep go => %v", files)
}

// Example: stage1 | transform stage | stage2 (struct A → struct B)

// RawResult is the output of stage1.
type RawResult struct {
	Lines []string
}

// ProcessedResult is the output of the transform and input to stage2.
type ProcessedResult struct {
	Count int
	First string
}

// stage1Output returns a stage that produces RawResult (simulates stage1).
func stage1Output(ctx context.Context, input interface{}) (interface{}, error) {
	return RawResult{Lines: []string{"a", "b", "c"}}, nil
}

// transformRawToProcessed converts RawResult -> ProcessedResult.
func transformRawToProcessed(ctx context.Context, r RawResult) (ProcessedResult, error) {
	first := ""
	if len(r.Lines) > 0 {
		first = r.Lines[0]
	}
	return ProcessedResult{Count: len(r.Lines), First: first}, nil
}

// stage2Consume consumes ProcessedResult (simulates stage2).
func stage2Consume(ctx context.Context, input interface{}) (interface{}, error) {
	p, ok := input.(ProcessedResult)
	if !ok {
		return nil, nil
	}
	return p.Count + len(p.First), nil // arbitrary: return int
}

func TestExampleTransformStage(t *testing.T) {
	ctx := context.Background()

	// Pipeline: stage1 -> Transform(RawResult -> ProcessedResult) -> stage2
	p := &pipeline.Pipeline{
		Name: "with-transform",
		Stages: []pipeline.Stage{
			stage1Output,
			pipeline.Transform(transformRawToProcessed),
			stage2Consume,
		},
	}

	result, err := p.RunWithInput(ctx, nil, nil)
	if err != nil {
		t.Fatalf("pipeline failed: %v", err)
	}

	// stage2 returns count + len(first) = 3 + 1 = 4
	if n, ok := result.(int); !ok || n != 4 {
		t.Errorf("expected 4 (int), got %v (%T)", result, result)
	}
}

// mockObserver records hook calls for tests.
type mockObserver struct {
	beforePipeline []string
	afterPipeline  []string
	beforeStage    []int
	afterStage     []int
}

func (m *mockObserver) BeforePipeline(ctx context.Context, runID, name string, payload interface{}) error {
	m.beforePipeline = append(m.beforePipeline, runID+":"+name)
	return nil
}

func (m *mockObserver) AfterPipeline(ctx context.Context, runID string, result interface{}, err error) error {
	m.afterPipeline = append(m.afterPipeline, runID)
	return nil
}

func (m *mockObserver) BeforeStage(ctx context.Context, runID string, stageIndex int, input interface{}) error {
	m.beforeStage = append(m.beforeStage, stageIndex)
	return nil
}

func (m *mockObserver) AfterStage(ctx context.Context, runID string, stageIndex int, input, output interface{}, stageErr error, d time.Duration) error {
	m.afterStage = append(m.afterStage, stageIndex)
	return nil
}

func TestExampleObserverHooks(t *testing.T) {
	ctx := context.Background()
	obs := &mockObserver{}

	p := &pipeline.Pipeline{
		Name:   "observed",
		Stages: []pipeline.Stage{stage1Output, pipeline.Transform(transformRawToProcessed), stage2Consume},
	}

	result, err := p.RunWithInput(ctx, nil, &pipeline.RunOptions{Observer: obs})
	if err != nil {
		t.Fatalf("pipeline failed: %v", err)
	}

	if n, ok := result.(int); !ok || n != 4 {
		t.Errorf("expected 4, got %v", result)
	}

	// One pipeline run: BeforePipeline once, AfterPipeline once
	if len(obs.beforePipeline) != 1 || len(obs.afterPipeline) != 1 {
		t.Errorf("expected 1 before/after pipeline, got before=%d after=%d", len(obs.beforePipeline), len(obs.afterPipeline))
	}
	// Three stages: BeforeStage/AfterStage each 3 times
	if len(obs.beforeStage) != 3 || len(obs.afterStage) != 3 {
		t.Errorf("expected 3 before/after stage, got before=%d after=%d", len(obs.beforeStage), len(obs.afterStage))
	}
	if obs.beforeStage[0] != 0 || obs.beforeStage[1] != 1 || obs.beforeStage[2] != 2 {
		t.Errorf("expected stage indices 0,1,2 got %v", obs.beforeStage)
	}
}
