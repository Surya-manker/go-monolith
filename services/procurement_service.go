package services

import (
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"go-monolith/models"
)

// ProcurementService coordinates supplier management, purchase orders, GRN receiving,
// and payment tracking. It operates across multiple stores within single transactions.
type ProcurementService struct {
	db           *sql.DB
	store        *models.ProcurementStore
	whStore      *models.WarehouseStore
	batchStore   *models.BatchStore
	productStore *models.ProductStore
}

func NewProcurementService(
	db *sql.DB,
	store *models.ProcurementStore,
	wh *models.WarehouseStore,
	batch *models.BatchStore,
	prod *models.ProductStore,
) *ProcurementService {
	return &ProcurementService{db: db, store: store, whStore: wh, batchStore: batch, productStore: prod}
}

// ── Suppliers ─────────────────────────────────────────────────────────────────

func (s *ProcurementService) ListSuppliers(bizID int) ([]models.Supplier, error) {
	return s.store.ListSuppliers(bizID)
}

func (s *ProcurementService) GetSupplier(id, bizID int) (*models.Supplier, error) {
	if id <= 0 {
		return nil, errors.New("invalid supplier ID")
	}
	return s.store.GetSupplier(id, bizID)
}

func (s *ProcurementService) CreateSupplier(bizID int, name, email, phone, gstin, pan, address, contactPerson, code, notes string, paymentTerms int, creditLimit float64) (*models.Supplier, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("supplier name is required")
	}
	sup := &models.Supplier{
		BusinessID:    bizID,
		SupplierCode:  strings.TrimSpace(code),
		Name:          name,
		Email:         strings.TrimSpace(email),
		Phone:         strings.TrimSpace(phone),
		GSTIN:         strings.TrimSpace(gstin),
		PAN:           strings.TrimSpace(pan),
		Address:       strings.TrimSpace(address),
		ContactPerson: strings.TrimSpace(contactPerson),
		PaymentTerms:  paymentTerms,
		CreditLimit:   creditLimit,
		Status:        "active",
		Notes:         strings.TrimSpace(notes),
	}
	if sup.PaymentTerms <= 0 {
		sup.PaymentTerms = 30
	}
	return s.store.CreateSupplier(sup)
}

func (s *ProcurementService) UpdateSupplier(id, bizID int, name, email, phone, gstin, pan, address, contactPerson, status, notes string, paymentTerms int, creditLimit float64) error {
	if id <= 0 {
		return errors.New("invalid supplier ID")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("supplier name is required")
	}
	return s.store.UpdateSupplier(&models.Supplier{
		ID:            id,
		BusinessID:    bizID,
		Name:          name,
		Email:         strings.TrimSpace(email),
		Phone:         strings.TrimSpace(phone),
		GSTIN:         strings.TrimSpace(gstin),
		PAN:           strings.TrimSpace(pan),
		Address:       strings.TrimSpace(address),
		ContactPerson: strings.TrimSpace(contactPerson),
		PaymentTerms:  paymentTerms,
		CreditLimit:   creditLimit,
		Status:        status,
		Notes:         strings.TrimSpace(notes),
	})
}

// ── Purchase Orders ───────────────────────────────────────────────────────────

type POItemInput struct {
	ProductID int
	Quantity  int
	UnitPrice float64
	TaxRate   float64
}

func (s *ProcurementService) CreatePO(bizID, supplierID, warehouseID int, expectedDate *time.Time, notes string, items []POItemInput) (*models.ProcurementOrder, error) {
	if supplierID <= 0 {
		return nil, errors.New("please select a supplier")
	}
	if warehouseID <= 0 {
		return nil, errors.New("please select a destination warehouse")
	}
	if len(items) == 0 {
		return nil, errors.New("purchase order must have at least one item")
	}

	sup, err := s.store.GetSupplier(supplierID, bizID)
	if err != nil {
		return nil, errors.New("supplier not found")
	}

	var orderItems []models.ProcurementOrderItem
	var subtotal, taxTotal float64

	for _, item := range items {
		if item.Quantity <= 0 {
			return nil, errors.New("item quantity must be greater than zero")
		}
		prod, err := s.productStore.Get(item.ProductID, bizID)
		if err != nil {
			return nil, fmt.Errorf("product %d not found", item.ProductID)
		}
		lineSubtotal := math.Round(float64(item.Quantity)*item.UnitPrice*100) / 100
		taxAmt := math.Round(lineSubtotal*item.TaxRate/100*100) / 100
		lineTotal := lineSubtotal + taxAmt
		subtotal += lineSubtotal
		taxTotal += taxAmt

		orderItems = append(orderItems, models.ProcurementOrderItem{
			ProductID:   prod.ID,
			ProductName: prod.Name,
			SKU:         prod.SKU,
			Quantity:    item.Quantity,
			UnitPrice:   item.UnitPrice,
			TaxRate:     item.TaxRate,
			TaxAmount:   math.Round(taxAmt*100) / 100,
			LineTotal:   math.Round(lineTotal*100) / 100,
		})
	}

	po := &models.ProcurementOrder{
		BusinessID:   bizID,
		SupplierID:   supplierID,
		SupplierName: sup.Name,
		PONumber:     s.store.NextPONumber(bizID),
		WarehouseID:  warehouseID,
		ExpectedDate: expectedDate,
		Notes:        strings.TrimSpace(notes),
		Subtotal:     math.Round(subtotal*100) / 100,
		TaxTotal:     math.Round(taxTotal*100) / 100,
		GrandTotal:   math.Round((subtotal+taxTotal)*100) / 100,
		Items:        orderItems,
	}
	return s.store.CreateOrder(po)
}

