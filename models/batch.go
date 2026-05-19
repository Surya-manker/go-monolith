package models

import (
	"database/sql"
	"fmt"
	"time"
)

// ── Domain types ──────────────────────────────────────────────────────────────

type Batch struct {
	ID            int
	BusinessID    int
	ProductID     int
	ProductName   string // joined
	SKU           string // joined
	WarehouseID   int
	WarehouseName string // joined
	BatchNumber   string
	LotNumber     string
	MfgDate       *time.Time
	ExpiryDate    *time.Time
	Quantity      int
	Status        string // active | expired | consumed | damaged
	Notes         string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	// Computed display helpers
	ExpiryDays  int    // negative = already expired
	ExpiryClass string // good | warning | danger | expired
}

type BatchLog struct {
	ID          int
	BatchID     int
	BatchNumber string
	ProductID   int
	ProductName string
	WarehouseID int
	BusinessID  int
	ChangeType  string
	QtyBefore   int
	QtyChange   int
	QtyAfter    int
	RefType     string
	RefID       int
	Note        string
	CreatedAt   time.Time
}

// BatchDeduction is returned by SelectFEFOTx to describe how much to take from each batch.
type BatchDeduction struct {
	BatchID  int
	Quantity int
}

// ── Returns ───────────────────────────────────────────────────────────────────

type SalesReturn struct {
	ID             int
	BusinessID     int
	WarehouseID    int
	WarehouseName  string
	ReturnNumber   string
	OriginalSaleID *int
	CustomerName   string
	CustomerPhone  string
	ReturnReason   string
	TotalAmount    float64
	Status         string
	CreatedAt      time.Time
	Items          []SalesReturnItem
}

type SalesReturnItem struct {
	ID        int
	ReturnID  int
	ProductID int
	BatchID   *int
	BatchNumber string
	ProductName string
	SKU         string
	Quantity    int
	UnitPrice   float64
	LineTotal   float64
	Condition   string // resalable | damaged | expired
	Notes       string
}

type PurchaseReturn struct {
	ID            int
	BusinessID    int
	WarehouseID   int
	WarehouseName string
	ReturnNumber  string
	VendorName    string
	ReturnReason  string
	TotalAmount   float64
	Status        string
	CreatedAt     time.Time
	Items         []PurchaseReturnItem
}

type PurchaseReturnItem struct {
	ID          int
	ReturnID    int
	ProductID   int
	BatchID     *int
	BatchNumber string
	ProductName string
	SKU         string
	Quantity    int
	UnitPrice   float64
	LineTotal   float64
	Notes       string
}

// ExpiryStats aggregates batch expiry info for the dashboard.
type ExpiryStats struct {
	ExpiredCount  int
	ExpiredQty    int
	ExpiringCount int // expiring within AlertDays
	ExpiringQty   int
	AlertDays     int
}

// ── Store ─────────────────────────────────────────────────────────────────────

type BatchStore struct {
	db *sql.DB
}

func NewBatchStore(db *sql.DB) *BatchStore {
	return &BatchStore{db: db}
}

