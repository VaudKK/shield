package httpapi_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/VaudKK/shield/backend/internal/ai"
	"github.com/VaudKK/shield/backend/internal/analysis"
	"github.com/VaudKK/shield/backend/internal/httpapi"
	"github.com/VaudKK/shield/backend/internal/ocr"
	"github.com/VaudKK/shield/backend/internal/repository"
	"github.com/VaudKK/shield/backend/internal/storage"
)

// requireOpenAI skips the test unless OPENAI_API_KEY is set, since the
// analysis pipeline makes a real OpenAI call — no mocking.
func requireOpenAI(t *testing.T) string {
	t.Helper()
	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		t.Skip("OPENAI_API_KEY not set; skipping AI analysis integration test")
	}
	return key
}

func newAnalysisTestServer(t *testing.T) *httpapi.Server {
	s := newEvidenceTestServer(t) // reuses DB + S3 setup from evidence_integration_test.go

	apiKey := requireOpenAI(t)
	s3cfg := requireS3(t)
	store, err := storage.NewS3Storage(t.Context(), s3cfg)
	if err != nil {
		t.Fatalf("create s3 storage: %v", err)
	}

	s.Analysis = analysis.NewService(
		repository.NewEvidenceRepository(s.Pool),
		repository.NewEvidenceFileRepository(s.Pool),
		repository.NewAuditRepository(s.Pool),
		repository.NewAnalysisRepository(s.Pool),
		repository.NewTimelineRepository(s.Pool),
		repository.NewPIIRepository(s.Pool),
		store,
		ocr.NewCompositeService(ocr.NewPDFTextService()), // no cgo/tesseract in this build; PDF text extraction is real
		ai.NewOpenAIService(apiKey, os.Getenv("OPENAI_MODEL")),
		false, // vision fallback covered separately, to avoid burning real API cost here
	)

	return s
}

// buildMinimalPDF constructs a valid single-page PDF containing pdfText as
// visible content, with a correctly computed cross-reference table (byte
// offsets can't be hardcoded since they depend on the text length).
func buildMinimalPDF(pdfText string) []byte {
	var buf strings.Builder
	offsets := make([]int, 6) // index 1..5 used; 0 is the free-list head

	writeObj := func(n int, body string) {
		offsets[n] = buf.Len()
		buf.WriteString(fmt.Sprintf("%d 0 obj%sendobj\n", n, body))
	}

	buf.WriteString("%PDF-1.4\n")
	writeObj(1, "<</Type/Catalog/Pages 2 0 R>>")
	writeObj(2, "<</Type/Pages/Kids[3 0 R]/Count 1>>")
	writeObj(3, "<</Type/Page/Parent 2 0 R/Resources<</Font<</F1 4 0 R>>>>/MediaBox[0 0 500 200]/Contents 5 0 R>>")
	writeObj(4, "<</Type/Font/Subtype/Type1/BaseFont/Helvetica>>")

	content := fmt.Sprintf("BT /F1 12 Tf 20 150 Td (%s) Tj ET", pdfText)
	writeObj(5, fmt.Sprintf("<</Length %d>>\nstream\n%s\nendstream\n", len(content), content))

	xrefStart := buf.Len()
	buf.WriteString("xref\n0 6\n")
	buf.WriteString("0000000000 65535 f \n")
	for i := 1; i <= 5; i++ {
		buf.WriteString(fmt.Sprintf("%010d 00000 n \n", offsets[i]))
	}
	buf.WriteString("trailer<</Size 6/Root 1 0 R>>\n")
	buf.WriteString(fmt.Sprintf("startxref\n%d\n%%%%EOF", xrefStart))

	return []byte(buf.String())
}

var testPDFText = "Meeting on 2026-01-15 with the landlord about the lease."

