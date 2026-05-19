package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"go-monolith/models"
	"go-monolith/services"
)

// ── Returns hub ───────────────────────────────────────────────────────────────

type ReturnsHubData struct {
	AppContext
	SalesReturns    []models.SalesReturn
	PurchaseReturns []models.PurchaseReturn
}

func (a *App) ReturnsIndex(w http.ResponseWriter, r *http.Request) {
	bizID := a.bizID(r)
	sr, _ := a.ReturnsService.ListSalesReturns(bizID, 20)
	pr, _ := a.ReturnsService.ListPurchaseReturns(bizID, 20)
	a.Renderer.Page(w, "returns.html", ReturnsHubData{
		AppContext:       a.ctx(r),
		SalesReturns:    sr,
		PurchaseReturns: pr,
	})
}

// ── Sales Returns ─────────────────────────────────────────────────────────────

type SalesReturnNewData struct {
	AppContext
	Warehouses []models.Warehouse
	Products   []models.Product
	Batches    []models.Batch
	Error      string
}

func (a *App) SalesReturnNewPage(w http.ResponseWriter, r *http.Request) {
	bizID := a.bizID(r)
	whs, _ := a.WarehouseService.List(bizID)
	products, _ := a.ProductService.List("", bizID)
	batches, _ := a.BatchService.List(bizID, 0, 0)
	a.Renderer.Page(w, "sales_return_new.html", SalesReturnNewData{
		AppContext:  a.ctx(r),
		Warehouses: whs,
		Products:   products,
		Batches:    batches,
	})
}

func (a *App) SalesReturnCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	bizID := a.bizID(r)

	whID, err := strconv.Atoi(r.FormValue("warehouse_id"))
	if err != nil || whID <= 0 {
		setToast(w, "Please select a warehouse", "error")
		http.Redirect(w, r, "/returns/sales/new", http.StatusSeeOther)
		return
	}

	productIDs := r.Form["product_id[]"]
	quantities := r.Form["quantity[]"]
	prices := r.Form["unit_price[]"]
	conditions := r.Form["condition[]"]
	batchIDs := r.Form["batch_id[]"]
	notes := r.Form["notes[]"]

	if len(productIDs) == 0 {
		setToast(w, "Please add at least one return item", "error")
		http.Redirect(w, r, "/returns/sales/new", http.StatusSeeOther)
		return
	}

	var items []services.ReturnItemInput
	for i := range productIDs {
		pid, _ := strconv.Atoi(productIDs[i])
		qty, _ := strconv.Atoi(safeIndex(quantities, i))
		price, _ := strconv.ParseFloat(safeIndex(prices, i), 64)
		cond := safeIndex(conditions, i)
		note := safeIndex(notes, i)

		var batchID *int
		if bid, err := strconv.Atoi(safeIndex(batchIDs, i)); err == nil && bid > 0 {
			batchID = &bid
		}

		items = append(items, services.ReturnItemInput{
			ProductID: pid,
			BatchID:   batchID,
			Quantity:  qty,
			UnitPrice: price,
			Condition: cond,
			Notes:     note,
		})
	}

	var origSaleID *int
	if v, err := strconv.Atoi(r.FormValue("original_sale_id")); err == nil && v > 0 {
		origSaleID = &v
	}

	ret, err := a.ReturnsService.CreateSalesReturn(services.SalesReturnInput{
		BusinessID:     bizID,
		WarehouseID:    whID,
		OriginalSaleID: origSaleID,
		CustomerName:   strings.TrimSpace(r.FormValue("customer_name")),
		CustomerPhone:  strings.TrimSpace(r.FormValue("customer_phone")),
		ReturnReason:   strings.TrimSpace(r.FormValue("return_reason")),
		Items:          items,
	})
	if err != nil {
		setToast(w, err.Error(), "error")
		http.Redirect(w, r, "/returns/sales/new", http.StatusSeeOther)
		return
	}

	a.auditLog(r, "sales_returns", "create", strconv.Itoa(ret.ID), map[string]string{
		"return_number": ret.ReturnNumber,
	})
	setToast(w, "Sales return "+ret.ReturnNumber+" created", "success")
	http.Redirect(w, r, "/returns", http.StatusSeeOther)
}

