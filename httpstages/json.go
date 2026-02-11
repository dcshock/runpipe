package httpstages

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dcshock/runpipe/pipeline"
)

// ParseJSON returns a stage that unmarshals the input from JSON into a value.
// Input must be []byte or string (response body). Output is the decoded value (e.g. map[string]interface{} for objects).
func ParseJSON() pipeline.Stage {
	return func(ctx context.Context, input interface{}) (interface{}, error) {
		var raw []byte
		switch v := input.(type) {
		case []byte:
			raw = v
		case string:
			raw = []byte(v)
		default:
			return nil, fmt.Errorf("parsejson: input must be []byte or string, got %T", input)
		}
		var out interface{}
		if err := json.Unmarshal(raw, &out); err != nil {
			return nil, fmt.Errorf("parsejson: %w", err)
		}
		return out, nil
	}
}

// ParseJSONTo returns a stage that unmarshals the input from JSON into a value of type T.
// Input must be []byte or string. Output is *T.
func ParseJSONTo[T any]() pipeline.Stage {
	return func(ctx context.Context, input interface{}) (interface{}, error) {
		var raw []byte
		switch v := input.(type) {
		case []byte:
			raw = v
		case string:
			raw = []byte(v)
		default:
			return nil, fmt.Errorf("parsejsonto: input must be []byte or string, got %T", input)
		}
		var out T
		if err := json.Unmarshal(raw, &out); err != nil {
			return nil, fmt.Errorf("parsejsonto: %w", err)
		}
		return &out, nil
	}
}
