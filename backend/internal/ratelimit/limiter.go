// Package ratelimit provides application-level request throttling.
//
// This is a defense-in-depth layer, not a substitute for a DDoS/WAF layer
// (Cloudflare or equivalent) in front of the deployed application.
package ratelimit

import "context"

// Limiter decides whether a request identified by key is allowed to
// proceed. Implementations must be safe for concurrent use.
type Limiter interface {
	Allow(ctx context.Context, key string) (bool, error)
}
