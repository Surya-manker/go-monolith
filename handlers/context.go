package handlers

import (
	"net/http"

	"go-monolith/middleware"
	"go-monolith/models"
)

// AppContext is embedded in every full-page data struct so templates can
// access .User.Name, .User.Role, etc. without changing their other fields.
type AppContext struct {
	User *models.User
}

func (a *App) ctx(r *http.Request) AppContext {
	return AppContext{User: middleware.UserFromContext(r.Context())}
}