func (s *BatchStore) Migrate() error {
	tables := []string{
		`CREATE TABLE IF NOT EXISTS batches (
			id           INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			business_id  INT NOT NULL DEFAULT 0,
			product_id   INT NOT NULL,
			warehouse_id INT NOT NULL DEFAULT 0,
			batch_number VARCHAR(100) NOT NULL DEFAULT '',
			lot_number   VARCHAR(100) NOT NULL DEFAULT '',
			mfg_date     DATE NULL,
			expiry_date  DATE NULL,
			quantity     INT NOT NULL DEFAULT 0,
			status       VARCHAR(20) NOT NULL DEFAULT 'active',
			notes        TEXT NOT NULL DEFAULT '',
			created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS batch_logs (
			id           INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			batch_id     INT NOT NULL,
			product_id   INT NOT NULL,
			warehouse_id INT NOT NULL DEFAULT 0,
			business_id  INT NOT NULL DEFAULT 0,
			change_type  VARCHAR(50) NOT NULL,
			qty_before   INT NOT NULL DEFAULT 0,
			qty_change   INT NOT NULL DEFAULT 0,
			qty_after    INT NOT NULL DEFAULT 0,
			ref_type     VARCHAR(50) NOT NULL DEFAULT '',
			ref_id       INT NOT NULL DEFAULT 0,
			note         TEXT NOT NULL DEFAULT '',
			created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS sales_returns (
			id               INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			business_id      INT NOT NULL DEFAULT 0,
			warehouse_id     INT NOT NULL DEFAULT 0,
			return_number    VARCHAR(50) NOT NULL DEFAULT '',
			original_sale_id INT NULL,
			customer_name    VARCHAR(255) NOT NULL DEFAULT '',
			customer_phone   VARCHAR(20) NOT NULL DEFAULT '',
			return_reason    TEXT NOT NULL DEFAULT '',
			total_amount     DECIMAL(12,2) NOT NULL DEFAULT 0,
			status           VARCHAR(20) NOT NULL DEFAULT 'completed',
			created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS sales_return_items (
			id             INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			return_id      INT NOT NULL,
			product_id     INT NOT NULL DEFAULT 0,
			batch_id       INT NULL,
			product_name   VARCHAR(255) NOT NULL DEFAULT '',
			sku            VARCHAR(255) NOT NULL DEFAULT '',
			quantity       INT NOT NULL DEFAULT 1,
			unit_price     DECIMAL(12,2) NOT NULL DEFAULT 0,
			line_total     DECIMAL(12,2) NOT NULL DEFAULT 0,
			item_condition VARCHAR(20) NOT NULL DEFAULT 'resalable',
			notes          TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS purchase_returns (
			id            INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			business_id   INT NOT NULL DEFAULT 0,
			warehouse_id  INT NOT NULL DEFAULT 0,
			return_number VARCHAR(50) NOT NULL DEFAULT '',
			vendor_name   VARCHAR(255) NOT NULL DEFAULT '',
			return_reason TEXT NOT NULL DEFAULT '',
			total_amount  DECIMAL(12,2) NOT NULL DEFAULT 0,
			status        VARCHAR(20) NOT NULL DEFAULT 'completed',
			created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS purchase_return_items (
			id           INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			return_id    INT NOT NULL,
			product_id   INT NOT NULL DEFAULT 0,
			batch_id     INT NULL,
			product_name VARCHAR(255) NOT NULL DEFAULT '',
			sku          VARCHAR(255) NOT NULL DEFAULT '',
			quantity     INT NOT NULL DEFAULT 1,
			unit_price   DECIMAL(12,2) NOT NULL DEFAULT 0,
			line_total   DECIMAL(12,2) NOT NULL DEFAULT 0,
			notes        TEXT NOT NULL DEFAULT ''
		)`,
	}
	for _, q := range tables {
		if _, err := s.db.Exec(q); err != nil {
			return err
		}
	}
	for _, idx := range []string{
		`CREATE INDEX idx_batches_biz        ON batches(business_id)`,
		`CREATE INDEX idx_batches_product    ON batches(product_id, warehouse_id)`,
		`CREATE INDEX idx_batches_expiry     ON batches(business_id, expiry_date, status)`,
		`CREATE INDEX idx_batch_logs_batch   ON batch_logs(batch_id)`,
		`CREATE INDEX idx_batch_logs_product ON batch_logs(product_id, business_id)`,
		`CREATE INDEX idx_sales_returns_biz  ON sales_returns(business_id)`,
		`CREATE INDEX idx_pur_returns_biz    ON purchase_returns(business_id)`,
	} {
		_, _ = s.db.Exec(idx)
	}
	return nil
}

// ── Batch CRUD ────────────────────────────────────────────────────────────────

