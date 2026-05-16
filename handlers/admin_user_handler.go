package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"go-monolith/models"
)

type adminUsersData struct {
	AppContext
	Users []models.User
	Error string
}

type adminUserEditData struct {
	AppContext
	EditUser *models.User
	Roles    []string
	Error    string
	Success  string
}

var allRoles = []string{"admin", "manager", "accountant", "staff"}

func (a *App) AdminUsersPage(w http.ResponseWriter, r *http.Request) {
	users, err := a.AuthService.ListUsers()
	if err != nil {
		http.Error(w, "could not load users", http.StatusInternalServerError)
		return
	}
	a.Renderer.Page(w, "admin_users.html", adminUsersData{AppContext: a.ctx(r), Users: users})
}

func (a *App) AdminUserEditPage(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	user, err := a.AuthService.GetUser(id)
	if err != nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	a.Renderer.Page(w, "admin_user_edit.html", adminUserEditData{
		AppContext: a.ctx(r),
		EditUser:  user,
		Roles:     allRoles,
	})
}

func (a *App) AdminUserUpdate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	id, err := strconv.Atoi(r.FormValue("id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	role := r.FormValue("role")
	newPwd := r.FormValue("new_password")

	renderEdit := func(msg, ok string) {
		u, _ := a.AuthService.GetUser(id)
		a.Renderer.Page(w, "admin_user_edit.html", adminUserEditData{
			AppContext: a.ctx(r),
			EditUser:  u,
			Roles:     allRoles,
			Error:     msg,
			Success:   ok,
		})
	}

	if name == "" || email == "" {
		renderEdit("Name and email are required", "")
		return
	}
	if newPwd != "" && len(newPwd) < 8 {
		renderEdit("New password must be at least 8 characters", "")
		return
	}

	if err := a.AuthService.AdminUpdateUser(id, name, email, role, newPwd); err != nil {
		renderEdit("Could not update user: "+err.Error(), "")
		return
	}

	renderEdit("", "User updated successfully")
}
