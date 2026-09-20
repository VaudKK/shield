// Package httpapi wires Shield's HTTP surface: routing, middleware, and handlers.
package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/VaudKK/shield/backend/internal/auth"
	"github.com/VaudKK/shield/backend/internal/ratelimit"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Server struct {
	Pool   *pgxpool.Pool
	Logger *slog.Logger
	Env    string

	Auth *auth.Service

	// AuthRateLimiter throttles the unauthenticated auth endpoints
	// (register/login), which are the most attractive brute-force targets.
	AuthRateLimiter ratelimit.Limiter

	AllowedOrigins []string
	Version        string
}

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()

	r.Use(chimiddleware.RequestID)
	r.Use(requestLogger(s.Logger))
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.Timeout(60 * time.Second))
	r.Use(securityHeaders)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   s.AllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type", "Authorization", "X-CSRF-Token"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Get("/health", s.handleHealth)

	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/auth", func(r chi.Router) {
			r.With(rateLimit(s.AuthRateLimiter, "AUTH_RATE_LIMITED")).Post("/register", s.handleRegister)
			r.With(rateLimit(s.AuthRateLimiter, "AUTH_RATE_LIMITED")).Post("/login", s.handleLogin)

			r.Group(func(r chi.Router) {
				r.Use(s.requireAuth)
				r.Get("/me", s.handleMe)
				r.With(requireCSRF).Post("/logout", s.handleLogout)
			})
		})

		// Evidence, disclosure, and audit routes are added in later phases.
	})

	return r
}
