package services

import (
	"errors"
	"fmt"

	"go-monolith/models"
)

type Field struct {
	Name  string
	Label string
	Type  string
	Value string
}

type ModuleConfig struct {
	Key         string
	Title       string
	Path        string
	Table       string
	Description string
	Fields      []Field
	Columns     []Field
}

type ModuleService struct {
	store   *models.ModuleStore
	modules map[string]ModuleConfig
}

func NewModuleService(store *models.ModuleStore) *ModuleService {
	configs := []ModuleConfig{
		{
			Key: "customers", Title: "Customers", Path: "/customers", Table: "customers",
			Description: "Customer profiles, GST details, billing addresses, and credit note history.",
			Fields:      []Field{{"name", "Name", "text", ""}, {"email", "Email", "email", ""}, {"phone", "Phone", "text", ""}, {"gstin", "GSTIN", "text", ""}},
		},
		{
			Key: "categories", Title: "Categories", Path: "/categories", Table: "categories",
			Description: "Group products and maintain catalog structure.",
			Fields:      []Field{{"name", "Name", "text", ""}, {"description", "Description", "text", ""}},
		},
		{
			Key: "vendors", Title: "Vendors", Path: "/vendors", Table: "vendors",
			Description: "Vendor records, deactivation, and purchase order relationships.",
			Fields:      []Field{{"name", "Name", "text", ""}, {"email", "Email", "email", ""}, {"phone", "Phone", "text", ""}, {"status", "Status", "text", "active"}},
		},
		{
			Key: "invoices", Title: "Invoices", Path: "/invoices", Table: "invoices",
			Description: "Create invoices, manage invoice status, payments, PDFs, and stock deduction.",
			Fields:      []Field{{"number", "Invoice Number", "text", ""}, {"customer", "Customer", "text", ""}, {"total", "Total", "number", ""}, {"status", "Status", "text", "pending"}},
		},
		{
			Key: "purchase-orders", Title: "Purchase Orders", Path: "/purchase-orders", Table: "purchase_orders",
			Description: "Purchase order creation, receiving, and stock credits.",
			Fields:      []Field{{"number", "PO Number", "text", ""}, {"vendor", "Vendor", "text", ""}, {"total", "Total", "number", ""}, {"status", "Status", "text", "draft"}},
		},
		{
			Key: "users", Title: "Users", Path: "/users", Table: "users",
			Description: "Staff accounts and role assignment for the monolith.",
			Fields:      []Field{{"name", "Name", "text", ""}, {"email", "Email", "email", ""}, {"role", "Role", "text", "staff"}},
		},
		{
			Key: "payments", Title: "Payments", Path: "/payments", Table: "payments",
			Description: "Payment records linked to invoices.",
			Fields:      []Field{{"invoice", "Invoice", "text", ""}, {"amount", "Amount", "number", ""}, {"method", "Method", "text", "cash"}},
		},
		{
			Key: "credit-notes", Title: "Credit Notes", Path: "/credit-notes", Table: "credit_notes",
			Description: "Credit note records for invoice returns and customer credits.",
			Fields:      []Field{{"number", "CN Number", "text", ""}, {"customer", "Customer", "text", ""}, {"total", "Total", "number", ""}, {"status", "Status", "text", "issued"}},
		},
		{
			Key: "jobs", Title: "Jobs", Path: "/jobs", Table: "jobs",
			Description: "Background job tracking for async work like PDF generation.",
			Fields:      []Field{{"name", "Name", "text", ""}, {"status", "Status", "text", "queued"}, {"detail", "Detail", "text", ""}},
		},
		{
			Key: "accounts", Title: "Accounting", Path: "/accounts", Table: "accounts",
			Description: "Simple chart of accounts and balances.",
			Fields:      []Field{{"name", "Name", "text", ""}, {"type", "Type", "text", "asset"}, {"balance", "Balance", "number", "0"}},
		},
	}

	modules := map[string]ModuleConfig{}
	for _, config := range configs {
		config.Columns = config.Fields
		modules[config.Key] = config
	}
	return &ModuleService{store: store, modules: modules}
}

func (s *ModuleService) Config(key string) (ModuleConfig, bool) {
	config, ok := s.modules[key]
	return config, ok
}

// ConfigOnly is an alias for Config for readability in handlers.
func (s *ModuleService) ConfigOnly(key string) (ModuleConfig, bool) {
	return s.Config(key)
}

