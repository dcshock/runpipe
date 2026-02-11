// Package pipeline: standard stages (stdlib-style) for common pipeline patterns.

package pipeline

import (
	"context"
	"fmt"
	"time"
)

// Identity returns a stage that passes the input through unchanged.
// Useful as a no-op, for observer/retry boundaries, or as a placeholder.
func Identity() Stage {
	return func(ctx context.Context, input interface{}) (interface{}, error) {
		return input, nil
	}
}

// Tap returns a stage that calls fn(ctx, input) then passes input through unchanged.
// Use for logging, metrics, or side effects without changing the value.
func Tap(fn func(context.Context, interface{})) Stage {
	return func(ctx context.Context, input interface{}) (interface{}, error) {
		fn(ctx, input)
		return input, nil
	}
}

// Validate returns a stage that passes input through only if predicate(v) is true.
// Otherwise it returns an error (or errMsg if predicate returns false).
// Input must be of type T; type assertion failure returns an error.
func Validate[T any](predicate func(T) bool, errMsg string) Stage {
	return func(ctx context.Context, input interface{}) (interface{}, error) {
		v, ok := input.(T)
		if !ok {
			var zero T
			return nil, fmt.Errorf("validate: expected %T, got %T", zero, input)
		}
		if !predicate(v) {
			if errMsg == "" {
				errMsg = "validation failed"
			}
			return nil, fmt.Errorf("%s", errMsg)
		}
		return input, nil
	}
}

// Constant returns a stage that ignores input and always outputs value.
// Useful to inject a fixed value (e.g. after a trigger or as a test source).
func Constant(value interface{}) Stage {
	return func(ctx context.Context, _ interface{}) (interface{}, error) {
		return value, nil
	}
}

// WithTimeout wraps inner so it runs with a context deadline of now+timeout.
// If inner does not return before the deadline, context.DeadlineExceeded is returned.
func WithTimeout(inner Stage, timeout time.Duration) Stage {
	return func(ctx context.Context, input interface{}) (interface{}, error) {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		return inner(ctx, input)
	}
}

// MapSlice returns a stage that converts []T to []U using convert for each element.
// Input must be []T; otherwise returns a type error.
func MapSlice[T, U any](convert ConvertFunc[T, U]) Stage {
	return func(ctx context.Context, input interface{}) (interface{}, error) {
		slice, ok := input.([]T)
		if !ok {
			var zero []T
			return nil, fmt.Errorf("mapslice: expected %T, got %T", zero, input)
		}
		out := make([]U, 0, len(slice))
		for i, v := range slice {
			u, err := convert(ctx, v)
			if err != nil {
				return nil, fmt.Errorf("mapslice[%d]: %w", i, err)
			}
			out = append(out, u)
		}
		return out, nil
	}
}

// FilterSlice returns a stage that keeps only elements of []T for which keep(v) is true.
// Input must be []T; output is the filtered []T.
func FilterSlice[T any](keep func(T) bool) Stage {
	return func(ctx context.Context, input interface{}) (interface{}, error) {
		slice, ok := input.([]T)
		if !ok {
			var zero []T
			return nil, fmt.Errorf("filterslice: expected %T, got %T", zero, input)
		}
		out := make([]T, 0, len(slice))
		for _, v := range slice {
			if keep(v) {
				out = append(out, v)
			}
		}
		return out, nil
	}
}
