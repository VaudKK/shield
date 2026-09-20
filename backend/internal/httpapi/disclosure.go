package httpapi

import (
	"errors"
	"net/http"

	"github.com/VaudKK/shield/backend/internal/disclosure"
	"github.com/VaudKK/shield/backend/internal/domain"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type disclosureResponse struct {
	ID                 string   `json:"id"`
	Title              string   `json:"title"`
	IncludeTimeline    bool     `json:"include_timeline"`
	IncludeSummary     bool     `json:"include_summary"`
	IncludePhotos      bool     `json:"include_photos"`
	RemovePhoneNumbers bool     `json:"remove_phone_numbers"`
	RemoveEmails       bool     `json:"remove_emails"`
	RemoveIDNumbers    bool     `json:"remove_id_numbers"`
	BlurFaces          bool     `json:"blur_faces"`
	RemoveMetadata     bool     `json:"remove_metadata"`
	CreatedAt          string   `json:"created_at"`
	EvidenceIDs        []string `json:"evidence_ids,omitempty"`
	URL                string   `json:"url,omitempty"`
}

func toDisclosureResponse(d *domain.Disclosure) disclosureResponse {
	return disclosureResponse{
		ID:                 d.ID.String(),
		Title:              d.Title,
		IncludeTimeline:    d.IncludeTimeline,
		IncludeSummary:     d.IncludeSummary,
		IncludePhotos:      d.IncludePhotos,
		RemovePhoneNumbers: d.RemovePhoneNumbers,
		RemoveEmails:       d.RemoveEmails,
		RemoveIDNumbers:    d.RemoveIDNumbers,
		BlurFaces:          d.BlurFaces,
		RemoveMetadata:     d.RemoveMetadata,
		CreatedAt:          d.CreatedAt.Format(timeFormat),
	}
}

type createDisclosureRequest struct {
	Title              string   `json:"title"`
	EvidenceIDs        []string `json:"evidence_ids"`
	IncludeTimeline    bool     `json:"include_timeline"`
	IncludeSummary     bool     `json:"include_summary"`
	IncludePhotos      bool     `json:"include_photos"`
	RemovePhoneNumbers bool     `json:"remove_phone_numbers"`
	RemoveEmails       bool     `json:"remove_emails"`
	RemoveIDNumbers    bool     `json:"remove_id_numbers"`
	BlurFaces          bool     `json:"blur_faces"`
	RemoveMetadata     bool     `json:"remove_metadata"`
}

func (s *Server) handleCreateDisclosure(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())

	var req createDisclosureRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	if len(req.Title) == 0 {
		req.Title = "Untitled Disclosure Package"
	}
	if len(req.Title) > 200 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Title is too long.")
		return
	}
	if len(req.EvidenceIDs) == 0 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Select at least one piece of evidence.")
		return
	}
	if len(req.EvidenceIDs) > 50 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "A package can include at most 50 items.")
		return
	}

	evidenceIDs := make([]uuid.UUID, len(req.EvidenceIDs))
	for i, raw := range req.EvidenceIDs {
		id, err := uuid.Parse(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid evidence ID in selection.")
			return
		}
		evidenceIDs[i] = id
	}

	result, err := s.Disclosure.Create(r.Context(), user.ID, disclosure.CreateInput{
		Title:              req.Title,
		EvidenceIDs:        evidenceIDs,
		IncludeTimeline:    req.IncludeTimeline,
		IncludeSummary:     req.IncludeSummary,
		IncludePhotos:      req.IncludePhotos,
		RemovePhoneNumbers: req.RemovePhoneNumbers,
		RemoveEmails:       req.RemoveEmails,
		RemoveIDNumbers:    req.RemoveIDNumbers,
		BlurFaces:          req.BlurFaces,
		RemoveMetadata:     req.RemoveMetadata,
	})
	switch {
	case errors.Is(err, disclosure.ErrNoEvidenceSelected):
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Select at least one piece of evidence.")
		return
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, "EVIDENCE_NOT_FOUND", "One or more selected evidence items could not be found.")
		return
	case err != nil:
		s.Logger.Error("create disclosure failed", "error", err)
		writeError(w, http.StatusInternalServerError, "DISCLOSURE_FAILED", "Could not create the disclosure package. Please try again.")
		return
	}

	resp := toDisclosureResponse(result.Disclosure)
	resp.URL = result.URL
	writeJSON(w, http.StatusCreated, resp)
}

func (s *Server) handleListDisclosures(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())

	list, err := s.Disclosure.List(r.Context(), user.ID)
	if err != nil {
		s.Logger.Error("list disclosures failed", "error", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong. Please try again.")
		return
	}

	out := make([]disclosureResponse, len(list))
	for i, d := range list {
		out[i] = toDisclosureResponse(&d)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleGetDisclosure(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid disclosure ID.")
		return
	}

	detail, err := s.Disclosure.Get(r.Context(), id, user.ID)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, "DISCLOSURE_NOT_FOUND", "Disclosure package could not be found.")
		return
	case err != nil:
		s.Logger.Error("get disclosure failed", "error", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong. Please try again.")
		return
	}

	resp := toDisclosureResponse(detail.Disclosure)
	resp.EvidenceIDs = make([]string, len(detail.EvidenceIDs))
	for i, id := range detail.EvidenceIDs {
		resp.EvidenceIDs[i] = id.String()
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleDownloadDisclosure(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid disclosure ID.")
		return
	}

	url, err := s.Disclosure.DownloadURL(r.Context(), id, user.ID)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, "DISCLOSURE_NOT_FOUND", "Disclosure package could not be found.")
		return
	case err != nil:
		s.Logger.Error("download disclosure failed", "error", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong. Please try again.")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"url": url})
}
