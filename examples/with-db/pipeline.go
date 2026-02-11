// Package main defines a demo pipeline that uses stdlib stages, parks, and resumes.
package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dcshock/runpipe/observer"
	"github.com/dcshock/runpipe/observer/repository"
	"github.com/dcshock/runpipe/pipeline"
)

const pipelineName = "demo-parked-resume"

// coerceToSliceString converts []interface{} (from JSON unmarshal on resume) to []string.
// Pass-through if input is already []string.
func coerceToSliceString(ctx context.Context, input interface{}) (interface{}, error) {
	if s, ok := input.([]string); ok {
		return s, nil
	}
	slice, ok := input.([]interface{})
	if !ok {
		return nil, fmt.Errorf("coerce: expected []string or []interface{}, got %T", input)
	}
	out := make([]string, 0, len(slice))
	for i, v := range slice {
		str, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("coerce: [%d] expected string, got %T", i, v)
		}
		out = append(out, str)
	}
	return out, nil
}

// DemoPipeline returns a pipeline that:
// 1. Validates input is a non-empty []string
// 2. Taps (logs) the input
// 3. Parks for parkDelay, then resumes
// 4. Coerces input to []string (for resumed runs where JSON gives []interface{})
// 5. Maps each string to uppercase via MapSlice
func DemoPipeline(queries *repository.Queries, parkDelay time.Duration) *pipeline.Pipeline {
	store := observer.NewParkedRunStore(queries)
	return &pipeline.Pipeline{
		Name: pipelineName,
		Stages: []pipeline.Stage{
			pipeline.Validate[[]string](func(s []string) bool { return len(s) > 0 }, "input must be non-empty []string"),
			pipeline.Tap(func(ctx context.Context, v interface{}) { _ = ctx; _ = v }),
			pipeline.ParkStageAfter(parkDelay, store.PersistFunc()),
			coerceToSliceString,
			pipeline.MapSlice(func(ctx context.Context, s string) (string, error) {
				return strings.ToUpper(s), nil
			}),
		},
	}
}

// PipelineName returns the name used for Resumer lookup.
func PipelineName() string { return pipelineName }