func (a *App) SalesReturnsList(w http.ResponseWriter, r *http.Request) {
	bizID := a.bizID(r)
	returns, _ := a.ReturnsService.ListSalesReturns(bizID, 100)
	a.Renderer.Page(w, "sales_returns.html", struct {
		AppContext
		Returns []models.SalesReturn
	}{AppContext: a.ctx(r), Returns: returns})
}

// ── Purchase Returns ──────────────────────────────────────────────────────────

type PurchaseReturnNewData struct {
	AppContext
	Warehouses []models.Warehouse
	Products   []models.Product
	Batches    []models.Batch
	Error      string
}

func (a *App) PurchaseReturnNewPage(w http.ResponseWriter, r *http.Request) {
	bizID := a.bizID(r)
	whs, _ := a.WarehouseService.List(bizID)
	products, _ := a.ProductService.List("", bizID)
	batches, _ := a.BatchService.List(bizID, 0, 0)
	a.Renderer.Page(w, "purchase_return_new.html", PurchaseReturnNewData{
		AppContext:  a.ctx(r),
		Warehouses: whs,
		Products:   products,
		Batches:    batches,
	})
}

func (a *App) PurchaseReturnCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	bizID := a.bizID(r)

	whID, err := strconv.Atoi(r.FormValue("warehouse_id"))
	if err != nil || whID <= 0 {
		setToast(w, "Please select a warehouse", "error")
		http.Redirect(w, r, "/returns/purchase/new", http.StatusSeeOther)
		return
	}

	productIDs := r.Form["product_id[]"]
	quantities := r.Form["quantity[]"]
	prices := r.Form["unit_price[]"]
	batchIDs := r.Form["batch_id[]"]
	notes := r.Form["notes[]"]

	if len(productIDs) == 0 {
		setToast(w, "Please add at least one return item", "error")
		http.Redirect(w, r, "/returns/purchase/new", http.StatusSeeOther)
		return
	}

	var items []services.ReturnItemInput
	for i := range productIDs {
		pid, _ := strconv.Atoi(productIDs[i])
		qty, _ := strconv.Atoi(safeIndex(quantities, i))
		price, _ := strconv.ParseFloat(safeIndex(prices, i), 64)
		note := safeIndex(notes, i)

		var batchID *int
		if bid, err := strconv.Atoi(safeIndex(batchIDs, i)); err == nil && bid > 0 {
			batchID = &bid
		}

		items = append(items, services.ReturnItemInput{
			ProductID: pid,
			BatchID:   batchID,
			Quantity:  qty,
			UnitPrice: price,
			Notes:     note,
		})
	}

	ret, err := a.ReturnsService.CreatePurchaseReturn(services.PurchaseReturnInput{
		BusinessID:   bizID,
		WarehouseID:  whID,
		VendorName:   strings.TrimSpace(r.FormValue("vendor_name")),
		ReturnReason: strings.TrimSpace(r.FormValue("return_reason")),
		Items:        items,
	})
	if err != nil {
		setToast(w, err.Error(), "error")
		http.Redirect(w, r, "/returns/purchase/new", http.StatusSeeOther)
		return
	}

	a.auditLog(r, "purchase_returns", "create", strconv.Itoa(ret.ID), map[string]string{
		"return_number": ret.ReturnNumber,
	})
	setToast(w, "Purchase return "+ret.ReturnNumber+" created", "success")
	http.Redirect(w, r, "/returns", http.StatusSeeOther)
}

func (a *App) PurchaseReturnsList(w http.ResponseWriter, r *http.Request) {
	bizID := a.bizID(r)
	returns, _ := a.ReturnsService.ListPurchaseReturns(bizID, 100)
	a.Renderer.Page(w, "purchase_returns.html", struct {
		AppContext
		Returns []models.PurchaseReturn
	}{AppContext: a.ctx(r), Returns: returns})
}

func safeIndex(slice []string, i int) string {
	if i < len(slice) {
		return slice[i]
	}
	return ""
}
