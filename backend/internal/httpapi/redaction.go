package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/VaudKK/shield/backend/internal/domain"
	"github.com/VaudKK/shield/backend/internal/redaction"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type reviewPIIRequest struct {
	Status string `json:"status"`
}

func (s *Server) handleReviewPII(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())

	evidenceID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid evidence ID.")
		return
	}
	piiID, err := uuid.Parse(chi.URLParam(r, "piiID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid PII item ID.")
		return
	}

	var req reviewPIIRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	var status domain.PIIStatus
	switch req.Status {
	case string(domain.PIIStatusAccepted):
		status = domain.PIIStatusAccepted
	case string(domain.PIIStatusRejected):
		status = domain.PIIStatusRejected
	case string(domain.PIIStatusDetected):
		status = domain.PIIStatusDetected
	default:
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", `status must be "accepted", "rejected", or "detected".`)
		return
	}

	updated, err := s.Redaction.ReviewPII(r.Context(), evidenceID, user.ID, piiID, status)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, "PII_NOT_FOUND", "PII item could not be found.")
		return
	case err != nil:
		s.Logger.Error("review pii failed", "error", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong. Please try again.")
		return
	}

	writeJSON(w, http.StatusOK, toPIIDetectionResponse(*updated))
}

type addManualPIIRequest struct {
	Type     string `json:"type"`
	Value    string `json:"value"`
	Location string `json:"location"`
}

func (s *Server) handleAddManualPII(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())

	evidenceID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid evidence ID.")
		return
	}

	var req addManualPIIRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	req.Type = strings.TrimSpace(req.Type)
	req.Value = strings.TrimSpace(req.Value)
	if req.Type == "" || req.Value == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "type and value are required.")
		return
	}

	created, err := s.Redaction.AddManualPII(r.Context(), evidenceID, user.ID, req.Type, req.Value, req.Location)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, "EVIDENCE_NOT_FOUND", "Evidence could not be found.")
		return
	case err != nil:
		s.Logger.Error("add manual pii failed", "error", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong. Please try again.")
		return
	}

	writeJSON(w, http.StatusCreated, toPIIDetectionResponse(*created))
}

func (s *Server) handleRedactEvidence(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())

	evidenceID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid evidence ID.")
		return
	}

	result, err := s.Redaction.Redact(r.Context(), evidenceID, user.ID)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, "EVIDENCE_NOT_FOUND", "Evidence could not be found.")
		return
	case errors.Is(err, redaction.ErrNoAcceptedPII):
		writeError(w, http.StatusBadRequest, "NO_ACCEPTED_PII", "Accept at least one detected item before creating a redaction.")
		return
	case err != nil:
		s.Logger.Error("redact evidence failed", "error", err)
		writeError(w, http.StatusInternalServerError, "REDACTION_FAILED", "Redaction failed. Please try again.")
		return
	}

	writeJSON(w, http.StatusCreated, toRedactionResultResponse(result))
}

func toPIIDetectionResponse(p domain.PIIDetection) piiDetectionResponse {
	return piiDetectionResponse{
		ID:       p.ID.String(),
		Type:     p.Type,
		Value:    p.Value,
		Location: p.Location,
		Method:   string(p.DetectionMethod),
		Status:   string(p.Status),
	}
}

type redactionItemResponse struct {
	PIIDetectionID string `json:"pii_detection_id"`
	Applied        bool   `json:"applied"`
}

type redactionResultResponse struct {
	File       evidenceFileResponse    `json:"file"`
	Redactions []redactionItemResponse `json:"redactions"`
}

func toRedactionResultResponse(r *redaction.RedactResult) redactionResultResponse {
	items := make([]redactionItemResponse, len(r.Redactions))
	for i, red := range r.Redactions {
		items[i] = redactionItemResponse{
			PIIDetectionID: red.PIIDetectionID.String(),
			Applied:        red.Applied,
		}
	}
	fileResp := toEvidenceFileResponse(*r.File)
	fileResp.URL = r.FileURL
	return redactionResultResponse{
		File:       fileResp,
		Redactions: items,
	}
}
