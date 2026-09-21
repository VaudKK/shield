package pdfredact

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_Redact(t *testing.T) {
	wantPDF := []byte("%PDF-1.4 fake redacted content")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/redact" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			t.Fatalf("parse multipart form: %v", err)
		}

		file, _, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("read file field: %v", err)
		}
		defer file.Close()
		uploaded, err := io.ReadAll(file)
		if err != nil {
			t.Fatalf("read uploaded bytes: %v", err)
		}
		if string(uploaded) != "original pdf bytes" {
			t.Errorf("unexpected uploaded content: %s", uploaded)
		}

		var values []string
		if err := json.Unmarshal([]byte(r.FormValue("values")), &values); err != nil {
			t.Fatalf("decode values field: %v", err)
		}
		if len(values) != 2 || values[0] != "jane@example.com" {
			t.Errorf("unexpected values: %v", values)
		}

		resp := redactResponse{
			PDFBase64: base64.StdEncoding.EncodeToString(wantPDF),
			Applied:   map[string]bool{"jane@example.com": true, "555-1234": false},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	result, err := client.Redact(t.Context(), []byte("original pdf bytes"), []string{"jane@example.com", "555-1234"})
	if err != nil {
		t.Fatalf("Redact: %v", err)
	}

	if string(result.PDF) != string(wantPDF) {
		t.Errorf("expected decoded PDF %q, got %q", wantPDF, result.PDF)
	}
	if !result.Applied["jane@example.com"] {
		t.Error("expected jane@example.com to be marked applied")
	}
	if result.Applied["555-1234"] {
		t.Error("expected 555-1234 to be marked not applied")
	}
}

func TestClient_Redact_ServiceError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.Redact(t.Context(), []byte("data"), []string{"x"})
	if err == nil {
		t.Fatal("expected an error when the service returns a non-200 status")
	}
}