func (s *ProcurementService) GetOrder(id, bizID int) (*models.ProcurementOrder, error) {
	return s.store.GetOrder(id, bizID)
}

func (s *ProcurementService) ListOrders(bizID int, status string) ([]models.ProcurementOrder, error) {
	return s.store.ListOrders(bizID, status)
}

func (s *ProcurementService) SubmitForApproval(id, bizID int) error {
	o, err := s.store.GetOrder(id, bizID)
	if err != nil || o.Status != "draft" {
		return errors.New("only draft POs can be submitted for approval")
	}
	return s.store.UpdateOrderStatus(id, "pending", "")
}

func (s *ProcurementService) ApprovePO(id, bizID int, approverName string) error {
	o, err := s.store.GetOrder(id, bizID)
	if err != nil || o.Status != "pending" {
		return errors.New("only pending POs can be approved")
	}
	return s.store.UpdateOrderStatus(id, "approved", approverName)
}

func (s *ProcurementService) CancelPO(id, bizID int) error {
	o, err := s.store.GetOrder(id, bizID)
	if err != nil {
		return err
	}
	if o.Status == "completed" || o.Status == "cancelled" {
		return errors.New("cannot cancel a completed or already cancelled PO")
	}
	return s.store.UpdateOrderStatus(id, "cancelled", "")
}

// ── GRN (Goods Receipt Note) ──────────────────────────────────────────────────

type GRNItemInput struct {
	OrderItemID int
	ProductID   int
	ReceivedQty int
	DamagedQty  int
	UnitPrice   float64
	BatchNumber string
	LotNumber   string
	MfgDate     *time.Time
	ExpiryDate  *time.Time
}

