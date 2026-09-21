// Package analysis orchestrates evidence analysis: OCR text extraction,
// deterministic PII detection, and OpenAI-backed summarization/timeline/gap
// analysis, persisting the combined result and recording audit events at
// each step.
package analysis

import (
	"context"
	"fmt"
	"strings"

	"github.com/VaudKK/shield/backend/internal/ai"
	"github.com/VaudKK/shield/backend/internal/domain"
	"github.com/VaudKK/shield/backend/internal/ocr"
	"github.com/VaudKK/shield/backend/internal/pii"
	"github.com/VaudKK/shield/backend/internal/repository"
	"github.com/VaudKK/shield/backend/internal/storage"
	"github.com/google/uuid"
)

type Service struct {
	evidence      *repository.EvidenceRepository
	files         *repository.EvidenceFileRepository
	audit         *repository.AuditRepository
	analysis      *repository.AnalysisRepository
	timeline      *repository.TimelineRepository
	piiRepo       *repository.PIIRepository
	storage       storage.Storage
	ocr           ocr.Service
	ai            ai.Service // nil disables AI analysis (OCR + regex PII still run)
	visionEnabled bool       // opt-in: also send the image itself to ai for image evidence
}

func NewService(
	evidenceRepo *repository.EvidenceRepository,
	filesRepo *repository.EvidenceFileRepository,
	auditRepo *repository.AuditRepository,
	analysisRepo *repository.AnalysisRepository,
	timelineRepo *repository.TimelineRepository,
	piiRepo *repository.PIIRepository,
	store storage.Storage,
	ocrService ocr.Service,
	aiService ai.Service,
	visionEnabled bool,
) *Service {
	return &Service{
		evidence:      evidenceRepo,
		files:         filesRepo,
		audit:         auditRepo,
		analysis:      analysisRepo,
		timeline:      timelineRepo,
		piiRepo:       piiRepo,
		storage:       store,
		ocr:           ocrService,
		ai:            aiService,
		visionEnabled: visionEnabled,
	}
}

type Result struct {
	Analysis *domain.Analysis
	Timeline []domain.TimelineEvent
	PII      []domain.PIIDetection
}

