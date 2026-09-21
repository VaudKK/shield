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
	"github.com/VaudKK/shield/backend/internal/redaction"
	"github.com/VaudKK/shield/backend/internal/repository"
	"github.com/VaudKK/shield/backend/internal/storage"
)

// mustEvidenceStorage creates a fresh S3 storage client against the same
// real bucket the other integration tests use (see requireS3 in
// evidence_integration_test.go).
func mustEvidenceStorage(t *testing.T) storage.Storage {
	t.Helper()
	s3cfg := requireS3(t)
	store, err := storage.NewS3Storage(t.Context(), s3cfg)
	if err != nil {
		t.Fatalf("create s3 storage: %v", err)
	}
	return store
}

// mustAnalysisService wires a real analysis.Service (PDF-only OCR, no cgo
// needed) so /analyze can populate ocr_text for the redaction transcript
// fallback. AI analysis itself may or may not succeed depending on
// OPENAI_API_KEY; either way OCR still runs.
func mustAnalysisService(t *testing.T, s *httpapi.Server) *analysis.Service {
	t.Helper()
	var aiService ai.Service
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		aiService = ai.NewOpenAIService(key, os.Getenv("OPENAI_MODEL"))
	}
	return analysis.NewService(
		repository.NewEvidenceRepository(s.Pool),
		repository.NewEvidenceFileRepository(s.Pool),
		repository.NewAuditRepository(s.Pool),
		repository.NewAnalysisRepository(s.Pool),
		repository.NewTimelineRepository(s.Pool),
		repository.NewPIIRepository(s.Pool),
		mustEvidenceStorage(t),
		ocr.NewCompositeService(ocr.NewPDFTextService()),
		aiService,
		false,
	)
}

