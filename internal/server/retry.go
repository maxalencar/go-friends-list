package server

import (
    "context"
    "time"
    "log"
)

type RetryPolicy struct {
    MaxAttempts   int
    BaseDelay     time.Duration
    BackoffFactor float64 // multiplier for each subsequent delay
}

// Retry executes the provided function according to the policy.
// The function should return an error; a nil error stops retrying.
func Retry(ctx context.Context, fn func() error, p RetryPolicy) error {
    if p.MaxAttempts <= 0 {
        p.MaxAttempts = 1
    }
    delay := p.BaseDelay
    for attempt := 1; attempt <= p.MaxAttempts; attempt++ {
        if err := fn(); err != nil {
            if attempt == p.MaxAttempts {
                return err
            }
            // wait before next attempt
            select {
            case <-time.After(delay):
                // continue
            case <-ctx.Done():
                return ctx.Err()
            }
            // increase delay exponentially if factor > 1
            if p.BackoffFactor > 1 {
                delay = time.Duration(float64(delay) * p.BackoffFactor)
            }
            continue
        }
        // success
        return nil
    }
    // Should never reach here
    log.Printf("Retry exhausted without returning error")
    return nil
}
