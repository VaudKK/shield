package httpapi

import (
	"net/http"
	"time"
)

const (
	sessionCookieName = "shield_session"
	csrfCookieName    = "shield_csrf"
)

func (s *Server) setSessionCookie(w http.ResponseWriter, token string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   s.Env == "production",
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) setCSRFCookie(w http.ResponseWriter, token string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name: csrfCookieName,
		// Deliberately not HttpOnly: the frontend reads this value and
		// echoes it back in the X-CSRF-Token header (double-submit pattern).
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: false,
		Secure:   s.Env == "production",
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) clearAuthCookies(w http.ResponseWriter) {
	for _, name := range []string{sessionCookieName, csrfCookieName} {
		http.SetCookie(w, &http.Cookie{
			Name:     name,
			Value:    "",
			Path:     "/",
			Expires:  time.Unix(0, 0),
			MaxAge:   -1,
			HttpOnly: name == sessionCookieName,
			Secure:   s.Env == "production",
			SameSite: http.SameSiteLaxMode,
		})
	}
}

func readCookie(r *http.Request, name string) string {
	c, err := r.Cookie(name)
	if err != nil {
		return ""
	}
	return c.Value
}
