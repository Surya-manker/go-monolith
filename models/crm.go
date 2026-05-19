package models

import (
	"database/sql"
	"fmt"
	"time"
)

// ── Domain types ──────────────────────────────────────────────────────────────

type CRMCustomer struct {
	ID              int
	BusinessID      int
	CustomerCode    string
	Name            string
	Email           string
	Phone           string
	GSTIN           string
	PAN             string
	BillingAddress  string
	ShippingAddress string
	ContactPerson   string
	CustomerGroup   string
	CreditLimit     float64
	PaymentTerms    int // days
	Status          string // active|inactive|blocked
	Notes           string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	// Computed
	TotalOrders     int
	TotalRevenue    float64
	TotalPaid       float64
	OutstandingDue  float64
}

type Quotation struct {
	ID             int
	BusinessID     int
	CustomerID     int
	CustomerName   string
	QuoteNumber    string
	Status         string // draft|sent|approved|rejected|converted|expired
	WarehouseID    int
	WarehouseName  string
	ValidUntil     *time.Time
	Notes          string
	Subtotal       float64
	TaxTotal       float64
	Discount       float64
	GrandTotal     float64
	ConvertedToID  *int // sales_order.id if converted
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Items          []QuotationItem
}

type QuotationItem struct {
	ID          int
	QuotationID int
	ProductID   int
	ProductName string
	SKU         string
	Quantity    int
	UnitPrice   float64
	TaxRate     float64
	TaxAmount   float64
	Discount    float64
	LineTotal   float64
}

type SalesOrder struct {
	ID              int
	BusinessID      int
	CustomerID      int
	CustomerName    string
	OrderNumber     string
	QuotationID     *int
	Status          string // draft|confirmed|packed|dispatched|delivered|completed|cancelled
	WarehouseID     int
	WarehouseName   string
	ShippingAddress string
	DeliveryDate    *time.Time
	Notes           string
	Subtotal        float64
	TaxTotal        float64
	Discount        float64
	GrandTotal      float64
	CreatedAt       time.Time
	UpdatedAt       time.Time
	Items           []SalesOrderItem
	TotalPaid       float64
	OutstandingDue  float64
}

type SalesOrderItem struct {
	ID           int
	OrderID      int
	ProductID    int
	ProductName  string
	SKU          string
	Quantity     int
	ReservedQty  int
	DeliveredQty int
	PendingQty   int // Quantity - DeliveredQty
	UnitPrice    float64
	TaxRate      float64
	TaxAmount    float64
	Discount     float64
	LineTotal    float64
}

type StockReservation struct {
	ID          int
	BusinessID  int
	WarehouseID int
	ProductID   int
	OrderID     int
	OrderItemID int
	ReservedQty int
	Status      string // active|fulfilled|released|expired
	ExpiresAt   *time.Time
	CreatedAt   time.Time
}

type DeliveryChallan struct {
	ID            int
	BusinessID    int
	OrderID       int
	OrderNumber   string
	CustomerID    int
	CustomerName  string
	ChallanNumber string
	WarehouseID   int
	WarehouseName string
	Status        string // draft|dispatched|delivered
	CourierName   string
	TrackingNumber string
	DispatchDate  *time.Time
	DeliveryDate  *time.Time
	Notes         string
	CreatedAt     time.Time
	Items         []DeliveryChallanItem
}

type DeliveryChallanItem struct {
	ID          int
	ChallanID   int
	OrderItemID int
	ProductID   int
	ProductName string
	SKU         string
	Quantity    int
	BatchID     *int
	BatchNumber string
}

type CustomerPayment struct {
	ID            int
	BusinessID    int
	CustomerID    int
	CustomerName  string
	OrderID       int
	OrderNumber   string
	PaymentNumber string
	Amount        float64
	PaymentMethod string
	PaymentType   string // advance|regular|refund
	Reference     string
	Notes         string
	CreatedAt     time.Time
}

// CRMStats for the dashboard.
type CRMStats struct {
	TotalCustomers    int
	PendingQuotes     int
	ActiveOrders      int
	PendingDeliveries int
	TotalOutstanding  float64
	MonthRevenue      float64
}

// ── Store ─────────────────────────────────────────────────────────────────────

type CRMStore struct {
	db *sql.DB
}

func NewCRMStore(db *sql.DB) *CRMStore {
	return &CRMStore{db: db}
}

