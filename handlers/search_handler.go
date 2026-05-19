package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"go-monolith/models"
	"go-monolith/services"
)

type SearchResult struct {
	Module string
	Path   string
	ID     string
	Title  string
	Sub    string
}

// Search performs a cross-module keyword search and returns an HTML partial.
func (a *App) Search(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(q) < 2 {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<div class="search-empty">Type at least 2 characters…</div>`)
		return
	}

	var results []SearchResult

	modules := []struct {
		key    string
		title  string
		path   string
		config services.ModuleConfig
	}{}

	bizID := a.bizID(r)
	for _, key := range []string{"customers", "invoices", "products_search", "vendors", "categories", "payments"} {
		if key == "products_search" {
			products, _ := a.ProductService.List(q, bizID)
			for _, p := range products {
				results = append(results, SearchResult{
					Module: "Products",
					Path:   "/products",
					ID:     fmt.Sprintf("%d", p.ID),
					Title:  p.Name,
					Sub:    fmt.Sprintf("SKU: %s · Stock: %d · %s", p.SKU, p.Stock, money(p.Price)),
				})
			}
			continue
		}
		cfg, ok := a.moduleService(r).ConfigOnly(key)
		if !ok {
			continue
		}
		modules = append(modules, struct {
			key    string
			title  string
			path   string
			config services.ModuleConfig
		}{key, cfg.Title, cfg.Path, cfg})
	}

	for _, mod := range modules {
		_, result, err := a.moduleService(r).ListPaged(mod.key, 1, 5, q, "", "", bizID)
		if err != nil {
			continue
		}
		for _, rec := range result.Records {
			r := buildSearchResult(mod.config, rec)
			results = append(results, r)
		}
	}

	a.Renderer.Partial(w, "search_results.html", map[string]any{
		"Query":   q,
		"Results": results,
	})
}

func buildSearchResult(cfg services.ModuleConfig, rec models.Record) SearchResult {
	title := rec["name"]
	if title == "" {
		title = rec["number"]
	}
	if title == "" {
		title = "#" + rec["id"]
	}

	// Build a subtitle from the first 2 non-name fields.
	var parts []string
	for _, f := range cfg.Fields {
		if f.Name == "name" || f.Name == "number" || rec[f.Name] == "" {
			continue
		}
		parts = append(parts, f.Label+": "+rec[f.Name])
		if len(parts) == 2 {
			break
		}
	}

	return SearchResult{
		Module: cfg.Title,
		Path:   cfg.Path,
		ID:     rec["id"],
		Title:  title,
		Sub:    strings.Join(parts, " · "),
	}
}
