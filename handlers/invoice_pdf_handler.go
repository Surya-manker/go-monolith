package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go-monolith/services"
)

func (a *App) InvoicePDF(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	// Load invoice record.
	_, inv, err := a.ModuleService.Get("invoices", id)
	if err != nil {
		http.Error(w, "invoice not found", http.StatusNotFound)
		return
	}

	// Load matching customer (best-effort; empty fields if not found).
	customer, _ := a.ModuleService.GetCustomerByName(inv["customer"])

	// Parse grand total.
	total, _ := strconv.ParseFloat(inv["total"], 64)

	// Parse created_at date — MySQL returns "2006-01-02 15:04:05" or similar.
	date := time.Now()
	for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02T15:04:05Z", time.RFC3339} {
		if t, err2 := time.Parse(layout, inv["created_at"]); err2 == nil {
			date = t
			break
		}
	}

	// Determine intra-state vs inter-state GST.
	buyerGSTIN := ""
	if customer != nil {
		buyerGSTIN = customer["gstin"]
	}
	isIntra := a.StateCode == "" ||
		buyerGSTIN == "" ||
		(len(buyerGSTIN) >= 2 && strings.HasPrefix(buyerGSTIN, a.StateCode))

	pdfData := services.InvoiceForPDF{
		Number:        inv["number"],
		Date:          date,
		Status:        inv["status"],
		SellerName:    a.SellerName,
		SellerGSTIN:   a.SellerGSTIN,
		SellerAddress: a.SellerAddress,
		GrandTotal:    total,
		TaxRate:       18.0,
		IsIntra:       isIntra,
	}
	if customer != nil {
		pdfData.BuyerName = customer["name"]
		pdfData.BuyerGSTIN = buyerGSTIN
		pdfData.BuyerPhone = customer["phone"]
		pdfData.BuyerEmail = customer["email"]
	}
	if pdfData.BuyerName == "" {
		pdfData.BuyerName = inv["customer"]
	}

	pdfBytes, err := services.GenerateInvoicePDF(pdfData)
	if err != nil {
		http.Error(w, "PDF generation failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	filename := fmt.Sprintf("invoice-%s.pdf", inv["number"])
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(pdfBytes)))
	w.Write(pdfBytes)
}
