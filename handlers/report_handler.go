package handlers

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go-monolith/models"
	"go-monolith/services"

	"github.com/go-pdf/fpdf"
)

// parseFilter reads common filter params from the request.
func parseFilter(r *http.Request) services.ReportFilter {
	f := services.ReportFilter{}
	f.From = r.URL.Query().Get("from")
	f.To = r.URL.Query().Get("to")
	f.WarehouseID, _ = strconv.Atoi(r.URL.Query().Get("warehouse_id"))
	f.ProductID, _ = strconv.Atoi(r.URL.Query().Get("product_id"))
	if d, _ := strconv.Atoi(r.URL.Query().Get("days")); d > 0 {
		f.Days = d
	}
	f.Defaults()
	return f
}

// ── Reports hub ───────────────────────────────────────────────────────────────

type ReportsHubData struct {
	AppContext
	Analytics  services.DashboardAnalytics
	Warehouses []models.Warehouse
	// Chart data serialized as template.JS for safe embedding
	SalesChartJSON    template.JS
	TopProductsJSON   template.JS
	PaymentChartJSON  template.JS
}

func (a *App) ReportsHub(w http.ResponseWriter, r *http.Request) {
	bizID := a.bizID(r)
	analytics, _ := a.ReportService.DashboardAnalytics(bizID)
	whs, _ := a.WarehouseService.List(bizID)

	salesJSON, _ := json.Marshal(analytics.DailySalesChart)
	topJSON, _ := json.Marshal(analytics.TopProducts)
	payJSON, _ := json.Marshal(analytics.PaymentMethods)

	a.Renderer.Page(w, "reports_hub.html", ReportsHubData{
		AppContext:        a.ctx(r),
		Analytics:         analytics,
		Warehouses:        whs,
		SalesChartJSON:    template.JS(salesJSON),
		TopProductsJSON:   template.JS(topJSON),
		PaymentChartJSON:  template.JS(payJSON),
	})
}

// ── Stock Valuation ───────────────────────────────────────────────────────────

type StockValuationData struct {
	AppContext
	Rows       []services.StockValuationRow
	Summary    services.StockSummary
	Warehouses []models.Warehouse
	Filter     services.ReportFilter
}

func (a *App) ReportStockValuation(w http.ResponseWriter, r *http.Request) {
	bizID := a.bizID(r)
	f := parseFilter(r)
	rows, summary, _ := a.ReportService.StockValuation(bizID, f.WarehouseID)
	whs, _ := a.WarehouseService.List(bizID)
	a.Renderer.Page(w, "report_stock_valuation.html", StockValuationData{
		AppContext: a.ctx(r), Rows: rows, Summary: summary, Warehouses: whs, Filter: f,
	})
}

// ── Warehouse Inventory ───────────────────────────────────────────────────────

type WarehouseInventoryData struct {
	AppContext
	Rows   []services.WarehouseInventoryRow
	Filter services.ReportFilter
}

func (a *App) ReportWarehouseInventory(w http.ResponseWriter, r *http.Request) {
	bizID := a.bizID(r)
	f := parseFilter(r)
	rows, _ := a.ReportService.WarehouseInventory(bizID)
	a.Renderer.Page(w, "report_warehouse_inventory.html", WarehouseInventoryData{
		AppContext: a.ctx(r), Rows: rows, Filter: f,
	})
}

// ── Stock Movement ────────────────────────────────────────────────────────────

type StockMovementData struct {
	AppContext
	Rows       []services.StockMovementRow
	Warehouses []models.Warehouse
	Products   []models.Product
	Filter     services.ReportFilter
}

func (a *App) ReportStockMovement(w http.ResponseWriter, r *http.Request) {
	bizID := a.bizID(r)
	f := parseFilter(r)
	rows, _ := a.ReportService.StockMovement(bizID, f)
	whs, _ := a.WarehouseService.List(bizID)
	products, _ := a.ProductService.List("", bizID)
	a.Renderer.Page(w, "report_stock_movement.html", StockMovementData{
		AppContext: a.ctx(r), Rows: rows, Warehouses: whs, Products: products, Filter: f,
	})
}

// ── Dead Stock ────────────────────────────────────────────────────────────────

type DeadStockData struct {
	AppContext
	Rows   []services.DeadStockRow
	Filter services.ReportFilter
}

