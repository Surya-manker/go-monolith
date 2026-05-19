package handlers

import (
	"net/http"

	"go-monolith/middleware"
	"go-monolith/models"
	"go-monolith/services"
)

type App struct {
	Renderer             *Renderer
	BusinessStore        *models.BusinessStore
	ProductService       *services.ProductService
	ModuleService        *services.ModuleService
	AuthService          *services.AuthService
	AuditService         *services.AuditService
	NotifService         *services.NotificationService
	WarehouseService     *services.WarehouseService
	BarcodeService       *services.BarcodeService
	BatchService         *services.BatchService
	ReturnsService       *services.ReturnsService
	POSService           *services.POSService
	ReportService        *services.ReportService
	ProcurementService   *services.ProcurementService
	CRMService           *services.CRMService
	FinanceService       *services.FinanceService
	POSCarts             *services.POSCartManager
	DemoSessions         *services.DemoSessionManager
	Mailer               services.Mailer
	// GST / seller config
	SellerName    string
	SellerGSTIN   string
	SellerAddress string
	StateCode     string
}

func (a *App) bizID(r *http.Request) int {
	if u := middleware.UserFromContext(r.Context()); u != nil {
		return u.BusinessID
	}
	return 0
}

func (a *App) moduleService(r *http.Request) *services.ModuleService {
	if cookie, err := r.Cookie("session"); err == nil {
		if store := a.DemoSessions.Get(cookie.Value); store != nil {
			return a.ModuleService.WithStore(store)
		}
	}
	return a.ModuleService
}
