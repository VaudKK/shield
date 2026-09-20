package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/VaudKK/shield/backend/internal/auth"
	"github.com/VaudKK/shield/backend/internal/ratelimit"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestRequireAuth_NoCookieRejected(t *testing.T) {
	// SessionUser returns early for an empty token without touching the
	// database, so a nil-backed service is safe to use here.
	s := &Server{Auth: auth.NewService(nil, nil)}

	req := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	rec := httptest.NewRecorder()

	s.requireAuth(okHandler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestRequireCSRF(t *testing.T) {
	tests := []struct {
		name       string
		cookie     string
		header     string
		wantStatus int
	}{
		{"missing both", "", "", http.StatusForbidden},
		{"missing header", "token-a", "", http.StatusForbidden},
		{"mismatched", "token-a", "token-b", http.StatusForbidden},
		{"matching", "token-a", "token-a", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/v1/auth/logout", nil)
			if tt.cookie != "" {
				req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: tt.cookie})
			}
			if tt.header != "" {
				req.Header.Set("X-CSRF-Token", tt.header)
			}
			rec := httptest.NewRecorder()

			requireCSRF(okHandler()).ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, rec.Code)
			}
		})
	}
}

type stubLimiter struct {
	allow bool
}

func (s stubLimiter) Allow(context.Context, string) (bool, error) {
	return s.allow, nil
}

func TestRateLimit(t *testing.T) {
	t.Run("allowed", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
		req.RemoteAddr = "203.0.113.1:1234"
		rec := httptest.NewRecorder()

		rateLimit(stubLimiter{allow: true}, "RATE_LIMITED")(okHandler()).ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("denied", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
		req.RemoteAddr = "203.0.113.1:1234"
		rec := httptest.NewRecorder()

		rateLimit(stubLimiter{allow: false}, "RATE_LIMITED")(okHandler()).ServeHTTP(rec, req)

		if rec.Code != http.StatusTooManyRequests {
			t.Fatalf("expected 429, got %d", rec.Code)
		}
	})
}

func TestInMemoryLimiter_EnforcesBurst(t *testing.T) {
	limiter := ratelimit.NewInMemoryLimiter(1, time.Minute, 2) // 2 burst, slow refill
	defer limiter.Close()

	ctx := context.Background()
	allowedCount := 0
	for i := 0; i < 4; i++ {
		ok, err := limiter.Allow(ctx, "same-key")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ok {
			allowedCount++
		}
	}

	if allowedCount != 2 {
		t.Errorf("expected exactly 2 allowed requests within burst, got %d", allowedCount)
	}
}
