package middleware

import (
	"context"
	"net/http"

	"go-monolith/models"
	"go-monolith/services"
)

type contextKey string

const userKey contextKey = "user"

// Auth redirects unauthenticated requests to /login.
func Auth(svc *services.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, err := svc.GetUserFromRequest(r)
			if err != nil {
				http.Redirect(w, r, "/login", http.StatusFound)
				return
			}
			ctx := context.WithValue(r.Context(), userKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserFromContext returns the authenticated user or nil.
func UserFromContext(ctx context.Context) *models.User {
	u, _ := ctx.Value(userKey).(*models.User)
	return u
}
