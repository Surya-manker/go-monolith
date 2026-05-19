package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"go-monolith/middleware"
	"go-monolith/models"
)

func apiJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func apiError(w http.ResponseWriter, status int, msg string) {
	apiJSON(w, status, map[string]string{"error": msg})
}

func (a *App) APIMe(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		apiError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	apiJSON(w, http.StatusOK, map[string]any{
		"id":          user.ID,
		"business_id": user.BusinessID,
		"name":        user.Name,
		"email":       user.Email,
		"role":        user.Role,
	})
}

func (a *App) APIProducts(w http.ResponseWriter, r *http.Request) {
	bizID := a.bizID(r)
	switch r.Method {
	case http.MethodGet:
		products, err := a.ProductService.List(r.URL.Query().Get("search"), bizID)
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
		p.BusinessID = bizID
		created, err := a.ProductService.Create(p)
		if err != nil {
			apiError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		apiJSON(w, http.StatusCreated, created)
	}
}

func (a *App) APIProduct(w http.ResponseWriter, r *http.Request) {
	bizID := a.bizID(r)
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		apiError(w, http.StatusBadRequest, "invalid id")
		return
	}
	switch r.Method {
	case http.MethodGet:
		p, err := a.ProductService.Get(id, bizID)
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
		p.BusinessID = bizID
		if err := a.ProductService.Update(p); err != nil {
			apiError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		updated, _ := a.ProductService.Get(id, bizID)
		apiJSON(w, http.StatusOK, updated)
	case http.MethodDelete:
		if err := a.ProductService.Delete(id, bizID); err != nil {
			apiError(w, http.StatusInternalServerError, err.Error())
			return
		}
		apiJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	}
}

func (a *App) APIModule(key string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bizID := a.bizID(r)
		switch r.Method {
		case http.MethodGet:
			_, result, err := a.moduleService(r).ListPaged(key, 1, 100, r.URL.Query().Get("search"), "id", "desc", bizID)
			if err != nil {
				apiError(w, http.StatusInternalServerError, err.Error())
				return
			}
			apiJSON(w, http.StatusOK, map[string]any{
				"data": result.Records, "total": result.Total, "page": result.Page,
			})
		case http.MethodPost:
			values := parseAPIValues(r)
			if err := a.moduleService(r).Create(key, values, bizID); err != nil {
				apiError(w, http.StatusUnprocessableEntity, err.Error())
				return
			}
			apiJSON(w, http.StatusCreated, map[string]string{"status": "created"})
		}
	}
}

func (a *App) APIModuleRecord(key string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bizID := a.bizID(r)
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			apiError(w, http.StatusBadRequest, "invalid id")
			return
		}
		switch r.Method {
		case http.MethodGet:
			_, rec, err := a.moduleService(r).Get(key, id, bizID)
			if err != nil {
				apiError(w, http.StatusNotFound, "not found")
				return
			}
			apiJSON(w, http.StatusOK, rec)
		case http.MethodPut:
			values := parseAPIValues(r)
			if err := a.moduleService(r).Update(key, id, values, bizID); err != nil {
				apiError(w, http.StatusUnprocessableEntity, err.Error())
				return
			}
			_, rec, _ := a.moduleService(r).Get(key, id, bizID)
			apiJSON(w, http.StatusOK, rec)
		case http.MethodDelete:
			if err := a.moduleService(r).Delete(key, id, bizID); err != nil {
				apiError(w, http.StatusInternalServerError, err.Error())
				return
			}
			apiJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
		}
	}
}

func (a *App) APIStats(w http.ResponseWriter, r *http.Request) {
	bizID := a.bizID(r)
	counts, err := a.moduleService(r).Counts(bizID)
	if err != nil {
		apiError(w, http.StatusInternalServerError, err.Error())
		return
	}
	totals, err := a.moduleService(r).Totals(bizID)
	if err != nil {
		apiError(w, http.StatusInternalServerError, err.Error())
		return
	}
	productCount, _ := a.ProductService.Count(bizID)
	lowStock, _ := a.ProductService.LowStockCount(bizID)
	pending, _ := a.moduleService(r).PendingInvoicesTotal(bizID)

	apiJSON(w, http.StatusOK, map[string]any{
		"products":              productCount,
		"low_stock":             lowStock,
		"modules":               counts,
		"invoice_total":         totals["invoice_total"],
		"purchase_total":        totals["po_total"],
		"pending_invoice_total": pending,
	})
}

func (a *App) APISearch(w http.ResponseWriter, r *http.Request) {
	bizID := a.bizID(r)
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(q) < 2 {
		apiJSON(w, http.StatusOK, map[string]any{"results": []any{}, "query": q})
		return
	}
	type APIResult struct {
		Module string        `json:"module"`
		Path   string        `json:"path"`
		ID     string        `json:"id"`
		Title  string        `json:"title"`
		Record models.Record `json:"record"`
	}
	var results []APIResult
	for _, key := range []string{"customers", "invoices", "vendors", "categories", "payments"} {
		cfg, ok := a.moduleService(r).ConfigOnly(key)
		if !ok {
			continue
		}
		_, result, err := a.moduleService(r).ListPaged(key, 1, 10, q, "", "", bizID)
		if err != nil {
			continue
		}
		for _, rec := range result.Records {
			title := rec["name"]
			if title == "" {
				title = rec["number"]
			}
			results = append(results, APIResult{
				Module: cfg.Title, Path: cfg.Path,
				ID: rec["id"], Title: title, Record: rec,
			})
		}
	}
	apiJSON(w, http.StatusOK, map[string]any{"results": results, "query": q, "count": len(results)})
}

func parseAPIValues(r *http.Request) map[string]string {
	values := map[string]string{}
	if r.Header.Get("Content-Type") == "application/json" {
		json.NewDecoder(r.Body).Decode(&values)
		return values
	}
	_ = r.ParseForm()
	for k, v := range r.Form {
		if len(v) > 0 {
			values[k] = strings.TrimSpace(v[0])
		}
	}
	return values
}
