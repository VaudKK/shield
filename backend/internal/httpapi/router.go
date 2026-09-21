// Package httpapi wires Shield's HTTP surface: routing, middleware, and handlers.
package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/VaudKK/shield/backend/internal/analysis"
	"github.com/VaudKK/shield/backend/internal/auth"
	"github.com/VaudKK/shield/backend/internal/disclosure"
	"github.com/VaudKK/shield/backend/internal/evidence"
	"github.com/VaudKK/shield/backend/internal/ratelimit"
	"github.com/VaudKK/shield/backend/internal/redaction"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Server struct {
	Pool   *pgxpool.Pool
	Logger *slog.Logger
	Env    string

	Auth       *auth.Service
	Evidence   *evidence.Service
	Analysis   *analysis.Service
	Redaction  *redaction.Service
	Disclosure *disclosure.Service

	// AuthRateLimiter throttles the unauthenticated auth endpoints
	// (register/login), which are the most attractive brute-force targets.
	AuthRateLimiter ratelimit.Limiter

	// UploadRateLimiter throttles evidence uploads per IP, independent of
	// the auth limiter.
	UploadRateLimiter ratelimit.Limiter

	// AnalysisRateLimiter throttles evidence analysis requests per IP —
	// each one is an OpenAI API call and worth rate limiting independently.
	AnalysisRateLimiter ratelimit.Limiter

	// DisclosureRateLimiter throttles package generation per IP — each one
	// does real work (image processing, zipping) and hits object storage.
	DisclosureRateLimiter ratelimit.Limiter

	AllowedOrigins []string
	Version        string
}

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()

	r.Use(chimiddleware.RequestID)
	r.Use(requestLogger(s.Logger))
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.Timeout(120 * time.Second))
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
		r.Route("/vaults", func(r chi.Router) {
			r.With(rateLimit(s.AuthRateLimiter, "AUTH_RATE_LIMITED")).Post("/", s.handleCreateVault)
			r.With(rateLimit(s.AuthRateLimiter, "AUTH_RATE_LIMITED")).Post("/recover", s.handleRecoverVault)
		})

		// Email/password accounts remain available as an optional,
		// unlinked-from-the-default-UI path — see the frontend's landing
		// page, which offers only vault creation/recovery by default.
		r.Route("/auth", func(r chi.Router) {
			r.With(rateLimit(s.AuthRateLimiter, "AUTH_RATE_LIMITED")).Post("/register", s.handleRegister)
			r.With(rateLimit(s.AuthRateLimiter, "AUTH_RATE_LIMITED")).Post("/login", s.handleLogin)

			r.Group(func(r chi.Router) {
				r.Use(s.requireAuth)
				r.Get("/me", s.handleMe)
				r.With(requireCSRF).Post("/logout", s.handleLogout)
			})
		})

		r.Route("/evidence", func(r chi.Router) {
			r.Use(s.requireAuth)

			r.Get("/", s.handleListEvidence)
			r.With(rateLimit(s.UploadRateLimiter, "UPLOAD_RATE_LIMITED"), requireCSRF).Post("/", s.handleUploadEvidence)
			r.Get("/{id}", s.handleGetEvidence)
			r.With(requireCSRF).Delete("/{id}", s.handleDeleteEvidence)
			r.Get("/{id}/moderation", s.handleGetEvidenceModeration)

			r.With(rateLimit(s.AnalysisRateLimiter, "ANALYSIS_RATE_LIMITED"), requireCSRF).Post("/{id}/analyze", s.handleAnalyzeEvidence)
			r.Get("/{id}/analysis", s.handleGetEvidenceAnalysis)
			r.Get("/{id}/timeline", s.handleGetEvidenceTimeline)
			r.Get("/{id}/pii", s.handleGetEvidencePII)
			r.With(requireCSRF).Post("/{id}/pii", s.handleAddManualPII)
			r.With(requireCSRF).Patch("/{id}/pii/{piiID}", s.handleReviewPII)
			r.With(requireCSRF).Post("/{id}/redact", s.handleRedactEvidence)
		})

		r.With(s.requireAuth).Get("/audit/{evidenceID}", s.handleEvidenceAudit)

		r.Route("/disclosures", func(r chi.Router) {
			r.Use(s.requireAuth)

			r.Get("/", s.handleListDisclosures)
			r.With(rateLimit(s.DisclosureRateLimiter, "DISCLOSURE_RATE_LIMITED"), requireCSRF).Post("/", s.handleCreateDisclosure)
			r.Get("/{id}", s.handleGetDisclosure)
			r.Get("/{id}/download", s.handleDownloadDisclosure)
		})
	})

	return r
}
