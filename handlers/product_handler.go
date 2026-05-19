package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"go-monolith/models"
)

type ProductView struct {
	ID                int
	Name              string
	Description       string
	SKU               string
	Barcode           string
	HSNCode           string
	Brand             string
	Unit              string
	Status            string
	Price             string
	PriceValue        string
	CostPrice         string
	CostPriceValue    string
	Stock             int
	TaxRate           string
	LowStockThreshold int
	IsLowStock        bool
	CreatedAt         string
}

type ProductsPageData struct {
	AppContext
	Products []ProductView
	Edit     *ProductView
	Stock    *ProductView
	Search   string
	Error    string
}

func (a *App) ProductsIndex(w http.ResponseWriter, r *http.Request) {
	bizID := a.bizID(r)
	search := strings.TrimSpace(r.URL.Query().Get("search"))
	products, err := a.ProductService.List(search, bizID)
	if err != nil {
		http.Error(w, "could not load products", http.StatusInternalServerError)
		return
	}
	data := ProductsPageData{
		AppContext: a.ctx(r),
		Products:  productViews(products),
		Search:    search,
	}
	if r.Header.Get("HX-Request") == "true" {
		a.Renderer.Partial(w, "products_table.html", data)
		return
	}
	a.Renderer.Page(w, "products.html", data)
}

func (a *App) ProductsCreate(w http.ResponseWriter, r *http.Request) {
	bizID := a.bizID(r)
	product, err := productFromRequest(r)
	if err == nil {
		product.BusinessID = bizID // ALWAYS from auth, never from form
		_, err = a.ProductService.Create(product)
		if err == nil {
			setToast(w, "Product added successfully", "success")
		}
	}
	a.renderProductsTable(w, r, "", err)
}

func (a *App) ProductsEdit(w http.ResponseWriter, r *http.Request) {
	product, err := a.productFromID(r)
	if err != nil {
		http.Error(w, "product not found", http.StatusNotFound)
		return
	}
	view := productView(*product)
	a.Renderer.Partial(w, "product_form.html", ProductsPageData{Edit: &view})
}

func (a *App) ProductsUpdate(w http.ResponseWriter, r *http.Request) {
	bizID := a.bizID(r)
	id, err := intQuery(r, "id")
	if err != nil {
		http.Error(w, "invalid product id", http.StatusBadRequest)
		return
	}
	product, err := productFromRequest(r)
	if err == nil {
		product.ID = id
		product.BusinessID = bizID
		err = a.ProductService.Update(product)
		if err == nil {
			setToast(w, "Product updated successfully", "success")
		}
	}
	a.renderProductsTable(w, r, "", err)
}

func (a *App) ProductsDelete(w http.ResponseWriter, r *http.Request) {
	bizID := a.bizID(r)
	id, err := intQuery(r, "id")
	if err == nil {
		err = a.ProductService.Delete(id, bizID)
		if err == nil {
			setToast(w, "Product deleted", "warning")
		}
	}
	a.renderProductsTable(w, r, "", err)
}

func (a *App) ProductsStockForm(w http.ResponseWriter, r *http.Request) {
	product, err := a.productFromID(r)
	if err != nil {
		http.Error(w, "product not found", http.StatusNotFound)
		return
	}
	view := productView(*product)
	a.Renderer.Partial(w, "stock_form.html", ProductsPageData{Stock: &view})
}

func (a *App) ProductsAdjustStock(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	bizID := a.bizID(r)
	id, err := intQuery(r, "id")
	if err != nil {
		http.Error(w, "invalid product id", http.StatusBadRequest)
		return
	}
	quantity, err := strconv.Atoi(r.FormValue("quantity_change"))
	if err == nil {
		err = a.ProductService.AdjustStock(id, bizID, r.FormValue("change_type"), quantity, strings.TrimSpace(r.FormValue("note")))
		if err == nil {
			setToast(w, "Stock updated successfully", "success")
		}
	}
	a.renderProductsTable(w, r, "", err)
}

func (a *App) renderProductsTable(w http.ResponseWriter, r *http.Request, search string, err error) {
	products, listErr := a.ProductService.List(search, a.bizID(r))
	if listErr != nil {
		http.Error(w, "could not load products", http.StatusInternalServerError)
		return
	}
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	a.Renderer.Partial(w, "products_table.html", ProductsPageData{
		AppContext: a.ctx(r),
		Products:  productViews(products),
		Search:    search,
		Error:     msg,
	})
}

func (a *App) productFromID(r *http.Request) (*models.Product, error) {
	id, err := intQuery(r, "id")
	if err != nil {
		return nil, err
	}
	return a.ProductService.Get(id, a.bizID(r))
}

func productFromRequest(r *http.Request) (models.Product, error) {
	if err := r.ParseForm(); err != nil {
		return models.Product{}, err
	}
	price, err := parseFloat(r, "price")
	if err != nil {
		return models.Product{}, err
	}
	costPrice, _ := parseFloat(r, "cost_price")
	taxRate, _ := parseFloat(r, "tax_rate")
	stock, _ := strconv.Atoi(r.FormValue("stock"))
	threshold, _ := strconv.Atoi(r.FormValue("low_stock_threshold"))
	status := r.FormValue("status")
	if status == "" {
		status = "active"
	}
	unit := r.FormValue("unit")
	if unit == "" {
		unit = "pcs"
	}

	return models.Product{
		Name:              strings.TrimSpace(r.FormValue("name")),
		Description:       strings.TrimSpace(r.FormValue("description")),
		SKU:               strings.TrimSpace(r.FormValue("sku")),
		Barcode:           strings.TrimSpace(r.FormValue("barcode")),
		HSNCode:           strings.TrimSpace(r.FormValue("hsn_code")),
		Brand:             strings.TrimSpace(r.FormValue("brand")),
		Unit:              unit,
		Status:            status,
		Price:             price,
		CostPrice:         costPrice,
		Stock:             stock,
		TaxRate:           taxRate,
		LowStockThreshold: threshold,
	}, nil
}

func productViews(products []models.Product) []ProductView {
	views := make([]ProductView, 0, len(products))
	for _, p := range products {
		views = append(views, productView(p))
	}
	return views
}

func productView(p models.Product) ProductView {
	return ProductView{
		ID:                p.ID,
		Name:              p.Name,
		Description:       p.Description,
		SKU:               p.SKU,
		Barcode:           p.Barcode,
		HSNCode:           p.HSNCode,
		Brand:             p.Brand,
		Unit:              p.Unit,
		Status:            p.Status,
		Price:             money(p.Price),
		PriceValue:        number(p.Price),
		CostPrice:         money(p.CostPrice),
		CostPriceValue:    number(p.CostPrice),
		Stock:             p.Stock,
		TaxRate:           fmt.Sprintf("%.2f", p.TaxRate),
		LowStockThreshold: p.LowStockThreshold,
		IsLowStock:        p.Stock <= p.LowStockThreshold,
		CreatedAt:         p.CreatedAt.Format("02 Jan 2006"),
	}
}

func intQuery(r *http.Request, name string) (int, error) {
	return strconv.Atoi(r.URL.Query().Get(name))
}

func parseFloat(r *http.Request, name string) (float64, error) {
	if r.FormValue(name) == "" {
		return 0, nil
	}
	return strconv.ParseFloat(r.FormValue(name), 64)
}

func money(value float64) string  { return fmt.Sprintf("Rs. %.2f", value) }
func number(value float64) string { return fmt.Sprintf("%.2f", value) }
