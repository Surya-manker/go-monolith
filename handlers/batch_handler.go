package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"go-monolith/models"
)

// ── Batch list ────────────────────────────────────────────────────────────────

type BatchesPageData struct {
	AppContext
	Batches     []models.Batch
	Warehouses  []models.Warehouse
	Products    []models.Product
	WarehouseID int
	ProductID   int
	StatusFilter string
}

func (a *App) BatchesIndex(w http.ResponseWriter, r *http.Request) {
	bizID := a.bizID(r)
	whID, _ := strconv.Atoi(r.URL.Query().Get("warehouse_id"))
	prodID, _ := strconv.Atoi(r.URL.Query().Get("product_id"))

	batches, err := a.BatchService.List(bizID, whID, prodID)
	if err != nil {
		http.Error(w, "could not load batches", http.StatusInternalServerError)
		return
	}
	whs, _ := a.WarehouseService.List(bizID)
	products, _ := a.ProductService.List("", bizID)

	a.Renderer.Page(w, "batches.html", BatchesPageData{
		AppContext:   a.ctx(r),
		Batches:     batches,
		Warehouses:  whs,
		Products:    products,
		WarehouseID: whID,
		ProductID:   prodID,
	})
}

// ── Receive batch (stock in with batch details) ───────────────────────────────

func (a *App) BatchReceive(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	bizID := a.bizID(r)

	productID, err := strconv.Atoi(r.FormValue("product_id"))
	if err != nil || productID <= 0 {
		setToast(w, "Please select a valid product", "error")
		http.Redirect(w, r, "/batches", http.StatusSeeOther)
		return
	}
	warehouseID, err := strconv.Atoi(r.FormValue("warehouse_id"))
	if err != nil || warehouseID <= 0 {
		setToast(w, "Please select a valid warehouse", "error")
		http.Redirect(w, r, "/batches", http.StatusSeeOther)
		return
	}
	qty, err := strconv.Atoi(r.FormValue("quantity"))
	if err != nil || qty <= 0 {
		setToast(w, "Quantity must be greater than zero", "error")
		http.Redirect(w, r, "/batches", http.StatusSeeOther)
		return
	}

	var mfgDate, expiryDate *time.Time
	if v := r.FormValue("mfg_date"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			mfgDate = &t
		}
	}
	if v := r.FormValue("expiry_date"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			expiryDate = &t
		}
	}

	_, err = a.BatchService.ReceiveBatch(
		bizID, productID, warehouseID, qty,
		r.FormValue("batch_number"), r.FormValue("lot_number"), r.FormValue("notes"),
		mfgDate, expiryDate,
		"",
	)
	if err != nil {
		setToast(w, err.Error(), "error")
	} else {
		a.auditLog(r, "batches", "receive", strconv.Itoa(productID), map[string]string{
			"batch_number": r.FormValue("batch_number"),
			"quantity":     strconv.Itoa(qty),
		})
		setToast(w, "Batch received and stock updated", "success")
	}
	http.Redirect(w, r, "/batches?warehouse_id="+strconv.Itoa(warehouseID), http.StatusSeeOther)
}

// ── Batch edit ────────────────────────────────────────────────────────────────

type BatchEditPageData struct {
	AppContext
	Batch *models.Batch
	Error string
}

func (a *App) BatchEdit(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil || id <= 0 {
		http.Error(w, "invalid batch ID", http.StatusBadRequest)
		return
	}
	batch, err := a.BatchService.Get(id, a.bizID(r))
	if err != nil {
		http.Error(w, "batch not found", http.StatusNotFound)
		return
	}
	a.Renderer.Page(w, "batch_edit.html", BatchEditPageData{AppContext: a.ctx(r), Batch: batch})
}

func (a *App) BatchUpdate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	id, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil || id <= 0 {
		http.Error(w, "invalid batch ID", http.StatusBadRequest)
		return
	}

	var mfgDate, expiryDate *time.Time
	if v := r.FormValue("mfg_date"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			mfgDate = &t
		}
	}
	if v := r.FormValue("expiry_date"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			expiryDate = &t
		}
	}

	err = a.BatchService.UpdateBatch(
		id, a.bizID(r),
		r.FormValue("batch_number"), r.FormValue("lot_number"), r.FormValue("notes"),
		mfgDate, expiryDate,
	)
	if err != nil {
		batch, _ := a.BatchService.Get(id, a.bizID(r))
		setToast(w, err.Error(), "error")
		a.Renderer.Page(w, "batch_edit.html", BatchEditPageData{AppContext: a.ctx(r), Batch: batch, Error: err.Error()})
		return
	}
	a.auditLog(r, "batches", "update", strconv.Itoa(id), nil)
	setToast(w, "Batch updated", "success")
	http.Redirect(w, r, "/batches", http.StatusSeeOther)
}

// ── Expiry report ─────────────────────────────────────────────────────────────

type ExpiryPageData struct {
	AppContext
	Expiring  []models.Batch
	Expired   []models.Batch
	Stats     models.ExpiryStats
	AlertDays int
}

func (a *App) BatchExpiryReport(w http.ResponseWriter, r *http.Request) {
	bizID := a.bizID(r)
	stats, _ := a.BatchService.ExpiryStats(bizID)
	expiring, _ := a.BatchService.ExpiringList(bizID)
	expired, _ := a.BatchService.ExpiredList(bizID)
	a.Renderer.Page(w, "batch_expiry.html", ExpiryPageData{
		AppContext: a.ctx(r),
		Expiring:  expiring,
		Expired:   expired,
		Stats:     stats,
		AlertDays: a.BatchService.AlertDays,
	})
}

// ── Write-off expired ─────────────────────────────────────────────────────────

func (a *App) BatchWriteOff(w http.ResponseWriter, r *http.Request) {
	bizID := a.bizID(r)
	count, err := a.BatchService.WriteOffExpired(bizID)
	if err != nil {
		setToast(w, "Write-off failed: "+err.Error(), "error")
	} else if count == 0 {
		setToast(w, "No expired batches to write off", "info")
	} else {
		a.auditLog(r, "batches", "write_off", "", map[string]string{"count": strconv.Itoa(count)})
		setToast(w, strconv.Itoa(count)+" expired batch(es) written off", "success")
	}
	http.Redirect(w, r, "/batches/expiry", http.StatusSeeOther)
}

// ── Batch movement logs ───────────────────────────────────────────────────────

type BatchLogsPageData struct {
	AppContext
	Logs      []models.BatchLog
	Products  []models.Product
	ProductID int
}

func (a *App) BatchLogs(w http.ResponseWriter, r *http.Request) {
	bizID := a.bizID(r)
	prodID, _ := strconv.Atoi(r.URL.Query().Get("product_id"))
	logs, _ := a.BatchService.BatchLogs(bizID, prodID)
	products, _ := a.ProductService.List("", bizID)
	a.Renderer.Page(w, "batch_logs.html", BatchLogsPageData{
		AppContext: a.ctx(r),
		Logs:      logs,
		Products:  products,
		ProductID: prodID,
	})
}

// helper
func strDefault(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}
