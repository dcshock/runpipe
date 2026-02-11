package httpstages

import (
	"context"
	"testing"
)

func TestParseJSON(t *testing.T) {
	stage := ParseJSON()
	out, err := stage(context.Background(), []byte(`{"a":1,"b":"x"}`))
	if err != nil {
		t.Fatal(err)
	}
	m, ok := out.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", out)
	}
	if m["a"].(float64) != 1 || m["b"].(string) != "x" {
		t.Errorf("map: %v", m)
	}
}

func TestParseJSON_StringInput(t *testing.T) {
	stage := ParseJSON()
	out, err := stage(context.Background(), `[1,2]`)
	if err != nil {
		t.Fatal(err)
	}
	sl, ok := out.([]interface{})
	if !ok {
		t.Fatalf("expected slice, got %T", out)
	}
	if len(sl) != 2 {
		t.Errorf("len: got %d", len(sl))
	}
}

func TestParseJSON_InvalidInput(t *testing.T) {
	stage := ParseJSON()
	_, err := stage(context.Background(), 42)
	if err == nil {
		t.Fatal("expected error for non-[]byte/string input")
	}
}

func TestParseJSONTo(t *testing.T) {
	type T struct {
		A int    `json:"a"`
		B string `json:"b"`
	}
	stage := ParseJSONTo[T]()
	out, err := stage(context.Background(), []byte(`{"a":1,"b":"x"}`))
	if err != nil {
		t.Fatal(err)
	}
	ptr, ok := out.(*T)
	if !ok {
		t.Fatalf("expected *T, got %T", out)
	}
	if ptr.A != 1 || ptr.B != "x" {
		t.Errorf("got %+v", ptr)
	}
}
