package config

import (
	"fmt"
	"time"

	"github.com/dcshock/runpipe/pipeline"
	"gopkg.in/yaml.v3"
)

// BuildOptions configures how a pipeline is built from config (e.g. retry persist, source, observers).
type BuildOptions struct {
	// RetryPersist is used when a stage has retry (exponential or fixed). Required if any stage uses retry.
	RetryPersist pipeline.ParkPersistWithTime

	// RetryAttemptStore is used for exponential backoff. If nil and a stage uses retry=exponential, a new MemoryAttemptStore is used (single-process only).
	RetryAttemptStore pipeline.AttemptStore

	// SourceRegistry is used when PipelineConfig.Source is set. The built pipeline's Source is set to the registered function.
	SourceRegistry *SourceRegistry

	// ObserverRegistry is used when PipelineConfig.Observers is set. BuildObserver returns pipeline.MultiObserver of the looked-up observers.
	ObserverRegistry *ObserverRegistry
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

// BuildObserver returns a pipeline.Observer for the config's Observers list by looking up each name
// in BuildOptions.ObserverRegistry and combining them with pipeline.MultiObserver. Use it when
// running a config-built pipeline so the run uses the observers specified in YAML.
// If cfg.Observers is empty or opts.ObserverRegistry is nil, returns (nil, nil); the caller
// can pass their own observer in RunOptions. If any observer name is not registered, returns an error.
func BuildObserver(cfg *PipelineConfig, opts *BuildOptions) (pipeline.Observer, error) {
	if cfg == nil || len(cfg.Observers) == 0 || opts == nil || opts.ObserverRegistry == nil {
		return nil, nil
	}
	list := make([]pipeline.Observer, 0, len(cfg.Observers))
	for i, name := range cfg.Observers {
		obs, ok := opts.ObserverRegistry.Get(name)
		if !ok {
			return nil, fmt.Errorf("observer %d: %q not in registry", i, name)
		}
		list = append(list, obs)
	}
	return pipeline.MultiObserver(list...), nil
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

// BuildSequence builds a pipeline.Sequence from a sequence config by looking up the named pipelines
// in the built pipeline map. Each name in seq.Pipelines must exist in builtPipelines.
func BuildSequence(seq *SequenceConfig, builtPipelines map[string]*pipeline.Pipeline) (*pipeline.Sequence, error) {
	if seq == nil {
		return nil, fmt.Errorf("SequenceConfig is nil")
	}
	out := make([]*pipeline.Pipeline, 0, len(seq.Pipelines))
	for i, name := range seq.Pipelines {
		p, ok := builtPipelines[name]
		if !ok {
			return nil, fmt.Errorf("sequence %q pipeline %d: %q not in built pipelines", seq.Name, i, name)
		}
		out = append(out, p)
	}
	return &pipeline.Sequence{Name: seq.Name, Pipelines: out}, nil
}

// BuildAllSequences builds a pipeline.Sequence for each entry in multi.Sequences using the given built pipelines.
func BuildAllSequences(multi *MultiPipelineConfig, builtPipelines map[string]*pipeline.Pipeline) (map[string]*pipeline.Sequence, error) {
	if multi == nil || len(multi.Sequences) == 0 {
		return map[string]*pipeline.Sequence{}, nil
	}
	out := make(map[string]*pipeline.Sequence, len(multi.Sequences))
	for name, cfg := range multi.Sequences {
		if cfg.Name == "" {
			cfg.Name = name
		}
		seq, err := BuildSequence(&cfg, builtPipelines)
		if err != nil {
			return nil, fmt.Errorf("sequence %q: %w", name, err)
		}
		out[name] = seq
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

