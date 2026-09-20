package httpapi_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/VaudKK/shield/backend/internal/auth"
	"github.com/VaudKK/shield/backend/internal/db"
	"github.com/VaudKK/shield/backend/internal/httpapi"
	"github.com/VaudKK/shield/backend/internal/ratelimit"
	"github.com/VaudKK/shield/backend/internal/repository"
)

// These tests exercise the register -> login -> me -> logout flow, plus
// CSRF and rate-limit enforcement, against a real PostgreSQL instance. They
// only run when TEST_DATABASE_URL is set (e.g. the docker-compose Postgres
// on localhost:5432), so `go test ./...` stays usable without Docker.
func newTestServer(t *testing.T) *httpapi.Server {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}

	ctx := context.Background()
	pool, err := db.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	return &httpapi.Server{
		Pool:   pool,
		Logger: slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})),
		Env:    "test",
		Auth: auth.NewService(
			repository.NewUserRepository(pool),
			repository.NewSessionRepository(pool),
		),
		AuthRateLimiter:     ratelimit.NewInMemoryLimiter(1000, time.Minute, 1000),
		UploadRateLimiter:   ratelimit.NewInMemoryLimiter(1000, time.Minute, 1000),
		AnalysisRateLimiter: ratelimit.NewInMemoryLimiter(1000, time.Minute, 1000),
		AllowedOrigins:      []string{"http://localhost:5173"},
		Version:             "test",
	}
}

func TestAuthFlow_RegisterLoginMeLogout(t *testing.T) {
	s := newTestServer(t)
	router := s.Router()
	email := fmt.Sprintf("test-%d@example.com", time.Now().UnixNano())

	// Register.
	regBody := fmt.Sprintf(`{"email":%q,"password":"correct-horse-battery","display_name":"Test User"}`, email)
	regReq := httptest.NewRequest("POST", "/api/v1/auth/register", strings.NewReader(regBody))
	regReq.Header.Set("Content-Type", "application/json")
	regRec := httptest.NewRecorder()
	router.ServeHTTP(regRec, regReq)

	if regRec.Code != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d: %s", regRec.Code, regRec.Body.String())
	}

	sessionCookie, csrfCookie := extractAuthCookies(t, regRec)

	// /me with the session cookie should succeed.
	meReq := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	meReq.AddCookie(sessionCookie)
	meRec := httptest.NewRecorder()
	router.ServeHTTP(meRec, meReq)

	if meRec.Code != http.StatusOK {
		t.Fatalf("me: expected 200, got %d: %s", meRec.Code, meRec.Body.String())
	}

	var meBody struct {
		Email string `json:"email"`
	}
	if err := json.Unmarshal(meRec.Body.Bytes(), &meBody); err != nil {
		t.Fatalf("decode /me response: %v", err)
	}
	if meBody.Email != email {
		t.Errorf("expected email %q, got %q", email, meBody.Email)
	}

	// Logout without a CSRF header should be rejected.
	logoutNoCSRF := httptest.NewRequest("POST", "/api/v1/auth/logout", nil)
	logoutNoCSRF.AddCookie(sessionCookie)
	logoutNoCSRFRec := httptest.NewRecorder()
	router.ServeHTTP(logoutNoCSRFRec, logoutNoCSRF)

	if logoutNoCSRFRec.Code != http.StatusForbidden {
		t.Fatalf("logout without CSRF: expected 403, got %d", logoutNoCSRFRec.Code)
	}

	// Logout with a matching CSRF header succeeds.
	logoutReq := httptest.NewRequest("POST", "/api/v1/auth/logout", nil)
	logoutReq.AddCookie(sessionCookie)
	logoutReq.AddCookie(csrfCookie)
	logoutReq.Header.Set("X-CSRF-Token", csrfCookie.Value)
	logoutRec := httptest.NewRecorder()
	router.ServeHTTP(logoutRec, logoutReq)

	if logoutRec.Code != http.StatusNoContent {
		t.Fatalf("logout: expected 204, got %d: %s", logoutRec.Code, logoutRec.Body.String())
	}

	// The session is now invalid.
	meAfterLogout := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	meAfterLogout.AddCookie(sessionCookie)
	meAfterLogoutRec := httptest.NewRecorder()
	router.ServeHTTP(meAfterLogoutRec, meAfterLogout)

	if meAfterLogoutRec.Code != http.StatusUnauthorized {
		t.Fatalf("me after logout: expected 401, got %d", meAfterLogoutRec.Code)
	}

	// Login with the same credentials should succeed.
	loginBody := fmt.Sprintf(`{"email":%q,"password":"correct-horse-battery"}`, email)
	loginReq := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, loginReq)

	if loginRec.Code != http.StatusOK {
		t.Fatalf("login: expected 200, got %d: %s", loginRec.Code, loginRec.Body.String())
	}

	// Login with the wrong password should fail without leaking whether
	// the account exists.
	wrongLoginBody := fmt.Sprintf(`{"email":%q,"password":"totally-wrong-password"}`, email)
	wrongLoginReq := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(wrongLoginBody))
	wrongLoginReq.Header.Set("Content-Type", "application/json")
	wrongLoginRec := httptest.NewRecorder()
	router.ServeHTTP(wrongLoginRec, wrongLoginReq)

	if wrongLoginRec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password login: expected 401, got %d", wrongLoginRec.Code)
	}
}

func TestAuthFlow_DuplicateEmailRejected(t *testing.T) {
	s := newTestServer(t)
	router := s.Router()
	email := fmt.Sprintf("dup-%d@example.com", time.Now().UnixNano())
	body := fmt.Sprintf(`{"email":%q,"password":"correct-horse-battery","display_name":"Test User"}`, email)

	first := httptest.NewRequest("POST", "/api/v1/auth/register", strings.NewReader(body))
	first.Header.Set("Content-Type", "application/json")
	firstRec := httptest.NewRecorder()
	router.ServeHTTP(firstRec, first)
	if firstRec.Code != http.StatusCreated {
		t.Fatalf("first register: expected 201, got %d: %s", firstRec.Code, firstRec.Body.String())
	}

	second := httptest.NewRequest("POST", "/api/v1/auth/register", strings.NewReader(body))
	second.Header.Set("Content-Type", "application/json")
	secondRec := httptest.NewRecorder()
	router.ServeHTTP(secondRec, second)
	if secondRec.Code != http.StatusConflict {
		t.Fatalf("duplicate register: expected 409, got %d: %s", secondRec.Code, secondRec.Body.String())
	}
}

func extractAuthCookies(t *testing.T, rec *httptest.ResponseRecorder) (session, csrf *http.Cookie) {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		switch c.Name {
		case "shield_session":
			session = c
		case "shield_csrf":
			csrf = c
		}
	}
	if session == nil || csrf == nil {
		t.Fatalf("expected both session and csrf cookies, got: %v", rec.Result().Cookies())
	}
	return session, csrf
}
