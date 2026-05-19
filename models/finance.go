package models

import (
	"database/sql"
	"fmt"
	"time"
)

// ── Domain types ──────────────────────────────────────────────────────────────

type ExpenseCategory struct {
	ID          int
	BusinessID  int
	Name        string
	Description string
	CreatedAt   time.Time
}

type Expense struct {
	ID              int
	BusinessID      int
	CategoryID      int
	CategoryName    string
	Amount          float64
	PaymentMethod   string // cash|bank|upi|card|cheque
	BankAccountID   *int
	BankAccountName string
	Reference       string
	Description     string
	ExpenseDate     time.Time
	Status          string // approved|pending|rejected
	CreatedAt       time.Time
}

type BankAccount struct {
	ID             int
	BusinessID     int
	AccountName    string
	BankName       string
	AccountNumber  string
	IFSC           string
	OpeningBalance float64
	CurrentBalance float64
	Status         string // active|closed
	CreatedAt      time.Time
}

type BankTransaction struct {
	ID              int
	BusinessID      int
	AccountID       int
	AccountName     string
	TransactionType string // credit|debit
	Amount          float64
	Reference       string
	Description     string
	TransactionDate time.Time
	CreatedAt       time.Time
}

// LedgerEntry is a single row in a customer/supplier/cash ledger.
// It is computed (not stored) — read-only.
type LedgerEntry struct {
	TxnDate     time.Time
	Description string
	RefType     string // sales_order|payment|pos_sale|expense|procurement_order|bank_transaction
	RefID       int
	RefNumber   string
	Debit       float64 // money going out or reducing balance
	Credit      float64 // money coming in or increasing balance
	Balance     float64 // running balance (set by caller after sorting)
}

// PLReport is a computed P&L summary.
type PLReport struct {
	From          string
	To            string
	POSRevenue    float64
	SORevenue     float64
	TotalRevenue  float64
	SalesReturns  float64
	NetRevenue    float64
	COGS          float64 // procurement orders completed in period
	GrossProfit   float64
	GrossMarginPct float64
	ExpensesByCategory []ExpenseSummary
	TotalExpenses float64
	NetProfit     float64
	NetMarginPct  float64
	// Tax
	OutputGST float64
	InputGST  float64
	NetGST    float64
}

type ExpenseSummary struct {
	Category string
	Amount   float64
}

// CashflowRow is one month/period row.
type CashflowRow struct {
	Period     string
	CashIn     float64
	CashOut    float64
	NetCashflow float64
}

// GSTReport is a computed GST summary.
type GSTReport struct {
	From          string
	To            string
	OutputGST     float64 // collected from customers (POS + SO)
	InputGST      float64 // paid to suppliers (procurement)
	NetGSTPayable float64
	ByMonth       []GSTMonthRow
}

type GSTMonthRow struct {
	Month     string
	OutputGST float64
	InputGST  float64
	Net       float64
}

// FinanceDashboard aggregates all key financial metrics.
type FinanceDashboard struct {
	TotalReceivables  float64 // customer outstanding
	TotalPayables     float64 // supplier outstanding
	CashBalance       float64 // cumulative cash in - cash out
	TotalBankBalance  float64 // sum of bank accounts
	MonthRevenue      float64
	MonthExpenses     float64
	MonthProfit       float64
	PendingExpenses   int
	TotalBankAccounts int
	RecentExpenses    []Expense
}

// ── Store ─────────────────────────────────────────────────────────────────────

type FinanceStore struct {
	db *sql.DB
}

func NewFinanceStore(db *sql.DB) *FinanceStore {
	return &FinanceStore{db: db}
}

