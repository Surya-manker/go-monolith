package handlers

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go-monolith/models"
	"go-monolith/services"
)

// ── Financial Dashboard ───────────────────────────────────────────────────────

type FinanceDashboardData struct {
	AppContext
	Dashboard models.FinanceDashboard
}

func (a *App) FinanceDashboard(w http.ResponseWriter, r *http.Request) {
	d, _ := a.FinanceService.Dashboard(a.bizID(r))
	a.Renderer.Page(w, "finance_dashboard.html", FinanceDashboardData{AppContext: a.ctx(r), Dashboard: d})
}

// ── Ledgers ───────────────────────────────────────────────────────────────────

type LedgerPageData struct {
	AppContext
	LedgerType  string // customer|supplier|cash
	Entries     []models.LedgerEntry
	Closing     float64
	Customers   []models.CRMCustomer
	Suppliers   []models.Supplier
	CustomerID  int
	SupplierID  int
	From        string
	To          string
	Error       string
}

func (a *App) FinanceLedger(w http.ResponseWriter, r *http.Request) {
	bizID := a.bizID(r)
	ltype := r.URL.Query().Get("type")
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	if from == "" {
		from = time.Now().AddDate(0, -1, 0).Format("2006-01-02")
	}
	if to == "" {
		to = time.Now().Format("2006-01-02")
	}

	customerID, _ := strconv.Atoi(r.URL.Query().Get("customer_id"))
	supplierID, _ := strconv.Atoi(r.URL.Query().Get("supplier_id"))

	var entries []models.LedgerEntry
	var closing float64

	switch ltype {
	case "customer":
		if customerID > 0 {
			entries, closing, _ = a.FinanceService.CustomerLedger(bizID, customerID, from, to)
		}
	case "supplier":
		if supplierID > 0 {
			entries, closing, _ = a.FinanceService.SupplierLedger(bizID, supplierID, from, to)
		}
	default:
		ltype = "cash"
		entries, closing, _ = a.FinanceService.CashLedger(bizID, from, to)
	}

	// Handle CSV export
	if r.URL.Query().Get("format") == "csv" {
		headers, rows := exportLedgerCSV(entries, ltype)
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s_ledger_%s_%s.csv"`, ltype, from, to))
		cw := csv.NewWriter(w)
		_ = cw.Write(headers)
		for _, row := range rows {
			_ = cw.Write(row)
		}
		cw.Flush()
		return
	}

	customers, _ := a.CRMService.ListCustomers(bizID)
	suppliers, _ := a.ProcurementService.ListSuppliers(bizID)

	a.Renderer.Page(w, "finance_ledger.html", LedgerPageData{
		AppContext:  a.ctx(r),
		LedgerType: ltype,
		Entries:    entries,
		Closing:    closing,
		Customers:  customers,
		Suppliers:  suppliers,
		CustomerID: customerID,
		SupplierID: supplierID,
		From:       from,
		To:         to,
	})
}

// ── Expenses ─────────────────────────────────────────────────────────────────

type ExpensesPageData struct {
	AppContext
	Expenses      []models.Expense
	Categories    []models.ExpenseCategory
	BankAccounts  []models.BankAccount
	CategoryID    int
	From          string
	To            string
	TotalAmount   float64
	PendingCount  int
	Error         string
}

func (a *App) FinanceExpenses(w http.ResponseWriter, r *http.Request) {
	bizID := a.bizID(r)
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	categoryID, _ := strconv.Atoi(r.URL.Query().Get("category_id"))

	expenses, _ := a.FinanceService.ListExpenses(bizID, from, to, categoryID)
	categories, _ := a.FinanceService.ListCategories(bizID)
	bankAccounts, _ := a.FinanceService.ListBankAccounts(bizID)

	var total float64
	var pending int
	for _, e := range expenses {
		if e.Status == "approved" {
			total += e.Amount
		}
		if e.Status == "pending" {
			pending++
		}
	}

	if from == "" {
		from = time.Now().AddDate(0, -1, 0).Format("2006-01-02")
	}
	if to == "" {
		to = time.Now().Format("2006-01-02")
	}

	a.Renderer.Page(w, "finance_expenses.html", ExpensesPageData{
		AppContext:    a.ctx(r),
		Expenses:     expenses,
		Categories:   categories,
		BankAccounts: bankAccounts,
		CategoryID:   categoryID,
		From:         from,
		To:           to,
		TotalAmount:  total,
		PendingCount: pending,
	})
}

