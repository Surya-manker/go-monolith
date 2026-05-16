package routes

import (
	"net/http"

	"go-monolith/handlers"
	"go-monolith/middleware"
)

func New(app *handlers.App) http.Handler {
	mux := http.NewServeMux()

	// Static assets — no auth required.
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// ── Auth routes (public, no CSRF needed) ────────────────────────────────
	mux.HandleFunc("GET /login", app.LoginPage)
	mux.HandleFunc("POST /login", app.LoginPost)
	mux.HandleFunc("GET /register", app.RegisterPage)
	mux.HandleFunc("POST /register", app.RegisterPost)
	mux.HandleFunc("GET /logout", app.Logout)
	mux.HandleFunc("POST /logout", app.Logout)

	// ── Protected routes ────────────────────────────────────────────────────
	authMux := http.NewServeMux()

	authMux.HandleFunc("GET /", app.Dashboard)
	authMux.HandleFunc("GET /dashboard", app.Dashboard)

	authMux.HandleFunc("GET /products", app.ProductsIndex)
	authMux.HandleFunc("POST /products", app.ProductsCreate)
	authMux.HandleFunc("GET /products/edit", app.ProductsEdit)
	authMux.HandleFunc("POST /products/update", app.ProductsUpdate)
	authMux.HandleFunc("POST /products/delete", app.ProductsDelete)
	authMux.HandleFunc("GET /products/stock", app.ProductsStockForm)
	authMux.HandleFunc("POST /products/stock", app.ProductsAdjustStock)

	for _, mod := range []string{
		"customers", "categories", "vendors", "invoices", "purchase-orders",
		"users", "payments", "credit-notes", "jobs", "accounts",
	} {
		path := "/" + mod
		authMux.HandleFunc("GET "+path, app.ModuleIndex(mod))
		authMux.HandleFunc("POST "+path, app.ModuleCreate(mod))
		authMux.HandleFunc("GET "+path+"/edit", app.ModuleEdit(mod))
		authMux.HandleFunc("POST "+path+"/update", app.ModuleUpdate(mod))
		authMux.HandleFunc("POST "+path+"/delete", app.ModuleDelete(mod))
	}

	authMux.HandleFunc("GET /search", app.Search)
	authMux.HandleFunc("GET /sse/dashboard", app.DashboardSSE)
	authMux.HandleFunc("GET /notifications", app.NotificationsPanel)
	authMux.HandleFunc("GET /notifications/badge", app.NotificationsBadge)
	authMux.HandleFunc("POST /notifications/read", app.NotificationMarkRead)
	authMux.HandleFunc("GET /stock-logs", app.StockLogs)
	authMux.HandleFunc("GET /reports", app.Reports)
	authMux.HandleFunc("GET /audit-logs", app.AuditLogs)
	authMux.HandleFunc("GET /invoices/pdf", app.InvoicePDF)

	authMux.HandleFunc("GET /profile", app.ProfilePage)
	authMux.HandleFunc("POST /profile", app.ProfileUpdate)

	// Admin-only: user management with password reset
	adminOnly := middleware.RequireRole("admin", "super_admin")
	authMux.Handle("GET /admin/users", adminOnly(http.HandlerFunc(app.AdminUsersPage)))
	authMux.Handle("GET /admin/users/edit", adminOnly(http.HandlerFunc(app.AdminUserEditPage)))
	authMux.Handle("POST /admin/users/update", adminOnly(http.HandlerFunc(app.AdminUserUpdate)))

	for _, mod := range []string{
		"customers", "categories", "vendors", "invoices", "purchase-orders",
		"users", "payments", "credit-notes", "jobs", "accounts",
	} {
		path := "/" + mod
		authMux.HandleFunc("GET "+path+"/trash", app.ModuleTrash(mod))
		authMux.HandleFunc("POST "+path+"/restore", app.ModuleRestore(mod))
		authMux.HandleFunc("POST "+path+"/purge", app.ModulePurge(mod))
	}

	// ── REST API (JSON) — auth required, CSRF not needed for API clients ──
	apiMux := http.NewServeMux()
	apiMux.HandleFunc("GET /api/v1/me",      app.APIMe)
	apiMux.HandleFunc("GET /api/v1/stats",   app.APIStats)
	apiMux.HandleFunc("GET /api/v1/search",  app.APISearch)

	apiMux.HandleFunc("GET /api/v1/products",     app.APIProducts)
	apiMux.HandleFunc("POST /api/v1/products",    app.APIProducts)
	apiMux.HandleFunc("GET /api/v1/products/{id}", app.APIProduct)
	apiMux.HandleFunc("PUT /api/v1/products/{id}", app.APIProduct)
	apiMux.HandleFunc("DELETE /api/v1/products/{id}", app.APIProduct)

	for _, mod := range []string{
		"customers", "categories", "vendors", "invoices", "purchase-orders",
		"payments", "credit-notes", "jobs", "accounts",
	} {
		path := "/api/v1/" + mod
		apiMux.HandleFunc("GET "+path,        app.APIModule(mod))
		apiMux.HandleFunc("POST "+path,       app.APIModule(mod))
		apiMux.HandleFunc("GET "+path+"/{id}",    app.APIModuleRecord(mod))
		apiMux.HandleFunc("PUT "+path+"/{id}",    app.APIModuleRecord(mod))
		apiMux.HandleFunc("DELETE "+path+"/{id}", app.APIModuleRecord(mod))
	}

	// API routes: auth via session cookie, no CSRF (API clients don't have cookies for CSRF).
	apiHandler := middleware.Auth(app.AuthService)(apiMux)
	mux.Handle("/api/", apiHandler)

	// Apply auth + CSRF middleware to all protected HTML routes.
	authHandler := middleware.Auth(app.AuthService)(middleware.CSRF(authMux))

	// Route: anything not matched above goes to the protected mux.
	mux.Handle("/", authHandler)

	// Global middleware: security headers + rate limiting (60 rps, burst 120).
	rl := middleware.NewRateLimiter(60, 120)
	return middleware.Security(rl.Middleware(mux))
}
