package middleware

import (
	"net/http"
	"os"
)

// MaxUploadMB is the request body size limit applied on all routes.
const MaxUploadMB = 10

// Security sets hardened HTTP response headers on every request.
func Security(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("X-XSS-Protection", "1; mode=block")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		h.Set("Content-Security-Policy",
			"default-src 'self'; script-src 'self' 'unsafe-inline' https://unpkg.com; "+
				"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; "+
				"font-src 'self' https://fonts.gstatic.com; "+
				"img-src 'self' data:; connect-src 'self';")

		// HSTS — only send over HTTPS (detected via env flag or X-Forwarded-Proto).
		if os.Getenv("HTTPS") == "true" || r.Header.Get("X-Forwarded-Proto") == "https" {
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		// Limit request body to prevent memory exhaustion.
		r.Body = http.MaxBytesReader(w, r.Body, MaxUploadMB*1024*1024)

		next.ServeHTTP(w, r)
	})
}
