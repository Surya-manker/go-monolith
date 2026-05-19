package services

import (
	"errors"
	"strings"

	"go-monolith/models"
)

type WarehouseService struct {
	store *models.WarehouseStore
}

func NewWarehouseService(store *models.WarehouseStore) *WarehouseService {
	return &WarehouseService{store: store}
}

func (s *WarehouseService) List(bizID int) ([]models.Warehouse, error) {
	return s.store.List(bizID)
}

func (s *WarehouseService) Get(id, bizID int) (*models.Warehouse, error) {
	if id <= 0 {
		return nil, errors.New("invalid warehouse ID")
	}
	return s.store.Get(id, bizID)
}

func (s *WarehouseService) Create(name, address, managerName string, isDefault bool, bizID int) (*models.Warehouse, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("warehouse name is required")
	}
	if len(name) > 255 {
		return nil, errors.New("warehouse name must be 255 characters or less")
	}
	return s.store.Create(models.Warehouse{
		BusinessID:  bizID,
		Name:        name,
		Address:     strings.TrimSpace(address),
		ManagerName: strings.TrimSpace(managerName),
		IsDefault:   isDefault,
	})
}

func (s *WarehouseService) Update(id int, name, address, managerName string, isDefault bool, bizID int) error {
	if id <= 0 {
		return errors.New("invalid warehouse ID")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("warehouse name is required")
	}
	if len(name) > 255 {
		return errors.New("warehouse name must be 255 characters or less")
	}
	return s.store.Update(models.Warehouse{
		ID:          id,
		BusinessID:  bizID,
		Name:        name,
		Address:     strings.TrimSpace(address),
		ManagerName: strings.TrimSpace(managerName),
		IsDefault:   isDefault,
	})
}

func (s *WarehouseService) Delete(id, bizID int) error {
	if id <= 0 {
		return errors.New("invalid warehouse ID")
	}
	return s.store.Delete(id, bizID)
}

func (s *WarehouseService) Count(bizID int) (int, error) {
	return s.store.Count(bizID)
}

func (s *WarehouseService) GetWarehouseStock(warehouseID, bizID int) ([]models.WarehouseStock, error) {
	if warehouseID <= 0 {
		return nil, errors.New("invalid warehouse ID")
	}
	return s.store.GetWarehouseStock(warehouseID, bizID)
}

func (s *WarehouseService) GetAllWarehouseStock(bizID int) ([]models.WarehouseStock, error) {
	return s.store.GetAllWarehouseStock(bizID)
}

func (s *WarehouseService) AdjustWarehouseStock(warehouseID, productID, bizID, delta int, changeType, note string) error {
	if warehouseID <= 0 {
		return errors.New("please select a valid warehouse")
	}
	if productID <= 0 {
		return errors.New("please select a valid product")
	}
	if delta == 0 {
		return errors.New("quantity change must be non-zero")
	}
	validTypes := map[string]bool{
		"purchase": true, "sale": true, "adjustment": true,
		"return": true, "damage": true, "opening": true,
	}
	if !validTypes[changeType] {
		return errors.New("invalid change type")
	}
	return s.store.AdjustWarehouseStock(warehouseID, productID, bizID, delta, changeType, note)
}

func (s *WarehouseService) ListTransfers(bizID int) ([]models.StockTransfer, error) {
	return s.store.ListTransfers(bizID)
}

func (s *WarehouseService) GetTransfer(id, bizID int) (*models.StockTransfer, error) {
	if id <= 0 {
		return nil, errors.New("invalid transfer ID")
	}
	return s.store.GetTransfer(id, bizID)
}

func (s *WarehouseService) CreateTransfer(fromID, toID, productID, qty, bizID int, note string) (*models.StockTransfer, error) {
	if fromID <= 0 {
		return nil, errors.New("please select a source warehouse")
	}
	if toID <= 0 {
		return nil, errors.New("please select a destination warehouse")
	}
	if productID <= 0 {
		return nil, errors.New("please select a product")
	}
	if qty <= 0 {
		return nil, errors.New("transfer quantity must be greater than zero")
	}
	if fromID == toID {
		return nil, errors.New("source and destination warehouse must be different")
	}
	return s.store.CreateTransfer(models.StockTransfer{
		BusinessID:      bizID,
		FromWarehouseID: fromID,
		ToWarehouseID:   toID,
		ProductID:       productID,
		Quantity:        qty,
		Note:            strings.TrimSpace(note),
		Status:          "completed",
	})
}

func (s *WarehouseService) CountTransfers(bizID int) (int, error) {
	return s.store.CountTransfers(bizID)
}
