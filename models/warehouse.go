package models

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"
)

type Warehouse struct {
	ID          int
	BusinessID  int
	Name        string
	Address     string
	ManagerName string
	IsDefault   bool
	CreatedAt   time.Time
}

type WarehouseStock struct {
	WarehouseID   int
	WarehouseName string
	ProductID     int
	ProductName   string
	SKU           string
	Barcode       string
	Unit          string
	BusinessID    int
	Quantity      int
	UpdatedAt     time.Time
}

type StockTransfer struct {
	ID              int
	BusinessID      int
	FromWarehouseID int
	FromWarehouse   string
	ToWarehouseID   int
	ToWarehouse     string
	ProductID       int
	ProductName     string
	SKU             string
	Quantity        int
	Status          string
	Note            string
	CreatedAt       time.Time
}

type WarehouseStore struct {
	db *sql.DB
}

func NewWarehouseStore(db *sql.DB) *WarehouseStore {
	return &WarehouseStore{db: db}
}

func (s *WarehouseStore) Migrate() error {
	tables := []string{
		`CREATE TABLE IF NOT EXISTS warehouses (
			id           INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			business_id  INT NOT NULL DEFAULT 0,
			name         VARCHAR(255) NOT NULL,
			address      TEXT NOT NULL DEFAULT '',
			manager_name VARCHAR(255) NOT NULL DEFAULT '',
			is_default   TINYINT(1) NOT NULL DEFAULT 0,
			created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS warehouse_stock (
			id           INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			warehouse_id INT NOT NULL,
			product_id   INT NOT NULL,
			business_id  INT NOT NULL DEFAULT 0,
			quantity     INT NOT NULL DEFAULT 0,
			updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			UNIQUE KEY uniq_wh_prod (warehouse_id, product_id)
		)`,
		`CREATE TABLE IF NOT EXISTS stock_transfers (
			id                INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			business_id       INT NOT NULL DEFAULT 0,
			from_warehouse_id INT NOT NULL,
			to_warehouse_id   INT NOT NULL,
			product_id        INT NOT NULL,
			quantity          INT NOT NULL,
			status            VARCHAR(20) NOT NULL DEFAULT 'completed',
			note              TEXT NOT NULL DEFAULT '',
			created_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
	}
	for _, q := range tables {
		if _, err := s.db.Exec(q); err != nil {
			return err
		}
	}
	for _, idx := range []string{
		`CREATE INDEX idx_warehouses_biz   ON warehouses(business_id)`,
		`CREATE INDEX idx_wh_stock_biz     ON warehouse_stock(business_id)`,
		`CREATE INDEX idx_wh_stock_prod    ON warehouse_stock(product_id)`,
		`CREATE INDEX idx_wh_stock_wh      ON warehouse_stock(warehouse_id)`,
		`CREATE INDEX idx_transfers_biz    ON stock_transfers(business_id)`,
		`CREATE INDEX idx_transfers_prod   ON stock_transfers(product_id)`,
	} {
		_, _ = s.db.Exec(idx)
	}
	return nil
}

// ── Validation helpers ─────────────────────────────────────────────────────────

// validateWarehouseBelongs checks the warehouse exists and belongs to this business.
func (s *WarehouseStore) validateWarehouseBelongs(tx *sql.Tx, warehouseID, bizID int) error {
	var id int
	err := tx.QueryRow(`SELECT id FROM warehouses WHERE id=? AND business_id=?`, warehouseID, bizID).Scan(&id)
	if err == sql.ErrNoRows {
		return fmt.Errorf("warehouse %d not found", warehouseID)
	}
	return err
}

// validateProductBelongs checks the product exists and belongs to this business.
func (s *WarehouseStore) validateProductBelongs(tx *sql.Tx, productID, bizID int) error {
	var id int
	err := tx.QueryRow(`SELECT id FROM products WHERE id=? AND business_id=?`, productID, bizID).Scan(&id)
	if err == sql.ErrNoRows {
		return fmt.Errorf("product %d not found", productID)
	}
	return err
}

// ── Warehouse CRUD ─────────────────────────────────────────────────────────────

func (s *WarehouseStore) List(bizID int) ([]Warehouse, error) {
	rows, err := s.db.Query(
		`SELECT id, business_id, name, address, manager_name, is_default, created_at
		 FROM warehouses WHERE business_id=? ORDER BY is_default DESC, name ASC`,
		bizID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Warehouse
	for rows.Next() {
		var w Warehouse
		if err := rows.Scan(&w.ID, &w.BusinessID, &w.Name, &w.Address, &w.ManagerName, &w.IsDefault, &w.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func (s *WarehouseStore) Get(id, bizID int) (*Warehouse, error) {
	var w Warehouse
	err := s.db.QueryRow(
		`SELECT id, business_id, name, address, manager_name, is_default, created_at
		 FROM warehouses WHERE id=? AND business_id=?`,
		id, bizID,
	).Scan(&w.ID, &w.BusinessID, &w.Name, &w.Address, &w.ManagerName, &w.IsDefault, &w.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &w, nil
}

// Create wraps the INSERT + default-unset in one transaction to prevent
// two concurrent creates from both being marked as default.
func (s *WarehouseStore) Create(w Warehouse) (*Warehouse, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var count int
	if err = tx.QueryRow(`SELECT COUNT(*) FROM warehouses WHERE business_id=?`, w.BusinessID).Scan(&count); err != nil {
		return nil, err
	}
	if count == 0 {
		w.IsDefault = true
	}

	res, err := tx.Exec(
		`INSERT INTO warehouses (business_id, name, address, manager_name, is_default) VALUES (?,?,?,?,?)`,
		w.BusinessID, w.Name, w.Address, w.ManagerName, w.IsDefault,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()

	if w.IsDefault {
		if _, err = tx.Exec(
			`UPDATE warehouses SET is_default=0 WHERE business_id=? AND id!=?`, w.BusinessID, id,
		); err != nil {
			return nil, err
		}
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return s.Get(int(id), w.BusinessID)
}

// Update wraps the UPDATE + default-unset in one transaction.
func (s *WarehouseStore) Update(w Warehouse) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.Exec(
		`UPDATE warehouses SET name=?, address=?, manager_name=?, is_default=? WHERE id=? AND business_id=?`,
		w.Name, w.Address, w.ManagerName, w.IsDefault, w.ID, w.BusinessID,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("warehouse not found")
	}

	if w.IsDefault {
		if _, err = tx.Exec(
			`UPDATE warehouses SET is_default=0 WHERE business_id=? AND id!=?`, w.BusinessID, w.ID,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *WarehouseStore) Delete(id, bizID int) error {
	var isDefault bool
	if err := s.db.QueryRow(
		`SELECT is_default FROM warehouses WHERE id=? AND business_id=?`, id, bizID,
	).Scan(&isDefault); err != nil {
		if err == sql.ErrNoRows {
			return errors.New("warehouse not found")
		}
		return err
	}
	if isDefault {
		return errors.New("cannot delete the default warehouse — set another as default first")
	}

	var qty int
	if err := s.db.QueryRow(
		`SELECT COALESCE(SUM(quantity), 0) FROM warehouse_stock WHERE warehouse_id=? AND business_id=?`, id, bizID,
	).Scan(&qty); err != nil {
		return err
	}
	if qty > 0 {
		return fmt.Errorf("cannot delete warehouse with %d units of stock — transfer it first", qty)
	}

	res, err := s.db.Exec(`DELETE FROM warehouses WHERE id=? AND business_id=?`, id, bizID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("warehouse not found")
	}
	return nil
}

func (s *WarehouseStore) Count(bizID int) (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM warehouses WHERE business_id=?`, bizID).Scan(&count)
	return count, err
}

