package httpapi_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/VaudKK/shield/backend/internal/evidence"
	"github.com/VaudKK/shield/backend/internal/httpapi"
	"github.com/VaudKK/shield/backend/internal/repository"
	"github.com/VaudKK/shield/backend/internal/storage"
)

// A minimal, valid one-pixel PNG, used so magic-byte sniffing accepts it.
var testPNG = []byte{
	0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
	0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4,
	0x89, 0x00, 0x00, 0x00, 0x0A, 0x49, 0x44, 0x41,
	0x54, 0x78, 0x9C, 0x63, 0x00, 0x01, 0x00, 0x00,
	0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00,
	0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE,
	0x42, 0x60, 0x82,
}

// requireS3 skips the test unless S3 credentials for a real bucket are
// present in the environment (the same S3_* variables the server itself
// reads). Test uploads use a "integration-tests/" key prefix and are
// deleted at the end of the run.
func requireS3(t *testing.T) storage.S3Config {
	t.Helper()

	cfg := storage.S3Config{
		Endpoint:  os.Getenv("S3_ENDPOINT"),
		Region:    os.Getenv("S3_REGION"),
		Bucket:    os.Getenv("S3_BUCKET"),
		AccessKey: os.Getenv("S3_ACCESS_KEY_ID"),
		SecretKey: os.Getenv("S3_SECRET_ACCESS_KEY"),
	}
	if cfg.Bucket == "" || cfg.AccessKey == "" || cfg.SecretKey == "" {
		t.Skip("S3 credentials not set; skipping evidence storage integration test")
	}
	if cfg.Region == "" {
		cfg.Region = "auto"
	}
	return cfg
}

func newEvidenceTestServer(t *testing.T) *httpapi.Server {
	base := newTestServer(t) // from auth_integration_test.go; also skips without TEST_DATABASE_URL

	s3cfg := requireS3(t)
	store, err := storage.NewS3Storage(t.Context(), s3cfg)
	if err != nil {
		t.Fatalf("create s3 storage: %v", err)
	}

	base.Evidence = evidence.NewService(
		repository.NewEvidenceRepository(base.Pool),
		repository.NewEvidenceFileRepository(base.Pool),
		repository.NewAuditRepository(base.Pool),
		store,
		nil, // no content-safety classifier in this test; covered separately in internal/contentsafety
	)

	return base
}

func multipartUpload(t *testing.T, filename string, content []byte, title string) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	if title != "" {
		if err := w.WriteField("title", title); err != nil {
			t.Fatalf("write title field: %v", err)
		}
	}

	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write file content: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	return &buf, w.FormDataContentType()
}

