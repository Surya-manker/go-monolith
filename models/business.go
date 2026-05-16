package models

import (
	"database/sql"
	"fmt"
	"strings"
)

type ModuleStore struct {
	db *sql.DB
}

type Record map[string]string

func NewModuleStore(db *sql.DB) *ModuleStore {
	return &ModuleStore{db: db}
}

func (s *ModuleStore) Migrate() error {
	tables := []string{
		`CREATE TABLE IF NOT EXISTS customers (
			id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			name TEXT NOT NULL, email VARCHAR(255) NOT NULL DEFAULT '',
			phone VARCHAR(50) NOT NULL DEFAULT '', gstin VARCHAR(20) NOT NULL DEFAULT '',
			deleted_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS categories (
			id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			name TEXT NOT NULL, description TEXT NOT NULL DEFAULT '',
			deleted_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS vendors (
			id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			name TEXT NOT NULL, email VARCHAR(255) NOT NULL DEFAULT '',
			phone VARCHAR(50) NOT NULL DEFAULT '', status VARCHAR(20) NOT NULL DEFAULT 'active',
			deleted_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS invoices (
			id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			number VARCHAR(50) NOT NULL UNIQUE, customer VARCHAR(255) NOT NULL,
			total DOUBLE NOT NULL DEFAULT 0, status VARCHAR(20) NOT NULL DEFAULT 'pending',
			deleted_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS purchase_orders (
			id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			number VARCHAR(50) NOT NULL UNIQUE, vendor VARCHAR(255) NOT NULL,
			total DOUBLE NOT NULL DEFAULT 0, status VARCHAR(20) NOT NULL DEFAULT 'draft',
			deleted_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS users (
			id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			name TEXT NOT NULL, email VARCHAR(255) NOT NULL DEFAULT '',
			role VARCHAR(30) NOT NULL DEFAULT 'staff',
			deleted_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS payments (
			id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			invoice VARCHAR(50) NOT NULL, amount DOUBLE NOT NULL DEFAULT 0,
			method VARCHAR(30) NOT NULL DEFAULT 'cash',
			deleted_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS credit_notes (
			id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			number VARCHAR(50) NOT NULL UNIQUE, customer VARCHAR(255) NOT NULL,
			total DOUBLE NOT NULL DEFAULT 0, status VARCHAR(20) NOT NULL DEFAULT 'issued',
			deleted_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS jobs (
			id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			name TEXT NOT NULL, status VARCHAR(20) NOT NULL DEFAULT 'queued',
			detail TEXT NOT NULL DEFAULT '',
			deleted_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS accounts (
			id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			name TEXT NOT NULL, type VARCHAR(20) NOT NULL DEFAULT 'asset',
			balance DOUBLE NOT NULL DEFAULT 0,
			deleted_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
	}
	for _, q := range tables {
		if _, err := s.db.Exec(q); err != nil {
			return err
		}
	}
	// Add deleted_at to pre-existing tables that were created without it (ignore "duplicate column" errors).
	for _, tbl := range []string{
		"customers", "categories", "vendors", "invoices", "purchase_orders",
		"users", "payments", "credit_notes", "jobs", "accounts",
	} {
		_, _ = s.db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN deleted_at DATETIME", tbl))
	}

	// Indexes — ignore errors on re-run (duplicate key name).
	for _, idx := range []string{
		`CREATE INDEX idx_customers_deleted   ON customers(deleted_at)`,
		`CREATE INDEX idx_categories_deleted  ON categories(deleted_at)`,
		`CREATE INDEX idx_vendors_deleted     ON vendors(deleted_at)`,
		`CREATE INDEX idx_invoices_deleted    ON invoices(deleted_at)`,
		`CREATE INDEX idx_invoices_status     ON invoices(status)`,
		`CREATE INDEX idx_invoices_customer   ON invoices(customer(191))`,
		`CREATE INDEX idx_purchase_deleted    ON purchase_orders(deleted_at)`,
		`CREATE INDEX idx_payments_invoice    ON payments(invoice)`,
		`CREATE INDEX idx_payments_deleted    ON payments(deleted_at)`,
		`CREATE INDEX idx_credit_notes_del    ON credit_notes(deleted_at)`,
		`CREATE INDEX idx_jobs_status         ON jobs(status)`,
		`CREATE INDEX idx_accounts_type       ON accounts(type)`,
	} {
		_, _ = s.db.Exec(idx)
	}
	return nil
}

// PageResult carries the rows plus metadata for pagination.
type PageResult struct {
	Records  []Record
	Total    int
	Page     int
	PerPage  int
	LastPage int
}

// List returns all non-deleted records sorted newest first.
func (s *ModuleStore) List(table string, columns []string) ([]Record, error) {
	q := fmt.Sprintf(
		"SELECT id, %s, created_at FROM %s WHERE deleted_at IS NULL ORDER BY id DESC",
		strings.Join(columns, ", "), table,
	)
	return s.query(q, columns)
}

// ListPaged returns paginated non-deleted records plus a total count.
func (s *ModuleStore) ListPaged(table string, columns []string, page, perPage int, search, sortCol, sortDir string) (PageResult, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 200 {
		perPage = 25
	}
	if sortDir != "asc" {
		sortDir = "desc"
	}
	// Only allow sorting by known column names to prevent injection.
	safeSort := "id"
	for _, c := range append(columns, "created_at", "id") {
		if c == sortCol {
			safeSort = sortCol
			break
		}
	}

	var whereClause string
	var args []any
	if search != "" {
		// Search across all text columns.
		parts := make([]string, len(columns))
		for i, c := range columns {
			parts[i] = fmt.Sprintf("CAST(%s AS CHAR) LIKE CONCAT('%%', ?, '%%')", c)
			args = append(args, search)
		}
		whereClause = "deleted_at IS NULL AND (" + strings.Join(parts, " OR ") + ")"
	} else {
		whereClause = "deleted_at IS NULL"
	}

	var total int
	countQ := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s", table, whereClause)
	if err := s.db.QueryRow(countQ, args...).Scan(&total); err != nil {
		return PageResult{}, err
	}

	offset := (page - 1) * perPage
	listQ := fmt.Sprintf(
		"SELECT id, %s, created_at FROM %s WHERE %s ORDER BY %s %s LIMIT ? OFFSET ?",
		strings.Join(columns, ", "), table, whereClause, safeSort, sortDir,
	)
	listArgs := append(args, perPage, offset)
	records, err := s.query(listQ, columns, listArgs...)
	if err != nil {
		return PageResult{}, err
	}

	lastPage := total / perPage
	if total%perPage != 0 {
		lastPage++
	}
	return PageResult{Records: records, Total: total, Page: page, PerPage: perPage, LastPage: lastPage}, nil
}

// Trash lists soft-deleted records.
func (s *ModuleStore) Trash(table string, columns []string) ([]Record, error) {
	q := fmt.Sprintf(
		"SELECT id, %s, created_at FROM %s WHERE deleted_at IS NOT NULL ORDER BY deleted_at DESC",
		strings.Join(columns, ", "), table,
	)
	return s.query(q, columns)
}

func (s *ModuleStore) Get(table string, columns []string, id int) (Record, error) {
	q := fmt.Sprintf(
		"SELECT id, %s, created_at FROM %s WHERE id = ? AND deleted_at IS NULL",
		strings.Join(columns, ", "), table,
	)
	scanColumns := append([]string{"id"}, append(columns, "created_at")...)
	values := make([]sql.NullString, len(scanColumns))
	args := make([]any, len(scanColumns))
	for i := range values {
		args[i] = &values[i]
	}
	if err := s.db.QueryRow(q, id).Scan(args...); err != nil {
		return nil, err
	}
	record := Record{}
	for i, col := range scanColumns {
		record[col] = values[i].String
	}
	return record, nil
}

func (s *ModuleStore) Create(table string, columns []string, values []string) error {
	marks := make([]string, len(columns))
	args := make([]any, len(values))
	for i, v := range values {
		marks[i] = "?"
		args[i] = v
	}
	q := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", table, strings.Join(columns, ", "), strings.Join(marks, ", "))
	_, err := s.db.Exec(q, args...)
	return err
}

func (s *ModuleStore) Update(table string, columns []string, values []string, id int) error {
	sets := make([]string, len(columns))
	args := make([]any, 0, len(values)+1)
	for i, col := range columns {
		sets[i] = col + " = ?"
		args = append(args, values[i])
	}
	args = append(args, id)
	q := fmt.Sprintf("UPDATE %s SET %s WHERE id = ? AND deleted_at IS NULL", table, strings.Join(sets, ", "))
	_, err := s.db.Exec(q, args...)
	return err
}

// Delete performs a soft delete by setting deleted_at.
func (s *ModuleStore) Delete(table string, id int) error {
	_, err := s.db.Exec(fmt.Sprintf("UPDATE %s SET deleted_at = CURRENT_TIMESTAMP WHERE id = ?", table), id)
	return err
}

// HardDelete permanently removes a record (trash→purge flow).
func (s *ModuleStore) HardDelete(table string, id int) error {
	_, err := s.db.Exec(fmt.Sprintf("DELETE FROM %s WHERE id = ? AND deleted_at IS NOT NULL", table), id)
	return err
}

// Restore clears deleted_at, making the record active again.
func (s *ModuleStore) Restore(table string, id int) error {
	_, err := s.db.Exec(fmt.Sprintf("UPDATE %s SET deleted_at = NULL WHERE id = ?", table), id)
	return err
}

func (s *ModuleStore) Count(table string) (int, error) {
	var count int
	err := s.db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE deleted_at IS NULL", table)).Scan(&count)
	return count, err
}

func (s *ModuleStore) Sum(table, column string) (float64, error) {
	var total sql.NullFloat64
	err := s.db.QueryRow(
		fmt.Sprintf("SELECT COALESCE(SUM(%s), 0) FROM %s WHERE deleted_at IS NULL", column, table),
	).Scan(&total)
	return total.Float64, err
}

// RecentActivity returns the latest N audit log entries for the activity feed.
func (s *ModuleStore) RecentActivity(limit int) ([]Record, error) {
	rows, err := s.db.Query(`
		SELECT id, COALESCE(user_name,'system'), module, action, record_id, created_at
		FROM audit_logs ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var records []Record
	for rows.Next() {
		var id, user, module, action, recID, ts string
		if err := rows.Scan(&id, &user, &module, &action, &recID, &ts); err != nil {
			return nil, err
		}
		records = append(records, Record{
			"id": id, "user_name": user, "module": module,
			"action": action, "record_id": recID, "created_at": ts,
		})
	}
	return records, rows.Err()
}

// TopCustomers returns customers with the highest invoice totals.
func (s *ModuleStore) TopCustomers(limit int) ([]Record, error) {
	rows, err := s.db.Query(`
		SELECT customer, COUNT(*) as invoice_count,
		       COALESCE(SUM(total),0) as total_value
		FROM invoices
		WHERE deleted_at IS NULL
		GROUP BY customer
		ORDER BY total_value DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var records []Record
	for rows.Next() {
		var name, count, total string
		if err := rows.Scan(&name, &count, &total); err != nil {
			return nil, err
		}
		records = append(records, Record{
			"customer": name, "invoice_count": count, "total_value": total,
		})
	}
	return records, rows.Err()
}

// PendingInvoicesTotal returns total value of non-paid invoices.
func (s *ModuleStore) PendingInvoicesTotal() (float64, error) {
	var total sql.NullFloat64
	err := s.db.QueryRow(`
		SELECT COALESCE(SUM(total),0) FROM invoices
		WHERE status != 'paid' AND deleted_at IS NULL`).Scan(&total)
	return total.Float64, err
}

func (s *ModuleStore) StockLogs() ([]Record, error) {
	rows, err := s.db.Query(`
		SELECT stock_logs.id, products.name, stock_logs.change_type,
		       stock_logs.quantity_before, stock_logs.quantity_change,
		       stock_logs.quantity_after, stock_logs.note, stock_logs.created_at
		FROM stock_logs
		JOIN products ON products.id = stock_logs.product_id
		ORDER BY stock_logs.id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var records []Record
	for rows.Next() {
		var id, name, ct, before, change, after, note, ts string
		if err := rows.Scan(&id, &name, &ct, &before, &change, &after, &note, &ts); err != nil {
			return nil, err
		}
		records = append(records, Record{
			"id": id, "product": name, "change_type": ct,
			"quantity_before": before, "quantity_change": change,
			"quantity_after": after, "note": note, "created_at": ts,
		})
	}
	return records, rows.Err()
}

// FindByField looks up a single record by an exact field match.
func (s *ModuleStore) FindByField(table, field, value string, columns []string) (Record, error) {
	q := fmt.Sprintf(
		"SELECT id, %s, created_at FROM %s WHERE %s = ? AND deleted_at IS NULL LIMIT 1",
		strings.Join(columns, ", "), table, field,
	)
	scanColumns := append([]string{"id"}, append(columns, "created_at")...)
	values := make([]sql.NullString, len(scanColumns))
	args := make([]any, len(scanColumns))
	for i := range values {
		args[i] = &values[i]
	}
	if err := s.db.QueryRow(q, value).Scan(args...); err != nil {
		return nil, err
	}
	rec := Record{}
	for i, col := range scanColumns {
		rec[col] = values[i].String
	}
	return rec, nil
}

// query is a shared row scanner used by List, Trash, ListPaged, etc.
func (s *ModuleStore) query(q string, columns []string, args ...any) ([]Record, error) {
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	scanColumns := append([]string{"id"}, append(columns, "created_at")...)
	var records []Record
	for rows.Next() {
		vals := make([]sql.NullString, len(scanColumns))
		ptrs := make([]any, len(scanColumns))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		rec := Record{}
		for i, col := range scanColumns {
			rec[col] = vals[i].String
		}
		records = append(records, rec)
	}
	return records, rows.Err()
}
