// Command api runs Shield's HTTP API server.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/VaudKK/shield/backend/internal/ai"
	"github.com/VaudKK/shield/backend/internal/analysis"
	"github.com/VaudKK/shield/backend/internal/auth"
	"github.com/VaudKK/shield/backend/internal/config"
	"github.com/VaudKK/shield/backend/internal/contentsafety"
	"github.com/VaudKK/shield/backend/internal/db"
	"github.com/VaudKK/shield/backend/internal/disclosure"
	"github.com/VaudKK/shield/backend/internal/evidence"
	"github.com/VaudKK/shield/backend/internal/httpapi"
	"github.com/VaudKK/shield/backend/internal/ocr"
	"github.com/VaudKK/shield/backend/internal/ratelimit"
	"github.com/VaudKK/shield/backend/internal/redaction"
	"github.com/VaudKK/shield/backend/internal/repository"
	"github.com/VaudKK/shield/backend/internal/storage"
)

var version = "dev"

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	if err := run(logger); err != nil {
		logger.Error("fatal startup error", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool); err != nil {
		return err
	}
	logger.Info("migrations applied")

	authService := auth.NewService(
		repository.NewUserRepository(pool),
		repository.NewSessionRepository(pool),
	)

	// 10 requests per minute per IP, with a small burst allowance — enough
	// for a real user retrying a typo, tight enough to slow brute forcing.
	authLimiter := ratelimit.NewInMemoryLimiter(10, time.Minute, 5)

	// 20 uploads per minute per IP, generous enough for a real session
	// adding several pieces of evidence in a row.
	uploadLimiter := ratelimit.NewInMemoryLimiter(20, time.Minute, 10)

	// 10 analysis runs per minute per IP — each one is a real OpenAI call.
	analysisLimiter := ratelimit.NewInMemoryLimiter(10, time.Minute, 5)

	// 10 disclosure packages per minute per IP.
	disclosureLimiter := ratelimit.NewInMemoryLimiter(10, time.Minute, 5)

	objectStorage, err := storage.NewS3Storage(ctx, storage.S3Config{
		Endpoint:  cfg.S3Endpoint,
		Region:    cfg.S3Region,
		Bucket:    cfg.S3Bucket,
		AccessKey: cfg.S3AccessKey,
		SecretKey: cfg.S3SecretKey,
	})
	if err != nil {
		return err
	}

	var classifier contentsafety.Classifier
	if cfg.NudeNetServiceURL != "" {
		classifier = contentsafety.NewNudeNetClient(cfg.NudeNetServiceURL)
	} else {
		logger.Warn("NUDENET_SERVICE_URL not set; uploaded images will stay quarantined pending manual review")
	}

	evidenceService := evidence.NewService(
		repository.NewEvidenceRepository(pool),
		repository.NewEvidenceFileRepository(pool),
		repository.NewAuditRepository(pool),
		objectStorage,
		classifier,
	)

	ocrService := ocr.NewCompositeService(
		ocr.NewTesseractService(),
		ocr.NewPDFTextService(),
	)

	var aiService ai.Service
	if cfg.OpenAIAPIKey != "" {
		aiService = ai.NewOpenAIService(cfg.OpenAIAPIKey, cfg.OpenAIModel)
	} else {
		logger.Warn("OPENAI_API_KEY not set; evidence analysis will run OCR and PII detection only")
	}

	analysisService := analysis.NewService(
		repository.NewEvidenceRepository(pool),
		repository.NewEvidenceFileRepository(pool),
		repository.NewAuditRepository(pool),
		repository.NewAnalysisRepository(pool),
		repository.NewTimelineRepository(pool),
		repository.NewPIIRepository(pool),
		objectStorage,
		ocrService,
		aiService,
	)

	redactionService := redaction.NewService(
		repository.NewEvidenceRepository(pool),
		repository.NewEvidenceFileRepository(pool),
		repository.NewAuditRepository(pool),
		repository.NewPIIRepository(pool),
		repository.NewRedactionRepository(pool),
		repository.NewAnalysisRepository(pool),
		objectStorage,
		ocrService,
	)

	disclosureService := disclosure.NewService(
		repository.NewEvidenceRepository(pool),
		repository.NewEvidenceFileRepository(pool),
		repository.NewAnalysisRepository(pool),
		repository.NewTimelineRepository(pool),
		repository.NewPIIRepository(pool),
		repository.NewAuditRepository(pool),
		repository.NewDisclosureRepository(pool),
		objectStorage,
		ocrService,
	)

	server := &httpapi.Server{
		Pool:                  pool,
		Logger:                logger,
		Env:                   cfg.Env,
		Auth:                  authService,
		Evidence:              evidenceService,
		Analysis:              analysisService,
		Redaction:             redactionService,
		Disclosure:            disclosureService,
		AuthRateLimiter:       authLimiter,
		UploadRateLimiter:     uploadLimiter,
		AnalysisRateLimiter:   analysisLimiter,
		DisclosureRateLimiter: disclosureLimiter,
		AllowedOrigins:        cfg.CORSAllowedOrigins,
		Version:               version,
	}

	httpServer := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           server.Router(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       120 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("server starting", "port", cfg.Port, "env", cfg.Env)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		logger.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return httpServer.Shutdown(shutdownCtx)
	}
}
