package pipeline

import (
	"context"
	"testing"
	"time"
)

func TestExponentialBackoffPersist_DelayIncreases(t *testing.T) {
	store := NewMemoryAttemptStore()
	var persisted []ParkedRun
	base := func(ctx context.Context, parked ParkedRun) error {
		persisted = append(persisted, parked)
		return nil
	}

	policy := ExponentialBackoffPolicy{
		Initial:    10 * time.Millisecond,
		Multiplier: 2,
		Cap:        0,
		MaxAttempts: 0,
	}
	persist := ExponentialBackoffPersist(policy, store, base)

	ctx := context.Background()
	runID := "run-1"

	// First park: attempt 0 -> delay = 10ms
	parked1 := ParkedRun{RunState: RunState{RunID: runID}}
	if err := persist(ctx, parked1); err != nil {
		t.Fatal(err)
	}
	if len(persisted) != 1 {
		t.Fatalf("expected 1 persisted, got %d", len(persisted))
	}
	delay1 := persisted[0].ResumeAt.Sub(time.Now())
	if delay1 < 8*time.Millisecond || delay1 > 15*time.Millisecond {
		t.Errorf("first delay expected ~10ms, got %v", delay1)
	}

	// Second park: attempt 1 -> delay = 20ms
	parked2 := ParkedRun{RunState: RunState{RunID: runID}}
	if err := persist(ctx, parked2); err != nil {
		t.Fatal(err)
	}
	delay2 := persisted[1].ResumeAt.Sub(time.Now())
	if delay2 < 18*time.Millisecond || delay2 > 25*time.Millisecond {
		t.Errorf("second delay expected ~20ms, got %v", delay2)
	}

	// Third park: attempt 2 -> delay = 40ms
	parked3 := ParkedRun{RunState: RunState{RunID: runID}}
	if err := persist(ctx, parked3); err != nil {
		t.Fatal(err)
	}
	delay3 := persisted[2].ResumeAt.Sub(time.Now())
	if delay3 < 35*time.Millisecond || delay3 > 45*time.Millisecond {
		t.Errorf("third delay expected ~40ms, got %v", delay3)
	}
}

func TestExponentialBackoffPersist_Cap(t *testing.T) {
	store := NewMemoryAttemptStore()
	var lastResumeAt time.Time
	base := func(ctx context.Context, parked ParkedRun) error {
		lastResumeAt = parked.ResumeAt
		return nil
	}

	policy := ExponentialBackoffPolicy{
		Initial:    10 * time.Millisecond,
		Multiplier: 2,
		Cap:        25 * time.Millisecond,
		MaxAttempts: 0,
	}
	persist := ExponentialBackoffPersist(policy, store, base)
	ctx := context.Background()
	runID := "run-cap"

	// attempt 0: 10ms, 1: 20ms, 2: 40ms -> capped at 25ms
	for i := 0; i < 3; i++ {
		if err := persist(ctx, ParkedRun{RunState: RunState{RunID: runID}}); err != nil {
			t.Fatal(err)
		}
	}
	delay := lastResumeAt.Sub(time.Now())
	if delay > 30*time.Millisecond {
		t.Errorf("delay should be capped at 25ms, got %v", delay)
	}
}

func TestExponentialBackoffPersist_MaxAttempts(t *testing.T) {
	store := NewMemoryAttemptStore()
	base := func(ctx context.Context, parked ParkedRun) error { return nil }

	policy := ExponentialBackoffPolicy{
		Initial:     10 * time.Millisecond,
		Multiplier:  2,
		MaxAttempts: 2,
	}
	persist := ExponentialBackoffPersist(policy, store, base)
	ctx := context.Background()
	runID := "run-max"

	// attempt 0 and 1 succeed
	if err := persist(ctx, ParkedRun{RunState: RunState{RunID: runID}}); err != nil {
		t.Fatal(err)
	}
	if err := persist(ctx, ParkedRun{RunState: RunState{RunID: runID}}); err != nil {
		t.Fatal(err)
	}
	// attempt 2 >= MaxAttempts (2) -> error
	if err := persist(ctx, ParkedRun{RunState: RunState{RunID: runID}}); err == nil {
		t.Fatal("expected error when attempt >= MaxAttempts")
	}
}

func TestRetryPolicyFromExponential(t *testing.T) {
	policy := ExponentialBackoffPolicy{
		Initial:    time.Second,
		ShouldRetry: func(err error) bool { return IsRetryable(err) },
	}
	rp := RetryPolicyFromExponential(policy)
	if rp.Backoff != time.Second {
		t.Errorf("Backoff want 1s, got %v", rp.Backoff)
	}
	if rp.ShouldRetry == nil {
		t.Error("ShouldRetry should be set")
	}
}

func TestMemoryAttemptStore(t *testing.T) {
	store := NewMemoryAttemptStore()
	ctx := context.Background()

	n, err := store.GetAttempt(ctx, "x")
	if err != nil || n != 0 {
		t.Fatalf("GetAttempt fresh: got %d, %v", n, err)
	}
	if err := store.SetAttempt(ctx, "x", 3); err != nil {
		t.Fatal(err)
	}
	n, err = store.GetAttempt(ctx, "x")
	if err != nil || n != 3 {
		t.Fatalf("GetAttempt after set: got %d, %v", n, err)
	}
}
