// Package main seeds InvoBill with a realistic demo dataset covering 90 days
// of business activity. It is idempotent — safe to run multiple times.
// Usage: go run ./seed/
package main

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
)

// ── entry point ──────────────────────────────────────────────────────────────

func main() {
	_ = godotenv.Load("app.env")

	dsn := os.Getenv("DATABASE_DSN")
	if dsn == "" {
		dsn = "root:@tcp(127.0.0.1:3306)/invobill?parseTime=true&charset=utf8mb4"
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		log.Fatalf("cannot connect to MySQL: %v\nSet DATABASE_DSN env var.", err)
	}
	fmt.Println("Connected — seeding InvoBill demo dataset...")

	bizID := seedBusiness(db)
	userIDs := seedUsers(db, bizID)
	_ = userIDs

	// Generic module tables (business_id scoped).
	seedCategories(db, bizID)
	seedVendors(db, bizID)

	// Products must come before everything that references product IDs.
	productIDs := seedProducts(db, bizID)

	// Warehouses, then stock distribution.
	whIDs := seedWarehouses(db, bizID)
	seedWarehouseStock(db, bizID, whIDs, productIDs)
	seedStockTransfers(db, bizID, whIDs, productIDs)

	// Batches (FEFO-tracked inventory: food + pharma).
	seedBatches(db, bizID, whIDs, productIDs)

	// Suppliers (procurement module).
	supplierIDs := seedSuppliers(db, bizID)
	poIDs := seedProcurementOrders(db, bizID, supplierIDs, whIDs, productIDs)
	seedSupplierPayments(db, bizID, supplierIDs, poIDs)

	// CRM customers + orders.
	crmCustomerIDs := seedCRMCustomers(db, bizID)
	soIDs := seedSalesOrders(db, bizID, crmCustomerIDs, whIDs, productIDs)
	seedDeliveries(db, bizID, soIDs, crmCustomerIDs, whIDs)
	seedCRMPayments(db, bizID, soIDs, crmCustomerIDs)

	// Generic invoice module.
	seedInvoices(db, bizID)
	seedPayments(db, bizID)

	// POS sales — 45 days of daily transactions.
	seedPOSSales(db, bizID, whIDs, productIDs)

	// Finance module.
	catIDs := seedExpenseCategories(db, bizID)
	bankIDs := seedBankAccounts(db, bizID)
	seedExpenses(db, bizID, catIDs, bankIDs)
	seedBankTransactions(db, bizID, bankIDs)

	// Notifications for admin.
	seedNotifications(db, userIDs)

	// Audit log.
	seedAuditLog(db, bizID, userIDs)

	fmt.Println("\n✅ Seed complete!")
	fmt.Println("  Admin    → admin@invobill.com  / admin123456")
	fmt.Println("  Manager  → manager@invobill.com  / manager123456")
	fmt.Println("  Staff    → staff@invobill.com  / staff123456")
	fmt.Println("  Accounts → accounts@invobill.com / accounts123456")
	fmt.Println("\nOpen http://localhost:8080 and log in.")
}

// ── helpers ───────────────────────────────────────────────────────────────────