// ── Warehouse Stock ────────────────────────────────────────────────────────────

func (s *WarehouseStore) GetWarehouseStock(warehouseID, bizID int) ([]WarehouseStock, error) {
	rows, err := s.db.Query(
		`SELECT ws.warehouse_id, w.name, ws.product_id, p.name, p.sku,
		        COALESCE(p.barcode,''), COALESCE(p.unit,'pcs'),
		        ws.business_id, ws.quantity, ws.updated_at
		 FROM warehouse_stock ws
		 JOIN warehouses w ON w.id = ws.warehouse_id AND w.business_id = ws.business_id
		 JOIN products   p ON p.id = ws.product_id   AND p.business_id = ws.business_id
		 WHERE ws.business_id=? AND ws.warehouse_id=?
		 ORDER BY p.name ASC`,
		bizID, warehouseID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanWarehouseStock(rows)
}

func (s *WarehouseStore) GetAllWarehouseStock(bizID int) ([]WarehouseStock, error) {
	rows, err := s.db.Query(
		`SELECT ws.warehouse_id, w.name, ws.product_id, p.name, p.sku,
		        COALESCE(p.barcode,''), COALESCE(p.unit,'pcs'),
		        ws.business_id, ws.quantity, ws.updated_at
		 FROM warehouse_stock ws
		 JOIN warehouses w ON w.id = ws.warehouse_id AND w.business_id = ws.business_id
		 JOIN products   p ON p.id = ws.product_id   AND p.business_id = ws.business_id
		 WHERE ws.business_id=?
		 ORDER BY w.name ASC, p.name ASC`,
		bizID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanWarehouseStock(rows)
}

func scanWarehouseStock(rows *sql.Rows) ([]WarehouseStock, error) {
	var out []WarehouseStock
	for rows.Next() {
		var ws WarehouseStock
		if err := rows.Scan(
			&ws.WarehouseID, &ws.WarehouseName, &ws.ProductID, &ws.ProductName,
			&ws.SKU, &ws.Barcode, &ws.Unit, &ws.BusinessID, &ws.Quantity, &ws.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, ws)
	}
	return out, rows.Err()
}

// AdjustWarehouseStock adjusts stock for a product in a specific warehouse.
// Uses SELECT FOR UPDATE to prevent race conditions under concurrent requests.
// Validates both warehouse and product belong to the business before mutating.
func (s *WarehouseStore) AdjustWarehouseStock(warehouseID, productID, bizID, delta int, changeType, note string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Validate ownership — both inside the transaction for consistency.
	if err = s.validateWarehouseBelongs(tx, warehouseID, bizID); err != nil {
		return err
	}
	if err = s.validateProductBelongs(tx, productID, bizID); err != nil {
		return err
	}

	// Lock the warehouse_stock row (or gap) before reading to prevent concurrent over-deduction.
	var currentQty int
	scanErr := tx.QueryRow(
		`SELECT quantity FROM warehouse_stock WHERE warehouse_id=? AND product_id=? FOR UPDATE`,
		warehouseID, productID,
	).Scan(&currentQty)

	if scanErr == sql.ErrNoRows {
		if delta < 0 {
			return errors.New("no stock in this warehouse to deduct")
		}
		if _, err = tx.Exec(
			`INSERT INTO warehouse_stock (warehouse_id, product_id, business_id, quantity) VALUES (?,?,?,?)`,
			warehouseID, productID, bizID, delta,
		); err != nil {
			return err
		}
	} else if scanErr != nil {
		return scanErr
	} else {
		newQty := currentQty + delta
		if newQty < 0 {
			return fmt.Errorf("insufficient stock: warehouse has %d, requested %d", currentQty, -delta)
		}
		if _, err = tx.Exec(
			`UPDATE warehouse_stock SET quantity=?, updated_at=CURRENT_TIMESTAMP WHERE warehouse_id=? AND product_id=?`,
			newQty, warehouseID, productID,
		); err != nil {
			return err
		}
	}

	// Re-compute product total WITHIN the transaction and filtered by business_id.
	var totalStock int
	if err = tx.QueryRow(
		`SELECT COALESCE(SUM(quantity), 0) FROM warehouse_stock WHERE product_id=? AND business_id=?`,
		productID, bizID,
	).Scan(&totalStock); err != nil {
		return err
	}

	var before int
	_ = tx.QueryRow(`SELECT stock FROM products WHERE id=? AND business_id=?`, productID, bizID).Scan(&before)

	if _, err = tx.Exec(
		`UPDATE products SET stock=?, updated_at=CURRENT_TIMESTAMP WHERE id=? AND business_id=?`,
		totalStock, productID, bizID,
	); err != nil {
		return err
	}

	if _, err = tx.Exec(
		`INSERT INTO stock_logs (product_id, warehouse_id, change_type, quantity_before, quantity_change, quantity_after, note)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		productID, warehouseID, changeType, before, delta, totalStock, note,
	); err != nil {
		return err
	}

	return tx.Commit()
}

// ── Stock Transfers ────────────────────────────────────────────────────────────

func (s *WarehouseStore) ListTransfers(bizID int) ([]StockTransfer, error) {
	rows, err := s.db.Query(
		`SELECT t.id, t.business_id, t.from_warehouse_id, fw.name, t.to_warehouse_id, tw.name,
		        t.product_id, p.name, p.sku, t.quantity, t.status, t.note, t.created_at
		 FROM stock_transfers t
		 JOIN warehouses fw ON fw.id = t.from_warehouse_id AND fw.business_id = t.business_id
		 JOIN warehouses tw ON tw.id = t.to_warehouse_id   AND tw.business_id = t.business_id
		 JOIN products   p  ON p.id  = t.product_id        AND p.business_id  = t.business_id
		 WHERE t.business_id=?
		 ORDER BY t.created_at DESC`,
		bizID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTransfers(rows)
}

func (s *WarehouseStore) GetTransfer(id, bizID int) (*StockTransfer, error) {
	rows, err := s.db.Query(
		`SELECT t.id, t.business_id, t.from_warehouse_id, fw.name, t.to_warehouse_id, tw.name,
		        t.product_id, p.name, p.sku, t.quantity, t.status, t.note, t.created_at
		 FROM stock_transfers t
		 JOIN warehouses fw ON fw.id = t.from_warehouse_id AND fw.business_id = t.business_id
		 JOIN warehouses tw ON tw.id = t.to_warehouse_id   AND tw.business_id = t.business_id
		 JOIN products   p  ON p.id  = t.product_id        AND p.business_id  = t.business_id
		 WHERE t.id=? AND t.business_id=?`,
		id, bizID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ts, err := scanTransfers(rows)
	if err != nil {
		return nil, err
	}
	if len(ts) == 0 {
		return nil, errors.New("transfer not found")
	}
	return &ts[0], nil
}

func scanTransfers(rows *sql.Rows) ([]StockTransfer, error) {
	var out []StockTransfer
	for rows.Next() {
		var t StockTransfer
		if err := rows.Scan(
			&t.ID, &t.BusinessID, &t.FromWarehouseID, &t.FromWarehouse,
			&t.ToWarehouseID, &t.ToWarehouse,
			&t.ProductID, &t.ProductName, &t.SKU,
			&t.Quantity, &t.Status, &t.Note, &t.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// CreateTransfer moves stock between two warehouses atomically.
// Uses FOR UPDATE locks on both source rows to prevent double-deduction races.
func (s *WarehouseStore) CreateTransfer(t StockTransfer) (*StockTransfer, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Validate ownership.
	if err = s.validateWarehouseBelongs(tx, t.FromWarehouseID, t.BusinessID); err != nil {
		return nil, fmt.Errorf("source: %w", err)
	}
	if err = s.validateWarehouseBelongs(tx, t.ToWarehouseID, t.BusinessID); err != nil {
		return nil, fmt.Errorf("destination: %w", err)
	}
	if err = s.validateProductBelongs(tx, t.ProductID, t.BusinessID); err != nil {
		return nil, err
	}

	// Lock source row to prevent concurrent over-deduction (smaller ID first to avoid deadlock).
	lockOrder := []int{t.FromWarehouseID, t.ToWarehouseID}
	if t.ToWarehouseID < t.FromWarehouseID {
		lockOrder = []int{t.ToWarehouseID, t.FromWarehouseID}
	}
	for _, whID := range lockOrder {
		_, lockErr := tx.Exec(
			`INSERT INTO warehouse_stock (warehouse_id, product_id, business_id, quantity)
			 VALUES (?,?,?,0)
			 ON DUPLICATE KEY UPDATE quantity=quantity`,
			whID, t.ProductID, t.BusinessID,
		)
		if lockErr != nil {
			return nil, lockErr
		}
	}

	// Lock both rows.
	var srcQty int
	if err = tx.QueryRow(
		`SELECT quantity FROM warehouse_stock WHERE warehouse_id=? AND product_id=? FOR UPDATE`,
		t.FromWarehouseID, t.ProductID,
	).Scan(&srcQty); err != nil {
		return nil, err
	}
	if srcQty < t.Quantity {
		return nil, fmt.Errorf("insufficient stock: source warehouse has %d, requested %d", srcQty, t.Quantity)
	}

	var dstQty int
	_ = tx.QueryRow(
		`SELECT quantity FROM warehouse_stock WHERE warehouse_id=? AND product_id=? FOR UPDATE`,
		t.ToWarehouseID, t.ProductID,
	).Scan(&dstQty)

	// Deduct from source.
	if _, err = tx.Exec(
		`UPDATE warehouse_stock SET quantity=quantity-?, updated_at=CURRENT_TIMESTAMP WHERE warehouse_id=? AND product_id=?`,
		t.Quantity, t.FromWarehouseID, t.ProductID,
	); err != nil {
		return nil, err
	}

	// Add to destination.
	if _, err = tx.Exec(
		`UPDATE warehouse_stock SET quantity=quantity+?, updated_at=CURRENT_TIMESTAMP WHERE warehouse_id=? AND product_id=?`,
		t.Quantity, t.ToWarehouseID, t.ProductID,
	); err != nil {
		return nil, err
	}

	// Insert transfer record.
	res, err := tx.Exec(
		`INSERT INTO stock_transfers (business_id, from_warehouse_id, to_warehouse_id, product_id, quantity, status, note)
		 VALUES (?,?,?,?,?,'completed',?)`,
		t.BusinessID, t.FromWarehouseID, t.ToWarehouseID, t.ProductID, t.Quantity, t.Note,
	)
	if err != nil {
		return nil, err
	}

	// Log both movements — failures here should NOT abort the transfer.
	fromNote := "Transfer to warehouse ID " + strconv.Itoa(t.ToWarehouseID)
	toNote := "Transfer from warehouse ID " + strconv.Itoa(t.FromWarehouseID)
	if t.FromWarehouse != "" {
		toNote = "Transfer from " + t.FromWarehouse
	}
	if t.ToWarehouse != "" {
		fromNote = "Transfer to " + t.ToWarehouse
	}
	_, _ = tx.Exec(
		`INSERT INTO stock_logs (product_id, warehouse_id, change_type, quantity_before, quantity_change, quantity_after, note)
		 VALUES (?,?,'transfer_out',?,?,?,?)`,
		t.ProductID, t.FromWarehouseID, srcQty, -t.Quantity, srcQty-t.Quantity, fromNote,
	)
	_, _ = tx.Exec(
		`INSERT INTO stock_logs (product_id, warehouse_id, change_type, quantity_before, quantity_change, quantity_after, note)
		 VALUES (?,?,'transfer_in',?,?,?,?)`,
		t.ProductID, t.ToWarehouseID, dstQty, t.Quantity, dstQty+t.Quantity, toNote,
	)

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	id, _ := res.LastInsertId()
	return s.GetTransfer(int(id), t.BusinessID)
}

// AdjustWarehouseStockTx is the same as AdjustWarehouseStock but operates within
// an existing transaction (for atomic POS checkout). The caller manages BEGIN/COMMIT.
func (s *WarehouseStore) AdjustWarehouseStockTx(tx *sql.Tx, warehouseID, productID, bizID, delta int, changeType, note string) error {
	var currentQty int
	scanErr := tx.QueryRow(
		`SELECT quantity FROM warehouse_stock WHERE warehouse_id=? AND product_id=? FOR UPDATE`,
		warehouseID, productID,
	).Scan(&currentQty)

	if scanErr == sql.ErrNoRows {
		if delta < 0 {
			return fmt.Errorf("no stock in warehouse for product %d", productID)
		}
		if _, err := tx.Exec(
			`INSERT INTO warehouse_stock (warehouse_id, product_id, business_id, quantity) VALUES (?,?,?,?)`,
			warehouseID, productID, bizID, delta,
		); err != nil {
			return err
		}
	} else if scanErr != nil {
		return scanErr
	} else {
		newQty := currentQty + delta
		if newQty < 0 {
			return fmt.Errorf("insufficient stock: warehouse has %d, need %d", currentQty, -delta)
		}
		if _, err := tx.Exec(
			`UPDATE warehouse_stock SET quantity=?, updated_at=CURRENT_TIMESTAMP WHERE warehouse_id=? AND product_id=?`,
			newQty, warehouseID, productID,
		); err != nil {
			return err
		}
	}

	var totalStock int
	_ = tx.QueryRow(
		`SELECT COALESCE(SUM(quantity),0) FROM warehouse_stock WHERE product_id=? AND business_id=?`,
		productID, bizID,
	).Scan(&totalStock)

	var before int
	_ = tx.QueryRow(`SELECT stock FROM products WHERE id=? AND business_id=?`, productID, bizID).Scan(&before)

	if _, err := tx.Exec(
		`UPDATE products SET stock=?, updated_at=CURRENT_TIMESTAMP WHERE id=? AND business_id=?`,
		totalStock, productID, bizID,
	); err != nil {
		return err
	}

	_, _ = tx.Exec(
		`INSERT INTO stock_logs (product_id, warehouse_id, change_type, quantity_before, quantity_change, quantity_after, note)
		 VALUES (?,?,?,?,?,?,?)`,
		productID, warehouseID, changeType, before, delta, totalStock, note,
	)
	return nil
}

func (s *WarehouseStore) CountTransfers(bizID int) (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM stock_transfers WHERE business_id=?`, bizID).Scan(&count)
	return count, err
}
