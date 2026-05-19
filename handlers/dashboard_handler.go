package handlers

import (
	"fmt"
	"net/http"
	"sync"

	"go-monolith/models"
)

type DashboardData struct {
	AppContext
	ProductCount        int
	LowStockCount       int
	LowStockItems       []ProductView
	DeadStockCount      int
	StockValue          string
	StockCostValue      string
	WarehouseCount      int
	TransferCount       int
	Counts              map[string]int
	InvoiceTotal        string
	PurchaseTotal       string
	PendingInvoiceTotal string
	RecentActivity      []models.Record
	TopCustomers        []models.Record
	ExpiryStats         models.ExpiryStats
	ExpiringBatches     []models.Batch
}

func (a *App) Dashboard(w http.ResponseWriter, r *http.Request) {
	bizID := a.bizID(r)
	modSvc := a.moduleService(r)

	// Fan-out: run all independent DB queries concurrently.
	var (
		mu sync.Mutex

		count      int
		lowCount   int
		lowStock   []models.Product
		deadCount  int
		stockVal   float64
		costVal    float64
		whCount    int
		trCount    int
		counts     map[string]int
		totals     map[string]string
		pending    float64
		activity   []models.Record
		topCust    []models.Record
		expStats   models.ExpiryStats
		expBatches []models.Batch
	)

	type job struct {
		name string
		fn   func() error
	}
	jobs := []job{
		{"product_count", func() error {
			v, err := a.ProductService.Count(bizID)
			mu.Lock(); count = v; mu.Unlock()
			return err
		}},
		{"low_stock_count", func() error {
			v, err := a.ProductService.LowStockCount(bizID)
			mu.Lock(); lowCount = v; mu.Unlock()
			return err
		}},
		{"low_stock_list", func() error {
			v, err := a.ProductService.LowStock(5, bizID)
			mu.Lock(); lowStock = v; mu.Unlock()
			return err
		}},
		{"dead_stock_count", func() error {
			v, err := a.ProductService.DeadStockCount(bizID)
			mu.Lock(); deadCount = v; mu.Unlock()
			return err
		}},
		{"stock_value", func() error {
			sv, cv, err := a.ProductService.TotalStockValue(bizID)
			mu.Lock(); stockVal = sv; costVal = cv; mu.Unlock()
			return err
		}},
		{"warehouse_count", func() error {
			v, err := a.WarehouseService.Count(bizID)
			mu.Lock(); whCount = v; mu.Unlock()
			return err
		}},
		{"transfer_count", func() error {
			v, err := a.WarehouseService.CountTransfers(bizID)
			mu.Lock(); trCount = v; mu.Unlock()
			return err
		}},
		{"module_counts", func() error {
			v, err := modSvc.Counts(bizID)
			mu.Lock(); counts = v; mu.Unlock()
			return err
		}},
		{"module_totals", func() error {
			v, err := modSvc.Totals(bizID)
			mu.Lock(); totals = v; mu.Unlock()
			return err
		}},
		{"pending_invoices", func() error {
			v, err := modSvc.PendingInvoicesTotal(bizID)
			mu.Lock(); pending = v; mu.Unlock()
			return err
		}},
		{"recent_activity", func() error {
			v, err := modSvc.RecentActivity(8, bizID)
			mu.Lock(); activity = v; mu.Unlock()
			return err
		}},
		{"top_customers", func() error {
			v, err := modSvc.TopCustomers(5, bizID)
			mu.Lock(); topCust = v; mu.Unlock()
			return err
		}},
		{"expiry_stats", func() error {
			v, err := a.BatchService.ExpiryStats(bizID)
			mu.Lock(); expStats = v; mu.Unlock()
			return err
		}},
		{"expiry_batches", func() error {
			v, err := a.BatchService.ExpiringList(bizID)
			mu.Lock(); expBatches = v; mu.Unlock()
			return err
		}},
	}

	var wg sync.WaitGroup
	for _, j := range jobs {
		wg.Add(1)
		go func(j job) {
			defer wg.Done()
			if err := j.fn(); err != nil {
				// Non-fatal: dashboard degrades gracefully on partial failures.
				_ = err
			}
		}(j)
	}
	wg.Wait()

	if len(expBatches) > 5 {
		expBatches = expBatches[:5]
	}

	a.Renderer.Page(w, "dashboard.html", DashboardData{
		AppContext:          a.ctx(r),
		ProductCount:        count,
		LowStockCount:       lowCount,
		LowStockItems:       productViews(lowStock),
		DeadStockCount:      deadCount,
		StockValue:          fmt.Sprintf("Rs. %.2f", stockVal),
		StockCostValue:      fmt.Sprintf("Rs. %.2f", costVal),
		WarehouseCount:      whCount,
		TransferCount:       trCount,
		Counts:              counts,
		InvoiceTotal:        totals["invoice_total"],
		PurchaseTotal:       totals["po_total"],
		PendingInvoiceTotal: fmt.Sprintf("Rs. %.2f", pending),
		RecentActivity:      activity,
		TopCustomers:        topCust,
		ExpiryStats:         expStats,
		ExpiringBatches:     expBatches,
	})
}