func insert(db *sql.DB, q string, args ...any) int64 {
	res, err := db.Exec(q, args...)
	if err != nil {
		log.Printf("[seed] insert error (%s): %v", q[:min(50, len(q))], err)
		return 0
	}
	id, _ := res.LastInsertId()
	return id
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func hashpw(pw string) string {
	h, _ := bcrypt.GenerateFromPassword([]byte(pw), 10)
	return string(h)
}

// ago returns a time N days before today at a random hour.
func ago(days int) time.Time {
	return time.Now().AddDate(0, 0, -days).
		Truncate(24 * time.Hour).
		Add(time.Duration(8+rand.Intn(10)) * time.Hour)
}

func dateFmt(t time.Time) string { return t.Format("2006-01-02") }
func datetimeFmt(t time.Time) string { return t.Format("2006-01-02 15:04:05") }

// rowExists checks if a unique text value already exists in a table.
func rowExists(db *sql.DB, table, col, val string) bool {
	var id int
	return db.QueryRow(
		fmt.Sprintf("SELECT id FROM `%s` WHERE `%s` = ? LIMIT 1", table, col),
		val,
	).Scan(&id) == nil
}

// ── business ─────────────────────────────────────────────────────────────────

func seedBusiness(db *sql.DB) int {
	var id int
	err := db.QueryRow(`SELECT id FROM businesses WHERE name = ? LIMIT 1`,
		"Mehta Electronics & General Store").Scan(&id)
	if err == nil {
		fmt.Printf("  skip business (id=%d)\n", id)
		return id
	}
	res, err := db.Exec(`INSERT INTO businesses
		(name, email, phone, gstin, address, state_code, status)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"Mehta Electronics & General Store",
		"admin@mehtaelectronics.in",
		"9900012345",
		"27AABCM1234N1Z5",
		"Shop 12, Gandhi Nagar Market, Mumbai, MH 400001",
		"27",
		"active",
	)
	if err != nil {
		log.Fatal("seed business:", err)
	}
	id64, _ := res.LastInsertId()
	fmt.Printf("  ✓ business: Mehta Electronics & General Store (id=%d)\n", int(id64))
	return int(id64)
}

// ── users ─────────────────────────────────────────────────────────────────────

func seedUsers(db *sql.DB, bizID int) map[string]int {
	users := []struct {
		name, email, pw, role string
	}{
		{"Anand Mehta", "admin@invobill.com", "admin123456", "admin"},
		{"Ramesh Kumar", "manager@invobill.com", "manager123456", "manager"},
		{"Sunita Yadav", "staff@invobill.com", "staff123456", "staff"},
		{"Kavita Sharma", "accounts@invobill.com", "accounts123456", "accountant"},
	}
	ids := map[string]int{}
	for _, u := range users {
		var id int
		err := db.QueryRow("SELECT id FROM users WHERE email = ? LIMIT 1", u.email).Scan(&id)
		if err == nil {
			fmt.Printf("  skip user: %s\n", u.email)
			ids[u.role] = id
			// Ensure business_id is set.
			db.Exec(`UPDATE users SET business_id=? WHERE id=?`, bizID, id)
			continue
		}
		res, _ := db.Exec(
			`INSERT INTO users (business_id, name, email, password_hash, role) VALUES (?,?,?,?,?)`,
			bizID, u.name, u.email, hashpw(u.pw), u.role,
		)
		id64, _ := res.LastInsertId()
		ids[u.role] = int(id64)
		fmt.Printf("  ✓ user: %s [%s]\n", u.email, u.role)
	}
	return ids
}

// ── categories ────────────────────────────────────────────────────────────────

func seedCategories(db *sql.DB, bizID int) {
	cats := [][2]string{
		{"Electronics", "Laptops, phones, accessories, cables"},
		{"Clothing & Apparel", "Garments, fashion, accessories"},
		{"Food & Beverages", "Grocery, packaged foods, drinks"},
		{"Office Supplies", "Stationery, printing, office essentials"},
		{"Tools & Hardware", "Workshop tools, fasteners, maintenance"},
		{"Furniture & Fixtures", "Office and home furniture"},
		{"Pharmaceuticals", "OTC medicines, health supplements"},
		{"Cosmetics & Personal Care", "Skincare, haircare, grooming"},
	}
	for _, c := range cats {
		var id int
		if db.QueryRow(`SELECT id FROM categories WHERE name=? AND business_id=?`, c[0], bizID).Scan(&id) == nil {
			continue
		}
		insert(db, `INSERT INTO categories (business_id, name, description) VALUES (?,?,?)`, bizID, c[0], c[1])
		fmt.Printf("  ✓ category: %s\n", c[0])
	}
}

// ── vendors (generic module) ──────────────────────────────────────────────────

func seedVendors(db *sql.DB, bizID int) {
	vendors := []struct{ name, email, phone, status string }{
		{"TechCorp India Pvt Ltd", "supply@techcorp.in", "9800012345", "active"},
		{"FashionHub Wholesalers", "orders@fashionhub.in", "9900098765", "active"},
		{"FoodMart Distributors", "bulk@foodmart.in", "9700056789", "active"},
		{"OfficeEssentials Co.", "sales@officeessentials.in", "9600034567", "active"},
		{"HardwarePro Suppliers", "info@hardwarepro.in", "9500078901", "active"},
		{"FurniWorld Pvt Ltd", "contact@furniworld.in", "9400011223", "active"},
		{"MediSupply India", "orders@medisupply.in", "9300099887", "active"},
		{"BeautyBazaar Wholesale", "bulk@beautybazaar.in", "9200088776", "active"},
	}
	for _, v := range vendors {
		var id int
		if db.QueryRow(`SELECT id FROM vendors WHERE name=? AND business_id=?`, v.name, bizID).Scan(&id) == nil {
			continue
		}
		insert(db, `INSERT INTO vendors (business_id, name, email, phone, status) VALUES (?,?,?,?,?)`,
			bizID, v.name, v.email, v.phone, v.status)
		fmt.Printf("  ✓ vendor: %s\n", v.name)
	}
}

// ── products ─────────────────────────────────────────────────────────────────

func seedProducts(db *sql.DB, bizID int) []int {
	type prod struct {
		sku, name, desc, unit string
		price, cost           float64
		stock, threshold      int
		tax                   float64
		barcode               string
		hsn                   string
	}
	products := []prod{
		// Electronics
		{"ELEC-001", "Laptop 15\" Core i5", "Core i5 12th Gen, 8GB RAM, 512GB SSD, Full HD", "pcs", 52999, 42000, 12, 2, 18, "8901234560001", "8471"},
		{"ELEC-002", "Wireless Mouse Ergonomic", "2.4GHz USB receiver, 1600 DPI, silent click", "pcs", 799, 420, 85, 10, 18, "8901234560002", "8471"},
		{"ELEC-003", "USB-C Hub 7-in-1", "HDMI 4K, USB 3.0 x3, SD/TF card, PD 100W", "pcs", 1499, 880, 40, 5, 18, "8901234560003", "8473"},
		{"ELEC-004", "Bluetooth Headphones ANC", "Active noise cancelling, 30hr battery, foldable", "pcs", 3499, 2100, 25, 4, 18, "8901234560004", "8518"},
		{"ELEC-005", "Mechanical Keyboard TKL", "87-key, Blue switches, RGB backlight, USB-C", "pcs", 4299, 2700, 18, 3, 18, "8901234560005", "8471"},
		{"ELEC-006", "Power Bank 20000mAh", "65W PD fast charge, dual USB-A + USB-C", "pcs", 1999, 1200, 35, 5, 18, "8901234560006", "8507"},
		{"ELEC-007", "Smart LED Bulb 9W", "WiFi controlled, 16M colors, voice assistant", "pcs", 599, 320, 120, 20, 18, "8901234560007", "8539"},
		// Food & Beverages
		{"FOOD-001", "Basmati Rice Premium 5kg", "Extra long grain, aged, aromatic", "bag", 425, 310, 200, 40, 0, "8901234561001", "1006"},
		{"FOOD-002", "Sunflower Oil 1L", "Refined, fortified with Vitamins A, D, E", "bottle", 165, 125, 180, 30, 5, "8901234561002", "1512"},
		{"FOOD-003", "Whole Wheat Atta 10kg", "Stone ground, high fibre, 100% whole wheat", "bag", 490, 370, 250, 50, 0, "8901234561003", "1101"},
		{"FOOD-004", "Tata Salt 1kg", "Vacuum evaporated iodized salt", "pack", 28, 20, 500, 100, 0, "8901234561004", "2501"},
		{"FOOD-005", "Maggi Noodles 70g (Pack of 12)", "Masala flavour, quick cook", "pack", 145, 108, 300, 60, 5, "8901234561005", "1902"},
		{"FOOD-006", "Amul Butter 500g", "Pasteurised salted butter", "pack", 285, 228, 80, 15, 12, "8901234561006", "0405"},
		// Office Supplies
		{"OFFC-001", "A4 Paper 500 Sheets 80GSM", "Bright white, multi-purpose, laser & inkjet", "ream", 380, 265, 300, 30, 12, "8901234562001", "4802"},
		{"OFFC-002", "Ball Pen Blue Box/10", "Smooth medium point, 1mm tip", "box", 85, 48, 600, 60, 12, "8901234562002", "9608"},
		{"OFFC-003", "Stapler 30-Sheet", "Full strip, includes 1000 staples", "pcs", 320, 195, 55, 8, 18, "8901234562003", "8305"},
		{"OFFC-004", "Sticky Notes 3x3 (6 pads)", "Pastel colours, repositionable", "set", 199, 120, 120, 20, 18, "8901234562004", "4817"},
		// Pharmaceuticals
		{"PHRM-001", "Paracetamol 500mg Strip/10", "Fever & pain relief, IP grade", "strip", 12, 7, 400, 80, 0, "8901234563001", "3004"},
		{"PHRM-002", "Vitamin C 500mg x30", "Ascorbic acid, immune support", "bottle", 85, 55, 150, 25, 5, "8901234563002", "2936"},
		{"PHRM-003", "Antacid Syrup 200ml", "Mint flavour, fast relief", "bottle", 68, 42, 120, 20, 5, "8901234563003", "3004"},
		// Clothing
		{"CLTH-001", "Cotton T-Shirt Round Neck M", "180 GSM combed cotton, preshrunk", "pcs", 349, 175, 180, 20, 5, "8901234564001", "6109"},
		{"CLTH-002", "Slim Fit Denim Jeans 32", "98% cotton 2% elastane, mid-rise", "pcs", 999, 570, 90, 10, 5, "8901234564002", "6203"},
		{"CLTH-003", "Formal Shirt White L", "55% cotton 45% polyester, wrinkle resistant", "pcs", 749, 420, 130, 15, 5, "8901234564003", "6205"},
		// Hardware
		{"TOOL-001", "Hammer 500g Steel", "Drop forged, rubber grip handle", "pcs", 320, 185, 38, 5, 18, "8901234565001", "8205"},
		{"TOOL-002", "Screwdriver Set 6pc", "Phillips PH1/PH2, Flat 4/6/8mm, magnetised", "set", 495, 295, 32, 5, 18, "8901234565002", "8205"},
	}

	var ids []int
	for _, p := range products {
		var id int
		if db.QueryRow(`SELECT id FROM products WHERE sku=? AND business_id=?`, p.sku, bizID).Scan(&id) == nil {
			ids = append(ids, id)
			continue
		}
		res, err := db.Exec(`
			INSERT INTO products
			(business_id, name, description, sku, barcode, hsn_code, unit, status,
			 price, cost_price, stock, tax_rate, low_stock_threshold)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			bizID, p.name, p.desc, p.sku, p.barcode, p.hsn, p.unit, "active",
			p.price, p.cost, p.stock, p.tax, p.threshold,
		)
		if err != nil {
			log.Printf("  product %s: %v", p.sku, err)
			continue
		}
		id64, _ := res.LastInsertId()
		ids = append(ids, int(id64))

		// Seed a few stock log entries per product.
		for _, d := range []int{85, 60, 45, 30, 15} {
			db.Exec(`INSERT INTO stock_logs
				(product_id, warehouse_id, change_type, quantity_before, quantity_change, quantity_after, note, created_at)
				VALUES (?,?,?,?,?,?,?,?)`,
				int(id64), 0, "purchase", p.stock-rand.Intn(20), rand.Intn(20)+5, p.stock,
				"initial stock / purchase receipt",
				datetimeFmt(ago(d)),
			)
		}
		fmt.Printf("  ✓ product: %s\n", p.sku)
	}
	return ids
}

// ── warehouses ────────────────────────────────────────────────────────────────