func (s *ModuleService) List(key string) (ModuleConfig, []models.Record, error) {
	config, err := s.mustConfig(key)
	if err != nil {
		return ModuleConfig{}, nil, err
	}
	records, err := s.store.List(config.Table, fieldNames(config.Fields))
	return config, records, err
}

func (s *ModuleService) ListPaged(key string, page, perPage int, search, sortCol, sortDir string) (ModuleConfig, models.PageResult, error) {
	config, err := s.mustConfig(key)
	if err != nil {
		return ModuleConfig{}, models.PageResult{}, err
	}
	result, err := s.store.ListPaged(config.Table, fieldNames(config.Fields), page, perPage, search, sortCol, sortDir)
	return config, result, err
}

func (s *ModuleService) Trash(key string) (ModuleConfig, []models.Record, error) {
	config, err := s.mustConfig(key)
	if err != nil {
		return ModuleConfig{}, nil, err
	}
	records, err := s.store.Trash(config.Table, fieldNames(config.Fields))
	return config, records, err
}

func (s *ModuleService) Restore(key string, id int) error {
	config, err := s.mustConfig(key)
	if err != nil {
		return err
	}
	return s.store.Restore(config.Table, id)
}

func (s *ModuleService) HardDelete(key string, id int) error {
	config, err := s.mustConfig(key)
	if err != nil {
		return err
	}
	return s.store.HardDelete(config.Table, id)
}

func (s *ModuleService) Get(key string, id int) (ModuleConfig, models.Record, error) {
	config, err := s.mustConfig(key)
	if err != nil {
		return ModuleConfig{}, nil, err
	}
	record, err := s.store.Get(config.Table, fieldNames(config.Fields), id)
	return config, record, err
}

func (s *ModuleService) Create(key string, values map[string]string) error {
	config, err := s.mustConfig(key)
	if err != nil {
		return err
	}
	if values["name"] == "" && values["number"] == "" {
		return errors.New("primary field is required")
	}
	return s.store.Create(config.Table, fieldNames(config.Fields), fieldValues(config.Fields, values))
}

func (s *ModuleService) Update(key string, id int, values map[string]string) error {
	config, err := s.mustConfig(key)
	if err != nil {
		return err
	}
	return s.store.Update(config.Table, fieldNames(config.Fields), fieldValues(config.Fields, values), id)
}

func (s *ModuleService) Delete(key string, id int) error {
	config, err := s.mustConfig(key)
	if err != nil {
		return err
	}
	return s.store.Delete(config.Table, id)
}

func (s *ModuleService) Counts() (map[string]int, error) {
	out := map[string]int{}
	for key, config := range s.modules {
		count, err := s.store.Count(config.Table)
		if err != nil {
			return nil, err
		}
		out[key] = count
	}
	return out, nil
}

func (s *ModuleService) StockLogs() ([]models.Record, error) {
	return s.store.StockLogs()
}

func (s *ModuleService) RecentActivity(limit int) ([]models.Record, error) {
	return s.store.RecentActivity(limit)
}

func (s *ModuleService) TopCustomers(limit int) ([]models.Record, error) {
	return s.store.TopCustomers(limit)
}

func (s *ModuleService) PendingInvoicesTotal() (float64, error) {
	return s.store.PendingInvoicesTotal()
}

func (s *ModuleService) Totals() (map[string]string, error) {
	invoices, err := s.store.Sum("invoices", "total")
	if err != nil {
		return nil, err
	}
	pos, err := s.store.Sum("purchase_orders", "total")
	if err != nil {
		return nil, err
	}
	return map[string]string{
		"invoice_total": fmt.Sprintf("Rs. %.2f", invoices),
		"po_total":      fmt.Sprintf("Rs. %.2f", pos),
	}, nil
}

// GetCustomerByName looks up a customer by their name field.
func (s *ModuleService) GetCustomerByName(name string) (models.Record, error) {
	return s.store.FindByField("customers", "name", name, []string{"name", "email", "phone", "gstin"})
}

func (s *ModuleService) mustConfig(key string) (ModuleConfig, error) {
	config, ok := s.Config(key)
	if !ok {
		return ModuleConfig{}, errors.New("unknown module")
	}
	return config, nil
}

func fieldNames(fields []Field) []string {
	names := make([]string, len(fields))
	for i, field := range fields {
		names[i] = field.Name
	}
	return names
}

func fieldValues(fields []Field, values map[string]string) []string {
	out := make([]string, len(fields))
	for i, field := range fields {
		out[i] = values[field.Name]
	}
	return out
}
