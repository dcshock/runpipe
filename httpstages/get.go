package httpstages

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/dcshock/runpipe/pipeline"
)

// Get returns a stage that performs an HTTP GET to the fixed url and returns the response body as []byte.
// The pipeline context is used for the request (timeout and cancellation). If client is nil, http.DefaultClient is used.
func Get(client *http.Client, url string) pipeline.Stage {
	if client == nil {
		client = http.DefaultClient
	}
	return func(ctx context.Context, _ interface{}) (interface{}, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("http get: new request: %w", err)
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("http get %q: %w", url, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("http get %q: status %d", url, resp.StatusCode)
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("http get %q: read body: %w", url, err)
		}
		return body, nil
	}
}

// Fetch returns a stage that performs an HTTP GET to the URL from the previous stage's output.
// Input must be a string URL. Returns the response body as []byte. Uses pipeline context for the request.
// If client is nil, http.DefaultClient is used.
func Fetch(client *http.Client) pipeline.Stage {
	if client == nil {
		client = http.DefaultClient
	}
	return func(ctx context.Context, input interface{}) (interface{}, error) {
		url, ok := input.(string)
		if !ok {
			return nil, fmt.Errorf("http fetch: input must be URL string, got %T", input)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("http fetch: new request: %w", err)
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("http fetch %q: %w", url, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("http fetch %q: status %d", url, resp.StatusCode)
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("http fetch %q: read body: %w", url, err)
		}
		return body, nil
	}
}