func seedWarehouses(db *sql.DB, bizID int) []int {
	whs := []struct {
		name, address, manager string
		isDefault              int
	}{
		{"Main Warehouse — Mumbai", "Plot 15, MIDC Industrial Area, Andheri East, Mumbai 400093", "Ramesh Kumar", 1},
		{"Retail Store — Dadar", "Shop 12, Gandhi Nagar Market, Dadar West, Mumbai 400028", "Sunita Yadav", 0},
		{"Cold Storage — Thane", "Unit 4, Freeze Zone Industrial Park, Thane 400601", "Ajay Singh", 0},
	}
	var ids []int
	for _, w := range whs {
		var id int
		if db.QueryRow(`SELECT id FROM warehouses WHERE name=? AND business_id=?`, w.name, bizID).Scan(&id) == nil {
			ids = append(ids, id)
			continue
		}
		res, _ := db.Exec(`INSERT INTO warehouses (business_id, name, address, manager_name, is_default) VALUES (?,?,?,?,?)`,
			bizID, w.name, w.address, w.manager, w.isDefault)
		id64, _ := res.LastInsertId()
		ids = append(ids, int(id64))
		fmt.Printf("  ✓ warehouse: %s\n", w.name)
	}
	return ids
}

// ── warehouse stock ───────────────────────────────────────────────────────────

func seedWarehouseStock(db *sql.DB, bizID int, whIDs, productIDs []int) {
	if len(whIDs) == 0 || len(productIDs) == 0 {
		return
	}
	// Distribute each product across the first two warehouses.
	for i, pid := range productIDs {
		// Get current total stock.
		var stock int
		db.QueryRow(`SELECT stock FROM products WHERE id=?`, pid).Scan(&stock)
		if stock == 0 {
			stock = rand.Intn(50) + 10
		}

		// Roughly 60% main, 40% retail.
		mainQty := int(float64(stock) * 0.6)
		retailQty := stock - mainQty

		pairs := [][2]int{
			{whIDs[0], mainQty},
		}
		if len(whIDs) > 1 {
			pairs = append(pairs, [2]int{whIDs[1], retailQty})
		}
		// Put food items in cold storage too (warehouses[2] if it exists).
		if len(whIDs) > 2 && i >= 7 && i <= 12 {
			db.Exec(`INSERT INTO warehouse_stock (warehouse_id, product_id, business_id, quantity)
				VALUES (?,?,?,?) ON DUPLICATE KEY UPDATE quantity=VALUES(quantity)`,
				whIDs[2], pid, bizID, rand.Intn(30)+10)
		}
		for _, p := range pairs {
			db.Exec(`INSERT INTO warehouse_stock (warehouse_id, product_id, business_id, quantity)
				VALUES (?,?,?,?) ON DUPLICATE KEY UPDATE quantity=VALUES(quantity)`,
				p[0], pid, bizID, p[1])
		}
	}
	fmt.Println("  ✓ warehouse stock distributed")
}

// ── stock transfers ───────────────────────────────────────────────────────────

func seedStockTransfers(db *sql.DB, bizID int, whIDs, productIDs []int) {
	if len(whIDs) < 2 || len(productIDs) == 0 {
		return
	}
	transfers := []struct {
		fromIdx, toIdx, prodIdx, qty int
		note                         string
		daysAgo                      int
	}{
		{0, 1, 0, 3, "Retail restock — Laptop", 45},
		{0, 1, 1, 20, "Retail restock — Mouse", 40},
		{0, 1, 7, 50, "Retail restock — Rice", 35},
		{0, 1, 12, 30, "Retail restock — Butter", 30},
		{0, 1, 20, 30, "Retail restock — T-Shirts", 25},
		{1, 0, 2, 5, "Return to main — overstocked hubs", 20},
		{0, 1, 17, 20, "Retail restock — Paracetamol", 18},
		{0, 1, 13, 40, "Retail restock — A4 Paper", 15},
	}
	for _, t := range transfers {
		if t.fromIdx >= len(whIDs) || t.toIdx >= len(whIDs) || t.prodIdx >= len(productIDs) {
			continue
		}
		db.Exec(`INSERT INTO stock_transfers
			(business_id, from_warehouse_id, to_warehouse_id, product_id, quantity, status, note, created_at)
			VALUES (?,?,?,?,?,?,?,?)`,
			bizID, whIDs[t.fromIdx], whIDs[t.toIdx], productIDs[t.prodIdx],
			t.qty, "completed", t.note, datetimeFmt(ago(t.daysAgo)),
		)
	}
	fmt.Println("  ✓ stock transfers")
}

// ── batches ───────────────────────────────────────────────────────────────────

func seedBatches(db *sql.DB, bizID int, whIDs, productIDs []int) {
	// Batch-tracked products: food (idx 7-12) and pharma (idx 17-19).
	type batchSpec struct {
		prodIdx    int
		batchNum   string
		lotNum     string
		mfgDaysAgo int // days before today
		expiryDays int // days from today (negative = already expired)
		qty        int
		status     string
	}
	specs := []batchSpec{
		{7, "RICE-2025-01", "LOT-R01", 180, 540, 100, "active"},
		{7, "RICE-2025-02", "LOT-R02", 90, 630, 80, "active"},
		{8, "OIL-2025-03", "LOT-O03", 120, 180, 60, "active"},
		{8, "OIL-2025-04", "LOT-O04", 10, 350, 80, "active"},
		{9, "ATTA-2025-05", "LOT-A05", 60, 120, 120, "active"},
		{9, "ATTA-2024-06", "LOT-A06", 400, -15, 30, "expired"},  // already expired
		{10, "SALT-2025-07", "LOT-S07", 200, 1200, 200, "active"},
		{11, "MAGGI-2025-08", "LOT-M08", 30, 270, 150, "active"},
		{11, "MAGGI-2025-09", "LOT-M09", 5, 25, 60, "active"},   // expiring soon
		{12, "BUTR-2025-10", "LOT-B10", 10, 45, 40, "active"},   // expiring soon
		{12, "BUTR-2024-11", "LOT-B11", 60, -5, 10, "expired"},  // expired
		{17, "PARA-2025-12", "LOT-P12", 300, 730, 200, "active"},
		{17, "PARA-2025-13", "LOT-P13", 90, 365, 150, "active"},
		{18, "VITC-2025-14", "LOT-V14", 180, 540, 80, "active"},
		{19, "ANTA-2025-15", "LOT-AN15", 150, 360, 60, "active"},
	}

	whID := 0
	if len(whIDs) > 0 {
		whID = whIDs[0]
	}

	for _, s := range specs {
		if s.prodIdx >= len(productIDs) {
			continue
		}
		pid := productIDs[s.prodIdx]

		var id int
		if db.QueryRow(`SELECT id FROM batches WHERE batch_number=? AND business_id=?`, s.batchNum, bizID).Scan(&id) == nil {
			continue
		}

		mfgDate := time.Now().AddDate(0, 0, -s.mfgDaysAgo)
		expiryDate := time.Now().AddDate(0, 0, s.expiryDays)

		db.Exec(`INSERT INTO batches
			(business_id, product_id, warehouse_id, batch_number, lot_number,
			 mfg_date, expiry_date, quantity, status, created_at)
			VALUES (?,?,?,?,?,?,?,?,?,?)`,
			bizID, pid, whID, s.batchNum, s.lotNum,
			dateFmt(mfgDate), dateFmt(expiryDate),
			s.qty, s.status, datetimeFmt(ago(s.mfgDaysAgo)),
		)
		fmt.Printf("  ✓ batch: %s (%s)\n", s.batchNum, s.status)
	}
}

// ── suppliers ─────────────────────────────────────────────────────────────────

