package pipeline

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"
)

func TestIdentity(t *testing.T) {
	ctx := context.Background()
	stage := Identity()

	// Comparable inputs
	for _, in := range []interface{}{nil, 42, "hello"} {
		out, err := stage(ctx, in)
		if err != nil {
			t.Errorf("Identity(%v): err = %v", in, err)
		}
		if out != in {
			t.Errorf("Identity(%v): got %v", in, out)
		}
	}
	// Slice: same value passed through
	slice := []int{1, 2, 3}
	out, err := stage(ctx, slice)
	if err != nil {
		t.Errorf("Identity(slice): err = %v", err)
	}
	if !reflect.DeepEqual(out, slice) {
		t.Errorf("Identity(slice): got %v", out)
	}
}

func TestTap(t *testing.T) {
	ctx := context.Background()
	var seenCtx context.Context
	var seenInput interface{}
	stage := Tap(func(c context.Context, v interface{}) {
		seenCtx = c
		seenInput = v
	})

	input := "tapped"
	out, err := stage(ctx, input)
	if err != nil {
		t.Fatalf("Tap: err = %v", err)
	}
	if seenCtx != ctx || seenInput != input {
		t.Errorf("Tap: fn called with ctx=%v input=%v", seenCtx, seenInput)
	}
	if out != input {
		t.Errorf("Tap: want output %v, got %v", input, out)
	}
}

func TestValidate_Pass(t *testing.T) {
	ctx := context.Background()
	stage := Validate[int](func(n int) bool { return n > 0 }, "must be positive")

	out, err := stage(ctx, 42)
	if err != nil {
		t.Errorf("Validate(42): err = %v", err)
	}
	if out != 42 {
		t.Errorf("Validate(42): got %v", out)
	}
}

func TestValidate_Fail(t *testing.T) {
	ctx := context.Background()
	stage := Validate[int](func(n int) bool { return n > 0 }, "must be positive")

	_, err := stage(ctx, 0)
	if err == nil {
		t.Fatal("Validate(0): expected error")
	}
	if err.Error() != "must be positive" {
		t.Errorf("Validate(0): got %q", err.Error())
	}
}

func TestValidate_WrongType(t *testing.T) {
	ctx := context.Background()
	stage := Validate[int](func(n int) bool { return true }, "")

	_, err := stage(ctx, "not an int")
	if err == nil {
		t.Fatal("Validate(string): expected error")
	}
	if err.Error() == "" {
		t.Errorf("Validate(string): expected non-empty error, got %q", err.Error())
	}
}

func TestValidate_DefaultErrMsg(t *testing.T) {
	ctx := context.Background()
	stage := Validate[int](func(n int) bool { return false }, "")

	_, err := stage(ctx, 1)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "validation failed" {
		t.Errorf("got %q", err.Error())
	}
}

func TestConstant(t *testing.T) {
	ctx := context.Background()
	val := "fixed"
	stage := Constant(val)

	for _, in := range []interface{}{nil, 0, "ignored"} {
		out, err := stage(ctx, in)
		if err != nil {
			t.Errorf("Constant(%v): err = %v", in, err)
		}
		if out != val {
			t.Errorf("Constant(%v): got %v", in, out)
		}
	}
}

func TestWithTimeout_Completes(t *testing.T) {
	ctx := context.Background()
	inner := Transform(func(ctx context.Context, n int) (int, error) { return n * 2, nil })
	stage := WithTimeout(inner, time.Second)

	out, err := stage(ctx, 21)
	if err != nil {
		t.Fatalf("WithTimeout: err = %v", err)
	}
	if out != 42 {
		t.Errorf("WithTimeout: got %v", out)
	}
}

func TestWithTimeout_Exceeded(t *testing.T) {
	ctx := context.Background()
	inner := func(ctx context.Context, input interface{}) (interface{}, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	stage := WithTimeout(Stage(inner), 10*time.Millisecond)

	_, err := stage(ctx, nil)
	if err == nil {
		t.Fatal("WithTimeout: expected deadline exceeded")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("WithTimeout: got %v", err)
	}
}

func TestMapSlice(t *testing.T) {
	ctx := context.Background()
	convert := func(ctx context.Context, n int) (string, error) {
		return fmt.Sprintf("%d", n), nil
	}
	stage := MapSlice(convert)

	input := []int{1, 2, 3}
	out, err := stage(ctx, input)
	if err != nil {
		t.Fatalf("MapSlice: err = %v", err)
	}
	got, ok := out.([]string)
	if !ok {
		t.Fatalf("MapSlice: got %T", out)
	}
	want := []string{"1", "2", "3"}
	if len(got) != len(want) {
		t.Fatalf("MapSlice: len %d want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("MapSlice: [%d] got %q want %q", i, got[i], want[i])
		}
	}
}

func TestMapSlice_WrongType(t *testing.T) {
	ctx := context.Background()
	stage := MapSlice(func(ctx context.Context, n int) (string, error) { return "", nil })

	_, err := stage(ctx, "not a slice")
	if err == nil {
		t.Fatal("MapSlice(string): expected error")
	}
}

func TestMapSlice_ConvertError(t *testing.T) {
	ctx := context.Background()
	convertErr := errors.New("convert failed")
	stage := MapSlice(func(ctx context.Context, n int) (string, error) {
		if n == 2 {
			return "", convertErr
		}
		return fmt.Sprintf("%d", n), nil
	})

	_, err := stage(ctx, []int{1, 2, 3})
	if err == nil {
		t.Fatal("MapSlice: expected error")
	}
	if !errors.Is(err, convertErr) {
		t.Errorf("MapSlice: got %v", err)
	}
}

func TestFilterSlice(t *testing.T) {
	ctx := context.Background()
	stage := FilterSlice[int](func(n int) bool { return n%2 == 0 })

	input := []int{1, 2, 3, 4, 5}
	out, err := stage(ctx, input)
	if err != nil {
		t.Fatalf("FilterSlice: err = %v", err)
	}
	got, ok := out.([]int)
	if !ok {
		t.Fatalf("FilterSlice: got %T", out)
	}
	want := []int{2, 4}
	if len(got) != len(want) {
		t.Fatalf("FilterSlice: len %d want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("FilterSlice: [%d] got %v want %v", i, got[i], want[i])
		}
	}
}

func TestFilterSlice_EmptyResult(t *testing.T) {
	ctx := context.Background()
	stage := FilterSlice[int](func(n int) bool { return n > 100 })

	out, err := stage(ctx, []int{1, 2, 3})
	if err != nil {
		t.Fatalf("FilterSlice: err = %v", err)
	}
	got := out.([]int)
	if len(got) != 0 {
		t.Errorf("FilterSlice: got %v", got)
	}
}

func TestFilterSlice_WrongType(t *testing.T) {
	ctx := context.Background()
	stage := FilterSlice[int](func(n int) bool { return true })

	_, err := stage(ctx, []string{"a"})
	if err == nil {
		t.Fatal("FilterSlice([]string): expected error")
	}
}
