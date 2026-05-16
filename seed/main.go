package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
)

func main() {
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
		log.Fatalf("cannot connect to MySQL: %v", err)
	}
	fmt.Println("Connected to MySQL — seeding...")

	seedUsers(db)
	seedCategories(db)
	seedVendors(db)
	seedCustomers(db)
	seedProducts(db)
	seedInvoices(db)
	seedPayments(db)
	seedAccounts(db)

	fmt.Println("\nDone! Login at http://localhost:8080")
	fmt.Println("  Admin  → admin@invobill.com  / admin123456")
	fmt.Println("  Staff  → staff@invobill.com  / staff123456")
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func exists(db *sql.DB, table, col, val string) bool {
	var id int
	err := db.QueryRow(
		fmt.Sprintf("SELECT id FROM %s WHERE %s = ? AND deleted_at IS NULL LIMIT 1", table, col),
		val,
	).Scan(&id)
	return err == nil
}

func insert(db *sql.DB, q string, args ...any) int64 {
	res, err := db.Exec(q, args...)
	if err != nil {
		log.Printf("insert error: %v", err)
		return 0
	}
	id, _ := res.LastInsertId()
	return id
}

func hashpw(pw string) string {
	h, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	if err != nil {
		log.Fatal(err)
	}
	return string(h)
}

// ─── users ───────────────────────────────────────────────────────────────────

func seedUsers(db *sql.DB) {
	users := []struct{ name, email, pw, role string }{
		{"Admin User", "admin@invobill.com", "admin123456", "admin"},
		{"Staff User", "staff@invobill.com", "staff123456", "staff"},
		{"Ramesh Manager", "manager@invobill.com", "manager123", "manager"},
		{"Sita Accountant", "accounts@invobill.com", "accounts123", "accountant"},
	}
	for _, u := range users {
		var id int
		err := db.QueryRow("SELECT id FROM users WHERE email = ? LIMIT 1", u.email).Scan(&id)
		if err == nil {
			fmt.Printf("  skip user: %s (already exists)\n", u.email)
			continue
		}
		db.Exec(
			`INSERT INTO users (name, email, password_hash, role) VALUES (?, ?, ?, ?)`,
			u.name, u.email, hashpw(u.pw), u.role,
		)
		fmt.Printf("  ✓ user: %s [%s]\n", u.email, u.role)
	}
}

// ─── categories ──────────────────────────────────────────────────────────────

func seedCategories(db *sql.DB) {
	cats := []struct{ name, desc string }{
		{"Electronics", "Laptops, phones, accessories"},
		{"Clothing", "Apparel and fashion items"},
		{"Food & Beverages", "Grocery and consumables"},
		{"Office Supplies", "Stationery and office essentials"},
		{"Tools & Hardware", "Workshop and maintenance tools"},
		{"Furniture", "Office and home furniture"},
	}
	for _, c := range cats {
		if exists(db, "categories", "name", c.name) {
			continue
		}
		insert(db, `INSERT INTO categories (name, description) VALUES (?, ?)`, c.name, c.desc)
		fmt.Printf("  ✓ category: %s\n", c.name)
	}
}

// ─── vendors ─────────────────────────────────────────────────────────────────

func seedVendors(db *sql.DB) {
	vendors := []struct{ name, email, phone, status string }{
		{"TechCorp India Pvt Ltd", "supply@techcorp.in", "9800012345", "active"},
		{"FashionHub Wholesalers", "orders@fashionhub.in", "9900098765", "active"},
		{"FoodMart Distributors", "bulk@foodmart.in", "9700056789", "active"},
		{"OfficeEssentials Co.", "sales@officeessentials.in", "9600034567", "active"},
		{"HardwarePro Suppliers", "info@hardwarepro.in", "9500078901", "active"},
		{"FurniWorld Pvt Ltd", "contact@furniworld.in", "9400011223", "inactive"},
	}
	for _, v := range vendors {
		if exists(db, "vendors", "name", v.name) {
			continue
		}
		insert(db, `INSERT INTO vendors (name, email, phone, status) VALUES (?, ?, ?, ?)`,
			v.name, v.email, v.phone, v.status)
		fmt.Printf("  ✓ vendor: %s\n", v.name)
	}
}

