// Package config provides a stage registry and human-readable pipeline configuration.
//
// Register stages by name, then define pipelines in YAML (or structs) that reference
// those names and optional modifiers (retry, timeout):
//
//	my-pipeline:
//	  name: my-pipeline
//	  stages:
//	    - fetch
//	    - name: parse
//	      retry: exponential
//	      timeout: 60s
//	      initial: 5s
//	      max_attempts: 5
//	    - validate
//
// Build a pipeline with BuildPipeline(registry, config, opts). Use BuildOptions.RetryPersist
// when any stage has retry (e.g. from observer.ParkedRunStore.PersistFunc()).
package config
