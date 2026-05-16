package handlers

import (
	"net/http"
	"strings"

	"go-monolith/middleware"
)

type profileData struct {
	AppContext
	Error   string
	Success string
}

func (a *App) ProfilePage(w http.ResponseWriter, r *http.Request) {
	a.Renderer.Page(w, "profile.html", profileData{AppContext: a.ctx(r)})
}

func (a *App) ProfileUpdate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		a.Renderer.Page(w, "profile.html", profileData{AppContext: a.ctx(r), Error: "Invalid request"})
		return
	}
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	currentPwd := r.FormValue("current_password")
	newPwd := r.FormValue("new_password")

	if name == "" {
		a.Renderer.Page(w, "profile.html", profileData{AppContext: a.ctx(r), Error: "Name cannot be empty"})
		return
	}

	// If user wants to change password, validate current password first.
	if newPwd != "" {
		if len(newPwd) < 8 {
			a.Renderer.Page(w, "profile.html", profileData{AppContext: a.ctx(r), Error: "New password must be at least 8 characters"})
			return
		}
		if err := a.AuthService.VerifyPassword(user.ID, currentPwd); err != nil {
			a.Renderer.Page(w, "profile.html", profileData{AppContext: a.ctx(r), Error: "Current password is incorrect"})
			return
		}
		if err := a.AuthService.UpdatePassword(user.ID, newPwd); err != nil {
			a.Renderer.Page(w, "profile.html", profileData{AppContext: a.ctx(r), Error: "Could not update password"})
			return
		}
	}

	if err := a.AuthService.UpdateName(user.ID, name); err != nil {
		a.Renderer.Page(w, "profile.html", profileData{AppContext: a.ctx(r), Error: "Could not update profile"})
		return
	}

	// Re-fetch updated user context.
	a.Renderer.Page(w, "profile.html", profileData{AppContext: a.ctx(r), Success: "Profile updated successfully"})
}