// ─── customers ───────────────────────────────────────────────────────────────

func seedCustomers(db *sql.DB) {
	customers := []struct{ name, email, phone, gstin string }{
		{"Rahul Sharma", "rahul.sharma@gmail.com", "9811001122", "27AAPFU0939F1ZV"},
		{"Priya Patel Enterprises", "priya@ppatel.co.in", "9822334455", "24AAACR5055K1ZA"},
		{"Sunita Trading Co.", "sunita@sunitatrading.in", "9833556677", "29AABCT1332L1ZN"},
		{"Rajan Industries", "rajan@rajanindustries.in", "9844778899", "33AAACR5055K1ZB"},
		{"Meena Retail Shop", "meena@meenaretail.in", "9855990011", ""},
		{"Global Tech Solutions", "purchase@globaltech.in", "9866112233", "09AAACG4104N1ZJ"},
		{"Anil Kumar", "anil.kumar@yahoo.com", "9877334455", ""},
		{"Laxmi Stores", "laxmi@laxmistores.in", "9888556677", "19AABCT1332L1ZM"},
	}
	for _, c := range customers {
		if exists(db, "customers", "email", c.email) {
			continue
		}
		insert(db, `INSERT INTO customers (name, email, phone, gstin) VALUES (?, ?, ?, ?)`,
			c.name, c.email, c.phone, c.gstin)
		fmt.Printf("  ✓ customer: %s\n", c.name)
	}
}

// ─── products ────────────────────────────────────────────────────────────────