// CreateGRN atomically:
//  1. Creates batch records (if batch info provided)
//  2. Adjusts warehouse_stock for received items
//  3. Updates PO item received quantities
//  4. Updates PO status (partially_received → completed)
//  5. Creates the GRN record
func (s *ProcurementService) CreateGRN(bizID, orderID, supplierID, warehouseID int, notes string, items []GRNItemInput) (*models.ProcurementGRN, error) {
	if warehouseID <= 0 {
		return nil, errors.New("please select a warehouse")
	}
	if len(items) == 0 {
		return nil, errors.New("GRN must have at least one item")
	}

	// Validate warehouse belongs to business.
	if _, err := s.whStore.Get(warehouseID, bizID); err != nil {
		return nil, errors.New("warehouse not found")
	}

	supplierName := ""
	if supplierID > 0 {
		if sup, err := s.store.GetSupplier(supplierID, bizID); err == nil {
			supplierName = sup.Name
		}
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var grnItems []models.ProcurementGRNItem
	totalReceived := 0

	for _, item := range items {
		if item.ReceivedQty <= 0 && item.DamagedQty <= 0 {
			continue
		}
		prod, err := s.productStore.Get(item.ProductID, bizID)
		if err != nil {
			return nil, fmt.Errorf("product %d not found", item.ProductID)
		}

		sellableQty := item.ReceivedQty // damaged qty does NOT go to sellable stock

		if sellableQty > 0 {
			note := fmt.Sprintf("GRN receive — %s", strings.TrimSpace(notes))
			if note == "GRN receive — " {
				note = "GRN receive"
			}

			if strings.TrimSpace(item.BatchNumber) != "" {
				// Create batch + adjust warehouse stock within this TX
				b := &models.Batch{
					BusinessID:  bizID,
					ProductID:   item.ProductID,
					WarehouseID: warehouseID,
					BatchNumber: strings.TrimSpace(item.BatchNumber),
					LotNumber:   strings.TrimSpace(item.LotNumber),
					Quantity:    sellableQty,
					MfgDate:     item.MfgDate,
					ExpiryDate:  item.ExpiryDate,
				}
				if _, err = s.batchStore.CreateBatchInTx(tx, b); err != nil {
					return nil, fmt.Errorf("%s batch creation: %w", prod.Name, err)
				}
			}

			// Adjust warehouse stock (always — batch is a tracking layer on top).
			if err = s.whStore.AdjustWarehouseStockTx(tx, warehouseID, item.ProductID, bizID, sellableQty, "purchase", note); err != nil {
				return nil, fmt.Errorf("%s: %w", prod.Name, err)
			}
		}

		// Update PO item received quantity.
		if item.OrderItemID > 0 {
			if err = s.store.UpdateItemReceivedQty(tx, item.OrderItemID, item.ReceivedQty); err != nil {
				return nil, err
			}
		}

		totalReceived += item.ReceivedQty

		var mfgStr, expStr string
		if item.MfgDate != nil {
			mfgStr = item.MfgDate.Format("2006-01-02")
		}
		if item.ExpiryDate != nil {
			expStr = item.ExpiryDate.Format("2006-01-02")
		}
		_ = mfgStr
		_ = expStr

		grnItems = append(grnItems, models.ProcurementGRNItem{
			OrderItemID: item.OrderItemID,
			ProductID:   item.ProductID,
			ProductName: prod.Name,
			SKU:         prod.SKU,
			ReceivedQty: item.ReceivedQty,
			DamagedQty:  item.DamagedQty,
			BatchNumber: strings.TrimSpace(item.BatchNumber),
			LotNumber:   strings.TrimSpace(item.LotNumber),
			MfgDate:     item.MfgDate,
			ExpiryDate:  item.ExpiryDate,
			UnitPrice:   item.UnitPrice,
		})
	}

	if totalReceived == 0 {
		return nil, errors.New("no items with received quantity > 0")
	}

	// Update PO completion status.
	if orderID > 0 {
		if err = s.store.CheckOrderCompletion(tx, orderID); err != nil {
			return nil, err
		}
	}

	grn := &models.ProcurementGRN{
		BusinessID:    bizID,
		OrderID:       orderID,
		SupplierID:    supplierID,
		SupplierName:  supplierName,
		GRNNumber:     s.store.NextGRNNumber(bizID),
		WarehouseID:   warehouseID,
		Notes:         strings.TrimSpace(notes),
		TotalReceived: totalReceived,
		Items:         grnItems,
	}

	grnID, err := s.store.CreateGRNTx(tx, grn)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}

	return s.store.GetGRN(int(grnID), bizID)
}

func (s *ProcurementService) ListGRN(bizID int) ([]models.ProcurementGRN, error) {
	return s.store.ListGRN(bizID)
}

func (s *ProcurementService) GetGRN(id, bizID int) (*models.ProcurementGRN, error) {
	return s.store.GetGRN(id, bizID)
}

// ── Supplier Payments ─────────────────────────────────────────────────────────

func (s *ProcurementService) RecordPayment(bizID, supplierID, orderID int, amount float64, method, reference, notes string) (*models.SupplierPayment, error) {
	if supplierID <= 0 {
		return nil, errors.New("please select a supplier")
	}
	if amount <= 0 {
		return nil, errors.New("payment amount must be greater than zero")
	}
	validMethods := map[string]bool{"cash": true, "bank_transfer": true, "upi": true, "cheque": true, "other": true}
	if !validMethods[method] {
		method = "cash"
	}

	sup, err := s.store.GetSupplier(supplierID, bizID)
	if err != nil {
		return nil, errors.New("supplier not found")
	}

	poNumber := ""
	if orderID > 0 {
		if po, err := s.store.GetOrder(orderID, bizID); err == nil {
			poNumber = po.PONumber
		}
	}

	return s.store.CreatePayment(&models.SupplierPayment{
		BusinessID:    bizID,
		SupplierID:    supplierID,
		SupplierName:  sup.Name,
		OrderID:       orderID,
		PONumber:      poNumber,
		Amount:        math.Round(amount*100) / 100,
		PaymentMethod: method,
		Reference:     strings.TrimSpace(reference),
		Notes:         strings.TrimSpace(notes),
	})
}

func (s *ProcurementService) ListPayments(bizID, supplierID int) ([]models.SupplierPayment, error) {
	return s.store.ListPayments(bizID, supplierID)
}

// ── Analytics ─────────────────────────────────────────────────────────────────

func (s *ProcurementService) ReorderSuggestions(bizID int) ([]models.ReorderSuggestion, error) {
	return s.store.ReorderSuggestions(bizID)
}

func (s *ProcurementService) Stats(bizID int) (models.ProcurementStats, error) {
	return s.store.Stats(bizID)
}

func (s *ProcurementService) TotalDues(bizID int) (float64, error) {
	return s.store.TotalDues(bizID)
}