func (a *App) FinanceExpenseCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	bizID := a.bizID(r)
	categoryID, _ := strconv.Atoi(r.FormValue("category_id"))
	amount, _ := strconv.ParseFloat(r.FormValue("amount"), 64)
	paymentMethod := r.FormValue("payment_method")
	status := r.FormValue("status")
	if status == "" {
		status = "approved"
	}

	var bankAccountID *int
	if bid, err := strconv.Atoi(r.FormValue("bank_account_id")); err == nil && bid > 0 {
		bankAccountID = &bid
	}

	expenseDate := time.Now()
	if v := r.FormValue("expense_date"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			expenseDate = t
		}
	}

	expense, err := a.FinanceService.CreateExpense(
		bizID, categoryID, amount, paymentMethod, bankAccountID,
		r.FormValue("reference"), r.FormValue("description"), expenseDate, status,
	)
	if err != nil {
		setToast(w, err.Error(), "error")
		http.Redirect(w, r, "/finance/expenses", http.StatusSeeOther)
		return
	}
	a.auditLog(r, "expenses", "create", strconv.Itoa(expense.ID), map[string]string{
		"amount": fmt.Sprintf("%.2f", expense.Amount),
	})
	setToast(w, fmt.Sprintf("Expense Rs. %.2f recorded", expense.Amount), "success")
	http.Redirect(w, r, "/finance/expenses", http.StatusSeeOther)
}

func (a *App) FinanceExpenseApprove(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.URL.Query().Get("id"))
	if err := a.FinanceService.ApproveExpense(id, a.bizID(r)); err != nil {
		setToast(w, err.Error(), "error")
	} else {
		setToast(w, "Expense approved", "success")
	}
	http.Redirect(w, r, "/finance/expenses", http.StatusSeeOther)
}

func (a *App) FinanceExpenseReject(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.URL.Query().Get("id"))
	if err := a.FinanceService.RejectExpense(id, a.bizID(r)); err != nil {
		setToast(w, err.Error(), "error")
	} else {
		setToast(w, "Expense rejected", "warning")
	}
	http.Redirect(w, r, "/finance/expenses", http.StatusSeeOther)
}

func (a *App) FinanceCategoryCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	_, err := a.FinanceService.CreateCategory(a.bizID(r), r.FormValue("name"), r.FormValue("description"))
	if err != nil {
		setToast(w, err.Error(), "error")
	} else {
		setToast(w, "Category created", "success")
	}
	http.Redirect(w, r, "/finance/expenses", http.StatusSeeOther)
}

// ── Bank ─────────────────────────────────────────────────────────────────────

type BankPageData struct {
	AppContext
	Accounts     []models.BankAccount
	Transactions []models.BankTransaction
	SelectedID   int
	TotalBalance float64
}

func (a *App) FinanceBank(w http.ResponseWriter, r *http.Request) {
	bizID := a.bizID(r)
	accountID, _ := strconv.Atoi(r.URL.Query().Get("account_id"))
	accounts, _ := a.FinanceService.ListBankAccounts(bizID)
	txns, _ := a.FinanceService.ListBankTransactions(bizID, accountID)

	var total float64
	for _, acc := range accounts {
		if acc.Status == "active" {
			total += acc.CurrentBalance
		}
	}

	a.Renderer.Page(w, "finance_bank.html", BankPageData{
		AppContext:    a.ctx(r),
		Accounts:     accounts,
		Transactions: txns,
		SelectedID:   accountID,
		TotalBalance: total,
	})
}

func (a *App) FinanceBankCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	openingBalance, _ := strconv.ParseFloat(r.FormValue("opening_balance"), 64)
	acc, err := a.FinanceService.CreateBankAccount(
		a.bizID(r),
		r.FormValue("account_name"), r.FormValue("bank_name"),
		r.FormValue("account_number"), r.FormValue("ifsc"), openingBalance,
	)
	if err != nil {
		setToast(w, err.Error(), "error")
	} else {
		setToast(w, "Bank account "+acc.AccountName+" added", "success")
	}
	http.Redirect(w, r, "/finance/bank", http.StatusSeeOther)
}

func (a *App) FinanceBankTransaction(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	accountID, _ := strconv.Atoi(r.FormValue("account_id"))
	amount, _ := strconv.ParseFloat(r.FormValue("amount"), 64)
	txnDate := time.Now()
	if v := r.FormValue("transaction_date"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			txnDate = t
		}
	}
	err := a.FinanceService.AddBankTransaction(
		a.bizID(r), accountID, r.FormValue("transaction_type"), amount,
		r.FormValue("reference"), r.FormValue("description"), txnDate,
	)
	if err != nil {
		setToast(w, err.Error(), "error")
	} else {
		setToast(w, "Transaction recorded", "success")
	}
	http.Redirect(w, r, "/finance/bank?account_id="+strconv.Itoa(accountID), http.StatusSeeOther)
}

// ── P&L ───────────────────────────────────────────────────────────────────────

type PLPageData struct {
	AppContext
	Report models.PLReport
	Filter services.ReportFilter
}

func (a *App) FinancePL(w http.ResponseWriter, r *http.Request) {
	f := parseFilter(r)
	report, _ := a.FinanceService.ProfitLoss(a.bizID(r), f.From, f.To)
	a.Renderer.Page(w, "finance_pl.html", PLPageData{AppContext: a.ctx(r), Report: report, Filter: f})
}

