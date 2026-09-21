package httpapi_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/VaudKK/shield/backend/internal/analysis"
	"github.com/VaudKK/shield/backend/internal/contentsafety"
	"github.com/VaudKK/shield/backend/internal/evidence"
	"github.com/VaudKK/shield/backend/internal/httpapi"
	"github.com/VaudKK/shield/backend/internal/repository"
	"github.com/VaudKK/shield/backend/internal/storage"
)

// fakeModerationService lets tests control exactly what a content-safety
// scan reports, without a real NudeNet service running.
type fakeModerationService struct {
	result *contentsafety.ModerationResult
	err    error
}

func (f *fakeModerationService) Scan(ctx context.Context, file string) (*contentsafety.ModerationResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.result, nil
}

func newModerationTestServer(t *testing.T, moderator contentsafety.ModerationService) *httpapi.Server {
	s := newEvidenceTestServer(t)

	s3cfg := requireS3(t)
	store, err := storage.NewS3Storage(t.Context(), s3cfg)
	if err != nil {
		t.Fatalf("create s3 storage: %v", err)
	}

	s.Evidence = evidence.NewService(
		repository.NewEvidenceRepository(s.Pool),
		repository.NewEvidenceFileRepository(s.Pool),
		repository.NewAuditRepository(s.Pool),
		repository.NewModerationRepository(s.Pool),
		store,
		moderator,
	)

	return s
}

// registerAndUpload registers a fresh user and uploads testPNG as evidence,
// returning the decoded upload response and the session/CSRF cookies for
// further requests.
func registerAndUpload(t *testing.T, router http.Handler) (evidenceID, status string, session, csrf *http.Cookie) {
	t.Helper()

	email := fmt.Sprintf("moderation-%d@example.com", time.Now().UnixNano())
	regBody := fmt.Sprintf(`{"email":%q,"password":"correct horse battery staple","display_name":"Test"}`, email)
	regReq := httptest.NewRequest("POST", "/api/v1/auth/register", strings.NewReader(regBody))
	regReq.Header.Set("Content-Type", "application/json")
	regRec := httptest.NewRecorder()
	router.ServeHTTP(regRec, regReq)
	if regRec.Code != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d: %s", regRec.Code, regRec.Body.String())
	}
	session, csrf = extractAuthCookies(t, regRec)

	body, contentType := multipartUpload(t, "photo.png", testPNG, "Test Photo")
	uploadReq := httptest.NewRequest("POST", "/api/v1/evidence/", body)
	uploadReq.Header.Set("Content-Type", contentType)
	uploadReq.Header.Set("X-CSRF-Token", csrf.Value)
	uploadReq.AddCookie(session)
	uploadReq.AddCookie(csrf)
	uploadRec := httptest.NewRecorder()
	router.ServeHTTP(uploadRec, uploadReq)
	if uploadRec.Code != http.StatusCreated {
		t.Fatalf("upload: expected 201, got %d: %s", uploadRec.Code, uploadRec.Body.String())
	}

	var uploaded struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(uploadRec.Body.Bytes(), &uploaded); err != nil {
		t.Fatalf("decode upload response: %v", err)
	}
	return uploaded.ID, uploaded.Status, session, csrf
}

