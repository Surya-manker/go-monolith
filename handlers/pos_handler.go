package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"go-monolith/models"
	"go-monolith/services"
)

// cartSession returns the session token for the POS cart key.
func cartSession(r *http.Request) string {
	if c, err := r.Cookie("session"); err == nil {
		return "pos:" + c.Value
	}
	return "pos:anon"
}

// ── POS page data ─────────────────────────────────────────────────────────────

type POSPageData struct {
	AppContext
	Cart        *services.POSCart
	Warehouses  []models.Warehouse
	WarehouseID int
	Products    []models.Product
	Sales       []models.POSSale
	TodayCount  int
	TodayTotal  float64
	SellerName  string
}

// ── Main POS terminal ─────────────────────────────────────────────────────────

func (a *App) POSIndex(w http.ResponseWriter, r *http.Request) {
	bizID := a.bizID(r)
	cart := a.POSCarts.Get(cartSession(r))
	whs, _ := a.WarehouseService.List(bizID)
	products, _ := a.ProductService.List("", bizID)
	sales, _ := a.POSService.ListSales(bizID, 10)
	todayCount, todayTotal, _ := a.POSService.TodayTotal(bizID)

	whID, _ := strconv.Atoi(r.URL.Query().Get("warehouse_id"))
	if whID == 0 && len(whs) > 0 {
		for _, wh := range whs {
			if wh.IsDefault {
				whID = wh.ID
				break
			}
		}
		if whID == 0 {
			whID = whs[0].ID
		}
	}

	// pos_cart.html must be included because pos.html calls {{ template "pos_cart" . }}
	// and that template is defined in the separate partial, not inline.
	a.Renderer.PageWith(w, "pos.html", POSPageData{
		AppContext:  a.ctx(r),
		Cart:        cart,
		Warehouses:  whs,
		WarehouseID: whID,
		Products:    products,
		Sales:       sales,
		TodayCount:  todayCount,
		TodayTotal:  todayTotal,
		SellerName:  a.SellerName,
	}, "pos_cart.html")
}

// ── Product search / barcode lookup ──────────────────────────────────────────
// GET /pos/search?q=<query>&warehouse_id=<id>

func (a *App) POSSearch(w http.ResponseWriter, r *http.Request) {
	bizID := a.bizID(r)
	q := strings.TrimSpace(r.URL.Query().Get("q"))

	var products []models.Product
	if q != "" {
		// Try barcode exact match first.
		if p, err := a.ProductService.GetByBarcode(q, bizID); err == nil {
			products = []models.Product{*p}
		} else {
			products, _ = a.ProductService.List(q, bizID)
		}
	}

	a.Renderer.Partial(w, "pos_search_results.html", struct {
		Products []models.Product
		Query    string
	}{Products: products, Query: q})
}

// ── Cart operations ───────────────────────────────────────────────────────────

func (a *App) POSCartAdd(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	bizID := a.bizID(r)
	sess := cartSession(r)
	cart := a.POSCarts.Get(sess)

	productID, err := strconv.Atoi(r.FormValue("product_id"))
	if err != nil || productID <= 0 {
		setToast(w, "Invalid product", "error")
		a.renderCart(w, r, cart)
		return
	}
	qty, _ := strconv.Atoi(r.FormValue("quantity"))
	if qty <= 0 {
		qty = 1
	}

	product, err := a.ProductService.Get(productID, bizID)
	if err != nil {
		setToast(w, "Product not found", "error")
		a.renderCart(w, r, cart)
		return
	}

	cart.AddItem(services.CartItem{
		ProductID:   product.ID,
		ProductName: product.Name,
		SKU:         product.SKU,
		Barcode:     product.Barcode,
		UnitPrice:   product.Price,
		TaxRate:     product.TaxRate,
		Quantity:    qty,
	})
	a.POSCarts.Save(sess, cart)
	setToast(w, product.Name+" added to cart", "success")
	a.renderCart(w, r, cart)
}

func (a *App) POSCartUpdate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	sess := cartSession(r)
	cart := a.POSCarts.Get(sess)

	productID, _ := strconv.Atoi(r.FormValue("product_id"))

	// Support both absolute qty and relative delta (delta=+1 or delta=-1).
	if deltaStr := r.FormValue("delta"); deltaStr != "" {
		delta, _ := strconv.Atoi(deltaStr)
		current := 0
		for _, item := range cart.Items {
			if item.ProductID == productID {
				current = item.Quantity
				break
			}
		}
		cart.UpdateQty(productID, current+delta)
	} else {
		qty, _ := strconv.Atoi(r.FormValue("quantity"))
		cart.UpdateQty(productID, qty)
	}
	a.POSCarts.Save(sess, cart)
	a.renderCart(w, r, cart)
}

