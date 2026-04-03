package retry

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"
)

type Config struct {
	MaxAttempts   int
	InitialDelay  time.Duration
	MaxDelay      time.Duration
	Multiplier    float64
	Jitter        bool
	RetryableErrs []func(error) bool
}

func DefaultConfig() *Config {
	return &Config{
		MaxAttempts:  5,
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		Jitter:       true,
		RetryableErrs: []func(error) bool{
			func(err error) bool { return strings.Contains(err.Error(), "timeout") },
			func(err error) bool { return strings.Contains(err.Error(), "429") },
			func(err error) bool { return strings.Contains(err.Error(), "503") },
			func(err error) bool { return strings.Contains(err.Error(), "connection refused") },
			func(err error) bool { return strings.Contains(err.Error(), "TLS handshake") },
			func(err error) bool { return strings.Contains(err.Error(), "context deadline exceeded") },
		},
	}
}

func Do(ctx context.Context, cfg *Config, operation func() error) error {
	var lastErr error
	delay := cfg.InitialDelay

	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return fmt.Errorf("retry cancelled: %w", ctx.Err())
		default:
		}

		err := operation()
		if err == nil {
			return nil
		}
		lastErr = err

		if !isRetryable(err, cfg.RetryableErrs) {
			return fmt.Errorf("non-retryable error after %d attempt(s): %w", attempt, err)
		}

		if attempt == cfg.MaxAttempts {
			break
		}

		sleep := delay
		if cfg.Jitter {
			jitter := time.Duration(rand.Float64()*0.4-0.2) * sleep
			sleep = sleep + jitter
			if sleep < 0 {
				sleep = 0
			}
		}

		if sleep > cfg.MaxDelay {
			sleep = cfg.MaxDelay
		}

		select {
		case <-time.After(sleep):
		case <-ctx.Done():
			return fmt.Errorf("retry cancelled during backoff: %w", ctx.Err())
		}

		delay = time.Duration(float64(delay) * cfg.Multiplier)
		if delay > cfg.MaxDelay {
			delay = cfg.MaxDelay
		}
	}

	return fmt.Errorf("failed after %d attempts: %w", cfg.MaxAttempts, lastErr)
}

func isRetryable(err error, predicates []func(error) bool) bool {
	for _, p := range predicates {
		if p(err) {
			return true
		}
	}
	return false
}
