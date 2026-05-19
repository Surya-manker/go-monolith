package services

import (
	"database/sql"
	"fmt"
	"time"
)

// ── Filter ────────────────────────────────────────────────────────────────────

type ReportFilter struct {
	From        string // YYYY-MM-DD
	To          string // YYYY-MM-DD
	WarehouseID int
	ProductID   int
	Days        int // for dead stock / aging
}

func (f *ReportFilter) Defaults() {
	if f.From == "" {
		f.From = time.Now().AddDate(0, -1, 0).Format("2006-01-02")
	}
	if f.To == "" {
		f.To = time.Now().Format("2006-01-02")
	}
	if f.Days <= 0 {
		f.Days = 30
	}
}

func (f ReportFilter) ToDateTime() (string, string) {
	return f.From + " 00:00:00", f.To + " 23:59:59"
}

// ── Row types ─────────────────────────────────────────────────────────────────

type StockValuationRow struct {
	ProductID   int
	ProductName string
	SKU         string
	Barcode     string
	Brand       string
	Unit        string
	TotalQty    int
	CostPrice   float64
	SalePrice   float64
	CostValue   float64
	SaleValue   float64
	ProfitValue float64
}

type WarehouseInventoryRow struct {
	WarehouseID   int
	WarehouseName string
	ProductName   string
	SKU           string
	Quantity      int
	CostValue     float64
}

type StockMovementRow struct {
	ProductName    string
	SKU            string
	WarehouseName  string
	ChangeType     string
	QuantityBefore int
	QuantityChange int
	QuantityAfter  int
	Note           string
	CreatedAt      time.Time
}

type DeadStockRow struct {
	ProductID     int
	ProductName   string
	SKU           string
	WarehouseName string
	Quantity      int
	CostValue     float64
	LastMovement  *time.Time
	DaysSince     int
}

type LowStockRow struct {
	ProductID         int
	ProductName       string
	SKU               string
	CurrentStock      int
	LowStockThreshold int
	ShortBy           int
}

type DailySalesRow struct {
	Date         string
	SaleCount    int
	Revenue      float64
	TaxTotal     float64
	Discount     float64
	NetRevenue   float64
}

type ProductSalesRow struct {
	ProductName string
	SKU         string
	TotalQty    int
	Revenue     float64
	TaxAmount   float64
	NetRevenue  float64
}

type PaymentMethodRow struct {
	Method  string
	Count   int
	Total   float64
	Percent float64
}

type ReturnAnalyticsRow struct {
	Reason    string
	Count     int
	Amount    float64
	Condition string // for sales returns
}

// Summary aggregates for quick cards.
type SalesSummary struct {
	TotalSales    int
	TotalRevenue  float64
	TotalTax      float64
	TotalDiscount float64
	NetRevenue    float64
	AvgSaleValue  float64
}

type StockSummary struct {
	TotalProducts   int
	TotalQty        int
	TotalCostValue  float64
	TotalSaleValue  float64
	PotentialProfit float64
	LowStockCount   int
}

// ChartPoint is a generic label+value for chart series.
type ChartPoint struct {
	Label string  `json:"label"`
	Value float64 `json:"value"`
}

// ── Service ───────────────────────────────────────────────────────────────────

type ReportService struct {
	db *sql.DB
}

func NewReportService(db *sql.DB) *ReportService {
	return &ReportService{db: db}
}

// ── Inventory reports ─────────────────────────────────────────────────────────

