package models

import (
	"database/sql"
	"errors"
	"time"
)

type Product struct {
	ID                int
	BusinessID        int
	Name              string
	Description       string
	SKU               string
	Barcode           string
	HSNCode           string
	Brand             string
	Unit              string
	Status            string
	Price             float64
	CostPrice         float64
	Stock             int
	TaxRate           float64
	LowStockThreshold int
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type ProductStore struct {
	db *sql.DB
}

func NewProductStore(db *sql.DB) *ProductStore {
	return &ProductStore{db: db}
}

func (s *ProductStore) Migrate() error {
	tables := []string{
		`CREATE TABLE IF NOT EXISTS products (
			id                  INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			business_id         INT NOT NULL DEFAULT 0,
			name                TEXT NOT NULL,
			description         TEXT NOT NULL DEFAULT '',
			sku                 VARCHAR(255) NOT NULL DEFAULT '',
			barcode             VARCHAR(255) NOT NULL DEFAULT '',
			hsn_code            VARCHAR(50) NOT NULL DEFAULT '',
			brand               VARCHAR(100) NOT NULL DEFAULT '',
			unit                VARCHAR(50) NOT NULL DEFAULT 'pcs',
			status              VARCHAR(20) NOT NULL DEFAULT 'active',
			price               DOUBLE NOT NULL DEFAULT 0,
			cost_price          DOUBLE NOT NULL DEFAULT 0,
			stock               INT NOT NULL DEFAULT 0,
			tax_rate            DOUBLE NOT NULL DEFAULT 0,
			low_stock_threshold INT NOT NULL DEFAULT 10,
			created_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS stock_logs (
			id               INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			product_id       INT NOT NULL,
			warehouse_id     INT NOT NULL DEFAULT 0,
			change_type      VARCHAR(50) NOT NULL,
			quantity_before  INT NOT NULL,
			quantity_change  INT NOT NULL,
			quantity_after   INT NOT NULL,
			note             TEXT NOT NULL DEFAULT '',
			created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
	}
	for _, q := range tables {
		if _, err := s.db.Exec(q); err != nil {
			return err
		}
	}

	// Idempotent ALTER TABLE for existing deployments
	alters := []string{
		`ALTER TABLE products ADD COLUMN business_id INT NOT NULL DEFAULT 0`,
		`ALTER TABLE products ADD COLUMN barcode VARCHAR(255) NOT NULL DEFAULT ''`,
		`ALTER TABLE products ADD COLUMN hsn_code VARCHAR(50) NOT NULL DEFAULT ''`,
		`ALTER TABLE products ADD COLUMN brand VARCHAR(100) NOT NULL DEFAULT ''`,
		`ALTER TABLE products ADD COLUMN unit VARCHAR(50) NOT NULL DEFAULT 'pcs'`,
		`ALTER TABLE products ADD COLUMN status VARCHAR(20) NOT NULL DEFAULT 'active'`,
		`ALTER TABLE stock_logs ADD COLUMN warehouse_id INT NOT NULL DEFAULT 0`,
	}
	for _, a := range alters {
		_, _ = s.db.Exec(a)
	}

	for _, idx := range []string{
		`CREATE INDEX idx_products_biz       ON products(business_id)`,
		`CREATE INDEX idx_products_low_stock ON products(business_id, stock, low_stock_threshold)`,
		`CREATE INDEX idx_products_status    ON products(business_id, status)`,
		`CREATE INDEX idx_products_sku       ON products(business_id, sku)`,
		`CREATE INDEX idx_stock_logs_product ON stock_logs(product_id)`,
	} {
		_, _ = s.db.Exec(idx)
	}
	return nil
}

const productCols = `id, business_id, name, description, sku, barcode, hsn_code, brand, unit, status, price, cost_price, stock, tax_rate, low_stock_threshold, created_at, updated_at`

func scanProduct(row interface{ Scan(...any) error }) (*Product, error) {
	var p Product
	return &p, row.Scan(
		&p.ID, &p.BusinessID, &p.Name, &p.Description, &p.SKU,
		&p.Barcode, &p.HSNCode, &p.Brand, &p.Unit, &p.Status,
		&p.Price, &p.CostPrice, &p.Stock, &p.TaxRate, &p.LowStockThreshold,
		&p.CreatedAt, &p.UpdatedAt,
	)
}

func (s *ProductStore) List(search string, businessID int) ([]Product, error) {
	var query string
	var args []any
	if search == "" {
		query = `SELECT ` + productCols + ` FROM products WHERE business_id = ? ORDER BY name ASC`
		args = []any{businessID}
	} else {
		query = `SELECT ` + productCols + ` FROM products WHERE business_id = ? AND (name LIKE ? OR sku LIKE ? OR barcode LIKE ? OR brand LIKE ?) ORDER BY name ASC`
		like := "%" + search + "%"
		args = []any{businessID, like, like, like, like}
	}
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Product
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

func (s *ProductStore) Get(id, businessID int) (*Product, error) {
	row := s.db.QueryRow(
		`SELECT `+productCols+` FROM products WHERE id = ? AND business_id = ?`,
		id, businessID,
	)
	return scanProduct(row)
}

func (s *ProductStore) GetByBarcode(barcode string, businessID int) (*Product, error) {
	row := s.db.QueryRow(
		`SELECT `+productCols+` FROM products WHERE barcode = ? AND business_id = ? LIMIT 1`,
		barcode, businessID,
	)
	return scanProduct(row)
}

func (s *ProductStore) Count(businessID int) (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM products WHERE business_id = ?`, businessID).Scan(&count)
	return count, err
}

func (s *ProductStore) LowStockCount(businessID int) (int, error) {
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM products WHERE business_id = ? AND stock <= low_stock_threshold AND status='active'`,
		businessID,
	).Scan(&count)
	return count, err
}

func (s *ProductStore) LowStock(limit, businessID int) ([]Product, error) {
	rows, err := s.db.Query(
		`SELECT `+productCols+` FROM products WHERE business_id = ? AND stock <= low_stock_threshold AND status='active' ORDER BY stock ASC LIMIT ?`,
		businessID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Product
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

func (s *ProductStore) TotalStockValue(businessID int) (stockValue, costValue float64, err error) {
	err = s.db.QueryRow(
		`SELECT COALESCE(SUM(price * stock), 0), COALESCE(SUM(cost_price * stock), 0) FROM products WHERE business_id = ?`,
		businessID,
	).Scan(&stockValue, &costValue)
	return
}

func (s *ProductStore) DeadStockCount(businessID int) (int, error) {
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(DISTINCT p.id) FROM products p
		 LEFT JOIN stock_logs sl ON sl.product_id = p.id AND sl.created_at > DATE_SUB(NOW(), INTERVAL 30 DAY)
		 WHERE p.business_id = ? AND p.stock > 0 AND sl.id IS NULL`,
		businessID,
	).Scan(&count)
	return count, err
}

