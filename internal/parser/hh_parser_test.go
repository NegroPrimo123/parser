package parser

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"hh-parser/pkg/retry"
)

func TestGetVacancyDetails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := HHVacancyResponse{
			ID:           "123",
			Name:         "Test Vacancy",
			AlternateURL: "https://hh.ru/vacancy/123",
			Description:  "Test description",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	parser := NewHHParser()
	parser.baseURL = server.URL

	ctx := context.Background()
	vacancy, err := parser.GetVacancyDetails(ctx, "123")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if vacancy.HHID != "123" {
		t.Errorf("Expected HHID '123', got '%s'", vacancy.HHID)
	}
}

func TestRetryWithContextCancel(t *testing.T) {
	cfg := retry.DefaultConfig()
	cfg.MaxAttempts = 3
	cfg.InitialDelay = 100 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	var attempts int
	err := retry.Do(ctx, cfg, func() error {
		attempts++
		return errors.New("timeout")
	})

	if err == nil {
		t.Error("Expected error, got nil")
	}

	if attempts != 1 && attempts != 2 {
		t.Errorf("Expected 1 or 2 attempts due to timeout, got %d", attempts)
	}
}
