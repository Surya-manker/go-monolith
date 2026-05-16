package handlers

import "net/http"

func (a *App) About(w http.ResponseWriter, r *http.Request) {
	a.Renderer.Landing(w, "about.html", nil)
}
