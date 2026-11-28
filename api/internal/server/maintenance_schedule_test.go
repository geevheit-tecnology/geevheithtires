package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestScheduledMaintenanceAuthAndExecution(t *testing.T) {
	router := NewRouter(&App{})

	t.Run("rejects when CRON_SECRET is missing or short", func(t *testing.T) {
		t.Setenv("CRON_SECRET", "short")
		req := httptest.NewRequest(http.MethodPost, "/api/v1/internal/maintenance", nil)
		req.Header.Set("Authorization", "Bearer short")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("want 503, got %d", rec.Code)
		}
	})

	t.Run("rejects unauthorized request", func(t *testing.T) {
		t.Setenv("CRON_SECRET", "0123456789abcdef0123456789abcdef")
		req := httptest.NewRequest(http.MethodPost, "/api/v1/internal/maintenance", nil)
		req.Header.Set("Authorization", "Bearer wrong-token")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("want 401, got %d", rec.Code)
		}
	})

	t.Run("rejects GET method", func(t *testing.T) {
		t.Setenv("CRON_SECRET", "0123456789abcdef0123456789abcdef")
		req := httptest.NewRequest(http.MethodGet, "/api/v1/internal/maintenance", nil)
		req.Header.Set("Authorization", "Bearer 0123456789abcdef0123456789abcdef")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("want 405, got %d", rec.Code)
		}
	})

	t.Run("runs maintenance sweep with valid CRON_SECRET", func(t *testing.T) {
		t.Setenv("CRON_SECRET", "0123456789abcdef0123456789abcdef")
		prev := maintenanceRunner
		defer func() { maintenanceRunner = prev }()
		maintenanceRunner = func(a *App, ctx context.Context) (MaintenanceSweepResult, error) {
			return MaintenanceSweepResult{Companies: 2, Processed: 2, Failed: 0, DurationMs: 18}, nil
		}
		req := httptest.NewRequest(http.MethodPost, "/api/v1/internal/maintenance", nil)
		req.Header.Set("Authorization", "Bearer 0123456789abcdef0123456789abcdef")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
		}
		var body map[string]any
		if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["processed"] != float64(2) || body["status"] != "ok" {
			t.Fatalf("unexpected response body: %v", body)
		}
	})

	t.Run("returns 503 when database sweep fails", func(t *testing.T) {
		t.Setenv("CRON_SECRET", "0123456789abcdef0123456789abcdef")
		prev := maintenanceRunner
		defer func() { maintenanceRunner = prev }()
		maintenanceRunner = func(a *App, ctx context.Context) (MaintenanceSweepResult, error) {
			return MaintenanceSweepResult{}, errors.New("db unavailable")
		}
		req := httptest.NewRequest(http.MethodPost, "/api/v1/internal/maintenance", nil)
		req.Header.Set("Authorization", "Bearer 0123456789abcdef0123456789abcdef")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("want 503, got %d", rec.Code)
		}
	})
}
