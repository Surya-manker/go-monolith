package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"go-monolith/middleware"
	"go-monolith/models"
	"go-monolith/services"
)

type Pagination struct {
	Page     int
	PerPage  int
	Total    int
	LastPage int
	Search   string
	Sort     string
	Dir      string
}

func (p Pagination) HasPrev() bool { return p.Page > 1 }
func (p Pagination) HasNext() bool { return p.Page < p.LastPage }
func (p Pagination) Prev() int     { return p.Page - 1 }
func (p Pagination) Next() int     { return p.Page + 1 }

type CrudPageData struct {
	AppContext
	Config     services.ModuleConfig
	Records    []models.Record
	Edit       models.Record
	Error      string
	Pagination Pagination
}

func setToast(w http.ResponseWriter, message, typ string) {
	w.Header().Set("HX-Trigger", fmt.Sprintf(`{"showToast":{"message":%q,"type":%q}}`, message, typ))
}

func (a *App) moduleTitle(key string) string {
	if cfg, ok := a.ModuleService.ConfigOnly(key); ok {
		return cfg.Title
	}
	return "Record"
}

// ── Module CRUD handlers ──────────────────────────────────────────────────────

func (a *App) ModuleIndex(key string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		page, _ := strconv.Atoi(q.Get("page"))
		perPage, _ := strconv.Atoi(q.Get("per_page"))
		if perPage == 0 {
			perPage = 25
		}
		search := strings.TrimSpace(q.Get("search"))
		sort := q.Get("sort")
		dir := q.Get("dir")

		bizID := a.bizID(r)
		config, result, err := a.moduleService(r).ListPaged(key, page, perPage, search, sort, dir, bizID)
		if err != nil {
			http.Error(w, "could not load module", http.StatusInternalServerError)
			return
		}
		data := CrudPageData{
			AppContext: a.ctx(r),
			Config:     config,
			Records:    result.Records,
			Pagination: Pagination{
				Page: result.Page, PerPage: result.PerPage, Total: result.Total,
				LastPage: result.LastPage, Search: search, Sort: sort, Dir: dir,
			},
		}
		if r.Header.Get("HX-Request") == "true" {
			a.Renderer.Partial(w, "crud_table.html", data)
			return
		}
		a.Renderer.Page(w, "crud.html", data)
	}
}

func (a *App) ModuleCreate(key string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bizID := a.bizID(r)
		values := valuesFromRequest(r)
		err := a.moduleService(r).Create(key, values, bizID)
		if err == nil {
			a.auditLog(r, key, "create", "", values)
			setToast(w, a.moduleTitle(key)+" created successfully", "success")
		}
		a.renderModuleTable(w, r, key, err)
	}
}

func (a *App) ModuleEdit(key string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := idFromRequest(r)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		config, record, err := a.moduleService(r).Get(key, id, a.bizID(r))
		if err != nil {
			http.Error(w, "record not found", http.StatusNotFound)
			return
		}
		config.Fields = fieldsWithValues(config.Fields, record)
		a.Renderer.Partial(w, "crud_form.html", CrudPageData{Config: config, Edit: record})
	}
}

func (a *App) ModuleUpdate(key string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bizID := a.bizID(r)
		id, err := idFromRequest(r)
		values := valuesFromRequest(r)
		if err == nil {
			_, before, _ := a.moduleService(r).Get(key, id, bizID)
			err = a.moduleService(r).Update(key, id, values, bizID)
			if err == nil {
				a.auditLogChange(r, key, "update", strconv.Itoa(id), models.Record(before), models.Record(values))
				setToast(w, a.moduleTitle(key)+" updated successfully", "success")
			}
		}
		a.renderModuleTable(w, r, key, err)
	}
}

func (a *App) ModuleDelete(key string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bizID := a.bizID(r)
		id, err := idFromRequest(r)
		if err == nil {
			err = a.moduleService(r).Delete(key, id, bizID)
			if err == nil {
				a.auditLog(r, key, "delete", strconv.Itoa(id), nil)
				setToast(w, a.moduleTitle(key)+" moved to trash", "warning")
			}
		}
		a.renderModuleTable(w, r, key, err)
	}
}

// ── Trash / restore ───────────────────────────────────────────────────────────

func (a *App) ModuleTrash(key string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		config, records, err := a.moduleService(r).Trash(key, a.bizID(r))
		if err != nil {
			http.Error(w, "could not load trash", http.StatusInternalServerError)
			return
		}
		a.Renderer.Page(w, "trash.html", CrudPageData{AppContext: a.ctx(r), Config: config, Records: records})
	}
}

func (a *App) ModuleRestore(key string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bizID := a.bizID(r)
		id, err := idFromRequest(r)
		if err == nil {
			err = a.moduleService(r).Restore(key, id, bizID)
			if err == nil {
				a.auditLog(r, key, "restore", strconv.Itoa(id), nil)
				setToast(w, a.moduleTitle(key)+" restored", "success")
			}
		}
		config, records, _ := a.moduleService(r).Trash(key, bizID)
		a.Renderer.Partial(w, "trash_table.html", CrudPageData{Config: config, Records: records})
	}
}

