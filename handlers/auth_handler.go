package handlers

import (
	"net/http"
	"strings"
)

type authPageData struct {
	Error   string
	Success string
	Email   string
}

func (a *App) LoginPage(w http.ResponseWriter, r *http.Request) {
	a.Renderer.Auth(w, "login.html", authPageData{
		Error:   r.URL.Query().Get("error"),
		Success: r.URL.Query().Get("success"),
	})
}

func (a *App) LoginPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/login?error=invalid+request", http.StatusFound)
		return
	}
	email := strings.TrimSpace(r.FormValue("email"))
	password := r.FormValue("password")
	remember := r.FormValue("remember") == "on"
	ip := clientIP(r)
	ua := r.UserAgent()

	sess, err := a.AuthService.Login(email, password, ip, ua, remember)
	if err != nil {
		http.Redirect(w, r, "/login?error="+urlEncode(err.Error())+"&email="+urlEncode(email), http.StatusFound)
		return
	}
	a.AuthService.SetCookie(w, sess, remember)
	http.Redirect(w, r, "/dashboard", http.StatusFound)
}

func (a *App) RegisterPage(w http.ResponseWriter, r *http.Request) {
	a.Renderer.Auth(w, "register.html", authPageData{
		Error: r.URL.Query().Get("error"),
	})
}

func (a *App) RegisterPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/register?error=invalid+request", http.StatusFound)
		return
	}
	user, err := a.AuthService.Register(
		r.FormValue("name"),
		r.FormValue("email"),
		r.FormValue("password"),
		"staff",
	)
	if err != nil {
		http.Redirect(w, r, "/register?error="+urlEncode(err.Error()), http.StatusFound)
		return
	}
	// Auto-login after registration.
	sess, err := a.AuthService.Login(user.Email, r.FormValue("password"), clientIP(r), r.UserAgent(), false)
	if err != nil {
		http.Redirect(w, r, "/login?success=Account+created.+Please+log+in.", http.StatusFound)
		return
	}
	a.AuthService.SetCookie(w, sess, false)
	http.Redirect(w, r, "/dashboard", http.StatusFound)
}

func (a *App) Logout(w http.ResponseWriter, r *http.Request) {
	a.AuthService.Logout(r)
	a.AuthService.ClearCookie(w)
	http.Redirect(w, r, "/login?success=You+have+been+logged+out.", http.StatusFound)
}

// clientIP extracts the real client IP from the request.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	return r.RemoteAddr
}

func urlEncode(s string) string {
	return strings.ReplaceAll(s, " ", "+")
}
