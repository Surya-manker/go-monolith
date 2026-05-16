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
	search := strings.TrimSpace(r.URL.Query().Get("search"))
	products, err := a.ProductService.List(search)
	if err != nil {
		http.Error(w, "could not load products", http.StatusInternalServerError)
		return
	}
	data := ProductsPageData{
		AppContext: a.ctx(r),
		Products:  productViews(products),
		Search:    search,
	}
	// HTMX search requests only need the table fragment.
	if r.Header.Get("HX-Request") == "true" {
		a.Renderer.Partial(w, "products_table.html", data)
		return
	}
	a.Renderer.Page(w, "products.html", data)
}

func (a *App) ProductsCreate(w http.ResponseWriter, r *http.Request) {
	product, err := productFromRequest(r)
	if err == nil {
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
	id, err := intQuery(r, "id")
	if err != nil {
		http.Error(w, "invalid product id", http.StatusBadRequest)
		return
	}
	product, err := productFromRequest(r)
	if err == nil {
		product.ID = id
		err = a.ProductService.Update(product)
		if err == nil {
			setToast(w, "Product updated successfully", "success")
		}
	}
	a.renderProductsTable(w, r, "", err)
}

func (a *App) ProductsDelete(w http.ResponseWriter, r *http.Request) {
	id, err := intQuery(r, "id")
	if err == nil {
		err = a.ProductService.Delete(id)
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
	id, err := intQuery(r, "id")
	if err != nil {
		http.Error(w, "invalid product id", http.StatusBadRequest)
		return
	}
	quantity, err := strconv.Atoi(r.FormValue("quantity_change"))
	if err == nil {
		err = a.ProductService.AdjustStock(id, r.FormValue("change_type"), quantity, strings.TrimSpace(r.FormValue("note")))
		if err == nil {
			setToast(w, "Stock updated successfully", "success")
		}
	}
	a.renderProductsTable(w, r, "", err)
}

func (a *App) renderProductsTable(w http.ResponseWriter, r *http.Request, search string, err error) {
	products, listErr := a.ProductService.List(search)
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
	return a.ProductService.Get(id)
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

	return models.Product{
		Name:              strings.TrimSpace(r.FormValue("name")),
		Description:       strings.TrimSpace(r.FormValue("description")),
		SKU:               strings.TrimSpace(r.FormValue("sku")),
		Price:             price,
		CostPrice:         costPrice,
		Stock:             stock,
		TaxRate:           taxRate,
		LowStockThreshold: threshold,
	}, nil
}

func productViews(products []models.Product) []ProductView {
	views := make([]ProductView, 0, len(products))
	for _, product := range products {
		views = append(views, productView(product))
	}
	return views
}

func productView(product models.Product) ProductView {
	return ProductView{
		ID:                product.ID,
		Name:              product.Name,
		Description:       product.Description,
		SKU:               product.SKU,
		Price:             money(product.Price),
		PriceValue:        number(product.Price),
		CostPrice:         money(product.CostPrice),
		CostPriceValue:    number(product.CostPrice),
		Stock:             product.Stock,
		TaxRate:           fmt.Sprintf("%.2f", product.TaxRate),
		LowStockThreshold: product.LowStockThreshold,
		IsLowStock:        product.Stock <= product.LowStockThreshold,
		CreatedAt:         product.CreatedAt.Format("02 Jan 2006"),
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

func money(value float64) string {
	return fmt.Sprintf("Rs. %.2f", value)
}

func number(value float64) string {
	return fmt.Sprintf("%.2f", value)
}
