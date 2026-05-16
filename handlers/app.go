package handlers

import "go-monolith/services"

type App struct {
	Renderer       *Renderer
	ProductService *services.ProductService
	ModuleService  *services.ModuleService
	AuthService    *services.AuthService
	AuditService   *services.AuditService
	NotifService   *services.NotificationService
	// GST / seller config (read from env at startup)
	SellerName    string
	SellerGSTIN   string
	SellerAddress string
	StateCode     string // first 2 digits of seller GSTIN, e.g. "27"
}
