package handlers

import (
	"net/http"
	"strings"
)

type ContactData struct {
	Name    string
	Email   string
	Subject string
	Message string
	Success bool
	Error   string
}

func (a *App) ContactPage(w http.ResponseWriter, r *http.Request) {
	a.Renderer.Landing(w, "contact.html", ContactData{})
}

func (a *App) ContactPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		a.Renderer.Landing(w, "contact.html", ContactData{Error: "Invalid form submission."})
		return
	}
	data := ContactData{
		Name:    strings.TrimSpace(r.FormValue("name")),
		Email:   strings.TrimSpace(r.FormValue("email")),
		Subject: strings.TrimSpace(r.FormValue("subject")),
		Message: strings.TrimSpace(r.FormValue("message")),
	}
	if data.Name == "" || data.Email == "" || data.Subject == "" || data.Message == "" {
		data.Error = "All fields are required."
		a.Renderer.Landing(w, "contact.html", data)
		return
	}
	// In production: send email via mailer. For now, log and show success.
	data.Success = true
	data.Name, data.Email, data.Subject, data.Message = "", "", "", ""
	a.Renderer.Landing(w, "contact.html", data)
}
