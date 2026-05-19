package services

import (
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"

	"go-monolith/models"
)

// ReturnsService handles both sales returns and purchase returns.
// It coordinates warehouse stock adjustments and batch log updates atomically.
type ReturnsService struct {
	db           *sql.DB
	batchStore   *models.BatchStore
	whStore      *models.WarehouseStore
	productStore *models.ProductStore
}

func NewReturnsService(db *sql.DB, bs *models.BatchStore, wh *models.WarehouseStore, prod *models.ProductStore) *ReturnsService {
	return &ReturnsService{db: db, batchStore: bs, whStore: wh, productStore: prod}
}

// ── Input types ───────────────────────────────────────────────────────────────

type ReturnItemInput struct {
	ProductID  int
	BatchID    *int   // optional
	Quantity   int
	UnitPrice  float64
	Condition  string // resalable | damaged (sales returns only)
	Notes      string
}

type SalesReturnInput struct {
	BusinessID     int
	WarehouseID    int
	OriginalSaleID *int
	CustomerName   string
	CustomerPhone  string
	ReturnReason   string
	Items          []ReturnItemInput
}

type PurchaseReturnInput struct {
	BusinessID   int
	WarehouseID  int
	VendorName   string
	ReturnReason string
	Items        []ReturnItemInput
}

// ── Sales Return ─────────────────────────────────────────────────────────────
//
// Stock flow:
//   - condition=resalable → warehouse_stock += qty, batch qty += qty (if batch specified)
//   - condition=damaged   → warehouse_stock unchanged (item physically back but not sellable)
//                           stock_log entry with change_type='damage' at qty 0 for traceability

