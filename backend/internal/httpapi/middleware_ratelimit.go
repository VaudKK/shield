package httpapi

import (
	"net"
	"net/http"

	"github.com/VaudKK/shield/backend/internal/ratelimit"
)

// rateLimit throttles requests per client IP using the given limiter. It
// keys on the raw TCP connection address (r.RemoteAddr) rather than
// X-Forwarded-For, since that header is client-controlled and would let an
// attacker bypass the limit by rotating its value; deploying behind
// Cloudflare or an equivalent proxy is what actually protects against
// distributed abuse (see README's Security architecture section).
func rateLimit(limiter ratelimit.Limiter, code string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := clientIP(r)

			allowed, err := limiter.Allow(r.Context(), key)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong. Please try again.")
				return
			}
			if !allowed {
				writeError(w, http.StatusTooManyRequests, code, "Too many requests. Please slow down and try again shortly.")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