func (s *ReportService) StockValuation(bizID, warehouseID int) ([]StockValuationRow, StockSummary, error) {
	query := `
		SELECT p.id, p.name, p.sku, COALESCE(p.barcode,''), COALESCE(p.brand,''), COALESCE(p.unit,'pcs'),
		       p.stock,
		       p.cost_price, p.price
		FROM products p
		WHERE p.business_id=? AND p.status='active'`
	args := []any{bizID}

	// If warehouse filter, use warehouse_stock instead of products.stock
	if warehouseID > 0 {
		query = `
		SELECT p.id, p.name, p.sku, COALESCE(p.barcode,''), COALESCE(p.brand,''), COALESCE(p.unit,'pcs'),
		       COALESCE(ws.quantity,0),
		       p.cost_price, p.price
		FROM products p
		LEFT JOIN warehouse_stock ws ON ws.product_id=p.id AND ws.warehouse_id=?
		WHERE p.business_id=? AND p.status='active'`
		args = []any{warehouseID, bizID}
	}

	rows, err := s.db.Query(query+" ORDER BY p.name ASC", args...)
	if err != nil {
		return nil, StockSummary{}, err
	}
	defer rows.Close()

	var out []StockValuationRow
	var summary StockSummary
	for rows.Next() {
		var r StockValuationRow
		if err = rows.Scan(&r.ProductID, &r.ProductName, &r.SKU, &r.Barcode, &r.Brand, &r.Unit,
			&r.TotalQty, &r.CostPrice, &r.SalePrice); err != nil {
			return nil, StockSummary{}, err
		}
		r.CostValue = roundCents(float64(r.TotalQty) * r.CostPrice)
		r.SaleValue = roundCents(float64(r.TotalQty) * r.SalePrice)
		r.ProfitValue = roundCents(r.SaleValue - r.CostValue)
		out = append(out, r)

		summary.TotalProducts++
		summary.TotalQty += r.TotalQty
		summary.TotalCostValue += r.CostValue
		summary.TotalSaleValue += r.SaleValue
		summary.PotentialProfit += r.ProfitValue
	}
	if err = rows.Err(); err != nil {
		return nil, StockSummary{}, err
	}

	_ = s.db.QueryRow(
		`SELECT COUNT(*) FROM products WHERE business_id=? AND stock<=low_stock_threshold AND status='active'`,
		bizID,
	).Scan(&summary.LowStockCount)

	return out, summary, nil
}

