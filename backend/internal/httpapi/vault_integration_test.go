package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestVaultFlow_CreateReturnsIDAndRecoveryKey(t *testing.T) {
	s := newTestServer(t)
	router := s.Router()

	req := httptest.NewRequest("POST", "/api/v1/vaults/", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("create vault: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var body struct {
		VaultID     string `json:"vault_id"`
		RecoveryKey string `json:"recovery_key"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !strings.HasPrefix(body.VaultID, "SH-") {
		t.Errorf("expected vault_id to start with SH-, got %q", body.VaultID)
	}
	if len(body.RecoveryKey) < 16 {
		t.Errorf("expected a substantial recovery key, got %q", body.RecoveryKey)
	}

	// A session should already be active — no separate login step needed.
	sessionCookie, _ := extractAuthCookies(t, rec)
	meReq := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	meReq.AddCookie(sessionCookie)
	meRec := httptest.NewRecorder()
	router.ServeHTTP(meRec, meReq)
	if meRec.Code != http.StatusOK {
		t.Fatalf("me after vault creation: expected 200, got %d: %s", meRec.Code, meRec.Body.String())
	}

	var me struct {
		VaultID *string `json:"vault_id"`
		Email   *string `json:"email"`
	}
	if err := json.Unmarshal(meRec.Body.Bytes(), &me); err != nil {
		t.Fatalf("decode /me response: %v", err)
	}
	if me.VaultID == nil || *me.VaultID != body.VaultID {
		t.Errorf("expected /me vault_id to match creation response, got %+v", me)
	}
	if me.Email != nil {
		t.Errorf("expected no email on an anonymous vault, got %q", *me.Email)
	}
}

func TestVaultFlow_UploadAndRetrieveEvidence(t *testing.T) {
	s := newEvidenceTestServer(t)
	router := s.Router()

	createReq := httptest.NewRequest("POST", "/api/v1/vaults/", nil)
	createRec := httptest.NewRecorder()
	router.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create vault: expected 201, got %d: %s", createRec.Code, createRec.Body.String())
	}
	sessionCookie, csrfCookie := extractAuthCookies(t, createRec)

	body, contentType := multipartUpload(t, "vault-evidence.png", testPNG, "Vault Evidence")
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

	getReq := httptest.NewRequest("GET", "/api/v1/evidence/"+uploaded.ID, nil)
	getReq.AddCookie(sessionCookie)
	getRec := httptest.NewRecorder()
	router.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get: expected 200, got %d: %s", getRec.Code, getRec.Body.String())
	}
}

func TestVaultFlow_CrossVaultAccessDenied(t *testing.T) {
	s := newEvidenceTestServer(t)
	router := s.Router()

	// Vault A uploads evidence.
	aCreateReq := httptest.NewRequest("POST", "/api/v1/vaults/", nil)
	aCreateRec := httptest.NewRecorder()
	router.ServeHTTP(aCreateRec, aCreateReq)
	aSession, aCSRF := extractAuthCookies(t, aCreateRec)

	body, contentType := multipartUpload(t, "vault-a.png", testPNG, "Vault A Evidence")
	uploadReq := httptest.NewRequest("POST", "/api/v1/evidence/", body)
	uploadReq.Header.Set("Content-Type", contentType)
	uploadReq.Header.Set("X-CSRF-Token", aCSRF.Value)
	uploadReq.AddCookie(aSession)
	uploadReq.AddCookie(aCSRF)
	uploadRec := httptest.NewRecorder()
	router.ServeHTTP(uploadRec, uploadReq)
	if uploadRec.Code != http.StatusCreated {
		t.Fatalf("upload: expected 201, got %d: %s", uploadRec.Code, uploadRec.Body.String())
	}
	var uploaded struct {
		ID string `json:"id"`
	}
	json.Unmarshal(uploadRec.Body.Bytes(), &uploaded)

	// Vault B tries to read Vault A's evidence.
	bCreateReq := httptest.NewRequest("POST", "/api/v1/vaults/", nil)
	bCreateRec := httptest.NewRecorder()
	router.ServeHTTP(bCreateRec, bCreateReq)
	bSession, _ := extractAuthCookies(t, bCreateRec)

	crossReq := httptest.NewRequest("GET", "/api/v1/evidence/"+uploaded.ID, nil)
	crossReq.AddCookie(bSession)
	crossRec := httptest.NewRecorder()
	router.ServeHTTP(crossRec, crossReq)
	if crossRec.Code != http.StatusNotFound {
		t.Fatalf("cross-vault access: expected 404, got %d: %s", crossRec.Code, crossRec.Body.String())
	}

	// And an unauthenticated request (no session at all) must not succeed either.
	anonReq := httptest.NewRequest("GET", "/api/v1/evidence/"+uploaded.ID, nil)
	anonRec := httptest.NewRecorder()
	router.ServeHTTP(anonRec, anonReq)
	if anonRec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous access: expected 401, got %d: %s", anonRec.Code, anonRec.Body.String())
	}
}

func TestVaultFlow_Recovery(t *testing.T) {
	s := newTestServer(t)
	router := s.Router()

	createReq := httptest.NewRequest("POST", "/api/v1/vaults/", nil)
	createRec := httptest.NewRecorder()
	router.ServeHTTP(createRec, createReq)
	var created struct {
		VaultID     string `json:"vault_id"`
		RecoveryKey string `json:"recovery_key"`
	}
	json.Unmarshal(createRec.Body.Bytes(), &created)

	// Correct vault ID + recovery key succeeds.
	goodBody := `{"vault_id":"` + created.VaultID + `","recovery_key":"` + created.RecoveryKey + `"}`
	goodReq := httptest.NewRequest("POST", "/api/v1/vaults/recover", strings.NewReader(goodBody))
	goodReq.Header.Set("Content-Type", "application/json")
	goodRec := httptest.NewRecorder()
	router.ServeHTTP(goodRec, goodReq)
	if goodRec.Code != http.StatusOK {
		t.Fatalf("recover with correct key: expected 200, got %d: %s", goodRec.Code, goodRec.Body.String())
	}
	recoveredSession, _ := extractAuthCookies(t, goodRec)
	if recoveredSession == nil {
		t.Fatal("expected a session cookie after successful recovery")
	}

	// Wrong recovery key fails.
	badBody := `{"vault_id":"` + created.VaultID + `","recovery_key":"WRONG-KEYS-AAAA-BBBB"}`
	badReq := httptest.NewRequest("POST", "/api/v1/vaults/recover", strings.NewReader(badBody))
	badReq.Header.Set("Content-Type", "application/json")
	badRec := httptest.NewRecorder()
	router.ServeHTTP(badRec, badReq)
	if badRec.Code != http.StatusUnauthorized {
		t.Fatalf("recover with wrong key: expected 401, got %d: %s", badRec.Code, badRec.Body.String())
	}

	// Unknown vault ID fails the same way (no user enumeration).
	unknownBody := `{"vault_id":"SH-0000-0000","recovery_key":"WHATEVER-1111-2222-3333"}`
	unknownReq := httptest.NewRequest("POST", "/api/v1/vaults/recover", strings.NewReader(unknownBody))
	unknownReq.Header.Set("Content-Type", "application/json")
	unknownRec := httptest.NewRecorder()
	router.ServeHTTP(unknownRec, unknownReq)
	if unknownRec.Code != http.StatusUnauthorized {
		t.Fatalf("recover with unknown vault id: expected 401, got %d: %s", unknownRec.Code, unknownRec.Body.String())
	}
}
