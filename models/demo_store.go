package models

import (
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// DemoStore is a per-session in-memory store for the public demo.
// All operations are isolated — the real MySQL DB is never touched.
type DemoStore struct {
	mu     sync.RWMutex
	tables map[string][]Record
	nextID map[string]int
}

func NewDemoStore() *DemoStore {
	return &DemoStore{
		tables: make(map[string][]Record),
		nextID: make(map[string]int),
	}
}

// Seed populates the store with 90 days of realistic demo data.
func (s *DemoStore) Seed() {
	s.seedCategories()
	s.seedVendors()
	s.seedCustomers()
	s.seedInvoices()
	s.seedPurchaseOrders()
	s.seedPayments()
	s.seedCreditNotes()
	s.seedAccounts()
	s.seedUsers()
	s.seedTable("jobs", []string{"name", "status", "detail"}, nil)
}

func (s *DemoStore) seedCategories() {
	s.seedTable("categories", []string{"name", "description"}, [][]string{
		{"Electronics", "Laptops, phones, accessories, cables"},
		{"Clothing & Apparel", "Garments, fashion, accessories"},
		{"Food & Beverages", "Grocery, packaged foods, drinks"},
		{"Office Supplies", "Stationery, printing, office essentials"},
		{"Tools & Hardware", "Workshop tools, fasteners, maintenance"},
		{"Furniture & Fixtures", "Office and home furniture"},
		{"Pharmaceuticals", "OTC medicines, health supplements"},
		{"Cosmetics & Personal Care", "Skincare, haircare, grooming"},
	})
}

func (s *DemoStore) seedVendors() {
	s.seedTable("vendors", []string{"name", "email", "phone", "status"}, [][]string{
		{"TechCorp India Pvt Ltd", "supply@techcorp.in", "9800012345", "active"},
		{"FashionHub Wholesalers", "orders@fashionhub.in", "9900098765", "active"},
		{"FoodMart Distributors", "bulk@foodmart.in", "9700056789", "active"},
		{"OfficeEssentials Co.", "sales@officeessentials.in", "9600034567", "active"},
		{"HardwarePro Suppliers", "info@hardwarepro.in", "9500078901", "active"},
		{"MediSupply India", "orders@medisupply.in", "9300099887", "active"},
		{"FurniWorld Pvt Ltd", "contact@furniworld.in", "9400011223", "active"},
		{"BeautyBazaar Wholesale", "bulk@beautybazaar.in", "9200088776", "active"},
	})
}

func (s *DemoStore) seedCustomers() {
	s.seedTable("customers", []string{"name", "email", "phone", "gstin"}, [][]string{
		{"Global Tech Solutions", "purchase@globaltech.in", "9866112233", "09AAACG4104N1ZJ"},
		{"Priya Patel & Co.", "priya@ppatel.co.in", "9822334455", "24AAACR5055K1ZA"},
		{"Sunita Trading Co.", "sunita@sunitatrading.in", "9833556677", "29AABCT1332L1ZN"},
		{"Rahul Sharma Enterprises", "rahul@sharmaenterprises.in", "9811001122", "27AAPRS5678F1ZV"},
		{"Rajan Industries Pvt Ltd", "rajan@rajanindustries.in", "9844778899", "33AAACR5055K1ZB"},
		{"Laxmi Stores Chain", "laxmi@laxmistores.in", "9888556677", "19AABCT1332L1ZM"},
		{"Meena Retail Group", "meena@meenaretail.in", "9855990011", ""},
		{"Anil Kumar & Brothers", "anil@akbrothers.in", "9877334455", ""},
	})
}

func (s *DemoStore) seedInvoices() {
	// 90 days of invoices with varied statuses.
	invData := [][]string{
		{"INV-2025-001", "Global Tech Solutions", "125553", "paid"},
		{"INV-2025-002", "Priya Patel & Co.", "52400", "paid"},
		{"INV-2025-003", "Sunita Trading Co.", "63700", "paid"},
		{"INV-2025-004", "Rahul Sharma Enterprises", "48000", "paid"},
		{"INV-2025-005", "Rajan Industries Pvt Ltd", "73400", "paid"},
		{"INV-2025-006", "Laxmi Stores Chain", "28560", "paid"},
		{"INV-2025-007", "Meena Retail Group", "15000", "pending"},
		{"INV-2025-008", "Anil Kumar & Brothers", "18500", "pending"},
		{"INV-2025-009", "Global Tech Solutions", "89990", "pending"},
		{"INV-2025-010", "Sunita Trading Co.", "34800", "overdue"},
		{"INV-2025-011", "Rajan Industries Pvt Ltd", "22000", "overdue"},
		{"INV-2025-012", "Priya Patel & Co.", "67200", "draft"},
	}
	now := time.Now()
	daysAgo := []int{68, 55, 42, 35, 28, 20, 14, 10, 5, 45, 38, 2}
	cols := []string{"number", "customer", "total", "status"}
	for i, row := range invData {
		d := 0
		if i < len(daysAgo) {
			d = daysAgo[i]
		}
		t := now.AddDate(0, 0, -d).Format("2006-01-02 15:04:05")
		rec := Record{"id": strconv.Itoa(i + 1), "deleted_at": "", "created_at": t}
		for j, col := range cols {
			if j < len(row) {
				rec[col] = row[j]
			}
		}
		s.tables["invoices"] = append(s.tables["invoices"], rec)
	}
	s.nextID["invoices"] = len(invData) + 1
}

func (s *DemoStore) seedPurchaseOrders() {
	// Table name matches the module config Table field and MySQL table name.
	s.seedTable("purchase_orders", []string{"number", "vendor", "total", "status"}, [][]string{
		{"PO-2025-001", "TechCorp India Pvt Ltd", "248000", "completed"},
		{"PO-2025-002", "FoodMart Distributors", "68500", "completed"},
		{"PO-2025-003", "TechCorp India Pvt Ltd", "195000", "completed"},
		{"PO-2025-004", "MediSupply India", "22500", "completed"},
		{"PO-2025-005", "FoodMart Distributors", "42800", "partially_received"},
		{"PO-2025-006", "OfficeEssentials Co.", "38500", "approved"},
		{"PO-2025-007", "FashionHub Wholesalers", "85500", "pending"},
		{"PO-2025-008", "TechCorp India Pvt Ltd", "180000", "draft"},
	})
}

func (s *DemoStore) seedPayments() {
	s.seedTable("payments", []string{"invoice", "amount", "method"}, [][]string{
		{"INV-2025-001", "125553", "bank_transfer"},
		{"INV-2025-002", "52400", "cheque"},
		{"INV-2025-003", "63700", "upi"},
		{"INV-2025-004", "48000", "bank_transfer"},
		{"INV-2025-005", "73400", "bank_transfer"},
		{"INV-2025-006", "28560", "upi"},
		{"INV-2025-007", "10000", "cash"},
	})
}

func (s *DemoStore) seedCreditNotes() {
	s.seedTable("credit_notes", []string{"number", "customer", "total", "status"}, [][]string{
		{"CN-2025-001", "Sunita Trading Co.", "5500", "issued"},
		{"CN-2025-002", "Rahul Sharma Enterprises", "2000", "issued"},
		{"CN-2025-003", "Rajan Industries Pvt Ltd", "8200", "draft"},
	})
}

func (s *DemoStore) seedAccounts() {
	s.seedTable("accounts", []string{"name", "type", "balance"}, [][]string{
		{"Cash in Hand", "asset", "48500"},
		{"HDFC Current Account", "asset", "386240"},
		{"SBI Current Account", "asset", "195800"},
		{"Accounts Receivable", "asset", "230490"},
		{"Inventory", "asset", "875000"},
		{"Accounts Payable", "liability", "198500"},
		{"GST Payable", "liability", "28450"},
		{"Capital Account", "equity", "1000000"},
		{"Sales Revenue", "income", "491613"},
		{"Purchase Expense", "expense", "276500"},
		{"Rent Expense", "expense", "126000"},
		{"Salary Expense", "expense", "249000"},
		{"Marketing Expense", "expense", "55000"},
		{"Utilities Expense", "expense", "23900"},
	})
}

func (s *DemoStore) seedUsers() {
	s.seedTable("users", []string{"name", "email", "role"}, [][]string{
		{"Anand Mehta", "admin@invobill.com", "admin"},
		{"Ramesh Kumar", "manager@invobill.com", "manager"},
		{"Sunita Yadav", "staff@invobill.com", "staff"},
		{"Kavita Sharma", "accounts@invobill.com", "accountant"},
	})
}

// ── DemoModuleService helpers (called by services layer) ─────────────────────

func (s *DemoStore) Counts(_ int) (map[string]int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := map[string]int{}
	for _, table := range []string{"customers", "invoices", "purchase_orders", "payments", "credit_notes", "users", "categories", "vendors", "jobs"} {
		cnt := 0
		for _, rec := range s.tables[table] {
			if rec["deleted_at"] == "" {
				cnt++
			}
		}
		out[table] = cnt
	}
	return out, nil
}

func (s *DemoStore) Totals(_ int) (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var invTotal, poTotal float64
	for _, rec := range s.tables["invoices"] {
		if rec["deleted_at"] == "" {
			v, _ := strconv.ParseFloat(rec["total"], 64)
			invTotal += v
		}
	}
	for _, rec := range s.tables["purchase_orders"] {
		if rec["deleted_at"] == "" {
			v, _ := strconv.ParseFloat(rec["total"], 64)
			poTotal += v
		}
	}
	return map[string]string{
		"invoice_total": fmt.Sprintf("Rs. %.2f", invTotal),
		"po_total":      fmt.Sprintf("Rs. %.2f", poTotal),
	}, nil
}

func (s *DemoStore) RecentActivity(_ int, _ int) ([]Record, error) {
	now := time.Now()
	activities := []Record{
		{"action": "create", "module": "invoices", "record_id": "9", "user_name": "Anand Mehta",
			"created_at": now.AddDate(0, 0, -1).Format("2006-01-02 15:04")},
		{"action": "delivery", "module": "crm", "record_id": "6", "user_name": "Sunita Yadav",
			"created_at": now.AddDate(0, 0, -2).Format("2006-01-02 15:04")},
		{"action": "update", "module": "procurement", "record_id": "5", "user_name": "Ramesh Kumar",
			"created_at": now.AddDate(0, 0, -3).Format("2006-01-02 15:04")},
		{"action": "create", "module": "payments", "record_id": "7", "user_name": "Kavita Sharma",
			"created_at": now.AddDate(0, 0, -3).Format("2006-01-02 15:04")},
		{"action": "stock_adjust", "module": "products", "record_id": "2", "user_name": "Ramesh Kumar",
			"created_at": now.AddDate(0, 0, -4).Format("2006-01-02 15:04")},
		{"action": "create", "module": "pos", "record_id": "148", "user_name": "Sunita Yadav",
			"created_at": now.AddDate(0, 0, -4).Format("2006-01-02 15:04")},
		{"action": "create", "module": "customers", "record_id": "8", "user_name": "Sunita Yadav",
			"created_at": now.AddDate(0, 0, -5).Format("2006-01-02 15:04")},
		{"action": "approve", "module": "expenses", "record_id": "20", "user_name": "Anand Mehta",
			"created_at": now.AddDate(0, 0, -6).Format("2006-01-02 15:04")},
	}
	return activities, nil
}

func (s *DemoStore) TopCustomers(limit, _ int) ([]Record, error) {
	top := []Record{
		{"customer": "Global Tech Solutions", "invoice_count": "2", "total_value": "215543.00"},
		{"customer": "Priya Patel & Co.", "invoice_count": "2", "total_value": "119600.00"},
		{"customer": "Rajan Industries Pvt Ltd", "invoice_count": "2", "total_value": "95400.00"},
		{"customer": "Sunita Trading Co.", "invoice_count": "2", "total_value": "98500.00"},
		{"customer": "Laxmi Stores Chain", "invoice_count": "1", "total_value": "28560.00"},
	}
	if len(top) > limit {
		top = top[:limit]
	}
	return top, nil
}

func (s *DemoStore) PendingInvoicesTotal(_ int) (float64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var total float64
	for _, rec := range s.tables["invoices"] {
		if rec["deleted_at"] == "" && rec["status"] != "paid" && rec["status"] != "draft" {
			v, _ := strconv.ParseFloat(rec["total"], 64)
			total += v
		}
	}
	return total, nil
}

// SalesTrend returns daily revenue for the last N days (for sparkline charts).
func (s *DemoStore) SalesTrend(days int) []DailySales {
	rng := rand.New(rand.NewSource(42)) // deterministic for demo
	base := 35000.0
	out := make([]DailySales, days)
	for i := days - 1; i >= 0; i-- {
		day := time.Now().AddDate(0, 0, -i)
		weekday := day.Weekday()
		multiplier := 1.0
		if weekday == time.Saturday || weekday == time.Sunday {
			multiplier = 1.4
		}
		revenue := base*multiplier + (rng.Float64()-0.5)*15000
		if revenue < 5000 {
			revenue = 5000
		}
		out[days-1-i] = DailySales{
			Date:    day.Format("Jan 02"),
			Revenue: revenue,
		}
	}
	return out
}

type DailySales struct {
	Date    string
	Revenue float64
}

// ── IModuleStore implementation ───────────────────────────────────────────────

func (s *DemoStore) seedTable(table string, columns []string, rows [][]string) {
	now := time.Now().Format("2006-01-02 15:04:05")
	for i, row := range rows {
		rec := Record{"id": strconv.Itoa(i + 1), "deleted_at": "", "created_at": now}
		for j, col := range columns {
			if j < len(row) {
				rec[col] = row[j]
			}
		}
		s.tables[table] = append(s.tables[table], rec)
	}
	s.nextID[table] = len(rows) + 1
}

func (s *DemoStore) List(table string, columns []string, _ int) ([]Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []Record
	for _, rec := range s.tables[table] {
		if rec["deleted_at"] == "" {
			result = append(result, s.project(rec, columns))
		}
	}
	demoReverse(result)
	return result, nil
}

func (s *DemoStore) ListPaged(table string, columns []string, page, perPage int, search, sortCol, sortDir string, _ int) (PageResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 200 {
		perPage = 25
	}
	var filtered []Record
	for _, rec := range s.tables[table] {
		if rec["deleted_at"] != "" {
			continue
		}
		if search != "" {
			matched := false
			for _, col := range columns {
				if strings.Contains(strings.ToLower(rec[col]), strings.ToLower(search)) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		filtered = append(filtered, s.project(rec, columns))
	}
	if sortCol != "" {
		sort.SliceStable(filtered, func(i, j int) bool {
			if sortDir == "asc" {
				return filtered[i][sortCol] < filtered[j][sortCol]
			}
			return filtered[i][sortCol] > filtered[j][sortCol]
		})
	} else {
		demoReverse(filtered)
	}
	total := len(filtered)
	lastPage := (total + perPage - 1) / perPage
	if lastPage == 0 {
		lastPage = 1
	}
	start := (page - 1) * perPage
	end := start + perPage
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}
	return PageResult{Records: filtered[start:end], Total: total, Page: page, PerPage: perPage, LastPage: lastPage}, nil
}

func (s *DemoStore) Trash(table string, columns []string, _ int) ([]Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []Record
	for _, rec := range s.tables[table] {
		if rec["deleted_at"] != "" {
			result = append(result, s.project(rec, columns))
		}
	}
	return result, nil
}

func (s *DemoStore) Get(table string, columns []string, id, _ int) (Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sid := strconv.Itoa(id)
	for _, rec := range s.tables[table] {
		if rec["id"] == sid && rec["deleted_at"] == "" {
			return s.project(rec, columns), nil
		}
	}
	return nil, fmt.Errorf("record not found")
}

func (s *DemoStore) Create(table string, columns []string, values []string, _ int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := s.nextID[table]
	s.nextID[table]++
	rec := Record{
		"id":         strconv.Itoa(id),
		"deleted_at": "",
		"created_at": time.Now().Format("2006-01-02 15:04:05"),
	}
	for i, col := range columns {
		if i < len(values) {
			rec[col] = values[i]
		}
	}
	s.tables[table] = append(s.tables[table], rec)
	return nil
}

func (s *DemoStore) Update(table string, columns []string, values []string, id, _ int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sid := strconv.Itoa(id)
	for i, rec := range s.tables[table] {
		if rec["id"] == sid && rec["deleted_at"] == "" {
			for j, col := range columns {
				if j < len(values) {
					s.tables[table][i][col] = values[j]
				}
			}
			return nil
		}
	}
	return fmt.Errorf("record not found")
}

func (s *DemoStore) Delete(table string, id, _ int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sid := strconv.Itoa(id)
	for i, rec := range s.tables[table] {
		if rec["id"] == sid && rec["deleted_at"] == "" {
			s.tables[table][i]["deleted_at"] = time.Now().Format("2006-01-02 15:04:05")
			return nil
		}
	}
	return nil
}

func (s *DemoStore) HardDelete(table string, id, _ int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sid := strconv.Itoa(id)
	rows := s.tables[table][:0]
	for _, rec := range s.tables[table] {
		if rec["id"] != sid {
			rows = append(rows, rec)
		}
	}
	s.tables[table] = rows
	return nil
}

func (s *DemoStore) Restore(table string, id, _ int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sid := strconv.Itoa(id)
	for i, rec := range s.tables[table] {
		if rec["id"] == sid {
			s.tables[table][i]["deleted_at"] = ""
			return nil
		}
	}
	return nil
}

func (s *DemoStore) Count(table string, _ int) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	count := 0
	for _, rec := range s.tables[table] {
		if rec["deleted_at"] == "" {
			count++
		}
	}
	return count, nil
}

func (s *DemoStore) Sum(table, column string, _ int) (float64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var total float64
	for _, rec := range s.tables[table] {
		if rec["deleted_at"] == "" {
			v, _ := strconv.ParseFloat(rec[column], 64)
			total += v
		}
	}
	return total, nil
}

func (s *DemoStore) StockLogs(_ int) ([]Record, error)       { return nil, nil }

func (s *DemoStore) FindByField(table, field, value string, columns []string, _ int) (Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, rec := range s.tables[table] {
		if rec["deleted_at"] == "" && rec[field] == value {
			return s.project(rec, columns), nil
		}
	}
	return nil, fmt.Errorf("record not found")
}

func (s *DemoStore) project(rec Record, columns []string) Record {
	out := Record{"id": rec["id"], "created_at": rec["created_at"]}
	for _, col := range columns {
		out[col] = rec[col]
	}
	return out
}

func demoReverse(recs []Record) {
	for i, j := 0, len(recs)-1; i < j; i, j = i+1, j-1 {
		recs[i], recs[j] = recs[j], recs[i]
	}
}