func seedSuppliers(db *sql.DB, bizID int) []int {
	suppliers := []struct {
		code, name, email, phone, gstin, address, contact string
		terms                                              int
		limit                                              float64
	}{
		{"SUP-001", "TechCorp India Pvt Ltd", "supply@techcorp.in", "9800012345",
			"27AABCT1234R1ZX", "203, Tech Park, Andheri East, Mumbai 400093", "Vikram Shetty", 30, 500000},
		{"SUP-002", "FoodMart Distributors", "bulk@foodmart.in", "9700056789",
			"27AABCF5678M1ZY", "45, APMC Yard, Vashi, Navi Mumbai 400703", "Suresh Patil", 15, 200000},
		{"SUP-003", "FashionHub Wholesalers", "orders@fashionhub.in", "9900098765",
			"27AABCF9012K1ZP", "78, Dharavi Leather Complex, Mumbai 400017", "Deepa Shah", 30, 150000},
		{"SUP-004", "MediSupply India", "orders@medisupply.in", "9300099887",
			"27AABCM2468N1ZQ", "12, Santacruz Pharma Hub, Mumbai 400054", "Dr. Priya Nair", 7, 300000},
		{"SUP-005", "OfficeEssentials Co.", "sales@officeessentials.in", "9600034567",
			"27AABCO1357L1ZR", "55, Kurla Industrial Estate, Mumbai 400070", "Mohan Das", 30, 100000},
	}
	var ids []int
	for _, s := range suppliers {
		var id int
		if db.QueryRow(`SELECT id FROM suppliers WHERE supplier_code=? AND business_id=?`, s.code, bizID).Scan(&id) == nil {
			ids = append(ids, id)
			continue
		}
		res, _ := db.Exec(`INSERT INTO suppliers
			(business_id, supplier_code, name, email, phone, gstin, address, contact_person, payment_terms, credit_limit, status)
			VALUES (?,?,?,?,?,?,?,?,?,?,'active')`,
			bizID, s.code, s.name, s.email, s.phone, s.gstin, s.address, s.contact, s.terms, s.limit,
		)
		id64, _ := res.LastInsertId()
		ids = append(ids, int(id64))
		fmt.Printf("  ✓ supplier: %s\n", s.name)
	}
	return ids
}

// ── procurement orders ────────────────────────────────────────────────────────

