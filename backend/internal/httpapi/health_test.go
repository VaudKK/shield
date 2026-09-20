package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http/httptest"
	"testing"
)

func TestHandleHealth_NoDatabase(t *testing.T) {
	s := &Server{
		Logger:  slog.Default(),
		Version: "test",
	}

	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()

	s.handleHealth(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var body healthResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Status != "ok" {
		t.Errorf("expected status ok, got %q", body.Status)
	}
	if body.Version != "test" {
		t.Errorf("expected version test, got %q", body.Version)
	}
}