func (a *App) ReportDeadStock(w http.ResponseWriter, r *http.Request) {
	bizID := a.bizID(r)
	f := parseFilter(r)
	rows, _ := a.ReportService.DeadStock(bizID, f.Days)
	a.Renderer.Page(w, "report_dead_stock.html", DeadStockData{
		AppContext: a.ctx(r), Rows: rows, Filter: f,
	})
}

// ── Low Stock ─────────────────────────────────────────────────────────────────

type LowStockData struct {
	AppContext
	Rows []services.LowStockRow
}

func (a *App) ReportLowStock(w http.ResponseWriter, r *http.Request) {
	bizID := a.bizID(r)
	rows, _ := a.ReportService.LowStock(bizID)
	a.Renderer.Page(w, "report_low_stock.html", LowStockData{AppContext: a.ctx(r), Rows: rows})
}

// ── Sales Report ──────────────────────────────────────────────────────────────

type SalesReportData struct {
	AppContext
	Summary        services.SalesSummary
	DailyRows      []services.DailySalesRow
	ProductRows    []services.ProductSalesRow
	PaymentRows    []services.PaymentMethodRow
	Warehouses     []models.Warehouse
	Filter         services.ReportFilter
	SalesChartJSON template.JS
	TopProdJSON    template.JS
	PayChartJSON   template.JS
}

func (a *App) ReportSales(w http.ResponseWriter, r *http.Request) {
	bizID := a.bizID(r)
	f := parseFilter(r)
	summary, _ := a.ReportService.SalesSummaryData(bizID, f)
	daily, _ := a.ReportService.DailySales(bizID, f)
	products, _ := a.ReportService.ProductSales(bizID, f)
	payments, _ := a.ReportService.PaymentMethods(bizID, f)
	whs, _ := a.WarehouseService.List(bizID)

	// Build chart data
	chartPts := make([]services.ChartPoint, 0, len(daily))
	for i := len(daily) - 1; i >= 0; i-- {
		chartPts = append(chartPts, services.ChartPoint{Label: daily[i].Date, Value: daily[i].Revenue})
	}
	top5 := products
	if len(top5) > 10 {
		top5 = top5[:10]
	}
	salesJSON, _ := json.Marshal(chartPts)
	topJSON, _ := json.Marshal(top5)
	payJSON, _ := json.Marshal(payments)

	a.Renderer.Page(w, "report_sales.html", SalesReportData{
		AppContext:      a.ctx(r),
		Summary:         summary,
		DailyRows:       daily,
		ProductRows:     products,
		PaymentRows:     payments,
		Warehouses:      whs,
		Filter:          f,
		SalesChartJSON:  template.JS(salesJSON),
		TopProdJSON:     template.JS(topJSON),
		PayChartJSON:    template.JS(payJSON),
	})
}

// ── Returns Analytics ─────────────────────────────────────────────────────────

type ReturnsReportData struct {
	AppContext
	Report services.ReturnsReport
	Filter services.ReportFilter
}

func (a *App) ReportReturns(w http.ResponseWriter, r *http.Request) {
	bizID := a.bizID(r)
	f := parseFilter(r)
	report, _ := a.ReportService.ReturnsAnalytics(bizID, f)
	a.Renderer.Page(w, "report_returns.html", ReturnsReportData{
		AppContext: a.ctx(r), Report: report, Filter: f,
	})
}

// ── CSV Export ────────────────────────────────────────────────────────────────
// GET /reports/export?type=<report>&format=csv&from=...&to=...

func (a *App) ReportExport(w http.ResponseWriter, r *http.Request) {
	bizID := a.bizID(r)
	rtype := r.URL.Query().Get("type")
	format := r.URL.Query().Get("format")
	f := parseFilter(r)

	switch format {
	case "pdf":
		a.exportPDF(w, bizID, rtype, f)
	default:
		a.exportCSV(w, bizID, rtype, f)
	}
}