func seedProducts(db *sql.DB) {
	products := []struct {
		sku, name, desc, category string
		price, cost               float64
		stock, threshold          int
		tax                       float64
	}{
		{"ELEC-001", "Laptop 15 inch", "Core i5, 8GB RAM, 512GB SSD", "Electronics", 45000, 38000, 15, 3, 18},
		{"ELEC-002", "Wireless Mouse", "USB optical mouse, 1200 DPI", "Electronics", 499, 280, 80, 10, 18},
		{"ELEC-003", "USB-C Hub 7-in-1", "HDMI, USB 3.0, SD card reader", "Electronics", 1299, 750, 45, 5, 18},
		{"ELEC-004", "Bluetooth Headphones", "Over-ear, noise cancelling", "Electronics", 2999, 1800, 30, 5, 18},
		{"ELEC-005", "Mechanical Keyboard", "TKL, Blue switches, backlit", "Electronics", 3499, 2200, 20, 3, 18},
		{"CLTH-001", "Cotton T-Shirt (M)", "100% cotton, round neck", "Clothing", 299, 140, 200, 20, 5},
		{"CLTH-002", "Denim Jeans (32)", "Slim fit, stretchable", "Clothing", 899, 500, 100, 10, 5},
		{"CLTH-003", "Formal Shirt (L)", "Poly-cotton blend, white", "Clothing", 699, 380, 150, 15, 5},
		{"FOOD-001", "Basmati Rice 5kg", "Premium long grain rice", "Food & Beverages", 385, 290, 500, 50, 0},
		{"FOOD-002", "Refined Oil 1L", "Sunflower refined oil", "Food & Beverages", 145, 110, 300, 30, 5},
		{"FOOD-003", "Whole Wheat Flour 5kg", "Stone ground atta", "Food & Beverages", 250, 190, 400, 40, 0},
		{"OFFC-001", "A4 Paper Ream 500 Sheets", "80 GSM printer paper", "Office Supplies", 320, 220, 250, 20, 12},
		{"OFFC-002", "Ball Pen Blue (Box of 10)", "Smooth writing pens", "Office Supplies", 75, 40, 500, 50, 12},
		{"OFFC-003", "Stapler Heavy Duty", "50 sheet capacity", "Office Supplies", 395, 240, 60, 5, 18},
		{"TOOL-001", "Hammer 500g", "Wooden handle, steel head", "Tools & Hardware", 280, 160, 40, 5, 18},
		{"TOOL-002", "Screwdriver Set (6pc)", "Phillips & flathead", "Tools & Hardware", 450, 270, 35, 5, 18},
		{"FURN-001", "Office Chair Ergonomic", "Adjustable height, lumbar support", "Furniture", 8500, 5800, 8, 2, 18},
		{"FURN-002", "Wooden Study Table", "120x60cm, with drawer", "Furniture", 6200, 4100, 5, 1, 18},
	}
	for _, p := range products {
		var id int
		err := db.QueryRow("SELECT id FROM products WHERE sku = ? LIMIT 1", p.sku).Scan(&id)
		if err == nil {
			continue
		}
		insert(db, `
			INSERT INTO products (name, description, sku, price, cost_price, stock, tax_rate, low_stock_threshold)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			p.name, p.desc, p.sku, p.price, p.cost, p.stock, p.tax, p.threshold)
		fmt.Printf("  ✓ product: %s (%s)\n", p.name, p.sku)
	}
}

// ─── invoices ────────────────────────────────────────────────────────────────

func seedInvoices(db *sql.DB) {
	invoices := []struct {
		number, customer, status string
		total                    float64
	}{
		{"INV-2024-001", "Rahul Sharma", "paid", 46798},
		{"INV-2024-002", "Priya Patel Enterprises", "paid", 13497},
		{"INV-2024-003", "Sunita Trading Co.", "pending", 22680},
		{"INV-2024-004", "Global Tech Solutions", "pending", 54000},
		{"INV-2024-005", "Rajan Industries", "overdue", 8850},
		{"INV-2024-006", "Meena Retail Shop", "paid", 5985},
		{"INV-2024-007", "Anil Kumar", "draft", 3598},
		{"INV-2024-008", "Laxmi Stores", "pending", 19250},
	}
	for _, inv := range invoices {
		if exists(db, "invoices", "number", inv.number) {
			continue
		}
		insert(db, `INSERT INTO invoices (number, customer, total, status) VALUES (?, ?, ?, ?)`,
			inv.number, inv.customer, inv.total, inv.status)
		fmt.Printf("  ✓ invoice: %s (%s) ₹%.0f\n", inv.number, inv.status, inv.total)
	}
}

// ─── payments ────────────────────────────────────────────────────────────────

func seedPayments(db *sql.DB) {
	payments := []struct {
		invoice, method string
		amount          float64
	}{
		{"INV-2024-001", "upi", 46798},
		{"INV-2024-002", "bank_transfer", 13497},
		{"INV-2024-006", "cash", 5985},
		{"INV-2024-003", "cheque", 10000},
	}
	for _, p := range payments {
		var count int
		db.QueryRow("SELECT COUNT(*) FROM payments WHERE invoice = ? AND amount = ?", p.invoice, p.amount).Scan(&count)
		if count > 0 {
			continue
		}
		insert(db, `INSERT INTO payments (invoice, amount, method) VALUES (?, ?, ?)`,
			p.invoice, p.amount, p.method)
		fmt.Printf("  ✓ payment: %s ₹%.0f via %s\n", p.invoice, p.amount, p.method)
	}
}

// ─── accounts ────────────────────────────────────────────────────────────────

func seedAccounts(db *sql.DB) {
	accounts := []struct {
		name, typ string
		balance   float64
	}{
		{"Cash in Hand", "asset", 25000},
		{"Bank Account - HDFC", "asset", 185000},
		{"Accounts Receivable", "asset", 104780},
		{"Inventory", "asset", 320000},
		{"Accounts Payable", "liability", 45000},
		{"GST Payable", "liability", 18500},
		{"Capital Account", "equity", 500000},
		{"Sales Revenue", "income", 170810},
		{"Purchase Expense", "expense", 98000},
		{"Rent Expense", "expense", 15000},
		{"Salary Expense", "expense", 40000},
	}
	for _, a := range accounts {
		if exists(db, "accounts", "name", a.name) {
			continue
		}
		insert(db, `INSERT INTO accounts (name, type, balance) VALUES (?, ?, ?)`, a.name, a.typ, a.balance)
		fmt.Printf("  ✓ account: %s (%s)\n", a.name, a.typ)
	}
}
