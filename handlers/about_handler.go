package handlers

import "net/http"

func (a *App) About(w http.ResponseWriter, r *http.Request) {
	a.Renderer.Landing(w, "about.html", nil)
}

func (a *App) PricingPage(w http.ResponseWriter, r *http.Request) {
	a.Renderer.Landing(w, "pricing.html", nil)
}

func (a *App) FeaturesPage(w http.ResponseWriter, r *http.Request) {
	a.Renderer.Landing(w, "features.html", HomeData{
		Modules: []string{
			"Products", "Customers", "Vendors", "Categories",
			"Invoices", "Purchase Orders", "Payments", "Credit Notes",
			"Accounts", "Jobs", "Stock Logs", "Audit Logs", "Reports",
		},
	})
}
