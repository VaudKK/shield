// Package disclosure builds Safe Disclosure Packages: a user-curated,
// privacy-reviewed ZIP export (an HTML report plus selected evidence files)
// that never replaces or modifies the original evidence.
package disclosure

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/VaudKK/shield/backend/internal/domain"
	"github.com/VaudKK/shield/backend/internal/faceblur"
	"github.com/VaudKK/shield/backend/internal/ocr"
	"github.com/VaudKK/shield/backend/internal/redaction"
	"github.com/VaudKK/shield/backend/internal/repository"
	"github.com/VaudKK/shield/backend/internal/storage"
	"github.com/google/uuid"
)

// SignedURLTTL is how long a presigned link to a generated package stays
// valid.
const SignedURLTTL = 5 * time.Minute

type wordBoxer interface {
	ExtractWordBoxes(ctx context.Context, data []byte, mimeType string) ([]ocr.BoxedWord, error)
}

type Service struct {
	evidence     *repository.EvidenceRepository
	files        *repository.EvidenceFileRepository
	analysisRepo *repository.AnalysisRepository
	timelineRepo *repository.TimelineRepository
	piiRepo      *repository.PIIRepository
	audit        *repository.AuditRepository
	disclosures  *repository.DisclosureRepository
	storage      storage.Storage
	ocr          wordBoxer
}

func NewService(
	evidenceRepo *repository.EvidenceRepository,
	filesRepo *repository.EvidenceFileRepository,
	analysisRepo *repository.AnalysisRepository,
	timelineRepo *repository.TimelineRepository,
	piiRepo *repository.PIIRepository,
	auditRepo *repository.AuditRepository,
	disclosureRepo *repository.DisclosureRepository,
	store storage.Storage,
	ocrService wordBoxer,
) *Service {
	return &Service{
		evidence:     evidenceRepo,
		files:        filesRepo,
		analysisRepo: analysisRepo,
		timelineRepo: timelineRepo,
		piiRepo:      piiRepo,
		audit:        auditRepo,
		disclosures:  disclosureRepo,
		storage:      store,
		ocr:          ocrService,
	}
}

type CreateInput struct {
	Title       string
	EvidenceIDs []uuid.UUID

	IncludeTimeline bool
	IncludeSummary  bool
	IncludePhotos   bool

	RemovePhoneNumbers bool
	RemoveEmails       bool
	RemoveIDNumbers    bool
	BlurFaces          bool
	RemoveMetadata     bool
}

type CreateResult struct {
	Disclosure *domain.Disclosure
	URL        string
}

var ErrNoEvidenceSelected = fmt.Errorf("at least one evidence item must be selected")

// Create builds a Safe Disclosure Package for the selected evidence, owned
// by ownerID. Only PII the user hasn't explicitly rejected is subject to
// the toggle-based bulk redaction — an explicit reject on an item is
// treated as the user's deliberate choice to keep it visible, and a
// disclosure-time toggle doesn't override that.
func (s *Service) Create(ctx context.Context, ownerID uuid.UUID, input CreateInput) (*CreateResult, error) {
	if len(input.EvidenceIDs) == 0 {
		return nil, ErrNoEvidenceSelected
	}

	report := ReportData{
		Title:           input.Title,
		GeneratedAt:     time.Now().UTC().Format("2006-01-02 15:04:05 UTC"),
		IncludeSummary:  input.IncludeSummary,
		IncludeTimeline: input.IncludeTimeline,
		IncludePhotos:   input.IncludePhotos,
		Protections:     protectionLabels(input),
	}

	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)

	for i, evidenceID := range input.EvidenceIDs {
		ev, err := s.evidence.GetByIDForOwner(ctx, evidenceID, ownerID)
		if err != nil {
			return nil, fmt.Errorf("evidence %s: %w", evidenceID, err)
		}

		reportEv, includedBytes, includedName, err := s.buildEvidenceEntry(ctx, i, ev, input)
		if err != nil {
			return nil, err
		}
		report.Evidence = append(report.Evidence, reportEv)

		if includedBytes != nil {
			w, err := zw.Create("evidence/" + includedName)
			if err != nil {
				return nil, fmt.Errorf("add evidence to package: %w", err)
			}
			if _, err := w.Write(includedBytes); err != nil {
				return nil, fmt.Errorf("write evidence to package: %w", err)
			}
		}

		if input.IncludeTimeline {
			events, err := s.timelineRepo.ListByEvidence(ctx, ev.ID)
			if err != nil {
				return nil, fmt.Errorf("list timeline for %s: %w", ev.ID, err)
			}
			for _, e := range events {
				report.Timeline = append(report.Timeline, ReportTimelineEntry{
					Date:          e.EventDate,
					Description:   e.Description,
					EvidenceTitle: ev.Title,
				})
			}
		}
	}

	reportHTML, err := renderReport(report)
	if err != nil {
		return nil, fmt.Errorf("render report: %w", err)
	}
	rw, err := zw.Create("report.html")
	if err != nil {
		return nil, fmt.Errorf("add report to package: %w", err)
	}
	if _, err := rw.Write(reportHTML); err != nil {
		return nil, fmt.Errorf("write report to package: %w", err)
	}

	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("finalize package: %w", err)
	}

	disclosureID := uuid.New()
	storageKey := fmt.Sprintf("exports/%s/package.zip", disclosureID)
	if err := s.storage.Put(ctx, storageKey, bytes.NewReader(zipBuf.Bytes()), "application/zip"); err != nil {
		return nil, fmt.Errorf("store package: %w", err)
	}

	created, err := s.disclosures.Create(ctx, domain.Disclosure{
		ID:                 disclosureID,
		OwnerID:            ownerID,
		Title:              input.Title,
		IncludeTimeline:    input.IncludeTimeline,
		IncludeSummary:     input.IncludeSummary,
		IncludePhotos:      input.IncludePhotos,
		RemovePhoneNumbers: input.RemovePhoneNumbers,
		RemoveEmails:       input.RemoveEmails,
		RemoveIDNumbers:    input.RemoveIDNumbers,
		BlurFaces:          input.BlurFaces,
		RemoveMetadata:     input.RemoveMetadata,
		StorageKey:         storageKey,
	}, input.EvidenceIDs)
	if err != nil {
		return nil, fmt.Errorf("save disclosure: %w", err)
	}

	for _, evidenceID := range input.EvidenceIDs {
		actor := ownerID
		_ = s.audit.Record(ctx, evidenceID, domain.AuditEventExportCreated, &actor, map[string]any{
			"disclosure_id":    created.ID.String(),
			"disclosure_title": created.Title,
		})
	}

	url, err := s.storage.PresignGet(ctx, storageKey, SignedURLTTL)
	if err != nil {
		return nil, fmt.Errorf("presign package: %w", err)
	}

	return &CreateResult{Disclosure: created, URL: url}, nil
}

