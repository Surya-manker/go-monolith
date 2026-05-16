package models

import (
	"database/sql"
	"errors"
	"time"
)

type Product struct {
	ID                int
	Name              string
	Description       string
	SKU               string
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
			id              INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			name            TEXT NOT NULL,
			description     TEXT NOT NULL DEFAULT '',
			sku             VARCHAR(255) NOT NULL UNIQUE,
			price           DOUBLE NOT NULL,
			cost_price      DOUBLE NOT NULL DEFAULT 0,
			stock           INT NOT NULL DEFAULT 0,
			tax_rate        DOUBLE NOT NULL DEFAULT 0,
			low_stock_threshold INT NOT NULL DEFAULT 10,
			created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS stock_logs (
			id               INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			product_id       INT NOT NULL,
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
	// Indexes — ignore errors (already exists on re-run).
	for _, idx := range []string{
		`CREATE INDEX idx_products_sku       ON products(sku(191))`,
		`CREATE INDEX idx_products_low_stock ON products(stock, low_stock_threshold)`,
		`CREATE INDEX idx_stock_logs_product ON stock_logs(product_id)`,
		`CREATE INDEX idx_stock_logs_created ON stock_logs(created_at)`,
	} {
		_, _ = s.db.Exec(idx)
	}
	return nil
}

func (s *ProductStore) List(search string) ([]Product, error) {
	const base = `SELECT id, name, description, sku, price, cost_price, stock, tax_rate, low_stock_threshold, created_at, updated_at FROM products`
	var query string
	var args []any
	if search == "" {
		query = base + " ORDER BY id DESC"
	} else {
		query = base + " WHERE name LIKE ? OR sku LIKE ? ORDER BY id DESC"
		like := "%" + search + "%"
		args = []any{like, like}
	}
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var products []Product
	for rows.Next() {
		var product Product
		if err := rows.Scan(
			&product.ID,
			&product.Name,
			&product.Description,
			&product.SKU,
			&product.Price,
			&product.CostPrice,
			&product.Stock,
			&product.TaxRate,
			&product.LowStockThreshold,
			&product.CreatedAt,
			&product.UpdatedAt,
		); err != nil {
			return nil, err
		}
		products = append(products, product)
	}
	return products, rows.Err()
}

func (s *ProductStore) Get(id int) (*Product, error) {
	var product Product
	err := s.db.QueryRow(`
		SELECT id, name, description, sku, price, cost_price, stock, tax_rate, low_stock_threshold, created_at, updated_at
		FROM products
		WHERE id = ?
	`, id).Scan(
		&product.ID,
		&product.Name,
		&product.Description,
		&product.SKU,
		&product.Price,
		&product.CostPrice,
		&product.Stock,
		&product.TaxRate,
		&product.LowStockThreshold,
		&product.CreatedAt,
		&product.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &product, nil
}

func (s *ProductStore) Count() (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM products`).Scan(&count)
	return count, err
}

func (s *ProductStore) LowStockCount() (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM products WHERE stock <= low_stock_threshold`).Scan(&count)
	return count, err
}

func (s *ProductStore) LowStock(limit int) ([]Product, error) {
	rows, err := s.db.Query(`
		SELECT id, name, description, sku, price, cost_price, stock, tax_rate, low_stock_threshold, created_at, updated_at
		FROM products
		WHERE stock <= low_stock_threshold
		ORDER BY stock ASC, name ASC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var products []Product
	for rows.Next() {
		var product Product
		if err := rows.Scan(
			&product.ID,
			&product.Name,
			&product.Description,
			&product.SKU,
			&product.Price,
			&product.CostPrice,
			&product.Stock,
			&product.TaxRate,
			&product.LowStockThreshold,
			&product.CreatedAt,
			&product.UpdatedAt,
		); err != nil {
			return nil, err
		}
		products = append(products, product)
	}
	return products, rows.Err()
}

func (s *ProductStore) Create(product Product) (*Product, error) {
	result, err := s.db.Exec(`
		INSERT INTO products (name, description, sku, price, cost_price, stock, tax_rate, low_stock_threshold)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, product.Name, product.Description, product.SKU, product.Price, product.CostPrice, product.Stock, product.TaxRate, product.LowStockThreshold)
	if err != nil {
		return nil, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	return s.Get(int(id))
}

func (s *ProductStore) Update(product Product) error {
	_, err := s.db.Exec(`
		UPDATE products
		SET name = ?, description = ?, price = ?, cost_price = ?, tax_rate = ?, low_stock_threshold = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, product.Name, product.Description, product.Price, product.CostPrice, product.TaxRate, product.LowStockThreshold, product.ID)
	return err
}

func (s *ProductStore) Delete(id int) error {
	_, err := s.db.Exec(`DELETE FROM products WHERE id = ?`, id)
	return err
}

func (s *ProductStore) AdjustStock(id int, changeType string, quantityChange int, note string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var before int
	if err := tx.QueryRow(`SELECT stock FROM products WHERE id = ?`, id).Scan(&before); err != nil {
		return err
	}

	after := before + quantityChange
	if after < 0 {
		return errors.New("insufficient stock")
	}

	if _, err := tx.Exec(`UPDATE products SET stock = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, after, id); err != nil {
		return err
	}

	if _, err := tx.Exec(`
		INSERT INTO stock_logs (product_id, change_type, quantity_before, quantity_change, quantity_after, note)
		VALUES (?, ?, ?, ?, ?, ?)
	`, id, changeType, before, quantityChange, after, note); err != nil {
		return err
	}

	return tx.Commit()
}
