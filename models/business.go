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
	// All tenant tables now include business_id for data isolation.
	tables := []string{
		`CREATE TABLE IF NOT EXISTS customers (
			id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			business_id INT NOT NULL DEFAULT 0,
			name TEXT NOT NULL, email VARCHAR(255) NOT NULL DEFAULT '',
			phone VARCHAR(50) NOT NULL DEFAULT '', gstin VARCHAR(20) NOT NULL DEFAULT '',
			deleted_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS categories (
			id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			business_id INT NOT NULL DEFAULT 0,
			name TEXT NOT NULL, description TEXT NOT NULL DEFAULT '',
			deleted_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS vendors (
			id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			business_id INT NOT NULL DEFAULT 0,
			name TEXT NOT NULL, email VARCHAR(255) NOT NULL DEFAULT '',
			phone VARCHAR(50) NOT NULL DEFAULT '', status VARCHAR(20) NOT NULL DEFAULT 'active',
			deleted_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS invoices (
			id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			business_id INT NOT NULL DEFAULT 0,
			number VARCHAR(50) NOT NULL, customer VARCHAR(255) NOT NULL,
			total DOUBLE NOT NULL DEFAULT 0, status VARCHAR(20) NOT NULL DEFAULT 'pending',
			deleted_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS purchase_orders (
			id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			business_id INT NOT NULL DEFAULT 0,
			number VARCHAR(50) NOT NULL, vendor VARCHAR(255) NOT NULL,
			total DOUBLE NOT NULL DEFAULT 0, status VARCHAR(20) NOT NULL DEFAULT 'draft',
			deleted_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS users (
			id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			business_id INT NOT NULL DEFAULT 0,
			name TEXT NOT NULL, email VARCHAR(255) NOT NULL DEFAULT '',
			role VARCHAR(30) NOT NULL DEFAULT 'staff',
			deleted_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS payments (
			id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			business_id INT NOT NULL DEFAULT 0,
			invoice VARCHAR(50) NOT NULL, amount DOUBLE NOT NULL DEFAULT 0,
			method VARCHAR(30) NOT NULL DEFAULT 'cash',
			deleted_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS credit_notes (
			id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			business_id INT NOT NULL DEFAULT 0,
			number VARCHAR(50) NOT NULL, customer VARCHAR(255) NOT NULL,
			total DOUBLE NOT NULL DEFAULT 0, status VARCHAR(20) NOT NULL DEFAULT 'issued',
			deleted_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS jobs (
			id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			business_id INT NOT NULL DEFAULT 0,
			name TEXT NOT NULL, status VARCHAR(20) NOT NULL DEFAULT 'queued',
			detail TEXT NOT NULL DEFAULT '',
			deleted_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS accounts (
			id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			business_id INT NOT NULL DEFAULT 0,
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

	// Add missing columns to pre-existing tables — ignore "duplicate column" errors.
	allTables := []string{
		"customers", "categories", "vendors", "invoices", "purchase_orders",
		"users", "payments", "credit_notes", "jobs", "accounts",
	}
	for _, tbl := range allTables {
		_, _ = s.db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN deleted_at DATETIME", tbl))
		_, _ = s.db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN business_id INT NOT NULL DEFAULT 0", tbl))
	}

	// Indexes — ignore errors on re-run.
	for _, idx := range []string{
		// business_id indexes — critical for multi-tenant query performance
		`CREATE INDEX idx_customers_biz    ON customers(business_id)`,
		`CREATE INDEX idx_categories_biz   ON categories(business_id)`,
		`CREATE INDEX idx_vendors_biz      ON vendors(business_id)`,
		`CREATE INDEX idx_invoices_biz     ON invoices(business_id)`,
		`CREATE INDEX idx_purchase_biz     ON purchase_orders(business_id)`,
		`CREATE INDEX idx_users_biz        ON users(business_id)`,
		`CREATE INDEX idx_payments_biz     ON payments(business_id)`,
		`CREATE INDEX idx_credit_biz       ON credit_notes(business_id)`,
		`CREATE INDEX idx_jobs_biz         ON jobs(business_id)`,
		`CREATE INDEX idx_accounts_biz     ON accounts(business_id)`,
		// Other indexes
		`CREATE INDEX idx_customers_deleted   ON customers(deleted_at)`,
		`CREATE INDEX idx_categories_deleted  ON categories(deleted_at)`,
		`CREATE INDEX idx_vendors_deleted     ON vendors(deleted_at)`,
		`CREATE INDEX idx_invoices_deleted    ON invoices(deleted_at)`,
		`CREATE INDEX idx_invoices_status     ON invoices(status)`,
		`CREATE INDEX idx_purchase_deleted    ON purchase_orders(deleted_at)`,
		`CREATE INDEX idx_payments_invoice    ON payments(invoice)`,
		`CREATE INDEX idx_payments_deleted    ON payments(deleted_at)`,
		`CREATE INDEX idx_credit_notes_del    ON credit_notes(deleted_at)`,
		`CREATE INDEX idx_jobs_status         ON jobs(status)`,
	} {
		_, _ = s.db.Exec(idx)
	}
	return nil
}

// ── PageResult ────────────────────────────────────────────────────────────────

type PageResult struct {
	Records  []Record
	Total    int
	Page     int
	PerPage  int
	LastPage int
}

// ── Scoped read operations ────────────────────────────────────────────────────

// List returns all non-deleted records for a business, sorted newest first.
func (s *ModuleStore) List(table string, columns []string, businessID int) ([]Record, error) {
	q := fmt.Sprintf(
		"SELECT id, %s, created_at FROM %s WHERE deleted_at IS NULL AND business_id = ? ORDER BY id DESC",
		strings.Join(columns, ", "), table,
	)
	return s.query(q, columns, businessID)
}

// ListPaged returns paginated records for a business with search + sort.
func (s *ModuleStore) ListPaged(table string, columns []string, page, perPage int, search, sortCol, sortDir string, businessID int) (PageResult, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 200 {
		perPage = 25
	}
	if sortDir != "asc" {
		sortDir = "desc"
	}
	safeSort := "id"
	for _, c := range append(columns, "created_at", "id") {
		if c == sortCol {
			safeSort = sortCol
			break
		}
	}

	var args []any
	var whereClause string
	if search != "" {
		parts := make([]string, len(columns))
		for i, c := range columns {
			parts[i] = fmt.Sprintf("CAST(%s AS CHAR) LIKE CONCAT('%%', ?, '%%')", c)
			args = append(args, search)
		}
		whereClause = "deleted_at IS NULL AND business_id = ? AND (" + strings.Join(parts, " OR ") + ")"
	} else {
		whereClause = "deleted_at IS NULL AND business_id = ?"
	}
	// business_id always goes first in args
	args = append([]any{businessID}, args...)

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
	records, err := s.query(listQ, columns, append(args, perPage, offset)...)
	if err != nil {
		return PageResult{}, err
	}

	lastPage := total / perPage
	if total%perPage != 0 {
		lastPage++
	}
	return PageResult{Records: records, Total: total, Page: page, PerPage: perPage, LastPage: lastPage}, nil
}

// Trash lists soft-deleted records for a business.
func (s *ModuleStore) Trash(table string, columns []string, businessID int) ([]Record, error) {
	q := fmt.Sprintf(
		"SELECT id, %s, created_at FROM %s WHERE deleted_at IS NOT NULL AND business_id = ? ORDER BY deleted_at DESC",
		strings.Join(columns, ", "), table,
	)
	return s.query(q, columns, businessID)
}

// Get fetches a single record — returns an error (404 equivalent) if it belongs to a different business.
func (s *ModuleStore) Get(table string, columns []string, id, businessID int) (Record, error) {
	q := fmt.Sprintf(
		"SELECT id, %s, created_at FROM %s WHERE id = ? AND business_id = ? AND deleted_at IS NULL",
		strings.Join(columns, ", "), table,
	)
	scanColumns := append([]string{"id"}, append(columns, "created_at")...)
	values := make([]sql.NullString, len(scanColumns))
	args := make([]any, len(scanColumns))
	for i := range values {
		args[i] = &values[i]
	}
	if err := s.db.QueryRow(q, id, businessID).Scan(args...); err != nil {
		return nil, err
	}
	record := Record{}
	for i, col := range scanColumns {
		record[col] = values[i].String
	}
	return record, nil
}

// ── Write operations ──────────────────────────────────────────────────────────

// Create inserts a new record and injects business_id automatically.
func (s *ModuleStore) Create(table string, columns []string, values []string, businessID int) error {
	allCols := append([]string{"business_id"}, columns...)
	marks := make([]string, len(allCols))
	args := make([]any, len(allCols))
	marks[0] = "?"
	args[0] = businessID
	for i, v := range values {
		marks[i+1] = "?"
		args[i+1] = v
	}
	q := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		table, strings.Join(allCols, ", "), strings.Join(marks, ", "))
	_, err := s.db.Exec(q, args...)
	return err
}

// Update modifies a record only if it belongs to the given business.
func (s *ModuleStore) Update(table string, columns []string, values []string, id, businessID int) error {
	sets := make([]string, len(columns))
	args := make([]any, 0, len(values)+2)
	for i, col := range columns {
		sets[i] = col + " = ?"
		args = append(args, values[i])
	}
	args = append(args, id, businessID)
	q := fmt.Sprintf("UPDATE %s SET %s WHERE id = ? AND business_id = ? AND deleted_at IS NULL",
		table, strings.Join(sets, ", "))
	_, err := s.db.Exec(q, args...)
	return err
}

// Delete soft-deletes a record, scoped to the business.
func (s *ModuleStore) Delete(table string, id, businessID int) error {
	_, err := s.db.Exec(
		fmt.Sprintf("UPDATE %s SET deleted_at = CURRENT_TIMESTAMP WHERE id = ? AND business_id = ?", table),
		id, businessID,
	)
	return err
}

// HardDelete permanently removes a soft-deleted record, scoped to the business.
func (s *ModuleStore) HardDelete(table string, id, businessID int) error {
	_, err := s.db.Exec(
		fmt.Sprintf("DELETE FROM %s WHERE id = ? AND business_id = ? AND deleted_at IS NOT NULL", table),
		id, businessID,
	)
	return err
}

// Restore clears deleted_at, scoped to the business.
func (s *ModuleStore) Restore(table string, id, businessID int) error {
	_, err := s.db.Exec(
		fmt.Sprintf("UPDATE %s SET deleted_at = NULL WHERE id = ? AND business_id = ?", table),
		id, businessID,
	)
	return err
}

// ── Aggregate helpers ─────────────────────────────────────────────────────────

func (s *ModuleStore) Count(table string, businessID int) (int, error) {
	var count int
	err := s.db.QueryRow(
		fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE deleted_at IS NULL AND business_id = ?", table),
		businessID,
	).Scan(&count)
	return count, err
}

func (s *ModuleStore) Sum(table, column string, businessID int) (float64, error) {
	var total sql.NullFloat64
	err := s.db.QueryRow(
		fmt.Sprintf("SELECT COALESCE(SUM(%s), 0) FROM %s WHERE deleted_at IS NULL AND business_id = ?", column, table),
		businessID,
	).Scan(&total)
	return total.Float64, err
}

// ── Dashboard helpers — all scoped ───────────────────────────────────────────

func (s *ModuleStore) RecentActivity(limit, businessID int) ([]Record, error) {
	rows, err := s.db.Query(`
		SELECT id, COALESCE(user_name,'system'), module, action, record_id, created_at
		FROM audit_logs
		WHERE business_id = ?
		ORDER BY id DESC LIMIT ?`, businessID, limit)
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

func (s *ModuleStore) TopCustomers(limit, businessID int) ([]Record, error) {
	rows, err := s.db.Query(`
		SELECT customer, COUNT(*) as invoice_count,
		       COALESCE(SUM(total),0) as total_value
		FROM invoices
		WHERE deleted_at IS NULL AND business_id = ?
		GROUP BY customer
		ORDER BY total_value DESC
		LIMIT ?`, businessID, limit)
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

func (s *ModuleStore) PendingInvoicesTotal(businessID int) (float64, error) {
	var total sql.NullFloat64
	err := s.db.QueryRow(`
		SELECT COALESCE(SUM(total),0) FROM invoices
		WHERE status != 'paid' AND deleted_at IS NULL AND business_id = ?`,
		businessID).Scan(&total)
	return total.Float64, err
}

func (s *ModuleStore) StockLogs(businessID int) ([]Record, error) {
	rows, err := s.db.Query(`
		SELECT stock_logs.id, products.name, stock_logs.change_type,
		       stock_logs.quantity_before, stock_logs.quantity_change,
		       stock_logs.quantity_after, stock_logs.note, stock_logs.created_at
		FROM stock_logs
		JOIN products ON products.id = stock_logs.product_id
		WHERE products.business_id = ?
		ORDER BY stock_logs.id DESC`, businessID)
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

// FindByField looks up a single record by exact field match within a business.
func (s *ModuleStore) FindByField(table, field, value string, columns []string, businessID int) (Record, error) {
	q := fmt.Sprintf(
		"SELECT id, %s, created_at FROM %s WHERE %s = ? AND business_id = ? AND deleted_at IS NULL LIMIT 1",
		strings.Join(columns, ", "), table, field,
	)
	scanColumns := append([]string{"id"}, append(columns, "created_at")...)
	vals := make([]sql.NullString, len(scanColumns))
	args := make([]any, len(scanColumns))
	for i := range vals {
		args[i] = &vals[i]
	}
	if err := s.db.QueryRow(q, value, businessID).Scan(args...); err != nil {
		return nil, err
	}
	rec := Record{}
	for i, col := range scanColumns {
		rec[col] = vals[i].String
	}
	return rec, nil
}

// query is the shared row scanner.
func (s *ModuleStore) query(q string, columns []string, args ...any) ([]Record, error) {
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	scanColumns := append([]string{"id"}, append(columns, "created_at")...)
	var records []Record
	for rows.Next() {
		vs := make([]sql.NullString, len(scanColumns))
		ptrs := make([]any, len(scanColumns))
		for i := range vs {
			ptrs[i] = &vs[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		rec := Record{}
		for i, col := range scanColumns {
			rec[col] = vs[i].String
		}
		records = append(records, rec)
	}
	return records, rows.Err()
}