func protectionLabels(input CreateInput) []string {
	var out []string
	if input.RemovePhoneNumbers {
		out = append(out, "Phone numbers removed")
	}
	if input.RemoveEmails {
		out = append(out, "Email addresses removed")
	}
	if input.RemoveIDNumbers {
		out = append(out, "ID numbers removed")
	}
	if input.BlurFaces {
		out = append(out, "Faces blurred where detected (automated — always verify manually before sharing)")
	}
	if input.RemoveMetadata {
		out = append(out, "Sensitive file metadata removed from included photos")
	}
	return out
}

// buildEvidenceEntry gathers report data for one evidence item and, if
// IncludePhotos is set, produces the (possibly redacted) bytes to embed in
// the package.
func (s *Service) buildEvidenceEntry(ctx context.Context, index int, ev *domain.Evidence, input CreateInput) (ReportEvidence, []byte, string, error) {
	files, err := s.files.ListByEvidence(ctx, ev.ID)
	if err != nil {
		return ReportEvidence{}, nil, "", fmt.Errorf("list files for %s: %w", ev.ID, err)
	}
	var original *domain.EvidenceFile
	for i := range files {
		if files[i].Kind == domain.EvidenceFileKindOriginal {
			original = &files[i]
			break
		}
	}
	if original == nil {
		return ReportEvidence{}, nil, "", fmt.Errorf("evidence %s has no original file", ev.ID)
	}

	reportEv := ReportEvidence{
		Title:      ev.Title,
		Filename:   original.OriginalFilename,
		SHA256:     original.SHA256,
		UploadedAt: original.CreatedAt.Format("2006-01-02 15:04:05 UTC"),
	}

	analysis, analysisErr := s.analysisRepo.GetByEvidenceID(ctx, ev.ID)
	analyzed := analysisErr == nil

	if input.IncludeSummary && analyzed && analysis.Summary != "" {
		reportEv.Summary = analysis.Summary
		reportEv.HasSummary = true
	}

	if !input.IncludePhotos {
		return reportEv, nil, "", nil
	}

	pii, err := s.piiRepo.ListByEvidence(ctx, ev.ID)
	if err != nil {
		return ReportEvidence{}, nil, "", fmt.Errorf("list pii for %s: %w", ev.ID, err)
	}

	allowedType := map[string]bool{
		"email":              input.RemoveEmails,
		"phone_number":       input.RemovePhoneNumbers,
		"possible_id_number": input.RemoveIDNumbers,
	}

	var candidates []domain.PIIDetection
	for _, p := range pii {
		if p.Status == domain.PIIStatusRejected {
			continue
		}
		if allowedType[p.Type] {
			candidates = append(candidates, p)
		}
	}

	redactionRequested := input.RemovePhoneNumbers || input.RemoveEmails || input.RemoveIDNumbers
	if len(candidates) == 0 && redactionRequested && !analyzed {
		// The zero PII detections here are because this evidence has never
		// been analyzed, not because analysis found nothing — those are
		// very different situations, and silently including the original
		// unredacted file in either case would be a privacy footgun the
		// user has no way to notice.
		reportEv.RedactionNotes = append(reportEv.RedactionNotes,
			"This evidence hasn't been analyzed yet, so phone numbers, emails, and ID numbers "+
				"couldn't be checked for redaction — analyze it first, then create a new package.")
	}

	data, err := s.storage.GetObject(ctx, original.StorageKey)
	if err != nil {
		return ReportEvidence{}, nil, "", fmt.Errorf("fetch original %s: %w", ev.ID, err)
	}

	isImage := strings.HasPrefix(original.MimeType, "image/")
	includedName := fmt.Sprintf("%02d-%s", index+1, sanitizeForZip(original.OriginalFilename))

	if isImage {
		needsRework := len(candidates) > 0 || input.RemoveMetadata || input.BlurFaces
		if !needsRework {
			reportEv.IncludedAs = includedName
			return reportEv, data, includedName, nil
		}

		working := data
		reencoded := false

		if len(candidates) > 0 {
			words, err := s.ocr.ExtractWordBoxes(ctx, data, original.MimeType)
			if err != nil {
				reportEv.RedactionNotes = append(reportEv.RedactionNotes,
					fmt.Sprintf("Could not scan this image for text to redact (%v); included without text redaction.", err))
			} else {
				values := make([]string, len(candidates))
				for i, c := range candidates {
					values[i] = c.Value
				}
				redacted, applied, err := redaction.RedactImageForValues(working, words, values)
				if err != nil {
					return ReportEvidence{}, nil, "", fmt.Errorf("redact image %s: %w", ev.ID, err)
				}
				working = redacted
				reencoded = true
				for _, c := range candidates {
					if applied[c.Value] {
						reportEv.RedactionNotes = append(reportEv.RedactionNotes, fmt.Sprintf("%s: redacted", c.Type))
					} else {
						reportEv.RedactionNotes = append(reportEv.RedactionNotes, fmt.Sprintf("%s: could not be located in the image, not redacted", c.Type))
					}
				}
			}
		}

		if input.BlurFaces {
			// Detect against the original pixels, not an already-redacted
			// copy — a PII box drawn over part of a face shouldn't affect
			// whether the face itself is found.
			faces, err := faceblur.Detect(data)
			switch {
			case err != nil:
				reportEv.RedactionNotes = append(reportEv.RedactionNotes,
					fmt.Sprintf("Could not scan this image for faces (%v); included without face blurring.", err))
			case len(faces) == 0:
				reportEv.RedactionNotes = append(reportEv.RedactionNotes, "No faces detected to blur.")
			default:
				blurred, err := redaction.RedactRegions(working, faces)
				if err != nil {
					return ReportEvidence{}, nil, "", fmt.Errorf("blur faces in image %s: %w", ev.ID, err)
				}
				working = blurred
				reencoded = true
				reportEv.RedactionNotes = append(reportEv.RedactionNotes, fmt.Sprintf(
					"%d face(s) detected and blurred automatically — this is automated detection, always visually confirm before sharing.",
					len(faces)))
			}
		}

		if input.RemoveMetadata {
			if !reencoded {
				stripped, err := redaction.RedactRegions(working, nil)
				if err != nil {
					return ReportEvidence{}, nil, "", fmt.Errorf("strip metadata from image %s: %w", ev.ID, err)
				}
				working = stripped
			}
			reportEv.RedactionNotes = append(reportEv.RedactionNotes, "File metadata removed (re-encoded as PNG)")
		}

		includedName = strings.TrimSuffix(includedName, filepathExt(includedName)) + ".png"
		reportEv.IncludedAs = includedName
		return reportEv, working, includedName, nil
	}

	// Non-image (PDF): embed the original unless a redaction is needed, in
	// which case fall back to a redacted text transcript — the same
	// documented limitation as the per-evidence redaction flow.
	if len(candidates) == 0 {
		reportEv.IncludedAs = includedName
		return reportEv, data, includedName, nil
	}

	text := ""
	if analyzed {
		text = analysis.OCRText
	}
	values := make([]string, len(candidates))
	for i, c := range candidates {
		values[i] = c.Value
	}
	redactedText, applied := redaction.RedactTextForValues(text, values)
	for _, c := range candidates {
		if applied[c.Value] {
			reportEv.RedactionNotes = append(reportEv.RedactionNotes, fmt.Sprintf("%s: redacted (text transcript only — see note below)", c.Type))
		} else {
			reportEv.RedactionNotes = append(reportEv.RedactionNotes, fmt.Sprintf("%s: could not be located in the extracted text, not redacted", c.Type))
		}
	}
	reportEv.RedactionNotes = append(reportEv.RedactionNotes,
		"This PDF required redaction, so a redacted plain-text transcript was included instead of the original PDF — true in-place PDF redaction isn't supported in this version.")

	includedName = strings.TrimSuffix(includedName, filepathExt(includedName)) + "-redacted.txt"
	reportEv.IncludedAs = includedName
	return reportEv, []byte(redactedText), includedName, nil
}

func filepathExt(name string) string {
	if i := strings.LastIndex(name, "."); i != -1 {
		return name[i:]
	}
	return ""
}

func sanitizeForZip(name string) string {
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	return name
}
