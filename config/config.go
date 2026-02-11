package config

import (
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
)

// PipelineConfig is the root structure for a pipeline definition (e.g. from YAML).
type PipelineConfig struct {
	Name   string     `yaml:"name"`
	Source string     `yaml:"source"` // optional: name of a source registered in BuildOptions.SourceRegistry
	Stages []StageRef `yaml:"stages"`
}

// StageRef is a single stage entry: either a plain name or name + options.
// In YAML, a stage can be written as:
//   - fetch
//   - name: parse
//     retry: exponential
//     timeout: 60s
type StageRef struct {
	Name string `yaml:"name"`

	// Retry: "exponential" | "fixed" | "" (no retry)
	Retry string `yaml:"retry"`

	// Timeout applied around the stage (e.g. "60s"). Used for retry timeout or stage timeout.
	Timeout Duration `yaml:"timeout"`

	// For retry: initial backoff ("exponential") or fixed delay ("fixed")
	Initial Duration `yaml:"initial"`

	// For exponential retry: multiplier (default 2), cap (e.g. "5m"), max attempts
	Multiplier  float64  `yaml:"multiplier"`
	Cap         Duration `yaml:"cap"`
	MaxAttempts int      `yaml:"max_attempts"`
}

// UnmarshalYAML allows a stage to be a string (stage name only) or a struct.
func (s *StageRef) UnmarshalYAML(value *yaml.Node) error {
	var nameOnly string
	if err := value.Decode(&nameOnly); err == nil {
		s.Name = nameOnly
		return nil
	}
	type raw StageRef
	return value.Decode((*raw)(s))
}

// Duration is a time.Duration that unmarshals from YAML strings (e.g. "60s", "5m").
type Duration time.Duration

// UnmarshalYAML implements yaml.Unmarshaler.
func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("duration %q: %w", s, err)
	}
	*d = Duration(parsed)
	return nil
}

// Duration returns the standard time.Duration.
func (d Duration) Duration() time.Duration { return time.Duration(d) }

// ParsePipelineConfig parses YAML bytes into a single PipelineConfig.
func ParsePipelineConfig(data []byte) (*PipelineConfig, error) {
	var cfg PipelineConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// MultiPipelineConfig is the root structure for a file that defines multiple pipelines.
// Top-level key is "pipelines"; each value is a pipeline (name + stages).
type MultiPipelineConfig struct {
	Pipelines map[string]PipelineConfig `yaml:"pipelines"`
}

// ParseMultiPipelineConfig parses YAML bytes that contain a "pipelines" map from name to pipeline config.
// Example YAML:
//
//	pipelines:
//	  ingest:
//	    name: ingest
//	    stages: [fetch, parse]
//	  notify:
//	    name: notify
//	    stages: [validate, send]
func ParseMultiPipelineConfig(data []byte) (*MultiPipelineConfig, error) {
	var cfg MultiPipelineConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