func (s *ProductStore) Create(product Product) (*Product, error) {
	if product.Status == "" {
		product.Status = "active"
	}
	if product.Unit == "" {
		product.Unit = "pcs"
	}
	res, err := s.db.Exec(
		`INSERT INTO products (business_id, name, description, sku, barcode, hsn_code, brand, unit, status, price, cost_price, stock, tax_rate, low_stock_threshold)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		product.BusinessID, product.Name, product.Description, product.SKU,
		product.Barcode, product.HSNCode, product.Brand, product.Unit, product.Status,
		product.Price, product.CostPrice, product.Stock, product.TaxRate, product.LowStockThreshold,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.Get(int(id), product.BusinessID)
}

func (s *ProductStore) Update(product Product) error {
	if product.Unit == "" {
		product.Unit = "pcs"
	}
	if product.Status == "" {
		product.Status = "active"
	}
	_, err := s.db.Exec(
		`UPDATE products SET name=?, description=?, barcode=?, hsn_code=?, brand=?, unit=?, status=?,
		 price=?, cost_price=?, tax_rate=?, low_stock_threshold=?, updated_at=CURRENT_TIMESTAMP
		 WHERE id=? AND business_id=?`,
		product.Name, product.Description, product.Barcode, product.HSNCode, product.Brand,
		product.Unit, product.Status, product.Price, product.CostPrice,
		product.TaxRate, product.LowStockThreshold, product.ID, product.BusinessID,
	)
	return err
}

func (s *ProductStore) Delete(id, businessID int) error {
	_, err := s.db.Exec(`DELETE FROM products WHERE id = ? AND business_id = ?`, id, businessID)
	return err
}

func (s *ProductStore) AdjustStock(id, businessID int, changeType string, quantityChange int, note string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var before int
	if err := tx.QueryRow(
		`SELECT stock FROM products WHERE id = ? AND business_id = ?`, id, businessID,
	).Scan(&before); err != nil {
		return err
	}

	after := before + quantityChange
	if after < 0 {
		return errors.New("insufficient stock")
	}
	if _, err := tx.Exec(
		`UPDATE products SET stock=?, updated_at=CURRENT_TIMESTAMP WHERE id=? AND business_id=?`,
		after, id, businessID,
	); err != nil {
		return err
	}
	if _, err := tx.Exec(
		`INSERT INTO stock_logs (product_id, change_type, quantity_before, quantity_change, quantity_after, note) VALUES (?, ?, ?, ?, ?, ?)`,
		id, changeType, before, quantityChange, after, note,
	); err != nil {
		return err
	}
	return tx.Commit()
}
