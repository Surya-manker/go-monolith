package handlers

import (
	"net/http"
	"strconv"

	"go-monolith/middleware"
)

func (a *App) NotificationsPanel(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	notifs, err := a.NotifService.List(user.ID)
	if err != nil {
		http.Error(w, "could not load notifications", http.StatusInternalServerError)
		return
	}
	_ = a.NotifService.MarkAllRead(user.ID) // mark all as read when panel is opened
	a.Renderer.Partial(w, "notifications.html", map[string]any{
		"Notifications": notifs,
	})
}

func (a *App) NotificationsBadge(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		return
	}
	count, _ := a.NotifService.UnreadCount(user.ID)
	a.Renderer.Partial(w, "notif_badge.html", map[string]any{"Count": count})
}

func (a *App) NotificationMarkRead(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		return
	}
	id, _ := strconv.Atoi(r.URL.Query().Get("id"))
	_ = a.NotifService.MarkRead(id, user.ID)
	w.WriteHeader(http.StatusNoContent)
}
