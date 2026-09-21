package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/VaudKK/shield/backend/internal/disclosure"
	"github.com/VaudKK/shield/backend/internal/pdfredact"
	"github.com/VaudKK/shield/backend/internal/redaction"
	"github.com/VaudKK/shield/backend/internal/repository"
)

// fakePDFRedactor lets tests control exactly what "PDF redaction" reports,
// without the real pdf-redact-service running.
type fakePDFRedactor struct {
	result *pdfredact.Result
	err    error
}

func (f *fakePDFRedactor) Redact(ctx context.Context, pdfBytes []byte, values []string) (*pdfredact.Result, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.result, nil
}

// TestRedactionFlow_PDFInPlaceViaService exercises the per-evidence Redact
// flow with a configured PDF redaction service: the redacted file should
// come back as an actual application/pdf, not a text transcript.
func TestRedactionFlow_PDFInPlaceViaService(t *testing.T) {
	s := newEvidenceTestServer(t)

	fakeRedactedPDF := []byte("%PDF-1.4 fake in-place redacted bytes")
	s.Redaction = redaction.NewService(
		repository.NewEvidenceRepository(s.Pool),
		repository.NewEvidenceFileRepository(s.Pool),
		repository.NewAuditRepository(s.Pool),
		repository.NewPIIRepository(s.Pool),
		repository.NewRedactionRepository(s.Pool),
		repository.NewAnalysisRepository(s.Pool),
		mustEvidenceStorage(t),
		nil, // no word-box OCR needed: PDF path doesn't use it
		&fakePDFRedactor{result: &pdfredact.Result{
			PDF:     fakeRedactedPDF,
			Applied: map[string]bool{"555-867-5309": true},
		}},
	)
	s.Analysis = mustAnalysisService(t, s)

	router := s.Router()
	email := fmt.Sprintf("pdfredact-%d@example.com", time.Now().UnixNano())

	regBody := fmt.Sprintf(`{"email":%q,"password":"correct-horse-battery","display_name":"PDF Redact Tester"}`, email)
	regReq := httptest.NewRequest("POST", "/api/v1/auth/register", strings.NewReader(regBody))
	regReq.Header.Set("Content-Type", "application/json")
	regRec := httptest.NewRecorder()
	router.ServeHTTP(regRec, regReq)
	if regRec.Code != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d: %s", regRec.Code, regRec.Body.String())
	}
	sessionCookie, csrfCookie := extractAuthCookies(t, regRec)

	body, contentType := multipartUpload(t, "note.pdf", buildMinimalPDF("Call 555-867-5309."), "PDF Redact Evidence")
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

	analyzeReq := httptest.NewRequest("POST", "/api/v1/evidence/"+uploaded.ID+"/analyze", nil)
	analyzeReq.Header.Set("X-CSRF-Token", csrfCookie.Value)
	analyzeReq.AddCookie(sessionCookie)
	analyzeReq.AddCookie(csrfCookie)
	analyzeRec := httptest.NewRecorder()
	router.ServeHTTP(analyzeRec, analyzeReq)
	if analyzeRec.Code != http.StatusOK {
		t.Fatalf("analyze: expected 200, got %d: %s", analyzeRec.Code, analyzeRec.Body.String())
	}

	var analyzed struct {
		PII []struct {
			ID    string `json:"id"`
			Type  string `json:"type"`
			Value string `json:"value"`
		} `json:"pii"`
	}
	if err := json.Unmarshal(analyzeRec.Body.Bytes(), &analyzed); err != nil {
		t.Fatalf("decode analyze response: %v", err)
	}
	var phonePIIID string
	for _, p := range analyzed.PII {
		if p.Type == "phone_number" {
			phonePIIID = p.ID
		}
	}
	if phonePIIID == "" {
		t.Fatal("expected a detected phone_number PII item")
	}

	acceptReq := httptest.NewRequest("PATCH", "/api/v1/evidence/"+uploaded.ID+"/pii/"+phonePIIID, strings.NewReader(`{"status":"accepted"}`))
	acceptReq.Header.Set("Content-Type", "application/json")
	acceptReq.Header.Set("X-CSRF-Token", csrfCookie.Value)
	acceptReq.AddCookie(sessionCookie)
	acceptReq.AddCookie(csrfCookie)
	acceptRec := httptest.NewRecorder()
	router.ServeHTTP(acceptRec, acceptReq)
	if acceptRec.Code != http.StatusOK {
		t.Fatalf("accept pii: expected 200, got %d: %s", acceptRec.Code, acceptRec.Body.String())
	}

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
			MimeType string `json:"mime_type"`
			URL      string `json:"url"`
		} `json:"file"`
		Redactions []struct {
			Applied bool `json:"applied"`
		} `json:"redactions"`
	}
	if err := json.Unmarshal(redactRec.Body.Bytes(), &redactResult); err != nil {
		t.Fatalf("decode redact response: %v", err)
	}
	if redactResult.File.MimeType != "application/pdf" {
		t.Errorf("expected a real redacted PDF (mime application/pdf), got %q", redactResult.File.MimeType)
	}
	if len(redactResult.Redactions) != 1 || !redactResult.Redactions[0].Applied {
		t.Errorf("expected the phone number to be marked applied, got %+v", redactResult.Redactions)
	}

	resp, err := http.Get(redactResult.File.URL)
	if err != nil {
		t.Fatalf("fetch redacted file: %v", err)
	}
	defer resp.Body.Close()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		t.Fatalf("read redacted file body: %v", err)
	}
	if buf.String() != string(fakeRedactedPDF) {
		t.Errorf("expected the redacted file to be exactly what the PDF service returned, got %q", buf.String())
	}
}

