package services

import (
	"errors"

	"go-monolith/models"
)

type ProductService struct {
	store *models.ProductStore
}

func NewProductService(store *models.ProductStore) *ProductService {
	return &ProductService{store: store}
}

func (s *ProductService) List(search string) ([]models.Product, error) {
	return s.store.List(search)
}

func (s *ProductService) Get(id int) (*models.Product, error) {
	return s.store.Get(id)
}

func (s *ProductService) Count() (int, error) {
	return s.store.Count()
}

func (s *ProductService) LowStockCount() (int, error) {
	return s.store.LowStockCount()
}

func (s *ProductService) LowStock(limit int) ([]models.Product, error) {
	return s.store.LowStock(limit)
}

func (s *ProductService) Create(product models.Product) (*models.Product, error) {
	if product.Name == "" {
		return nil, errors.New("name is required")
	}
	if product.SKU == "" {
		return nil, errors.New("sku is required")
	}
	if product.Price <= 0 {
		return nil, errors.New("price must be greater than zero")
	}
	if product.Stock < 0 {
		return nil, errors.New("initial stock cannot be negative")
	}
	if product.LowStockThreshold <= 0 {
		product.LowStockThreshold = 10
	}
	return s.store.Create(product)
}

func (s *ProductService) Update(product models.Product) error {
	if product.Name == "" {
		return errors.New("name is required")
	}
	if product.Price <= 0 {
		return errors.New("price must be greater than zero")
	}
	if product.LowStockThreshold <= 0 {
		product.LowStockThreshold = 10
	}
	return s.store.Update(product)
}

func (s *ProductService) Delete(id int) error {
	return s.store.Delete(id)
}

func (s *ProductService) AdjustStock(id int, changeType string, quantityChange int, note string) error {
	if quantityChange == 0 {
		return errors.New("quantity change must be non-zero")
	}
	switch changeType {
	case "purchase", "adjustment", "return":
	default:
		return errors.New("invalid change type")
	}
	return s.store.AdjustStock(id, changeType, quantityChange, note)
}
