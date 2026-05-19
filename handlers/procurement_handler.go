package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go-monolith/models"
	"go-monolith/services"
)

// ── Procurement Dashboard ─────────────────────────────────────────────────────

type ProcurementDashboardData struct {
	AppContext
	Stats       models.ProcurementStats
	RecentPOs   []models.ProcurementOrder
	RecentGRNs  []models.ProcurementGRN
	Suggestions []models.ReorderSuggestion
}

func (a *App) ProcurementDashboard(w http.ResponseWriter, r *http.Request) {
	bizID := a.bizID(r)
	stats, _ := a.ProcurementService.Stats(bizID)
	recentPOs, _ := a.ProcurementService.ListOrders(bizID, "")
	if len(recentPOs) > 5 {
		recentPOs = recentPOs[:5]
	}
	recentGRNs, _ := a.ProcurementService.ListGRN(bizID)
	if len(recentGRNs) > 5 {
		recentGRNs = recentGRNs[:5]
	}
	suggestions, _ := a.ProcurementService.ReorderSuggestions(bizID)
	if len(suggestions) > 5 {
		suggestions = suggestions[:5]
	}
	a.Renderer.Page(w, "procurement_dashboard.html", ProcurementDashboardData{
		AppContext:   a.ctx(r),
		Stats:        stats,
		RecentPOs:    recentPOs,
		RecentGRNs:   recentGRNs,
		Suggestions:  suggestions,
	})
}

// ── Suppliers ─────────────────────────────────────────────────────────────────

type SuppliersPageData struct {
	AppContext
	Suppliers []models.Supplier
	Edit      *models.Supplier
	Error     string
}

func (a *App) SuppliersIndex(w http.ResponseWriter, r *http.Request) {
	suppliers, err := a.ProcurementService.ListSuppliers(a.bizID(r))
	if err != nil {
		http.Error(w, "could not load suppliers", http.StatusInternalServerError)
		return
	}
	a.Renderer.Page(w, "suppliers.html", SuppliersPageData{AppContext: a.ctx(r), Suppliers: suppliers})
}

func (a *App) SuppliersCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	bizID := a.bizID(r)
	payTerms, _ := strconv.Atoi(r.FormValue("payment_terms"))
	creditLimit, _ := strconv.ParseFloat(r.FormValue("credit_limit"), 64)

	sup, err := a.ProcurementService.CreateSupplier(
		bizID,
		r.FormValue("name"), r.FormValue("email"), r.FormValue("phone"),
		r.FormValue("gstin"), r.FormValue("pan"), r.FormValue("address"),
		r.FormValue("contact_person"), r.FormValue("supplier_code"), r.FormValue("notes"),
		payTerms, creditLimit,
	)
	if err != nil {
		suppliers, _ := a.ProcurementService.ListSuppliers(bizID)
		setToast(w, err.Error(), "error")
		a.Renderer.Partial(w, "suppliers_table.html", SuppliersPageData{AppContext: a.ctx(r), Suppliers: suppliers, Error: err.Error()})
		return
	}
	a.auditLog(r, "suppliers", "create", strconv.Itoa(sup.ID), map[string]string{"name": sup.Name})
	setToast(w, "Supplier "+sup.Name+" added", "success")
	suppliers, _ := a.ProcurementService.ListSuppliers(bizID)
	a.Renderer.Partial(w, "suppliers_table.html", SuppliersPageData{AppContext: a.ctx(r), Suppliers: suppliers})
}

func (a *App) SuppliersEdit(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil || id <= 0 {
		http.Error(w, "invalid supplier ID", http.StatusBadRequest)
		return
	}
	bizID := a.bizID(r)
	sup, err := a.ProcurementService.GetSupplier(id, bizID)
	if err != nil {
		http.Error(w, "supplier not found", http.StatusNotFound)
		return
	}
	suppliers, _ := a.ProcurementService.ListSuppliers(bizID)
	a.Renderer.Partial(w, "suppliers_table.html", SuppliersPageData{AppContext: a.ctx(r), Suppliers: suppliers, Edit: sup})
}