// TestRedactionFlow_PDFServiceFailureFallsBackToTranscript checks that a
// failing PDF redaction service degrades to the text-transcript fallback
// rather than failing the whole request.
func TestRedactionFlow_PDFServiceFailureFallsBackToTranscript(t *testing.T) {
	s := newEvidenceTestServer(t)
	s.Redaction = redaction.NewService(
		repository.NewEvidenceRepository(s.Pool),
		repository.NewEvidenceFileRepository(s.Pool),
		repository.NewAuditRepository(s.Pool),
		repository.NewPIIRepository(s.Pool),
		repository.NewRedactionRepository(s.Pool),
		repository.NewAnalysisRepository(s.Pool),
		mustEvidenceStorage(t),
		nil,
		&fakePDFRedactor{err: fmt.Errorf("pdf redaction service unreachable")},
	)
	s.Analysis = mustAnalysisService(t, s)

	router := s.Router()
	email := fmt.Sprintf("pdfredact-fallback-%d@example.com", time.Now().UnixNano())

	regBody := fmt.Sprintf(`{"email":%q,"password":"correct-horse-battery","display_name":"PDF Fallback Tester"}`, email)
	regReq := httptest.NewRequest("POST", "/api/v1/auth/register", strings.NewReader(regBody))
	regReq.Header.Set("Content-Type", "application/json")
	regRec := httptest.NewRecorder()
	router.ServeHTTP(regRec, regReq)
	sessionCookie, csrfCookie := extractAuthCookies(t, regRec)

	body, contentType := multipartUpload(t, "note.pdf", buildMinimalPDF("Call 555-867-5309."), "Fallback Evidence")
	uploadReq := httptest.NewRequest("POST", "/api/v1/evidence/", body)
	uploadReq.Header.Set("Content-Type", contentType)
	uploadReq.Header.Set("X-CSRF-Token", csrfCookie.Value)
	uploadReq.AddCookie(sessionCookie)
	uploadReq.AddCookie(csrfCookie)
	uploadRec := httptest.NewRecorder()
	router.ServeHTTP(uploadRec, uploadReq)
	var uploaded struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(uploadRec.Body.Bytes(), &uploaded)

	analyzeReq := httptest.NewRequest("POST", "/api/v1/evidence/"+uploaded.ID+"/analyze", nil)
	analyzeReq.Header.Set("X-CSRF-Token", csrfCookie.Value)
	analyzeReq.AddCookie(sessionCookie)
	analyzeReq.AddCookie(csrfCookie)
	analyzeRec := httptest.NewRecorder()
	router.ServeHTTP(analyzeRec, analyzeReq)

	var analyzed struct {
		PII []struct {
			ID   string `json:"id"`
			Type string `json:"type"`
		} `json:"pii"`
	}
	_ = json.Unmarshal(analyzeRec.Body.Bytes(), &analyzed)
	var phonePIIID string
	for _, p := range analyzed.PII {
		if p.Type == "phone_number" {
			phonePIIID = p.ID
		}
	}
	if phonePIIID == "" {
		t.Fatal("expected a detected phone_number PII item")
	}

	acceptReq := httptest.NewRequest("PATCH", "/api/v1/evidence/"+uploaded.ID+"/pii/"+phonePIIID, strings.NewReader(`{"status":"accepted"}`))
	acceptReq.Header.Set("Content-Type", "application/json")
	acceptReq.Header.Set("X-CSRF-Token", csrfCookie.Value)
	acceptReq.AddCookie(sessionCookie)
	acceptReq.AddCookie(csrfCookie)
	router.ServeHTTP(httptest.NewRecorder(), acceptReq)

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
			MimeType string `json:"mime_type"`
		} `json:"file"`
	}
	if err := json.Unmarshal(redactRec.Body.Bytes(), &redactResult); err != nil {
		t.Fatalf("decode redact response: %v", err)
	}
	if !strings.HasPrefix(redactResult.File.MimeType, "text/plain") {
		t.Errorf("expected the fallback text transcript (text/plain), got %q", redactResult.File.MimeType)
	}
}

