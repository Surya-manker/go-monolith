package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"go-monolith/models"
	"go-monolith/services"
)

// ── CRM Dashboard ─────────────────────────────────────────────────────────────

type CRMDashboardData struct {
	AppContext
	Stats       models.CRMStats
	RecentOrders []models.SalesOrder
	RecentQuotes []models.Quotation
	TopCustomers []models.CRMCustomer
}

func (a *App) CRMDashboard(w http.ResponseWriter, r *http.Request) {
	bizID := a.bizID(r)
	stats, _ := a.CRMService.Stats(bizID)

	orders, _ := a.CRMService.ListOrders(bizID, "")
	if len(orders) > 5 {
		orders = orders[:5]
	}
	quotes, _ := a.CRMService.ListQuotations(bizID, "")
	if len(quotes) > 5 {
		quotes = quotes[:5]
	}
	customers, _ := a.CRMService.ListCustomers(bizID)
	top := customers
	if len(top) > 5 {
		top = top[:5]
	}

	a.Renderer.Page(w, "crm_dashboard.html", CRMDashboardData{
		AppContext:    a.ctx(r),
		Stats:         stats,
		RecentOrders:  orders,
		RecentQuotes:  quotes,
		TopCustomers:  top,
	})
}

// ── Customers ─────────────────────────────────────────────────────────────────

type CRMCustomersData struct {
	AppContext
	Customers []models.CRMCustomer
	Edit      *models.CRMCustomer
	Error     string
}

func (a *App) CRMCustomersIndex(w http.ResponseWriter, r *http.Request) {
	customers, _ := a.CRMService.ListCustomers(a.bizID(r))
	a.Renderer.Page(w, "crm_customers.html", CRMCustomersData{AppContext: a.ctx(r), Customers: customers})
}

func (a *App) CRMCustomerCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	bizID := a.bizID(r)
	creditLimit, _ := strconv.ParseFloat(r.FormValue("credit_limit"), 64)
	payTerms, _ := strconv.Atoi(r.FormValue("payment_terms"))

	cust, err := a.CRMService.CreateCustomer(
		bizID,
		r.FormValue("name"), r.FormValue("email"), r.FormValue("phone"),
		r.FormValue("gstin"), r.FormValue("pan"),
		r.FormValue("billing_address"), r.FormValue("shipping_address"),
		r.FormValue("contact_person"), r.FormValue("customer_group"),
		r.FormValue("customer_code"), r.FormValue("notes"),
		creditLimit, payTerms, "active",
	)
	if err != nil {
		customers, _ := a.CRMService.ListCustomers(bizID)
		setToast(w, err.Error(), "error")
		a.Renderer.Partial(w, "crm_customers_table.html", CRMCustomersData{AppContext: a.ctx(r), Customers: customers, Error: err.Error()})
		return
	}
	a.auditLog(r, "crm_customers", "create", strconv.Itoa(cust.ID), map[string]string{"name": cust.Name})
	setToast(w, "Customer "+cust.Name+" added", "success")
	customers, _ := a.CRMService.ListCustomers(bizID)
	a.Renderer.Partial(w, "crm_customers_table.html", CRMCustomersData{AppContext: a.ctx(r), Customers: customers})
}

func (a *App) CRMCustomerEdit(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil || id <= 0 {
		http.Error(w, "invalid ID", http.StatusBadRequest)
		return
	}
	bizID := a.bizID(r)
	cust, err := a.CRMService.GetCustomer(id, bizID)
	if err != nil {
		http.Error(w, "customer not found", http.StatusNotFound)
		return
	}
	customers, _ := a.CRMService.ListCustomers(bizID)
	a.Renderer.Partial(w, "crm_customers_table.html", CRMCustomersData{AppContext: a.ctx(r), Customers: customers, Edit: cust})
}

