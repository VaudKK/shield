// Package contentsafety classifies image evidence for sensitive content via
// a self-hosted NudeNet service, and turns that classification into an
// evidence status through an explicit policy.
//
// NudeNet is a classification aid, not a legal, abuse, or CSAM detector.
// Nothing in this package ever deletes evidence — sensitive content may
// itself be legitimate evidence, so the result is a status the user is
// warned about and stays in control of, never an automatic removal.
package contentsafety

import "context"

type Label struct {
	Name  string
	Score float64
}

type Classification struct {
	Labels            []Label
	Sensitive         bool
	MaxSensitiveScore float64
}

// Classifier calls out to the content-safety service. Implementations must
// be safe for concurrent use.
type Classifier interface {
	Classify(ctx context.Context, imageBytes []byte, filename string) (*Classification, error)
}
