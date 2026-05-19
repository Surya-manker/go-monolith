package handlers

import (
	"net/http"

	"go-monolith/middleware"
	"go-monolith/models"
)

// AppContext is embedded in every full-page data struct so templates can
// access .User.Name, .User.Role, and .Path without changing their other fields.
type AppContext struct {
	User *models.User
	Path string // current request path, used for sidebar active-link detection
}

func (a *App) ctx(r *http.Request) AppContext {
	return AppContext{
		User: middleware.UserFromContext(r.Context()),
		Path: r.URL.Path,
	}
}
