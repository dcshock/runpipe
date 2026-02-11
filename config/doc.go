// Package config provides a stage registry and human-readable pipeline configuration.
//
// Register stages by name, then define pipelines in YAML (or structs) that reference
// those names and optional modifiers (retry, timeout):
//
//	name: my-pipeline
//	stages:
//	  - fetch
//	  - name: parse
//	    retry: exponential
//	    timeout: 60s
//	    initial: 5s
//	    max_attempts: 5
//	  - validate
//
// Parse with ParsePipelineConfig(data) or ParseMultiPipelineConfig(data) for multiple
// pipelines. Build with BuildPipeline(registry, config, opts). When any stage has retry,
// set BuildOptions.RetryPersist (e.g. observer.NewParkedRunStore(queries).PersistFunc()
// for Postgres). See config/README.md for how to enable RetryPersist.
package config