func seedProcurementOrders(db *sql.DB, bizID int, supplierIDs, whIDs, productIDs []int) []int {
	if len(supplierIDs) == 0 || len(whIDs) == 0 || len(productIDs) == 0 {
		return nil
	}
	type poSpec struct {
		poNum      string
		suppIdx    int
		whIdx      int
		status     string
		daysAgo    int
		items      [][3]float64 // prodIdx, qty, unitPrice
	}
	orders := []poSpec{
		{"PO-2025-001", 0, 0, "completed", 75,
			[][3]float64{{0, 5, 42000}, {1, 50, 420}, {3, 10, 2100}}},
		{"PO-2025-002", 1, 0, "completed", 60,
			[][3]float64{{7, 100, 310}, {8, 60, 125}, {9, 80, 370}}},
		{"PO-2025-003", 0, 0, "completed", 45,
			[][3]float64{{4, 5, 2700}, {2, 15, 880}, {5, 20, 1200}}},
		{"PO-2025-004", 3, 0, "completed", 40,
			[][3]float64{{17, 200, 7}, {18, 80, 55}, {19, 60, 42}}},
		{"PO-2025-005", 1, 0, "partially_received", 20,
			[][3]float64{{10, 300, 20}, {11, 200, 108}, {12, 50, 228}}},
		{"PO-2025-006", 4, 0, "approved", 15,
			[][3]float64{{13, 100, 265}, {14, 300, 48}, {15, 30, 195}}},
		{"PO-2025-007", 2, 0, "pending", 10,
			[][3]float64{{20, 80, 175}, {21, 40, 570}, {22, 60, 420}}},
		{"PO-2025-008", 0, 0, "draft", 3,
			[][3]float64{{6, 30, 320}, {0, 3, 42000}}},
	}

	var ids []int
	for _, o := range orders {
		var id int
		if db.QueryRow(`SELECT id FROM procurement_orders WHERE po_number=? AND business_id=?`, o.poNum, bizID).Scan(&id) == nil {
			ids = append(ids, id)
			continue
		}

		suppID := 0
		if o.suppIdx < len(supplierIDs) {
			suppID = supplierIDs[o.suppIdx]
		}
		whID := 0
		if o.whIdx < len(whIDs) {
			whID = whIDs[o.whIdx]
		}

		var suppName string
		db.QueryRow(`SELECT name FROM suppliers WHERE id=?`, suppID).Scan(&suppName)

		var subtotal, taxTotal, grandTotal float64
		for _, item := range o.items {
			if int(item[0]) >= len(productIDs) {
				continue
			}
			qty := item[1]
			up := item[2]
			// assume 18% GST for electronics, 0% for food
			taxRate := 18.0
			if int(item[0]) >= 7 && int(item[0]) <= 12 {
				taxRate = 0
			}
			lineTotal := qty * up
			taxAmt := lineTotal * taxRate / 100
			subtotal += lineTotal
			taxTotal += taxAmt
			grandTotal += lineTotal + taxAmt
		}

		expDate := time.Now().AddDate(0, 0, -o.daysAgo+30).Format("2006-01-02")
		poAt := datetimeFmt(ago(o.daysAgo))

		res, _ := db.Exec(`INSERT INTO procurement_orders
			(business_id, supplier_id, supplier_name, po_number, status, warehouse_id,
			 expected_date, subtotal, tax_total, grand_total, approved_by, created_at, updated_at)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			bizID, suppID, suppName, o.poNum, o.status, whID,
			expDate, subtotal, taxTotal, grandTotal,
			"Anand Mehta", poAt, poAt,
		)
		poID64, _ := res.LastInsertId()
		poID := int(poID64)
		ids = append(ids, poID)

		for _, item := range o.items {
			if int(item[0]) >= len(productIDs) {
				continue
			}
			pid := productIDs[int(item[0])]
			qty := int(item[1])
			up := item[2]
			taxRate := 18.0
			if int(item[0]) >= 7 && int(item[0]) <= 12 {
				taxRate = 0
			}
			lineTotal := float64(qty) * up
			taxAmt := lineTotal * taxRate / 100

			var prodName, sku string
			db.QueryRow(`SELECT name, sku FROM products WHERE id=?`, pid).Scan(&prodName, &sku)

			receivedQty := qty
			if o.status == "partially_received" {
				receivedQty = qty / 2
			} else if o.status == "pending" || o.status == "draft" || o.status == "approved" {
				receivedQty = 0
			}

			db.Exec(`INSERT INTO procurement_order_items
				(order_id, product_id, product_name, sku, quantity, received_qty, unit_price, tax_rate, tax_amount, line_total)
				VALUES (?,?,?,?,?,?,?,?,?,?)`,
				poID, pid, prodName, sku, qty, receivedQty, up, taxRate, taxAmt, lineTotal+taxAmt,
			)
		}
		fmt.Printf("  ✓ procurement order: %s (%s)\n", o.poNum, o.status)
	}
	return ids
}

// ── supplier payments ─────────────────────────────────────────────────────────

func seedSupplierPayments(db *sql.DB, bizID int, supplierIDs, poIDs []int) {
	if len(supplierIDs) == 0 || len(poIDs) == 0 {
		return
	}
	payments := []struct {
		payNum  string
		suppIdx int
		poIdx   int
		amount  float64
		method  string
		daysAgo int
	}{
		{"SPY-001", 0, 0, 250000, "bank_transfer", 68},
		{"SPY-002", 1, 1, 50000, "cheque", 55},
		{"SPY-003", 0, 2, 180000, "bank_transfer", 38},
		{"SPY-004", 3, 3, 15000, "upi", 33},
		{"SPY-005", 1, 4, 30000, "cash", 12},
		{"SPY-006", 4, 5, 40000, "bank_transfer", 8},
	}
	for _, p := range payments {
		var id int
		if db.QueryRow(`SELECT id FROM supplier_payments WHERE payment_number=? AND business_id=?`, p.payNum, bizID).Scan(&id) == nil {
			continue
		}
		suppID := 0
		if p.suppIdx < len(supplierIDs) {
			suppID = supplierIDs[p.suppIdx]
		}
		poID := 0
		if p.poIdx < len(poIDs) {
			poID = poIDs[p.poIdx]
		}

		var suppName, poNum string
		db.QueryRow(`SELECT name FROM suppliers WHERE id=?`, suppID).Scan(&suppName)
		db.QueryRow(`SELECT po_number FROM procurement_orders WHERE id=?`, poID).Scan(&poNum)

		db.Exec(`INSERT INTO supplier_payments
			(business_id, supplier_id, supplier_name, order_id, po_number,
			 payment_number, amount, payment_method, created_at)
			VALUES (?,?,?,?,?,?,?,?,?)`,
			bizID, suppID, suppName, poID, poNum,
			p.payNum, p.amount, p.method, datetimeFmt(ago(p.daysAgo)),
		)
		fmt.Printf("  ✓ supplier payment: %s Rs.%.0f\n", p.payNum, p.amount)
	}
}

// ── CRM customers ─────────────────────────────────────────────────────────────

func seedCRMCustomers(db *sql.DB, bizID int) []int {
	customers := []struct {
		code, name, email, phone, gstin, address, group string
		terms                                            int
		limit                                            float64
	}{
		{"CRM-001", "Rahul Sharma Enterprises", "rahul@sharmaenterprises.in", "9811001122",
			"27AAPRS5678F1ZV", "45, Khar West, Mumbai 400052", "retail", 30, 100000},
		{"CRM-002", "Priya Patel & Co.", "priya@ppatel.co.in", "9822334455",
			"24AAACR5055K1ZA", "78, CG Road, Ahmedabad 380009", "wholesale", 45, 500000},
		{"CRM-003", "Sunita Trading Co.", "sunita@sunitatrading.in", "9833556677",
			"29AABCT1332L1ZN", "12, Infantry Road, Bengaluru 560001", "distributor", 60, 750000},
		{"CRM-004", "Rajan Industries Pvt Ltd", "rajan@rajanindustries.in", "9844778899",
			"33AAACR5055K1ZB", "89, Nungambakkam High Road, Chennai 600034", "wholesale", 30, 300000},
		{"CRM-005", "Meena Retail Group", "meena@meenaretail.in", "9855990011",
			"", "23, MG Road, Indore 452001", "retail", 0, 50000},
		{"CRM-006", "Global Tech Solutions", "purchase@globaltech.in", "9866112233",
			"09AAACG4104N1ZJ", "Tower B, Cyber City, Gurugram 122002", "corporate", 30, 1000000},
		{"CRM-007", "Anil Kumar & Brothers", "anil@akbrothers.in", "9877334455",
			"", "67, New Market, Bhopal 462001", "retail", 15, 75000},
		{"CRM-008", "Laxmi Stores Chain", "laxmi@laxmistores.in", "9888556677",
			"19AABCT1332L1ZM", "14, Gariahat Road, Kolkata 700019", "distributor", 45, 400000},
	}
	var ids []int
	for _, c := range customers {
		var id int
		if db.QueryRow(`SELECT id FROM crm_customers WHERE customer_code=? AND business_id=?`, c.code, bizID).Scan(&id) == nil {
			ids = append(ids, id)
			continue
		}
		res, _ := db.Exec(`INSERT INTO crm_customers
			(business_id, customer_code, name, email, phone, gstin, billing_address,
			 customer_group, payment_terms, credit_limit, status)
			VALUES (?,?,?,?,?,?,?,?,?,?,'active')`,
			bizID, c.code, c.name, c.email, c.phone, c.gstin, c.address,
			c.group, c.terms, c.limit,
		)
		id64, _ := res.LastInsertId()
		ids = append(ids, int(id64))
		fmt.Printf("  ✓ CRM customer: %s\n", c.name)
	}
	return ids
}

// ── sales orders ──────────────────────────────────────────────────────────────

func seedSalesOrders(db *sql.DB, bizID int, customerIDs, whIDs, productIDs []int) []int {
	if len(customerIDs) == 0 || len(whIDs) == 0 || len(productIDs) == 0 {
		return nil
	}
	type soSpec struct {
		soNum   string
		custIdx int
		status  string
		daysAgo int
		items   [][3]float64 // prodIdx, qty, unitPrice
	}
	orders := []soSpec{
		{"SO-2025-001", 5, "completed", 70, [][3]float64{{0, 2, 52999}, {1, 5, 799}}},
		{"SO-2025-002", 1, "completed", 55, [][3]float64{{20, 50, 349}, {21, 20, 999}, {22, 30, 749}}},
		{"SO-2025-003", 2, "completed", 45, [][3]float64{{7, 100, 425}, {8, 50, 165}, {9, 80, 490}}},
		{"SO-2025-004", 0, "completed", 38, [][3]float64{{4, 3, 3499}, {3, 5, 3499}, {6, 10, 599}}},
		{"SO-2025-005", 3, "delivered", 30, [][3]float64{{0, 1, 52999}, {2, 3, 1499}, {5, 5, 1999}}},
		{"SO-2025-006", 7, "delivered", 22, [][3]float64{{13, 50, 380}, {14, 200, 85}, {15, 20, 320}}},
		{"SO-2025-007", 4, "packed", 14, [][3]float64{{20, 30, 349}, {21, 10, 999}}},
		{"SO-2025-008", 6, "confirmed", 8, [][3]float64{{23, 5, 320}, {24, 5, 495}}},
		{"SO-2025-009", 1, "draft", 3, [][3]float64{{0, 3, 52999}, {4, 5, 3499}}},
		{"SO-2025-010", 5, "confirmed", 1, [][3]float64{{6, 20, 599}, {5, 10, 1999}}},
	}

	var ids []int
	for _, o := range orders {
		var id int
		if db.QueryRow(`SELECT id FROM sales_orders WHERE order_number=? AND business_id=?`, o.soNum, bizID).Scan(&id) == nil {
			ids = append(ids, id)
			continue
		}
		custID := 0
		if o.custIdx < len(customerIDs) {
			custID = customerIDs[o.custIdx]
		}
		whID := 0
		if len(whIDs) > 0 {
			whID = whIDs[0]
		}
		var custName string
		db.QueryRow(`SELECT name FROM crm_customers WHERE id=?`, custID).Scan(&custName)

		var subtotal, taxTotal, grandTotal float64
		for _, item := range o.items {
			if int(item[0]) >= len(productIDs) {
				continue
			}
			qty := item[1]
			up := item[2]
			taxRate := 18.0
			if int(item[0]) >= 7 && int(item[0]) <= 12 {
				taxRate = 0
			}
			lineTotal := qty * up
			taxAmt := lineTotal * taxRate / 100
			subtotal += lineTotal
			taxTotal += taxAmt
			grandTotal += lineTotal + taxAmt
		}

		soAt := datetimeFmt(ago(o.daysAgo))
		res, _ := db.Exec(`INSERT INTO sales_orders
			(business_id, customer_id, customer_name, order_number, status, warehouse_id,
			 subtotal, tax_total, grand_total, created_at, updated_at)
			VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
			bizID, custID, custName, o.soNum, o.status, whID,
			subtotal, taxTotal, grandTotal, soAt, soAt,
		)
		soID64, _ := res.LastInsertId()
		soID := int(soID64)
		ids = append(ids, soID)

		for _, item := range o.items {
			if int(item[0]) >= len(productIDs) {
				continue
			}
			pid := productIDs[int(item[0])]
			qty := int(item[1])
			up := item[2]
			taxRate := 18.0
			if int(item[0]) >= 7 && int(item[0]) <= 12 {
				taxRate = 0
			}
			lineTotal := float64(qty) * up
			taxAmt := lineTotal * taxRate / 100

			var prodName, sku string
			db.QueryRow(`SELECT name, sku FROM products WHERE id=?`, pid).Scan(&prodName, &sku)

			db.Exec(`INSERT INTO sales_order_items
				(order_id, product_id, product_name, sku, quantity, unit_price, tax_rate, tax_amount, line_total)
				VALUES (?,?,?,?,?,?,?,?,?)`,
				soID, pid, prodName, sku, qty, up, taxRate, taxAmt, lineTotal+taxAmt,
			)
		}
		fmt.Printf("  ✓ sales order: %s (%s)\n", o.soNum, o.status)
	}
	return ids
}

