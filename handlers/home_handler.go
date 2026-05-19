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
			// Billing & Sales
			"GST Invoices (PDF)", "Credit Notes", "Payments Tracking", "Invoice Email",
			// Inventory
			"Products & Catalogue", "Warehouse Management", "Stock Transfers", "Barcode Generator",
			"Batch & Lot Tracking", "Expiry Date Alerts", "Stock Adjustments", "Dead Stock Reports",
			// CRM
			"CRM Dashboard", "Customer Profiles", "Quotations", "Sales Orders",
			"Delivery Challans", "Customer Payments",
			// Procurement
			"Supplier Management", "Purchase Orders", "Goods Receipt Note (GRN)", "Supplier Payments", "Reorder Alerts",
			// Finance
			"Expense Tracking", "Bank Accounts", "Cash Ledger", "Profit & Loss",
			"Cashflow Statement", "GST Summary Report",
			// POS
			"POS Terminal", "POS Sales History", "POS Receipt Printing",
			// Returns
			"Sales Returns", "Purchase Returns",
			// Reports
			"Stock Valuation", "Stock Movement", "Sales Analytics", "Low Stock Report",
			"Warehouse Inventory", "Returns Analytics",
			// Platform
			"Role-Based Access (5 roles)", "Audit Log", "REST API", "Dark Mode",
			"Live Dashboard (SSE)", "Multi-Warehouse", "PWA (Installable App)", "Backup & Restore",
		},
	})
}
