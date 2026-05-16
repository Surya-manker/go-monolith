package handlers

import "net/http"

type ModulePageData struct {
	AppContext
	Title       string
	Description string
}

func (a *App) ModulePage(title, description string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a.Renderer.Page(w, "module.html", ModulePageData{
			AppContext:   a.ctx(r),
			Title:       title,
			Description: description,
		})
	}
}
