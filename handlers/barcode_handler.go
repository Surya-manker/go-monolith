package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"go-monolith/models"
)

// ── Barcode management page ───────────────────────────────────────────────────

type BarcodesPageData struct {
	AppContext
	Products []models.Product
	Search   string
}

func (a *App) BarcodesIndex(w http.ResponseWriter, r *http.Request) {
	bizID := a.bizID(r)
	search := strings.TrimSpace(r.URL.Query().Get("search"))
	products, err := a.ProductService.List(search, bizID)
	if err != nil {
		http.Error(w, "could not load products", http.StatusInternalServerError)
		return
	}
	a.Renderer.Page(w, "barcodes.html", BarcodesPageData{
		AppContext: a.ctx(r),
		Products:  products,
		Search:    search,
	})
}

// ── Auto-generate barcodes ───────────────────────────────────────────────────

func (a *App) BarcodeAutoGenerate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	bizID := a.bizID(r)

	// If specific product ID, only auto-generate for that product.
	if idStr := r.FormValue("product_id"); idStr != "" {
		id, err := strconv.Atoi(idStr)
		if err != nil || id <= 0 {
			http.Error(w, "invalid product ID", http.StatusBadRequest)
			return
		}
		if err = a.ProductService.AutoGenerateBarcode(id, bizID, a.BarcodeService); err != nil {
			setToast(w, err.Error(), "error")
		} else {
			setToast(w, "Barcode generated", "success")
		}
	} else {
		// Bulk: generate for all products missing a barcode.
		count, err := a.ProductService.AutoGenerateAllBarcodes(bizID, a.BarcodeService)
		if err != nil {
			setToast(w, err.Error(), "error")
		} else {
			setToast(w, strconv.Itoa(count)+" barcodes generated", "success")
		}
	}

	products, _ := a.ProductService.List("", bizID)
	a.Renderer.Partial(w, "barcodes_table.html", BarcodesPageData{
		AppContext: a.ctx(r),
		Products:  products,
	})
}

// ── Barcode image endpoint ────────────────────────────────────────────────────
// GET /barcodes/image?id=<productID>&type=code128|ean13|qr&w=300&h=80

func (a *App) BarcodeImage(w http.ResponseWriter, r *http.Request) {
	bizID := a.bizID(r)
	id, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil || id <= 0 {
		http.Error(w, "invalid product ID", http.StatusBadRequest)
		return
	}
	product, err := a.ProductService.Get(id, bizID)
	if err != nil {
		http.Error(w, "product not found", http.StatusNotFound)
		return
	}

	barcodeText := product.Barcode
	if barcodeText == "" {
		barcodeText = product.SKU
	}
	if barcodeText == "" {
		barcodeText = strconv.Itoa(id)
	}

	btype := r.URL.Query().Get("type")
	ww, _ := strconv.Atoi(r.URL.Query().Get("w"))
	hh, _ := strconv.Atoi(r.URL.Query().Get("h"))
	if ww <= 0 {
		ww = 300
	}
	if hh <= 0 {
		hh = 80
	}

	var pngBytes []byte
	switch btype {
	case "qr":
		if ww > hh {
			ww = hh
		}
		pngBytes, err = a.BarcodeService.GenerateQR(barcodeText, ww)
	case "ean13":
		if len(barcodeText) < 12 || len(barcodeText) > 13 {
			// Fall back to auto-generated EAN-13
			barcodeText = a.BarcodeService.AutoGenerateEAN13(bizID, id)
		}
		pngBytes, err = a.BarcodeService.GenerateEAN13(barcodeText, ww, hh)
	default: // code128
		pngBytes, err = a.BarcodeService.GenerateCode128(barcodeText, ww, hh)
	}
	if err != nil {
		// Fallback: plain Code128 with product ID
		pngBytes, err = a.BarcodeService.GenerateCode128("PROD-"+strconv.Itoa(id), ww, hh)
		if err != nil {
			http.Error(w, "barcode generation failed", http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.WriteHeader(http.StatusOK)
	w.Write(pngBytes)
}

// ── Printable label sheet ─────────────────────────────────────────────────────

type LabelsPageData struct {
	AppContext
	Products  []models.Product
	BarcodeType string
	LabelSize   string
}

func (a *App) BarcodeLabels(w http.ResponseWriter, r *http.Request) {
	bizID := a.bizID(r)

	// Parse selected product IDs from query string: ?ids=1,2,3
	var products []models.Product
	if idsStr := r.URL.Query().Get("ids"); idsStr != "" {
		for _, part := range strings.Split(idsStr, ",") {
			id, err := strconv.Atoi(strings.TrimSpace(part))
			if err != nil || id <= 0 {
				continue
			}
			p, err := a.ProductService.Get(id, bizID)
			if err == nil {
				products = append(products, *p)
			}
		}
	} else {
		products, _ = a.ProductService.List("", bizID)
	}

	a.Renderer.Page(w, "barcode_labels.html", LabelsPageData{
		AppContext:   a.ctx(r),
		Products:    products,
		BarcodeType: r.URL.Query().Get("type"),
		LabelSize:   r.URL.Query().Get("size"),
	})
}

// ── Barcode product lookup (used by POS scanner + barcode search) ─────────────
// GET /barcodes/lookup?code=<barcode>  — returns JSON or HTML fragment

func (a *App) BarcodeLookup(w http.ResponseWriter, r *http.Request) {
	bizID := a.bizID(r)
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if code == "" {
		http.Error(w, "code is required", http.StatusBadRequest)
		return
	}

	product, err := a.ProductService.GetByBarcode(code, bizID)
	if err != nil {
		// Also try SKU lookup
		products, _ := a.ProductService.List(code, bizID)
		if len(products) == 0 {
			http.Error(w, "product not found", http.StatusNotFound)
			return
		}
		product = &products[0]
	}

	view := productView(*product)
	a.Renderer.Partial(w, "pos_product_card.html", struct {
		Product ProductView
	}{Product: view})
}
