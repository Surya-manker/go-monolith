package services

import (
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"

	"go-monolith/models"
)

// POSService handles POS checkout logic with FEFO batch support.
type POSService struct {
	db             *sql.DB
	posStore       *models.POSStore
	warehouseStore *models.WarehouseStore
	productStore   *models.ProductStore
	batchStore     *models.BatchStore
}

func NewPOSService(db *sql.DB, pos *models.POSStore, wh *models.WarehouseStore, prod *models.ProductStore, batch *models.BatchStore) *POSService {
	return &POSService{db: db, posStore: pos, warehouseStore: wh, productStore: prod, batchStore: batch}
}

type CheckoutInput struct {
	BusinessID    int
	WarehouseID   int
	Cart          *POSCart
	CustomerName  string
	CustomerPhone string
	PaymentMethod string
	AmountPaid    float64
	Discount      float64
}

// Checkout atomically deducts stock (FEFO when batches exist, else plain warehouse_stock)
// and creates the POS sale record.
func (s *POSService) Checkout(in CheckoutInput) (*models.POSSale, error) {
	if in.Cart == nil || in.Cart.IsEmpty() {
		return nil, errors.New("cart is empty")
	}
	if in.WarehouseID <= 0 {
		return nil, errors.New("please select a warehouse")
	}
	validPayment := map[string]bool{"cash": true, "upi": true, "card": true, "cheque": true}
	if !validPayment[strings.ToLower(in.PaymentMethod)] {
		in.PaymentMethod = "cash"
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var saleItems []models.POSSaleItem

	for _, ci := range in.Cart.Items {
		prod, err := s.productStore.Get(ci.ProductID, in.BusinessID)
		if err != nil {
			return nil, fmt.Errorf("product %q not found", ci.ProductName)
		}

		note := fmt.Sprintf("POS sale — %s", strings.TrimSpace(in.CustomerName))
		if note == "POS sale — " {
			note = "POS sale"
		}

		// Try FEFO batch deduction first.
		hasBatches, _ := s.batchStore.HasBatches(tx, ci.ProductID, in.WarehouseID, in.BusinessID)

		if hasBatches {
			// Check for expired-only stock.
			deductions, err := s.batchStore.SelectFEFOTx(tx, ci.ProductID, in.WarehouseID, in.BusinessID, ci.Quantity)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", prod.Name, err)
			}
			if len(deductions) == 0 {
				return nil, fmt.Errorf("%s: all batch stock has expired — cannot sell", prod.Name)
			}
			for _, d := range deductions {
				if err = s.batchStore.DeductBatchTx(tx, d.BatchID, d.Quantity, "sale_out", "pos_sale", 0, note); err != nil {
					return nil, fmt.Errorf("%s: %w", prod.Name, err)
				}
			}
		}

		// Deduct from warehouse_stock (always — batches are a tracking layer on top).
		if err = s.warehouseStore.AdjustWarehouseStockTx(
			tx, in.WarehouseID, ci.ProductID, in.BusinessID, -ci.Quantity, "sale", note,
		); err != nil {
			return nil, fmt.Errorf("%s: %w", prod.Name, err)
		}

		subtotal := roundCents(ci.UnitPrice * float64(ci.Quantity))
		taxAmt := roundCents(subtotal * ci.TaxRate / 100)
		lineTotal := roundCents(subtotal + taxAmt - ci.Discount)

		saleItems = append(saleItems, models.POSSaleItem{
			ProductID:   ci.ProductID,
			ProductName: prod.Name,
			SKU:         prod.SKU,
			Quantity:    ci.Quantity,
			UnitPrice:   ci.UnitPrice,
			TaxRate:     ci.TaxRate,
			TaxAmount:   taxAmt,
			Discount:    ci.Discount,
			LineTotal:   lineTotal,
		})
	}

	grandTotal := in.Cart.GrandTotal() - roundCents(in.Discount)
	if grandTotal < 0 {
		grandTotal = 0
	}
	change := math.Max(0, roundCents(in.AmountPaid-grandTotal))

	sale := &models.POSSale{
		BusinessID:    in.BusinessID,
		WarehouseID:   in.WarehouseID,
		SaleNumber:    s.posStore.NextSaleNumber(in.BusinessID),
		CustomerName:  strings.TrimSpace(in.CustomerName),
		CustomerPhone: strings.TrimSpace(in.CustomerPhone),
		Subtotal:      in.Cart.Subtotal(),
		Discount:      roundCents(in.Discount),
		TaxTotal:      in.Cart.TaxTotal(),
		GrandTotal:    grandTotal,
		PaymentMethod: in.PaymentMethod,
		AmountPaid:    roundCents(in.AmountPaid),
		ChangeGiven:   change,
		Items:         saleItems,
	}

	saleID, err := s.posStore.CreateSaleTx(tx, sale)
	if err != nil {
		return nil, err
	}

	// Back-fill pos_sale batch references in batch_logs (best-effort).
	for _, item := range saleItems {
		_, _ = tx.Exec(
			`UPDATE batch_logs SET ref_id=? WHERE batch_id IN (
			   SELECT id FROM batches WHERE product_id=? AND warehouse_id=? AND business_id=?
			 ) AND ref_type='' AND ref_id=0 AND change_type='sale_out'
			 ORDER BY id DESC LIMIT ?`,
			saleID, item.ProductID, in.WarehouseID, in.BusinessID, item.Quantity,
		)
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return s.posStore.Get(int(saleID), in.BusinessID)
}

func (s *POSService) GetSale(id, bizID int) (*models.POSSale, error) {
	return s.posStore.Get(id, bizID)
}

func (s *POSService) ListSales(bizID, limit int) ([]models.POSSale, error) {
	if limit <= 0 {
		limit = 50
	}
	return s.posStore.List(bizID, limit)
}

func (s *POSService) TodayTotal(bizID int) (int, float64, error) {
	return s.posStore.TodayTotal(bizID)
}
