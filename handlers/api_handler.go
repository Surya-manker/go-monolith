package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"go-monolith/middleware"
	"go-monolith/models"
)

// ── Helpers ──────────────────────────────────────────────────────────────────

func apiJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func apiError(w http.ResponseWriter, status int, msg string) {
	apiJSON(w, status, map[string]string{"error": msg})
}

// ── Auth info ─────────────────────────────────────────────────────────────────

func (a *App) APIMe(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		apiError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	apiJSON(w, http.StatusOK, map[string]any{
		"id":    user.ID,
		"name":  user.Name,
		"email": user.Email,
		"role":  user.Role,
	})
}

// ── Products API ──────────────────────────────────────────────────────────────

func (a *App) APIProducts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		search := r.URL.Query().Get("search")
		products, err := a.ProductService.List(search)
		if err != nil {
			apiError(w, http.StatusInternalServerError, err.Error())
			return
		}
		apiJSON(w, http.StatusOK, map[string]any{"data": products, "count": len(products)})

	case http.MethodPost:
		p, err := productFromRequest(r)
		if err != nil {
			apiError(w, http.StatusBadRequest, err.Error())
			return
		}
		created, err := a.ProductService.Create(p)
		if err != nil {
			apiError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		apiJSON(w, http.StatusCreated, created)
	}
}

func (a *App) APIProduct(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		apiError(w, http.StatusBadRequest, "invalid id")
		return
	}
	switch r.Method {
	case http.MethodGet:
		p, err := a.ProductService.Get(id)
		if err != nil {
			apiError(w, http.StatusNotFound, "product not found")
			return
		}
		apiJSON(w, http.StatusOK, p)
	case http.MethodPut:
		p, err := productFromRequest(r)
		if err != nil {
			apiError(w, http.StatusBadRequest, err.Error())
			return
		}
		p.ID = id
		if err := a.ProductService.Update(p); err != nil {
			apiError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		updated, _ := a.ProductService.Get(id)
		apiJSON(w, http.StatusOK, updated)
	case http.MethodDelete:
		if err := a.ProductService.Delete(id); err != nil {
			apiError(w, http.StatusInternalServerError, err.Error())
			return
		}
		apiJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	}
}

// ── Generic module API ────────────────────────────────────────────────────────

func (a *App) APIModule(key string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			search := r.URL.Query().Get("search")
			_, result, err := a.ModuleService.ListPaged(key, 1, 100, search, "id", "desc")
			if err != nil {
				apiError(w, http.StatusInternalServerError, err.Error())
				return
			}
			apiJSON(w, http.StatusOK, map[string]any{
				"data":  result.Records,
				"total": result.Total,
				"page":  result.Page,
			})
		case http.MethodPost:
			_ = r.ParseForm()
			values := map[string]string{}
			for k, v := range r.Form {
				values[k] = strings.TrimSpace(v[0])
			}
			// Also support JSON body
			if r.Header.Get("Content-Type") == "application/json" {
				json.NewDecoder(r.Body).Decode(&values)
			}
			if err := a.ModuleService.Create(key, values); err != nil {
				apiError(w, http.StatusUnprocessableEntity, err.Error())
				return
			}
			apiJSON(w, http.StatusCreated, map[string]string{"status": "created"})
		}
	}
}

func (a *App) APIModuleRecord(key string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			apiError(w, http.StatusBadRequest, "invalid id")
			return
		}
		switch r.Method {
		case http.MethodGet:
			_, rec, err := a.ModuleService.Get(key, id)
			if err != nil {
				apiError(w, http.StatusNotFound, "not found")
				return
			}
			apiJSON(w, http.StatusOK, rec)
		case http.MethodPut:
			values := map[string]string{}
			if r.Header.Get("Content-Type") == "application/json" {
				json.NewDecoder(r.Body).Decode(&values)
			} else {
				_ = r.ParseForm()
				for k, v := range r.Form {
					values[k] = strings.TrimSpace(v[0])
				}
			}
			if err := a.ModuleService.Update(key, id, values); err != nil {
				apiError(w, http.StatusUnprocessableEntity, err.Error())
				return
			}
			_, rec, _ := a.ModuleService.Get(key, id)
			apiJSON(w, http.StatusOK, rec)
		case http.MethodDelete:
			if err := a.ModuleService.Delete(key, id); err != nil {
				apiError(w, http.StatusInternalServerError, err.Error())
				return
			}
			apiJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
		}
	}
}

// ── Dashboard stats API ───────────────────────────────────────────────────────

func (a *App) APIStats(w http.ResponseWriter, r *http.Request) {
	counts, err := a.ModuleService.Counts()
	if err != nil {
		apiError(w, http.StatusInternalServerError, err.Error())
		return
	}
	totals, err := a.ModuleService.Totals()
	if err != nil {
		apiError(w, http.StatusInternalServerError, err.Error())
		return
	}
	productCount, _ := a.ProductService.Count()
	lowStock, _ := a.ProductService.LowStockCount()
	pending, _ := a.ModuleService.PendingInvoicesTotal()

	apiJSON(w, http.StatusOK, map[string]any{
		"products":             productCount,
		"low_stock":            lowStock,
		"modules":              counts,
		"invoice_total":        totals["invoice_total"],
		"purchase_total":       totals["po_total"],
		"pending_invoice_total": pending,
	})
}

// ── Search API ────────────────────────────────────────────────────────────────

func (a *App) APISearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(q) < 2 {
		apiJSON(w, http.StatusOK, map[string]any{"results": []any{}, "query": q})
		return
	}

	type APIResult struct {
		Module string         `json:"module"`
		Path   string         `json:"path"`
		ID     string         `json:"id"`
		Title  string         `json:"title"`
		Record models.Record  `json:"record"`
	}

	var results []APIResult
	for _, key := range []string{"customers", "invoices", "vendors", "categories", "payments"} {
		cfg, ok := a.ModuleService.ConfigOnly(key)
		if !ok {
			continue
		}
		_, result, err := a.ModuleService.ListPaged(key, 1, 10, q, "", "")
		if err != nil {
			continue
		}
		for _, rec := range result.Records {
			title := rec["name"]
			if title == "" {
				title = rec["number"]
			}
			results = append(results, APIResult{
				Module: cfg.Title,
				Path:   cfg.Path,
				ID:     rec["id"],
				Title:  title,
				Record: rec,
			})
		}
	}

	apiJSON(w, http.StatusOK, map[string]any{"results": results, "query": q, "count": len(results)})
}
