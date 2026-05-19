package services

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"go-monolith/models"
)

// FinanceService provides inventory-focused financial summaries.
// It does NOT implement double-entry accounting — only aggregation from
// existing transactional tables (POS, sales orders, procurement, expenses, bank).
type FinanceService struct {
	store *models.FinanceStore
}

func NewFinanceService(store *models.FinanceStore) *FinanceService {
	return &FinanceService{store: store}
}

// ── Expense Categories ────────────────────────────────────────────────────────

func (s *FinanceService) ListCategories(bizID int) ([]models.ExpenseCategory, error) {
	// Ensure defaults exist on first access.
	_ = s.store.EnsureDefaultCategories(bizID)
	return s.store.ListCategories(bizID)
}

func (s *FinanceService) CreateCategory(bizID int, name, description string) (*models.ExpenseCategory, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("category name is required")
	}
	return s.store.CreateCategory(bizID, name, strings.TrimSpace(description))
}

// ── Expenses ─────────────────────────────────────────────────────────────────

func (s *FinanceService) ListExpenses(bizID int, from, to string, categoryID int) ([]models.Expense, error) {
	if from == "" {
		from = time.Now().AddDate(0, -1, 0).Format("2006-01-02")
	}
	if to == "" {
		to = time.Now().Format("2006-01-02")
	}
	return s.store.ListExpenses(bizID, from, to, categoryID)
}

func (s *FinanceService) CreateExpense(bizID, categoryID int, amount float64, paymentMethod string, bankAccountID *int, reference, description string, expenseDate time.Time, status string) (*models.Expense, error) {
	if amount <= 0 {
		return nil, errors.New("expense amount must be greater than zero")
	}
	validMethods := map[string]bool{"cash": true, "bank": true, "upi": true, "card": true, "cheque": true}
	if !validMethods[paymentMethod] {
		paymentMethod = "cash"
	}
	if status == "" {
		status = "approved"
	}
	return s.store.CreateExpense(&models.Expense{
		BusinessID:    bizID,
		CategoryID:    categoryID,
		Amount:        math.Round(amount*100) / 100,
		PaymentMethod: paymentMethod,
		BankAccountID: bankAccountID,
		Reference:     strings.TrimSpace(reference),
		Description:   strings.TrimSpace(description),
		ExpenseDate:   expenseDate,
		Status:        status,
	})
}

func (s *FinanceService) ApproveExpense(id, bizID int) error {
	return s.store.UpdateExpenseStatus(id, "approved")
}

func (s *FinanceService) RejectExpense(id, bizID int) error {
	return s.store.UpdateExpenseStatus(id, "rejected")
}

// ── Bank ─────────────────────────────────────────────────────────────────────

func (s *FinanceService) ListBankAccounts(bizID int) ([]models.BankAccount, error) {
	return s.store.ListBankAccounts(bizID)
}

func (s *FinanceService) CreateBankAccount(bizID int, accountName, bankName, accountNumber, ifsc string, openingBalance float64) (*models.BankAccount, error) {
	accountName = strings.TrimSpace(accountName)
	if accountName == "" {
		return nil, errors.New("account name is required")
	}
	return s.store.CreateBankAccount(&models.BankAccount{
		BusinessID:     bizID,
		AccountName:    accountName,
		BankName:       strings.TrimSpace(bankName),
		AccountNumber:  strings.TrimSpace(accountNumber),
		IFSC:           strings.TrimSpace(ifsc),
		OpeningBalance: openingBalance,
	})
}

func (s *FinanceService) AddBankTransaction(bizID, accountID int, txnType string, amount float64, reference, description string, txnDate time.Time) error {
	if amount <= 0 {
		return errors.New("transaction amount must be greater than zero")
	}
	if txnType != "credit" && txnType != "debit" {
		return errors.New("transaction type must be 'credit' or 'debit'")
	}
	return s.store.AddBankTransaction(&models.BankTransaction{
		BusinessID:      bizID,
		AccountID:       accountID,
		TransactionType: txnType,
		Amount:          math.Round(amount*100) / 100,
		Reference:       strings.TrimSpace(reference),
		Description:     strings.TrimSpace(description),
		TransactionDate: txnDate,
	})
}

