package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/packages/param"
	"github.com/openai/openai-go/v3/responses"
)

const defaultModel = "gpt-4o-mini"

const systemInstructions = `You are Shield's evidence analysis assistant. Shield helps people preserve
evidence related to abuse, harassment, corruption, or other incidents. You are not a law
enforcement system, a judge, an investigator, or a legal advisor.

Rules you must follow:
- Base everything only on the text provided to you. Never invent dates, names, locations,
  amounts, or events that are not present in the text.
- If information is unclear, ambiguous, or missing, say so explicitly (using the gaps field
  or "unknown" for a date) rather than guessing or filling it in.
- Use neutral, hedged language: "the document appears to state", "the available evidence
  contains", "the date could not be determined". Never write phrases like "this proves",
  "the perpetrator", "this person is guilty", or "this definitely happened".
- Do not offer opinions on fault, legality, or what the user should do or claim.
- Identify personally identifiable information (names, phone numbers, emails, addresses,
  ID numbers, dates of birth, vehicle registrations) that appears in the text, purely as a
  factual extraction — this supports the user's own privacy review, not an investigation.`

type OpenAIService struct {
	client openai.Client
	model  string
}

func NewOpenAIService(apiKey, model string) *OpenAIService {
	if model == "" {
		model = defaultModel
	}
	return &OpenAIService{
		client: openai.NewClient(option.WithAPIKey(apiKey)),
		model:  model,
	}
}

const maxAttempts = 3

func (s *OpenAIService) Analyze(ctx context.Context, input AnalysisInput) (*AnalysisResult, error) {
	if strings.TrimSpace(input.ExtractedText) == "" {
		return &AnalysisResult{
			Summary:  "No readable text was found in this evidence, so it could not be analyzed.",
			Timeline: []TimelineEntry{},
			PII:      []PIIEntry{},
			Gaps: []GapEntry{{
				Description: "No text could be extracted from this file.",
				Confidence:  "high",
			}},
		}, nil
	}

	userContent := fmt.Sprintf(
		"Filename: %s\nUploaded: %s\n\nExtracted text:\n%s",
		input.Filename, input.UploadedAt.Format(time.RFC3339), input.ExtractedText,
	)

	params := responses.ResponseNewParams{
		Model:        s.model,
		Instructions: param.NewOpt(systemInstructions),
		Input: responses.ResponseNewParamsInputUnion{
			OfString: param.NewOpt(userContent),
		},
		Text: responses.ResponseTextConfigParam{
			Format: responses.ResponseFormatTextConfigUnionParam{
				OfJSONSchema: &responses.ResponseFormatTextJSONSchemaConfigParam{
					Name:   "evidence_analysis",
					Schema: analysisSchema,
					Strict: param.NewOpt(true),
				},
			},
		},
	}

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		result, err := s.attempt(ctx, params)
		if err == nil {
			return result, nil
		}
		lastErr = err

		if attempt < maxAttempts {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(attempt) * time.Second):
			}
		}
	}

	return nil, fmt.Errorf("analyze evidence after %d attempts: %w", maxAttempts, lastErr)
}

func (s *OpenAIService) attempt(ctx context.Context, params responses.ResponseNewParams) (*AnalysisResult, error) {
	resp, err := s.client.Responses.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("call openai: %w", err)
	}

	var result AnalysisResult
	if err := json.Unmarshal([]byte(resp.OutputText()), &result); err != nil {
		return nil, fmt.Errorf("parse model output: %w", err)
	}

	if err := validate(&result); err != nil {
		return nil, fmt.Errorf("model output failed validation: %w", err)
	}

	return &result, nil
}

var validConfidence = map[string]bool{"low": true, "medium": true, "high": true}

// validate re-checks the model's output even though the schema was
// requested with strict:true — raw model output is never trusted without
// server-side validation, regardless of what was asked for.
func validate(r *AnalysisResult) error {
	if r.Timeline == nil || r.PII == nil || r.Gaps == nil {
		return fmt.Errorf("missing required array field")
	}
	for _, g := range r.Gaps {
		if !validConfidence[g.Confidence] {
			return fmt.Errorf("invalid gap confidence %q", g.Confidence)
		}
	}
	for _, t := range r.Timeline {
		if strings.TrimSpace(t.Date) == "" || strings.TrimSpace(t.Description) == "" {
			return fmt.Errorf("timeline entry missing date or description")
		}
	}
	for _, p := range r.PII {
		if strings.TrimSpace(p.Type) == "" || strings.TrimSpace(p.Value) == "" {
			return fmt.Errorf("pii entry missing type or value")
		}
	}
	return nil
}
