package handlers

import (
	"log"
	"net/http"
	"os"
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

	// Always log to server console so messages are never lost.
	log.Printf("[CONTACT] From: %s <%s> | Subject: %s | Message: %s",
		data.Name, data.Email, data.Subject, data.Message)

	// Send email to the admin address configured via CONTACT_EMAIL env var.
	// Falls back to SMTP_FROM, then a hardcoded default.
	adminEmail := os.Getenv("CONTACT_EMAIL")
	if adminEmail == "" {
		adminEmail = os.Getenv("SMTP_FROM")
	}
	if adminEmail == "" {
		adminEmail = "admin@invobill.in"
	}

	if err := a.Mailer.SendContactMessage(adminEmail, data.Name, data.Email, data.Subject, data.Message); err != nil {
		log.Printf("[CONTACT] email send failed: %v", err)
		// Don't show error to user — message is already logged above.
	}

	data.Success = true
	data.Name, data.Email, data.Subject, data.Message = "", "", "", ""
	a.Renderer.Landing(w, "contact.html", data)
}