func TestRedactionFlow_PDFTranscript(t *testing.T) {
	s := newEvidenceTestServer(t)

	s.Redaction = redaction.NewService(
		repository.NewEvidenceRepository(s.Pool),
		repository.NewEvidenceFileRepository(s.Pool),
		repository.NewAuditRepository(s.Pool),
		repository.NewPIIRepository(s.Pool),
		repository.NewRedactionRepository(s.Pool),
		repository.NewAnalysisRepository(s.Pool),
		mustEvidenceStorage(t),
		ocr.NewCompositeService(ocr.NewPDFTextService()),
		nil, // no PDF redaction service configured: exercises the text-transcript fallback
	)
	s.Analysis = mustAnalysisService(t, s)

	router := s.Router()
	email := fmt.Sprintf("redaction-%d@example.com", time.Now().UnixNano())

	regBody := fmt.Sprintf(`{"email":%q,"password":"correct-horse-battery","display_name":"Redaction Tester"}`, email)
	regReq := httptest.NewRequest("POST", "/api/v1/auth/register", strings.NewReader(regBody))
	regReq.Header.Set("Content-Type", "application/json")
	regRec := httptest.NewRecorder()
	router.ServeHTTP(regRec, regReq)
	if regRec.Code != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d: %s", regRec.Code, regRec.Body.String())
	}
	sessionCookie, csrfCookie := extractAuthCookies(t, regRec)

	pdfText := "Meeting on 2026-01-15 with Jane Doe, phone 555-867-5309."
	body, contentType := multipartUpload(t, "note.pdf", buildMinimalPDF(pdfText), "Redaction PDF")
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

	// Analyze to populate ocr_text (the redaction transcript fallback reads
	// this), independent of whether the AI call itself succeeds.
	analyzeReq := httptest.NewRequest("POST", "/api/v1/evidence/"+uploaded.ID+"/analyze", nil)
	analyzeReq.Header.Set("X-CSRF-Token", csrfCookie.Value)
	analyzeReq.AddCookie(sessionCookie)
	analyzeReq.AddCookie(csrfCookie)
	analyzeRec := httptest.NewRecorder()
	router.ServeHTTP(analyzeRec, analyzeReq)
	if analyzeRec.Code != http.StatusOK {
		t.Fatalf("analyze: expected 200, got %d: %s", analyzeRec.Code, analyzeRec.Body.String())
	}

	// Add a manual PII entry for the name (regex won't catch "Jane Doe").
	manualBody := `{"type":"name","value":"Jane Doe","location":"note"}`
	manualReq := httptest.NewRequest("POST", "/api/v1/evidence/"+uploaded.ID+"/pii", strings.NewReader(manualBody))
	manualReq.Header.Set("Content-Type", "application/json")
	manualReq.Header.Set("X-CSRF-Token", csrfCookie.Value)
	manualReq.AddCookie(sessionCookie)
	manualReq.AddCookie(csrfCookie)
	manualRec := httptest.NewRecorder()
	router.ServeHTTP(manualRec, manualReq)
	if manualRec.Code != http.StatusCreated {
		t.Fatalf("add manual pii: expected 201, got %d: %s", manualRec.Code, manualRec.Body.String())
	}

	// List PII, find the regex-detected phone number, and reject it (it
	// should then NOT be redacted).
	piiReq := httptest.NewRequest("GET", "/api/v1/evidence/"+uploaded.ID+"/pii", nil)
	piiReq.AddCookie(sessionCookie)
	piiRec := httptest.NewRecorder()
	router.ServeHTTP(piiRec, piiReq)
	if piiRec.Code != http.StatusOK {
		t.Fatalf("list pii: expected 200, got %d: %s", piiRec.Code, piiRec.Body.String())
	}
	var piiItems []struct {
		ID     string `json:"id"`
		Type   string `json:"type"`
		Value  string `json:"value"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(piiRec.Body.Bytes(), &piiItems); err != nil {
		t.Fatalf("decode pii list: %v", err)
	}

	var phoneID, nameID string
	for _, item := range piiItems {
		if item.Type == "phone_number" {
			phoneID = item.ID
		}
		if item.Value == "Jane Doe" {
			nameID = item.ID
		}
	}
	if phoneID == "" {
		t.Fatalf("expected a regex-detected phone_number entry, got %+v", piiItems)
	}
	if nameID == "" {
		t.Fatalf("expected the manually-added name entry, got %+v", piiItems)
	}

	// Accept the phone number.
	acceptBody := `{"status":"accepted"}`
	acceptReq := httptest.NewRequest("PATCH", "/api/v1/evidence/"+uploaded.ID+"/pii/"+phoneID, strings.NewReader(acceptBody))
	acceptReq.Header.Set("Content-Type", "application/json")
	acceptReq.Header.Set("X-CSRF-Token", csrfCookie.Value)
	acceptReq.AddCookie(sessionCookie)
	acceptReq.AddCookie(csrfCookie)
	acceptRec := httptest.NewRecorder()
	router.ServeHTTP(acceptRec, acceptReq)
	if acceptRec.Code != http.StatusOK {
		t.Fatalf("accept phone: expected 200, got %d: %s", acceptRec.Code, acceptRec.Body.String())
	}

	// Reject the manually-added name — it should NOT end up redacted.
	rejectBody := `{"status":"rejected"}`
	rejectReq := httptest.NewRequest("PATCH", "/api/v1/evidence/"+uploaded.ID+"/pii/"+nameID, strings.NewReader(rejectBody))
	rejectReq.Header.Set("Content-Type", "application/json")
	rejectReq.Header.Set("X-CSRF-Token", csrfCookie.Value)
	rejectReq.AddCookie(sessionCookie)
	rejectReq.AddCookie(csrfCookie)
	rejectRec := httptest.NewRecorder()
	router.ServeHTTP(rejectRec, rejectReq)
	if rejectRec.Code != http.StatusOK {
		t.Fatalf("reject name: expected 200, got %d: %s", rejectRec.Code, rejectRec.Body.String())
	}

	// Redact: only the accepted phone number should be covered.
	redactReq := httptest.NewRequest("POST", "/api/v1/evidence/"+uploaded.ID+"/redact", nil)
	redactReq.Header.Set("X-CSRF-Token", csrfCookie.Value)
	redactReq.AddCookie(sessionCookie)
	redactReq.AddCookie(csrfCookie)
	redactRec := httptest.NewRecorder()
	router.ServeHTTP(redactRec, redactReq)
	if redactRec.Code != http.StatusCreated {
		t.Fatalf("redact: expected 201, got %d: %s", redactRec.Code, redactRec.Body.String())
	}

	var redactResult struct {
		File struct {
			ID       string `json:"id"`
			Kind     string `json:"kind"`
			MimeType string `json:"mime_type"`
			URL      string `json:"url"`
		} `json:"file"`
		Redactions []struct {
			PIIDetectionID string `json:"pii_detection_id"`
			Applied        bool   `json:"applied"`
		} `json:"redactions"`
	}
	if err := json.Unmarshal(redactRec.Body.Bytes(), &redactResult); err != nil {
		t.Fatalf("decode redact response: %v", err)
	}
	if redactResult.File.Kind != "redacted" {
		t.Errorf("expected kind redacted, got %q", redactResult.File.Kind)
	}
	if len(redactResult.Redactions) != 1 {
		t.Fatalf("expected exactly 1 redaction (only the accepted phone), got %d: %+v", len(redactResult.Redactions), redactResult.Redactions)
	}
	if !redactResult.Redactions[0].Applied {
		t.Error("expected the phone number to be found and applied in the transcript")
	}

	// Fetch the redacted file's content via its signed URL and verify the
	// phone number is gone but the rejected name is still present (proving
	// only accepted items were redacted, not a blanket wipe).
	if redactResult.File.URL == "" {
		t.Fatal("expected a signed URL for the redacted file")
	}
	resp, err := http.Get(redactResult.File.URL)
	if err != nil {
		t.Fatalf("fetch redacted file: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 fetching redacted file, got %d", resp.StatusCode)
	}
	var transcript strings.Builder
	buf := make([]byte, 4096)
	for {
		n, readErr := resp.Body.Read(buf)
		transcript.Write(buf[:n])
		if readErr != nil {
			break
		}
	}
	got := transcript.String()
	if strings.Contains(got, "555-867-5309") {
		t.Errorf("expected phone number to be redacted from transcript, got: %s", got)
	}
	if !strings.Contains(got, "[REDACTED]") {
		t.Errorf("expected a [REDACTED] marker in transcript, got: %s", got)
	}
	if !strings.Contains(got, "Jane Doe") {
		t.Errorf("expected rejected name to remain untouched in transcript, got: %s", got)
	}

	// Original evidence detail should now list both the original and the
	// new redacted file, and the original must be byte-for-byte unchanged
	// (checked via its hash, already verified in evidence_integration_test.go;
	// here we just confirm both files are listed).
	getReq := httptest.NewRequest("GET", "/api/v1/evidence/"+uploaded.ID, nil)
	getReq.AddCookie(sessionCookie)
	getRec := httptest.NewRecorder()
	router.ServeHTTP(getRec, getReq)
	var detail struct {
		Files []struct {
			Kind string `json:"kind"`
		} `json:"files"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode evidence detail: %v", err)
	}
	kinds := map[string]bool{}
	for _, f := range detail.Files {
		kinds[f.Kind] = true
	}
	if !kinds["original"] || !kinds["redacted"] {
		t.Errorf("expected both original and redacted files listed, got %+v", detail.Files)
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
