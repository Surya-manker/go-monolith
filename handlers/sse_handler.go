package handlers

import (
	"fmt"
	"net/http"
	"time"
)

// DashboardSSE streams live metric updates to the dashboard every 30 seconds.
// HTMX SSE extension listens for the "dashboard" event and swaps the metric values.
func (a *App) DashboardSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	send := func() {
		productCount, _ := a.ProductService.Count()
		lowStock, _ := a.ProductService.LowStockCount()
		counts, _ := a.ModuleService.Counts()
		totals, _ := a.ModuleService.Totals()
		pending, _ := a.ModuleService.PendingInvoicesTotal()

		data := fmt.Sprintf(
			`<span id="sse-products">%d</span>`+
				`<span id="sse-lowstock">%d</span>`+
				`<span id="sse-customers">%d</span>`+
				`<span id="sse-invoices">%d</span>`+
				`<span id="sse-invoice-total">%s</span>`+
				`<span id="sse-pending">Rs. %.2f</span>`,
			productCount, lowStock,
			counts["customers"], counts["invoices"],
			totals["invoice_total"], pending,
		)

		fmt.Fprintf(w, "event: dashboard\ndata: %s\n\n", data)
		flusher.Flush()
	}

	// Send immediately on connect.
	send()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			send()
		}
	}
}
