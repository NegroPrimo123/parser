package retry

import (
	"context"
	"errors"
	"testing"
)

func TestRetrySuccess(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxAttempts = 3
	var attempts int
	err := Do(context.Background(), cfg, func() error {
		attempts++
		if attempts < 2 {
			return errors.New("timeout")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
}
