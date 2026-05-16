package handlers

import (
	"fmt"
	"net/http"

	"go-monolith/models"
)

type DashboardData struct {
	AppContext
	ProductCount         int
	LowStockCount        int
	LowStockItems        []ProductView
	Counts               map[string]int
	InvoiceTotal         string
	PurchaseTotal        string
	PendingInvoiceTotal  string
	RecentActivity       []models.Record
	TopCustomers         []models.Record
}

func (a *App) Dashboard(w http.ResponseWriter, r *http.Request) {
	count, err := a.ProductService.Count()
	if err != nil {
		http.Error(w, "dashboard error", http.StatusInternalServerError)
		return
	}
	lowCount, err := a.ProductService.LowStockCount()
	if err != nil {
		http.Error(w, "dashboard error", http.StatusInternalServerError)
		return
	}
	lowStock, err := a.ProductService.LowStock(5)
	if err != nil {
		http.Error(w, "dashboard error", http.StatusInternalServerError)
		return
	}
	counts, err := a.ModuleService.Counts()
	if err != nil {
		http.Error(w, "dashboard error", http.StatusInternalServerError)
		return
	}
	totals, err := a.ModuleService.Totals()
	if err != nil {
		http.Error(w, "dashboard error", http.StatusInternalServerError)
		return
	}
	pending, _ := a.ModuleService.PendingInvoicesTotal()
	activity, _ := a.ModuleService.RecentActivity(8)
	topCustomers, _ := a.ModuleService.TopCustomers(5)

	a.Renderer.Page(w, "dashboard.html", DashboardData{
		AppContext:          a.ctx(r),
		ProductCount:        count,
		LowStockCount:       lowCount,
		LowStockItems:       productViews(lowStock),
		Counts:              counts,
		InvoiceTotal:        totals["invoice_total"],
		PurchaseTotal:       totals["po_total"],
		PendingInvoiceTotal: fmt.Sprintf("Rs. %.2f", pending),
		RecentActivity:      activity,
		TopCustomers:        topCustomers,
	})
}
