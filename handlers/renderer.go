package handlers

import (
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

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

// Auth renders a standalone auth page (no sidebar).
func (r *Renderer) Auth(w http.ResponseWriter, page string, data any) {
	r.execute(w, "auth", data,
		filepath.Join(r.root, "layouts", "auth.html"),
		filepath.Join(r.root, "pages", page),
	)
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
	}
}
