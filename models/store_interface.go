package models

// IModuleStore is satisfied by both ModuleStore (MySQL) and DemoStore (in-memory).
type IModuleStore interface {
	List(table string, columns []string, businessID int) ([]Record, error)
	ListPaged(table string, columns []string, page, perPage int, search, sortCol, sortDir string, businessID int) (PageResult, error)
	Trash(table string, columns []string, businessID int) ([]Record, error)
	Restore(table string, id, businessID int) error
	HardDelete(table string, id, businessID int) error
	Get(table string, columns []string, id, businessID int) (Record, error)
	Create(table string, columns []string, values []string, businessID int) error
	Update(table string, columns []string, values []string, id, businessID int) error
	Delete(table string, id, businessID int) error
	Count(table string, businessID int) (int, error)
	Sum(table, column string, businessID int) (float64, error)
	StockLogs(businessID int) ([]Record, error)
	RecentActivity(limit, businessID int) ([]Record, error)
	TopCustomers(limit, businessID int) ([]Record, error)
	PendingInvoicesTotal(businessID int) (float64, error)
	FindByField(table, field, value string, columns []string, businessID int) (Record, error)
}
