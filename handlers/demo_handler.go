package handlers

import "net/http"

// Demo logs the visitor in as the seeded demo account and registers the resulting
// auth session token in DemoSessions. All CRUD calls for this token are routed to
// an isolated in-memory store — the real database is never touched.
// A normal admin login creates a different token that is NOT in DemoSessions,
// so real MySQL is used automatically, even in the same browser.
func (a *App) Demo(w http.ResponseWriter, r *http.Request) {
	sess, err := a.AuthService.Login(
		"admin@invobill.com",
		"admin123456",
		clientIP(r),
		r.UserAgent(),
		false,
	)
	if err != nil {
		http.Redirect(w, r, "/login?error=Demo+account+unavailable.+Please+run+the+seed+script.", http.StatusFound)
		return
	}
	a.AuthService.SetCookie(w, sess, false)

	// Bind this specific auth token → fresh in-memory DemoStore.
	// Other auth tokens (including a subsequent normal login) won't be in this map.
	a.DemoSessions.Register(sess.ID)

	// Non-HttpOnly so JS can show the "Demo Mode" banner.
	http.SetCookie(w, &http.Cookie{
		Name:     "demo_mode",
		Value:    "1",
		Path:     "/",
		MaxAge:   86400,
		HttpOnly: false,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, "/dashboard", http.StatusFound)
}