func (s *ReportService) WarehouseInventory(bizID int) ([]WarehouseInventoryRow, error) {
	rows, err := s.db.Query(`
		SELECT ws.warehouse_id, COALESCE(w.name,'?'), p.name, p.sku, ws.quantity,
		       ws.quantity * p.cost_price
		FROM warehouse_stock ws
		JOIN products   p ON p.id=ws.product_id   AND p.business_id=ws.business_id
		JOIN warehouses w ON w.id=ws.warehouse_id AND w.business_id=ws.business_id
		WHERE ws.business_id=? AND ws.quantity>0
		ORDER BY w.name ASC, p.name ASC`, bizID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WarehouseInventoryRow
	for rows.Next() {
		var r WarehouseInventoryRow
		if err = rows.Scan(&r.WarehouseID, &r.WarehouseName, &r.ProductName, &r.SKU, &r.Quantity, &r.CostValue); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *ReportService) StockMovement(bizID int, f ReportFilter) ([]StockMovementRow, error) {
	from, to := f.ToDateTime()
	query := `
		SELECT p.name, p.sku, COALESCE(w.name,'—'),
		       sl.change_type, sl.quantity_before, sl.quantity_change, sl.quantity_after,
		       sl.note, sl.created_at
		FROM stock_logs sl
		LEFT JOIN products   p ON p.id=sl.product_id
		LEFT JOIN warehouses w ON w.id=sl.warehouse_id
		WHERE p.business_id=? AND sl.created_at BETWEEN ? AND ?`
	args := []any{bizID, from, to}

	if f.ProductID > 0 {
		query += ` AND sl.product_id=?`
		args = append(args, f.ProductID)
	}
	if f.WarehouseID > 0 {
		query += ` AND sl.warehouse_id=?`
		args = append(args, f.WarehouseID)
	}
	query += ` ORDER BY sl.created_at DESC LIMIT 1000`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []StockMovementRow
	for rows.Next() {
		var r StockMovementRow
		if err = rows.Scan(&r.ProductName, &r.SKU, &r.WarehouseName,
			&r.ChangeType, &r.QuantityBefore, &r.QuantityChange, &r.QuantityAfter,
			&r.Note, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *ReportService) DeadStock(bizID, days int) ([]DeadStockRow, error) {
	rows, err := s.db.Query(`
		SELECT p.id, p.name, p.sku,
		       COALESCE(w.name,'—'),
		       COALESCE(ws.quantity, p.stock),
		       COALESCE(ws.quantity, p.stock) * p.cost_price,
		       MAX(sl.created_at),
		       COALESCE(DATEDIFF(NOW(), MAX(sl.created_at)), 9999)
		FROM products p
		LEFT JOIN warehouse_stock ws ON ws.product_id=p.id AND ws.business_id=p.business_id
		LEFT JOIN warehouses      w  ON w.id=ws.warehouse_id AND w.business_id=p.business_id
		LEFT JOIN stock_logs      sl ON sl.product_id=p.id AND sl.created_at > DATE_SUB(NOW(), INTERVAL ? DAY)
		WHERE p.business_id=? AND p.stock>0 AND p.status='active'
		GROUP BY p.id, ws.warehouse_id
		HAVING MAX(sl.created_at) IS NULL OR DATEDIFF(NOW(), MAX(sl.created_at)) >= ?
		ORDER BY DATEDIFF(NOW(), MAX(sl.created_at)) DESC`,
		days, bizID, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DeadStockRow
	for rows.Next() {
		var r DeadStockRow
		if err = rows.Scan(&r.ProductID, &r.ProductName, &r.SKU, &r.WarehouseName,
			&r.Quantity, &r.CostValue, &r.LastMovement, &r.DaysSince); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *ReportService) LowStock(bizID int) ([]LowStockRow, error) {
	rows, err := s.db.Query(`
		SELECT id, name, sku, stock, low_stock_threshold,
		       GREATEST(0, low_stock_threshold - stock)
		FROM products
		WHERE business_id=? AND stock<=low_stock_threshold AND status='active'
		ORDER BY (low_stock_threshold - stock) DESC`, bizID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LowStockRow
	for rows.Next() {
		var r LowStockRow
		if err = rows.Scan(&r.ProductID, &r.ProductName, &r.SKU, &r.CurrentStock, &r.LowStockThreshold, &r.ShortBy); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ── Sales reports ─────────────────────────────────────────────────────────────

func (s *ReportService) SalesSummaryData(bizID int, f ReportFilter) (SalesSummary, error) {
	from, to := f.ToDateTime()
	var summary SalesSummary
	err := s.db.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(grand_total),0), COALESCE(SUM(tax_total),0),
		       COALESCE(SUM(discount),0)
		FROM pos_sales
		WHERE business_id=? AND status='completed' AND created_at BETWEEN ? AND ?`,
		bizID, from, to,
	).Scan(&summary.TotalSales, &summary.TotalRevenue, &summary.TotalTax, &summary.TotalDiscount)
	if err != nil {
		return SalesSummary{}, err
	}
	summary.NetRevenue = roundCents(summary.TotalRevenue - summary.TotalTax)
	if summary.TotalSales > 0 {
		summary.AvgSaleValue = roundCents(summary.TotalRevenue / float64(summary.TotalSales))
	}
	return summary, nil
}

func (s *ReportService) DailySales(bizID int, f ReportFilter) ([]DailySalesRow, error) {
	from, to := f.ToDateTime()
	rows, err := s.db.Query(`
		SELECT DATE(created_at), COUNT(*), SUM(grand_total), SUM(tax_total),
		       SUM(discount), SUM(grand_total)-SUM(tax_total)
		FROM pos_sales
		WHERE business_id=? AND status='completed' AND created_at BETWEEN ? AND ?
		GROUP BY DATE(created_at) ORDER BY DATE(created_at) DESC`,
		bizID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DailySalesRow
	for rows.Next() {
		var r DailySalesRow
		if err = rows.Scan(&r.Date, &r.SaleCount, &r.Revenue, &r.TaxTotal, &r.Discount, &r.NetRevenue); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *ReportService) ProductSales(bizID int, f ReportFilter) ([]ProductSalesRow, error) {
	from, to := f.ToDateTime()
	rows, err := s.db.Query(`
		SELECT i.product_name, i.sku,
		       SUM(i.quantity), SUM(i.line_total), SUM(i.tax_amount),
		       SUM(i.line_total)-SUM(i.tax_amount)
		FROM pos_sale_items i
		JOIN pos_sales s ON s.id=i.sale_id
		WHERE s.business_id=? AND s.status='completed' AND s.created_at BETWEEN ? AND ?
		GROUP BY i.product_id, i.product_name, i.sku
		ORDER BY SUM(i.line_total) DESC LIMIT 100`,
		bizID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ProductSalesRow
	for rows.Next() {
		var r ProductSalesRow
		if err = rows.Scan(&r.ProductName, &r.SKU, &r.TotalQty, &r.Revenue, &r.TaxAmount, &r.NetRevenue); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *ReportService) PaymentMethods(bizID int, f ReportFilter) ([]PaymentMethodRow, error) {
	from, to := f.ToDateTime()
	rows, err := s.db.Query(`
		SELECT payment_method, COUNT(*), SUM(grand_total)
		FROM pos_sales
		WHERE business_id=? AND status='completed' AND created_at BETWEEN ? AND ?
		GROUP BY payment_method ORDER BY SUM(grand_total) DESC`,
		bizID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PaymentMethodRow
	var total float64
	for rows.Next() {
		var r PaymentMethodRow
		if err = rows.Scan(&r.Method, &r.Count, &r.Total); err != nil {
			return nil, err
		}
		total += r.Total
		out = append(out, r)
	}
	// Compute percentages
	for i := range out {
		if total > 0 {
			out[i].Percent = roundCents(out[i].Total / total * 100)
		}
	}
	return out, rows.Err()
}

// DailySalesChart returns ChartPoints ordered ASC (for line chart x-axis).
func (s *ReportService) DailySalesChart(bizID int, f ReportFilter) ([]ChartPoint, error) {
	rows, err := s.DailySales(bizID, f)
	if err != nil {
		return nil, err
	}
	pts := make([]ChartPoint, 0, len(rows))
	// Reverse (daily sales are DESC, chart needs ASC)
	for i := len(rows) - 1; i >= 0; i-- {
		pts = append(pts, ChartPoint{Label: rows[i].Date, Value: rows[i].Revenue})
	}
	return pts, nil
}

func (s *ReportService) TopProductsChart(bizID int, f ReportFilter) ([]ChartPoint, error) {
	rows, err := s.ProductSales(bizID, f)
	if err != nil {
		return nil, err
	}
	n := 10
	if len(rows) < n {
		n = len(rows)
	}
	pts := make([]ChartPoint, n)
	for i := 0; i < n; i++ {
		pts[i] = ChartPoint{Label: rows[i].ProductName, Value: rows[i].Revenue}
	}
	return pts, nil
}

// ── Returns analytics ─────────────────────────────────────────────────────────

type ReturnsReport struct {
	SalesReturnCount     int
	SalesReturnAmount    float64
	PurchaseReturnCount  int
	PurchaseReturnAmount float64
	ByCondition          []ReturnAnalyticsRow
	ByReason             []ReturnAnalyticsRow
}

func (s *ReportService) ReturnsAnalytics(bizID int, f ReportFilter) (ReturnsReport, error) {
	from, to := f.ToDateTime()
	var report ReturnsReport

	_ = s.db.QueryRow(
		`SELECT COUNT(*), COALESCE(SUM(total_amount),0) FROM sales_returns WHERE business_id=? AND created_at BETWEEN ? AND ?`,
		bizID, from, to,
	).Scan(&report.SalesReturnCount, &report.SalesReturnAmount)

	_ = s.db.QueryRow(
		`SELECT COUNT(*), COALESCE(SUM(total_amount),0) FROM purchase_returns WHERE business_id=? AND created_at BETWEEN ? AND ?`,
		bizID, from, to,
	).Scan(&report.PurchaseReturnCount, &report.PurchaseReturnAmount)

	// Returns by condition
	rows, err := s.db.Query(`
		SELECT COALESCE(i.item_condition,'unknown'), COUNT(*), COALESCE(SUM(i.line_total),0)
		FROM sales_return_items i
		JOIN sales_returns r ON r.id=i.return_id
		WHERE r.business_id=? AND r.created_at BETWEEN ? AND ?
		GROUP BY i.item_condition`, bizID, from, to)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var r ReturnAnalyticsRow
			_ = rows.Scan(&r.Condition, &r.Count, &r.Amount)
			report.ByCondition = append(report.ByCondition, r)
		}
	}

	return report, nil
}

// ── Dashboard quick stats ─────────────────────────────────────────────────────

type DashboardAnalytics struct {
	TodaySaleCount  int
	TodayRevenue    float64
	WeekRevenue     float64
	MonthRevenue    float64
	TopProducts     []ProductSalesRow // top 5 this month
	PaymentMethods  []PaymentMethodRow
	DailySalesChart []ChartPoint // last 14 days
	StockValue      float64
	LowStockCount   int
}

func (s *ReportService) DashboardAnalytics(bizID int) (DashboardAnalytics, error) {
	var da DashboardAnalytics

	// Today
	_ = s.db.QueryRow(
		`SELECT COUNT(*), COALESCE(SUM(grand_total),0) FROM pos_sales WHERE business_id=? AND status='completed' AND DATE(created_at)=CURDATE()`,
		bizID,
	).Scan(&da.TodaySaleCount, &da.TodayRevenue)

	// This week
	_ = s.db.QueryRow(
		`SELECT COALESCE(SUM(grand_total),0) FROM pos_sales WHERE business_id=? AND status='completed' AND created_at >= DATE_SUB(CURDATE(),INTERVAL 7 DAY)`,
		bizID,
	).Scan(&da.WeekRevenue)

	// This month
	_ = s.db.QueryRow(
		`SELECT COALESCE(SUM(grand_total),0) FROM pos_sales WHERE business_id=? AND status='completed' AND YEAR(created_at)=YEAR(CURDATE()) AND MONTH(created_at)=MONTH(CURDATE())`,
		bizID,
	).Scan(&da.MonthRevenue)

	// Top 5 products this month
	monthFilter := ReportFilter{
		From: fmt.Sprintf("%d-%02d-01", time.Now().Year(), time.Now().Month()),
		To:   time.Now().Format("2006-01-02"),
	}
	products, _ := s.ProductSales(bizID, monthFilter)
	if len(products) > 5 {
		products = products[:5]
	}
	da.TopProducts = products

	// Payment methods this month
	da.PaymentMethods, _ = s.PaymentMethods(bizID, monthFilter)

	// Daily sales last 14 days for chart
	chartFilter := ReportFilter{
		From: time.Now().AddDate(0, 0, -13).Format("2006-01-02"),
		To:   time.Now().Format("2006-01-02"),
	}
	da.DailySalesChart, _ = s.DailySalesChart(bizID, chartFilter)

	// Stock value
	_ = s.db.QueryRow(
		`SELECT COALESCE(SUM(cost_price*stock),0) FROM products WHERE business_id=? AND status='active'`,
		bizID,
	).Scan(&da.StockValue)

	// Low stock
	_ = s.db.QueryRow(
		`SELECT COUNT(*) FROM products WHERE business_id=? AND stock<=low_stock_threshold AND status='active'`,
		bizID,
	).Scan(&da.LowStockCount)

	return da, nil
}
