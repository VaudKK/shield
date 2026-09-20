// Package storage wraps S3-compatible object storage. Original evidence is
// written once via PutOriginal and never overwritten or recompressed; reads
// only ever happen through short-lived signed URLs, never permanent public
// links.
package storage

import (
	"context"
	"io"
	"time"
)

type Storage interface {
	// PutOriginal streams body to the given key unmodified. Callers are
	// responsible for choosing a key that is never reused for a different
	// original file.
	PutOriginal(ctx context.Context, key string, body io.Reader, contentType string) error

	// PresignGet returns a time-limited URL for reading the object at key.
	// It never returns a permanent public URL.
	PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error)
}