// ── deliveries ────────────────────────────────────────────────────────────────

func seedDeliveries(db *sql.DB, bizID int, soIDs, customerIDs, whIDs []int) {
	if len(soIDs) < 6 || len(customerIDs) == 0 || len(whIDs) == 0 {
		return
	}
	deliveries := []struct {
		delNum  string
		soIdx   int
		custIdx int
		status  string
		daysAgo int
	}{
		{"DEL-2025-001", 0, 5, "delivered", 62},
		{"DEL-2025-002", 1, 1, "delivered", 48},
		{"DEL-2025-003", 2, 2, "delivered", 38},
		{"DEL-2025-004", 3, 0, "delivered", 30},
		{"DEL-2025-005", 4, 3, "delivered", 22},
		{"DEL-2025-006", 5, 7, "delivered", 15},
	}
	for _, d := range deliveries {
		if d.soIdx >= len(soIDs) || d.custIdx >= len(customerIDs) {
			continue
		}
		var id int
		if db.QueryRow(`SELECT id FROM deliveries WHERE delivery_number=? AND business_id=?`, d.delNum, bizID).Scan(&id) == nil {
			continue
		}
		var custName string
		db.QueryRow(`SELECT name FROM crm_customers WHERE id=?`, customerIDs[d.custIdx]).Scan(&custName)
		delAt := datetimeFmt(ago(d.daysAgo))
		db.Exec(`INSERT INTO deliveries
			(business_id, order_id, customer_id, customer_name, delivery_number,
			 warehouse_id, status, created_at, updated_at)
			VALUES (?,?,?,?,?,?,?,?,?)`,
			bizID, soIDs[d.soIdx], customerIDs[d.custIdx], custName,
			d.delNum, whIDs[0], d.status, delAt, delAt,
		)
		fmt.Printf("  ✓ delivery: %s\n", d.delNum)
	}
}

// ── CRM payments ─────────────────────────────────────────────────────────────

func seedCRMPayments(db *sql.DB, bizID int, soIDs, customerIDs []int) {
	if len(soIDs) == 0 || len(customerIDs) == 0 {
		return
	}
	payments := []struct {
		payNum  string
		custIdx int
		soIdx   int
		amount  float64
		method  string
		daysAgo int
	}{
		{"CPY-001", 5, 0, 125553, "bank_transfer", 65},
		{"CPY-002", 1, 1, 52400, "cheque", 50},
		{"CPY-003", 2, 2, 63700, "upi", 40},
		{"CPY-004", 0, 3, 48000, "bank_transfer", 32},
		{"CPY-005", 3, 4, 73400, "bank_transfer", 25},
		{"CPY-006", 7, 5, 28560, "upi", 17},
		{"CPY-007", 4, 6, 15000, "cash", 10},
	}
	for _, p := range payments {
		if p.custIdx >= len(customerIDs) || p.soIdx >= len(soIDs) {
			continue
		}
		var id int
		if db.QueryRow(`SELECT id FROM crm_payments WHERE payment_number=? AND business_id=?`, p.payNum, bizID).Scan(&id) == nil {
			continue
		}
		var custName string
		db.QueryRow(`SELECT name FROM crm_customers WHERE id=?`, customerIDs[p.custIdx]).Scan(&custName)

		db.Exec(`INSERT INTO crm_payments
			(business_id, customer_id, customer_name, order_id, payment_number,
			 amount, payment_method, created_at)
			VALUES (?,?,?,?,?,?,?,?)`,
			bizID, customerIDs[p.custIdx], custName, soIDs[p.soIdx],
			p.payNum, p.amount, p.method, datetimeFmt(ago(p.daysAgo)),
		)
		fmt.Printf("  ✓ CRM payment: %s Rs.%.0f\n", p.payNum, p.amount)
	}
}

// ── generic invoices ──────────────────────────────────────────────────────────

func seedInvoices(db *sql.DB, bizID int) {
	invoices := []struct {
		num, customer, status string
		total                 float64
		daysAgo               int
	}{
		{"INV-2025-001", "Global Tech Solutions", "paid", 125553, 68},
		{"INV-2025-002", "Priya Patel & Co.", "paid", 52400, 55},
		{"INV-2025-003", "Sunita Trading Co.", "paid", 63700, 42},
		{"INV-2025-004", "Rahul Sharma Enterprises", "paid", 48000, 35},
		{"INV-2025-005", "Rajan Industries Pvt Ltd", "paid", 73400, 28},
		{"INV-2025-006", "Laxmi Stores Chain", "paid", 28560, 20},
		{"INV-2025-007", "Meena Retail Group", "pending", 15000, 14},
		{"INV-2025-008", "Anil Kumar & Brothers", "pending", 18500, 10},
		{"INV-2025-009", "Global Tech Solutions", "pending", 89990, 5},
		{"INV-2025-010", "Sunita Trading Co.", "overdue", 34800, 45},
		{"INV-2025-011", "Rajan Industries Pvt Ltd", "overdue", 22000, 38},
		{"INV-2025-012", "Priya Patel & Co.", "draft", 67200, 2},
	}
	for _, inv := range invoices {
		var id int
		if db.QueryRow(`SELECT id FROM invoices WHERE number=? AND business_id=?`, inv.num, bizID).Scan(&id) == nil {
			continue
		}
		insert(db,
			`INSERT INTO invoices (business_id, number, customer, total, status, created_at) VALUES (?,?,?,?,?,?)`,
			bizID, inv.num, inv.customer, inv.total, inv.status, datetimeFmt(ago(inv.daysAgo)),
		)
		fmt.Printf("  ✓ invoice: %s (%s)\n", inv.num, inv.status)
	}
}

func seedPayments(db *sql.DB, bizID int) {
	payments := []struct {
		invoice, method string
		amount          float64
		daysAgo         int
	}{
		{"INV-2025-001", "bank_transfer", 125553, 65},
		{"INV-2025-002", "cheque", 52400, 52},
		{"INV-2025-003", "upi", 63700, 40},
		{"INV-2025-004", "bank_transfer", 48000, 33},
		{"INV-2025-005", "bank_transfer", 73400, 25},
		{"INV-2025-006", "upi", 28560, 18},
		{"INV-2025-007", "cash", 10000, 8},
	}
	for _, p := range payments {
		var count int
		db.QueryRow(`SELECT COUNT(*) FROM payments WHERE invoice=? AND amount=? AND business_id=?`,
			p.invoice, p.amount, bizID).Scan(&count)
		if count > 0 {
			continue
		}
		insert(db, `INSERT INTO payments (business_id, invoice, amount, method, created_at) VALUES (?,?,?,?,?)`,
			bizID, p.invoice, p.amount, p.method, datetimeFmt(ago(p.daysAgo)),
		)
		fmt.Printf("  ✓ payment: %s Rs.%.0f\n", p.invoice, p.amount)
	}
}

// ── POS sales ─────────────────────────────────────────────────────────────────

