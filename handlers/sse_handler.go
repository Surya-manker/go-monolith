package handlers

import (
	"fmt"
	"net/http"
	"time"
)

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

	bizID := a.bizID(r)

	send := func() {
		productCount, _ := a.ProductService.Count(bizID)
		lowStock, _ := a.ProductService.LowStockCount(bizID)
		counts, _ := a.moduleService(r).Counts(bizID)
		totals, _ := a.moduleService(r).Totals(bizID)
		pending, _ := a.moduleService(r).PendingInvoicesTotal(bizID)

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