func (a *App) SuppliersUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil || id <= 0 {
		http.Error(w, "invalid supplier ID", http.StatusBadRequest)
		return
	}
	if err = r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	bizID := a.bizID(r)
	payTerms, _ := strconv.Atoi(r.FormValue("payment_terms"))
	creditLimit, _ := strconv.ParseFloat(r.FormValue("credit_limit"), 64)

	err = a.ProcurementService.UpdateSupplier(
		id, bizID,
		r.FormValue("name"), r.FormValue("email"), r.FormValue("phone"),
		r.FormValue("gstin"), r.FormValue("pan"), r.FormValue("address"),
		r.FormValue("contact_person"), r.FormValue("status"), r.FormValue("notes"),
		payTerms, creditLimit,
	)
	if err != nil {
		suppliers, _ := a.ProcurementService.ListSuppliers(bizID)
		setToast(w, err.Error(), "error")
		a.Renderer.Partial(w, "suppliers_table.html", SuppliersPageData{AppContext: a.ctx(r), Suppliers: suppliers, Error: err.Error()})
		return
	}
	a.auditLog(r, "suppliers", "update", strconv.Itoa(id), nil)
	setToast(w, "Supplier updated", "success")
	suppliers, _ := a.ProcurementService.ListSuppliers(bizID)
	a.Renderer.Partial(w, "suppliers_table.html", SuppliersPageData{AppContext: a.ctx(r), Suppliers: suppliers})
}

func (a *App) SupplierView(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil || id <= 0 {
		http.Error(w, "invalid supplier ID", http.StatusBadRequest)
		return
	}
	bizID := a.bizID(r)
	sup, err := a.ProcurementService.GetSupplier(id, bizID)
	if err != nil {
		http.Error(w, "supplier not found", http.StatusNotFound)
		return
	}
	orders, _ := a.ProcurementService.ListOrders(bizID, "")
	payments, _ := a.ProcurementService.ListPayments(bizID, id)

	// Filter orders by supplier
	var supplierOrders []models.ProcurementOrder
	for _, o := range orders {
		if o.SupplierID == id {
			supplierOrders = append(supplierOrders, o)
		}
	}

	a.Renderer.Page(w, "supplier_view.html", struct {
		AppContext
		Supplier *models.Supplier
		Orders   []models.ProcurementOrder
		Payments []models.SupplierPayment
	}{AppContext: a.ctx(r), Supplier: sup, Orders: supplierOrders, Payments: payments})
}

// ── Purchase Orders ───────────────────────────────────────────────────────────

type POsPageData struct {
	AppContext
	Orders     []models.ProcurementOrder
	StatusFilter string
}

func (a *App) POsIndex(w http.ResponseWriter, r *http.Request) {
	bizID := a.bizID(r)
	status := r.URL.Query().Get("status")
	orders, _ := a.ProcurementService.ListOrders(bizID, status)
	a.Renderer.Page(w, "purchase_orders_list.html", POsPageData{
		AppContext: a.ctx(r), Orders: orders, StatusFilter: status,
	})
}

type PONewData struct {
	AppContext
	Suppliers  []models.Supplier
	Warehouses []models.Warehouse
	Products   []models.Product
	Error      string
}

func (a *App) PONew(w http.ResponseWriter, r *http.Request) {
	bizID := a.bizID(r)
	suppliers, _ := a.ProcurementService.ListSuppliers(bizID)
	whs, _ := a.WarehouseService.List(bizID)
	products, _ := a.ProductService.List("", bizID)
	a.Renderer.Page(w, "purchase_order_new.html", PONewData{
		AppContext:  a.ctx(r),
		Suppliers:  suppliers,
		Warehouses: whs,
		Products:   products,
	})
}

