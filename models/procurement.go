package models

import (
	"database/sql"
	"fmt"
	"time"
)

// ── Domain types ──────────────────────────────────────────────────────────────

type Supplier struct {
	ID            int
	BusinessID    int
	SupplierCode  string
	Name          string
	Email         string
	Phone         string
	GSTIN         string
	PAN           string
	Address       string
	ContactPerson string
	PaymentTerms  int // days
	CreditLimit   float64
	Status        string // active | inactive | blacklisted
	Notes         string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	// Computed
	TotalPurchases  float64
	TotalPaid       float64
	OutstandingDue  float64
}

type ProcurementOrder struct {
	ID             int
	BusinessID     int
	SupplierID     int
	SupplierName   string
	PONumber       string
	Status         string // draft|pending|approved|partially_received|completed|cancelled
	WarehouseID    int
	WarehouseName  string
	ExpectedDate   *time.Time
	Notes          string
	Subtotal       float64
	TaxTotal       float64
	GrandTotal     float64
	ApprovedBy     string
	ApprovedAt     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Items          []ProcurementOrderItem
	ReceivedValue  float64 // sum of received items × unit_price
}

type ProcurementOrderItem struct {
	ID          int
	OrderID     int
	ProductID   int
	ProductName string
	SKU         string
	Quantity    int
	ReceivedQty int
	PendingQty  int // Quantity - ReceivedQty
	UnitPrice   float64
	TaxRate     float64
	TaxAmount   float64
	LineTotal   float64
}

type ProcurementGRN struct {
	ID            int
	BusinessID    int
	OrderID       int    // 0 if direct receive without PO
	PONumber      string // joined
	SupplierID    int
	SupplierName  string
	GRNNumber     string
	WarehouseID   int
	WarehouseName string
	Notes         string
	TotalReceived int
	CreatedAt     time.Time
	Items         []ProcurementGRNItem
}

type ProcurementGRNItem struct {
	ID          int
	GRNID       int
	OrderItemID int // 0 if no PO
	ProductID   int
	ProductName string
	SKU         string
	OrderedQty  int
	ReceivedQty int
	DamagedQty  int
	BatchNumber string
	LotNumber   string
	MfgDate     *time.Time
	ExpiryDate  *time.Time
	UnitPrice   float64
}

type SupplierPayment struct {
	ID            int
	BusinessID    int
	SupplierID    int
	SupplierName  string
	OrderID       int
	PONumber      string
	PaymentNumber string
	Amount        float64
	PaymentMethod string
	Reference     string
	Notes         string
	CreatedAt     time.Time
}

// ReorderSuggestion is a product that needs procurement.
type ReorderSuggestion struct {
	ProductID          int
	ProductName        string
	SKU                string
	CurrentStock       int
	LowStockThreshold  int
	SuggestedOrderQty  int
	LastSupplierID     int
	LastSupplierName   string
	LastPurchasePrice  float64
	EstimatedCost      float64
}

// ── Store ─────────────────────────────────────────────────────────────────────

type ProcurementStore struct {
	db *sql.DB
}

func NewProcurementStore(db *sql.DB) *ProcurementStore {
	return &ProcurementStore{db: db}
}