func (a *App) ModulePurge(key string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bizID := a.bizID(r)
		id, err := idFromRequest(r)
		if err == nil {
			err = a.moduleService(r).HardDelete(key, id, bizID)
			if err == nil {
				a.auditLog(r, key, "purge", strconv.Itoa(id), nil)
				setToast(w, a.moduleTitle(key)+" permanently deleted", "error")
			}
		}
		config, records, _ := a.moduleService(r).Trash(key, bizID)
		a.Renderer.Partial(w, "trash_table.html", CrudPageData{Config: config, Records: records})
	}
}

// ── Readonly pages ────────────────────────────────────────────────────────────

func (a *App) StockLogs(w http.ResponseWriter, r *http.Request) {
	records, err := a.moduleService(r).StockLogs(a.bizID(r))
	if err != nil {
		http.Error(w, "could not load stock logs", http.StatusInternalServerError)
		return
	}
	config := services.ModuleConfig{
		Title: "Stock Logs", Path: "/stock-logs",
		Description: "Audit trail for purchases, returns, sales, and manual adjustments.",
		Columns: []services.Field{
			{Name: "product", Label: "Product"}, {Name: "change_type", Label: "Type"},
			{Name: "quantity_before", Label: "Before"}, {Name: "quantity_change", Label: "Change"},
			{Name: "quantity_after", Label: "After"}, {Name: "note", Label: "Note"},
			{Name: "created_at", Label: "Time"},
		},
	}
	a.Renderer.Page(w, "readonly.html", CrudPageData{AppContext: a.ctx(r), Config: config, Records: records})
}

func (a *App) AuditLogs(w http.ResponseWriter, r *http.Request) {
	records, err := a.AuditService.List(500)
	if err != nil {
		http.Error(w, "could not load audit logs", http.StatusInternalServerError)
		return
	}
	config := services.ModuleConfig{
		Title: "Audit Log", Path: "/audit-logs",
		Description: "Complete history of all data changes.",
		Columns: []services.Field{
			{Name: "user_name", Label: "User"}, {Name: "module", Label: "Module"},
			{Name: "action", Label: "Action"}, {Name: "record_id", Label: "Record"},
			{Name: "new_value", Label: "Changes"}, {Name: "ip_address", Label: "IP"},
			{Name: "created_at", Label: "Time"},
		},
	}
	a.Renderer.Page(w, "readonly.html", CrudPageData{AppContext: a.ctx(r), Config: config, Records: records})
}

func (a *App) Reports(w http.ResponseWriter, r *http.Request) {
	// Kept for backward compatibility — new hub is at /reports via ReportsHub.
	http.Redirect(w, r, "/reports", http.StatusFound)
}

// ── Internal helpers ──────────────────────────────────────────────────────────

func (a *App) renderModuleTable(w http.ResponseWriter, r *http.Request, key string, err error) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	search := strings.TrimSpace(q.Get("search"))
	sort := q.Get("sort")
	dir := q.Get("dir")

	config, result, listErr := a.moduleService(r).ListPaged(key, page, 25, search, sort, dir, a.bizID(r))
	if listErr != nil {
		http.Error(w, "could not load module", http.StatusInternalServerError)
		return
	}
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	a.Renderer.Partial(w, "crud_table.html", CrudPageData{
		Config:  config,
		Records: result.Records,
		Error:   msg,
		Pagination: Pagination{
			Page: result.Page, PerPage: result.PerPage, Total: result.Total,
			LastPage: result.LastPage, Search: search, Sort: sort, Dir: dir,
		},
	})
}

func (a *App) auditLog(r *http.Request, module, action, recID string, vals map[string]string) {
	if a.AuditService == nil {
		return
	}
	user := middleware.UserFromContext(r.Context())
	var uid *int
	name := "anonymous"
	if user != nil {
		uid = &user.ID
		name = user.Name
	}
	a.AuditService.Log(uid, name, module, action, recID, vals, clientIP(r))
}

func (a *App) auditLogChange(r *http.Request, module, action, recID string, before, after models.Record) {
	if a.AuditService == nil {
		return
	}
	user := middleware.UserFromContext(r.Context())
	var uid *int
	name := "anonymous"
	if user != nil {
		uid = &user.ID
		name = user.Name
	}
	a.AuditService.LogChange(uid, name, module, action, recID, map[string]string(before), map[string]string(after), clientIP(r))
}

func valuesFromRequest(r *http.Request) map[string]string {
	_ = r.ParseForm()
	values := map[string]string{}
	for key, value := range r.Form {
		if len(value) > 0 {
			values[key] = strings.TrimSpace(value[0])
		}
	}
	return values
}

func idFromRequest(r *http.Request) (int, error) {
	return strconv.Atoi(r.URL.Query().Get("id"))
}

func fieldsWithValues(fields []services.Field, record models.Record) []services.Field {
	out := make([]services.Field, len(fields))
	for i, field := range fields {
		field.Value = record[field.Name]
		out[i] = field
	}
	return out
}