const batchJoinCols = `
	b.id, b.business_id, b.product_id,
	COALESCE(p.name,''), COALESCE(p.sku,''),
	b.warehouse_id, COALESCE(w.name,''),
	b.batch_number, b.lot_number,
	b.mfg_date, b.expiry_date,
	b.quantity, b.status, b.notes,
	b.created_at, b.updated_at`

func scanBatch(row interface{ Scan(...any) error }) (*Batch, error) {
	var b Batch
	var mfg, exp *time.Time
	err := row.Scan(
		&b.ID, &b.BusinessID, &b.ProductID,
		&b.ProductName, &b.SKU,
		&b.WarehouseID, &b.WarehouseName,
		&b.BatchNumber, &b.LotNumber,
		&mfg, &exp,
		&b.Quantity, &b.Status, &b.Notes,
		&b.CreatedAt, &b.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	b.MfgDate = mfg
	b.ExpiryDate = exp
	b.computeExpiryClass()
	return &b, nil
}

func (b *Batch) computeExpiryClass() {
	if b.ExpiryDate == nil {
		b.ExpiryClass = "good"
		b.ExpiryDays = 9999
		return
	}
	days := int(time.Until(*b.ExpiryDate).Hours() / 24)
	b.ExpiryDays = days
	switch {
	case days < 0:
		b.ExpiryClass = "expired"
	case days <= 30:
		b.ExpiryClass = "danger"
	case days <= 90:
		b.ExpiryClass = "warning"
	default:
		b.ExpiryClass = "good"
	}
}

func (s *BatchStore) List(bizID, warehouseID, productID int) ([]Batch, error) {
	query := `SELECT` + batchJoinCols + `
		FROM batches b
		LEFT JOIN products   p ON p.id = b.product_id
		LEFT JOIN warehouses w ON w.id = b.warehouse_id AND w.business_id = b.business_id
		WHERE b.business_id=?`
	args := []any{bizID}
	if warehouseID > 0 {
		query += ` AND b.warehouse_id=?`
		args = append(args, warehouseID)
	}
	if productID > 0 {
		query += ` AND b.product_id=?`
		args = append(args, productID)
	}
	query += ` ORDER BY b.expiry_date ASC, b.created_at DESC`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Batch
	for rows.Next() {
		b, err := scanBatch(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *b)
	}
	return out, rows.Err()
}

func (s *BatchStore) Get(id, bizID int) (*Batch, error) {
	row := s.db.QueryRow(
		`SELECT`+batchJoinCols+`
		 FROM batches b
		 LEFT JOIN products   p ON p.id = b.product_id
		 LEFT JOIN warehouses w ON w.id = b.warehouse_id AND w.business_id = b.business_id
		 WHERE b.id=? AND b.business_id=?`,
		id, bizID,
	)
	return scanBatch(row)
}

func (s *BatchStore) Create(b *Batch) (*Batch, error) {
	if b.BatchNumber == "" {
		b.BatchNumber = fmt.Sprintf("BATCH-%d-%d", b.ProductID, time.Now().Unix())
	}
	res, err := s.db.Exec(
		`INSERT INTO batches
		 (business_id, product_id, warehouse_id, batch_number, lot_number, mfg_date, expiry_date, quantity, status, notes)
		 VALUES (?,?,?,?,?,?,?,?,?,?)`,
		b.BusinessID, b.ProductID, b.WarehouseID, b.BatchNumber, b.LotNumber,
		b.MfgDate, b.ExpiryDate, b.Quantity, "active", b.Notes,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	// Log the initial receipt
	_, _ = s.db.Exec(
		`INSERT INTO batch_logs (batch_id, product_id, warehouse_id, business_id, change_type, qty_before, qty_change, qty_after, note)
		 VALUES (?,?,?,?,'purchase_in',0,?,?,?)`,
		id, b.ProductID, b.WarehouseID, b.BusinessID, b.Quantity, b.Quantity, "Batch created",
	)
	return s.Get(int(id), b.BusinessID)
}

func (s *BatchStore) Update(b *Batch) error {
	_, err := s.db.Exec(
		`UPDATE batches SET batch_number=?, lot_number=?, mfg_date=?, expiry_date=?, notes=?, updated_at=CURRENT_TIMESTAMP
		 WHERE id=? AND business_id=?`,
		b.BatchNumber, b.LotNumber, b.MfgDate, b.ExpiryDate, b.Notes, b.ID, b.BusinessID,
	)
	return err
}

func (s *BatchStore) UpdateStatus(id int, status string) error {
	_, err := s.db.Exec(
		`UPDATE batches SET status=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		status, id,
	)
	return err
}

// ── FEFO / FIFO selection ─────────────────────────────────────────────────────

// HasBatches returns true if the product+warehouse has ANY active batches.
// Used to decide whether to use FEFO or fall back to plain warehouse_stock deduction.
func (s *BatchStore) HasBatches(tx *sql.Tx, productID, warehouseID, bizID int) (bool, error) {
	var count int
	err := tx.QueryRow(
		`SELECT COUNT(*) FROM batches
		 WHERE product_id=? AND warehouse_id=? AND business_id=? AND status='active' AND quantity > 0`,
		productID, warehouseID, bizID,
	).Scan(&count)
	return count > 0, err
}

// SelectFEFOTx selects batches for deduction using FEFO (First Expiry First Out).
// For batches without expiry, FIFO (created_at ASC) is used.
// Returns an ordered list of (batchID, qty) deductions that together satisfy requiredQty.
// Returns ErrNoRows-equivalent (empty slice, no error) when no batches exist → use plain stock.
func (s *BatchStore) SelectFEFOTx(tx *sql.Tx, productID, warehouseID, bizID, requiredQty int) ([]BatchDeduction, error) {
	rows, err := tx.Query(
		`SELECT id, quantity
		 FROM batches
		 WHERE product_id=? AND warehouse_id=? AND business_id=?
		   AND status='active' AND quantity > 0
		   AND (expiry_date IS NULL OR expiry_date >= CURDATE())
		 ORDER BY
		   CASE WHEN expiry_date IS NULL THEN 1 ELSE 0 END,
		   expiry_date ASC,
		   created_at ASC
		 FOR UPDATE`,
		productID, warehouseID, bizID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deductions []BatchDeduction
	remaining := requiredQty
	for rows.Next() && remaining > 0 {
		var batchID, qty int
		if err = rows.Scan(&batchID, &qty); err != nil {
			return nil, err
		}
		take := qty
		if take > remaining {
			take = remaining
		}
		deductions = append(deductions, BatchDeduction{BatchID: batchID, Quantity: take})
		remaining -= take
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	if remaining > 0 && len(deductions) > 0 {
		return nil, fmt.Errorf("insufficient batch stock: need %d more units from valid batches", remaining)
	}
	return deductions, nil
}

// DeductBatchTx deducts qty from a specific batch within a transaction.
// Marks batch as 'consumed' when quantity reaches 0.
func (s *BatchStore) DeductBatchTx(tx *sql.Tx, batchID, qty int, changeType, refType string, refID int, note string) error {
	var before int
	if err := tx.QueryRow(
		`SELECT quantity FROM batches WHERE id=? FOR UPDATE`, batchID,
	).Scan(&before); err != nil {
		return err
	}
	after := before - qty
	if after < 0 {
		return fmt.Errorf("batch %d has only %d units, cannot deduct %d", batchID, before, qty)
	}

	status := "active"
	if after == 0 {
		status = "consumed"
	}
	if _, err := tx.Exec(
		`UPDATE batches SET quantity=?, status=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		after, status, batchID,
	); err != nil {
		return err
	}

	var productID, warehouseID, bizID int
	_ = tx.QueryRow(`SELECT product_id, warehouse_id, business_id FROM batches WHERE id=?`, batchID).
		Scan(&productID, &warehouseID, &bizID)

	_, _ = tx.Exec(
		`INSERT INTO batch_logs (batch_id, product_id, warehouse_id, business_id, change_type, qty_before, qty_change, qty_after, ref_type, ref_id, note)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		batchID, productID, warehouseID, bizID, changeType, before, -qty, after, refType, refID, note,
	)
	return nil
}

// AddToBatchTx adds qty to a batch within a transaction (for returns).
func (s *BatchStore) AddToBatchTx(tx *sql.Tx, batchID, qty int, changeType, note string) error {
	var before int
	if err := tx.QueryRow(`SELECT quantity FROM batches WHERE id=? FOR UPDATE`, batchID).Scan(&before); err != nil {
		return err
	}
	after := before + qty
	if _, err := tx.Exec(
		`UPDATE batches SET quantity=?, status='active', updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		after, batchID,
	); err != nil {
		return err
	}
	var productID, warehouseID, bizID int
	_ = tx.QueryRow(`SELECT product_id, warehouse_id, business_id FROM batches WHERE id=?`, batchID).
		Scan(&productID, &warehouseID, &bizID)

	_, _ = tx.Exec(
		`INSERT INTO batch_logs (batch_id, product_id, warehouse_id, business_id, change_type, qty_before, qty_change, qty_after, note)
		 VALUES (?,?,?,?,?,?,?,?,?)`,
		batchID, productID, warehouseID, bizID, changeType, before, qty, after, note,
	)
	return nil
}

// WriteOffExpiredBatches marks all expired batches as 'expired' status and returns count.
// CreateBatchInTx inserts a batch record within a caller-managed transaction.
// Called by the GRN service to create batches inside the same stock-update TX.
func (s *BatchStore) CreateBatchInTx(tx *sql.Tx, b *Batch) (int64, error) {
	if b.BatchNumber == "" {
		b.BatchNumber = fmt.Sprintf("BATCH-%d-%d", b.ProductID, time.Now().Unix())
	}
	res, err := tx.Exec(
		`INSERT INTO batches (business_id, product_id, warehouse_id, batch_number, lot_number,
		 mfg_date, expiry_date, quantity, status, notes)
		 VALUES (?,?,?,?,?,?,?,?,'active',?)`,
		b.BusinessID, b.ProductID, b.WarehouseID, b.BatchNumber, b.LotNumber,
		b.MfgDate, b.ExpiryDate, b.Quantity, b.Notes,
	)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	_, _ = tx.Exec(
		`INSERT INTO batch_logs (batch_id, product_id, warehouse_id, business_id, change_type, qty_before, qty_change, qty_after, ref_type, note)
		 VALUES (?,?,?,?,'purchase_in',0,?,?,'grn','Received via GRN')`,
		id, b.ProductID, b.WarehouseID, b.BusinessID, b.Quantity, b.Quantity,
	)
	return id, nil
}

func (s *BatchStore) WriteOffExpiredBatches(bizID int) (int, error) {
	res, err := s.db.Exec(
		`UPDATE batches SET status='expired', updated_at=CURRENT_TIMESTAMP
		 WHERE business_id=? AND status='active' AND expiry_date < CURDATE() AND quantity > 0`,
		bizID,
	)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// ── Expiry stats ──────────────────────────────────────────────────────────────

func (s *BatchStore) ExpiryStats(bizID, alertDays int) (ExpiryStats, error) {
	var stats ExpiryStats
	stats.AlertDays = alertDays

	err := s.db.QueryRow(
		`SELECT
			COUNT(CASE WHEN expiry_date < CURDATE() AND status IN ('active','expired') THEN 1 END),
			COALESCE(SUM(CASE WHEN expiry_date < CURDATE() AND status IN ('active','expired') THEN quantity ELSE 0 END),0),
			COUNT(CASE WHEN expiry_date >= CURDATE() AND expiry_date <= DATE_ADD(CURDATE(), INTERVAL ? DAY) AND status='active' THEN 1 END),
			COALESCE(SUM(CASE WHEN expiry_date >= CURDATE() AND expiry_date <= DATE_ADD(CURDATE(), INTERVAL ? DAY) AND status='active' THEN quantity ELSE 0 END),0)
		 FROM batches
		 WHERE business_id=? AND quantity > 0`,
		alertDays, alertDays, bizID,
	).Scan(&stats.ExpiredCount, &stats.ExpiredQty, &stats.ExpiringCount, &stats.ExpiringQty)
	return stats, err
}

func (s *BatchStore) ExpiringList(bizID, alertDays int) ([]Batch, error) {
	rows, err := s.db.Query(
		`SELECT`+batchJoinCols+`
		 FROM batches b
		 LEFT JOIN products   p ON p.id = b.product_id
		 LEFT JOIN warehouses w ON w.id = b.warehouse_id AND w.business_id = b.business_id
		 WHERE b.business_id=? AND b.quantity > 0 AND b.status='active'
		   AND b.expiry_date IS NOT NULL
		   AND b.expiry_date <= DATE_ADD(CURDATE(), INTERVAL ? DAY)
		 ORDER BY b.expiry_date ASC`,
		bizID, alertDays,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Batch
	for rows.Next() {
		b, err := scanBatch(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *b)
	}
	return out, rows.Err()
}

func (s *BatchStore) ExpiredList(bizID int) ([]Batch, error) {
	rows, err := s.db.Query(
		`SELECT`+batchJoinCols+`
		 FROM batches b
		 LEFT JOIN products   p ON p.id = b.product_id
		 LEFT JOIN warehouses w ON w.id = b.warehouse_id AND w.business_id = b.business_id
		 WHERE b.business_id=? AND b.quantity > 0
		   AND b.expiry_date < CURDATE() AND b.status != 'consumed'
		 ORDER BY b.expiry_date ASC`,
		bizID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Batch
	for rows.Next() {
		b, err := scanBatch(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *b)
	}
	return out, rows.Err()
}

func (s *BatchStore) BatchLogs(bizID, productID int, limit int) ([]BatchLog, error) {
	rows, err := s.db.Query(
		`SELECT bl.id, bl.batch_id, COALESCE(b.batch_number,''),
		        bl.product_id, COALESCE(p.name,''),
		        bl.warehouse_id, bl.business_id,
		        bl.change_type, bl.qty_before, bl.qty_change, bl.qty_after,
		        bl.ref_type, bl.ref_id, bl.note, bl.created_at
		 FROM batch_logs bl
		 LEFT JOIN batches  b ON b.id = bl.batch_id
		 LEFT JOIN products p ON p.id = bl.product_id
		 WHERE bl.business_id=? AND (? = 0 OR bl.product_id=?)
		 ORDER BY bl.created_at DESC LIMIT ?`,
		bizID, productID, productID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BatchLog
	for rows.Next() {
		var bl BatchLog
		if err = rows.Scan(
			&bl.ID, &bl.BatchID, &bl.BatchNumber,
			&bl.ProductID, &bl.ProductName,
			&bl.WarehouseID, &bl.BusinessID,
			&bl.ChangeType, &bl.QtyBefore, &bl.QtyChange, &bl.QtyAfter,
			&bl.RefType, &bl.RefID, &bl.Note, &bl.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, bl)
	}
	return out, rows.Err()
}

// ── Sales Returns ─────────────────────────────────────────────────────────────

func (s *BatchStore) ListSalesReturns(bizID, limit int) ([]SalesReturn, error) {
	rows, err := s.db.Query(
		`SELECT r.id, r.business_id, r.warehouse_id, COALESCE(w.name,''),
		        r.return_number, r.original_sale_id,
		        r.customer_name, r.customer_phone, r.return_reason,
		        r.total_amount, r.status, r.created_at
		 FROM sales_returns r
		 LEFT JOIN warehouses w ON w.id = r.warehouse_id AND w.business_id = r.business_id
		 WHERE r.business_id=?
		 ORDER BY r.created_at DESC LIMIT ?`,
		bizID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SalesReturn
	for rows.Next() {
		var r SalesReturn
		if err = rows.Scan(
			&r.ID, &r.BusinessID, &r.WarehouseID, &r.WarehouseName,
			&r.ReturnNumber, &r.OriginalSaleID,
			&r.CustomerName, &r.CustomerPhone, &r.ReturnReason,
			&r.TotalAmount, &r.Status, &r.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *BatchStore) CreateSalesReturnTx(tx *sql.Tx, r *SalesReturn) (int64, error) {
	res, err := tx.Exec(
		`INSERT INTO sales_returns
		 (business_id, warehouse_id, return_number, original_sale_id,
		  customer_name, customer_phone, return_reason, total_amount, status)
		 VALUES (?,?,?,?,?,?,?,?,'completed')`,
		r.BusinessID, r.WarehouseID, r.ReturnNumber, r.OriginalSaleID,
		r.CustomerName, r.CustomerPhone, r.ReturnReason, r.TotalAmount,
	)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()

	for _, item := range r.Items {
		if _, err = tx.Exec(
			`INSERT INTO sales_return_items
			 (return_id, product_id, batch_id, product_name, sku, quantity, unit_price, line_total, item_condition, notes)
			 VALUES (?,?,?,?,?,?,?,?,?,?)`,
			id, item.ProductID, item.BatchID, item.ProductName, item.SKU,
			item.Quantity, item.UnitPrice, item.LineTotal, item.Condition, item.Notes,
		); err != nil {
			return 0, err
		}
	}
	return id, nil
}

// ── Purchase Returns ─────────────────────────────────────────────────────────

func (s *BatchStore) ListPurchaseReturns(bizID, limit int) ([]PurchaseReturn, error) {
	rows, err := s.db.Query(
		`SELECT r.id, r.business_id, r.warehouse_id, COALESCE(w.name,''),
		        r.return_number, r.vendor_name, r.return_reason,
		        r.total_amount, r.status, r.created_at
		 FROM purchase_returns r
		 LEFT JOIN warehouses w ON w.id = r.warehouse_id AND w.business_id = r.business_id
		 WHERE r.business_id=?
		 ORDER BY r.created_at DESC LIMIT ?`,
		bizID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PurchaseReturn
	for rows.Next() {
		var r PurchaseReturn
		if err = rows.Scan(
			&r.ID, &r.BusinessID, &r.WarehouseID, &r.WarehouseName,
			&r.ReturnNumber, &r.VendorName, &r.ReturnReason,
			&r.TotalAmount, &r.Status, &r.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *BatchStore) CreatePurchaseReturnTx(tx *sql.Tx, r *PurchaseReturn) (int64, error) {
	res, err := tx.Exec(
		`INSERT INTO purchase_returns
		 (business_id, warehouse_id, return_number, vendor_name, return_reason, total_amount, status)
		 VALUES (?,?,?,?,?,?,'completed')`,
		r.BusinessID, r.WarehouseID, r.ReturnNumber, r.VendorName, r.ReturnReason, r.TotalAmount,
	)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()

	for _, item := range r.Items {
		if _, err = tx.Exec(
			`INSERT INTO purchase_return_items
			 (return_id, product_id, batch_id, product_name, sku, quantity, unit_price, line_total, notes)
			 VALUES (?,?,?,?,?,?,?,?,?)`,
			id, item.ProductID, item.BatchID, item.ProductName, item.SKU,
			item.Quantity, item.UnitPrice, item.LineTotal, item.Notes,
		); err != nil {
			return 0, err
		}
	}
	return id, nil
}

func (s *BatchStore) NextReturnNumber(bizID int, prefix string) string {
	var count int
	table := "sales_returns"
	if prefix == "PUR-RET" {
		table = "purchase_returns"
	}
	_ = s.db.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE business_id=?`, table), bizID).Scan(&count)
	return fmt.Sprintf("%s-%s-%04d", prefix, time.Now().Format("20060102"), count+1)
}