func (a *App) POCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	bizID := a.bizID(r)

	supplierID, _ := strconv.Atoi(r.FormValue("supplier_id"))
	warehouseID, _ := strconv.Atoi(r.FormValue("warehouse_id"))
	notes := strings.TrimSpace(r.FormValue("notes"))

	var expectedDate *time.Time
	if v := r.FormValue("expected_date"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			expectedDate = &t
		}
	}

	productIDs := r.Form["product_id[]"]
	quantities := r.Form["quantity[]"]
	prices := r.Form["unit_price[]"]
	taxRates := r.Form["tax_rate[]"]

	var items []services.POItemInput
	for i := range productIDs {
		pid, _ := strconv.Atoi(safeIndex(productIDs, i))
		qty, _ := strconv.Atoi(safeIndex(quantities, i))
		price, _ := strconv.ParseFloat(safeIndex(prices, i), 64)
		taxRate, _ := strconv.ParseFloat(safeIndex(taxRates, i), 64)
		if pid > 0 && qty > 0 {
			items = append(items, services.POItemInput{
				ProductID: pid, Quantity: qty, UnitPrice: price, TaxRate: taxRate,
			})
		}
	}

	po, err := a.ProcurementService.CreatePO(bizID, supplierID, warehouseID, expectedDate, notes, items)
	if err != nil {
		suppliers, _ := a.ProcurementService.ListSuppliers(bizID)
		whs, _ := a.WarehouseService.List(bizID)
		products, _ := a.ProductService.List("", bizID)
		setToast(w, err.Error(), "error")
		a.Renderer.Page(w, "purchase_order_new.html", PONewData{
			AppContext: a.ctx(r), Suppliers: suppliers, Warehouses: whs, Products: products, Error: err.Error(),
		})
		return
	}
	a.auditLog(r, "procurement_orders", "create", strconv.Itoa(po.ID), map[string]string{"po_number": po.PONumber})
	setToast(w, "PO "+po.PONumber+" created", "success")
	http.Redirect(w, r, "/procurement/orders/view?id="+strconv.Itoa(po.ID), http.StatusSeeOther)
}

func (a *App) POView(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil || id <= 0 {
		http.Error(w, "invalid PO ID", http.StatusBadRequest)
		return
	}
	po, err := a.ProcurementService.GetOrder(id, a.bizID(r))
	if err != nil {
		http.Error(w, "PO not found", http.StatusNotFound)
		return
	}
	a.Renderer.Page(w, "purchase_order_view.html", struct {
		AppContext
		Order *models.ProcurementOrder
	}{AppContext: a.ctx(r), Order: po})
}

func (a *App) POSubmit(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil || id <= 0 {
		http.Error(w, "invalid ID", http.StatusBadRequest)
		return
	}
	if err = a.ProcurementService.SubmitForApproval(id, a.bizID(r)); err != nil {
		setToast(w, err.Error(), "error")
	} else {
		a.auditLog(r, "procurement_orders", "submit", strconv.Itoa(id), nil)
		setToast(w, "PO submitted for approval", "success")
	}
	http.Redirect(w, r, "/procurement/orders/view?id="+strconv.Itoa(id), http.StatusSeeOther)
}

func (a *App) POApprove(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil || id <= 0 {
		http.Error(w, "invalid ID", http.StatusBadRequest)
		return
	}
	user := a.ctx(r).User
	approverName := "Admin"
	if user != nil {
		approverName = user.Name
	}
	if err = a.ProcurementService.ApprovePO(id, a.bizID(r), approverName); err != nil {
		setToast(w, err.Error(), "error")
	} else {
		a.auditLog(r, "procurement_orders", "approve", strconv.Itoa(id), nil)
		setToast(w, "PO approved — ready to receive", "success")
	}
	http.Redirect(w, r, "/procurement/orders/view?id="+strconv.Itoa(id), http.StatusSeeOther)
}

func (a *App) POCancel(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil || id <= 0 {
		http.Error(w, "invalid ID", http.StatusBadRequest)
		return
	}
	if err = a.ProcurementService.CancelPO(id, a.bizID(r)); err != nil {
		setToast(w, err.Error(), "error")
	} else {
		a.auditLog(r, "procurement_orders", "cancel", strconv.Itoa(id), nil)
		setToast(w, "PO cancelled", "warning")
	}
	http.Redirect(w, r, "/procurement/orders/view?id="+strconv.Itoa(id), http.StatusSeeOther)
}

