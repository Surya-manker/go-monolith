package handlers

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go-monolith/middleware"
)

type Renderer struct {
	root string
}

func NewRenderer(root string) *Renderer {
	return &Renderer{root: root}
}

// Page renders a full authenticated page (sidebar layout).
func (r *Renderer) Page(w http.ResponseWriter, page string, data any) {
	r.execute(w, "base", data,
		filepath.Join(r.root, "layouts", "base.html"),
		filepath.Join(r.root, "partials", "header.html"),
		filepath.Join(r.root, "partials", "footer.html"),
		filepath.Join(r.root, "pages", page),
	)
}

// PageWith renders a full authenticated page plus any extra named partials.
// Use this when a page template calls {{ template "X" }} where X is defined
// in a separate partial file (not inline in the page template itself).
func (r *Renderer) PageWith(w http.ResponseWriter, page string, data any, extraPartials ...string) {
	files := []string{
		filepath.Join(r.root, "layouts", "base.html"),
		filepath.Join(r.root, "partials", "header.html"),
		filepath.Join(r.root, "partials", "footer.html"),
		filepath.Join(r.root, "pages", page),
	}
	for _, p := range extraPartials {
		files = append(files, filepath.Join(r.root, "partials", p))
	}
	r.execute(w, "base", data, files...)
}

// Auth renders a standalone auth page (no sidebar).
func (r *Renderer) Auth(w http.ResponseWriter, page string, data any) {
	r.execute(w, "auth", data,
		filepath.Join(r.root, "layouts", "auth.html"),
		filepath.Join(r.root, "pages", page),
	)
}

// Landing renders the public landing page (no sidebar, no auth).
func (r *Renderer) Landing(w http.ResponseWriter, page string, data any) {
	r.execute(w, "landing", data,
		filepath.Join(r.root, "layouts", "landing.html"),
		filepath.Join(r.root, "pages", page),
	)
}

// Error renders a full-page error response using the base layout.
func (r *Renderer) Error(w http.ResponseWriter, code int, title, message string) {
	type errData struct {
		Code    int
		Title   string
		Message string
	}
	w.WriteHeader(code)
	r.Page(w, "error.html", errData{Code: code, Title: title, Message: message})
}

// Partial renders an HTML fragment for an HTMX swap.
func (r *Renderer) Partial(w http.ResponseWriter, partial string, data any) {
	content, err := os.ReadFile(filepath.Join(r.root, "partials", partial))
	if err != nil {
		log.Printf("[Partial] read %s: %v", partial, err)
		http.Error(w, "read: "+err.Error(), http.StatusInternalServerError)
		return
	}
	// Root named "" so associated templates created inside {{ define }} blocks
	// can inherit the FuncMap via nameSpace.set[""].text.
	tmpl, err := template.New("").Funcs(funcMap()).Parse(string(content))
	if err != nil {
		log.Printf("[Partial] parse %s: %v", partial, err)
		http.Error(w, "parse: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if err := tmpl.Execute(w, data); err != nil {
		log.Printf("[Partial] execute %s: %v", partial, err)
	}
}

// execute is the core renderer.
//
// Root template is named "" so all associated templates produced by Parse can
// resolve FuncMap via nameSpace.set[""].text (the pattern html/template uses
// internally in ParseFiles). Calling Parse once per file — instead of combining
// into one string — lets later {{ define "title" }} blocks silently replace the
// {{ block "title" }} default set by the base layout without raising "multiple
// definition of template" (which only fires within a single Parse call).
func (r *Renderer) execute(w http.ResponseWriter, entry string, data any, files ...string) {
	tmpl := template.New("").Funcs(funcMap())
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			log.Printf("[Page] read %s: %v", f, err)
			http.Error(w, "read: "+err.Error(), http.StatusInternalServerError)
			return
		}
		tmpl, err = tmpl.Parse(string(src))
		if err != nil {
			log.Printf("[Page] parse %s: %v", f, err)
			http.Error(w, "parse: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, entry, data); err != nil {
		log.Printf("[Page] execute %s: %v", entry, err)
		http.Error(w, "execute: "+err.Error(), http.StatusInternalServerError)
	}
}

func funcMap() template.FuncMap {
	return template.FuncMap{
		"initial": func(s string) string {
			s = strings.TrimSpace(s)
			if len(s) == 0 {
				return "?"
			}
			return strings.ToUpper(string([]rune(s)[0]))
		},
		"hasPermission": middleware.HasPermission,
		"orStr": func(a, b string) string {
			if a != "" {
				return a
			}
			return b
		},
		"upper": strings.ToUpper,
		"lower": strings.ToLower,
		"now":   time.Now,
		"jsonSafe": func(v any) template.JS {
			b, _ := json.Marshal(v)
			return template.JS(b)
		},
		"sub":  func(a, b float64) float64 { return a - b },
		"add":  func(a, b float64) float64 { return a + b },
		"absf": func(a float64) float64 { if a < 0 { return -a }; return a },
		// navActive returns "sidebar-link--active" when the link href matches
		// the current path. linkHref may include a query string (?k=v).
		// Rules (same as the JS approach, but authoritative):
		//   exact match          /reports == /reports          → active
		//   prefix+slash match   /finance  on /finance/expenses → active
		//   dashboard special    /dashboard on /              → active
		"navActive": func(currentPath, linkHref string) string {
			// Strip query string from the link href.
			hrefPath := linkHref
			if i := strings.Index(linkHref, "?"); i != -1 {
				hrefPath = linkHref[:i]
			}
			if hrefPath == "/dashboard" {
				if currentPath == "/" || currentPath == "/dashboard" {
					return "sidebar-link--active"
				}
				return ""
			}
			if hrefPath == currentPath {
				return "sidebar-link--active"
			}
			if len(hrefPath) > 1 && strings.HasPrefix(currentPath, hrefPath+"/") {
				return "sidebar-link--active"
			}
			return ""
		},
		"sub1":    func(n int) int { return n - 1 },
		"add1":    func(n int) int { return n + 1 },
		"float64": func(n int) float64 { return float64(n) },
		"int":     func(n float64) int { return int(n) },
		"pct": func(done, total int) int {
			if total == 0 {
				return 0
			}
			return done * 100 / total
		},
		"remaining": func(done, total int) int { return total - done },
		"mul": func(a, b float64) float64 { return a * b },
		"div": func(a, b float64) float64 {
			if b == 0 {
				return 0
			}
			return a / b
		},
		"today": func() string { return time.Now().Format("2006-01-02") },
		"seq": func(from, to int) []int {
			var out []int
			for i := from; i <= to; i++ {
				out = append(out, i)
			}
			return out
		},
		"monthName": func(m int) string {
			months := []string{"", "January", "February", "March", "April", "May", "June",
				"July", "August", "September", "October", "November", "December"}
			if m >= 1 && m <= 12 {
				return months[m]
			}
			return ""
		},
		// dateOnly accepts time.Time or string and returns "YYYY-MM-DD".
		"dateOnly": func(v any) string {
			switch t := v.(type) {
			case time.Time:
				return t.Format("2006-01-02")
			case string:
				if len(t) >= 10 {
					return t[:10]
				}
				return t
			default:
				return ""
			}
		},
	}
}
