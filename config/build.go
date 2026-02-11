package config

import (
	"fmt"
	"time"

	"github.com/dcshock/runpipe/pipeline"
	"gopkg.in/yaml.v3"
)

// BuildOptions configures how a pipeline is built from config (e.g. retry persist, source).
type BuildOptions struct {
	// RetryPersist is used when a stage has retry (exponential or fixed). Required if any stage uses retry.
	RetryPersist pipeline.ParkPersistWithTime

	// RetryAttemptStore is used for exponential backoff. If nil and a stage uses retry=exponential, a new MemoryAttemptStore is used (single-process only).
	RetryAttemptStore pipeline.AttemptStore

	// SourceRegistry is used when PipelineConfig.Source is set. The built pipeline's Source is set to the registered function.
	SourceRegistry *SourceRegistry
}

// BuildPipeline builds a pipeline.Pipeline from config and registry. Stage names in config must be registered.
// For stages with retry, RetryPersist must be set in opts; for exponential retry, RetryAttemptStore is used (or a new in-memory store if nil).
func BuildPipeline(reg *Registry, cfg *PipelineConfig, opts *BuildOptions) (*pipeline.Pipeline, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	if len(cfg.Stages) == 0 {
		p := &pipeline.Pipeline{Name: cfg.Name, Stages: nil}
		setSource(p, cfg, opts)
		return p, nil
	}
	stages := make([]pipeline.Stage, 0, len(cfg.Stages))
	for i, ref := range cfg.Stages {
		if ref.Name == "" {
			return nil, fmt.Errorf("stage %d: name required", i)
		}
		stage, ok := reg.Get(ref.Name)
		if !ok {
			return nil, fmt.Errorf("stage %d: %q not in registry", i, ref.Name)
		}
		stage, err := wrapStage(stage, ref, opts)
		if err != nil {
			return nil, fmt.Errorf("stage %d (%q): %w", i, ref.Name, err)
		}
		stages = append(stages, stage)
	}
	p := &pipeline.Pipeline{Name: cfg.Name, Stages: stages}
	setSource(p, cfg, opts)
	return p, nil
}

func setSource(p *pipeline.Pipeline, cfg *PipelineConfig, opts *BuildOptions) {
	if cfg.Source == "" || opts == nil || opts.SourceRegistry == nil {
		return
	}
	if src, ok := opts.SourceRegistry.Get(cfg.Source); ok {
		p.Source = src
	}
}

func wrapStage(s pipeline.Stage, ref StageRef, opts *BuildOptions) (pipeline.Stage, error) {
	if ref.Timeout > 0 {
		s = pipeline.WithTimeout(s, ref.Timeout.Duration())
	}
	if ref.Retry == "" {
		return s, nil
	}
	if opts == nil || opts.RetryPersist == nil {
		return nil, fmt.Errorf("retry requires BuildOptions.RetryPersist")
	}
	initial := ref.Initial.Duration()
	if initial <= 0 {
		initial = time.Second
	}
	switch ref.Retry {
	case "fixed":
		policy := pipeline.RetryPolicy{
			Backoff:     initial,
			ShouldRetry: pipeline.IsRetryable,
		}
		return pipeline.Retry(s, policy, opts.RetryPersist), nil
	case "exponential":
		store := opts.RetryAttemptStore
		if store == nil {
			store = pipeline.NewMemoryAttemptStore()
		}
		policy := pipeline.ExponentialBackoffPolicy{
			Initial:     initial,
			Multiplier:  2,
			Cap:         ref.Cap.Duration(),
			MaxAttempts: ref.MaxAttempts,
			ShouldRetry: pipeline.IsRetryable,
		}
		if ref.Multiplier > 0 {
			policy.Multiplier = ref.Multiplier
		}
		persist := pipeline.ExponentialBackoffPersist(policy, store, opts.RetryPersist)
		retryPolicy := pipeline.RetryPolicyFromExponential(policy)
		return pipeline.Retry(s, retryPolicy, persist), nil
	default:
		return nil, fmt.Errorf("retry %q not supported (use \"fixed\" or \"exponential\")", ref.Retry)
	}
}

// BuildAllPipelines builds a pipeline.Pipeline for each entry in multi. Keys are pipeline names.
// If a pipeline config's Name is empty, the map key is used as the pipeline name.
func BuildAllPipelines(reg *Registry, multi *MultiPipelineConfig, opts *BuildOptions) (map[string]*pipeline.Pipeline, error) {
	if multi == nil {
		return nil, fmt.Errorf("MultiPipelineConfig is nil")
	}
	out := make(map[string]*pipeline.Pipeline, len(multi.Pipelines))
	for name, cfg := range multi.Pipelines {
		if cfg.Name == "" {
			cfg.Name = name
		}
		p, err := BuildPipeline(reg, &cfg, opts)
		if err != nil {
			return nil, fmt.Errorf("pipeline %q: %w", name, err)
		}
		out[name] = p
	}
	return out, nil
}

// PipelineConfigFromMap parses a single pipeline from a map (e.g. one key in a multi-pipeline YAML).
// The key is the pipeline name; the value is the stages list.
func PipelineConfigFromMap(name string, stages interface{}) (*PipelineConfig, error) {
	// Re-encode and decode so we can reuse StageRef unmarshaling
	data, err := yaml.Marshal(map[string]interface{}{"name": name, "stages": stages})
	if err != nil {
		return nil, err
	}
	return ParsePipelineConfig(data)
}