func (a *App) POSCartRemove(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	sess := cartSession(r)
	cart := a.POSCarts.Get(sess)

	productID, _ := strconv.Atoi(r.FormValue("product_id"))
	cart.RemoveItem(productID)
	a.POSCarts.Save(sess, cart)
	setToast(w, "Item removed", "warning")
	a.renderCart(w, r, cart)
}

func (a *App) POSCartClear(w http.ResponseWriter, r *http.Request) {
	sess := cartSession(r)
	a.POSCarts.Clear(sess)
	newCart := &services.POSCart{}
	setToast(w, "Cart cleared", "warning")
	a.renderCart(w, r, newCart)
}

func (a *App) renderCart(w http.ResponseWriter, r *http.Request, cart *services.POSCart) {
	bizID := a.bizID(r)
	whs, _ := a.WarehouseService.List(bizID)
	whID, _ := strconv.Atoi(r.FormValue("warehouse_id"))
	if whID == 0 {
		whID, _ = strconv.Atoi(r.URL.Query().Get("warehouse_id"))
	}
	a.Renderer.Partial(w, "pos_cart.html", struct {
		Cart        *services.POSCart
		Warehouses  []models.Warehouse
		WarehouseID int
		SellerName  string
	}{
		Cart:        cart,
		Warehouses:  whs,
		WarehouseID: whID,
		SellerName:  a.SellerName,
	})
}

// ── Checkout ─────────────────────────────────────────────────────────────────

func (a *App) POSCheckout(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	bizID := a.bizID(r)
	sess := cartSession(r)
	cart := a.POSCarts.Get(sess)

	if cart.IsEmpty() {
		setToast(w, "Cart is empty", "error")
		http.Redirect(w, r, "/pos", http.StatusSeeOther)
		return
	}

	whID, err := strconv.Atoi(r.FormValue("warehouse_id"))
	if err != nil || whID <= 0 {
		setToast(w, "Please select a warehouse", "error")
		http.Redirect(w, r, "/pos", http.StatusSeeOther)
		return
	}

	amountPaid, _ := strconv.ParseFloat(r.FormValue("amount_paid"), 64)
	discount, _ := strconv.ParseFloat(r.FormValue("discount"), 64)

	sale, err := a.POSService.Checkout(services.CheckoutInput{
		BusinessID:    bizID,
		WarehouseID:   whID,
		Cart:          cart,
		CustomerName:  strings.TrimSpace(r.FormValue("customer_name")),
		CustomerPhone: strings.TrimSpace(r.FormValue("customer_phone")),
		PaymentMethod: r.FormValue("payment_method"),
		AmountPaid:    amountPaid,
		Discount:      discount,
	})
	if err != nil {
		setToast(w, err.Error(), "error")
		http.Redirect(w, r, "/pos", http.StatusSeeOther)
		return
	}

	a.auditLog(r, "pos_sales", "create", strconv.Itoa(sale.ID), map[string]string{
		"sale_number": sale.SaleNumber,
		"grand_total": strconv.FormatFloat(sale.GrandTotal, 'f', 2, 64),
	})
	a.POSCarts.Clear(sess)
	http.Redirect(w, r, "/pos/receipt?id="+strconv.Itoa(sale.ID), http.StatusSeeOther)
}

// ── Receipt ───────────────────────────────────────────────────────────────────

type ReceiptPageData struct {
	AppContext
	Sale       *models.POSSale
	SellerName string
	SellerAddr string
	GSTIN      string
}

func (a *App) POSReceipt(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil || id <= 0 {
		http.Error(w, "invalid sale ID", http.StatusBadRequest)
		return
	}
	sale, err := a.POSService.GetSale(id, a.bizID(r))
	if err != nil {
		http.Error(w, "sale not found", http.StatusNotFound)
		return
	}
	a.Renderer.Page(w, "pos_receipt.html", ReceiptPageData{
		AppContext:  a.ctx(r),
		Sale:        sale,
		SellerName: a.SellerName,
		SellerAddr: a.SellerAddress,
		GSTIN:      a.SellerGSTIN,
	})
}

// ── Sales history ─────────────────────────────────────────────────────────────

func (a *App) POSSalesHistory(w http.ResponseWriter, r *http.Request) {
	bizID := a.bizID(r)
	sales, _ := a.POSService.ListSales(bizID, 100)
	todayCount, todayTotal, _ := a.POSService.TodayTotal(bizID)
	a.Renderer.Page(w, "pos_sales.html", struct {
		AppContext
		Sales      []models.POSSale
		TodayCount int
		TodayTotal float64
	}{
		AppContext:  a.ctx(r),
		Sales:      sales,
		TodayCount: todayCount,
		TodayTotal: todayTotal,
	})
}