func seedPOSSales(db *sql.DB, bizID int, whIDs, productIDs []int) {
	if len(whIDs) == 0 || len(productIDs) == 0 {
		return
	}
	whID := whIDs[0]
	if len(whIDs) > 1 {
		whID = whIDs[1] // retail store
	}

	// POS item templates: prodIdx, qty, unitPrice
	type posItem struct {
		prodIdx int
		qty     int
		price   float64
	}
	saleTemplates := [][]posItem{
		{{1, 2, 799}, {13, 1, 380}, {14, 3, 85}},
		{{7, 5, 425}, {8, 2, 165}, {10, 2, 28}},
		{{20, 3, 349}, {16, 2, 599}},
		{{17, 2, 12}, {18, 1, 85}, {19, 1, 68}},
		{{11, 4, 145}, {12, 1, 285}},
		{{1, 1, 799}, {2, 1, 1499}, {5, 1, 1999}},
		{{13, 2, 380}, {14, 5, 85}, {15, 1, 320}},
		{{7, 10, 425}, {9, 3, 490}, {10, 5, 28}},
		{{20, 5, 349}, {21, 1, 999}},
		{{3, 1, 3499}, {1, 1, 799}},
	}
	methods := []string{"cash", "upi", "card", "cash", "upi", "cash", "upi", "card"}

	saleNum := 1
	for day := 45; day >= 1; day-- {
		salesPerDay := rand.Intn(4) + 2 // 2–5 sales per day
		for s := 0; s < salesPerDay; s++ {
			tmpl := saleTemplates[rand.Intn(len(saleTemplates))]
			method := methods[rand.Intn(len(methods))]

			var subtotal, taxTotal float64
			for _, item := range tmpl {
				if item.prodIdx >= len(productIDs) {
					continue
				}
				lineTotal := float64(item.qty) * item.price
				taxRate := 18.0
				if item.prodIdx >= 7 && item.prodIdx <= 12 {
					taxRate = 0
				}
				subtotal += lineTotal
				taxTotal += lineTotal * taxRate / 100
			}
			grandTotal := subtotal + taxTotal

			saleNumStr := fmt.Sprintf("POS-%04d", saleNum)
			createdAt := ago(day).Add(time.Duration(rand.Intn(8)+9) * time.Hour)

			res, err := db.Exec(`INSERT INTO pos_sales
				(business_id, warehouse_id, sale_number, customer_name, subtotal,
				 tax_total, grand_total, payment_method, amount_paid, change_given, status, created_at)
				VALUES (?,?,?,'Walk-in Customer',?,?,?,?,?,?,'completed',?)`,
				bizID, whID, saleNumStr, subtotal, taxTotal, grandTotal,
				method, grandTotal, 0, datetimeFmt(createdAt),
			)
			if err == nil {
				saleID64, _ := res.LastInsertId()
				for _, item := range tmpl {
					if item.prodIdx >= len(productIDs) {
						continue
					}
					pid := productIDs[item.prodIdx]
					taxRate := 18.0
					if item.prodIdx >= 7 && item.prodIdx <= 12 {
						taxRate = 0
					}
					lineTotal := float64(item.qty) * item.price
					taxAmt := lineTotal * taxRate / 100
					var prodName, sku string
					db.QueryRow(`SELECT name, sku FROM products WHERE id=?`, pid).Scan(&prodName, &sku)
					db.Exec(`INSERT INTO pos_sale_items
						(sale_id, product_id, product_name, sku, quantity, unit_price, tax_rate, tax_amount, line_total)
						VALUES (?,?,?,?,?,?,?,?,?)`,
						saleID64, pid, prodName, sku, item.qty, item.price, taxRate, taxAmt, lineTotal+taxAmt,
					)
				}
				saleNum++
			}
		}
	}
	fmt.Printf("  ✓ POS sales: %d transactions (45 days)\n", saleNum-1)
}

// ── expense categories ────────────────────────────────────────────────────────

func seedExpenseCategories(db *sql.DB, bizID int) []int {
	cats := [][2]string{
		{"Rent & Lease", "Monthly rent for shop and warehouse"},
		{"Salaries & Wages", "Staff salaries, wages, and bonuses"},
		{"Utilities", "Electricity, water, internet, phone"},
		{"Marketing & Advertising", "Promotions, ads, digital marketing"},
		{"Transport & Logistics", "Delivery charges, fuel, vehicle maintenance"},
		{"Office & Admin", "Stationery, printing, office miscellaneous"},
		{"Repairs & Maintenance", "Equipment, fixtures, property repairs"},
		{"Professional Services", "CA fees, legal, consulting charges"},
	}
	var ids []int
	for _, c := range cats {
		var id int
		if db.QueryRow(`SELECT id FROM expense_categories WHERE name=? AND business_id=?`, c[0], bizID).Scan(&id) == nil {
			ids = append(ids, id)
			continue
		}
		res, _ := db.Exec(`INSERT INTO expense_categories (business_id, name, description) VALUES (?,?,?)`, bizID, c[0], c[1])
		id64, _ := res.LastInsertId()
		ids = append(ids, int(id64))
		fmt.Printf("  ✓ expense category: %s\n", c[0])
	}
	return ids
}

// ── bank accounts ─────────────────────────────────────────────────────────────

func seedBankAccounts(db *sql.DB, bizID int) []int {
	accounts := []struct {
		name, bank, num, ifsc string
		opening                float64
	}{
		{"Current Account — HDFC", "HDFC Bank", "50100123456789", "HDFC0001234", 250000},
		{"Current Account — SBI", "State Bank of India", "32456789012345", "SBIN0001567", 180000},
		{"Cash Register", "Cash", "CASH-001", "N/A", 45000},
	}
	var ids []int
	for _, a := range accounts {
		var id int
		if db.QueryRow(`SELECT id FROM bank_accounts WHERE account_number=? AND business_id=?`, a.num, bizID).Scan(&id) == nil {
			ids = append(ids, id)
			continue
		}
		res, _ := db.Exec(`INSERT INTO bank_accounts
			(business_id, account_name, bank_name, account_number, ifsc, opening_balance, current_balance, status)
			VALUES (?,?,?,?,?,?,?,'active')`,
			bizID, a.name, a.bank, a.num, a.ifsc, a.opening, a.opening,
		)
		id64, _ := res.LastInsertId()
		ids = append(ids, int(id64))
		fmt.Printf("  ✓ bank account: %s\n", a.name)
	}
	return ids
}

// ── expenses ──────────────────────────────────────────────────────────────────

func seedExpenses(db *sql.DB, bizID int, catIDs, bankIDs []int) {
	if len(catIDs) == 0 {
		return
	}
	type expSpec struct {
		catIdx  int
		amount  float64
		method  string
		desc    string
		daysAgo int
		status  string
	}
	expenses := []expSpec{
		{0, 45000, "bank", "Shop rent — Gandhi Nagar Market, May 2025", 30, "approved"},
		{0, 18000, "bank", "Warehouse rent — Andheri East, May 2025", 30, "approved"},
		{1, 85000, "bank", "Staff salaries — May 2025", 15, "approved"},
		{2, 8500, "bank", "Electricity bill — MSEDCL April 2025", 28, "approved"},
		{2, 3200, "upi", "Jio Fiber internet — May 2025", 20, "approved"},
		{3, 12000, "upi", "Google Ads — Electronics campaign", 25, "approved"},
		{3, 8000, "upi", "Instagram/Facebook boost", 18, "approved"},
		{4, 4500, "cash", "Delivery charges — 3PL courier", 22, "approved"},
		{4, 2200, "cash", "Fuel reimbursement — staff bike", 16, "approved"},
		{5, 1850, "cash", "Stationery and printing supplies", 35, "approved"},
		{6, 6500, "bank", "AC repair — Main Warehouse", 12, "approved"},
		{7, 18000, "bank", "CA firm — quarterly accounting", 40, "approved"},
		{0, 45000, "bank", "Shop rent — Gandhi Nagar Market, April 2025", 60, "approved"},
		{0, 18000, "bank", "Warehouse rent — Andheri East, April 2025", 60, "approved"},
		{1, 82000, "bank", "Staff salaries — April 2025", 45, "approved"},
		{3, 15000, "bank", "Newspaper ad + pamphlet distribution", 50, "approved"},
		{2, 9200, "bank", "Electricity bill — MSEDCL March 2025", 58, "approved"},
		{4, 5800, "upi", "Delivery partner charges — March", 55, "approved"},
		{6, 3200, "cash", "Shelf repair and painting", 42, "approved"},
		{1, 5000, "cash", "Bonus — Diwali gift for staff", 7, "pending"},
	}

	bankID := 0
	if len(bankIDs) > 0 {
		bankID = bankIDs[0]
	}

	for i, e := range expenses {
		catID := 0
		if e.catIdx < len(catIDs) {
			catID = catIDs[e.catIdx]
		}
		method := e.method
		if method == "bank" {
			method = "bank"
		}
		var count int
		db.QueryRow(`SELECT COUNT(*) FROM expenses WHERE description=? AND business_id=?`, e.desc, bizID).Scan(&count)
		if count > 0 {
			continue
		}
		db.Exec(`INSERT INTO expenses
			(business_id, category_id, amount, payment_method, bank_account_id, description,
			 expense_date, status, created_at)
			VALUES (?,?,?,?,?,?,?,?,?)`,
			bizID, catID, e.amount, method, bankID, e.desc,
			dateFmt(time.Now().AddDate(0, 0, -e.daysAgo)),
			e.status, datetimeFmt(ago(e.daysAgo)),
		)
		_ = i
	}
	fmt.Printf("  ✓ expenses: %d records\n", len(expenses))
}