func (s *FinanceService) ListBankTransactions(bizID, accountID int) ([]models.BankTransaction, error) {
	return s.store.ListBankTransactions(bizID, accountID)
}

// ── Ledgers ───────────────────────────────────────────────────────────────────

// CustomerLedger returns all debit/credit entries for a customer with a running balance.
// Positive closing balance = customer owes us money.
func (s *FinanceService) CustomerLedger(bizID, customerID int, from, to string) ([]models.LedgerEntry, float64, error) {
	entries, err := s.store.CustomerLedger(bizID, customerID, from, to)
	if err != nil {
		return nil, 0, err
	}
	return computeRunningBalance(entries)
}

// SupplierLedger returns all debit/credit entries for a supplier with a running balance.
// Positive closing balance = we owe the supplier money.
func (s *FinanceService) SupplierLedger(bizID, supplierID int, from, to string) ([]models.LedgerEntry, float64, error) {
	entries, err := s.store.SupplierLedger(bizID, supplierID, from, to)
	if err != nil {
		return nil, 0, err
	}
	return computeRunningBalance(entries)
}

// CashLedger returns all cash in/out entries with a running cash balance.
func (s *FinanceService) CashLedger(bizID int, from, to string) ([]models.LedgerEntry, float64, error) {
	entries, err := s.store.CashLedger(bizID, from, to)
	if err != nil {
		return nil, 0, err
	}
	return computeRunningBalance(entries)
}

// computeRunningBalance fills in the Balance field for each entry in sequence.
// Credit = increase balance, Debit = decrease balance.
func computeRunningBalance(entries []models.LedgerEntry) ([]models.LedgerEntry, float64, error) {
	var balance float64
	for i := range entries {
		balance += entries[i].Credit - entries[i].Debit
		entries[i].Balance = math.Round(balance*100) / 100
	}
	return entries, math.Round(balance*100) / 100, nil
}

// ── P&L ───────────────────────────────────────────────────────────────────────

func (s *FinanceService) ProfitLoss(bizID int, from, to string) (models.PLReport, error) {
	return s.store.PLData(bizID, from, to)
}

// ── Cashflow ──────────────────────────────────────────────────────────────────

func (s *FinanceService) CashflowByMonth(bizID int, from, to string) ([]models.CashflowRow, error) {
	return s.store.CashflowByMonth(bizID, from, to)
}

// ── GST ───────────────────────────────────────────────────────────────────────

func (s *FinanceService) GSTSummary(bizID int, from, to string) (models.GSTReport, error) {
	report := models.GSTReport{From: from, To: to}
	monthly, err := s.store.GSTByMonth(bizID, from, to)
	if err != nil {
		return report, err
	}
	report.ByMonth = monthly
	for _, m := range monthly {
		report.OutputGST += m.OutputGST
		report.InputGST += m.InputGST
	}
	report.NetGSTPayable = math.Round((report.OutputGST-report.InputGST)*100) / 100
	return report, nil
}

// ── Dashboard ─────────────────────────────────────────────────────────────────

func (s *FinanceService) Dashboard(bizID int) (models.FinanceDashboard, error) {
	return s.store.DashboardData(bizID)
}

// ── CSV Export helpers ────────────────────────────────────────────────────────

func FormatLedgerCSV(entries []models.LedgerEntry) ([]string, [][]string) {
	headers := []string{"Date", "Description", "Reference", "Type", "Debit (Rs.)", "Credit (Rs.)", "Balance (Rs.)"}
	var rows [][]string
	for _, e := range entries {
		rows = append(rows, []string{
			e.TxnDate.Format("2006-01-02"),
			e.Description,
			e.RefNumber,
			e.RefType,
			fmt.Sprintf("%.2f", e.Debit),
			fmt.Sprintf("%.2f", e.Credit),
			fmt.Sprintf("%.2f", e.Balance),
		})
	}
	return headers, rows
}
