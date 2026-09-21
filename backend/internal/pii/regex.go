// Package pii detects personally identifiable information deterministically
// via regular expressions. This is intentionally separate from and runs
// independently of the AI service — Shield does not rely entirely on an LLM
// for PII detection; the AI's contextual detections (internal/ai) supplement
// this, not the other way around.
package pii

import (
	"regexp"
	"strings"
)

type Detection struct {
	Type     string
	Value    string
	Location string
}

var (
	emailRe = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)

	// A loose international-friendly phone pattern: an optional leading +,
	// then 7-15 digits with optional separators. Phone formats vary too
	// widely to validate strictly; this trades some false positives for
	// not missing real numbers.
	phoneRe = regexp.MustCompile(`\+?\d[\d .\-()]{6,17}\d`)

	// ISO-style dates (2026-01-15, 2026/01/15) match the loose phone
	// pattern above; excluded explicitly so dates aren't misreported as
	// phone numbers.
	isoDateRe = regexp.MustCompile(`^\d{4}[.\-/]\d{2}[.\-/]\d{2}$`)

	// A decimal point directly between digits (212.25, -0.84) is a strong
	// signal of a price, percentage, or other decimal number, not a phone
	// number — real phone numbers don't use "." as a fractional separator.
	// This is what tabular numeric data (stock tables, spreadsheets: rows
	// of space-separated decimal columns) was matching the loose phone
	// pattern on before this exclusion existed.
	decimalPointRe = regexp.MustCompile(`\d\.\d`)

	// A run of 9+ digits with no separators, which commonly indicates an ID,
	// account, or reference number. Labeled conservatively as
	// "possible_id_number" rather than a specific ID type, since the format
	// can't be determined from digits alone.
	longDigitRunRe = regexp.MustCompile(`\b\d{9,}\b`)
)

const contextRadius = 25

// Detect scans text for PII using deterministic patterns only. It returns
// no more than one detection per unique (type, value) pair.
func Detect(text string) []Detection {
	seen := make(map[string]bool)
	var out []Detection

	add := func(kind string, loc []int) {
		value := text[loc[0]:loc[1]]
		key := kind + "|" + value
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, Detection{
			Type:     kind,
			Value:    value,
			Location: context(text, loc[0], loc[1]),
		})
	}

	for _, loc := range emailRe.FindAllStringIndex(text, -1) {
		add("email", loc)
	}
	for _, loc := range phoneRe.FindAllStringIndex(text, -1) {
		matched := text[loc[0]:loc[1]]
		if isoDateRe.MatchString(matched) || decimalPointRe.MatchString(matched) {
			continue
		}
		add("phone_number", loc)
	}
	for _, loc := range longDigitRunRe.FindAllStringIndex(text, -1) {
		add("possible_id_number", loc)
	}

	return out
}

func context(text string, start, end int) string {
	from := max(0, start-contextRadius)
	to := min(len(text), end+contextRadius)
	snippet := strings.TrimSpace(text[from:to])
	if from > 0 {
		snippet = "…" + snippet
	}
	if to < len(text) {
		snippet += "…"
	}
	return snippet
}
