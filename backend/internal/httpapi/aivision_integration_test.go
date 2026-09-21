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

	"github.com/VaudKK/shield/backend/internal/ai"
	"github.com/VaudKK/shield/backend/internal/analysis"
	"github.com/VaudKK/shield/backend/internal/contentsafety"
	"github.com/VaudKK/shield/backend/internal/evidence"
	"github.com/VaudKK/shield/backend/internal/httpapi"
	"github.com/VaudKK/shield/backend/internal/ocr"
	"github.com/VaudKK/shield/backend/internal/repository"
)

// fakeOCR returns a fixed text result for any image, letting tests control
// exactly what OCR "found" — empty, or a long string of garbage, without a
// real Tesseract call.
type fakeOCR struct{ text string }

func (f fakeOCR) ExtractText(_ context.Context, _ []byte, mimeType string) (ocr.Result, error) {
	if !strings.HasPrefix(mimeType, "image/") {
		return ocr.Result{Applicable: false}, nil
	}
	return ocr.Result{Text: f.text, Applicable: true}, nil
}

// capturingAI records the ai.AnalysisInput it was called with, so tests can
// assert exactly what was (or wasn't) sent to "OpenAI" without a real call.
type capturingAI struct {
	lastInput ai.AnalysisInput
	called    bool
}

func (c *capturingAI) Analyze(_ context.Context, input ai.AnalysisInput) (*ai.AnalysisResult, error) {
	c.lastInput = input
	c.called = true
	return &ai.AnalysisResult{
		Summary:  "test summary",
		Timeline: []ai.TimelineEntry{},
		PII:      []ai.PIIEntry{},
		Gaps:     []ai.GapEntry{},
	}, nil
}

func newVisionTestServer(t *testing.T, fakeAI *capturingAI, visionEnabled bool, ocrText string) *httpapi.Server {
	s := newEvidenceTestServer(t)

	// Uploaded images need a "safe" moderation verdict before analysis is
	// allowed to run at all (see the content-safety gate) — this test is
	// about the vision fallback, not moderation, so fake a clean scan.
	s.Evidence = evidence.NewService(
		repository.NewEvidenceRepository(s.Pool),
		repository.NewEvidenceFileRepository(s.Pool),
		repository.NewAuditRepository(s.Pool),
		repository.NewModerationRepository(s.Pool),
		mustEvidenceStorage(t),
		&fakeModerationService{result: &contentsafety.ModerationResult{Status: "safe"}},
	)

	s.Analysis = analysis.NewService(
		repository.NewEvidenceRepository(s.Pool),
		repository.NewEvidenceFileRepository(s.Pool),
		repository.NewAuditRepository(s.Pool),
		repository.NewAnalysisRepository(s.Pool),
		repository.NewTimelineRepository(s.Pool),
		repository.NewPIIRepository(s.Pool),
		mustEvidenceStorage(t),
		fakeOCR{text: ocrText},
		fakeAI,
		visionEnabled,
	)
	return s
}

func registerAndUploadPNG(t *testing.T, router http.Handler) (evidenceID string, session, csrf *http.Cookie) {
	t.Helper()
	email := fmt.Sprintf("vision-%d@example.com", time.Now().UnixNano())
	regBody := fmt.Sprintf(`{"email":%q,"password":"correct-horse-battery","display_name":"Vision Tester"}`, email)
	regReq := httptest.NewRequest("POST", "/api/v1/auth/register", strings.NewReader(regBody))
	regReq.Header.Set("Content-Type", "application/json")
	regRec := httptest.NewRecorder()
	router.ServeHTTP(regRec, regReq)
	if regRec.Code != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d: %s", regRec.Code, regRec.Body.String())
	}
	session, csrf = extractAuthCookies(t, regRec)

	body, contentType := multipartUpload(t, "photo.png", testPNG, "Vision Test Photo")
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
		ID string `json:"id"`
	}
	if err := json.Unmarshal(uploadRec.Body.Bytes(), &uploaded); err != nil {
		t.Fatalf("decode upload response: %v", err)
	}
	return uploaded.ID, session, csrf
}

func analyzeEvidence(t *testing.T, router http.Handler, evidenceID string, session, csrf *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", "/api/v1/evidence/"+evidenceID+"/analyze", nil)
	req.Header.Set("X-CSRF-Token", csrf.Value)
	req.AddCookie(session)
	req.AddCookie(csrf)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestAIVision_UsedWhenEnabledAndOCRTextEmpty(t *testing.T) {
	fakeAI := &capturingAI{}
	s := newVisionTestServer(t, fakeAI, true, "")
	router := s.Router()

	evidenceID, session, csrf := registerAndUploadPNG(t, router)
	rec := analyzeEvidence(t, router, evidenceID, session, csrf)
	if rec.Code != http.StatusOK {
		t.Fatalf("analyze: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if !fakeAI.called {
		t.Fatal("expected the AI service to be called")
	}
	if len(fakeAI.lastInput.ImageBytes) == 0 {
		t.Error("expected image bytes to be attached when vision is enabled for image evidence")
	}
	if fakeAI.lastInput.ImageMimeType != "image/png" {
		t.Errorf("expected image mime type image/png, got %q", fakeAI.lastInput.ImageMimeType)
	}
}

// TestAIVision_UsedEvenWhenOCRTextIsLongButGarbled is a regression test for
// a real finding: a busy scene photo produced 220 characters of garbled
// OCR text (from cluttered background signage), which is far more than an
// early length-based "OCR text is weak" heuristic would have treated as
// weak — even though the text was just as useless as if OCR had found
// nothing. The trigger must not depend on text length.
func TestAIVision_UsedEvenWhenOCRTextIsLongButGarbled(t *testing.T) {
	fakeAI := &capturingAI{}
	garbled := strings.Repeat("xq7 zK9# jj2 ", 20) // 240 chars of nonsense
	s := newVisionTestServer(t, fakeAI, true, garbled)
	router := s.Router()

	evidenceID, session, csrf := registerAndUploadPNG(t, router)
	rec := analyzeEvidence(t, router, evidenceID, session, csrf)
	if rec.Code != http.StatusOK {
		t.Fatalf("analyze: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if len(fakeAI.lastInput.ImageBytes) == 0 {
		t.Error("expected image bytes to be attached even though OCR text was long, since it was garbled/useless")
	}
	if fakeAI.lastInput.ExtractedText != garbled {
		t.Error("expected the (garbled) OCR text to still be sent alongside the image, not replaced by it")
	}
}

func TestAIVision_NotUsedWhenDisabled(t *testing.T) {
	fakeAI := &capturingAI{}
	s := newVisionTestServer(t, fakeAI, false, "") // disabled, even though OCR text is weak
	router := s.Router()

	evidenceID, session, csrf := registerAndUploadPNG(t, router)
	rec := analyzeEvidence(t, router, evidenceID, session, csrf)
	if rec.Code != http.StatusOK {
		t.Fatalf("analyze: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if !fakeAI.called {
		t.Fatal("expected the AI service to be called")
	}
	if len(fakeAI.lastInput.ImageBytes) != 0 {
		t.Error("expected no image bytes to be attached when vision is disabled")
	}
}