// Analyze runs the full OCR -> PII -> AI pipeline for one piece of
// evidence owned by ownerID, replacing any previous analysis result.
func (s *Service) Analyze(ctx context.Context, evidenceID, ownerID uuid.UUID) (*Result, error) {
	ev, err := s.evidence.GetByIDForOwner(ctx, evidenceID, ownerID)
	if err != nil {
		return nil, err
	}
	if ev.Status == domain.EvidenceStatusQuarantined {
		// Content-safety scanning hasn't finished (or was never
		// configured) for this evidence — running OCR/AI before that
		// completes would skip the review/sensitive gate entirely, so
		// refuse rather than silently proceeding.
		return nil, domain.ErrEvidenceNotReady
	}

	files, err := s.files.ListByEvidence(ctx, ev.ID)
	if err != nil {
		return nil, fmt.Errorf("list evidence files: %w", err)
	}

	var original *domain.EvidenceFile
	for i := range files {
		if files[i].Kind == domain.EvidenceFileKindOriginal {
			original = &files[i]
			break
		}
	}
	if original == nil {
		return nil, fmt.Errorf("evidence has no original file")
	}

	data, err := s.storage.GetObject(ctx, original.StorageKey)
	if err != nil {
		return nil, fmt.Errorf("fetch original for analysis: %w", err)
	}

	ocrResult, err := s.ocr.ExtractText(ctx, data, original.MimeType)
	if err != nil {
		return nil, fmt.Errorf("extract text: %w", err)
	}
	if err := s.audit.Record(ctx, ev.ID, domain.AuditEventOCRCompleted, nil, map[string]any{
		"applicable":  ocrResult.Applicable,
		"text_length": len(ocrResult.Text),
	}); err != nil {
		return nil, fmt.Errorf("record ocr audit event: %w", err)
	}

	regexDetections := pii.Detect(ocrResult.Text)

	var aiResult *ai.AnalysisResult
	var aiErr error
	if s.ai != nil {
		aiInput := ai.AnalysisInput{
			Filename:      original.OriginalFilename,
			UploadedAt:    original.CreatedAt,
			ExtractedText: ocrResult.Text,
		}

		// Vision fallback: only when explicitly enabled, and only for
		// image evidence — never for PDFs, and never just because a key is
		// configured. Sent alongside the OCR text (not instead of it) and
		// not gated on OCR text length: a busy scene can produce plenty of
		// *garbled* text that's just as useless as no text at all, so
		// "text is short" isn't a reliable proxy for "text is bad" — the
		// model gets both signals and can judge for itself.
		visionUsed := false
		if s.visionEnabled && strings.HasPrefix(original.MimeType, "image/") {
			aiInput.ImageBytes = data
			aiInput.ImageMimeType = original.MimeType
			visionUsed = true
		}

		aiResult, aiErr = s.ai.Analyze(ctx, aiInput)
		auditMeta := map[string]any{"vision_used": visionUsed}
		if aiErr != nil {
			auditMeta["error"] = aiErr.Error()
		} else {
			auditMeta["timeline_entries"] = len(aiResult.Timeline)
			auditMeta["gaps"] = len(aiResult.Gaps)
		}
		_ = s.audit.Record(ctx, ev.ID, domain.AuditEventAIAnalysisCompleted, nil, auditMeta)
	}

	analysisRow := domain.Analysis{
		EvidenceID: ev.ID,
		OCRText:    ocrResult.Text,
	}
	var timelineEvents []domain.TimelineEvent
	var piiDetections []domain.PIIDetection

	if aiResult != nil {
		analysisRow.Summary = aiResult.Summary
		analysisRow.Model = "openai"
		analysisRow.Gaps = make([]domain.Gap, len(aiResult.Gaps))
		for i, g := range aiResult.Gaps {
			analysisRow.Gaps[i] = domain.Gap{Description: g.Description, Confidence: g.Confidence}
		}
		for _, t := range aiResult.Timeline {
			timelineEvents = append(timelineEvents, domain.TimelineEvent{
				EvidenceID:  ev.ID,
				EventDate:   t.Date,
				Description: t.Description,
				Source:      domain.TimelineSourceAI,
			})
		}
	} else if s.ai == nil {
		analysisRow.Summary = "AI analysis is not configured for this deployment."
	} else {
		analysisRow.Summary = "AI analysis could not be completed right now. Please try again later."
	}

	piiDetections = mergeDetections(ev.ID, regexDetections, aiResultPII(aiResult))

	if _, err := s.analysis.Upsert(ctx, analysisRow); err != nil {
		return nil, fmt.Errorf("save analysis: %w", err)
	}
	if err := s.timeline.ReplaceAIEventsForEvidence(ctx, ev.ID, timelineEvents); err != nil {
		return nil, fmt.Errorf("save timeline: %w", err)
	}
	if err := s.piiRepo.ReplaceForEvidence(ctx, ev.ID, piiDetections); err != nil {
		return nil, fmt.Errorf("save pii detections: %w", err)
	}
	if err := s.audit.Record(ctx, ev.ID, domain.AuditEventPIIDetected, nil, map[string]any{
		"count": len(piiDetections),
	}); err != nil {
		return nil, fmt.Errorf("record pii audit event: %w", err)
	}

	storedAnalysis, err := s.analysis.GetByEvidenceID(ctx, ev.ID)
	if err != nil {
		return nil, fmt.Errorf("reload analysis: %w", err)
	}
	storedTimeline, err := s.timeline.ListByEvidence(ctx, ev.ID)
	if err != nil {
		return nil, fmt.Errorf("reload timeline: %w", err)
	}
	storedPII, err := s.piiRepo.ListByEvidence(ctx, ev.ID)
	if err != nil {
		return nil, fmt.Errorf("reload pii: %w", err)
	}

	return &Result{Analysis: storedAnalysis, Timeline: storedTimeline, PII: storedPII}, nil
}

func aiResultPII(r *ai.AnalysisResult) []ai.PIIEntry {
	if r == nil {
		return nil
	}
	return r.PII
}

// mergeDetections combines deterministic regex detections with the AI's
// contextual ones, preferring the regex entry when both report the same
// value (regex detections are exact-pattern matches; more reliable for the
// same value than a model's paraphrase of it).
func mergeDetections(evidenceID uuid.UUID, regexHits []pii.Detection, aiHits []ai.PIIEntry) []domain.PIIDetection {
	seen := make(map[string]bool)
	var out []domain.PIIDetection

	for _, d := range regexHits {
		key := strings.ToLower(strings.TrimSpace(d.Value))
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, domain.PIIDetection{
			EvidenceID:      evidenceID,
			Type:            d.Type,
			Value:           d.Value,
			Location:        d.Location,
			DetectionMethod: domain.PIIDetectionMethodRegex,
			Status:          domain.PIIStatusDetected,
		})
	}

	for _, d := range aiHits {
		key := strings.ToLower(strings.TrimSpace(d.Value))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, domain.PIIDetection{
			EvidenceID:      evidenceID,
			Type:            d.Type,
			Value:           d.Value,
			Location:        d.Location,
			DetectionMethod: domain.PIIDetectionMethodAI,
			Status:          domain.PIIStatusDetected,
		})
	}

	return out
}
