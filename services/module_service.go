package services

import (
	"errors"
	"fmt"
	"time"

	"go-monolith/models"
)

// numberPrefixes lists modules that have an auto-generated "number" field.
// When the user submits an empty number, we generate one automatically.
var numberPrefixes = map[string]string{
	"invoices":       "INV",
	"purchase-orders": "PO",
	"credit-notes":   "CN",
}

type Field struct {
	Name        string
	Label       string
	Type        string
	Value       string
	Placeholder string
}

type ModuleConfig struct {
	Key          string
	Title        string
	Path         string
	Table        string
	Description  string
	Fields       []Field
	Columns      []Field
	// Placeholders is a safe map for template access (index never panics on missing key).
	Placeholders map[string]string
}

type ModuleService struct {
	store   models.IModuleStore
	modules map[string]ModuleConfig
}

func NewModuleService(store models.IModuleStore) *ModuleService {
	// f is a helper to build a Field with all properties.
	f := func(name, label, typ, value, placeholder string) Field {
		return Field{Name: name, Label: label, Type: typ, Value: value, Placeholder: placeholder}
	}

	configs := []ModuleConfig{
		{
			Key: "customers", Title: "Customers", Path: "/customers", Table: "customers",
			Description: "Customer profiles, GST details, billing addresses.",
			Fields: []Field{
				f("name",  "Name",  "text",  "", "e.g. Sharma Enterprises"),
				f("email", "Email", "email", "", "billing@sharma.com"),
				f("phone", "Phone", "text",  "", "+91 98765 43210"),
				f("gstin", "GSTIN", "text",  "", "27AABCS1234A1Z5"),
			},
		},
		{
			Key: "categories", Title: "Categories", Path: "/categories", Table: "categories",
			Description: "Group products and maintain catalog structure.",
			Fields: []Field{
				f("name",        "Name",        "text", "", "e.g. Electronics"),
				f("description", "Description", "text", "", "Brief description"),
			},
		},
		{
			Key: "vendors", Title: "Vendors", Path: "/vendors", Table: "vendors",
			Description: "Vendor records and purchase order relationships.",
			Fields: []Field{
				f("name",   "Name",   "text",  "", "e.g. ABC Suppliers Pvt. Ltd."),
				f("email",  "Email",  "email", "", "contact@abcsuppliers.com"),
				f("phone",  "Phone",  "text",  "", "+91 98765 43210"),
				f("status", "Status", "text",  "active", "active"),
			},
		},
		{
			Key: "invoices", Title: "Invoices", Path: "/invoices", Table: "invoices",
			Description: "Create invoices, manage status, payments, PDFs, and stock deduction.",
			Fields: []Field{
				f("number",   "Invoice Number", "text",   "", "INV-2025-001"),
				f("customer", "Customer",       "text",   "", "Customer name"),
				f("total",    "Total (Rs.)",    "number", "", "0.00"),
				f("status",   "Status",         "text",   "pending", "pending"),
			},
		},
		{
			Key: "purchase-orders", Title: "Purchase Orders", Path: "/purchase-orders", Table: "purchase_orders",
			Description: "Purchase order creation, receiving, and stock credits.",
			Fields: []Field{
				f("number", "PO Number",    "text",   "", "PO-2025-001"),
				f("vendor", "Vendor",       "text",   "", "Vendor name"),
				f("total",  "Total (Rs.)",  "number", "", "0.00"),
				f("status", "Status",       "text",   "draft", "draft"),
			},
		},
		{
			Key: "users", Title: "Users", Path: "/users", Table: "users",
			Description: "Staff accounts and role assignment.",
			Fields: []Field{
				f("name",  "Full Name", "text",  "", "e.g. Rahul Sharma"),
				f("email", "Email",     "email", "", "rahul@example.com"),
				f("role",  "Role",      "text",  "staff", "staff / admin / accountant"),
			},
		},
		{
			Key: "payments", Title: "Payments", Path: "/payments", Table: "payments",
			Description: "Payment records linked to invoices.",
			Fields: []Field{
				f("invoice", "Invoice No.", "text",   "", "INV-2025-001"),
				f("amount",  "Amount (Rs.)","number", "", "0.00"),
				f("method",  "Method",      "text",   "cash", "cash / upi / bank transfer"),
			},
		},
		{
			Key: "credit-notes", Title: "Credit Notes", Path: "/credit-notes", Table: "credit_notes",
			Description: "Credit note records for invoice returns.",
			Fields: []Field{
				f("number",   "CN Number",   "text",   "", "CN-2025-001"),
				f("customer", "Customer",    "text",   "", "Customer name"),
				f("total",    "Total (Rs.)", "number", "", "0.00"),
				f("status",   "Status",      "text",   "issued", "issued / settled"),
			},
		},
		{
			Key: "jobs", Title: "Jobs", Path: "/jobs", Table: "jobs",
			Description: "Background job tracking for async work.",
			Fields: []Field{
				f("name",   "Job Name", "text", "", "e.g. PDF generation"),
				f("status", "Status",   "text", "queued", "queued / completed / failed"),
				f("detail", "Detail",   "text", "", "Additional details"),
			},
		},
	}

	modules := map[string]ModuleConfig{}
	for _, config := range configs {
		config.Columns = config.Fields

		// Build Placeholders map from Field.Placeholder so templates can use
		// safe map access (index never panics) instead of struct field access.
		ph := make(map[string]string, len(config.Fields))
		for _, field := range config.Fields {
			if field.Placeholder != "" {
				ph[field.Name] = field.Placeholder
			} else {
				ph[field.Name] = field.Label
			}
		}
		config.Placeholders = ph

		modules[config.Key] = config
	}
	return &ModuleService{store: store, modules: modules}
}

