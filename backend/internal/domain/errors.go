// Package domain holds Shield's core data types, shared across service and
// repository layers.
package domain

import "errors"

var ErrNotFound = errors.New("not found")

// ErrEvidenceNotReady means the evidence hasn't finished its content-safety
// scan yet (status is still "quarantined"), so OCR/AI analysis can't run.
var ErrEvidenceNotReady = errors.New("evidence is not ready for analysis")
