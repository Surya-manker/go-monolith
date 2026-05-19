package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"go-monolith/models"
)

// ── Warehouses CRUD ────────────────────────────────────────────────────────────

type WarehousesPageData struct {
	AppContext
	Warehouses []models.Warehouse
	Edit       *models.Warehouse
	Error      string
}

func (a *App) WarehousesIndex(w http.ResponseWriter, r *http.Request) {
	whs, err := a.WarehouseService.List(a.bizID(r))
	if err != nil {
		http.Error(w, "could not load warehouses", http.StatusInternalServerError)
		return
	}
	a.Renderer.Page(w, "warehouses.html", WarehousesPageData{AppContext: a.ctx(r), Warehouses: whs})
}

func (a *App) WarehousesCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}
	bizID := a.bizID(r)
	_, err := a.WarehouseService.Create(
		r.FormValue("name"), r.FormValue("address"), r.FormValue("manager_name"),
		r.FormValue("is_default") == "1", bizID,
	)
	if err != nil {
		whs, _ := a.WarehouseService.List(bizID)
		setToast(w, err.Error(), "error")
		a.Renderer.Partial(w, "warehouses_table.html", WarehousesPageData{AppContext: a.ctx(r), Warehouses: whs, Error: err.Error()})
		return
	}
	a.auditLog(r, "warehouses", "create", "", map[string]string{"name": r.FormValue("name")})
	setToast(w, "Warehouse added successfully", "success")
	whs, _ := a.WarehouseService.List(bizID)
	a.Renderer.Partial(w, "warehouses_table.html", WarehousesPageData{AppContext: a.ctx(r), Warehouses: whs})
}

func (a *App) WarehousesEdit(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil || id <= 0 {
		http.Error(w, "invalid warehouse ID", http.StatusBadRequest)
		return
	}
	bizID := a.bizID(r)
	wh, err := a.WarehouseService.Get(id, bizID)
	if err != nil {
		http.Error(w, "warehouse not found", http.StatusNotFound)
		return
	}
	whs, _ := a.WarehouseService.List(bizID)
	a.Renderer.Partial(w, "warehouses_table.html", WarehousesPageData{AppContext: a.ctx(r), Warehouses: whs, Edit: wh})
}

func (a *App) WarehousesUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil || id <= 0 {
		http.Error(w, "invalid warehouse ID", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}
	bizID := a.bizID(r)
	err = a.WarehouseService.Update(
		id, r.FormValue("name"), r.FormValue("address"), r.FormValue("manager_name"),
		r.FormValue("is_default") == "1", bizID,
	)
	if err != nil {
		whs, _ := a.WarehouseService.List(bizID)
		setToast(w, err.Error(), "error")
		a.Renderer.Partial(w, "warehouses_table.html", WarehousesPageData{AppContext: a.ctx(r), Warehouses: whs, Error: err.Error()})
		return
	}
	a.auditLog(r, "warehouses", "update", strconv.Itoa(id), map[string]string{"name": r.FormValue("name")})
	setToast(w, "Warehouse updated successfully", "success")
	whs, _ := a.WarehouseService.List(bizID)
	a.Renderer.Partial(w, "warehouses_table.html", WarehousesPageData{AppContext: a.ctx(r), Warehouses: whs})
}

func (a *App) WarehousesDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil || id <= 0 {
		http.Error(w, "invalid warehouse ID", http.StatusBadRequest)
		return
	}
	bizID := a.bizID(r)
	err = a.WarehouseService.Delete(id, bizID)
	if err != nil {
		whs, _ := a.WarehouseService.List(bizID)
		setToast(w, err.Error(), "error")
		a.Renderer.Partial(w, "warehouses_table.html", WarehousesPageData{AppContext: a.ctx(r), Warehouses: whs, Error: err.Error()})
		return
	}
	a.auditLog(r, "warehouses", "delete", strconv.Itoa(id), nil)
	setToast(w, "Warehouse deleted", "warning")
	whs, _ := a.WarehouseService.List(bizID)
	a.Renderer.Partial(w, "warehouses_table.html", WarehousesPageData{AppContext: a.ctx(r), Warehouses: whs})
}

// ── Warehouse Stock View ───────────────────────────────────────────────────────

type WarehouseStockPageData struct {
	AppContext
	Warehouses []models.Warehouse
	Products   []models.Product
	Stock      []models.WarehouseStock
	SelectedID int
	Error      string
}

func (a *App) WarehouseStockIndex(w http.ResponseWriter, r *http.Request) {
	bizID := a.bizID(r)
	whs, _ := a.WarehouseService.List(bizID)
	products, _ := a.ProductService.List("", bizID)

	selectedID, _ := strconv.Atoi(r.URL.Query().Get("warehouse_id"))
	var stock []models.WarehouseStock
	var stockErr error

	if selectedID > 0 {
		// Verify the warehouse belongs to this business before fetching its stock.
		if _, err := a.WarehouseService.Get(selectedID, bizID); err != nil {
			selectedID = 0
		} else {
			stock, stockErr = a.WarehouseService.GetWarehouseStock(selectedID, bizID)
		}
	} else {
		stock, stockErr = a.WarehouseService.GetAllWarehouseStock(bizID)
	}

	errMsg := ""
	if stockErr != nil {
		errMsg = stockErr.Error()
	}

	data := WarehouseStockPageData{
		AppContext:  a.ctx(r),
		Warehouses: whs,
		Products:   products,
		Stock:      stock,
		SelectedID: selectedID,
		Error:      errMsg,
	}
	if r.Header.Get("HX-Request") == "true" {
		a.Renderer.Partial(w, "warehouse_stock_table.html", data)
		return
	}
	a.Renderer.Page(w, "warehouse_stock.html", data)
}

