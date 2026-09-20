// Package ai wraps OpenAI-backed evidence analysis: plain-language
// summaries, a chronological timeline, contextual PII detection, and
// explicit evidence-gap identification. It never makes investigative or
// legal determinations — see the system instructions in openai.go — and
// every response is validated against a strict JSON schema before it's
// trusted by callers.
package ai

import (
	"context"
	"time"
)

type TimelineEntry struct {
	// Date is "YYYY-MM-DD" when determinable, or the literal string
	// "unknown" — the model is instructed never to guess a date.
	Date        string `json:"date"`
	Description string `json:"description"`
}

type PIIEntry struct {
	Type     string `json:"type"`
	Value    string `json:"value"`
	Location string `json:"location"`
}

type GapEntry struct {
	Description string `json:"description"`
	Confidence  string `json:"confidence"` // "low" | "medium" | "high"
}

type AnalysisResult struct {
	Summary  string          `json:"summary"`
	Timeline []TimelineEntry `json:"timeline"`
	PII      []PIIEntry      `json:"pii"`
	Gaps     []GapEntry      `json:"gaps"`
}

type AnalysisInput struct {
	Filename      string
	UploadedAt    time.Time
	ExtractedText string
}

type Service interface {
	Analyze(ctx context.Context, input AnalysisInput) (*AnalysisResult, error)
}
