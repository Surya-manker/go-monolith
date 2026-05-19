package main

import (
	"database/sql"
	"go-monolith/handlers"
	"go-monolith/models"
	"go-monolith/routes"
	"go-monolith/services"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load("app.env"); err == nil {
		log.Println("loaded config from app.env")
	}
	dsn := os.Getenv("DATABASE_DSN")
	if dsn == "" {
		dsn = "root:@tcp(127.0.0.1:3306)/invobill?parseTime=true&charset=utf8mb4"
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	if err := db.Ping(); err != nil {
		log.Fatalf("cannot connect to MySQL: %v\nSet DATABASE_DSN env var", err)
	}

	// Migrations — in dependency order.
	businessStore := models.NewBusinessStore(db)
	if err := businessStore.Migrate(); err != nil {
		log.Fatal("business migrate:", err)
	}
	productStore := models.NewProductStore(db)
	if err := productStore.Migrate(); err != nil {
		log.Fatal("product migrate:", err)
	}
	moduleStore := models.NewModuleStore(db)
	if err := moduleStore.Migrate(); err != nil {
		log.Fatal("module migrate:", err)
	}
	authStore := models.NewAuthStore(db)
	if err := authStore.Migrate(); err != nil {
		log.Fatal("auth migrate:", err)
	}
	auditStore := models.NewAuditStore(db)
	if err := auditStore.Migrate(); err != nil {
		log.Fatal("audit migrate:", err)
	}
	notifStore := models.NewNotificationStore(db)
	if err := notifStore.Migrate(); err != nil {
		log.Fatal("notif migrate:", err)
	}
	warehouseStore := models.NewWarehouseStore(db)
	if err := warehouseStore.Migrate(); err != nil {
		log.Fatal("warehouse migrate:", err)
	}
	posStore := models.NewPOSStore(db)
	if err := posStore.Migrate(); err != nil {
		log.Fatal("pos migrate:", err)
	}
	batchStore := models.NewBatchStore(db)
	if err := batchStore.Migrate(); err != nil {
		log.Fatal("batch migrate:", err)
	}

	// Seller / GST config.
	sellerGSTIN := os.Getenv("GST_SELLER_GSTIN")
	stateCode := os.Getenv("GST_STATE_CODE")
	if stateCode == "" && len(sellerGSTIN) >= 2 {
		stateCode = sellerGSTIN[:2]
	}
	sellerName := os.Getenv("GST_SELLER_NAME")
	if sellerName == "" {
		sellerName = "InvoBill Company"
	}

	services.SetBusinessStore(businessStore)

	procurementStore := models.NewProcurementStore(db)
	if err := procurementStore.Migrate(); err != nil {
		log.Fatal("procurement migrate:", err)
	}
	crmStore := models.NewCRMStore(db)
	if err := crmStore.Migrate(); err != nil {
		log.Fatal("crm migrate:", err)
	}
	financeStore := models.NewFinanceStore(db)
	if err := financeStore.Migrate(); err != nil {
		log.Fatal("finance migrate:", err)
	}

	productSvc := services.NewProductService(productStore)
	warehouseSvc := services.NewWarehouseService(warehouseStore)
	reportSvc := services.NewReportService(db)

	renderer := handlers.NewRenderer("templates")
	app := &handlers.App{
		Renderer:           renderer,
		BusinessStore:      businessStore,
		ProductService:     productSvc,
		ModuleService:      services.NewModuleService(moduleStore),
		AuthService:        services.NewAuthService(authStore),
		AuditService:       services.NewAuditService(auditStore),
		NotifService:       services.NewNotificationService(notifStore),
		WarehouseService:   warehouseSvc,
		ReportService:      reportSvc,
		ProcurementService: services.NewProcurementService(db, procurementStore, warehouseStore, batchStore, productStore),
		CRMService:         services.NewCRMService(db, crmStore, warehouseStore, batchStore, productStore),
		FinanceService:     services.NewFinanceService(financeStore),
		BarcodeService:     services.NewBarcodeService(),
		BatchService:       services.NewBatchService(db, batchStore, warehouseStore, productStore),
		ReturnsService:     services.NewReturnsService(db, batchStore, warehouseStore, productStore),
		POSService:         services.NewPOSService(db, posStore, warehouseStore, productStore, batchStore),
		POSCarts:           services.NewPOSCartManager(),
		DemoSessions:       services.NewDemoSessionManager(),
		Mailer:             services.NewMailer(),
		SellerName:         sellerName,
		SellerGSTIN:        sellerGSTIN,
		SellerAddress:      os.Getenv("GST_SELLER_ADDRESS"),
		StateCode:          stateCode,
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("server running at http://localhost:%s", port)
	if err := http.ListenAndServe(":"+port, routes.New(app)); err != nil {
		log.Fatal(err)
	}
}