func (s *CRMStore) Migrate() error {
	tables := []string{
		`CREATE TABLE IF NOT EXISTS crm_customers (
			id               INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			business_id      INT NOT NULL DEFAULT 0,
			customer_code    VARCHAR(50) NOT NULL DEFAULT '',
			name             VARCHAR(255) NOT NULL,
			email            VARCHAR(255) NOT NULL DEFAULT '',
			phone            VARCHAR(50) NOT NULL DEFAULT '',
			gstin            VARCHAR(20) NOT NULL DEFAULT '',
			pan              VARCHAR(20) NOT NULL DEFAULT '',
			billing_address  TEXT NOT NULL DEFAULT '',
			shipping_address TEXT NOT NULL DEFAULT '',
			contact_person   VARCHAR(255) NOT NULL DEFAULT '',
			customer_group   VARCHAR(100) NOT NULL DEFAULT '',
			credit_limit     DECIMAL(12,2) NOT NULL DEFAULT 0,
			payment_terms    INT NOT NULL DEFAULT 30,
			status           VARCHAR(20) NOT NULL DEFAULT 'active',
			notes            TEXT NOT NULL DEFAULT '',
			created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS quotations (
			id             INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			business_id    INT NOT NULL DEFAULT 0,
			customer_id    INT NOT NULL DEFAULT 0,
			customer_name  VARCHAR(255) NOT NULL DEFAULT '',
			quote_number   VARCHAR(50) NOT NULL DEFAULT '',
			status         VARCHAR(20) NOT NULL DEFAULT 'draft',
			warehouse_id   INT NOT NULL DEFAULT 0,
			valid_until    DATE NULL,
			notes          TEXT NOT NULL DEFAULT '',
			subtotal       DECIMAL(12,2) NOT NULL DEFAULT 0,
			tax_total      DECIMAL(12,2) NOT NULL DEFAULT 0,
			discount       DECIMAL(12,2) NOT NULL DEFAULT 0,
			grand_total    DECIMAL(12,2) NOT NULL DEFAULT 0,
			converted_to_id INT NULL,
			created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS quotation_items (
			id           INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			quotation_id INT NOT NULL,
			product_id   INT NOT NULL DEFAULT 0,
			product_name VARCHAR(255) NOT NULL DEFAULT '',
			sku          VARCHAR(255) NOT NULL DEFAULT '',
			quantity     INT NOT NULL DEFAULT 1,
			unit_price   DECIMAL(12,2) NOT NULL DEFAULT 0,
			tax_rate     DECIMAL(5,2) NOT NULL DEFAULT 0,
			tax_amount   DECIMAL(12,2) NOT NULL DEFAULT 0,
			discount     DECIMAL(12,2) NOT NULL DEFAULT 0,
			line_total   DECIMAL(12,2) NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS sales_orders (
			id               INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			business_id      INT NOT NULL DEFAULT 0,
			customer_id      INT NOT NULL DEFAULT 0,
			customer_name    VARCHAR(255) NOT NULL DEFAULT '',
			order_number     VARCHAR(50) NOT NULL DEFAULT '',
			quotation_id     INT NULL,
			status           VARCHAR(20) NOT NULL DEFAULT 'draft',
			warehouse_id     INT NOT NULL DEFAULT 0,
			shipping_address TEXT NOT NULL DEFAULT '',
			delivery_date    DATE NULL,
			notes            TEXT NOT NULL DEFAULT '',
			subtotal         DECIMAL(12,2) NOT NULL DEFAULT 0,
			tax_total        DECIMAL(12,2) NOT NULL DEFAULT 0,
			discount         DECIMAL(12,2) NOT NULL DEFAULT 0,
			grand_total      DECIMAL(12,2) NOT NULL DEFAULT 0,
			created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS sales_order_items (
			id            INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			order_id      INT NOT NULL,
			product_id    INT NOT NULL DEFAULT 0,
			product_name  VARCHAR(255) NOT NULL DEFAULT '',
			sku           VARCHAR(255) NOT NULL DEFAULT '',
			quantity      INT NOT NULL DEFAULT 1,
			reserved_qty  INT NOT NULL DEFAULT 0,
			delivered_qty INT NOT NULL DEFAULT 0,
			unit_price    DECIMAL(12,2) NOT NULL DEFAULT 0,
			tax_rate      DECIMAL(5,2) NOT NULL DEFAULT 0,
			tax_amount    DECIMAL(12,2) NOT NULL DEFAULT 0,
			discount      DECIMAL(12,2) NOT NULL DEFAULT 0,
			line_total    DECIMAL(12,2) NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS stock_reservations (
			id           INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			business_id  INT NOT NULL DEFAULT 0,
			warehouse_id INT NOT NULL DEFAULT 0,
			product_id   INT NOT NULL DEFAULT 0,
			order_id     INT NOT NULL DEFAULT 0,
			order_item_id INT NOT NULL DEFAULT 0,
			reserved_qty INT NOT NULL DEFAULT 0,
			status       VARCHAR(20) NOT NULL DEFAULT 'active',
			expires_at   DATETIME NULL,
			created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS delivery_challans (
			id              INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			business_id     INT NOT NULL DEFAULT 0,
			order_id        INT NOT NULL DEFAULT 0,
			customer_id     INT NOT NULL DEFAULT 0,
			customer_name   VARCHAR(255) NOT NULL DEFAULT '',
			challan_number  VARCHAR(50) NOT NULL DEFAULT '',
			warehouse_id    INT NOT NULL DEFAULT 0,
			status          VARCHAR(20) NOT NULL DEFAULT 'draft',
			courier_name    VARCHAR(255) NOT NULL DEFAULT '',
			tracking_number VARCHAR(255) NOT NULL DEFAULT '',
			dispatch_date   DATE NULL,
			delivery_date   DATE NULL,
			notes           TEXT NOT NULL DEFAULT '',
			created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS delivery_challan_items (
			id             INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			challan_id     INT NOT NULL,
			order_item_id  INT NOT NULL DEFAULT 0,
			product_id     INT NOT NULL DEFAULT 0,
			product_name   VARCHAR(255) NOT NULL DEFAULT '',
			sku            VARCHAR(255) NOT NULL DEFAULT '',
			quantity       INT NOT NULL DEFAULT 1,
			batch_id       INT NULL,
			batch_number   VARCHAR(100) NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS customer_payments (
			id             INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			business_id    INT NOT NULL DEFAULT 0,
			customer_id    INT NOT NULL DEFAULT 0,
			customer_name  VARCHAR(255) NOT NULL DEFAULT '',
			order_id       INT NOT NULL DEFAULT 0,
			order_number   VARCHAR(50) NOT NULL DEFAULT '',
			payment_number VARCHAR(50) NOT NULL DEFAULT '',
			amount         DECIMAL(12,2) NOT NULL DEFAULT 0,
			payment_method VARCHAR(20) NOT NULL DEFAULT 'cash',
			payment_type   VARCHAR(20) NOT NULL DEFAULT 'regular',
			reference      VARCHAR(255) NOT NULL DEFAULT '',
			notes          TEXT NOT NULL DEFAULT '',
			created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
	}
	for _, q := range tables {
		if _, err := s.db.Exec(q); err != nil {
			return err
		}
	}
	for _, idx := range []string{
		`CREATE INDEX idx_crm_customers_biz   ON crm_customers(business_id)`,
		`CREATE INDEX idx_quotations_biz      ON quotations(business_id)`,
		`CREATE INDEX idx_quotations_customer ON quotations(customer_id)`,
		`CREATE INDEX idx_sales_orders_biz    ON sales_orders(business_id)`,
		`CREATE INDEX idx_sales_orders_cust   ON sales_orders(customer_id)`,
		`CREATE INDEX idx_sales_orders_status ON sales_orders(business_id, status)`,
		`CREATE INDEX idx_sales_items_order   ON sales_order_items(order_id)`,
		`CREATE INDEX idx_reservations_prod   ON stock_reservations(product_id, warehouse_id, business_id)`,
		`CREATE INDEX idx_reservations_order  ON stock_reservations(order_id)`,
		`CREATE INDEX idx_challans_biz        ON delivery_challans(business_id)`,
		`CREATE INDEX idx_challans_order      ON delivery_challans(order_id)`,
		`CREATE INDEX idx_cust_payments_biz   ON customer_payments(business_id)`,
		`CREATE INDEX idx_cust_payments_cust  ON customer_payments(customer_id)`,
	} {
		_, _ = s.db.Exec(idx)
	}
	return nil
}

// ── CRM Customers ─────────────────────────────────────────────────────────────

func (s *CRMStore) ListCustomers(bizID int) ([]CRMCustomer, error) {
	rows, err := s.db.Query(`
		SELECT c.id, c.business_id, c.customer_code, c.name, c.email, c.phone,
		       c.gstin, c.pan, c.billing_address, c.shipping_address,
		       c.contact_person, c.customer_group, c.credit_limit, c.payment_terms,
		       c.status, c.notes, c.created_at, c.updated_at,
		       COALESCE((SELECT COUNT(*) FROM sales_orders o WHERE o.customer_id=c.id AND o.status!='cancelled'),0),
		       COALESCE((SELECT SUM(o.grand_total) FROM sales_orders o WHERE o.customer_id=c.id AND o.status='completed'),0),
		       COALESCE((SELECT SUM(p.amount) FROM customer_payments p WHERE p.customer_id=c.id),0)
		FROM crm_customers c WHERE c.business_id=? ORDER BY c.name ASC`, bizID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CRMCustomer
	for rows.Next() {
		c, err := scanCustomer(rows)
		if err != nil {
			return nil, err
		}
		c.OutstandingDue = c.TotalRevenue - c.TotalPaid
		out = append(out, *c)
	}
	return out, rows.Err()
}

func (s *CRMStore) GetCustomer(id, bizID int) (*CRMCustomer, error) {
	row := s.db.QueryRow(`
		SELECT c.id, c.business_id, c.customer_code, c.name, c.email, c.phone,
		       c.gstin, c.pan, c.billing_address, c.shipping_address,
		       c.contact_person, c.customer_group, c.credit_limit, c.payment_terms,
		       c.status, c.notes, c.created_at, c.updated_at,
		       COALESCE((SELECT COUNT(*) FROM sales_orders o WHERE o.customer_id=c.id AND o.status!='cancelled'),0),
		       COALESCE((SELECT SUM(o.grand_total) FROM sales_orders o WHERE o.customer_id=c.id AND o.status='completed'),0),
		       COALESCE((SELECT SUM(p.amount) FROM customer_payments p WHERE p.customer_id=c.id),0)
		FROM crm_customers c WHERE c.id=? AND c.business_id=?`, id, bizID)
	c, err := scanCustomer(row)
	if err != nil {
		return nil, err
	}
	c.OutstandingDue = c.TotalRevenue - c.TotalPaid
	return c, nil
}

func scanCustomer(row interface{ Scan(...any) error }) (*CRMCustomer, error) {
	var c CRMCustomer
	err := row.Scan(
		&c.ID, &c.BusinessID, &c.CustomerCode, &c.Name, &c.Email, &c.Phone,
		&c.GSTIN, &c.PAN, &c.BillingAddress, &c.ShippingAddress,
		&c.ContactPerson, &c.CustomerGroup, &c.CreditLimit, &c.PaymentTerms,
		&c.Status, &c.Notes, &c.CreatedAt, &c.UpdatedAt,
		&c.TotalOrders, &c.TotalRevenue, &c.TotalPaid,
	)
	return &c, err
}

func (s *CRMStore) CreateCustomer(c *CRMCustomer) (*CRMCustomer, error) {
	if c.CustomerCode == "" {
		var count int
		_ = s.db.QueryRow(`SELECT COUNT(*) FROM crm_customers WHERE business_id=?`, c.BusinessID).Scan(&count)
		c.CustomerCode = fmt.Sprintf("CUST-%04d", count+1)
	}
	res, err := s.db.Exec(`
		INSERT INTO crm_customers (business_id, customer_code, name, email, phone, gstin, pan,
		billing_address, shipping_address, contact_person, customer_group,
		credit_limit, payment_terms, status, notes)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		c.BusinessID, c.CustomerCode, c.Name, c.Email, c.Phone, c.GSTIN, c.PAN,
		c.BillingAddress, c.ShippingAddress, c.ContactPerson, c.CustomerGroup,
		c.CreditLimit, c.PaymentTerms, c.Status, c.Notes,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetCustomer(int(id), c.BusinessID)
}

func (s *CRMStore) UpdateCustomer(c *CRMCustomer) error {
	_, err := s.db.Exec(`
		UPDATE crm_customers SET name=?, email=?, phone=?, gstin=?, pan=?,
		billing_address=?, shipping_address=?, contact_person=?, customer_group=?,
		credit_limit=?, payment_terms=?, status=?, notes=?, updated_at=CURRENT_TIMESTAMP
		WHERE id=? AND business_id=?`,
		c.Name, c.Email, c.Phone, c.GSTIN, c.PAN,
		c.BillingAddress, c.ShippingAddress, c.ContactPerson, c.CustomerGroup,
		c.CreditLimit, c.PaymentTerms, c.Status, c.Notes, c.ID, c.BusinessID,
	)
	return err
}

func (s *CRMStore) CountCustomers(bizID int) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM crm_customers WHERE business_id=?`, bizID).Scan(&n)
	return n, err
}

// ── Quotations ────────────────────────────────────────────────────────────────

func (s *CRMStore) ListQuotations(bizID int, status string) ([]Quotation, error) {
	q := `SELECT o.id, o.business_id, o.customer_id, o.customer_name, o.quote_number,
		       o.status, o.warehouse_id, COALESCE(w.name,''),
		       o.valid_until, o.notes, o.subtotal, o.tax_total, o.discount, o.grand_total,
		       o.converted_to_id, o.created_at, o.updated_at
		FROM quotations o
		LEFT JOIN warehouses w ON w.id=o.warehouse_id AND w.business_id=o.business_id
		WHERE o.business_id=?`
	args := []any{bizID}
	if status != "" {
		q += ` AND o.status=?`
		args = append(args, status)
	}
	q += ` ORDER BY o.created_at DESC`

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Quotation
	for rows.Next() {
		var qt Quotation
		if err = scanQuotation(rows, &qt); err != nil {
			return nil, err
		}
		out = append(out, qt)
	}
	return out, rows.Err()
}

func (s *CRMStore) GetQuotation(id, bizID int) (*Quotation, error) {
	var qt Quotation
	err := s.db.QueryRow(`SELECT o.id, o.business_id, o.customer_id, o.customer_name, o.quote_number,
		       o.status, o.warehouse_id, COALESCE(w.name,''),
		       o.valid_until, o.notes, o.subtotal, o.tax_total, o.discount, o.grand_total,
		       o.converted_to_id, o.created_at, o.updated_at
		FROM quotations o
		LEFT JOIN warehouses w ON w.id=o.warehouse_id AND w.business_id=o.business_id
		WHERE o.id=? AND o.business_id=?`, id, bizID,
	).Scan(&qt.ID, &qt.BusinessID, &qt.CustomerID, &qt.CustomerName, &qt.QuoteNumber,
		&qt.Status, &qt.WarehouseID, &qt.WarehouseName,
		&qt.ValidUntil, &qt.Notes, &qt.Subtotal, &qt.TaxTotal, &qt.Discount, &qt.GrandTotal,
		&qt.ConvertedToID, &qt.CreatedAt, &qt.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	qt.Items, err = s.getQuotationItems(qt.ID)
	return &qt, err
}

func scanQuotation(row interface{ Scan(...any) error }, qt *Quotation) error {
	return row.Scan(&qt.ID, &qt.BusinessID, &qt.CustomerID, &qt.CustomerName, &qt.QuoteNumber,
		&qt.Status, &qt.WarehouseID, &qt.WarehouseName,
		&qt.ValidUntil, &qt.Notes, &qt.Subtotal, &qt.TaxTotal, &qt.Discount, &qt.GrandTotal,
		&qt.ConvertedToID, &qt.CreatedAt, &qt.UpdatedAt,
	)
}

func (s *CRMStore) getQuotationItems(qID int) ([]QuotationItem, error) {
	rows, err := s.db.Query(`SELECT id, quotation_id, product_id, product_name, sku,
		quantity, unit_price, tax_rate, tax_amount, discount, line_total
		FROM quotation_items WHERE quotation_id=? ORDER BY id`, qID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []QuotationItem
	for rows.Next() {
		var it QuotationItem
		if err = rows.Scan(&it.ID, &it.QuotationID, &it.ProductID, &it.ProductName, &it.SKU,
			&it.Quantity, &it.UnitPrice, &it.TaxRate, &it.TaxAmount, &it.Discount, &it.LineTotal); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (s *CRMStore) CreateQuotation(qt *Quotation) (*Quotation, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	res, err := tx.Exec(`INSERT INTO quotations (business_id, customer_id, customer_name, quote_number,
		status, warehouse_id, valid_until, notes, subtotal, tax_total, discount, grand_total)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		qt.BusinessID, qt.CustomerID, qt.CustomerName, qt.QuoteNumber,
		"draft", qt.WarehouseID, qt.ValidUntil, qt.Notes,
		qt.Subtotal, qt.TaxTotal, qt.Discount, qt.GrandTotal,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()

	for _, it := range qt.Items {
		if _, err = tx.Exec(`INSERT INTO quotation_items (quotation_id, product_id, product_name, sku,
			quantity, unit_price, tax_rate, tax_amount, discount, line_total)
			VALUES (?,?,?,?,?,?,?,?,?,?)`,
			id, it.ProductID, it.ProductName, it.SKU,
			it.Quantity, it.UnitPrice, it.TaxRate, it.TaxAmount, it.Discount, it.LineTotal,
		); err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetQuotation(int(id), qt.BusinessID)
}

func (s *CRMStore) UpdateQuotationStatus(id int, status string) error {
	_, err := s.db.Exec(`UPDATE quotations SET status=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`, status, id)
	return err
}

func (s *CRMStore) NextQuoteNumber(bizID int) string {
	var count int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM quotations WHERE business_id=?`, bizID).Scan(&count)
	return fmt.Sprintf("QT-%s-%04d", time.Now().Format("200601"), count+1)
}

// ── Sales Orders ──────────────────────────────────────────────────────────────

func (s *CRMStore) ListOrders(bizID int, status string) ([]SalesOrder, error) {
	q := `SELECT o.id, o.business_id, o.customer_id, o.customer_name, o.order_number,
		       o.quotation_id, o.status, o.warehouse_id, COALESCE(w.name,''),
		       o.shipping_address, o.delivery_date, o.notes,
		       o.subtotal, o.tax_total, o.discount, o.grand_total,
		       o.created_at, o.updated_at,
		       COALESCE((SELECT SUM(p.amount) FROM customer_payments p WHERE p.order_id=o.id),0)
		FROM sales_orders o
		LEFT JOIN warehouses w ON w.id=o.warehouse_id AND w.business_id=o.business_id
		WHERE o.business_id=?`
	args := []any{bizID}
	if status != "" {
		q += ` AND o.status=?`
		args = append(args, status)
	}
	q += ` ORDER BY o.created_at DESC`

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SalesOrder
	for rows.Next() {
		var o SalesOrder
		if err = scanOrder2(rows, &o); err != nil {
			return nil, err
		}
		o.OutstandingDue = o.GrandTotal - o.TotalPaid
		out = append(out, o)
	}
	return out, rows.Err()
}

func (s *CRMStore) GetOrder(id, bizID int) (*SalesOrder, error) {
	var o SalesOrder
	err := s.db.QueryRow(`SELECT o.id, o.business_id, o.customer_id, o.customer_name, o.order_number,
		       o.quotation_id, o.status, o.warehouse_id, COALESCE(w.name,''),
		       o.shipping_address, o.delivery_date, o.notes,
		       o.subtotal, o.tax_total, o.discount, o.grand_total,
		       o.created_at, o.updated_at,
		       COALESCE((SELECT SUM(p.amount) FROM customer_payments p WHERE p.order_id=o.id),0)
		FROM sales_orders o
		LEFT JOIN warehouses w ON w.id=o.warehouse_id AND w.business_id=o.business_id
		WHERE o.id=? AND o.business_id=?`, id, bizID,
	).Scan(&o.ID, &o.BusinessID, &o.CustomerID, &o.CustomerName, &o.OrderNumber,
		&o.QuotationID, &o.Status, &o.WarehouseID, &o.WarehouseName,
		&o.ShippingAddress, &o.DeliveryDate, &o.Notes,
		&o.Subtotal, &o.TaxTotal, &o.Discount, &o.GrandTotal,
		&o.CreatedAt, &o.UpdatedAt, &o.TotalPaid,
	)
	if err != nil {
		return nil, err
	}
	o.OutstandingDue = o.GrandTotal - o.TotalPaid
	o.Items, err = s.getOrderItems(o.ID)
	return &o, err
}

func scanOrder2(row interface{ Scan(...any) error }, o *SalesOrder) error {
	return row.Scan(&o.ID, &o.BusinessID, &o.CustomerID, &o.CustomerName, &o.OrderNumber,
		&o.QuotationID, &o.Status, &o.WarehouseID, &o.WarehouseName,
		&o.ShippingAddress, &o.DeliveryDate, &o.Notes,
		&o.Subtotal, &o.TaxTotal, &o.Discount, &o.GrandTotal,
		&o.CreatedAt, &o.UpdatedAt, &o.TotalPaid,
	)
}

func (s *CRMStore) getOrderItems(oID int) ([]SalesOrderItem, error) {
	rows, err := s.db.Query(`SELECT id, order_id, product_id, product_name, sku,
		quantity, reserved_qty, delivered_qty, unit_price, tax_rate, tax_amount, discount, line_total
		FROM sales_order_items WHERE order_id=? ORDER BY id`, oID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SalesOrderItem
	for rows.Next() {
		var it SalesOrderItem
		if err = rows.Scan(&it.ID, &it.OrderID, &it.ProductID, &it.ProductName, &it.SKU,
			&it.Quantity, &it.ReservedQty, &it.DeliveredQty,
			&it.UnitPrice, &it.TaxRate, &it.TaxAmount, &it.Discount, &it.LineTotal); err != nil {
			return nil, err
		}
		it.PendingQty = it.Quantity - it.DeliveredQty
		out = append(out, it)
	}
	return out, rows.Err()
}

func (s *CRMStore) CreateOrder(o *SalesOrder) (*SalesOrder, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	res, err := tx.Exec(`INSERT INTO sales_orders (business_id, customer_id, customer_name,
		order_number, quotation_id, status, warehouse_id, shipping_address, delivery_date, notes,
		subtotal, tax_total, discount, grand_total)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		o.BusinessID, o.CustomerID, o.CustomerName, o.OrderNumber, o.QuotationID,
		"draft", o.WarehouseID, o.ShippingAddress, o.DeliveryDate, o.Notes,
		o.Subtotal, o.TaxTotal, o.Discount, o.GrandTotal,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()

	for _, it := range o.Items {
		if _, err = tx.Exec(`INSERT INTO sales_order_items (order_id, product_id, product_name, sku,
			quantity, unit_price, tax_rate, tax_amount, discount, line_total)
			VALUES (?,?,?,?,?,?,?,?,?,?)`,
			id, it.ProductID, it.ProductName, it.SKU,
			it.Quantity, it.UnitPrice, it.TaxRate, it.TaxAmount, it.Discount, it.LineTotal,
		); err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetOrder(int(id), o.BusinessID)
}

func (s *CRMStore) UpdateOrderStatus(tx *sql.Tx, id int, status string) error {
	_, err := tx.Exec(`UPDATE sales_orders SET status=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`, status, id)
	return err
}

func (s *CRMStore) UpdateOrderStatusDirect(id int, status string) error {
	_, err := s.db.Exec(`UPDATE sales_orders SET status=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`, status, id)
	return err
}

func (s *CRMStore) NextOrderNumber(bizID int) string {
	var count int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM sales_orders WHERE business_id=?`, bizID).Scan(&count)
	return fmt.Sprintf("SO-%s-%04d", time.Now().Format("200601"), count+1)
}

// ── Stock Reservations ────────────────────────────────────────────────────────

func (s *CRMStore) CreateReservationsTx(tx *sql.Tx, reservations []StockReservation) error {
	for _, r := range reservations {
		if _, err := tx.Exec(`INSERT INTO stock_reservations (business_id, warehouse_id, product_id,
			order_id, order_item_id, reserved_qty, status, expires_at)
			VALUES (?,?,?,?,?,?,'active',?)`,
			r.BusinessID, r.WarehouseID, r.ProductID,
			r.OrderID, r.OrderItemID, r.ReservedQty, r.ExpiresAt,
		); err != nil {
			return err
		}
	}
	return nil
}

func (s *CRMStore) ReleaseReservationsTx(tx *sql.Tx, orderID int) error {
	_, err := tx.Exec(`UPDATE stock_reservations SET status='released' WHERE order_id=? AND status='active'`, orderID)
	return err
}

func (s *CRMStore) FulfillReservationsTx(tx *sql.Tx, orderID int) error {
	_, err := tx.Exec(`UPDATE stock_reservations SET status='fulfilled' WHERE order_id=? AND status='active'`, orderID)
	return err
}

func (s *CRMStore) GetReservedQty(productID, warehouseID, bizID int) (int, error) {
	var qty int
	err := s.db.QueryRow(`SELECT COALESCE(SUM(reserved_qty),0) FROM stock_reservations
		WHERE product_id=? AND warehouse_id=? AND business_id=? AND status='active'`,
		productID, warehouseID, bizID,
	).Scan(&qty)
	return qty, err
}

func (s *CRMStore) UpdateOrderItemReserved(tx *sql.Tx, orderItemID, reservedQty int) error {
	_, err := tx.Exec(`UPDATE sales_order_items SET reserved_qty=? WHERE id=?`, reservedQty, orderItemID)
	return err
}

func (s *CRMStore) UpdateOrderItemDelivered(tx *sql.Tx, orderItemID, additionalQty int) error {
	_, err := tx.Exec(`UPDATE sales_order_items SET delivered_qty=delivered_qty+? WHERE id=?`, additionalQty, orderItemID)
	return err
}

// ── Delivery Challans ─────────────────────────────────────────────────────────

func (s *CRMStore) ListChallans(bizID int) ([]DeliveryChallan, error) {
	rows, err := s.db.Query(`
		SELECT c.id, c.business_id, c.order_id, COALESCE(o.order_number,''),
		       c.customer_id, c.customer_name, c.challan_number,
		       c.warehouse_id, COALESCE(w.name,''), c.status,
		       c.courier_name, c.tracking_number,
		       c.dispatch_date, c.delivery_date, c.notes, c.created_at
		FROM delivery_challans c
		LEFT JOIN sales_orders o ON o.id=c.order_id
		LEFT JOIN warehouses w ON w.id=c.warehouse_id AND w.business_id=c.business_id
		WHERE c.business_id=? ORDER BY c.created_at DESC`, bizID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DeliveryChallan
	for rows.Next() {
		var ch DeliveryChallan
		if err = rows.Scan(
			&ch.ID, &ch.BusinessID, &ch.OrderID, &ch.OrderNumber,
			&ch.CustomerID, &ch.CustomerName, &ch.ChallanNumber,
			&ch.WarehouseID, &ch.WarehouseName, &ch.Status,
			&ch.CourierName, &ch.TrackingNumber,
			&ch.DispatchDate, &ch.DeliveryDate, &ch.Notes, &ch.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, ch)
	}
	return out, rows.Err()
}

func (s *CRMStore) GetChallan(id, bizID int) (*DeliveryChallan, error) {
	var ch DeliveryChallan
	err := s.db.QueryRow(`
		SELECT c.id, c.business_id, c.order_id, COALESCE(o.order_number,''),
		       c.customer_id, c.customer_name, c.challan_number,
		       c.warehouse_id, COALESCE(w.name,''), c.status,
		       c.courier_name, c.tracking_number,
		       c.dispatch_date, c.delivery_date, c.notes, c.created_at
		FROM delivery_challans c
		LEFT JOIN sales_orders o ON o.id=c.order_id
		LEFT JOIN warehouses w ON w.id=c.warehouse_id AND w.business_id=c.business_id
		WHERE c.id=? AND c.business_id=?`, id, bizID,
	).Scan(
		&ch.ID, &ch.BusinessID, &ch.OrderID, &ch.OrderNumber,
		&ch.CustomerID, &ch.CustomerName, &ch.ChallanNumber,
		&ch.WarehouseID, &ch.WarehouseName, &ch.Status,
		&ch.CourierName, &ch.TrackingNumber,
		&ch.DispatchDate, &ch.DeliveryDate, &ch.Notes, &ch.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.Query(`SELECT id, challan_id, order_item_id, product_id, product_name, sku,
		quantity, batch_id, batch_number FROM delivery_challan_items WHERE challan_id=?`, ch.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var it DeliveryChallanItem
		if err = rows.Scan(&it.ID, &it.ChallanID, &it.OrderItemID, &it.ProductID, &it.ProductName, &it.SKU,
			&it.Quantity, &it.BatchID, &it.BatchNumber); err != nil {
			return nil, err
		}
		ch.Items = append(ch.Items, it)
	}
	return &ch, rows.Err()
}

func (s *CRMStore) CreateChallanTx(tx *sql.Tx, ch *DeliveryChallan) (int64, error) {
	res, err := tx.Exec(`INSERT INTO delivery_challans (business_id, order_id, customer_id, customer_name,
		challan_number, warehouse_id, status, courier_name, tracking_number,
		dispatch_date, delivery_date, notes)
		VALUES (?,?,?,?,?,?,'draft',?,?,?,?,?)`,
		ch.BusinessID, ch.OrderID, ch.CustomerID, ch.CustomerName,
		ch.ChallanNumber, ch.WarehouseID, ch.CourierName, ch.TrackingNumber,
		ch.DispatchDate, ch.DeliveryDate, ch.Notes,
	)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	for _, it := range ch.Items {
		if _, err = tx.Exec(`INSERT INTO delivery_challan_items (challan_id, order_item_id, product_id,
			product_name, sku, quantity, batch_id, batch_number)
			VALUES (?,?,?,?,?,?,?,?)`,
			id, it.OrderItemID, it.ProductID, it.ProductName, it.SKU,
			it.Quantity, it.BatchID, it.BatchNumber,
		); err != nil {
			return 0, err
		}
	}
	return id, nil
}

func (s *CRMStore) UpdateChallanStatus(tx *sql.Tx, id int, status string) error {
	_, err := tx.Exec(`UPDATE delivery_challans SET status=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`, status, id)
	return err
}

func (s *CRMStore) NextChallanNumber(bizID int) string {
	var count int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM delivery_challans WHERE business_id=?`, bizID).Scan(&count)
	return fmt.Sprintf("DC-%s-%04d", time.Now().Format("200601"), count+1)
}

// ── Customer Payments ─────────────────────────────────────────────────────────

func (s *CRMStore) ListPayments(bizID, customerID int) ([]CustomerPayment, error) {
	query := `SELECT id, business_id, customer_id, customer_name, order_id, order_number,
		payment_number, amount, payment_method, payment_type, reference, notes, created_at
		FROM customer_payments WHERE business_id=?`
	args := []any{bizID}
	if customerID > 0 {
		query += ` AND customer_id=?`
		args = append(args, customerID)
	}
	query += ` ORDER BY created_at DESC`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CustomerPayment
	for rows.Next() {
		var p CustomerPayment
		if err = rows.Scan(&p.ID, &p.BusinessID, &p.CustomerID, &p.CustomerName,
			&p.OrderID, &p.OrderNumber, &p.PaymentNumber, &p.Amount,
			&p.PaymentMethod, &p.PaymentType, &p.Reference, &p.Notes, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *CRMStore) CreatePayment(p *CustomerPayment) (*CustomerPayment, error) {
	var count int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM customer_payments WHERE business_id=?`, p.BusinessID).Scan(&count)
	p.PaymentNumber = fmt.Sprintf("CPAY-%s-%04d", time.Now().Format("200601"), count+1)

	res, err := s.db.Exec(`INSERT INTO customer_payments (business_id, customer_id, customer_name,
		order_id, order_number, payment_number, amount, payment_method, payment_type, reference, notes)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		p.BusinessID, p.CustomerID, p.CustomerName,
		p.OrderID, p.OrderNumber, p.PaymentNumber, p.Amount,
		p.PaymentMethod, p.PaymentType, p.Reference, p.Notes,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	p.ID = int(id)
	return p, nil
}

func (s *CRMStore) TotalOutstanding(bizID int) (float64, error) {
	var outstanding float64
	err := s.db.QueryRow(`
		SELECT COALESCE(SUM(o.grand_total),0) - COALESCE(SUM(p.amount),0)
		FROM sales_orders o
		LEFT JOIN (SELECT order_id, SUM(amount) as amount FROM customer_payments WHERE business_id=? GROUP BY order_id) p ON p.order_id=o.id
		WHERE o.business_id=? AND o.status NOT IN ('cancelled','draft')`, bizID, bizID,
	).Scan(&outstanding)
	return outstanding, err
}

// ── Dashboard Stats ───────────────────────────────────────────────────────────

func (s *CRMStore) Stats(bizID int) (CRMStats, error) {
	var st CRMStats
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM crm_customers WHERE business_id=? AND status='active'`, bizID).Scan(&st.TotalCustomers)
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM quotations WHERE business_id=? AND status IN ('draft','sent','approved')`, bizID).Scan(&st.PendingQuotes)
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM sales_orders WHERE business_id=? AND status IN ('confirmed','packed','dispatched')`, bizID).Scan(&st.ActiveOrders)
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM delivery_challans WHERE business_id=? AND status='draft'`, bizID).Scan(&st.PendingDeliveries)
	_ = s.db.QueryRow(`SELECT COALESCE(SUM(grand_total),0) FROM sales_orders WHERE business_id=? AND status='completed' AND YEAR(created_at)=YEAR(CURDATE()) AND MONTH(created_at)=MONTH(CURDATE())`, bizID).Scan(&st.MonthRevenue)
	st.TotalOutstanding, _ = s.TotalOutstanding(bizID)
	return st, nil
}
