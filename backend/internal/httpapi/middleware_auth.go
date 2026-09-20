package httpapi

import (
	"crypto/subtle"
	"net/http"
)

// requireAuth resolves the session cookie to a user and attaches it to the
// request context, rejecting the request with 401 if there is none.
func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := readCookie(r, sessionCookieName)

		user, err := s.Auth.SessionUser(r.Context(), token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Sign in required.")
			return
		}

		next.ServeHTTP(w, r.WithContext(withUser(r.Context(), user)))
	})
}

// requireCSRF enforces the double-submit cookie pattern for state-changing
// requests made by an already-authenticated session: the client must echo
// the (non-HttpOnly) CSRF cookie value back in the X-CSRF-Token header.
// A cross-site page can trigger the request but cannot read the cookie to
// produce a matching header value.
func requireCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookieToken := readCookie(r, csrfCookieName)
		headerToken := r.Header.Get("X-CSRF-Token")

		if cookieToken == "" || headerToken == "" ||
			subtle.ConstantTimeCompare([]byte(cookieToken), []byte(headerToken)) != 1 {
			writeError(w, http.StatusForbidden, "CSRF_TOKEN_INVALID", "CSRF token missing or invalid.")
			return
		}

		next.ServeHTTP(w, r)
	})
}
