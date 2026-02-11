package httpstages

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGet(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer ts.Close()

	stage := Get(nil, ts.URL)
	out, err := stage(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	body, ok := out.([]byte)
	if !ok {
		t.Fatalf("expected []byte, got %T", out)
	}
	if string(body) != `{"status":"ok"}` {
		t.Errorf("body: got %q", body)
	}
}

func TestGet_Non2xx(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	stage := Get(nil, ts.URL)
	_, err := stage(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for 404")
	}
}

func TestFetch(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("body"))
	}))
	defer ts.Close()

	stage := Fetch(nil)
	out, err := stage(context.Background(), ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	body, ok := out.([]byte)
	if !ok {
		t.Fatalf("expected []byte, got %T", out)
	}
	if string(body) != "body" {
		t.Errorf("body: got %q", body)
	}
}

func TestFetch_InputNotString(t *testing.T) {
	stage := Fetch(nil)
	_, err := stage(context.Background(), 123)
	if err == nil {
		t.Fatal("expected error for non-string input")
	}
}
