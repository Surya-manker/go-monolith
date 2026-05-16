package handlers

import "net/http"

func (a *App) Generator(w http.ResponseWriter, r *http.Request) {
	a.Renderer.Landing(w, "generator.html", nil)
}