func (s *FinanceStore) Migrate() error {
	tables := []string{
		`CREATE TABLE IF NOT EXISTS expense_categories (
			id          INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			business_id INT NOT NULL DEFAULT 0,
			name        VARCHAR(100) NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS expenses (
			id              INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			business_id     INT NOT NULL DEFAULT 0,
			category_id     INT NOT NULL DEFAULT 0,
			amount          DECIMAL(12,2) NOT NULL DEFAULT 0,
			payment_method  VARCHAR(20) NOT NULL DEFAULT 'cash',
			bank_account_id INT NULL,
			reference       VARCHAR(255) NOT NULL DEFAULT '',
			description     TEXT NOT NULL DEFAULT '',
			expense_date    DATE NOT NULL,
			status          VARCHAR(20) NOT NULL DEFAULT 'approved',
			created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS bank_accounts (
			id              INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			business_id     INT NOT NULL DEFAULT 0,
			account_name    VARCHAR(255) NOT NULL,
			bank_name       VARCHAR(255) NOT NULL DEFAULT '',
			account_number  VARCHAR(50) NOT NULL DEFAULT '',
			ifsc            VARCHAR(20) NOT NULL DEFAULT '',
			opening_balance DECIMAL(12,2) NOT NULL DEFAULT 0,
			current_balance DECIMAL(12,2) NOT NULL DEFAULT 0,
			status          VARCHAR(20) NOT NULL DEFAULT 'active',
			created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS bank_transactions (
			id               INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			business_id      INT NOT NULL DEFAULT 0,
			account_id       INT NOT NULL DEFAULT 0,
			transaction_type VARCHAR(10) NOT NULL DEFAULT 'credit',
			amount           DECIMAL(12,2) NOT NULL DEFAULT 0,
			reference        VARCHAR(255) NOT NULL DEFAULT '',
			description      TEXT NOT NULL DEFAULT '',
			transaction_date DATE NOT NULL,
			created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
	}
	for _, q := range tables {
		if _, err := s.db.Exec(q); err != nil {
			return err
		}
	}
	for _, idx := range []string{
		`CREATE INDEX idx_expenses_biz      ON expenses(business_id)`,
		`CREATE INDEX idx_expenses_date     ON expenses(business_id, expense_date)`,
		`CREATE INDEX idx_expenses_cat      ON expenses(category_id)`,
		`CREATE INDEX idx_bank_accts_biz    ON bank_accounts(business_id)`,
		`CREATE INDEX idx_bank_txns_biz     ON bank_transactions(business_id)`,
		`CREATE INDEX idx_bank_txns_acct    ON bank_transactions(account_id)`,
	} {
		_, _ = s.db.Exec(idx)
	}
	// Seed default expense categories for new businesses.
	return nil
}

// ── Expense Categories ────────────────────────────────────────────────────────

func (s *FinanceStore) ListCategories(bizID int) ([]ExpenseCategory, error) {
	rows, err := s.db.Query(`
		SELECT id, business_id, name, description, created_at
		FROM expense_categories WHERE business_id=? ORDER BY name`, bizID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ExpenseCategory
	for rows.Next() {
		var c ExpenseCategory
		if err = rows.Scan(&c.ID, &c.BusinessID, &c.Name, &c.Description, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *FinanceStore) CreateCategory(bizID int, name, description string) (*ExpenseCategory, error) {
	res, err := s.db.Exec(`INSERT INTO expense_categories (business_id, name, description) VALUES (?,?,?)`, bizID, name, description)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	var c ExpenseCategory
	_ = s.db.QueryRow(`SELECT id, business_id, name, description, created_at FROM expense_categories WHERE id=?`, id).
		Scan(&c.ID, &c.BusinessID, &c.Name, &c.Description, &c.CreatedAt)
	return &c, nil
}

func (s *FinanceStore) EnsureDefaultCategories(bizID int) error {
	var count int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM expense_categories WHERE business_id=?`, bizID).Scan(&count)
	if count > 0 {
		return nil
	}
	defaults := []string{"Rent", "Salaries", "Utilities (Electricity/Water)", "Transport / Fuel", "Office Supplies", "Maintenance & Repairs", "Marketing & Advertising", "Miscellaneous"}
	for _, name := range defaults {
		_, _ = s.db.Exec(`INSERT INTO expense_categories (business_id, name) VALUES (?,?)`, bizID, name)
	}
	return nil
}

// ── Expenses ─────────────────────────────────────────────────────────────────