// WithStore returns a new ModuleService backed by a different store (e.g. demo in-memory store).
func (s *ModuleService) WithStore(store models.IModuleStore) *ModuleService {
	return &ModuleService{store: store, modules: s.modules}
}

func (s *ModuleService) Config(key string) (ModuleConfig, bool) {
	config, ok := s.modules[key]
	return config, ok
}

func (s *ModuleService) ConfigOnly(key string) (ModuleConfig, bool) {
	return s.Config(key)
}

func (s *ModuleService) List(key string, businessID int) (ModuleConfig, []models.Record, error) {
	config, err := s.mustConfig(key)
	if err != nil {
		return ModuleConfig{}, nil, err
	}
	records, err := s.store.List(config.Table, fieldNames(config.Fields), businessID)
	return config, records, err
}

func (s *ModuleService) ListPaged(key string, page, perPage int, search, sortCol, sortDir string, businessID int) (ModuleConfig, models.PageResult, error) {
	config, err := s.mustConfig(key)
	if err != nil {
		return ModuleConfig{}, models.PageResult{}, err
	}
	result, err := s.store.ListPaged(config.Table, fieldNames(config.Fields), page, perPage, search, sortCol, sortDir, businessID)
	return config, result, err
}

func (s *ModuleService) Trash(key string, businessID int) (ModuleConfig, []models.Record, error) {
	config, err := s.mustConfig(key)
	if err != nil {
		return ModuleConfig{}, nil, err
	}
	records, err := s.store.Trash(config.Table, fieldNames(config.Fields), businessID)
	return config, records, err
}

func (s *ModuleService) Restore(key string, id, businessID int) error {
	config, err := s.mustConfig(key)
	if err != nil {
		return err
	}
	return s.store.Restore(config.Table, id, businessID)
}

func (s *ModuleService) HardDelete(key string, id, businessID int) error {
	config, err := s.mustConfig(key)
	if err != nil {
		return err
	}
	return s.store.HardDelete(config.Table, id, businessID)
}

func (s *ModuleService) Get(key string, id, businessID int) (ModuleConfig, models.Record, error) {
	config, err := s.mustConfig(key)
	if err != nil {
		return ModuleConfig{}, nil, err
	}
	record, err := s.store.Get(config.Table, fieldNames(config.Fields), id, businessID)
	return config, record, err
}

func (s *ModuleService) Create(key string, values map[string]string, businessID int) error {
	config, err := s.mustConfig(key)
	if err != nil {
		return err
	}

	// Auto-generate "number" for modules like invoices, purchase-orders, credit-notes
	// when the user leaves the field blank.
	if prefix, ok := numberPrefixes[key]; ok && values["number"] == "" {
		count, _ := s.store.Count(config.Table, businessID)
		values["number"] = fmt.Sprintf("%s-%s-%04d", prefix, time.Now().Format("2006"), count+1)
	}

	if values["name"] == "" && values["number"] == "" {
		return errors.New("primary field is required")
	}
	return s.store.Create(config.Table, fieldNames(config.Fields), fieldValues(config.Fields, values), businessID)
}

func (s *ModuleService) Update(key string, id int, values map[string]string, businessID int) error {
	config, err := s.mustConfig(key)
	if err != nil {
		return err
	}
	return s.store.Update(config.Table, fieldNames(config.Fields), fieldValues(config.Fields, values), id, businessID)
}

func (s *ModuleService) Delete(key string, id, businessID int) error {
	config, err := s.mustConfig(key)
	if err != nil {
		return err
	}
	return s.store.Delete(config.Table, id, businessID)
}

func (s *ModuleService) Counts(businessID int) (map[string]int, error) {
	out := map[string]int{}
	for key, config := range s.modules {
		count, err := s.store.Count(config.Table, businessID)
		if err != nil {
			return nil, err
		}
		out[key] = count
	}
	return out, nil
}

func (s *ModuleService) StockLogs(businessID int) ([]models.Record, error) {
	return s.store.StockLogs(businessID)
}

func (s *ModuleService) RecentActivity(limit, businessID int) ([]models.Record, error) {
	return s.store.RecentActivity(limit, businessID)
}

func (s *ModuleService) TopCustomers(limit, businessID int) ([]models.Record, error) {
	return s.store.TopCustomers(limit, businessID)
}

func (s *ModuleService) PendingInvoicesTotal(businessID int) (float64, error) {
	return s.store.PendingInvoicesTotal(businessID)
}

func (s *ModuleService) Totals(businessID int) (map[string]string, error) {
	invoices, err := s.store.Sum("invoices", "total", businessID)
	if err != nil {
		return nil, err
	}
	pos, err := s.store.Sum("purchase_orders", "total", businessID)
	if err != nil {
		return nil, err
	}
	return map[string]string{
		"invoice_total": fmt.Sprintf("Rs. %.2f", invoices),
		"po_total":      fmt.Sprintf("Rs. %.2f", pos),
	}, nil
}

func (s *ModuleService) GetCustomerByName(name string, businessID int) (models.Record, error) {
	return s.store.FindByField("customers", "name", name, []string{"name", "email", "phone", "gstin"}, businessID)
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