func (s *ReturnsService) CreateSalesReturn(in SalesReturnInput) (*models.SalesReturn, error) {
	if in.BusinessID <= 0 || in.WarehouseID <= 0 {
		return nil, errors.New("invalid business or warehouse")
	}
	if len(in.Items) == 0 {
		return nil, errors.New("return must have at least one item")
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var totalAmount float64
	var returnItems []models.SalesReturnItem

	for _, item := range in.Items {
		if item.Quantity <= 0 {
			return nil, fmt.Errorf("return quantity must be > 0 for each item")
		}
		// Validate product.
		prod, err := s.productStore.Get(item.ProductID, in.BusinessID)
		if err != nil {
			return nil, fmt.Errorf("product %d not found", item.ProductID)
		}

		cond := strings.ToLower(item.Condition)
		if cond == "" {
			cond = "resalable"
		}
		if cond != "resalable" && cond != "damaged" && cond != "expired" {
			return nil, fmt.Errorf("invalid condition %q for item %s", item.Condition, prod.Name)
		}

		lineTotal := math.Round(float64(item.Quantity)*item.UnitPrice*100) / 100
		totalAmount += lineTotal

		note := fmt.Sprintf("Sales return — %s", in.CustomerName)
		if in.CustomerName == "" {
			note = "Sales return"
		}

		if cond == "resalable" {
			// Restore stock to warehouse.
			if err = s.whStore.AdjustWarehouseStockTx(tx, in.WarehouseID, item.ProductID, in.BusinessID, item.Quantity, "return", note); err != nil {
				return nil, fmt.Errorf("%s: %w", prod.Name, err)
			}
			// Restore to batch if specified.
			if item.BatchID != nil && *item.BatchID > 0 {
				if err = s.batchStore.AddToBatchTx(tx, *item.BatchID, item.Quantity, "return_in", note); err != nil {
					return nil, fmt.Errorf("%s batch restore: %w", prod.Name, err)
				}
			}
		}
		// Damaged/expired items: item comes back physically but doesn't go into sellable stock.
		// Log the event in stock_logs with qty_change=0 for traceability.

		batchNumber := ""
		if item.BatchID != nil && *item.BatchID > 0 {
			_ = tx.QueryRow(`SELECT batch_number FROM batches WHERE id=?`, *item.BatchID).Scan(&batchNumber)
		}

		returnItems = append(returnItems, models.SalesReturnItem{
			ProductID:   item.ProductID,
			BatchID:     item.BatchID,
			BatchNumber: batchNumber,
			ProductName: prod.Name,
			SKU:         prod.SKU,
			Quantity:    item.Quantity,
			UnitPrice:   item.UnitPrice,
			LineTotal:   lineTotal,
			Condition:   cond,
			Notes:       strings.TrimSpace(item.Notes),
		})
	}

	ret := &models.SalesReturn{
		BusinessID:     in.BusinessID,
		WarehouseID:    in.WarehouseID,
		ReturnNumber:   s.batchStore.NextReturnNumber(in.BusinessID, "SAL-RET"),
		OriginalSaleID: in.OriginalSaleID,
		CustomerName:   strings.TrimSpace(in.CustomerName),
		CustomerPhone:  strings.TrimSpace(in.CustomerPhone),
		ReturnReason:   strings.TrimSpace(in.ReturnReason),
		TotalAmount:    math.Round(totalAmount*100) / 100,
		Items:          returnItems,
	}

	retID, err := s.batchStore.CreateSalesReturnTx(tx, ret)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	ret.ID = int(retID)
	return ret, nil
}

// ── Purchase Return ──────────────────────────────────────────────────────────
//
// Stock flow: warehouse_stock -= qty for each item (goods leaving back to supplier)
//             batch qty -= qty if batch specified

func (s *ReturnsService) CreatePurchaseReturn(in PurchaseReturnInput) (*models.PurchaseReturn, error) {
	if in.BusinessID <= 0 || in.WarehouseID <= 0 {
		return nil, errors.New("invalid business or warehouse")
	}
	if in.VendorName == "" {
		return nil, errors.New("vendor name is required")
	}
	if len(in.Items) == 0 {
		return nil, errors.New("return must have at least one item")
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var totalAmount float64
	var returnItems []models.PurchaseReturnItem

	for _, item := range in.Items {
		if item.Quantity <= 0 {
			return nil, errors.New("return quantity must be > 0")
		}
		prod, err := s.productStore.Get(item.ProductID, in.BusinessID)
		if err != nil {
			return nil, fmt.Errorf("product %d not found", item.ProductID)
		}

		lineTotal := math.Round(float64(item.Quantity)*item.UnitPrice*100) / 100
		totalAmount += lineTotal

		note := fmt.Sprintf("Purchase return to %s", in.VendorName)

		// Deduct from warehouse stock.
		if err = s.whStore.AdjustWarehouseStockTx(tx, in.WarehouseID, item.ProductID, in.BusinessID, -item.Quantity, "return", note); err != nil {
			return nil, fmt.Errorf("%s: %w", prod.Name, err)
		}

		// Deduct from batch if specified.
		if item.BatchID != nil && *item.BatchID > 0 {
			if err = s.batchStore.DeductBatchTx(tx, *item.BatchID, item.Quantity, "return_out", "purchase_return", 0, note); err != nil {
				return nil, fmt.Errorf("%s batch deduct: %w", prod.Name, err)
			}
		}

		batchNumber := ""
		if item.BatchID != nil && *item.BatchID > 0 {
			_ = tx.QueryRow(`SELECT batch_number FROM batches WHERE id=?`, *item.BatchID).Scan(&batchNumber)
		}

		returnItems = append(returnItems, models.PurchaseReturnItem{
			ProductID:   item.ProductID,
			BatchID:     item.BatchID,
			BatchNumber: batchNumber,
			ProductName: prod.Name,
			SKU:         prod.SKU,
			Quantity:    item.Quantity,
			UnitPrice:   item.UnitPrice,
			LineTotal:   lineTotal,
			Notes:       strings.TrimSpace(item.Notes),
		})
	}

	ret := &models.PurchaseReturn{
		BusinessID:   in.BusinessID,
		WarehouseID:  in.WarehouseID,
		ReturnNumber: s.batchStore.NextReturnNumber(in.BusinessID, "PUR-RET"),
		VendorName:   strings.TrimSpace(in.VendorName),
		ReturnReason: strings.TrimSpace(in.ReturnReason),
		TotalAmount:  math.Round(totalAmount*100) / 100,
		Items:        returnItems,
	}

	retID, err := s.batchStore.CreatePurchaseReturnTx(tx, ret)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	ret.ID = int(retID)
	return ret, nil
}

// ── List ──────────────────────────────────────────────────────────────────────

func (s *ReturnsService) ListSalesReturns(bizID, limit int) ([]models.SalesReturn, error) {
	if limit <= 0 {
		limit = 100
	}
	return s.batchStore.ListSalesReturns(bizID, limit)
}

func (s *ReturnsService) ListPurchaseReturns(bizID, limit int) ([]models.PurchaseReturn, error) {
	if limit <= 0 {
		limit = 100
	}
	return s.batchStore.ListPurchaseReturns(bizID, limit)
}
