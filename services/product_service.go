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

func (s *ProductService) List(search string, businessID int) ([]models.Product, error) {
	return s.store.List(search, businessID)
}

func (s *ProductService) Get(id, businessID int) (*models.Product, error) {
	return s.store.Get(id, businessID)
}

func (s *ProductService) Count(businessID int) (int, error) {
	return s.store.Count(businessID)
}

func (s *ProductService) LowStockCount(businessID int) (int, error) {
	return s.store.LowStockCount(businessID)
}

func (s *ProductService) LowStock(limit, businessID int) ([]models.Product, error) {
	return s.store.LowStock(limit, businessID)
}

func (s *ProductService) Create(product models.Product) (*models.Product, error) {
	if product.BusinessID == 0 {
		return nil, errors.New("business_id is required")
	}
	if product.Name == "" {
		return nil, errors.New("name is required")
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

func (s *ProductService) Delete(id, businessID int) error {
	return s.store.Delete(id, businessID)
}

func (s *ProductService) AdjustStock(id, businessID int, changeType string, quantityChange int, note string) error {
	if quantityChange == 0 {
		return errors.New("quantity change must be non-zero")
	}
	validTypes := map[string]bool{
		"purchase": true, "adjustment": true, "return": true,
		"damage": true, "opening": true, "sale": true,
	}
	if !validTypes[changeType] {
		return errors.New("invalid change type")
	}
	return s.store.AdjustStock(id, businessID, changeType, quantityChange, note)
}

func (s *ProductService) TotalStockValue(businessID int) (stockValue, costValue float64, err error) {
	return s.store.TotalStockValue(businessID)
}

func (s *ProductService) DeadStockCount(businessID int) (int, error) {
	return s.store.DeadStockCount(businessID)
}

func (s *ProductService) GetByBarcode(barcode string, businessID int) (*models.Product, error) {
	return s.store.GetByBarcode(barcode, businessID)
}

// AutoGenerateBarcode assigns an EAN-13 barcode to a single product if it doesn't have one.
func (s *ProductService) AutoGenerateBarcode(productID, businessID int, bs *BarcodeService) error {
	product, err := s.store.Get(productID, businessID)
	if err != nil {
		return err
	}
	if product.Barcode != "" {
		return nil // already has one
	}
	product.Barcode = bs.AutoGenerateEAN13(businessID, productID)
	return s.store.Update(*product)
}

// AutoGenerateAllBarcodes generates EAN-13 barcodes for every product that lacks one.
// Returns the count of products updated.
func (s *ProductService) AutoGenerateAllBarcodes(businessID int, bs *BarcodeService) (int, error) {
	products, err := s.store.List("", businessID)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, p := range products {
		if p.Barcode != "" {
			continue
		}
		p.Barcode = bs.AutoGenerateEAN13(businessID, p.ID)
		if updateErr := s.store.Update(p); updateErr == nil {
			count++
		}
	}
	return count, nil
}
