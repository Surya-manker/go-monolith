package handlers

import (
	"net/http"

	"go-monolith/middleware"
)

type HomeData struct {
	Modules []string
}

func (a *App) Home(w http.ResponseWriter, r *http.Request) {
	// Already logged in → go straight to dashboard.
	if user := middleware.UserFromContext(r.Context()); user != nil {
		http.Redirect(w, r, "/dashboard", http.StatusFound)
		return
	}
	// Try session without hard redirect.
	if u, err := a.AuthService.GetUserFromRequest(r); err == nil && u != nil {
		http.Redirect(w, r, "/dashboard", http.StatusFound)
		return
	}

	a.Renderer.Landing(w, "home.html", HomeData{
		Modules: []string{
			"Products", "Customers", "Vendors", "Categories",
			"Invoices", "Purchase Orders", "Payments", "Credit Notes",
			"Accounts", "Jobs", "Stock Logs", "Audit Logs", "Reports",
		},
	})
}