// ── Warehouse Stock Adjust ─────────────────────────────────────────────────────

func (a *App) WarehouseStockAdjust(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}
	bizID := a.bizID(r)

	warehouseID, whErr := strconv.Atoi(r.FormValue("warehouse_id"))
	if whErr != nil || warehouseID <= 0 {
		setToast(w, "Please select a valid warehouse", "error")
		a.renderStockTable(w, r, bizID, 0)
		return
	}
	productID, prodErr := strconv.Atoi(r.FormValue("product_id"))
	if prodErr != nil || productID <= 0 {
		setToast(w, "Please select a valid product", "error")
		a.renderStockTable(w, r, bizID, warehouseID)
		return
	}
	qty, qtyErr := strconv.Atoi(r.FormValue("quantity_change"))
	if qtyErr != nil || qty == 0 {
		setToast(w, "Quantity must be a non-zero number", "error")
		a.renderStockTable(w, r, bizID, warehouseID)
		return
	}

	changeType := r.FormValue("change_type")
	note := strings.TrimSpace(r.FormValue("note"))

	err := a.WarehouseService.AdjustWarehouseStock(warehouseID, productID, bizID, qty, changeType, note)
	if err != nil {
		setToast(w, err.Error(), "error")
	} else {
		a.auditLog(r, "warehouse_stock", "adjust", strconv.Itoa(productID), map[string]string{
			"warehouse_id": strconv.Itoa(warehouseID),
			"change_type":  changeType,
			"quantity":     strconv.Itoa(qty),
			"note":         note,
		})
		setToast(w, "Stock adjusted successfully", "success")
	}
	a.renderStockTable(w, r, bizID, warehouseID)
}

func (a *App) renderStockTable(w http.ResponseWriter, r *http.Request, bizID, selectedID int) {
	var stock []models.WarehouseStock
	if selectedID > 0 {
		stock, _ = a.WarehouseService.GetWarehouseStock(selectedID, bizID)
	} else {
		stock, _ = a.WarehouseService.GetAllWarehouseStock(bizID)
	}
	whs, _ := a.WarehouseService.List(bizID)
	products, _ := a.ProductService.List("", bizID)
	a.Renderer.Partial(w, "warehouse_stock_table.html", WarehouseStockPageData{
		AppContext:  a.ctx(r),
		Warehouses: whs,
		Products:   products,
		Stock:      stock,
		SelectedID: selectedID,
	})
}

// ── Stock Transfers ────────────────────────────────────────────────────────────

type TransfersPageData struct {
	AppContext
	Transfers  []models.StockTransfer
	Warehouses []models.Warehouse
	Products   []models.Product
	Error      string
}

func (a *App) TransfersIndex(w http.ResponseWriter, r *http.Request) {
	bizID := a.bizID(r)
	transfers, _ := a.WarehouseService.ListTransfers(bizID)
	whs, _ := a.WarehouseService.List(bizID)
	products, _ := a.ProductService.List("", bizID)
	a.Renderer.Page(w, "stock_transfers.html", TransfersPageData{
		AppContext:  a.ctx(r),
		Transfers:  transfers,
		Warehouses: whs,
		Products:   products,
	})
}

func (a *App) TransfersCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}
	bizID := a.bizID(r)

	fromID, err := strconv.Atoi(r.FormValue("from_warehouse_id"))
	if err != nil || fromID <= 0 {
		setToast(w, "Please select a source warehouse", "error")
		a.renderTransfersTable(w, r, bizID, "Please select a source warehouse")
		return
	}
	toID, err := strconv.Atoi(r.FormValue("to_warehouse_id"))
	if err != nil || toID <= 0 {
		setToast(w, "Please select a destination warehouse", "error")
		a.renderTransfersTable(w, r, bizID, "Please select a destination warehouse")
		return
	}
	productID, err := strconv.Atoi(r.FormValue("product_id"))
	if err != nil || productID <= 0 {
		setToast(w, "Please select a product", "error")
		a.renderTransfersTable(w, r, bizID, "Please select a product")
		return
	}
	qty, err := strconv.Atoi(r.FormValue("quantity"))
	if err != nil || qty <= 0 {
		setToast(w, "Transfer quantity must be greater than zero", "error")
		a.renderTransfersTable(w, r, bizID, "Transfer quantity must be greater than zero")
		return
	}

	_, err = a.WarehouseService.CreateTransfer(fromID, toID, productID, qty, bizID, r.FormValue("note"))
	if err != nil {
		setToast(w, err.Error(), "error")
		a.renderTransfersTable(w, r, bizID, err.Error())
		return
	}

	a.auditLog(r, "stock_transfers", "create", "", map[string]string{
		"from_warehouse": strconv.Itoa(fromID),
		"to_warehouse":   strconv.Itoa(toID),
		"product_id":     strconv.Itoa(productID),
		"quantity":       strconv.Itoa(qty),
	})
	setToast(w, "Stock transferred successfully", "success")
	a.renderTransfersTable(w, r, bizID, "")
}

func (a *App) renderTransfersTable(w http.ResponseWriter, r *http.Request, bizID int, errMsg string) {
	transfers, _ := a.WarehouseService.ListTransfers(bizID)
	whs, _ := a.WarehouseService.List(bizID)
	products, _ := a.ProductService.List("", bizID)
	a.Renderer.Partial(w, "transfers_table.html", TransfersPageData{
		AppContext:  a.ctx(r),
		Transfers:  transfers,
		Warehouses: whs,
		Products:   products,
		Error:      errMsg,
	})
}