// ── GRN ──────────────────────────────────────────────────────────────────────

type GRNNewData struct {
	AppContext
	Order      *models.ProcurementOrder
	Suppliers  []models.Supplier
	Warehouses []models.Warehouse
	Products   []models.Product
	Error      string
}

func (a *App) GRNIndex(w http.ResponseWriter, r *http.Request) {
	bizID := a.bizID(r)
	grns, _ := a.ProcurementService.ListGRN(bizID)
	a.Renderer.Page(w, "grn_list.html", struct {
		AppContext
		GRNs []models.ProcurementGRN
	}{AppContext: a.ctx(r), GRNs: grns})
}

func (a *App) GRNNew(w http.ResponseWriter, r *http.Request) {
	bizID := a.bizID(r)
	suppliers, _ := a.ProcurementService.ListSuppliers(bizID)
	whs, _ := a.WarehouseService.List(bizID)
	products, _ := a.ProductService.List("", bizID)

	var order *models.ProcurementOrder
	if poID, _ := strconv.Atoi(r.URL.Query().Get("po_id")); poID > 0 {
		order, _ = a.ProcurementService.GetOrder(poID, bizID)
	}

	a.Renderer.Page(w, "grn_new.html", GRNNewData{
		AppContext:  a.ctx(r),
		Order:      order,
		Suppliers:  suppliers,
		Warehouses: whs,
		Products:   products,
	})
}

func (a *App) GRNCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	bizID := a.bizID(r)

	orderID, _ := strconv.Atoi(r.FormValue("order_id"))
	supplierID, _ := strconv.Atoi(r.FormValue("supplier_id"))
	warehouseID, _ := strconv.Atoi(r.FormValue("warehouse_id"))
	notes := strings.TrimSpace(r.FormValue("notes"))

	productIDs := r.Form["product_id[]"]
	orderItemIDs := r.Form["order_item_id[]"]
	receivedQtys := r.Form["received_qty[]"]
	damagedQtys := r.Form["damaged_qty[]"]
	batchNums := r.Form["batch_number[]"]
	lotNums := r.Form["lot_number[]"]
	mfgDates := r.Form["mfg_date[]"]
	expiryDates := r.Form["expiry_date[]"]
	unitPrices := r.Form["unit_price[]"]

	var items []services.GRNItemInput
	for i := range productIDs {
		pid, _ := strconv.Atoi(safeIndex(productIDs, i))
		if pid <= 0 {
			continue
		}
		rcvQty, _ := strconv.Atoi(safeIndex(receivedQtys, i))
		dmgQty, _ := strconv.Atoi(safeIndex(damagedQtys, i))
		orderItemID, _ := strconv.Atoi(safeIndex(orderItemIDs, i))
		price, _ := strconv.ParseFloat(safeIndex(unitPrices, i), 64)

		var mfgDate, expiryDate *time.Time
		if v := safeIndex(mfgDates, i); v != "" {
			if t, err := time.Parse("2006-01-02", v); err == nil {
				mfgDate = &t
			}
		}
		if v := safeIndex(expiryDates, i); v != "" {
			if t, err := time.Parse("2006-01-02", v); err == nil {
				expiryDate = &t
			}
		}

		items = append(items, services.GRNItemInput{
			OrderItemID: orderItemID,
			ProductID:   pid,
			ReceivedQty: rcvQty,
			DamagedQty:  dmgQty,
			UnitPrice:   price,
			BatchNumber: safeIndex(batchNums, i),
			LotNumber:   safeIndex(lotNums, i),
			MfgDate:     mfgDate,
			ExpiryDate:  expiryDate,
		})
	}

	grn, err := a.ProcurementService.CreateGRN(bizID, orderID, supplierID, warehouseID, notes, items)
	if err != nil {
		setToast(w, err.Error(), "error")
		http.Redirect(w, r, "/procurement/grn/new", http.StatusSeeOther)
		return
	}
	a.auditLog(r, "procurement_grn", "create", strconv.Itoa(grn.ID), map[string]string{
		"grn_number": grn.GRNNumber, "total_received": strconv.Itoa(grn.TotalReceived),
	})
	setToast(w, fmt.Sprintf("GRN %s created — %d items received", grn.GRNNumber, grn.TotalReceived), "success")
	http.Redirect(w, r, "/procurement/grn/view?id="+strconv.Itoa(grn.ID), http.StatusSeeOther)
}

