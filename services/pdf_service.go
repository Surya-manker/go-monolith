package services

import (
	"bytes"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
)

type InvoiceForPDF struct {
	Number        string
	Date          time.Time
	Status        string
	SellerName    string
	SellerGSTIN   string
	SellerAddress string
	BuyerName     string
	BuyerGSTIN    string
	BuyerPhone    string
	BuyerEmail    string
	GrandTotal    float64
	TaxRate       float64 // e.g. 18.0
	IsIntra       bool    // true → CGST+SGST, false → IGST
}

func (inv *InvoiceForPDF) computeTax() (taxable, cgst, sgst, igst float64) {
	taxable = math.Round(inv.GrandTotal/(1+inv.TaxRate/100)*100) / 100
	totalTax := math.Round((inv.GrandTotal-taxable)*100) / 100
	if inv.IsIntra || inv.BuyerGSTIN == "" {
		cgst = math.Round(totalTax/2*100) / 100
		sgst = totalTax - cgst
	} else {
		igst = totalTax
	}
	return
}

// GenerateInvoicePDF returns a GST-compliant invoice as PDF bytes.
func GenerateInvoicePDF(inv InvoiceForPDF) ([]byte, error) {
	const (
		marginL = 15.0
		pageW   = 180.0 // A4 210 − 15 − 15
		bR, bG, bB = 47, 99, 136  // brand blue #2f6388
		hR, hG, hB = 235, 243, 249 // light tint
	)

	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(marginL, 15, 15)
	pdf.AddPage()

	// ── Title bar ────────────────────────────────────────────────────
	pdf.SetFillColor(bR, bG, bB)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Helvetica", "B", 18)
	pdf.CellFormat(pageW, 13, "TAX INVOICE", "", 1, "C", true, 0, "")
	pdf.Ln(4)

	// ── Seller (left) + Invoice meta (right) ─────────────────────────
	topY := pdf.GetY()
	lw := 100.0
	rw := pageW - lw
	rx := marginL + lw

	// Left: seller
	setFont := func(style string, size float64) {
		pdf.SetFont("Helvetica", style, size)
	}
	pdf.SetTextColor(80, 80, 80)
	setFont("B", 8)
	pdf.CellFormat(lw, 5, "FROM", "", 1, "L", false, 0, "")
	pdf.SetTextColor(0, 0, 0)
	setFont("B", 11)
	pdf.CellFormat(lw, 6, inv.SellerName, "", 1, "L", false, 0, "")
	setFont("", 9)
	if inv.SellerAddress != "" {
		pdf.CellFormat(lw, 5, inv.SellerAddress, "", 1, "L", false, 0, "")
	}
	if inv.SellerGSTIN != "" {
		setFont("B", 9)
		pdf.CellFormat(lw, 5, "GSTIN: "+inv.SellerGSTIN, "", 1, "L", false, 0, "")
	}
	leftEndY := pdf.GetY()

	// Right: invoice meta
	ry := topY
	rightRow := func(label, value string) {
		pdf.SetXY(rx, ry)
		pdf.SetTextColor(80, 80, 80)
		setFont("", 9)
		pdf.CellFormat(rw/2, 5.5, label, "", 0, "L", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
		setFont("B", 9)
		pdf.CellFormat(rw/2, 5.5, value, "", 0, "L", false, 0, "")
		ry += 5.5
	}
	rightRow("Invoice No:", inv.Number)
	rightRow("Date:", inv.Date.Format("02 Jan 2006"))
	rightRow("Status:", strings.ToUpper(inv.Status))

	// Move below both columns
	endY := leftEndY
	if ry > endY {
		endY = ry
	}
	pdf.SetXY(marginL, endY)
	pdf.Ln(5)

	// ── Divider ───────────────────────────────────────────────────────
	pdf.SetDrawColor(bR, bG, bB)
	pdf.SetLineWidth(0.4)
	pdf.Line(marginL, pdf.GetY(), marginL+pageW, pdf.GetY())
	pdf.Ln(5)

	// ── Buyer ─────────────────────────────────────────────────────────
	pdf.SetTextColor(80, 80, 80)
	setFont("B", 8)
	pdf.CellFormat(pageW, 5, "BILL TO", "", 1, "L", false, 0, "")
	pdf.SetTextColor(0, 0, 0)
	setFont("B", 11)
	pdf.CellFormat(pageW, 6, inv.BuyerName, "", 1, "L", false, 0, "")
	setFont("", 9)
	contact := ""
	if inv.BuyerPhone != "" {
		contact += "Ph: " + inv.BuyerPhone
	}
	if inv.BuyerEmail != "" {
		if contact != "" {
			contact += "   |   "
		}
		contact += inv.BuyerEmail
	}
	if contact != "" {
		pdf.CellFormat(pageW, 5, contact, "", 1, "L", false, 0, "")
	}
	if inv.BuyerGSTIN != "" {
		setFont("B", 9)
		pdf.CellFormat(pageW, 5, "GSTIN: "+inv.BuyerGSTIN, "", 1, "L", false, 0, "")
	}
	pdf.Ln(5)

	// ── Divider ───────────────────────────────────────────────────────
	pdf.Line(marginL, pdf.GetY(), marginL+pageW, pdf.GetY())
	pdf.Ln(5)

	// ── Items table ───────────────────────────────────────────────────
	taxable, cgst, sgst, igst := inv.computeTax()

	c1, c2, c3, c4, c5 := 76.0, 26.0, 22.0, 28.0, pageW-76-26-22-28
	tableHeader := func(labels []string, widths []float64) {
		pdf.SetFillColor(hR, hG, hB)
		pdf.SetTextColor(0, 0, 0)
		setFont("B", 9)
		for i, lbl := range labels {
			pdf.CellFormat(widths[i], 7, lbl, "1", 0, "C", true, 0, "")
		}
		pdf.Ln(7)
	}

	var headers []string
	var widths []float64
	if inv.IsIntra || inv.BuyerGSTIN == "" {
		headers = []string{"Description", "Taxable Amt (Rs.)", "GST Rate", "CGST (Rs.)", "SGST (Rs.)"}
		widths = []float64{c1, c2, c3, c4, c5}
	} else {
		headers = []string{"Description", "Taxable Amt (Rs.)", "GST Rate", "IGST (Rs.)"}
		widths = []float64{c1, c2, c3, c4 + c5}
	}
	tableHeader(headers, widths)

	// Item row
	pdf.SetTextColor(0, 0, 0)
	setFont("", 9)
	desc := fmt.Sprintf("Goods / Services - Invoice %s", inv.Number)
	pdf.CellFormat(c1, 7, desc, "1", 0, "L", false, 0, "")
	pdf.CellFormat(c2, 7, fmt.Sprintf("%.2f", taxable), "1", 0, "R", false, 0, "")
	pdf.CellFormat(c3, 7, fmt.Sprintf("%.0f%%", inv.TaxRate), "1", 0, "C", false, 0, "")
	if inv.IsIntra || inv.BuyerGSTIN == "" {
		pdf.CellFormat(c4, 7, fmt.Sprintf("%.2f", cgst), "1", 0, "R", false, 0, "")
		pdf.CellFormat(c5, 7, fmt.Sprintf("%.2f", sgst), "1", 0, "R", false, 0, "")
	} else {
		pdf.CellFormat(c4+c5, 7, fmt.Sprintf("%.2f", igst), "1", 0, "R", false, 0, "")
	}
	pdf.Ln(7)
	pdf.Ln(6)

	// ── Totals (right-aligned box) ────────────────────────────────────
	lbl := 55.0
	val := 35.0
	tx := marginL + pageW - lbl - val

	totRow := func(label, value string, fill bool) {
		pdf.SetX(tx)
		if fill {
			pdf.SetFillColor(bR, bG, bB)
			pdf.SetTextColor(255, 255, 255)
			setFont("B", 10)
			pdf.CellFormat(lbl, 8, label, "1", 0, "R", true, 0, "")
			pdf.CellFormat(val, 8, value, "1", 1, "R", true, 0, "")
		} else {
			pdf.SetFillColor(hR, hG, hB)
			pdf.SetTextColor(0, 0, 0)
			setFont("", 9)
			pdf.CellFormat(lbl, 6, label, "1", 0, "R", true, 0, "")
			setFont("B", 9)
			pdf.CellFormat(val, 6, value, "1", 1, "R", true, 0, "")
		}
	}

	totRow("Taxable Amount:", fmt.Sprintf("Rs. %.2f", taxable), false)
	if inv.IsIntra || inv.BuyerGSTIN == "" {
		totRow(fmt.Sprintf("CGST @ %.1f%%:", inv.TaxRate/2), fmt.Sprintf("Rs. %.2f", cgst), false)
		totRow(fmt.Sprintf("SGST @ %.1f%%:", inv.TaxRate/2), fmt.Sprintf("Rs. %.2f", sgst), false)
	} else {
		totRow(fmt.Sprintf("IGST @ %.0f%%:", inv.TaxRate), fmt.Sprintf("Rs. %.2f", igst), false)
	}
	totRow("Grand Total:", fmt.Sprintf("Rs. %.2f", inv.GrandTotal), true)

	// ── Footer ─────────────────────────────────────────────────────────
	pdf.SetTextColor(120, 120, 120)
	setFont("I", 8)
	pdf.Ln(10)
	pdf.CellFormat(pageW, 5, "This is a computer-generated invoice. No signature required.", "", 1, "C", false, 0, "")
	pdf.CellFormat(pageW, 5, "Thank you for your business!", "", 1, "C", false, 0, "")

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
