package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

const (
	csrfCookie = "csrf_token"
	CSRFHeader = "X-CSRF-Token"
)

// CSRF validates the CSRF token for mutating requests.
// Login/register routes are exempted (handled in routes.go by not applying this middleware).
// HTMX sends the token via X-CSRF-Token header (configured in base.html).
func CSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := ensureCSRFCookie(r, w)
		if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodDelete {
			got := r.Header.Get(CSRFHeader)
			if got == "" {
				got = r.FormValue("_csrf")
			}
			if got != token {
				http.Error(w, "invalid CSRF token", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// ensureCSRFCookie returns the existing token from the cookie or sets a new one.
func ensureCSRFCookie(r *http.Request, w http.ResponseWriter) string {
	if c, err := r.Cookie(csrfCookie); err == nil && c.Value != "" {
		return c.Value
	}
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	token := hex.EncodeToString(b)
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: false, // JS must read it for HTMX header injection
		SameSite: http.SameSiteLaxMode,
	})
	return token
}