// ── Cashflow ──────────────────────────────────────────────────────────────────

type CashflowPageData struct {
	AppContext
	Rows   []models.CashflowRow
	Filter services.ReportFilter
	TotalIn  float64
	TotalOut float64
	NetFlow  float64
}

func (a *App) FinanceCashflow(w http.ResponseWriter, r *http.Request) {
	f := parseFilter(r)
	rows, _ := a.FinanceService.CashflowByMonth(a.bizID(r), f.From, f.To)

	var totalIn, totalOut float64
	for _, row := range rows {
		totalIn += row.CashIn
		totalOut += row.CashOut
	}

	a.Renderer.Page(w, "finance_cashflow.html", CashflowPageData{
		AppContext: a.ctx(r),
		Rows:      rows,
		Filter:    f,
		TotalIn:   totalIn,
		TotalOut:  totalOut,
		NetFlow:   totalIn - totalOut,
	})
}

// ── GST Summary ───────────────────────────────────────────────────────────────

type GSTPageData struct {
	AppContext
	Report models.GSTReport
	Filter services.ReportFilter
}

func (a *App) FinanceGST(w http.ResponseWriter, r *http.Request) {
	f := parseFilter(r)
	report, _ := a.FinanceService.GSTSummary(a.bizID(r), f.From, f.To)
	a.Renderer.Page(w, "finance_gst.html", GSTPageData{AppContext: a.ctx(r), Report: report, Filter: f})
}

// ── Export ────────────────────────────────────────────────────────────────────

func (a *App) FinanceExport(w http.ResponseWriter, r *http.Request) {
	bizID := a.bizID(r)
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	if from == "" {
		from = time.Now().AddDate(0, -1, 0).Format("2006-01-02")
	}
	if to == "" {
		to = time.Now().Format("2006-01-02")
	}
	rtype := r.URL.Query().Get("type")

	var headers []string
	var csvRows [][]string
	filename := rtype + "_" + time.Now().Format("20060102") + ".csv"

	switch rtype {
	case "expenses":
		expenses, _ := a.FinanceService.ListExpenses(bizID, from, to, 0)
		headers = []string{"Date", "Category", "Description", "Method", "Reference", "Amount", "Status"}
		for _, e := range expenses {
			csvRows = append(csvRows, []string{
				e.ExpenseDate.Format("2006-01-02"), e.CategoryName, e.Description,
				e.PaymentMethod, e.Reference, fmt.Sprintf("%.2f", e.Amount), e.Status,
			})
		}
	case "cashflow":
		rows, _ := a.FinanceService.CashflowByMonth(bizID, from, to)
		headers = []string{"Period", "Cash In (Rs.)", "Cash Out (Rs.)", "Net (Rs.)"}
		for _, row := range rows {
			csvRows = append(csvRows, []string{
				row.Period, fmt.Sprintf("%.2f", row.CashIn),
				fmt.Sprintf("%.2f", row.CashOut), fmt.Sprintf("%.2f", row.NetCashflow),
			})
		}
	case "pl":
		report, _ := a.FinanceService.ProfitLoss(bizID, from, to)
		headers = []string{"Item", "Amount (Rs.)"}
		csvRows = [][]string{
			{"Revenue (POS)", fmt.Sprintf("%.2f", report.POSRevenue)},
			{"Revenue (Sales Orders)", fmt.Sprintf("%.2f", report.SORevenue)},
			{"Total Revenue", fmt.Sprintf("%.2f", report.TotalRevenue)},
			{"Sales Returns", fmt.Sprintf("%.2f", report.SalesReturns)},
			{"Net Revenue", fmt.Sprintf("%.2f", report.NetRevenue)},
			{"COGS (Procurement)", fmt.Sprintf("%.2f", report.COGS)},
			{"Gross Profit", fmt.Sprintf("%.2f", report.GrossProfit)},
			{"Total Expenses", fmt.Sprintf("%.2f", report.TotalExpenses)},
			{"Net Profit", fmt.Sprintf("%.2f", report.NetProfit)},
		}
	default:
		http.Error(w, "unknown export type", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	cw := csv.NewWriter(w)
	_ = cw.Write(headers)
	for _, row := range csvRows {
		_ = cw.Write(row)
	}
	cw.Flush()
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func exportLedgerCSV(entries []models.LedgerEntry, ltype string) ([]string, [][]string) {
	headers := []string{"Date", "Description", "Reference", "Credit (Rs.)", "Debit (Rs.)", "Balance (Rs.)"}
	var rows [][]string
	for _, e := range entries {
		rows = append(rows, []string{
			e.TxnDate.Format("2006-01-02"), e.Description, e.RefNumber,
			fmt.Sprintf("%.2f", e.Credit), fmt.Sprintf("%.2f", e.Debit),
			fmt.Sprintf("%.2f", e.Balance),
		})
	}
	_ = strings.ToLower(ltype) // suppress unused warning
	return headers, rows
}
