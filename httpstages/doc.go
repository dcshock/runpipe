// Package httpstages provides pipeline stages for HTTP requests and response handling.
//
// Use Get or Fetch to perform a GET request, ParseJSON to unmarshal the response body,
// and Expect to verify the parsed result and error if not as expected.
//
// Example pipeline: GET url → ParseJSON → Expect(predicate)
//
//	p := &pipeline.Pipeline{
//	    Name: "check-api",
//	    Stages: []pipeline.Stage{
//	        httpstages.Get(nil, "https://api.example.com/status"),
//	        httpstages.ParseJSON(),
//	        httpstages.Expect(func(v interface{}) error {
//	            m, _ := v.(map[string]interface{})
//	            if m["status"] != "ok" { return fmt.Errorf("unexpected status") }
//	            return nil
//	        }),
//	    },
//	}
package httpstages