func TestEvidenceFlow_UploadGetAuditDelete(t *testing.T) {
	s := newEvidenceTestServer(t)
	router := s.Router()
	email := fmt.Sprintf("evidence-%d@example.com", time.Now().UnixNano())

	// Register to get a session.
	regBody := fmt.Sprintf(`{"email":%q,"password":"correct-horse-battery","display_name":"Evidence Tester"}`, email)
	regReq := httptest.NewRequest("POST", "/api/v1/auth/register", strings.NewReader(regBody))
	regReq.Header.Set("Content-Type", "application/json")
	regRec := httptest.NewRecorder()
	router.ServeHTTP(regRec, regReq)
	if regRec.Code != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d: %s", regRec.Code, regRec.Body.String())
	}
	sessionCookie, csrfCookie := extractAuthCookies(t, regRec)

	// Upload a valid PNG.
	body, contentType := multipartUpload(t, "photo.png", testPNG, "Test Photo")
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
		ID    string `json:"id"`
		Title string `json:"title"`
		Files []struct {
			SHA256    string `json:"sha256"`
			SizeBytes int64  `json:"size_bytes"`
			MimeType  string `json:"mime_type"`
		} `json:"files"`
	}
	if err := json.Unmarshal(uploadRec.Body.Bytes(), &uploaded); err != nil {
		t.Fatalf("decode upload response: %v", err)
	}
	if uploaded.Title != "Test Photo" {
		t.Errorf("expected title %q, got %q", "Test Photo", uploaded.Title)
	}
	if len(uploaded.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(uploaded.Files))
	}
	if uploaded.Files[0].MimeType != "image/png" {
		t.Errorf("expected mime image/png, got %q", uploaded.Files[0].MimeType)
	}
	if uploaded.Files[0].SizeBytes != int64(len(testPNG)) {
		t.Errorf("expected size %d, got %d", len(testPNG), uploaded.Files[0].SizeBytes)
	}
	if uploaded.Files[0].SHA256 == "" {
		t.Error("expected a non-empty sha256")
	}

	// Uploading an unsupported type should be rejected.
	badBody, badCT := multipartUpload(t, "notes.txt", []byte("just some text"), "")
	badReq := httptest.NewRequest("POST", "/api/v1/evidence/", badBody)
	badReq.Header.Set("Content-Type", badCT)
	badReq.Header.Set("X-CSRF-Token", csrfCookie.Value)
	badReq.AddCookie(sessionCookie)
	badReq.AddCookie(csrfCookie)
	badRec := httptest.NewRecorder()
	router.ServeHTTP(badRec, badReq)
	if badRec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("bad upload: expected 415, got %d: %s", badRec.Code, badRec.Body.String())
	}

	// Get the evidence back: should include a signed original_url.
	getReq := httptest.NewRequest("GET", "/api/v1/evidence/"+uploaded.ID, nil)
	getReq.AddCookie(sessionCookie)
	getRec := httptest.NewRecorder()
	router.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get: expected 200, got %d: %s", getRec.Code, getRec.Body.String())
	}

	var detail struct {
		OriginalURL string `json:"original_url"`
		Status      string `json:"status"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode get response: %v", err)
	}
	if detail.OriginalURL == "" {
		t.Error("expected a signed original_url")
	}
	if detail.Status != "quarantined" {
		t.Errorf("expected status quarantined, got %q", detail.Status)
	}

	// Audit trail should have at least upload, hash, and view events.
	auditReq := httptest.NewRequest("GET", "/api/v1/audit/"+uploaded.ID, nil)
	auditReq.AddCookie(sessionCookie)
	auditRec := httptest.NewRecorder()
	router.ServeHTTP(auditRec, auditReq)
	if auditRec.Code != http.StatusOK {
		t.Fatalf("audit: expected 200, got %d: %s", auditRec.Code, auditRec.Body.String())
	}

	var events []struct {
		EventType string `json:"event_type"`
	}
	if err := json.Unmarshal(auditRec.Body.Bytes(), &events); err != nil {
		t.Fatalf("decode audit response: %v", err)
	}
	wantTypes := map[string]bool{"EVIDENCE_UPLOADED": false, "HASH_CREATED": false, "EVIDENCE_VIEWED": false}
	for _, e := range events {
		if _, ok := wantTypes[e.EventType]; ok {
			wantTypes[e.EventType] = true
		}
	}
	for eventType, seen := range wantTypes {
		if !seen {
			t.Errorf("expected audit trail to include %s", eventType)
		}
	}

	// Another user must not be able to see this evidence.
	otherEmail := fmt.Sprintf("other-%d@example.com", time.Now().UnixNano())
	otherRegBody := fmt.Sprintf(`{"email":%q,"password":"correct-horse-battery","display_name":"Other User"}`, otherEmail)
	otherRegReq := httptest.NewRequest("POST", "/api/v1/auth/register", strings.NewReader(otherRegBody))
	otherRegReq.Header.Set("Content-Type", "application/json")
	otherRegRec := httptest.NewRecorder()
	router.ServeHTTP(otherRegRec, otherRegReq)
	otherSession, _ := extractAuthCookies(t, otherRegRec)

	crossReq := httptest.NewRequest("GET", "/api/v1/evidence/"+uploaded.ID, nil)
	crossReq.AddCookie(otherSession)
	crossRec := httptest.NewRecorder()
	router.ServeHTTP(crossRec, crossReq)
	if crossRec.Code != http.StatusNotFound {
		t.Fatalf("cross-user get: expected 404, got %d: %s", crossRec.Code, crossRec.Body.String())
	}

	// Delete, then confirm it's gone for the owner too.
	delReq := httptest.NewRequest("DELETE", "/api/v1/evidence/"+uploaded.ID, nil)
	delReq.Header.Set("X-CSRF-Token", csrfCookie.Value)
	delReq.AddCookie(sessionCookie)
	delReq.AddCookie(csrfCookie)
	delRec := httptest.NewRecorder()
	router.ServeHTTP(delRec, delReq)
	if delRec.Code != http.StatusNoContent {
		t.Fatalf("delete: expected 204, got %d: %s", delRec.Code, delRec.Body.String())
	}

	getAfterDeleteReq := httptest.NewRequest("GET", "/api/v1/evidence/"+uploaded.ID, nil)
	getAfterDeleteReq.AddCookie(sessionCookie)
	getAfterDeleteRec := httptest.NewRecorder()
	router.ServeHTTP(getAfterDeleteRec, getAfterDeleteReq)
	if getAfterDeleteRec.Code != http.StatusNotFound {
		t.Fatalf("get after delete: expected 404, got %d: %s", getAfterDeleteRec.Code, getAfterDeleteRec.Body.String())
	}
}