// TestDisclosureFlow_PDFInPlaceViaService confirms package creation also
// uses the real PDF redaction service when one is configured, producing an
// actual redacted .pdf in the ZIP instead of a .txt transcript.
func TestDisclosureFlow_PDFInPlaceViaService(t *testing.T) {
	s := newEvidenceTestServer(t)
	s.Analysis = mustAnalysisService(t, s)

	fakeRedactedPDF := []byte("%PDF-1.4 fake in-place redacted bytes for a package")
	s.Disclosure = disclosure.NewService(
		repository.NewEvidenceRepository(s.Pool),
		repository.NewEvidenceFileRepository(s.Pool),
		repository.NewAnalysisRepository(s.Pool),
		repository.NewTimelineRepository(s.Pool),
		repository.NewPIIRepository(s.Pool),
		repository.NewAuditRepository(s.Pool),
		repository.NewDisclosureRepository(s.Pool),
		mustEvidenceStorage(t),
		nil,
		&fakePDFRedactor{result: &pdfredact.Result{
			PDF:     fakeRedactedPDF,
			Applied: map[string]bool{"555-867-5309": true, "jane@example.com": true},
		}},
	)

	router := s.Router()
	email := fmt.Sprintf("disclosure-pdfredact-%d@example.com", time.Now().UnixNano())

	regBody := fmt.Sprintf(`{"email":%q,"password":"correct-horse-battery","display_name":"Disclosure PDF Redact Tester"}`, email)
	regReq := httptest.NewRequest("POST", "/api/v1/auth/register", strings.NewReader(regBody))
	regReq.Header.Set("Content-Type", "application/json")
	regRec := httptest.NewRecorder()
	router.ServeHTTP(regRec, regReq)
	sessionCookie, csrfCookie := extractAuthCookies(t, regRec)

	body, contentType := multipartUpload(t, "note.pdf", buildMinimalPDF("Contact jane@example.com or 555-867-5309."), "Disclosure PDF")
	uploadReq := httptest.NewRequest("POST", "/api/v1/evidence/", body)
	uploadReq.Header.Set("Content-Type", contentType)
	uploadReq.Header.Set("X-CSRF-Token", csrfCookie.Value)
	uploadReq.AddCookie(sessionCookie)
	uploadReq.AddCookie(csrfCookie)
	uploadRec := httptest.NewRecorder()
	router.ServeHTTP(uploadRec, uploadReq)
	var uploaded struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(uploadRec.Body.Bytes(), &uploaded)

	analyzeReq := httptest.NewRequest("POST", "/api/v1/evidence/"+uploaded.ID+"/analyze", nil)
	analyzeReq.Header.Set("X-CSRF-Token", csrfCookie.Value)
	analyzeReq.AddCookie(sessionCookie)
	analyzeReq.AddCookie(csrfCookie)
	router.ServeHTTP(httptest.NewRecorder(), analyzeReq)

	createBody := fmt.Sprintf(`{
		"title": "Disclosure PDF Redact Test",
		"evidence_ids": [%q],
		"include_photos": true,
		"remove_phone_numbers": true,
		"remove_emails": true
	}`, uploaded.ID)
	createReq := httptest.NewRequest("POST", "/api/v1/disclosures/", strings.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("X-CSRF-Token", csrfCookie.Value)
	createReq.AddCookie(sessionCookie)
	createReq.AddCookie(csrfCookie)
	createRec := httptest.NewRecorder()
	router.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create disclosure: expected 201, got %d: %s", createRec.Code, createRec.Body.String())
	}
	var created struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}

	reportHTML, evidenceBytes := downloadAndReadPackage(t, created.URL)
	if string(evidenceBytes) != string(fakeRedactedPDF) {
		t.Errorf("expected the package to contain exactly what the PDF service returned, got %q", evidenceBytes)
	}
	if !strings.Contains(reportHTML, "redacted in place") {
		t.Error("expected the report to note the PDF was redacted in place")
	}
}