func (s *ProcurementStore) Migrate() error {
	tables := []string{
		`CREATE TABLE IF NOT EXISTS suppliers (
			id             INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			business_id    INT NOT NULL DEFAULT 0,
			supplier_code  VARCHAR(50) NOT NULL DEFAULT '',
			name           VARCHAR(255) NOT NULL,
			email          VARCHAR(255) NOT NULL DEFAULT '',
			phone          VARCHAR(50) NOT NULL DEFAULT '',
			gstin          VARCHAR(20) NOT NULL DEFAULT '',
			pan            VARCHAR(20) NOT NULL DEFAULT '',
			address        TEXT NOT NULL DEFAULT '',
			contact_person VARCHAR(255) NOT NULL DEFAULT '',
			payment_terms  INT NOT NULL DEFAULT 30,
			credit_limit   DECIMAL(12,2) NOT NULL DEFAULT 0,
			status         VARCHAR(20) NOT NULL DEFAULT 'active',
			notes          TEXT NOT NULL DEFAULT '',
			created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS procurement_orders (
			id              INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			business_id     INT NOT NULL DEFAULT 0,
			supplier_id     INT NOT NULL DEFAULT 0,
			supplier_name   VARCHAR(255) NOT NULL DEFAULT '',
			po_number       VARCHAR(50) NOT NULL DEFAULT '',
			status          VARCHAR(30) NOT NULL DEFAULT 'draft',
			warehouse_id    INT NOT NULL DEFAULT 0,
			expected_date   DATE NULL,
			notes           TEXT NOT NULL DEFAULT '',
			subtotal        DECIMAL(12,2) NOT NULL DEFAULT 0,
			tax_total       DECIMAL(12,2) NOT NULL DEFAULT 0,
			grand_total     DECIMAL(12,2) NOT NULL DEFAULT 0,
			approved_by     VARCHAR(255) NOT NULL DEFAULT '',
			approved_at     DATETIME NULL,
			created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS procurement_order_items (
			id           INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			order_id     INT NOT NULL,
			product_id   INT NOT NULL DEFAULT 0,
			product_name VARCHAR(255) NOT NULL DEFAULT '',
			sku          VARCHAR(255) NOT NULL DEFAULT '',
			quantity     INT NOT NULL DEFAULT 1,
			received_qty INT NOT NULL DEFAULT 0,
			unit_price   DECIMAL(12,2) NOT NULL DEFAULT 0,
			tax_rate     DECIMAL(5,2) NOT NULL DEFAULT 0,
			tax_amount   DECIMAL(12,2) NOT NULL DEFAULT 0,
			line_total   DECIMAL(12,2) NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS procurement_grn (
			id             INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			business_id    INT NOT NULL DEFAULT 0,
			order_id       INT NOT NULL DEFAULT 0,
			supplier_id    INT NOT NULL DEFAULT 0,
			supplier_name  VARCHAR(255) NOT NULL DEFAULT '',
			grn_number     VARCHAR(50) NOT NULL DEFAULT '',
			warehouse_id   INT NOT NULL DEFAULT 0,
			notes          TEXT NOT NULL DEFAULT '',
			total_received INT NOT NULL DEFAULT 0,
			created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS procurement_grn_items (
			id              INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			grn_id          INT NOT NULL,
			order_item_id   INT NOT NULL DEFAULT 0,
			product_id      INT NOT NULL DEFAULT 0,
			product_name    VARCHAR(255) NOT NULL DEFAULT '',
			sku             VARCHAR(255) NOT NULL DEFAULT '',
			ordered_qty     INT NOT NULL DEFAULT 0,
			received_qty    INT NOT NULL DEFAULT 0,
			damaged_qty     INT NOT NULL DEFAULT 0,
			batch_number    VARCHAR(100) NOT NULL DEFAULT '',
			lot_number      VARCHAR(100) NOT NULL DEFAULT '',
			mfg_date        DATE NULL,
			expiry_date     DATE NULL,
			unit_price      DECIMAL(12,2) NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS supplier_payments (
			id             INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			business_id    INT NOT NULL DEFAULT 0,
			supplier_id    INT NOT NULL DEFAULT 0,
			supplier_name  VARCHAR(255) NOT NULL DEFAULT '',
			order_id       INT NOT NULL DEFAULT 0,
			payment_number VARCHAR(50) NOT NULL DEFAULT '',
			amount         DECIMAL(12,2) NOT NULL DEFAULT 0,
			payment_method VARCHAR(20) NOT NULL DEFAULT 'cash',
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
		`CREATE INDEX idx_suppliers_biz       ON suppliers(business_id)`,
		`CREATE INDEX idx_proc_orders_biz     ON procurement_orders(business_id)`,
		`CREATE INDEX idx_proc_orders_supp    ON procurement_orders(supplier_id)`,
		`CREATE INDEX idx_proc_orders_status  ON procurement_orders(business_id, status)`,
		`CREATE INDEX idx_proc_items_order    ON procurement_order_items(order_id)`,
		`CREATE INDEX idx_proc_grn_biz        ON procurement_grn(business_id)`,
		`CREATE INDEX idx_proc_grn_order      ON procurement_grn(order_id)`,
		`CREATE INDEX idx_proc_grn_items_grn  ON procurement_grn_items(grn_id)`,
		`CREATE INDEX idx_supp_payments_biz   ON supplier_payments(business_id)`,
		`CREATE INDEX idx_supp_payments_supp  ON supplier_payments(supplier_id)`,
	} {
		_, _ = s.db.Exec(idx)
	}
	return nil
}

// ── Suppliers ─────────────────────────────────────────────────────────────────

func (s *ProcurementStore) ListSuppliers(bizID int) ([]Supplier, error) {
	rows, err := s.db.Query(`
		SELECT s.id, s.business_id, s.supplier_code, s.name, s.email, s.phone,
		       s.gstin, s.pan, s.address, s.contact_person,
		       s.payment_terms, s.credit_limit, s.status, s.notes,
		       s.created_at, s.updated_at,
		       COALESCE(SUM(po.grand_total),0),
		       COALESCE(SUM(sp.amount),0)
		FROM suppliers s
		LEFT JOIN procurement_orders po ON po.supplier_id=s.id AND po.status='completed'
		LEFT JOIN supplier_payments  sp ON sp.supplier_id=s.id
		WHERE s.business_id=?
		GROUP BY s.id
		ORDER BY s.name ASC`, bizID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Supplier
	for rows.Next() {
		var sup Supplier
		if err = rows.Scan(
			&sup.ID, &sup.BusinessID, &sup.SupplierCode, &sup.Name, &sup.Email, &sup.Phone,
			&sup.GSTIN, &sup.PAN, &sup.Address, &sup.ContactPerson,
			&sup.PaymentTerms, &sup.CreditLimit, &sup.Status, &sup.Notes,
			&sup.CreatedAt, &sup.UpdatedAt,
			&sup.TotalPurchases, &sup.TotalPaid,
		); err != nil {
			return nil, err
		}
		sup.OutstandingDue = sup.TotalPurchases - sup.TotalPaid
		out = append(out, sup)
	}
	return out, rows.Err()
}

func (s *ProcurementStore) GetSupplier(id, bizID int) (*Supplier, error) {
	var sup Supplier
	err := s.db.QueryRow(`
		SELECT s.id, s.business_id, s.supplier_code, s.name, s.email, s.phone,
		       s.gstin, s.pan, s.address, s.contact_person,
		       s.payment_terms, s.credit_limit, s.status, s.notes,
		       s.created_at, s.updated_at,
		       COALESCE((SELECT SUM(po.grand_total) FROM procurement_orders po WHERE po.supplier_id=s.id AND po.status='completed'),0),
		       COALESCE((SELECT SUM(sp.amount) FROM supplier_payments sp WHERE sp.supplier_id=s.id),0)
		FROM suppliers s WHERE s.id=? AND s.business_id=?`, id, bizID,
	).Scan(
		&sup.ID, &sup.BusinessID, &sup.SupplierCode, &sup.Name, &sup.Email, &sup.Phone,
		&sup.GSTIN, &sup.PAN, &sup.Address, &sup.ContactPerson,
		&sup.PaymentTerms, &sup.CreditLimit, &sup.Status, &sup.Notes,
		&sup.CreatedAt, &sup.UpdatedAt,
		&sup.TotalPurchases, &sup.TotalPaid,
	)
	if err != nil {
		return nil, err
	}
	sup.OutstandingDue = sup.TotalPurchases - sup.TotalPaid
	return &sup, nil
}

func (s *ProcurementStore) CreateSupplier(sup *Supplier) (*Supplier, error) {
	if sup.SupplierCode == "" {
		// Auto-generate supplier code
		var count int
		_ = s.db.QueryRow(`SELECT COUNT(*) FROM suppliers WHERE business_id=?`, sup.BusinessID).Scan(&count)
		sup.SupplierCode = fmt.Sprintf("SUP-%04d", count+1)
	}
	res, err := s.db.Exec(`
		INSERT INTO suppliers (business_id, supplier_code, name, email, phone, gstin, pan,
		                       address, contact_person, payment_terms, credit_limit, status, notes)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		sup.BusinessID, sup.SupplierCode, sup.Name, sup.Email, sup.Phone,
		sup.GSTIN, sup.PAN, sup.Address, sup.ContactPerson,
		sup.PaymentTerms, sup.CreditLimit, sup.Status, sup.Notes,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetSupplier(int(id), sup.BusinessID)
}

func (s *ProcurementStore) UpdateSupplier(sup *Supplier) error {
	_, err := s.db.Exec(`
		UPDATE suppliers SET name=?, email=?, phone=?, gstin=?, pan=?, address=?,
		contact_person=?, payment_terms=?, credit_limit=?, status=?, notes=?,
		updated_at=CURRENT_TIMESTAMP
		WHERE id=? AND business_id=?`,
		sup.Name, sup.Email, sup.Phone, sup.GSTIN, sup.PAN, sup.Address,
		sup.ContactPerson, sup.PaymentTerms, sup.CreditLimit, sup.Status, sup.Notes,
		sup.ID, sup.BusinessID,
	)
	return err
}

func (s *ProcurementStore) CountSuppliers(bizID int) (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM suppliers WHERE business_id=?`, bizID).Scan(&count)
	return count, err
}

// ── Purchase Orders ───────────────────────────────────────────────────────────

func (s *ProcurementStore) ListOrders(bizID int, status string) ([]ProcurementOrder, error) {
	query := `
		SELECT o.id, o.business_id, o.supplier_id, o.supplier_name, o.po_number,
		       o.status, o.warehouse_id, COALESCE(w.name,''),
		       o.expected_date, o.notes, o.subtotal, o.tax_total, o.grand_total,
		       o.approved_by, o.approved_at, o.created_at, o.updated_at
		FROM procurement_orders o
		LEFT JOIN warehouses w ON w.id=o.warehouse_id AND w.business_id=o.business_id
		WHERE o.business_id=?`
	args := []any{bizID}
	if status != "" {
		query += ` AND o.status=?`
		args = append(args, status)
	}
	query += ` ORDER BY o.created_at DESC`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ProcurementOrder
	for rows.Next() {
		var o ProcurementOrder
		if err = scanOrder(rows, &o); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func (s *ProcurementStore) GetOrder(id, bizID int) (*ProcurementOrder, error) {
	var o ProcurementOrder
	err := s.db.QueryRow(`
		SELECT o.id, o.business_id, o.supplier_id, o.supplier_name, o.po_number,
		       o.status, o.warehouse_id, COALESCE(w.name,''),
		       o.expected_date, o.notes, o.subtotal, o.tax_total, o.grand_total,
		       o.approved_by, o.approved_at, o.created_at, o.updated_at
		FROM procurement_orders o
		LEFT JOIN warehouses w ON w.id=o.warehouse_id AND w.business_id=o.business_id
		WHERE o.id=? AND o.business_id=?`, id, bizID,
	).Scan(
		&o.ID, &o.BusinessID, &o.SupplierID, &o.SupplierName, &o.PONumber,
		&o.Status, &o.WarehouseID, &o.WarehouseName,
		&o.ExpectedDate, &o.Notes, &o.Subtotal, &o.TaxTotal, &o.GrandTotal,
		&o.ApprovedBy, &o.ApprovedAt, &o.CreatedAt, &o.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	rows, err := s.db.Query(`
		SELECT id, order_id, product_id, product_name, sku, quantity, received_qty,
		       unit_price, tax_rate, tax_amount, line_total
		FROM procurement_order_items WHERE order_id=? ORDER BY id`, o.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var item ProcurementOrderItem
		if err = rows.Scan(
			&item.ID, &item.OrderID, &item.ProductID, &item.ProductName, &item.SKU,
			&item.Quantity, &item.ReceivedQty, &item.UnitPrice, &item.TaxRate, &item.TaxAmount, &item.LineTotal,
		); err != nil {
			return nil, err
		}
		item.PendingQty = item.Quantity - item.ReceivedQty
		o.Items = append(o.Items, item)
	}
	return &o, rows.Err()
}

func scanOrder(row interface{ Scan(...any) error }, o *ProcurementOrder) error {
	return row.Scan(
		&o.ID, &o.BusinessID, &o.SupplierID, &o.SupplierName, &o.PONumber,
		&o.Status, &o.WarehouseID, &o.WarehouseName,
		&o.ExpectedDate, &o.Notes, &o.Subtotal, &o.TaxTotal, &o.GrandTotal,
		&o.ApprovedBy, &o.ApprovedAt, &o.CreatedAt, &o.UpdatedAt,
	)
}

func (s *ProcurementStore) CreateOrder(o *ProcurementOrder) (*ProcurementOrder, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	res, err := tx.Exec(`
		INSERT INTO procurement_orders (business_id, supplier_id, supplier_name, po_number,
		status, warehouse_id, expected_date, notes, subtotal, tax_total, grand_total)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		o.BusinessID, o.SupplierID, o.SupplierName, o.PONumber,
		"draft", o.WarehouseID, o.ExpectedDate, o.Notes,
		o.Subtotal, o.TaxTotal, o.GrandTotal,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()

	for _, item := range o.Items {
		if _, err = tx.Exec(`
			INSERT INTO procurement_order_items (order_id, product_id, product_name, sku,
			quantity, unit_price, tax_rate, tax_amount, line_total)
			VALUES (?,?,?,?,?,?,?,?,?)`,
			id, item.ProductID, item.ProductName, item.SKU,
			item.Quantity, item.UnitPrice, item.TaxRate, item.TaxAmount, item.LineTotal,
		); err != nil {
			return nil, err
		}
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetOrder(int(id), o.BusinessID)
}

func (s *ProcurementStore) UpdateOrderStatus(id int, status, approvedBy string) error {
	if approvedBy != "" {
		_, err := s.db.Exec(`
			UPDATE procurement_orders SET status=?, approved_by=?, approved_at=CURRENT_TIMESTAMP,
			updated_at=CURRENT_TIMESTAMP WHERE id=?`, status, approvedBy, id)
		return err
	}
	_, err := s.db.Exec(`
		UPDATE procurement_orders SET status=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		status, id)
	return err
}

func (s *ProcurementStore) NextPONumber(bizID int) string {
	var count int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM procurement_orders WHERE business_id=?`, bizID).Scan(&count)
	return fmt.Sprintf("PO-%s-%04d", time.Now().Format("200601"), count+1)
}

func (s *ProcurementStore) CountOrders(bizID int, status string) (int, error) {
	var count int
	var err error
	if status != "" {
		err = s.db.QueryRow(`SELECT COUNT(*) FROM procurement_orders WHERE business_id=? AND status=?`, bizID, status).Scan(&count)
	} else {
		err = s.db.QueryRow(`SELECT COUNT(*) FROM procurement_orders WHERE business_id=?`, bizID).Scan(&count)
	}
	return count, err
}

// UpdateItemReceivedQty updates how much of an order item has been received.
func (s *ProcurementStore) UpdateItemReceivedQty(tx *sql.Tx, itemID, additionalQty int) error {
	_, err := tx.Exec(`UPDATE procurement_order_items SET received_qty=received_qty+? WHERE id=?`,
		additionalQty, itemID)
	return err
}

// CheckOrderCompletion checks if all items received; updates order status accordingly.
func (s *ProcurementStore) CheckOrderCompletion(tx *sql.Tx, orderID int) error {
	var total, received int
	_ = tx.QueryRow(`SELECT COALESCE(SUM(quantity),0), COALESCE(SUM(received_qty),0) FROM procurement_order_items WHERE order_id=?`, orderID).Scan(&total, &received)
	status := "partially_received"
	if received >= total && total > 0 {
		status = "completed"
	}
	_, err := tx.Exec(`UPDATE procurement_orders SET status=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`, status, orderID)
	return err
}

// ── GRN ──────────────────────────────────────────────────────────────────────

func (s *ProcurementStore) ListGRN(bizID int) ([]ProcurementGRN, error) {
	rows, err := s.db.Query(`
		SELECT g.id, g.business_id, g.order_id, COALESCE(po.po_number,'—'),
		       g.supplier_id, g.supplier_name, g.grn_number,
		       g.warehouse_id, COALESCE(w.name,''), g.notes, g.total_received, g.created_at
		FROM procurement_grn g
		LEFT JOIN procurement_orders po ON po.id=g.order_id
		LEFT JOIN warehouses w ON w.id=g.warehouse_id AND w.business_id=g.business_id
		WHERE g.business_id=? ORDER BY g.created_at DESC`, bizID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ProcurementGRN
	for rows.Next() {
		var g ProcurementGRN
		if err = rows.Scan(
			&g.ID, &g.BusinessID, &g.OrderID, &g.PONumber,
			&g.SupplierID, &g.SupplierName, &g.GRNNumber,
			&g.WarehouseID, &g.WarehouseName, &g.Notes, &g.TotalReceived, &g.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (s *ProcurementStore) GetGRN(id, bizID int) (*ProcurementGRN, error) {
	var g ProcurementGRN
	err := s.db.QueryRow(`
		SELECT g.id, g.business_id, g.order_id, COALESCE(po.po_number,'—'),
		       g.supplier_id, g.supplier_name, g.grn_number,
		       g.warehouse_id, COALESCE(w.name,''), g.notes, g.total_received, g.created_at
		FROM procurement_grn g
		LEFT JOIN procurement_orders po ON po.id=g.order_id
		LEFT JOIN warehouses w ON w.id=g.warehouse_id AND w.business_id=g.business_id
		WHERE g.id=? AND g.business_id=?`, id, bizID,
	).Scan(
		&g.ID, &g.BusinessID, &g.OrderID, &g.PONumber,
		&g.SupplierID, &g.SupplierName, &g.GRNNumber,
		&g.WarehouseID, &g.WarehouseName, &g.Notes, &g.TotalReceived, &g.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.Query(`
		SELECT id, grn_id, order_item_id, product_id, product_name, sku,
		       ordered_qty, received_qty, damaged_qty,
		       batch_number, lot_number, mfg_date, expiry_date, unit_price
		FROM procurement_grn_items WHERE grn_id=?`, g.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var item ProcurementGRNItem
		if err = rows.Scan(
			&item.ID, &item.GRNID, &item.OrderItemID, &item.ProductID, &item.ProductName, &item.SKU,
			&item.OrderedQty, &item.ReceivedQty, &item.DamagedQty,
			&item.BatchNumber, &item.LotNumber, &item.MfgDate, &item.ExpiryDate, &item.UnitPrice,
		); err != nil {
			return nil, err
		}
		g.Items = append(g.Items, item)
	}
	return &g, rows.Err()
}

func (s *ProcurementStore) CreateGRNTx(tx *sql.Tx, g *ProcurementGRN) (int64, error) {
	res, err := tx.Exec(`
		INSERT INTO procurement_grn (business_id, order_id, supplier_id, supplier_name,
		grn_number, warehouse_id, notes, total_received)
		VALUES (?,?,?,?,?,?,?,?)`,
		g.BusinessID, g.OrderID, g.SupplierID, g.SupplierName,
		g.GRNNumber, g.WarehouseID, g.Notes, g.TotalReceived,
	)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()

	for _, item := range g.Items {
		if _, err = tx.Exec(`
			INSERT INTO procurement_grn_items (grn_id, order_item_id, product_id, product_name, sku,
			ordered_qty, received_qty, damaged_qty, batch_number, lot_number, mfg_date, expiry_date, unit_price)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			id, item.OrderItemID, item.ProductID, item.ProductName, item.SKU,
			item.OrderedQty, item.ReceivedQty, item.DamagedQty,
			item.BatchNumber, item.LotNumber, item.MfgDate, item.ExpiryDate, item.UnitPrice,
		); err != nil {
			return 0, err
		}
	}
	return id, nil
}

func (s *ProcurementStore) NextGRNNumber(bizID int) string {
	var count int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM procurement_grn WHERE business_id=?`, bizID).Scan(&count)
	return fmt.Sprintf("GRN-%s-%04d", time.Now().Format("200601"), count+1)
}

// ── Supplier Payments ─────────────────────────────────────────────────────────

func (s *ProcurementStore) ListPayments(bizID, supplierID int) ([]SupplierPayment, error) {
	query := `
		SELECT id, business_id, supplier_id, supplier_name, order_id,
		       payment_number, amount, payment_method, reference, notes, created_at
		FROM supplier_payments WHERE business_id=?`
	args := []any{bizID}
	if supplierID > 0 {
		query += ` AND supplier_id=?`
		args = append(args, supplierID)
	}
	query += ` ORDER BY created_at DESC`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SupplierPayment
	for rows.Next() {
		var p SupplierPayment
		if err = rows.Scan(
			&p.ID, &p.BusinessID, &p.SupplierID, &p.SupplierName, &p.OrderID,
			&p.PaymentNumber, &p.Amount, &p.PaymentMethod, &p.Reference, &p.Notes, &p.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *ProcurementStore) CreatePayment(p *SupplierPayment) (*SupplierPayment, error) {
	var count int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM supplier_payments WHERE business_id=?`, p.BusinessID).Scan(&count)
	p.PaymentNumber = fmt.Sprintf("PAY-%s-%04d", time.Now().Format("200601"), count+1)

	res, err := s.db.Exec(`
		INSERT INTO supplier_payments (business_id, supplier_id, supplier_name, order_id,
		payment_number, amount, payment_method, reference, notes)
		VALUES (?,?,?,?,?,?,?,?,?)`,
		p.BusinessID, p.SupplierID, p.SupplierName, p.OrderID,
		p.PaymentNumber, p.Amount, p.PaymentMethod, p.Reference, p.Notes,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	p.ID = int(id)
	return p, nil
}

func (s *ProcurementStore) TotalDues(bizID int) (float64, error) {
	var dues float64
	err := s.db.QueryRow(`
		SELECT COALESCE(SUM(po.grand_total),0) - COALESCE(SUM(sp.amount),0)
		FROM (SELECT supplier_id, SUM(grand_total) as grand_total
		      FROM procurement_orders WHERE business_id=? AND status='completed'
		      GROUP BY supplier_id) po
		LEFT JOIN (SELECT supplier_id, SUM(amount) as amount
		           FROM supplier_payments WHERE business_id=?
		           GROUP BY supplier_id) sp ON sp.supplier_id=po.supplier_id`,
		bizID, bizID,
	).Scan(&dues)
	return dues, err
}

// ── Reorder Suggestions ───────────────────────────────────────────────────────

func (s *ProcurementStore) ReorderSuggestions(bizID int) ([]ReorderSuggestion, error) {
	rows, err := s.db.Query(`
		SELECT p.id, p.name, p.sku, p.stock, p.low_stock_threshold,
		       GREATEST(p.low_stock_threshold * 2 - p.stock, p.low_stock_threshold) as suggested_qty,
		       COALESCE(last_supp.supplier_id, 0),
		       COALESCE(last_supp.supplier_name, ''),
		       COALESCE(last_supp.unit_price, 0),
		       GREATEST(p.low_stock_threshold * 2 - p.stock, p.low_stock_threshold) * COALESCE(last_supp.unit_price, 0)
		FROM products p
		LEFT JOIN (
		    SELECT gi.product_id, po.supplier_id, po.supplier_name, gi.unit_price
		    FROM procurement_grn_items gi
		    JOIN procurement_grn g ON g.id=gi.grn_id
		    JOIN procurement_orders po ON po.id=g.order_id
		    WHERE g.business_id=?
		    ORDER BY g.created_at DESC
		    LIMIT 1000
		) last_supp ON last_supp.product_id=p.id
		WHERE p.business_id=? AND p.stock<=p.low_stock_threshold AND p.status='active'
		ORDER BY (p.low_stock_threshold-p.stock) DESC`,
		bizID, bizID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ReorderSuggestion
	for rows.Next() {
		var r ReorderSuggestion
		if err = rows.Scan(
			&r.ProductID, &r.ProductName, &r.SKU, &r.CurrentStock, &r.LowStockThreshold,
			&r.SuggestedOrderQty, &r.LastSupplierID, &r.LastSupplierName,
			&r.LastPurchasePrice, &r.EstimatedCost,
		); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ── Procurement Stats (for dashboard) ────────────────────────────────────────

type ProcurementStats struct {
	PendingPOs     int
	PendingGRNs    int
	TotalDues      float64
	TotalSuppliers int
	ReorderCount   int
	MonthSpend     float64
}

func (s *ProcurementStore) Stats(bizID int) (ProcurementStats, error) {
	var st ProcurementStats
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM procurement_orders WHERE business_id=? AND status IN ('pending','approved')`, bizID).Scan(&st.PendingPOs)
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM suppliers WHERE business_id=? AND status='active'`, bizID).Scan(&st.TotalSuppliers)
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM products WHERE business_id=? AND stock<=low_stock_threshold AND status='active'`, bizID).Scan(&st.ReorderCount)
	_ = s.db.QueryRow(`
		SELECT COALESCE(SUM(grand_total),0) FROM procurement_orders
		WHERE business_id=? AND status='completed'
		AND YEAR(created_at)=YEAR(CURDATE()) AND MONTH(created_at)=MONTH(CURDATE())`, bizID).Scan(&st.MonthSpend)
	st.TotalDues, _ = s.TotalDues(bizID)
	return st, nil
}
