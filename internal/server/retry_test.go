package server

import (
    "context"
    "errors"
    "testing"
    "time"
)

func TestRetrySuccessFirstAttempt(t *testing.T) {
    attempts := 0
    fn := func() error { attempts++; return nil }
    err := Retry(context.Background(), fn, RetryPolicy{MaxAttempts: 3, BaseDelay: 10 * time.Millisecond})
    if err != nil {
        t.Fatalf("expected nil error, got %v", err)
    }
    if attempts != 1 {
        t.Fatalf("expected 1 attempt, got %d", attempts)
    }
}

func TestRetryExceedsAttempts(t *testing.T) {
    attempts := 0
    fn := func() error { attempts++; return errors.New("fail") }
    policy := RetryPolicy{MaxAttempts: 2, BaseDelay: 1 * time.Millisecond}
    start := time.Now()
    err := Retry(context.Background(), fn, policy)
    elapsed := time.Since(start)
    if err == nil {
        t.Fatalf("expected error after retries")
    }
    if attempts != 2 {
        t.Fatalf("expected 2 attempts, got %d", attempts)
    }
    // Expect at least one delay between attempts
    if elapsed < policy.BaseDelay {
        t.Fatalf("expected at least %v elapsed, got %v", policy.BaseDelay, elapsed)
    }
}

func TestRetryContextCancel(t *testing.T) {
    attempts := 0
    fn := func() error { attempts++; return errors.New("fail") }
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
    defer cancel()
    policy := RetryPolicy{MaxAttempts: 5, BaseDelay: 10 * time.Millisecond}
    err := Retry(ctx, fn, policy)
    if err == nil {
        t.Fatalf("expected context deadline error")
    }
    if attempts != 1 {
        t.Fatalf("expected only 1 attempt before cancellation, got %d", attempts)
    }
}