func (s *FinanceStore) ListExpenses(bizID int, from, to string, categoryID int) ([]Expense, error) {
	q := `SELECT e.id, e.business_id, e.category_id, COALESCE(c.name,'Uncategorized'),
		e.amount, e.payment_method, e.bank_account_id, COALESCE(b.account_name,''),
		e.reference, e.description, e.expense_date, e.status, e.created_at
		FROM expenses e
		LEFT JOIN expense_categories c ON c.id=e.category_id
		LEFT JOIN bank_accounts b ON b.id=e.bank_account_id
		WHERE e.business_id=?`
	args := []any{bizID}
	if from != "" {
		q += ` AND e.expense_date >= ?`
		args = append(args, from)
	}
	if to != "" {
		q += ` AND e.expense_date <= ?`
		args = append(args, to)
	}
	if categoryID > 0 {
		q += ` AND e.category_id=?`
		args = append(args, categoryID)
	}
	q += ` ORDER BY e.expense_date DESC, e.id DESC`

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Expense
	for rows.Next() {
		var e Expense
		if err = rows.Scan(&e.ID, &e.BusinessID, &e.CategoryID, &e.CategoryName,
			&e.Amount, &e.PaymentMethod, &e.BankAccountID, &e.BankAccountName,
			&e.Reference, &e.Description, &e.ExpenseDate, &e.Status, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *FinanceStore) CreateExpense(e *Expense) (*Expense, error) {
	res, err := s.db.Exec(`
		INSERT INTO expenses (business_id, category_id, amount, payment_method, bank_account_id,
		reference, description, expense_date, status)
		VALUES (?,?,?,?,?,?,?,?,?)`,
		e.BusinessID, e.CategoryID, e.Amount, e.PaymentMethod, e.BankAccountID,
		e.Reference, e.Description, e.ExpenseDate.Format("2006-01-02"), e.Status,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	e.ID = int(id)
	// If bank payment, update bank account balance.
	if e.BankAccountID != nil && *e.BankAccountID > 0 {
		_, _ = s.db.Exec(`UPDATE bank_accounts SET current_balance=current_balance-? WHERE id=?`, e.Amount, *e.BankAccountID)
	}
	return e, nil
}

func (s *FinanceStore) UpdateExpenseStatus(id int, status string) error {
	_, err := s.db.Exec(`UPDATE expenses SET status=? WHERE id=?`, status, id)
	return err
}

func (s *FinanceStore) SumExpenses(bizID int, from, to string) (float64, error) {
	var total float64
	err := s.db.QueryRow(`
		SELECT COALESCE(SUM(amount),0) FROM expenses
		WHERE business_id=? AND status='approved'
		AND expense_date BETWEEN ? AND ?`, bizID, from, to,
	).Scan(&total)
	return total, err
}

func (s *FinanceStore) ExpensesByCategory(bizID int, from, to string) ([]ExpenseSummary, error) {
	rows, err := s.db.Query(`
		SELECT COALESCE(c.name,'Uncategorized'), COALESCE(SUM(e.amount),0)
		FROM expenses e
		LEFT JOIN expense_categories c ON c.id=e.category_id
		WHERE e.business_id=? AND e.status='approved' AND e.expense_date BETWEEN ? AND ?
		GROUP BY e.category_id, c.name ORDER BY SUM(e.amount) DESC`, bizID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ExpenseSummary
	for rows.Next() {
		var s ExpenseSummary
		_ = rows.Scan(&s.Category, &s.Amount)
		out = append(out, s)
	}
	return out, rows.Err()
}

func (s *FinanceStore) CountPendingExpenses(bizID int) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM expenses WHERE business_id=? AND status='pending'`, bizID).Scan(&n)
	return n, err
}

// ── Bank Accounts ─────────────────────────────────────────────────────────────

func (s *FinanceStore) ListBankAccounts(bizID int) ([]BankAccount, error) {
	rows, err := s.db.Query(`
		SELECT id, business_id, account_name, bank_name, account_number, ifsc,
		opening_balance, current_balance, status, created_at
		FROM bank_accounts WHERE business_id=? ORDER BY account_name`, bizID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BankAccount
	for rows.Next() {
		var b BankAccount
		if err = rows.Scan(&b.ID, &b.BusinessID, &b.AccountName, &b.BankName, &b.AccountNumber, &b.IFSC,
			&b.OpeningBalance, &b.CurrentBalance, &b.Status, &b.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (s *FinanceStore) CreateBankAccount(b *BankAccount) (*BankAccount, error) {
	res, err := s.db.Exec(`INSERT INTO bank_accounts (business_id, account_name, bank_name, account_number, ifsc, opening_balance, current_balance, status)
		VALUES (?,?,?,?,?,?,?,?)`,
		b.BusinessID, b.AccountName, b.BankName, b.AccountNumber, b.IFSC,
		b.OpeningBalance, b.OpeningBalance, "active",
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	b.ID = int(id)
	b.CurrentBalance = b.OpeningBalance
	return b, nil
}

func (s *FinanceStore) AddBankTransaction(tx *BankTransaction) error {
	_, err := s.db.Exec(`INSERT INTO bank_transactions (business_id, account_id, transaction_type, amount, reference, description, transaction_date)
		VALUES (?,?,?,?,?,?,?)`,
		tx.BusinessID, tx.AccountID, tx.TransactionType, tx.Amount, tx.Reference, tx.Description,
		tx.TransactionDate.Format("2006-01-02"),
	)
	if err != nil {
		return err
	}
	if tx.TransactionType == "credit" {
		_, err = s.db.Exec(`UPDATE bank_accounts SET current_balance=current_balance+? WHERE id=?`, tx.Amount, tx.AccountID)
	} else {
		_, err = s.db.Exec(`UPDATE bank_accounts SET current_balance=current_balance-? WHERE id=?`, tx.Amount, tx.AccountID)
	}
	return err
}

func (s *FinanceStore) ListBankTransactions(bizID, accountID int) ([]BankTransaction, error) {
	q := `SELECT bt.id, bt.business_id, bt.account_id, COALESCE(ba.account_name,''),
		bt.transaction_type, bt.amount, bt.reference, bt.description,
		bt.transaction_date, bt.created_at
		FROM bank_transactions bt
		LEFT JOIN bank_accounts ba ON ba.id=bt.account_id
		WHERE bt.business_id=?`
	args := []any{bizID}
	if accountID > 0 {
		q += ` AND bt.account_id=?`
		args = append(args, accountID)
	}
	q += ` ORDER BY bt.transaction_date DESC, bt.id DESC`

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BankTransaction
	for rows.Next() {
		var tx BankTransaction
		if err = rows.Scan(&tx.ID, &tx.BusinessID, &tx.AccountID, &tx.AccountName,
			&tx.TransactionType, &tx.Amount, &tx.Reference, &tx.Description,
			&tx.TransactionDate, &tx.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, tx)
	}
	return out, rows.Err()
}

func (s *FinanceStore) TotalBankBalance(bizID int) (float64, error) {
	var total float64
	err := s.db.QueryRow(`SELECT COALESCE(SUM(current_balance),0) FROM bank_accounts WHERE business_id=? AND status='active'`, bizID).Scan(&total)
	return total, err
}

// ── Ledger queries ────────────────────────────────────────────────────────────

func (s *FinanceStore) CustomerLedger(bizID, customerID int, from, to string) ([]LedgerEntry, error) {
	q := `
		SELECT DATE(created_at) as txn_date, CONCAT('Sales Order ', order_number) as desc,
		       'sales_order' as ref_type, id as ref_id, order_number as ref_num,
		       grand_total as credit, 0.0 as debit
		FROM sales_orders
		WHERE business_id=? AND customer_id=? AND status NOT IN ('draft','cancelled')
		  AND DATE(created_at) BETWEEN ? AND ?

		UNION ALL

		SELECT DATE(created_at), CONCAT('Payment ', payment_number),
		       'payment', id, payment_number,
		       0.0, amount
		FROM customer_payments
		WHERE business_id=? AND customer_id=?
		  AND DATE(created_at) BETWEEN ? AND ?

		ORDER BY txn_date ASC, ref_type ASC`

	rows, err := s.db.Query(q, bizID, customerID, from, to, bizID, customerID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanLedgerRows(rows)
}

func (s *FinanceStore) SupplierLedger(bizID, supplierID int, from, to string) ([]LedgerEntry, error) {
	q := `
		SELECT DATE(created_at), CONCAT('Purchase Order ', po_number),
		       'procurement_order', id, po_number,
		       grand_total, 0.0
		FROM procurement_orders
		WHERE business_id=? AND supplier_id=? AND status='completed'
		  AND DATE(created_at) BETWEEN ? AND ?

		UNION ALL

		SELECT DATE(created_at), CONCAT('Payment ', payment_number),
		       'payment', id, payment_number,
		       0.0, amount
		FROM supplier_payments
		WHERE business_id=? AND supplier_id=?
		  AND DATE(created_at) BETWEEN ? AND ?

		ORDER BY txn_date ASC, ref_type ASC`

	rows, err := s.db.Query(q, bizID, supplierID, from, to, bizID, supplierID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanLedgerRows(rows)
}

func (s *FinanceStore) CashLedger(bizID int, from, to string) ([]LedgerEntry, error) {
	q := `
		SELECT DATE(created_at), CONCAT('POS Sale ', COALESCE(sale_number,'')),
		       'pos_sale', id, COALESCE(sale_number,''),
		       grand_total, 0.0
		FROM pos_sales
		WHERE business_id=? AND payment_method='cash' AND status='completed'
		  AND DATE(created_at) BETWEEN ? AND ?

		UNION ALL

		SELECT DATE(created_at), CONCAT('Customer Payment ', payment_number),
		       'cust_payment', id, payment_number,
		       amount, 0.0
		FROM customer_payments
		WHERE business_id=? AND payment_method='cash'
		  AND DATE(created_at) BETWEEN ? AND ?

		UNION ALL

		SELECT DATE(created_at), CONCAT('Supplier Payment ', payment_number),
		       'supp_payment', id, payment_number,
		       0.0, amount
		FROM supplier_payments
		WHERE business_id=? AND payment_method='cash'
		  AND DATE(created_at) BETWEEN ? AND ?

		UNION ALL

		SELECT DATE(expense_date), CONCAT('Expense: ', COALESCE(description,'')),
		       'expense', id, COALESCE(reference,''),
		       0.0, amount
		FROM expenses
		WHERE business_id=? AND payment_method='cash' AND status='approved'
		  AND expense_date BETWEEN ? AND ?

		ORDER BY txn_date ASC`

	rows, err := s.db.Query(q,
		bizID, from, to,
		bizID, from, to,
		bizID, from, to,
		bizID, from, to,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanLedgerRows(rows)
}

func scanLedgerRows(rows *sql.Rows) ([]LedgerEntry, error) {
	var out []LedgerEntry
	for rows.Next() {
		var e LedgerEntry
		var d time.Time
		if err := rows.Scan(&d, &e.Description, &e.RefType, &e.RefID, &e.RefNumber, &e.Credit, &e.Debit); err != nil {
			return nil, err
		}
		e.TxnDate = d
		out = append(out, e)
	}
	return out, rows.Err()
}

// ── P&L queries ───────────────────────────────────────────────────────────────

func (s *FinanceStore) PLData(bizID int, from, to string) (PLReport, error) {
	pl := PLReport{From: from, To: to}

	// Revenue from POS
	_ = s.db.QueryRow(`SELECT COALESCE(SUM(grand_total),0) FROM pos_sales WHERE business_id=? AND status='completed' AND DATE(created_at) BETWEEN ? AND ?`, bizID, from, to).Scan(&pl.POSRevenue)

	// Revenue from completed Sales Orders
	_ = s.db.QueryRow(`SELECT COALESCE(SUM(grand_total),0) FROM sales_orders WHERE business_id=? AND status IN ('completed','delivered') AND DATE(created_at) BETWEEN ? AND ?`, bizID, from, to).Scan(&pl.SORevenue)

	// Sales returns
	_ = s.db.QueryRow(`SELECT COALESCE(SUM(total_amount),0) FROM sales_returns WHERE business_id=? AND DATE(created_at) BETWEEN ? AND ?`, bizID, from, to).Scan(&pl.SalesReturns)

	pl.TotalRevenue = pl.POSRevenue + pl.SORevenue
	pl.NetRevenue = pl.TotalRevenue - pl.SalesReturns

	// COGS from completed procurement orders
	_ = s.db.QueryRow(`SELECT COALESCE(SUM(grand_total),0) FROM procurement_orders WHERE business_id=? AND status='completed' AND DATE(created_at) BETWEEN ? AND ?`, bizID, from, to).Scan(&pl.COGS)

	pl.GrossProfit = pl.NetRevenue - pl.COGS
	if pl.NetRevenue > 0 {
		pl.GrossMarginPct = (pl.GrossProfit / pl.NetRevenue) * 100
	}

	// Expenses by category
	pl.ExpensesByCategory, _ = s.ExpensesByCategory(bizID, from, to)
	for _, ec := range pl.ExpensesByCategory {
		pl.TotalExpenses += ec.Amount
	}

	pl.NetProfit = pl.GrossProfit - pl.TotalExpenses
	if pl.NetRevenue > 0 {
		pl.NetMarginPct = (pl.NetProfit / pl.NetRevenue) * 100
	}

	// GST
	_ = s.db.QueryRow(`
		SELECT COALESCE(SUM(i.tax_amount),0) FROM pos_sale_items i
		JOIN pos_sales s ON s.id=i.sale_id
		WHERE s.business_id=? AND s.status='completed' AND DATE(s.created_at) BETWEEN ? AND ?`, bizID, from, to,
	).Scan(&pl.OutputGST)

	var soGST float64
	_ = s.db.QueryRow(`
		SELECT COALESCE(SUM(i.tax_amount),0) FROM sales_order_items i
		JOIN sales_orders o ON o.id=i.order_id
		WHERE o.business_id=? AND o.status IN ('completed','delivered') AND DATE(o.created_at) BETWEEN ? AND ?`, bizID, from, to,
	).Scan(&soGST)
	pl.OutputGST += soGST

	_ = s.db.QueryRow(`
		SELECT COALESCE(SUM(i.tax_amount),0) FROM procurement_order_items i
		JOIN procurement_orders o ON o.id=i.order_id
		WHERE o.business_id=? AND o.status='completed' AND DATE(o.created_at) BETWEEN ? AND ?`, bizID, from, to,
	).Scan(&pl.InputGST)

	pl.NetGST = pl.OutputGST - pl.InputGST
	return pl, nil
}

// ── Cashflow ──────────────────────────────────────────────────────────────────

func (s *FinanceStore) CashflowByMonth(bizID int, from, to string) ([]CashflowRow, error) {
	rows, err := s.db.Query(`
		SELECT period, SUM(cash_in), SUM(cash_out) FROM (
			SELECT DATE_FORMAT(created_at,'%Y-%m') as period, grand_total as cash_in, 0.0 as cash_out
			FROM pos_sales WHERE business_id=? AND payment_method='cash' AND status='completed' AND DATE(created_at) BETWEEN ? AND ?

			UNION ALL

			SELECT DATE_FORMAT(created_at,'%Y-%m'), amount, 0.0
			FROM customer_payments WHERE business_id=? AND payment_method='cash' AND DATE(created_at) BETWEEN ? AND ?

			UNION ALL

			SELECT DATE_FORMAT(created_at,'%Y-%m'), 0.0, amount
			FROM supplier_payments WHERE business_id=? AND payment_method='cash' AND DATE(created_at) BETWEEN ? AND ?

			UNION ALL

			SELECT DATE_FORMAT(expense_date,'%Y-%m'), 0.0, amount
			FROM expenses WHERE business_id=? AND payment_method='cash' AND status='approved' AND expense_date BETWEEN ? AND ?
		) sub
		GROUP BY period ORDER BY period ASC`,
		bizID, from, to,
		bizID, from, to,
		bizID, from, to,
		bizID, from, to,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CashflowRow
	for rows.Next() {
		var r CashflowRow
		_ = rows.Scan(&r.Period, &r.CashIn, &r.CashOut)
		r.NetCashflow = r.CashIn - r.CashOut
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *FinanceStore) CashBalance(bizID int) (float64, error) {
	var cashIn, cashOut float64
	_ = s.db.QueryRow(`SELECT COALESCE(SUM(grand_total),0) FROM pos_sales WHERE business_id=? AND payment_method='cash' AND status='completed'`, bizID).Scan(&cashIn)
	var custCash float64
	_ = s.db.QueryRow(`SELECT COALESCE(SUM(amount),0) FROM customer_payments WHERE business_id=? AND payment_method='cash'`, bizID).Scan(&custCash)
	cashIn += custCash
	_ = s.db.QueryRow(`SELECT COALESCE(SUM(amount),0) FROM supplier_payments WHERE business_id=? AND payment_method='cash'`, bizID).Scan(&cashOut)
	var expCash float64
	_ = s.db.QueryRow(`SELECT COALESCE(SUM(amount),0) FROM expenses WHERE business_id=? AND payment_method='cash' AND status='approved'`, bizID).Scan(&expCash)
	cashOut += expCash
	return cashIn - cashOut, nil
}

// ── GST ───────────────────────────────────────────────────────────────────────

func (s *FinanceStore) GSTByMonth(bizID int, from, to string) ([]GSTMonthRow, error) {
	rows, err := s.db.Query(`
		SELECT period, SUM(output_gst), SUM(input_gst) FROM (
			SELECT DATE_FORMAT(s.created_at,'%Y-%m') as period, SUM(i.tax_amount) as output_gst, 0.0 as input_gst
			FROM pos_sale_items i JOIN pos_sales s ON s.id=i.sale_id
			WHERE s.business_id=? AND s.status='completed' AND DATE(s.created_at) BETWEEN ? AND ?
			GROUP BY period

			UNION ALL

			SELECT DATE_FORMAT(o.created_at,'%Y-%m'), SUM(i.tax_amount), 0.0
			FROM sales_order_items i JOIN sales_orders o ON o.id=i.order_id
			WHERE o.business_id=? AND o.status IN ('completed','delivered') AND DATE(o.created_at) BETWEEN ? AND ?
			GROUP BY period

			UNION ALL

			SELECT DATE_FORMAT(o.created_at,'%Y-%m'), 0.0, SUM(i.tax_amount)
			FROM procurement_order_items i JOIN procurement_orders o ON o.id=i.order_id
			WHERE o.business_id=? AND o.status='completed' AND DATE(o.created_at) BETWEEN ? AND ?
			GROUP BY period
		) sub GROUP BY period ORDER BY period ASC`,
		bizID, from, to,
		bizID, from, to,
		bizID, from, to,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GSTMonthRow
	for rows.Next() {
		var r GSTMonthRow
		_ = rows.Scan(&r.Month, &r.OutputGST, &r.InputGST)
		r.Net = r.OutputGST - r.InputGST
		out = append(out, r)
	}
	return out, rows.Err()
}

// ── Dashboard aggregation ────────────────────────────────────────────────────

func (s *FinanceStore) DashboardData(bizID int) (FinanceDashboard, error) {
	var d FinanceDashboard

	// Receivables = sum of all non-cancelled SO grand_total - sum of customer_payments
	var soTotal, custPaid float64
	_ = s.db.QueryRow(`SELECT COALESCE(SUM(grand_total),0) FROM sales_orders WHERE business_id=? AND status NOT IN ('draft','cancelled')`, bizID).Scan(&soTotal)
	_ = s.db.QueryRow(`SELECT COALESCE(SUM(amount),0) FROM customer_payments WHERE business_id=?`, bizID).Scan(&custPaid)
	d.TotalReceivables = soTotal - custPaid
	if d.TotalReceivables < 0 {
		d.TotalReceivables = 0
	}

	// Payables = sum of completed PO - sum of supplier_payments
	var poTotal, suppPaid float64
	_ = s.db.QueryRow(`SELECT COALESCE(SUM(grand_total),0) FROM procurement_orders WHERE business_id=? AND status='completed'`, bizID).Scan(&poTotal)
	_ = s.db.QueryRow(`SELECT COALESCE(SUM(amount),0) FROM supplier_payments WHERE business_id=?`, bizID).Scan(&suppPaid)
	d.TotalPayables = poTotal - suppPaid
	if d.TotalPayables < 0 {
		d.TotalPayables = 0
	}

	d.CashBalance, _ = s.CashBalance(bizID)
	d.TotalBankBalance, _ = s.TotalBankBalance(bizID)

	// This month
	now := time.Now()
	from := fmt.Sprintf("%d-%02d-01", now.Year(), now.Month())
	to := now.Format("2006-01-02")

	var posRev, soRev float64
	_ = s.db.QueryRow(`SELECT COALESCE(SUM(grand_total),0) FROM pos_sales WHERE business_id=? AND status='completed' AND DATE(created_at) BETWEEN ? AND ?`, bizID, from, to).Scan(&posRev)
	_ = s.db.QueryRow(`SELECT COALESCE(SUM(grand_total),0) FROM sales_orders WHERE business_id=? AND status IN ('completed','delivered') AND DATE(created_at) BETWEEN ? AND ?`, bizID, from, to).Scan(&soRev)
	d.MonthRevenue = posRev + soRev

	d.MonthExpenses, _ = s.SumExpenses(bizID, from, to)
	d.MonthProfit = d.MonthRevenue - d.MonthExpenses
	d.PendingExpenses, _ = s.CountPendingExpenses(bizID)

	var bankCount int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM bank_accounts WHERE business_id=? AND status='active'`, bizID).Scan(&bankCount)
	d.TotalBankAccounts = bankCount

	// Recent expenses
	d.RecentExpenses, _ = s.ListExpenses(bizID, "", "", 0)
	if len(d.RecentExpenses) > 5 {
		d.RecentExpenses = d.RecentExpenses[:5]
	}

	return d, nil
}