func TestModerationFlow_SensitiveContentIsFlaggedAndPersisted(t *testing.T) {
	moderator := &fakeModerationService{result: &contentsafety.ModerationResult{
		Status:      "sensitive",
		Confidence:  0.91,
		Labels:      []string{"FEMALE_BREAST_EXPOSED"},
		BoundingBox: []contentsafety.BoundingBox{{X: 1, Y: 2, Width: 3, Height: 4}},
	}}
	s := newModerationTestServer(t, moderator)
	router := s.Router()

	evidenceID, status, session, csrf := registerAndUpload(t, router)
	if status != "sensitive" {
		t.Fatalf("expected evidence status %q, got %q", "sensitive", status)
	}

	modReq := httptest.NewRequest("GET", "/api/v1/evidence/"+evidenceID+"/moderation", nil)
	modReq.AddCookie(session)
	modReq.AddCookie(csrf)
	modRec := httptest.NewRecorder()
	router.ServeHTTP(modRec, modReq)
	if modRec.Code != http.StatusOK {
		t.Fatalf("get moderation: expected 200, got %d: %s", modRec.Code, modRec.Body.String())
	}

	var result struct {
		Status        string  `json:"status"`
		Confidence    float64 `json:"confidence"`
		Labels        []string
		BoundingBoxes []struct {
			X, Y, Width, Height float64
		} `json:"bounding_boxes"`
	}
	if err := json.Unmarshal(modRec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode moderation response: %v", err)
	}
	if result.Status != "sensitive" {
		t.Errorf("expected status %q, got %q", "sensitive", result.Status)
	}
	if result.Confidence != 0.91 {
		t.Errorf("expected confidence 0.91, got %v", result.Confidence)
	}
	if len(result.Labels) != 1 || result.Labels[0] != "FEMALE_BREAST_EXPOSED" {
		t.Errorf("expected labels [FEMALE_BREAST_EXPOSED], got %v", result.Labels)
	}
	if len(result.BoundingBoxes) != 1 || result.BoundingBoxes[0].Width != 3 {
		t.Errorf("expected one bounding box with width 3, got %v", result.BoundingBoxes)
	}
}

func TestModerationFlow_SafeContentContinuesNormally(t *testing.T) {
	moderator := &fakeModerationService{result: &contentsafety.ModerationResult{
		Status:     "safe",
		Confidence: 0,
		Labels:     []string{},
	}}
	s := newModerationTestServer(t, moderator)
	router := s.Router()

	_, status, _, _ := registerAndUpload(t, router)
	if status != "safe" {
		t.Fatalf("expected evidence status %q, got %q", "safe", status)
	}
}

func TestModerationFlow_ScanFailureDegradesToReviewNotRejection(t *testing.T) {
	moderator := &fakeModerationService{err: fmt.Errorf("nudenet service unreachable")}
	s := newModerationTestServer(t, moderator)
	router := s.Router()

	_, status, _, _ := registerAndUpload(t, router)
	if status != "review" {
		t.Fatalf("expected a scan failure to degrade to %q (never rejected), got %q", "review", status)
	}
}

// TestAnalyzeEvidence_RefusesQuarantinedEvidence checks the backend-side
// guard that stops OCR/AI analysis from running before content-safety
// scanning has produced a verdict — defense in depth alongside the
// frontend's explicit "Continue Processing" gate for review/sensitive
// evidence.
func TestAnalyzeEvidence_RefusesQuarantinedEvidence(t *testing.T) {
	s := newEvidenceTestServer(t) // no moderator configured: image uploads stay quarantined
	s.Analysis = analysis.NewService(
		repository.NewEvidenceRepository(s.Pool),
		repository.NewEvidenceFileRepository(s.Pool),
		repository.NewAuditRepository(s.Pool),
		repository.NewAnalysisRepository(s.Pool),
		repository.NewTimelineRepository(s.Pool),
		repository.NewPIIRepository(s.Pool),
		nil, // storage.Storage; unreached, the quarantined guard returns first
		nil, // ocr.Service; unreached
		nil, // ai.Service; unreached
	)
	router := s.Router()

	evidenceID, status, session, csrf := registerAndUpload(t, router)
	if status != "quarantined" {
		t.Fatalf("expected evidence status %q, got %q", "quarantined", status)
	}

	analyzeReq := httptest.NewRequest("POST", "/api/v1/evidence/"+evidenceID+"/analyze", nil)
	analyzeReq.Header.Set("X-CSRF-Token", csrf.Value)
	analyzeReq.AddCookie(session)
	analyzeReq.AddCookie(csrf)
	analyzeRec := httptest.NewRecorder()
	router.ServeHTTP(analyzeRec, analyzeReq)

	if analyzeRec.Code != http.StatusConflict {
		t.Fatalf("expected 409 for quarantined evidence, got %d: %s", analyzeRec.Code, analyzeRec.Body.String())
	}
}
