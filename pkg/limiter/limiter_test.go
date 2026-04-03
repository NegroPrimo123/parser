package limiter

import (
	"context"
	"testing"
	"time"
)

func TestRateLimiter_Wait(t *testing.T) {
	limiter := NewRateLimiter(10) // 10 requests per second
	
	start := time.Now()
	for i := 0; i < 10; i++ {
		if err := limiter.Wait(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	elapsed := time.Since(start)
	
	// 10 запросов должны занять минимум 1 секунду
	if elapsed < 900*time.Millisecond {
		t.Errorf("Expected at least 900ms, got %v", elapsed)
	}
}

func TestRateLimiter_WaitContextCancel(t *testing.T) {
	limiter := NewRateLimiter(1) // 1 request per second
	
	// Первый запрос проходит
	if err := limiter.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	
	// Второй запрос должен ждать
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	
	err := limiter.Wait(ctx)
	if err != context.DeadlineExceeded {
		t.Errorf("Expected deadline exceeded, got %v", err)
	}
}