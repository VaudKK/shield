package httpapi

import (
	"errors"
	"net/http"

	"github.com/VaudKK/shield/backend/internal/auth"
	"github.com/VaudKK/shield/backend/internal/domain"
	"github.com/VaudKK/shield/backend/internal/security"
)

type userResponse struct {
	ID          string  `json:"id"`
	Email       *string `json:"email,omitempty"`
	VaultID     *string `json:"vault_id,omitempty"`
	DisplayName string  `json:"display_name"`
	CreatedAt   string  `json:"created_at"`
}

func toUserResponse(u *domain.User) userResponse {
	return userResponse{
		ID:          u.ID.String(),
		Email:       u.Email,
		VaultID:     u.VaultID,
		DisplayName: u.DisplayName,
		CreatedAt:   u.CreatedAt.Format(timeFormat),
	}
}

type registerRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	var errs fieldErrors
	errs = append(errs, validateEmail(req.Email)...)
	errs = append(errs, validatePassword(req.Password)...)
	errs = append(errs, validateDisplayName(req.DisplayName)...)
	if len(errs) > 0 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", errs.messages())
		return
	}

	user, err := s.Auth.Register(r.Context(), req.Email, req.Password, req.DisplayName)
	switch {
	case errors.Is(err, auth.ErrEmailTaken):
		writeError(w, http.StatusConflict, "EMAIL_TAKEN", "An account with this email already exists.")
		return
	case err != nil:
		s.Logger.Error("register failed", "error", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong. Please try again.")
		return
	}

	s.startSession(w, r, user)
	writeJSON(w, http.StatusCreated, toUserResponse(user))
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	user, err := s.Auth.Authenticate(r.Context(), req.Email, req.Password)
	switch {
	case errors.Is(err, auth.ErrInvalidCredentials):
		writeError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Incorrect email or password.")
		return
	case err != nil:
		s.Logger.Error("login failed", "error", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong. Please try again.")
		return
	}

	s.startSession(w, r, user)
	writeJSON(w, http.StatusOK, toUserResponse(user))
}

type createVaultResponse struct {
	VaultID     string `json:"vault_id"`
	RecoveryKey string `json:"recovery_key"`
}

// handleCreateVault creates an anonymous vault and immediately starts a
// session for it, so the client can go straight to the dashboard. The raw
// recovery key is returned here exactly once and is never stored or logged
// — only its bcrypt hash persists.
func (s *Server) handleCreateVault(w http.ResponseWriter, r *http.Request) {
	user, rawRecoveryKey, err := s.Auth.CreateVault(r.Context())
	if err != nil {
		s.Logger.Error("create vault failed", "error", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong. Please try again.")
		return
	}

	s.startSession(w, r, user)
	writeJSON(w, http.StatusCreated, createVaultResponse{
		VaultID:     *user.VaultID,
		RecoveryKey: rawRecoveryKey,
	})
}

type recoverVaultRequest struct {
	VaultID     string `json:"vault_id"`
	RecoveryKey string `json:"recovery_key"`
}

func (s *Server) handleRecoverVault(w http.ResponseWriter, r *http.Request) {
	var req recoverVaultRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	if req.VaultID == "" || req.RecoveryKey == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Vault ID and recovery key are required.")
		return
	}

	user, err := s.Auth.RecoverVault(r.Context(), req.VaultID, req.RecoveryKey)
	switch {
	case errors.Is(err, auth.ErrInvalidVaultRecovery):
		writeError(w, http.StatusUnauthorized, "INVALID_VAULT_RECOVERY", "Incorrect vault ID or recovery key.")
		return
	case err != nil:
		s.Logger.Error("recover vault failed", "error", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong. Please try again.")
		return
	}

	s.startSession(w, r, user)
	writeJSON(w, http.StatusOK, toUserResponse(user))
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	token := readCookie(r, sessionCookieName)
	if err := s.Auth.Logout(r.Context(), token); err != nil {
		s.Logger.Error("logout failed", "error", err)
	}
	s.clearAuthCookies(w)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Sign in required.")
		return
	}
	writeJSON(w, http.StatusOK, toUserResponse(user))
}

// startSession issues a new session and CSRF token pair and sets both as
// cookies on the response.
func (s *Server) startSession(w http.ResponseWriter, r *http.Request, user *domain.User) {
	token, expiresAt, err := s.Auth.IssueSession(r.Context(), user.ID)
	if err != nil {
		s.Logger.Error("issue session failed", "error", err)
		return
	}
	s.setSessionCookie(w, token, expiresAt)

	csrfToken, _, err := security.GenerateToken()
	if err != nil {
		s.Logger.Error("generate csrf token failed", "error", err)
		return
	}
	s.setCSRFCookie(w, csrfToken, expiresAt)
}
