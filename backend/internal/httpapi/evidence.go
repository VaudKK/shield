package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/VaudKK/shield/backend/internal/domain"
	"github.com/VaudKK/shield/backend/internal/evidence"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type evidenceFileResponse struct {
	ID               string `json:"id"`
	Kind             string `json:"kind"`
	OriginalFilename string `json:"original_filename"`
	MimeType         string `json:"mime_type"`
	SizeBytes        int64  `json:"size_bytes"`
	SHA256           string `json:"sha256"`
	CreatedAt        string `json:"created_at"`
	URL              string `json:"url,omitempty"`
}

type evidenceResponse struct {
	ID          string                 `json:"id"`
	Title       string                 `json:"title"`
	Status      string                 `json:"status"`
	Analyzed    bool                   `json:"analyzed"`
	CreatedAt   string                 `json:"created_at"`
	UpdatedAt   string                 `json:"updated_at"`
	Files       []evidenceFileResponse `json:"files,omitempty"`
	OriginalURL string                 `json:"original_url,omitempty"`
}

func toEvidenceResponse(e *domain.Evidence) evidenceResponse {
	return evidenceResponse{
		ID:        e.ID.String(),
		Title:     e.Title,
		Status:    string(e.Status),
		Analyzed:  e.Analyzed,
		CreatedAt: e.CreatedAt.Format(timeFormat),
		UpdatedAt: e.UpdatedAt.Format(timeFormat),
	}
}

func toEvidenceFileResponse(f domain.EvidenceFile) evidenceFileResponse {
	return evidenceFileResponse{
		ID:               f.ID.String(),
		Kind:             string(f.Kind),
		OriginalFilename: f.OriginalFilename,
		MimeType:         f.MimeType,
		SizeBytes:        f.SizeBytes,
		SHA256:           f.SHA256,
		CreatedAt:        f.CreatedAt.Format(timeFormat),
	}
}

const timeFormat = "2006-01-02T15:04:05Z07:00"

// handleUploadEvidence handles multipart evidence uploads. The request body
// is capped up front; the file's real type is sniffed from content, not
// trusted from the client Content-Type; and the original bytes are streamed
// straight to object storage unmodified.
func (s *Server) handleUploadEvidence(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())

	r.Body = http.MaxBytesReader(w, r.Body, evidence.MaxUploadBytes)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "FILE_TOO_LARGE", "The upload is too large or malformed.")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "FILE_REQUIRED", "Attach a file under the \"file\" field.")
		return
	}
	defer file.Close()

	title := strings.TrimSpace(r.FormValue("title"))
	filename := sanitizeFilename(header.Filename)

	result, err := s.Evidence.Upload(r.Context(), user.ID, title, file, filename)
	switch {
	case errors.Is(err, evidence.ErrUnsupportedFileType):
		writeError(w, http.StatusUnsupportedMediaType, "UNSUPPORTED_FILE_TYPE", err.Error())
		return
	case err != nil:
		s.Logger.Error("evidence upload failed", "error", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong. Please try again.")
		return
	}

	resp := toEvidenceResponse(result.Evidence)
	resp.Files = []evidenceFileResponse{toEvidenceFileResponse(*result.File)}
	writeJSON(w, http.StatusCreated, resp)
}

func (s *Server) handleListEvidence(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())

	list, err := s.Evidence.List(r.Context(), user.ID)
	if err != nil {
		s.Logger.Error("list evidence failed", "error", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong. Please try again.")
		return
	}

	out := make([]evidenceResponse, len(list))
	for i, e := range list {
		out[i] = toEvidenceResponse(&e)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleGetEvidence(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid evidence ID.")
		return
	}

	detail, err := s.Evidence.Get(r.Context(), id, user.ID)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, "EVIDENCE_NOT_FOUND", "Evidence could not be found.")
		return
	case err != nil:
		s.Logger.Error("get evidence failed", "error", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong. Please try again.")
		return
	}

	resp := toEvidenceResponse(detail.Evidence)
	resp.OriginalURL = detail.OriginalURL
	resp.Files = make([]evidenceFileResponse, len(detail.Files))
	for i, f := range detail.Files {
		fileResp := toEvidenceFileResponse(f)
		fileResp.URL = detail.FileURLs[f.ID]
		resp.Files[i] = fileResp
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleDeleteEvidence(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid evidence ID.")
		return
	}

	err = s.Evidence.Delete(r.Context(), id, user.ID)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, "EVIDENCE_NOT_FOUND", "Evidence could not be found.")
		return
	case err != nil:
		s.Logger.Error("delete evidence failed", "error", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong. Please try again.")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type auditEventResponse struct {
	ID        string         `json:"id"`
	EventType string         `json:"event_type"`
	ActorID   *string        `json:"actor_id,omitempty"`
	Metadata  map[string]any `json:"metadata"`
	CreatedAt string         `json:"created_at"`
}

func (s *Server) handleEvidenceAudit(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())

	id, err := uuid.Parse(chi.URLParam(r, "evidenceID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid evidence ID.")
		return
	}

	events, err := s.Evidence.AuditTrail(r.Context(), id, user.ID)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, "EVIDENCE_NOT_FOUND", "Evidence could not be found.")
		return
	case err != nil:
		s.Logger.Error("evidence audit trail failed", "error", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong. Please try again.")
		return
	}

	out := make([]auditEventResponse, len(events))
	for i, e := range events {
		var actorID *string
		if e.ActorID != nil {
			id := e.ActorID.String()
			actorID = &id
		}
		out[i] = auditEventResponse{
			ID:        e.ID.String(),
			EventType: e.EventType,
			ActorID:   actorID,
			Metadata:  e.Metadata,
			CreatedAt: e.CreatedAt.Format(timeFormat),
		}
	}
	writeJSON(w, http.StatusOK, out)
}

type moderationResultResponse struct {
	Status        string        `json:"status"`
	Confidence    float64       `json:"confidence"`
	Labels        []string      `json:"labels"`
	BoundingBoxes []boundingBox `json:"bounding_boxes"`
	CreatedAt     string        `json:"created_at"`
}

type boundingBox struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

func (s *Server) handleGetEvidenceModeration(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid evidence ID.")
		return
	}

	result, err := s.Evidence.Moderation(r.Context(), id, user.ID)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, "MODERATION_NOT_FOUND", "This evidence has no content-safety scan result.")
		return
	case err != nil:
		s.Logger.Error("get evidence moderation failed", "error", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong. Please try again.")
		return
	}

	boxes := make([]boundingBox, len(result.BoundingBox))
	for i, b := range result.BoundingBox {
		boxes[i] = boundingBox{X: b.X, Y: b.Y, Width: b.Width, Height: b.Height}
	}
	writeJSON(w, http.StatusOK, moderationResultResponse{
		Status:        string(result.Status),
		Confidence:    result.Confidence,
		Labels:        result.Labels,
		BoundingBoxes: boxes,
		CreatedAt:     result.CreatedAt.Format(timeFormat),
	})
}

// sanitizeFilename strips any directory components a browser or client
// might send, keeping only the base name.
func sanitizeFilename(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	if i := strings.LastIndex(name, "/"); i != -1 {
		name = name[i+1:]
	}
	if name == "" {
		return "upload"
	}
	return name
}