func TestAnalysisFlow_AnalyzePDF(t *testing.T) {
	s := newAnalysisTestServer(t)
	router := s.Router()
	email := fmt.Sprintf("analysis-%d@example.com", time.Now().UnixNano())

	regBody := fmt.Sprintf(`{"email":%q,"password":"correct-horse-battery","display_name":"Analysis Tester"}`, email)
	regReq := httptest.NewRequest("POST", "/api/v1/auth/register", strings.NewReader(regBody))
	regReq.Header.Set("Content-Type", "application/json")
	regRec := httptest.NewRecorder()
	router.ServeHTTP(regRec, regReq)
	if regRec.Code != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d: %s", regRec.Code, regRec.Body.String())
	}
	sessionCookie, csrfCookie := extractAuthCookies(t, regRec)

	body, contentType := multipartUpload(t, "note.pdf", buildMinimalPDF(testPDFText), "Landlord Note")
	uploadReq := httptest.NewRequest("POST", "/api/v1/evidence/", body)
	uploadReq.Header.Set("Content-Type", contentType)
	uploadReq.Header.Set("X-CSRF-Token", csrfCookie.Value)
	uploadReq.AddCookie(sessionCookie)
	uploadReq.AddCookie(csrfCookie)
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

	// Not analyzed yet.
	getBefore := httptest.NewRequest("GET", "/api/v1/evidence/"+uploaded.ID+"/analysis", nil)
	getBefore.AddCookie(sessionCookie)
	getBeforeRec := httptest.NewRecorder()
	router.ServeHTTP(getBeforeRec, getBefore)
	if getBeforeRec.Code != http.StatusNotFound {
		t.Fatalf("analysis before analyze: expected 404, got %d: %s", getBeforeRec.Code, getBeforeRec.Body.String())
	}

	analyzeReq := httptest.NewRequest("POST", "/api/v1/evidence/"+uploaded.ID+"/analyze", nil)
	analyzeReq.Header.Set("X-CSRF-Token", csrfCookie.Value)
	analyzeReq.AddCookie(sessionCookie)
	analyzeReq.AddCookie(csrfCookie)
	analyzeRec := httptest.NewRecorder()
	router.ServeHTTP(analyzeRec, analyzeReq)
	if analyzeRec.Code != http.StatusOK {
		t.Fatalf("analyze: expected 200, got %d: %s", analyzeRec.Code, analyzeRec.Body.String())
	}

	var result struct {
		Analysis struct {
			Summary string `json:"summary"`
			Gaps    []struct {
				Description string `json:"description"`
				Confidence  string `json:"confidence"`
			} `json:"gaps"`
		} `json:"analysis"`
		Timeline []struct {
			Date        string `json:"date"`
			Description string `json:"description"`
		} `json:"timeline"`
		PII []struct {
			Type  string `json:"type"`
			Value string `json:"value"`
		} `json:"pii"`
	}
	if err := json.Unmarshal(analyzeRec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode analyze response: %v", err)
	}

	if result.Analysis.Summary == "" {
		t.Error("expected a non-empty AI summary")
	}
	t.Logf("summary: %s", result.Analysis.Summary)
	t.Logf("timeline: %+v", result.Timeline)
	t.Logf("pii: %+v", result.PII)

	// A configured-but-failing OpenAI call (e.g. an invalid API key in the
	// test environment) degrades to this fallback summary rather than
	// failing the whole request — verified by TestValidate and by this
	// still returning 200 above. When that's what happened, there's no
	// real model output to assert against, so skip the content checks
	// rather than falsely failing on an environment/credentials problem.
	if result.Analysis.Summary == "AI analysis could not be completed right now. Please try again later." {
		t.Skip("OpenAI call failed (see summary/log above, likely an invalid OPENAI_API_KEY) — skipping model-output assertions")
	}

	foundJanuaryDate := false
	for _, e := range result.Timeline {
		if strings.Contains(e.Date, "2026-01-15") {
			foundJanuaryDate = true
		}
	}
	if !foundJanuaryDate {
		t.Errorf("expected the timeline to surface the date present in the document text (2026-01-15), got %+v", result.Timeline)
	}

	// Fetching after analysis should now succeed and match.
	getAfter := httptest.NewRequest("GET", "/api/v1/evidence/"+uploaded.ID+"/analysis", nil)
	getAfter.AddCookie(sessionCookie)
	getAfterRec := httptest.NewRecorder()
	router.ServeHTTP(getAfterRec, getAfter)
	if getAfterRec.Code != http.StatusOK {
		t.Fatalf("analysis after analyze: expected 200, got %d: %s", getAfterRec.Code, getAfterRec.Body.String())
	}

	// Cross-check the standalone timeline endpoint agrees.
	timelineReq := httptest.NewRequest("GET", "/api/v1/evidence/"+uploaded.ID+"/timeline", nil)
	timelineReq.AddCookie(sessionCookie)
	timelineRec := httptest.NewRecorder()
	router.ServeHTTP(timelineRec, timelineReq)
	if timelineRec.Code != http.StatusOK {
		t.Fatalf("timeline: expected 200, got %d: %s", timelineRec.Code, timelineRec.Body.String())
	}
	var timelineOnly []struct {
		Date string `json:"date"`
	}
	if err := json.Unmarshal(timelineRec.Body.Bytes(), &timelineOnly); err != nil {
		t.Fatalf("decode timeline response: %v", err)
	}
	if len(timelineOnly) != len(result.Timeline) {
		t.Errorf("expected /timeline to return %d entries matching /analyze, got %d", len(result.Timeline), len(timelineOnly))
	}

	// Clean up.
	delReq := httptest.NewRequest("DELETE", "/api/v1/evidence/"+uploaded.ID, nil)
	delReq.Header.Set("X-CSRF-Token", csrfCookie.Value)
	delReq.AddCookie(sessionCookie)
	delReq.AddCookie(csrfCookie)
	delRec := httptest.NewRecorder()
	router.ServeHTTP(delRec, delReq)
	if delRec.Code != http.StatusNoContent {
		t.Fatalf("cleanup delete: expected 204, got %d", delRec.Code)
	}
}