func (a *App) GRNView(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil || id <= 0 {
		http.Error(w, "invalid GRN ID", http.StatusBadRequest)
		return
	}
	grn, err := a.ProcurementService.GetGRN(id, a.bizID(r))
	if err != nil {
		http.Error(w, "GRN not found", http.StatusNotFound)
		return
	}
	a.Renderer.Page(w, "grn_view.html", struct {
		AppContext
		GRN        *models.ProcurementGRN
		SellerName string
	}{AppContext: a.ctx(r), GRN: grn, SellerName: a.SellerName})
}

// ── Supplier Payments ─────────────────────────────────────────────────────────

type PaymentsPageData struct {
	AppContext
	Payments   []models.SupplierPayment
	Suppliers  []models.Supplier
	Orders     []models.ProcurementOrder
	TotalPaid  float64
	SupplierID int
}

func (a *App) PaymentsIndex(w http.ResponseWriter, r *http.Request) {
	bizID := a.bizID(r)
	supplierID, _ := strconv.Atoi(r.URL.Query().Get("supplier_id"))
	payments, _ := a.ProcurementService.ListPayments(bizID, supplierID)
	suppliers, _ := a.ProcurementService.ListSuppliers(bizID)
	orders, _ := a.ProcurementService.ListOrders(bizID, "completed")

	var totalPaid float64
	for _, p := range payments {
		totalPaid += p.Amount
	}
	a.Renderer.Page(w, "supplier_payments.html", PaymentsPageData{
		AppContext:  a.ctx(r),
		Payments:   payments,
		Suppliers:  suppliers,
		Orders:     orders,
		TotalPaid:  totalPaid,
		SupplierID: supplierID,
	})
}

func (a *App) PaymentCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	bizID := a.bizID(r)
	supplierID, _ := strconv.Atoi(r.FormValue("supplier_id"))
	orderID, _ := strconv.Atoi(r.FormValue("order_id"))
	amount, _ := strconv.ParseFloat(r.FormValue("amount"), 64)

	payment, err := a.ProcurementService.RecordPayment(
		bizID, supplierID, orderID, amount,
		r.FormValue("payment_method"), r.FormValue("reference"), r.FormValue("notes"),
	)
	if err != nil {
		setToast(w, err.Error(), "error")
	} else {
		a.auditLog(r, "supplier_payments", "create", strconv.Itoa(payment.ID), map[string]string{
			"amount": fmt.Sprintf("%.2f", payment.Amount),
		})
		setToast(w, fmt.Sprintf("Payment %s recorded — Rs. %.2f", payment.PaymentNumber, payment.Amount), "success")
	}
	http.Redirect(w, r, "/procurement/payments", http.StatusSeeOther)
}

// ── Reorder Suggestions ───────────────────────────────────────────────────────

func (a *App) ReorderSuggestions(w http.ResponseWriter, r *http.Request) {
	bizID := a.bizID(r)
	suggestions, _ := a.ProcurementService.ReorderSuggestions(bizID)
	suppliers, _ := a.ProcurementService.ListSuppliers(bizID)
	a.Renderer.Page(w, "reorder_suggestions.html", struct {
		AppContext
		Suggestions []models.ReorderSuggestion
		Suppliers   []models.Supplier
	}{AppContext: a.ctx(r), Suggestions: suggestions, Suppliers: suppliers})
}