func (a *App) CRMCustomerUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil || id <= 0 {
		http.Error(w, "invalid ID", http.StatusBadRequest)
		return
	}
	if err = r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	bizID := a.bizID(r)
	creditLimit, _ := strconv.ParseFloat(r.FormValue("credit_limit"), 64)
	payTerms, _ := strconv.Atoi(r.FormValue("payment_terms"))

	err = a.CRMService.UpdateCustomer(
		id, bizID,
		r.FormValue("name"), r.FormValue("email"), r.FormValue("phone"),
		r.FormValue("gstin"), r.FormValue("pan"),
		r.FormValue("billing_address"), r.FormValue("shipping_address"),
		r.FormValue("contact_person"), r.FormValue("customer_group"),
		r.FormValue("status"), r.FormValue("notes"),
		creditLimit, payTerms,
	)
	if err != nil {
		customers, _ := a.CRMService.ListCustomers(bizID)
		setToast(w, err.Error(), "error")
		a.Renderer.Partial(w, "crm_customers_table.html", CRMCustomersData{AppContext: a.ctx(r), Customers: customers, Error: err.Error()})
		return
	}
	a.auditLog(r, "crm_customers", "update", strconv.Itoa(id), nil)
	setToast(w, "Customer updated", "success")
	customers, _ := a.CRMService.ListCustomers(bizID)
	a.Renderer.Partial(w, "crm_customers_table.html", CRMCustomersData{AppContext: a.ctx(r), Customers: customers})
}

func (a *App) CRMCustomerView(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil || id <= 0 {
		http.Error(w, "invalid ID", http.StatusBadRequest)
		return
	}
	bizID := a.bizID(r)
	cust, err := a.CRMService.GetCustomer(id, bizID)
	if err != nil {
		http.Error(w, "customer not found", http.StatusNotFound)
		return
	}
	orders, _ := a.CRMService.ListOrders(bizID, "")
	payments, _ := a.CRMService.ListPayments(bizID, id)

	var custOrders []models.SalesOrder
	for _, o := range orders {
		if o.CustomerID == id {
			custOrders = append(custOrders, o)
		}
	}

	a.Renderer.Page(w, "crm_customer_view.html", struct {
		AppContext
		Customer *models.CRMCustomer
		Orders   []models.SalesOrder
		Payments []models.CustomerPayment
	}{AppContext: a.ctx(r), Customer: cust, Orders: custOrders, Payments: payments})
}

// ── Quotations ────────────────────────────────────────────────────────────────

type QuotationsData struct {
	AppContext
	Quotations   []models.Quotation
	Customers    []models.CRMCustomer
	Warehouses   []models.Warehouse
	Products     []models.Product
	StatusFilter string
}

func (a *App) QuotationsIndex(w http.ResponseWriter, r *http.Request) {
	bizID := a.bizID(r)
	status := r.URL.Query().Get("status")
	quotes, _ := a.CRMService.ListQuotations(bizID, status)
	a.Renderer.Page(w, "crm_quotations.html", struct {
		AppContext
		Quotations   []models.Quotation
		StatusFilter string
	}{AppContext: a.ctx(r), Quotations: quotes, StatusFilter: status})
}

func (a *App) QuotationNew(w http.ResponseWriter, r *http.Request) {
	bizID := a.bizID(r)
	a.Renderer.Page(w, "crm_quotation_new.html", QuotationsData{
		AppContext:  a.ctx(r),
		Customers:  mustListCustomers(a, bizID),
		Warehouses: mustListWarehouses(a, bizID),
		Products:   mustListProducts(a, bizID),
	})
}

func (a *App) QuotationCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	bizID := a.bizID(r)
	customerID, _ := strconv.Atoi(r.FormValue("customer_id"))
	warehouseID, _ := strconv.Atoi(r.FormValue("warehouse_id"))

	var validUntil *time.Time
	if v := r.FormValue("valid_until"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			validUntil = &t
		}
	}

	items := parseSalesItems(r)
	qt, err := a.CRMService.CreateQuotation(bizID, customerID, warehouseID, validUntil, r.FormValue("notes"), items)
	if err != nil {
		setToast(w, err.Error(), "error")
		http.Redirect(w, r, "/crm/quotations/new", http.StatusSeeOther)
		return
	}
	a.auditLog(r, "quotations", "create", strconv.Itoa(qt.ID), map[string]string{"quote_number": qt.QuoteNumber})
	setToast(w, "Quotation "+qt.QuoteNumber+" created", "success")
	http.Redirect(w, r, "/crm/quotations/view?id="+strconv.Itoa(qt.ID), http.StatusSeeOther)
}

func (a *App) QuotationView(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil || id <= 0 {
		http.Error(w, "invalid ID", http.StatusBadRequest)
		return
	}
	qt, err := a.CRMService.GetQuotation(id, a.bizID(r))
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	a.Renderer.Page(w, "crm_quotation_view.html", struct {
		AppContext
		Quote      *models.Quotation
		SellerName string
	}{AppContext: a.ctx(r), Quote: qt, SellerName: a.SellerName})
}

