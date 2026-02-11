package httpstages

import (
	"context"
	"fmt"
	"reflect"

	"github.com/dcshock/runpipe/pipeline"
)

// Expect returns a stage that runs the predicate on the input. If the predicate returns an error,
// the stage returns that error and the pipeline fails. Otherwise the input is passed through unchanged.
// Use after ParseJSON to verify the decoded result (e.g. check status field, required keys).
func Expect(predicate func(interface{}) error) pipeline.Stage {
	if predicate == nil {
		panic("httpstages.Expect: predicate must not be nil")
	}
	return func(ctx context.Context, input interface{}) (interface{}, error) {
		if err := predicate(input); err != nil {
			return nil, fmt.Errorf("expect: %w", err)
		}
		return input, nil
	}
}

// ExpectEqual returns a stage that checks the input equals expected using reflect.DeepEqual.
// Works for primitives, slices, and maps (e.g. parsed JSON).
func ExpectEqual(expected interface{}) pipeline.Stage {
	return Expect(func(v interface{}) error {
		if !reflect.DeepEqual(v, expected) {
			return fmt.Errorf("got %v, want %v", v, expected)
		}
		return nil
	})
}
