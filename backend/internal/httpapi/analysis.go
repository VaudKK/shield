package httpapi

import (
	"errors"
	"net/http"

	"github.com/VaudKK/shield/backend/internal/domain"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type gapResponse struct {
	Description string `json:"description"`
	Confidence  string `json:"confidence"`
}

type analysisResponse struct {
	Summary    string        `json:"summary"`
	Gaps       []gapResponse `json:"gaps"`
	Model      string        `json:"model"`
	AnalyzedAt string        `json:"analyzed_at"`
}

type timelineEventResponse struct {
	ID          string `json:"id"`
	Date        string `json:"date"`
	Description string `json:"description"`
	Source      string `json:"source"`
}

type piiDetectionResponse struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Value    string `json:"value"`
	Location string `json:"location"`
	Method   string `json:"detection_method"`
	Status   string `json:"status"`
}

type evidenceAnalysisResponse struct {
	Analysis analysisResponse        `json:"analysis"`
	Timeline []timelineEventResponse `json:"timeline"`
	PII      []piiDetectionResponse  `json:"pii"`
}

func toEvidenceAnalysisResponse(a *domain.Analysis, timeline []domain.TimelineEvent, piiDetections []domain.PIIDetection) evidenceAnalysisResponse {
	gaps := make([]gapResponse, len(a.Gaps))
	for i, g := range a.Gaps {
		gaps[i] = gapResponse{Description: g.Description, Confidence: g.Confidence}
	}

	timelineResp := make([]timelineEventResponse, len(timeline))
	for i, t := range timeline {
		timelineResp[i] = timelineEventResponse{
			ID:          t.ID.String(),
			Date:        t.EventDate,
			Description: t.Description,
			Source:      string(t.Source),
		}
	}

	piiResp := make([]piiDetectionResponse, len(piiDetections))
	for i, p := range piiDetections {
		piiResp[i] = piiDetectionResponse{
			ID:       p.ID.String(),
			Type:     p.Type,
			Value:    p.Value,
			Location: p.Location,
			Method:   string(p.DetectionMethod),
			Status:   string(p.Status),
		}
	}

	return evidenceAnalysisResponse{
		Analysis: analysisResponse{
			Summary:    a.Summary,
			Gaps:       gaps,
			Model:      a.Model,
			AnalyzedAt: a.AnalyzedAt.Format(timeFormat),
		},
		Timeline: timelineResp,
		PII:      piiResp,
	}
}

func (s *Server) handleGetEvidenceAnalysis(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid evidence ID.")
		return
	}

	result, err := s.Analysis.Get(r.Context(), id, user.ID)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, "ANALYSIS_NOT_FOUND", "This evidence has not been analyzed yet.")
		return
	case err != nil:
		s.Logger.Error("get evidence analysis failed", "error", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong. Please try again.")
		return
	}

	writeJSON(w, http.StatusOK, toEvidenceAnalysisResponse(result.Analysis, result.Timeline, result.PII))
}

func (s *Server) handleAnalyzeEvidence(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid evidence ID.")
		return
	}

	result, err := s.Analysis.Analyze(r.Context(), id, user.ID)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, "EVIDENCE_NOT_FOUND", "Evidence could not be found.")
		return
	case err != nil:
		s.Logger.Error("analyze evidence failed", "error", err)
		writeError(w, http.StatusInternalServerError, "ANALYSIS_FAILED", "Analysis failed. Please try again.")
		return
	}

	writeJSON(w, http.StatusOK, toEvidenceAnalysisResponse(result.Analysis, result.Timeline, result.PII))
}

func (s *Server) handleGetEvidenceTimeline(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid evidence ID.")
		return
	}

	events, err := s.Analysis.Timeline(r.Context(), id, user.ID)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, "EVIDENCE_NOT_FOUND", "Evidence could not be found.")
		return
	case err != nil:
		s.Logger.Error("get timeline failed", "error", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong. Please try again.")
		return
	}

	out := make([]timelineEventResponse, len(events))
	for i, t := range events {
		out[i] = timelineEventResponse{
			ID:          t.ID.String(),
			Date:        t.EventDate,
			Description: t.Description,
			Source:      string(t.Source),
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleGetEvidencePII(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid evidence ID.")
		return
	}

	detections, err := s.Analysis.PII(r.Context(), id, user.ID)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, "EVIDENCE_NOT_FOUND", "Evidence could not be found.")
		return
	case err != nil:
		s.Logger.Error("get pii failed", "error", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong. Please try again.")
		return
	}

	out := make([]piiDetectionResponse, len(detections))
	for i, p := range detections {
		out[i] = piiDetectionResponse{
			ID:       p.ID.String(),
			Type:     p.Type,
			Value:    p.Value,
			Location: p.Location,
			Method:   string(p.DetectionMethod),
			Status:   string(p.Status),
		}
	}
	writeJSON(w, http.StatusOK, out)
}
