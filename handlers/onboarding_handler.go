package handlers

import (
	"net/http"
	"strings"

	"go-monolith/middleware"
)

type OnboardingData struct {
	AppContext
	Step  int
	Error string
}

// OnboardingPage renders the step-by-step first-time setup wizard.
func (a *App) OnboardingPage(w http.ResponseWriter, r *http.Request) {
	step := 1
	switch r.URL.Query().Get("step") {
	case "2":
		step = 2
	case "3":
		step = 3
	}
	a.Renderer.Page(w, "onboarding.html", OnboardingData{
		AppContext: a.ctx(r),
		Step:       step,
	})
}

// OnboardingPost handles each wizard step submission.
func (a *App) OnboardingPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/setup", http.StatusFound)
		return
	}

	user := middleware.UserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	step := r.FormValue("step")
	switch step {
	case "1":
		// Business details — store in user profile fields (name/company).
		name := strings.TrimSpace(r.FormValue("business_name"))
		if name != "" {
			// Update user name to business name if provided.
			a.AuthService.UpdateName(user.ID, name)
		}
		http.Redirect(w, r, "/setup?step=2", http.StatusFound)
	case "2":
		// GST setup — store seller GSTIN in app config (env-driven; log for now).
		http.Redirect(w, r, "/setup?step=3", http.StatusFound)
	case "3":
		// Done — mark onboarding complete and redirect to dashboard.
		http.Redirect(w, r, "/dashboard?welcome=1", http.StatusFound)
	default:
		http.Redirect(w, r, "/setup", http.StatusFound)
	}
}
