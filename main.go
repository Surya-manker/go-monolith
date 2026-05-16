package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"

	"go-monolith/handlers"
	"go-monolith/models"
	"go-monolith/routes"
	"go-monolith/services"
)

func main() {
	// Load app.env if it exists — silently ignored if missing.
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
		log.Fatalf("cannot connect to MySQL: %v\nSet DATABASE_DSN env var, e.g.:\n  DATABASE_DSN=root:password@tcp(127.0.0.1:3306)/invobill?parseTime=true", err)
	}

	// Run migrations in dependency order.
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

	// Seller / GST config from environment.
	sellerGSTIN := os.Getenv("GST_SELLER_GSTIN")
	stateCode := os.Getenv("GST_STATE_CODE")
	if stateCode == "" && len(sellerGSTIN) >= 2 {
		stateCode = sellerGSTIN[:2]
	}
	sellerName := os.Getenv("GST_SELLER_NAME")
	if sellerName == "" {
		sellerName = "InvoBill Company"
	}

	renderer := handlers.NewRenderer("templates")
	app := &handlers.App{
		Renderer:       renderer,
		ProductService: services.NewProductService(productStore),
		ModuleService:  services.NewModuleService(moduleStore),
		AuthService:    services.NewAuthService(authStore),
		AuditService:   services.NewAuditService(auditStore),
		NotifService:   services.NewNotificationService(notifStore),
		Mailer:         services.NewMailer(),
		SellerName:     sellerName,
		SellerGSTIN:    sellerGSTIN,
		SellerAddress:  os.Getenv("GST_SELLER_ADDRESS"),
		StateCode:      stateCode,
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