func (a *App) exportCSV(w http.ResponseWriter, bizID int, rtype string, f services.ReportFilter) {
	var headers []string
	var rows [][]string
	filename := rtype + "_" + time.Now().Format("20060102") + ".csv"

	switch rtype {
	case "stock-valuation":
		data, _, _ := a.ReportService.StockValuation(bizID, f.WarehouseID)
		headers = []string{"Product", "SKU", "Barcode", "Brand", "Unit", "Qty", "Cost Price", "Sale Price", "Cost Value", "Sale Value", "Profit Value"}
		for _, r := range data {
			rows = append(rows, []string{
				r.ProductName, r.SKU, r.Barcode, r.Brand, r.Unit,
				strconv.Itoa(r.TotalQty),
				fmt.Sprintf("%.2f", r.CostPrice), fmt.Sprintf("%.2f", r.SalePrice),
				fmt.Sprintf("%.2f", r.CostValue), fmt.Sprintf("%.2f", r.SaleValue), fmt.Sprintf("%.2f", r.ProfitValue),
			})
		}

	case "stock-movement":
		data, _ := a.ReportService.StockMovement(bizID, f)
		headers = []string{"Product", "SKU", "Warehouse", "Type", "Before", "Change", "After", "Note", "Date"}
		for _, r := range data {
			rows = append(rows, []string{
				r.ProductName, r.SKU, r.WarehouseName, r.ChangeType,
				strconv.Itoa(r.QuantityBefore), strconv.Itoa(r.QuantityChange), strconv.Itoa(r.QuantityAfter),
				r.Note, r.CreatedAt.Format("2006-01-02 15:04:05"),
			})
		}

	case "sales":
		data, _ := a.ReportService.DailySales(bizID, f)
		headers = []string{"Date", "Sales Count", "Revenue", "Tax", "Discount", "Net Revenue"}
		for _, r := range data {
			rows = append(rows, []string{
				r.Date, strconv.Itoa(r.SaleCount),
				fmt.Sprintf("%.2f", r.Revenue), fmt.Sprintf("%.2f", r.TaxTotal),
				fmt.Sprintf("%.2f", r.Discount), fmt.Sprintf("%.2f", r.NetRevenue),
			})
		}

	case "product-sales":
		data, _ := a.ReportService.ProductSales(bizID, f)
		headers = []string{"Product", "SKU", "Qty Sold", "Revenue", "Tax", "Net Revenue"}
		for _, r := range data {
			rows = append(rows, []string{
				r.ProductName, r.SKU, strconv.Itoa(r.TotalQty),
				fmt.Sprintf("%.2f", r.Revenue), fmt.Sprintf("%.2f", r.TaxAmount), fmt.Sprintf("%.2f", r.NetRevenue),
			})
		}

	case "dead-stock":
		data, _ := a.ReportService.DeadStock(bizID, f.Days)
		headers = []string{"Product", "SKU", "Warehouse", "Qty", "Cost Value", "Last Movement", "Days Stale"}
		for _, r := range data {
			lastMove := "Never"
			if r.LastMovement != nil {
				lastMove = r.LastMovement.Format("2006-01-02")
			}
			rows = append(rows, []string{
				r.ProductName, r.SKU, r.WarehouseName, strconv.Itoa(r.Quantity),
				fmt.Sprintf("%.2f", r.CostValue), lastMove, strconv.Itoa(r.DaysSince),
			})
		}

	case "low-stock":
		data, _ := a.ReportService.LowStock(bizID)
		headers = []string{"Product", "SKU", "Current Stock", "Threshold", "Short By"}
		for _, r := range data {
			rows = append(rows, []string{
				r.ProductName, r.SKU, strconv.Itoa(r.CurrentStock),
				strconv.Itoa(r.LowStockThreshold), strconv.Itoa(r.ShortBy),
			})
		}

	case "warehouse-inventory":
		data, _ := a.ReportService.WarehouseInventory(bizID)
		headers = []string{"Warehouse", "Product", "SKU", "Qty", "Cost Value"}
		for _, r := range data {
			rows = append(rows, []string{
				r.WarehouseName, r.ProductName, r.SKU,
				strconv.Itoa(r.Quantity), fmt.Sprintf("%.2f", r.CostValue),
			})
		}

	default:
		http.Error(w, "unknown report type", http.StatusBadRequest)
		return
	}

	writeCSV(w, filename, headers, rows)
}

func writeCSV(w http.ResponseWriter, filename string, headers []string, rows [][]string) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	cw := csv.NewWriter(w)
	_ = cw.Write(headers)
	for _, row := range rows {
		_ = cw.Write(row)
	}
	cw.Flush()
}

// ── PDF Export ────────────────────────────────────────────────────────────────

