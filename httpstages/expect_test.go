package httpstages

import (
	"context"
	"errors"
	"testing"
)

func TestExpect(t *testing.T) {
	stage := Expect(func(v interface{}) error {
		m, ok := v.(map[string]interface{})
		if !ok {
			return errors.New("not a map")
		}
		if m["status"] != "ok" {
			return errors.New("status not ok")
		}
		return nil
	})
	out, err := stage(context.Background(), map[string]interface{}{"status": "ok"})
	if err != nil {
		t.Fatal(err)
	}
	if out.(map[string]interface{})["status"] != "ok" {
		t.Error("expected input passed through")
	}
}

func TestExpect_Fail(t *testing.T) {
	stage := Expect(func(v interface{}) error {
		return errors.New("nope")
	})
	_, err := stage(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestExpectEqual(t *testing.T) {
	stage := ExpectEqual(map[string]interface{}{"a": float64(1)})
	out, err := stage(context.Background(), map[string]interface{}{"a": float64(1)})
	if err != nil {
		t.Fatal(err)
	}
	if out == nil {
		t.Error("expected input passed through")
	}
}

func TestExpectEqual_Fail(t *testing.T) {
	stage := ExpectEqual("expected")
	_, err := stage(context.Background(), "other")
	if err == nil {
		t.Fatal("expected error")
	}
}