func (a *App) QuotationSend(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.URL.Query().Get("id"))
	if err := a.CRMService.SendQuotation(id, a.bizID(r)); err != nil {
		setToast(w, err.Error(), "error")
	} else {
		setToast(w, "Quotation marked as sent", "success")
	}
	http.Redirect(w, r, "/crm/quotations/view?id="+strconv.Itoa(id), http.StatusSeeOther)
}

func (a *App) QuotationApprove(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.URL.Query().Get("id"))
	if err := a.CRMService.ApproveQuotation(id, a.bizID(r)); err != nil {
		setToast(w, err.Error(), "error")
	} else {
		a.auditLog(r, "quotations", "approve", strconv.Itoa(id), nil)
		setToast(w, "Quotation approved", "success")
	}
	http.Redirect(w, r, "/crm/quotations/view?id="+strconv.Itoa(id), http.StatusSeeOther)
}

func (a *App) QuotationReject(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.URL.Query().Get("id"))
	if err := a.CRMService.RejectQuotation(id, a.bizID(r)); err != nil {
		setToast(w, err.Error(), "error")
	} else {
		setToast(w, "Quotation rejected", "warning")
	}
	http.Redirect(w, r, "/crm/quotations/view?id="+strconv.Itoa(id), http.StatusSeeOther)
}

func (a *App) QuotationConvert(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.URL.Query().Get("id"))
	so, err := a.CRMService.ConvertQuotation(id, a.bizID(r))
	if err != nil {
		setToast(w, err.Error(), "error")
		http.Redirect(w, r, "/crm/quotations/view?id="+strconv.Itoa(id), http.StatusSeeOther)
		return
	}
	a.auditLog(r, "quotations", "convert", strconv.Itoa(id), map[string]string{"order_number": so.OrderNumber})
	setToast(w, "Quotation converted to Sales Order "+so.OrderNumber, "success")
	http.Redirect(w, r, "/crm/orders/view?id="+strconv.Itoa(so.ID), http.StatusSeeOther)
}

// ── Sales Orders ──────────────────────────────────────────────────────────────

func (a *App) SalesOrdersIndex(w http.ResponseWriter, r *http.Request) {
	bizID := a.bizID(r)
	status := r.URL.Query().Get("status")
	orders, _ := a.CRMService.ListOrders(bizID, status)
	a.Renderer.Page(w, "crm_orders.html", struct {
		AppContext
		Orders       []models.SalesOrder
		StatusFilter string
	}{AppContext: a.ctx(r), Orders: orders, StatusFilter: status})
}

func (a *App) SalesOrderNew(w http.ResponseWriter, r *http.Request) {
	bizID := a.bizID(r)
	a.Renderer.Page(w, "crm_order_new.html", struct {
		AppContext
		Customers  []models.CRMCustomer
		Warehouses []models.Warehouse
		Products   []models.Product
	}{
		AppContext:  a.ctx(r),
		Customers:  mustListCustomers(a, bizID),
		Warehouses: mustListWarehouses(a, bizID),
		Products:   mustListProducts(a, bizID),
	})
}

func (a *App) SalesOrderCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	bizID := a.bizID(r)
	customerID, _ := strconv.Atoi(r.FormValue("customer_id"))
	warehouseID, _ := strconv.Atoi(r.FormValue("warehouse_id"))

	var deliveryDate *time.Time
	if v := r.FormValue("delivery_date"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			deliveryDate = &t
		}
	}

	items := parseSalesItems(r)
	so, err := a.CRMService.CreateSalesOrder(bizID, customerID, warehouseID, nil, r.FormValue("shipping_address"), deliveryDate, r.FormValue("notes"), items)
	if err != nil {
		setToast(w, err.Error(), "error")
		http.Redirect(w, r, "/crm/orders/new", http.StatusSeeOther)
		return
	}
	a.auditLog(r, "sales_orders", "create", strconv.Itoa(so.ID), map[string]string{"order_number": so.OrderNumber})
	setToast(w, "Sales Order "+so.OrderNumber+" created", "success")
	http.Redirect(w, r, "/crm/orders/view?id="+strconv.Itoa(so.ID), http.StatusSeeOther)
}

