// Package storage wraps S3-compatible object storage. Original evidence is
// written once via Put and never overwritten or recompressed; reads only
// ever happen through short-lived signed URLs, never permanent public
// links.
package storage

import (
	"context"
	"io"
	"time"
)

type Storage interface {
	// Put streams body to the given key unmodified. Callers are responsible
	// for choosing a key that is never reused for different content — used
	// both for original evidence (under originals/) and derived files like
	// redacted copies (under redacted/), which are written once and never
	// overwritten either.
	Put(ctx context.Context, key string, body io.Reader, contentType string) error

	// PresignGet returns a time-limited URL for reading the object at key.
	// It never returns a permanent public URL.
	PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error)

	// GetObject reads the object at key back into memory. Used internally
	// (e.g. for OCR) — never exposed directly to clients.
	GetObject(ctx context.Context, key string) ([]byte, error)
}
