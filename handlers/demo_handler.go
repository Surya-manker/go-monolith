package handlers

import "net/http"

// Demo logs the visitor in as the seeded demo account (read-only feel,
// but actually a real session). The demo user is created by the seed script.
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
	// Set a cookie flag so the UI can show a "Demo Mode" banner.
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
