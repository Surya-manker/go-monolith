package middleware

import (
	"net/http"
)

// Permission represents a module+action pair. "*" is a wildcard.
type Permission struct{ Module, Action string }

// rolePerms maps each role to its allowed permissions.
var rolePerms = map[string][]Permission{
	"super_admin": {{"*", "*"}},
	"admin":       {{"*", "*"}},
	"manager": {
		{"products", "*"}, {"customers", "*"}, {"categories", "*"},
		{"vendors", "*"}, {"invoices", "*"}, {"purchase-orders", "*"},
		{"payments", "view"}, {"payments", "create"},
		{"stock-logs", "view"}, {"reports", "view"},
		{"jobs", "view"}, {"credit-notes", "view"}, {"users", "view"},
		{"warehouses", "*"}, {"transfers", "*"},
		{"pos", "*"}, {"barcodes", "*"},
		{"batches", "*"}, {"returns", "*"},
		{"suppliers", "*"}, {"procurement", "*"},
		{"crm", "*"},
		{"finance", "*"},
	},
	"warehouse_manager": {
		{"products", "view"}, {"categories", "view"},
		{"warehouses", "*"}, {"transfers", "*"},
		{"stock-logs", "view"}, {"reports", "view"},
		{"purchase-orders", "view"},
		{"pos", "*"}, {"barcodes", "view"},
		{"batches", "*"}, {"returns", "*"},
	},
	"staff": {
		{"products", "view"}, {"products", "create"},
		{"customers", "view"}, {"customers", "create"},
		{"invoices", "view"}, {"invoices", "create"},
		{"categories", "view"}, {"vendors", "view"},
		{"stock-logs", "view"}, {"purchase-orders", "view"},
		{"warehouses", "view"}, {"transfers", "view"},
		{"pos", "create"}, {"pos", "view"},
		{"barcodes", "view"},
		{"batches", "view"}, {"batches", "create"},
		{"returns", "view"}, {"returns", "create"},
		{"suppliers", "view"}, {"procurement", "view"},
		{"crm", "view"}, {"crm", "create"},
		{"finance", "view"},
	},
	"accountant": {
		{"invoices", "*"}, {"payments", "*"},
		{"credit-notes", "*"}, {"reports", "*"},
		{"customers", "view"}, {"products", "view"},
		{"categories", "view"}, {"vendors", "view"},
		{"warehouses", "view"}, {"stock-logs", "view"},
	},
}

// HasPermission returns true if the role is allowed module:action.
func HasPermission(role, module, action string) bool {
	for _, p := range rolePerms[role] {
		mOK := p.Module == "*" || p.Module == module
		aOK := p.Action == "*" || p.Action == action
		if mOK && aOK {
			return true
		}
	}
	return false
}

// RequireRole allows only the listed roles; returns 403 for all others.
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	set := make(map[string]bool, len(roles))
	for _, r := range roles {
		set[r] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := UserFromContext(r.Context())
			if user == nil || !set[user.Role] {
				http.Error(w, "403 Forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequirePermission returns a middleware that enforces module:action permission.
func RequirePermission(module, action string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := UserFromContext(r.Context())
			if user == nil || !HasPermission(user.Role, module, action) {
				http.Error(w, "403 forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
