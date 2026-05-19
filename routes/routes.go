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

	// ── Health / readiness probes ─────────────────────────────────────────────
	mux.HandleFunc("GET /health", app.Health)
	mux.HandleFunc("GET /health/ready", app.Ready)

	// ── SEO ───────────────────────────────────────────────────────────────────
	mux.HandleFunc("GET /robots.txt", app.RobotsTxt)
	mux.HandleFunc("GET /sitemap.xml", app.SitemapXML)

	// ── Public pages ─────────────────────────────────────────────────────────
	mux.HandleFunc("GET /{$}", app.Home)
	mux.HandleFunc("GET /about", app.About)
	mux.HandleFunc("GET /contact", app.ContactPage)
	mux.HandleFunc("POST /contact", app.ContactPost)
	mux.HandleFunc("GET /generator", app.Generator)
	mux.HandleFunc("GET /pricing", app.PricingPage)
	mux.HandleFunc("GET /features", app.FeaturesPage)
	mux.HandleFunc("GET /demo", app.Demo)

	// ── Auth routes (public, no CSRF needed) ────────────────────────────────
	mux.HandleFunc("GET /login", app.LoginPage)
	mux.HandleFunc("POST /login", app.LoginPost)
	mux.HandleFunc("GET /register", app.RegisterPage)
	mux.HandleFunc("POST /register", app.RegisterPost)
	mux.HandleFunc("GET /logout", app.Logout)
	mux.HandleFunc("POST /logout", app.Logout)

	// ── Protected routes ────────────────────────────────────────────────────
	authMux := http.NewServeMux()

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
		"users", "payments", "credit-notes", "jobs",
	} {
		path := "/" + mod
		authMux.HandleFunc("GET "+path, app.ModuleIndex(mod))
		authMux.HandleFunc("POST "+path, app.ModuleCreate(mod))
		authMux.HandleFunc("GET "+path+"/edit", app.ModuleEdit(mod))
		authMux.HandleFunc("POST "+path+"/update", app.ModuleUpdate(mod))
		authMux.HandleFunc("POST "+path+"/delete", app.ModuleDelete(mod))
	}

	// ── Batch & Expiry ───────────────────────────────────────────────────────
	authMux.HandleFunc("GET /batches", app.BatchesIndex)
	authMux.HandleFunc("POST /batches/receive", app.BatchReceive)
	authMux.HandleFunc("GET /batches/edit", app.BatchEdit)
	authMux.HandleFunc("POST /batches/update", app.BatchUpdate)
	authMux.HandleFunc("GET /batches/expiry", app.BatchExpiryReport)
	authMux.HandleFunc("POST /batches/write-off", app.BatchWriteOff)
	authMux.HandleFunc("GET /batches/logs", app.BatchLogs)

	// ── Returns ──────────────────────────────────────────────────────────────
	authMux.HandleFunc("GET /returns", app.ReturnsIndex)
	authMux.HandleFunc("GET /returns/sales", app.SalesReturnsList)
	authMux.HandleFunc("GET /returns/sales/new", app.SalesReturnNewPage)
	authMux.HandleFunc("POST /returns/sales", app.SalesReturnCreate)
	authMux.HandleFunc("GET /returns/purchase", app.PurchaseReturnsList)
	authMux.HandleFunc("GET /returns/purchase/new", app.PurchaseReturnNewPage)
	authMux.HandleFunc("POST /returns/purchase", app.PurchaseReturnCreate)

	// ── Barcode system ───────────────────────────────────────────────────────
	authMux.HandleFunc("GET /barcodes", app.BarcodesIndex)
	authMux.HandleFunc("GET /barcodes/image", app.BarcodeImage)
	authMux.HandleFunc("POST /barcodes/auto-generate", app.BarcodeAutoGenerate)
	authMux.HandleFunc("GET /barcodes/labels", app.BarcodeLabels)
	authMux.HandleFunc("GET /barcodes/lookup", app.BarcodeLookup)

	// ── POS system ───────────────────────────────────────────────────────────
	authMux.HandleFunc("GET /pos", app.POSIndex)
	authMux.HandleFunc("GET /pos/search", app.POSSearch)
	authMux.HandleFunc("POST /pos/cart/add", app.POSCartAdd)
	authMux.HandleFunc("POST /pos/cart/update", app.POSCartUpdate)
	authMux.HandleFunc("POST /pos/cart/remove", app.POSCartRemove)
	authMux.HandleFunc("POST /pos/cart/clear", app.POSCartClear)
	authMux.HandleFunc("POST /pos/checkout", app.POSCheckout)
	authMux.HandleFunc("GET /pos/receipt", app.POSReceipt)
	authMux.HandleFunc("GET /pos/sales", app.POSSalesHistory)

	// ── Warehouses ───────────────────────────────────────────────────────────
	authMux.HandleFunc("GET /warehouses", app.WarehousesIndex)
	authMux.HandleFunc("POST /warehouses", app.WarehousesCreate)
	authMux.HandleFunc("GET /warehouses/edit", app.WarehousesEdit)
	authMux.HandleFunc("POST /warehouses/update", app.WarehousesUpdate)
	authMux.HandleFunc("POST /warehouses/delete", app.WarehousesDelete)
	authMux.HandleFunc("GET /warehouses/stock", app.WarehouseStockIndex)
	authMux.HandleFunc("POST /warehouses/stock/adjust", app.WarehouseStockAdjust)

	// ── Stock Transfers ──────────────────────────────────────────────────────
	authMux.HandleFunc("GET /transfers", app.TransfersIndex)
	authMux.HandleFunc("POST /transfers", app.TransfersCreate)

	// ── Finance ──────────────────────────────────────────────────────────────
	authMux.HandleFunc("GET /finance", app.FinanceDashboard)
	authMux.HandleFunc("GET /finance/ledger", app.FinanceLedger)
	authMux.HandleFunc("GET /finance/expenses", app.FinanceExpenses)
	authMux.HandleFunc("POST /finance/expenses", app.FinanceExpenseCreate)
	authMux.HandleFunc("POST /finance/expenses/approve", app.FinanceExpenseApprove)
	authMux.HandleFunc("POST /finance/expenses/reject", app.FinanceExpenseReject)
	authMux.HandleFunc("POST /finance/categories", app.FinanceCategoryCreate)
	authMux.HandleFunc("GET /finance/bank", app.FinanceBank)
	authMux.HandleFunc("POST /finance/bank", app.FinanceBankCreate)
	authMux.HandleFunc("POST /finance/bank/transaction", app.FinanceBankTransaction)
	authMux.HandleFunc("GET /finance/pl", app.FinancePL)
	authMux.HandleFunc("GET /finance/cashflow", app.FinanceCashflow)
	authMux.HandleFunc("GET /finance/gst", app.FinanceGST)
	authMux.HandleFunc("GET /finance/export", app.FinanceExport)

	// ── CRM ──────────────────────────────────────────────────────────────────
	authMux.HandleFunc("GET /crm", app.CRMDashboard)
	authMux.HandleFunc("GET /crm/customers", app.CRMCustomersIndex)
	authMux.HandleFunc("POST /crm/customers", app.CRMCustomerCreate)
	authMux.HandleFunc("GET /crm/customers/edit", app.CRMCustomerEdit)
	authMux.HandleFunc("POST /crm/customers/update", app.CRMCustomerUpdate)
	authMux.HandleFunc("GET /crm/customers/view", app.CRMCustomerView)

	authMux.HandleFunc("GET /crm/quotations", app.QuotationsIndex)
	authMux.HandleFunc("GET /crm/quotations/new", app.QuotationNew)
	authMux.HandleFunc("POST /crm/quotations", app.QuotationCreate)
	authMux.HandleFunc("GET /crm/quotations/view", app.QuotationView)
	authMux.HandleFunc("POST /crm/quotations/send", app.QuotationSend)
	authMux.HandleFunc("POST /crm/quotations/approve", app.QuotationApprove)
	authMux.HandleFunc("POST /crm/quotations/reject", app.QuotationReject)
	authMux.HandleFunc("POST /crm/quotations/convert", app.QuotationConvert)

	authMux.HandleFunc("GET /crm/orders", app.SalesOrdersIndex)
	authMux.HandleFunc("GET /crm/orders/new", app.SalesOrderNew)
	authMux.HandleFunc("POST /crm/orders", app.SalesOrderCreate)
	authMux.HandleFunc("GET /crm/orders/view", app.SalesOrderView)
	authMux.HandleFunc("POST /crm/orders/confirm", app.SalesOrderConfirm)
	authMux.HandleFunc("POST /crm/orders/pack", app.SalesOrderPack)
	authMux.HandleFunc("POST /crm/orders/cancel", app.SalesOrderCancel)

	authMux.HandleFunc("GET /crm/delivery", app.DeliveryIndex)
	authMux.HandleFunc("GET /crm/delivery/new", app.DeliveryNew)
	authMux.HandleFunc("POST /crm/delivery", app.DeliveryCreate)
	authMux.HandleFunc("GET /crm/delivery/view", app.DeliveryView)
	authMux.HandleFunc("POST /crm/delivery/delivered", app.DeliveryMarkDelivered)

	authMux.HandleFunc("GET /crm/payments", app.CRMPaymentsIndex)
	authMux.HandleFunc("POST /crm/payments", app.CRMPaymentCreate)

	// ── Procurement / Suppliers ───────────────────────────────────────────────
	authMux.HandleFunc("GET /suppliers", app.SuppliersIndex)
	authMux.HandleFunc("POST /suppliers", app.SuppliersCreate)
	authMux.HandleFunc("GET /suppliers/edit", app.SuppliersEdit)
	authMux.HandleFunc("POST /suppliers/update", app.SuppliersUpdate)
	authMux.HandleFunc("GET /suppliers/view", app.SupplierView)

	authMux.HandleFunc("GET /procurement", app.ProcurementDashboard)
	authMux.HandleFunc("GET /procurement/orders", app.POsIndex)
	authMux.HandleFunc("GET /procurement/orders/new", app.PONew)
	authMux.HandleFunc("POST /procurement/orders", app.POCreate)
	authMux.HandleFunc("GET /procurement/orders/view", app.POView)
	authMux.HandleFunc("POST /procurement/orders/submit", app.POSubmit)
	authMux.HandleFunc("POST /procurement/orders/approve", app.POApprove)
	authMux.HandleFunc("POST /procurement/orders/cancel", app.POCancel)

	authMux.HandleFunc("GET /procurement/grn", app.GRNIndex)
	authMux.HandleFunc("GET /procurement/grn/new", app.GRNNew)
	authMux.HandleFunc("POST /procurement/grn", app.GRNCreate)
	authMux.HandleFunc("GET /procurement/grn/view", app.GRNView)

	authMux.HandleFunc("GET /procurement/payments", app.PaymentsIndex)
	authMux.HandleFunc("POST /procurement/payments", app.PaymentCreate)

	authMux.HandleFunc("GET /procurement/reorder", app.ReorderSuggestions)

	// ── Reports ──────────────────────────────────────────────────────────────
	authMux.HandleFunc("GET /reports", app.ReportsHub)
	authMux.HandleFunc("GET /reports/stock-valuation", app.ReportStockValuation)
	authMux.HandleFunc("GET /reports/warehouse-inventory", app.ReportWarehouseInventory)
	authMux.HandleFunc("GET /reports/stock-movement", app.ReportStockMovement)
	authMux.HandleFunc("GET /reports/dead-stock", app.ReportDeadStock)
	authMux.HandleFunc("GET /reports/low-stock", app.ReportLowStock)
	authMux.HandleFunc("GET /reports/sales", app.ReportSales)
	authMux.HandleFunc("GET /reports/returns", app.ReportReturns)
	authMux.HandleFunc("GET /reports/export", app.ReportExport)

	authMux.HandleFunc("GET /search", app.Search)
	authMux.HandleFunc("GET /sse/dashboard", app.DashboardSSE)
	authMux.HandleFunc("GET /notifications", app.NotificationsPanel)
	authMux.HandleFunc("GET /notifications/badge", app.NotificationsBadge)
	authMux.HandleFunc("POST /notifications/read", app.NotificationMarkRead)
	authMux.HandleFunc("GET /stock-logs", app.StockLogs)
	authMux.HandleFunc("GET /audit-logs", app.AuditLogs)
	authMux.HandleFunc("GET /invoices/pdf", app.InvoicePDF)

	authMux.HandleFunc("GET /profile", app.ProfilePage)
	authMux.HandleFunc("POST /profile", app.ProfileUpdate)

	// Onboarding wizard + checklist
	authMux.HandleFunc("GET /setup", app.OnboardingPage)
	authMux.HandleFunc("POST /setup", app.OnboardingPost)
	authMux.HandleFunc("GET /checklist", app.ChecklistPage)

	// Admin-only: user management with password reset
	adminOnly := middleware.RequireRole("admin", "super_admin")
	authMux.Handle("GET /admin/users", adminOnly(http.HandlerFunc(app.AdminUsersPage)))
	authMux.Handle("GET /admin/users/edit", adminOnly(http.HandlerFunc(app.AdminUserEditPage)))
	authMux.Handle("POST /admin/users/update", adminOnly(http.HandlerFunc(app.AdminUserUpdate)))

	for _, mod := range []string{
		"customers", "categories", "vendors", "invoices", "purchase-orders",
		"users", "payments", "credit-notes", "jobs",
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
		"payments", "credit-notes", "jobs",
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

	// Global middleware stack (outermost → innermost):
	//   Recovery → RequestID → Logger → Security → RateLimit → handler
	rl := middleware.NewRateLimiter(60, 120)
	return middleware.Recovery(
		middleware.RequestID(
			middleware.Logger(
				middleware.Security(
					rl.Middleware(mux),
				),
			),
		),
	)
}