func (a *App) exportPDF(w http.ResponseWriter, bizID int, rtype string, f services.ReportFilter) {
	pdf := fpdf.New("L", "mm", "A4", "")
	pdf.SetFont("Arial", "B", 14)
	pdf.AddPage()

	title := strings.ToUpper(rtype[:1]) + rtype[1:] + " Report"
	title = strings.ReplaceAll(title, "-", " ")
	subtitle := fmt.Sprintf("From: %s  To: %s  Generated: %s", f.From, f.To, time.Now().Format("02 Jan 2006 15:04"))

	// Header
	pdf.SetFillColor(37, 99, 235)
	pdf.SetTextColor(255, 255, 255)
	pdf.CellFormat(0, 12, "  "+title, "", 1, "L", true, 0, "")
	pdf.SetFontSize(9)
	pdf.SetTextColor(100, 100, 100)
	pdf.SetFillColor(255, 255, 255)
	pdf.CellFormat(0, 7, "  "+subtitle, "", 1, "L", false, 0, "")
	pdf.Ln(4)

	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(0, 0, 0)

	switch rtype {
	case "stock-valuation":
		data, summary, _ := a.ReportService.StockValuation(bizID, f.WarehouseID)
		headers := []string{"Product", "SKU", "Qty", "Cost", "Sale", "Cost Value", "Sale Value"}
		widths := []float64{55, 30, 15, 20, 20, 25, 25}
		renderPDFTable(pdf, headers, widths, func(addRow func([]string)) {
			for _, r := range data {
				addRow([]string{
					r.ProductName, r.SKU, strconv.Itoa(r.TotalQty),
					fmt.Sprintf("%.2f", r.CostPrice), fmt.Sprintf("%.2f", r.SalePrice),
					fmt.Sprintf("%.2f", r.CostValue), fmt.Sprintf("%.2f", r.SaleValue),
				})
			}
		})
		pdf.Ln(4)
		pdf.SetFont("Arial", "B", 9)
		pdf.CellFormat(0, 7, fmt.Sprintf("TOTAL: %d products | Cost Value: Rs. %.2f | Sale Value: Rs. %.2f",
			summary.TotalProducts, summary.TotalCostValue, summary.TotalSaleValue), "", 1, "R", false, 0, "")

	case "sales":
		data, _ := a.ReportService.DailySales(bizID, f)
		headers := []string{"Date", "Sales", "Revenue", "Tax", "Discount", "Net"}
		widths := []float64{35, 20, 35, 30, 30, 35}
		renderPDFTable(pdf, headers, widths, func(addRow func([]string)) {
			for _, r := range data {
				addRow([]string{
					r.Date, strconv.Itoa(r.SaleCount),
					fmt.Sprintf("Rs. %.2f", r.Revenue), fmt.Sprintf("Rs. %.2f", r.TaxTotal),
					fmt.Sprintf("Rs. %.2f", r.Discount), fmt.Sprintf("Rs. %.2f", r.NetRevenue),
				})
			}
		})

	case "dead-stock":
		data, _ := a.ReportService.DeadStock(bizID, f.Days)
		headers := []string{"Product", "SKU", "Warehouse", "Qty", "Cost Value", "Days Stale"}
		widths := []float64{60, 30, 40, 15, 30, 25}
		renderPDFTable(pdf, headers, widths, func(addRow func([]string)) {
			for _, r := range data {
				lastMove := "Never"
				if r.LastMovement != nil {
					lastMove = fmt.Sprintf("%d days", r.DaysSince)
				}
				addRow([]string{r.ProductName, r.SKU, r.WarehouseName,
					strconv.Itoa(r.Quantity), fmt.Sprintf("Rs.%.2f", r.CostValue), lastMove})
			}
		})

	default:
		http.Error(w, "PDF not supported for this report type", http.StatusBadRequest)
		return
	}

	filename := rtype + "_" + time.Now().Format("20060102") + ".pdf"
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		http.Error(w, "PDF generation failed", http.StatusInternalServerError)
		return
	}
	w.Write(buf.Bytes())
}

func renderPDFTable(pdf *fpdf.Fpdf, headers []string, widths []float64, fillRows func(func([]string))) {
	pdf.SetFont("Arial", "B", 8)
	pdf.SetFillColor(243, 244, 246)
	pdf.SetTextColor(0, 0, 0)
	for i, h := range headers {
		pdf.CellFormat(widths[i], 7, " "+h, "1", 0, "L", true, 0, "")
	}
	pdf.Ln(-1)

	pdf.SetFont("Arial", "", 8)
	row := 0
	fillRows(func(cells []string) {
		fill := row%2 == 1
		if fill {
			pdf.SetFillColor(249, 250, 251)
		} else {
			pdf.SetFillColor(255, 255, 255)
		}
		for i, cell := range cells {
			if i < len(widths) {
				pdf.CellFormat(widths[i], 6, " "+cell, "1", 0, "L", true, 0, "")
			}
		}
		pdf.Ln(-1)
		row++
	})
}
