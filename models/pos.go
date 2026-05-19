package models

import (
	"database/sql"
	"fmt"
	"time"
)

// ── Domain types ──────────────────────────────────────────────────────────────

type POSSale struct {
	ID            int
	BusinessID    int
	WarehouseID   int
	WarehouseName string
	SaleNumber    string
	CustomerName  string
	CustomerPhone string
	Subtotal      float64
	Discount      float64
	TaxTotal      float64
	GrandTotal    float64
	PaymentMethod string
	AmountPaid    float64
	ChangeGiven   float64
	Status        string
	ItemCount     int
	CreatedAt     time.Time
	Items         []POSSaleItem
}

type POSSaleItem struct {
	ID          int
	SaleID      int
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

// ── Store ─────────────────────────────────────────────────────────────────────

type POSStore struct {
	db *sql.DB
}

func NewPOSStore(db *sql.DB) *POSStore {
	return &POSStore{db: db}
}

func (s *POSStore) Migrate() error {
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS pos_sales (
		id             INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
		business_id    INT NOT NULL DEFAULT 0,
		warehouse_id   INT NOT NULL DEFAULT 0,
		sale_number    VARCHAR(50) NOT NULL DEFAULT '',
		customer_name  VARCHAR(255) NOT NULL DEFAULT '',
		customer_phone VARCHAR(20) NOT NULL DEFAULT '',
		subtotal       DECIMAL(12,2) NOT NULL DEFAULT 0,
		discount       DECIMAL(12,2) NOT NULL DEFAULT 0,
		tax_total      DECIMAL(12,2) NOT NULL DEFAULT 0,
		grand_total    DECIMAL(12,2) NOT NULL DEFAULT 0,
		payment_method VARCHAR(20) NOT NULL DEFAULT 'cash',
		amount_paid    DECIMAL(12,2) NOT NULL DEFAULT 0,
		change_given   DECIMAL(12,2) NOT NULL DEFAULT 0,
		status         VARCHAR(20) NOT NULL DEFAULT 'completed',
		created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`CREATE TABLE IF NOT EXISTS pos_sale_items (
		id           INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
		sale_id      INT NOT NULL,
		product_id   INT NOT NULL DEFAULT 0,
		product_name VARCHAR(255) NOT NULL DEFAULT '',
		sku          VARCHAR(255) NOT NULL DEFAULT '',
		quantity     INT NOT NULL DEFAULT 1,
		unit_price   DECIMAL(12,2) NOT NULL DEFAULT 0,
		tax_rate     DECIMAL(5,2) NOT NULL DEFAULT 0,
		tax_amount   DECIMAL(12,2) NOT NULL DEFAULT 0,
		discount     DECIMAL(12,2) NOT NULL DEFAULT 0,
		line_total   DECIMAL(12,2) NOT NULL DEFAULT 0
	)`)
	if err != nil {
		return err
	}
	for _, idx := range []string{
		`CREATE INDEX idx_pos_sales_biz    ON pos_sales(business_id)`,
		`CREATE INDEX idx_pos_sales_date   ON pos_sales(business_id, created_at)`,
		`CREATE INDEX idx_pos_items_sale   ON pos_sale_items(sale_id)`,
		`CREATE INDEX idx_pos_items_prod   ON pos_sale_items(product_id)`,
	} {
		_, _ = s.db.Exec(idx)
	}
	return nil
}

// CreateSaleTx inserts the sale header + all items within the caller's transaction.
// Callers must begin and commit the transaction themselves.
func (s *POSStore) CreateSaleTx(tx *sql.Tx, sale *POSSale) (int64, error) {
	res, err := tx.Exec(
		`INSERT INTO pos_sales
		 (business_id, warehouse_id, sale_number, customer_name, customer_phone,
		  subtotal, discount, tax_total, grand_total,
		  payment_method, amount_paid, change_given, status)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,'completed')`,
		sale.BusinessID, sale.WarehouseID, sale.SaleNumber,
		sale.CustomerName, sale.CustomerPhone,
		sale.Subtotal, sale.Discount, sale.TaxTotal, sale.GrandTotal,
		sale.PaymentMethod, sale.AmountPaid, sale.ChangeGiven,
	)
	if err != nil {
		return 0, err
	}
	saleID, _ := res.LastInsertId()

	for _, item := range sale.Items {
		if _, err = tx.Exec(
			`INSERT INTO pos_sale_items
			 (sale_id, product_id, product_name, sku, quantity, unit_price, tax_rate, tax_amount, discount, line_total)
			 VALUES (?,?,?,?,?,?,?,?,?,?)`,
			saleID, item.ProductID, item.ProductName, item.SKU,
			item.Quantity, item.UnitPrice, item.TaxRate, item.TaxAmount, item.Discount, item.LineTotal,
		); err != nil {
			return 0, err
		}
	}
	return saleID, nil
}

// NextSaleNumber returns the next sequential sale number for the business (best-effort).
func (s *POSStore) NextSaleNumber(bizID int) string {
	var count int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM pos_sales WHERE business_id=?`, bizID).Scan(&count)
	return fmt.Sprintf("POS-%s-%04d", time.Now().Format("20060102"), count+1)
}

func (s *POSStore) List(bizID int, limit int) ([]POSSale, error) {
	rows, err := s.db.Query(
		`SELECT s.id, s.business_id, s.warehouse_id, COALESCE(w.name,''),
		        s.sale_number, s.customer_name, s.customer_phone,
		        s.subtotal, s.discount, s.tax_total, s.grand_total,
		        s.payment_method, s.amount_paid, s.change_given, s.status,
		        (SELECT COUNT(*) FROM pos_sale_items WHERE sale_id=s.id), s.created_at
		 FROM pos_sales s
		 LEFT JOIN warehouses w ON w.id = s.warehouse_id AND w.business_id = s.business_id
		 WHERE s.business_id=?
		 ORDER BY s.created_at DESC
		 LIMIT ?`,
		bizID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []POSSale
	for rows.Next() {
		var s POSSale
		if err = rows.Scan(
			&s.ID, &s.BusinessID, &s.WarehouseID, &s.WarehouseName,
			&s.SaleNumber, &s.CustomerName, &s.CustomerPhone,
			&s.Subtotal, &s.Discount, &s.TaxTotal, &s.GrandTotal,
			&s.PaymentMethod, &s.AmountPaid, &s.ChangeGiven, &s.Status,
			&s.ItemCount, &s.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (s *POSStore) Get(id, bizID int) (*POSSale, error) {
	var sale POSSale
	err := s.db.QueryRow(
		`SELECT s.id, s.business_id, s.warehouse_id, COALESCE(w.name,''),
		        s.sale_number, s.customer_name, s.customer_phone,
		        s.subtotal, s.discount, s.tax_total, s.grand_total,
		        s.payment_method, s.amount_paid, s.change_given, s.status,
		        (SELECT COUNT(*) FROM pos_sale_items WHERE sale_id=s.id), s.created_at
		 FROM pos_sales s
		 LEFT JOIN warehouses w ON w.id = s.warehouse_id AND w.business_id = s.business_id
		 WHERE s.id=? AND s.business_id=?`,
		id, bizID,
	).Scan(
		&sale.ID, &sale.BusinessID, &sale.WarehouseID, &sale.WarehouseName,
		&sale.SaleNumber, &sale.CustomerName, &sale.CustomerPhone,
		&sale.Subtotal, &sale.Discount, &sale.TaxTotal, &sale.GrandTotal,
		&sale.PaymentMethod, &sale.AmountPaid, &sale.ChangeGiven, &sale.Status,
		&sale.ItemCount, &sale.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	rows, err := s.db.Query(
		`SELECT id, sale_id, product_id, product_name, sku, quantity,
		        unit_price, tax_rate, tax_amount, discount, line_total
		 FROM pos_sale_items WHERE sale_id=? ORDER BY id`,
		sale.ID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var item POSSaleItem
		if err = rows.Scan(
			&item.ID, &item.SaleID, &item.ProductID, &item.ProductName, &item.SKU,
			&item.Quantity, &item.UnitPrice, &item.TaxRate, &item.TaxAmount, &item.Discount, &item.LineTotal,
		); err != nil {
			return nil, err
		}
		sale.Items = append(sale.Items, item)
	}
	return &sale, rows.Err()
}

func (s *POSStore) TodayTotal(bizID int) (count int, total float64, err error) {
	err = s.db.QueryRow(
		`SELECT COUNT(*), COALESCE(SUM(grand_total),0) FROM pos_sales
		 WHERE business_id=? AND DATE(created_at)=CURDATE() AND status='completed'`,
		bizID,
	).Scan(&count, &total)
	return
}