// ── bank transactions ─────────────────────────────────────────────────────────

func seedBankTransactions(db *sql.DB, bizID int, bankIDs []int) {
	if len(bankIDs) == 0 {
		return
	}
	bankID := bankIDs[0]
	txns := []struct {
		txType, desc, ref string
		amount            float64
		daysAgo           int
	}{
		{"credit", "Payment received — Global Tech Solutions SO-2025-001", "RTGS-4521", 125553, 65},
		{"credit", "Payment received — Priya Patel & Co. SO-2025-002", "NEFT-8932", 52400, 52},
		{"credit", "Payment received — Sunita Trading SO-2025-003", "UPI-7634", 63700, 40},
		{"debit", "Supplier payment — TechCorp India PO-2025-001", "RTGS-1122", 250000, 68},
		{"debit", "Supplier payment — FoodMart Distributors PO-2025-002", "CHQ-5544", 50000, 55},
		{"credit", "Payment received — Rajan Industries SO-2025-005", "NEFT-3321", 73400, 25},
		{"debit", "Staff salaries May 2025", "SAL-0525", 85000, 15},
		{"debit", "Rent payment May 2025", "NEFT-6677", 63000, 30},
		{"credit", "POS sales batch — Week 1", "POS-BATCH-W1", 38500, 42},
		{"credit", "POS sales batch — Week 2", "POS-BATCH-W2", 45200, 35},
		{"credit", "POS sales batch — Week 3", "POS-BATCH-W3", 41800, 28},
		{"credit", "POS sales batch — Week 4", "POS-BATCH-W4", 39600, 21},
		{"debit", "CA firm payment — quarterly", "NEFT-2233", 18000, 40},
		{"debit", "Google Ads payment", "UPI-8899", 20000, 22},
		{"credit", "Payment received — Laxmi Stores SO-2025-006", "UPI-4455", 28560, 17},
	}
	for _, t := range txns {
		var count int
		db.QueryRow(`SELECT COUNT(*) FROM bank_transactions WHERE reference=? AND business_id=?`, t.ref, bizID).Scan(&count)
		if count > 0 {
			continue
		}
		db.Exec(`INSERT INTO bank_transactions
			(business_id, account_id, transaction_type, amount, reference, description, transaction_date, created_at)
			VALUES (?,?,?,?,?,?,?,?)`,
			bizID, bankID, t.txType, t.amount, t.ref, t.desc,
			dateFmt(time.Now().AddDate(0, 0, -t.daysAgo)),
			datetimeFmt(ago(t.daysAgo)),
		)
	}
	fmt.Printf("  ✓ bank transactions: %d records\n", len(txns))
}

// ── notifications ─────────────────────────────────────────────────────────────

func seedNotifications(db *sql.DB, userIDs map[string]int) {
	adminID := userIDs["admin"]
	if adminID == 0 {
		return
	}
	notifs := []struct {
		nType, message, module, recordID string
	}{
		{"warning", "3 batches are expiring within 30 days — review Expiry Tracker", "batches", ""},
		{"error", "2 batches have already expired — write off required", "batches", ""},
		{"info", "New sales order SO-2025-010 received from Global Tech Solutions", "crm", "10"},
		{"warning", "Product 'Basmati Rice 5kg' is below reorder threshold — 200 units remaining", "products", ""},
		{"success", "Purchase Order PO-2025-003 fully received and GRN created", "procurement", "3"},
		{"info", "Invoice INV-2025-010 is overdue by 15 days — Rs.34,800 pending", "invoices", "10"},
		{"warning", "Low stock alert: Laptop 15\" — only 12 units in Main Warehouse", "products", ""},
		{"success", "Delivery DEL-2025-006 marked as delivered to Laxmi Stores Chain", "crm", "6"},
	}
	for _, n := range notifs {
		var count int
		db.QueryRow(`SELECT COUNT(*) FROM notifications WHERE user_id=? AND message=?`, adminID, n.message).Scan(&count)
		if count > 0 {
			continue
		}
		db.Exec(`INSERT INTO notifications (user_id, type, message, module, record_id) VALUES (?,?,?,?,?)`,
			adminID, n.nType, n.message, n.module, n.recordID,
		)
	}
	fmt.Printf("  ✓ notifications: %d records\n", len(notifs))
}

// ── audit log ─────────────────────────────────────────────────────────────────

func seedAuditLog(db *sql.DB, bizID int, userIDs map[string]int) {
	adminID := userIDs["admin"]
	staffID := userIDs["staff"]
	if adminID == 0 {
		return
	}

	type auditEntry struct {
		module, action string
		recordID       int
		userName       string
		userID         int
		daysAgo        int
	}
	entries := []auditEntry{
		{"products", "create", 1, "Anand Mehta", adminID, 80},
		{"products", "update", 3, "Ramesh Kumar", userIDs["manager"], 75},
		{"invoices", "create", 1, "Anand Mehta", adminID, 68},
		{"invoices", "update", 1, "Kavita Sharma", userIDs["accountant"], 65},
		{"procurement", "create", 1, "Anand Mehta", adminID, 75},
		{"procurement", "update", 1, "Ramesh Kumar", userIDs["manager"], 70},
		{"customers", "create", 1, "Sunita Yadav", staffID, 60},
		{"pos", "create", 1, "Sunita Yadav", staffID, 45},
		{"products", "stock_adjust", 2, "Ramesh Kumar", userIDs["manager"], 50},
		{"warehouse", "transfer", 1, "Ramesh Kumar", userIDs["manager"], 40},
		{"crm", "create", 1, "Sunita Yadav", staffID, 38},
		{"finance", "expense_create", 1, "Kavita Sharma", userIDs["accountant"], 35},
		{"invoices", "create", 5, "Anand Mehta", adminID, 28},
		{"batches", "create", 1, "Ramesh Kumar", userIDs["manager"], 25},
		{"users", "update", 2, "Anand Mehta", adminID, 20},
		{"pos", "create", 15, "Sunita Yadav", staffID, 15},
		{"procurement", "create", 5, "Ramesh Kumar", userIDs["manager"], 12},
		{"finance", "bank_transaction", 10, "Kavita Sharma", userIDs["accountant"], 8},
		{"invoices", "create", 10, "Anand Mehta", adminID, 5},
		{"crm", "delivery_create", 6, "Sunita Yadav", staffID, 3},
	}

	for _, e := range entries {
		uid := e.userID
		if uid == 0 {
			uid = adminID
		}
		db.Exec(`INSERT INTO audit_log
			(business_id, user_id, user_name, module, action, record_id, created_at)
			VALUES (?,?,?,?,?,?,?)`,
			bizID, uid, e.userName, e.module, e.action, e.recordID,
			datetimeFmt(ago(e.daysAgo)),
		)
	}
	fmt.Printf("  ✓ audit log: %d entries\n", len(entries))
}