func (a *App) SalesOrderView(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil || id <= 0 {
		http.Error(w, "invalid ID", http.StatusBadRequest)
		return
	}
	so, err := a.CRMService.GetOrder(id, a.bizID(r))
	if err != nil {
		http.Error(w, "order not found", http.StatusNotFound)
		return
	}
	a.Renderer.Page(w, "crm_order_view.html", struct {
		AppContext
		Order      *models.SalesOrder
		SellerName string
	}{AppContext: a.ctx(r), Order: so, SellerName: a.SellerName})
}

func (a *App) SalesOrderConfirm(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.URL.Query().Get("id"))
	if err := a.CRMService.ConfirmOrder(id, a.bizID(r)); err != nil {
		setToast(w, err.Error(), "error")
	} else {
		a.auditLog(r, "sales_orders", "confirm", strconv.Itoa(id), nil)
		setToast(w, "Order confirmed — stock reserved", "success")
	}
	http.Redirect(w, r, "/crm/orders/view?id="+strconv.Itoa(id), http.StatusSeeOther)
}

func (a *App) SalesOrderPack(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.URL.Query().Get("id"))
	if err := a.CRMService.PackOrder(id, a.bizID(r)); err != nil {
		setToast(w, err.Error(), "error")
	} else {
		setToast(w, "Order marked as packed", "success")
	}
	http.Redirect(w, r, "/crm/orders/view?id="+strconv.Itoa(id), http.StatusSeeOther)
}

func (a *App) SalesOrderCancel(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.URL.Query().Get("id"))
	if err := a.CRMService.CancelOrder(id, a.bizID(r)); err != nil {
		setToast(w, err.Error(), "error")
	} else {
		a.auditLog(r, "sales_orders", "cancel", strconv.Itoa(id), nil)
		setToast(w, "Order cancelled — reservation released", "warning")
	}
	http.Redirect(w, r, "/crm/orders/view?id="+strconv.Itoa(id), http.StatusSeeOther)
}

// ── Delivery Challans ─────────────────────────────────────────────────────────

func (a *App) DeliveryIndex(w http.ResponseWriter, r *http.Request) {
	challans, _ := a.CRMService.ListChallans(a.bizID(r))
	a.Renderer.Page(w, "crm_delivery.html", struct {
		AppContext
		Challans []models.DeliveryChallan
	}{AppContext: a.ctx(r), Challans: challans})
}

func (a *App) DeliveryNew(w http.ResponseWriter, r *http.Request) {
	bizID := a.bizID(r)
	orderID, _ := strconv.Atoi(r.URL.Query().Get("order_id"))
	var order *models.SalesOrder
	if orderID > 0 {
		order, _ = a.CRMService.GetOrder(orderID, bizID)
	}
	a.Renderer.Page(w, "crm_delivery_new.html", struct {
		AppContext
		Order *models.SalesOrder
	}{AppContext: a.ctx(r), Order: order})
}

func (a *App) DeliveryCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	bizID := a.bizID(r)
	orderID, _ := strconv.Atoi(r.FormValue("order_id"))

	productIDs := r.Form["product_id[]"]
	orderItemIDs := r.Form["order_item_id[]"]
	quantities := r.Form["quantity[]"]

	var items []services.ChallanItemInput
	for i := range productIDs {
		pid, _ := strconv.Atoi(safeIndex(productIDs, i))
		qty, _ := strconv.Atoi(safeIndex(quantities, i))
		oiID, _ := strconv.Atoi(safeIndex(orderItemIDs, i))
		if pid > 0 && qty > 0 {
			items = append(items, services.ChallanItemInput{
				OrderItemID: oiID, ProductID: pid, Quantity: qty,
			})
		}
	}

	challan, err := a.CRMService.CreateDeliveryChallan(
		bizID, orderID,
		r.FormValue("courier_name"), r.FormValue("tracking_number"), r.FormValue("notes"),
		items,
	)
	if err != nil {
		setToast(w, err.Error(), "error")
		http.Redirect(w, r, "/crm/delivery/new?order_id="+strconv.Itoa(orderID), http.StatusSeeOther)
		return
	}
	a.auditLog(r, "delivery_challans", "dispatch", strconv.Itoa(challan.ID), map[string]string{"challan_number": challan.ChallanNumber})
	setToast(w, fmt.Sprintf("Challan %s dispatched — stock deducted", challan.ChallanNumber), "success")
	http.Redirect(w, r, "/crm/delivery/view?id="+strconv.Itoa(challan.ID), http.StatusSeeOther)
}

func (a *App) DeliveryView(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil || id <= 0 {
		http.Error(w, "invalid ID", http.StatusBadRequest)
		return
	}
	challan, err := a.CRMService.GetChallan(id, a.bizID(r))
	if err != nil {
		http.Error(w, "challan not found", http.StatusNotFound)
		return
	}
	a.Renderer.Page(w, "crm_delivery_view.html", struct {
		AppContext
		Challan    *models.DeliveryChallan
		SellerName string
	}{AppContext: a.ctx(r), Challan: challan, SellerName: a.SellerName})
}

func (a *App) DeliveryMarkDelivered(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.URL.Query().Get("id"))
	if err := a.CRMService.MarkDelivered(id, a.bizID(r)); err != nil {
		setToast(w, err.Error(), "error")
	} else {
		setToast(w, "Delivery confirmed", "success")
	}
	http.Redirect(w, r, "/crm/delivery/view?id="+strconv.Itoa(id), http.StatusSeeOther)
}

// ── Customer Payments ─────────────────────────────────────────────────────────

func (a *App) CRMPaymentsIndex(w http.ResponseWriter, r *http.Request) {
	bizID := a.bizID(r)
	customerID, _ := strconv.Atoi(r.URL.Query().Get("customer_id"))
	payments, _ := a.CRMService.ListPayments(bizID, customerID)
	customers, _ := a.CRMService.ListCustomers(bizID)
	orders, _ := a.CRMService.ListOrders(bizID, "")

	var totalPaid float64
	for _, p := range payments {
		totalPaid += p.Amount
	}
	a.Renderer.Page(w, "crm_payments.html", struct {
		AppContext
		Payments   []models.CustomerPayment
		Customers  []models.CRMCustomer
		Orders     []models.SalesOrder
		TotalPaid  float64
		CustomerID int
	}{AppContext: a.ctx(r), Payments: payments, Customers: customers, Orders: orders, TotalPaid: totalPaid, CustomerID: customerID})
}

func (a *App) CRMPaymentCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	bizID := a.bizID(r)
	customerID, _ := strconv.Atoi(r.FormValue("customer_id"))
	orderID, _ := strconv.Atoi(r.FormValue("order_id"))
	amount, _ := strconv.ParseFloat(r.FormValue("amount"), 64)

	payment, err := a.CRMService.RecordPayment(
		bizID, customerID, orderID, amount,
		r.FormValue("payment_method"), r.FormValue("payment_type"),
		r.FormValue("reference"), r.FormValue("notes"),
	)
	if err != nil {
		setToast(w, err.Error(), "error")
	} else {
		a.auditLog(r, "customer_payments", "create", strconv.Itoa(payment.ID), map[string]string{
			"amount": fmt.Sprintf("%.2f", payment.Amount),
		})
		setToast(w, fmt.Sprintf("Payment %s recorded — Rs. %.2f", payment.PaymentNumber, payment.Amount), "success")
	}
	http.Redirect(w, r, "/crm/payments", http.StatusSeeOther)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func parseSalesItems(r *http.Request) []services.OrderItemInput {
	productIDs := r.Form["product_id[]"]
	quantities := r.Form["quantity[]"]
	prices := r.Form["unit_price[]"]
	taxRates := r.Form["tax_rate[]"]
	discounts := r.Form["discount[]"]

	var items []services.OrderItemInput
	for i := range productIDs {
		pid, _ := strconv.Atoi(safeIndex(productIDs, i))
		qty, _ := strconv.Atoi(safeIndex(quantities, i))
		price, _ := strconv.ParseFloat(safeIndex(prices, i), 64)
		taxRate, _ := strconv.ParseFloat(safeIndex(taxRates, i), 64)
		discount, _ := strconv.ParseFloat(safeIndex(discounts, i), 64)
		if pid > 0 && qty > 0 {
			items = append(items, services.OrderItemInput{
				ProductID: pid, Quantity: qty, UnitPrice: price, TaxRate: taxRate, Discount: discount,
			})
		}
	}
	return items
}

func mustListCustomers(a *App, bizID int) []models.CRMCustomer {
	c, _ := a.CRMService.ListCustomers(bizID)
	return c
}

func mustListWarehouses(a *App, bizID int) []models.Warehouse {
	w, _ := a.WarehouseService.List(bizID)
	return w
}

func mustListProducts(a *App, bizID int) []models.Product {
	p, _ := a.ProductService.List("", bizID)
	return p
}
