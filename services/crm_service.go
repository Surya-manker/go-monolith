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

// CRMService manages the full customer sales lifecycle:
// Customers → Quotations → Sales Orders (with stock reservation) → Delivery Challans → Payments.
type CRMService struct {
	db           *sql.DB
	store        *models.CRMStore
	whStore      *models.WarehouseStore
	batchStore   *models.BatchStore
	productStore *models.ProductStore
}

func NewCRMService(db *sql.DB, store *models.CRMStore, wh *models.WarehouseStore, batch *models.BatchStore, prod *models.ProductStore) *CRMService {
	return &CRMService{db: db, store: store, whStore: wh, batchStore: batch, productStore: prod}
}

// ── Customers ─────────────────────────────────────────────────────────────────

func (s *CRMService) ListCustomers(bizID int) ([]models.CRMCustomer, error) {
	return s.store.ListCustomers(bizID)
}

func (s *CRMService) GetCustomer(id, bizID int) (*models.CRMCustomer, error) {
	if id <= 0 {
		return nil, errors.New("invalid customer ID")
	}
	return s.store.GetCustomer(id, bizID)
}

func (s *CRMService) CreateCustomer(bizID int, name, email, phone, gstin, pan, billingAddr, shippingAddr, contactPerson, group, code, notes string, creditLimit float64, payTerms int, status string) (*models.CRMCustomer, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("customer name is required")
	}
	if status == "" {
		status = "active"
	}
	if payTerms <= 0 {
		payTerms = 30
	}
	return s.store.CreateCustomer(&models.CRMCustomer{
		BusinessID:      bizID,
		CustomerCode:    strings.TrimSpace(code),
		Name:            name,
		Email:           strings.TrimSpace(email),
		Phone:           strings.TrimSpace(phone),
		GSTIN:           strings.TrimSpace(gstin),
		PAN:             strings.TrimSpace(pan),
		BillingAddress:  strings.TrimSpace(billingAddr),
		ShippingAddress: strings.TrimSpace(shippingAddr),
		ContactPerson:   strings.TrimSpace(contactPerson),
		CustomerGroup:   strings.TrimSpace(group),
		CreditLimit:     creditLimit,
		PaymentTerms:    payTerms,
		Status:          status,
		Notes:           strings.TrimSpace(notes),
	})
}

func (s *CRMService) UpdateCustomer(id, bizID int, name, email, phone, gstin, pan, billingAddr, shippingAddr, contactPerson, group, status, notes string, creditLimit float64, payTerms int) error {
	if id <= 0 {
		return errors.New("invalid customer ID")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("customer name is required")
	}
	return s.store.UpdateCustomer(&models.CRMCustomer{
		ID: id, BusinessID: bizID,
		Name: name, Email: strings.TrimSpace(email), Phone: strings.TrimSpace(phone),
		GSTIN: strings.TrimSpace(gstin), PAN: strings.TrimSpace(pan),
		BillingAddress: strings.TrimSpace(billingAddr), ShippingAddress: strings.TrimSpace(shippingAddr),
		ContactPerson: strings.TrimSpace(contactPerson), CustomerGroup: strings.TrimSpace(group),
		CreditLimit: creditLimit, PaymentTerms: payTerms, Status: status, Notes: strings.TrimSpace(notes),
	})
}

// ── Quotations ────────────────────────────────────────────────────────────────

type OrderItemInput struct {
	ProductID int
	Quantity  int
	UnitPrice float64
	TaxRate   float64
	Discount  float64
}

func (s *CRMService) CreateQuotation(bizID, customerID, warehouseID int, validUntil *time.Time, notes string, items []OrderItemInput) (*models.Quotation, error) {
	if customerID <= 0 {
		return nil, errors.New("please select a customer")
	}
	if len(items) == 0 {
		return nil, errors.New("quotation must have at least one item")
	}
	cust, err := s.store.GetCustomer(customerID, bizID)
	if err != nil {
		return nil, errors.New("customer not found")
	}

	var qtItems []models.QuotationItem
	var subtotal, taxTotal float64
	for _, item := range items {
		if item.Quantity <= 0 {
			return nil, errors.New("quantity must be greater than zero")
		}
		prod, err := s.productStore.Get(item.ProductID, bizID)
		if err != nil {
			return nil, fmt.Errorf("product %d not found", item.ProductID)
		}
		lineSubtotal := math.Round(float64(item.Quantity)*item.UnitPrice*100) / 100
		taxAmt := math.Round(lineSubtotal*item.TaxRate/100*100) / 100
		lineTotal := lineSubtotal + taxAmt - item.Discount
		subtotal += lineSubtotal
		taxTotal += taxAmt
		qtItems = append(qtItems, models.QuotationItem{
			ProductID: prod.ID, ProductName: prod.Name, SKU: prod.SKU,
			Quantity: item.Quantity, UnitPrice: item.UnitPrice,
			TaxRate: item.TaxRate, TaxAmount: taxAmt,
			Discount: item.Discount, LineTotal: math.Round(lineTotal*100) / 100,
		})
	}

	return s.store.CreateQuotation(&models.Quotation{
		BusinessID: bizID, CustomerID: customerID, CustomerName: cust.Name,
		QuoteNumber: s.store.NextQuoteNumber(bizID), WarehouseID: warehouseID,
		ValidUntil: validUntil, Notes: strings.TrimSpace(notes),
		Subtotal:   math.Round(subtotal*100) / 100,
		TaxTotal:   math.Round(taxTotal*100) / 100,
		GrandTotal: math.Round((subtotal+taxTotal)*100) / 100,
		Items:      qtItems,
	})
}

func (s *CRMService) GetQuotation(id, bizID int) (*models.Quotation, error) {
	return s.store.GetQuotation(id, bizID)
}

func (s *CRMService) ListQuotations(bizID int, status string) ([]models.Quotation, error) {
	return s.store.ListQuotations(bizID, status)
}

func (s *CRMService) SendQuotation(id, bizID int) error {
	qt, err := s.store.GetQuotation(id, bizID)
	if err != nil || qt.Status != "draft" {
		return errors.New("only draft quotations can be sent")
	}
	return s.store.UpdateQuotationStatus(id, "sent")
}

func (s *CRMService) ApproveQuotation(id, bizID int) error {
	qt, err := s.store.GetQuotation(id, bizID)
	if err != nil || (qt.Status != "sent" && qt.Status != "draft") {
		return errors.New("quotation must be in draft or sent status to approve")
	}
	return s.store.UpdateQuotationStatus(id, "approved")
}

func (s *CRMService) RejectQuotation(id, bizID int) error {
	qt, err := s.store.GetQuotation(id, bizID)
	if err != nil || qt.Status == "converted" || qt.Status == "rejected" {
		return errors.New("cannot reject this quotation")
	}
	return s.store.UpdateQuotationStatus(id, "rejected")
}

// ConvertQuotation creates a sales order from a quotation.
func (s *CRMService) ConvertQuotation(qtID, bizID int) (*models.SalesOrder, error) {
	qt, err := s.store.GetQuotation(qtID, bizID)
	if err != nil {
		return nil, errors.New("quotation not found")
	}
	if qt.Status == "converted" {
		return nil, errors.New("quotation already converted")
	}
	if qt.Status == "rejected" || qt.Status == "expired" {
		return nil, errors.New("cannot convert a rejected or expired quotation")
	}

	var orderItems []OrderItemInput
	for _, it := range qt.Items {
		orderItems = append(orderItems, OrderItemInput{
			ProductID: it.ProductID, Quantity: it.Quantity,
			UnitPrice: it.UnitPrice, TaxRate: it.TaxRate, Discount: it.Discount,
		})
	}

	so, err := s.CreateSalesOrder(bizID, qt.CustomerID, qt.WarehouseID, &qtID, "", nil, qt.Notes, orderItems)
	if err != nil {
		return nil, err
	}
	_ = s.store.UpdateQuotationStatus(qtID, "converted")
	return so, nil
}

// ── Sales Orders ──────────────────────────────────────────────────────────────

func (s *CRMService) CreateSalesOrder(bizID, customerID, warehouseID int, quotationID *int, shippingAddr string, deliveryDate *time.Time, notes string, items []OrderItemInput) (*models.SalesOrder, error) {
	if customerID <= 0 {
		return nil, errors.New("please select a customer")
	}
	if len(items) == 0 {
		return nil, errors.New("sales order must have at least one item")
	}
	cust, err := s.store.GetCustomer(customerID, bizID)
	if err != nil {
		return nil, errors.New("customer not found")
	}

	var soItems []models.SalesOrderItem
	var subtotal, taxTotal float64
	for _, item := range items {
		if item.Quantity <= 0 {
			return nil, errors.New("quantity must be greater than zero")
		}
		prod, err := s.productStore.Get(item.ProductID, bizID)
		if err != nil {
			return nil, fmt.Errorf("product %d not found", item.ProductID)
		}
		lineSubtotal := math.Round(float64(item.Quantity)*item.UnitPrice*100) / 100
		taxAmt := math.Round(lineSubtotal*item.TaxRate/100*100) / 100
		lineTotal := lineSubtotal + taxAmt - item.Discount
		subtotal += lineSubtotal
		taxTotal += taxAmt
		soItems = append(soItems, models.SalesOrderItem{
			ProductID: prod.ID, ProductName: prod.Name, SKU: prod.SKU,
			Quantity: item.Quantity, UnitPrice: item.UnitPrice,
			TaxRate: item.TaxRate, TaxAmount: taxAmt,
			Discount: item.Discount, LineTotal: math.Round(lineTotal*100) / 100,
		})
	}

	return s.store.CreateOrder(&models.SalesOrder{
		BusinessID:      bizID,
		CustomerID:      customerID,
		CustomerName:    cust.Name,
		OrderNumber:     s.store.NextOrderNumber(bizID),
		QuotationID:     quotationID,
		WarehouseID:     warehouseID,
		ShippingAddress: strings.TrimSpace(shippingAddr),
		DeliveryDate:    deliveryDate,
		Notes:           strings.TrimSpace(notes),
		Subtotal:        math.Round(subtotal*100) / 100,
		TaxTotal:        math.Round(taxTotal*100) / 100,
		GrandTotal:      math.Round((subtotal+taxTotal)*100) / 100,
		Items:           soItems,
	})
}

func (s *CRMService) GetOrder(id, bizID int) (*models.SalesOrder, error) {
	return s.store.GetOrder(id, bizID)
}

func (s *CRMService) ListOrders(bizID int, status string) ([]models.SalesOrder, error) {
	return s.store.ListOrders(bizID, status)
}

// ConfirmOrder creates stock reservations and moves order to 'confirmed'.
// Available stock = warehouse_stock - active reservations.
func (s *CRMService) ConfirmOrder(id, bizID int) error {
	order, err := s.store.GetOrder(id, bizID)
	if err != nil {
		return err
	}
	if order.Status != "draft" {
		return errors.New("only draft orders can be confirmed")
	}
	if order.WarehouseID <= 0 {
		return errors.New("order has no warehouse assigned")
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var reservations []models.StockReservation
	for _, item := range order.Items {
		// Check available stock = warehouse_stock - active reservations.
		var warehouseQty int
		_ = tx.QueryRow(`SELECT COALESCE(quantity,0) FROM warehouse_stock WHERE warehouse_id=? AND product_id=? FOR UPDATE`,
			order.WarehouseID, item.ProductID).Scan(&warehouseQty)

		reservedAlready, _ := s.store.GetReservedQty(item.ProductID, order.WarehouseID, bizID)
		available := warehouseQty - reservedAlready

		if available < item.Quantity {
			return fmt.Errorf("%s: only %d available (need %d)", item.ProductName, available, item.Quantity)
		}

		if err = s.store.UpdateOrderItemReserved(tx, item.ID, item.Quantity); err != nil {
			return err
		}

		expiresAt := time.Now().Add(7 * 24 * time.Hour) // 7-day reservation
		reservations = append(reservations, models.StockReservation{
			BusinessID:  bizID,
			WarehouseID: order.WarehouseID,
			ProductID:   item.ProductID,
			OrderID:     order.ID,
			OrderItemID: item.ID,
			ReservedQty: item.Quantity,
			ExpiresAt:   &expiresAt,
		})
	}

	if err = s.store.CreateReservationsTx(tx, reservations); err != nil {
		return err
	}
	if err = s.store.UpdateOrderStatus(tx, id, "confirmed"); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *CRMService) PackOrder(id, bizID int) error {
	return s.updateOrderStatus(id, bizID, "confirmed", "packed")
}

func (s *CRMService) CancelOrder(id, bizID int) error {
	order, err := s.store.GetOrder(id, bizID)
	if err != nil {
		return err
	}
	if order.Status == "completed" || order.Status == "cancelled" {
		return errors.New("cannot cancel a completed or already cancelled order")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err = s.store.ReleaseReservationsTx(tx, id); err != nil {
		return err
	}
	if err = s.store.UpdateOrderStatus(tx, id, "cancelled"); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *CRMService) updateOrderStatus(id, bizID int, requiredStatus, newStatus string) error {
	order, err := s.store.GetOrder(id, bizID)
	if err != nil {
		return err
	}
	if order.Status != requiredStatus {
		return fmt.Errorf("order must be in '%s' status", requiredStatus)
	}
	return s.store.UpdateOrderStatusDirect(id, newStatus)
}

// ── Available Stock ───────────────────────────────────────────────────────────

// AvailableStock returns warehouse stock minus active reservations.
func (s *CRMService) AvailableStock(productID, warehouseID, bizID int) (warehouseQty, reservedQty, available int) {
	_ = s.db.QueryRow(`SELECT COALESCE(quantity,0) FROM warehouse_stock WHERE warehouse_id=? AND product_id=? AND business_id=?`,
		warehouseID, productID, bizID).Scan(&warehouseQty)
	reservedQty, _ = s.store.GetReservedQty(productID, warehouseID, bizID)
	available = warehouseQty - reservedQty
	return
}

// ── Delivery Challans ─────────────────────────────────────────────────────────

type ChallanItemInput struct {
	OrderItemID int
	ProductID   int
	Quantity    int
	BatchID     *int
}

// CreateDeliveryChallan builds a challan from an order and dispatches it,
// deducting warehouse stock and fulfilling reservations atomically.
func (s *CRMService) CreateDeliveryChallan(bizID, orderID int, courierName, trackingNumber, notes string, items []ChallanItemInput) (*models.DeliveryChallan, error) {
	order, err := s.store.GetOrder(orderID, bizID)
	if err != nil {
		return nil, errors.New("order not found")
	}
	if order.Status != "confirmed" && order.Status != "packed" {
		return nil, errors.New("order must be confirmed or packed to create a delivery challan")
	}
	if len(items) == 0 {
		return nil, errors.New("delivery challan must have at least one item")
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var challanItems []models.DeliveryChallanItem
	for _, item := range items {
		if item.Quantity <= 0 {
			continue
		}
		prod, err := s.productStore.Get(item.ProductID, bizID)
		if err != nil {
			return nil, fmt.Errorf("product %d not found", item.ProductID)
		}

		note := fmt.Sprintf("Delivery Challan — Order %s", order.OrderNumber)

		// FEFO batch deduction if batches exist.
		hasBatches, _ := s.batchStore.HasBatches(tx, item.ProductID, order.WarehouseID, bizID)
		if hasBatches {
			deductions, err := s.batchStore.SelectFEFOTx(tx, item.ProductID, order.WarehouseID, bizID, item.Quantity)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", prod.Name, err)
			}
			for _, d := range deductions {
				if err = s.batchStore.DeductBatchTx(tx, d.BatchID, d.Quantity, "sale_out", "delivery_challan", orderID, note); err != nil {
					return nil, fmt.Errorf("%s: %w", prod.Name, err)
				}
			}
		}

		// Deduct warehouse stock.
		if err = s.whStore.AdjustWarehouseStockTx(tx, order.WarehouseID, item.ProductID, bizID, -item.Quantity, "sale", note); err != nil {
			return nil, fmt.Errorf("%s: %w", prod.Name, err)
		}

		// Update delivered qty on order item.
		if err = s.store.UpdateOrderItemDelivered(tx, item.OrderItemID, item.Quantity); err != nil {
			return nil, err
		}

		batchNumber := ""
		if item.BatchID != nil {
			_ = tx.QueryRow(`SELECT batch_number FROM batches WHERE id=?`, *item.BatchID).Scan(&batchNumber)
		}
		challanItems = append(challanItems, models.DeliveryChallanItem{
			OrderItemID: item.OrderItemID, ProductID: item.ProductID,
			ProductName: prod.Name, SKU: prod.SKU,
			Quantity: item.Quantity, BatchID: item.BatchID, BatchNumber: batchNumber,
		})
	}

	// Fulfill reservations.
	if err = s.store.FulfillReservationsTx(tx, orderID); err != nil {
		return nil, err
	}

	now := time.Now()
	ch := &models.DeliveryChallan{
		BusinessID:     bizID,
		OrderID:        orderID,
		CustomerID:     order.CustomerID,
		CustomerName:   order.CustomerName,
		ChallanNumber:  s.store.NextChallanNumber(bizID),
		WarehouseID:    order.WarehouseID,
		CourierName:    strings.TrimSpace(courierName),
		TrackingNumber: strings.TrimSpace(trackingNumber),
		DispatchDate:   &now,
		Notes:          strings.TrimSpace(notes),
		Items:          challanItems,
	}
	challanID, err := s.store.CreateChallanTx(tx, ch)
	if err != nil {
		return nil, err
	}
	if err = s.store.UpdateChallanStatus(tx, int(challanID), "dispatched"); err != nil {
		return nil, err
	}
	if err = s.store.UpdateOrderStatus(tx, orderID, "dispatched"); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return s.store.GetChallan(int(challanID), bizID)
}

func (s *CRMService) ListChallans(bizID int) ([]models.DeliveryChallan, error) {
	return s.store.ListChallans(bizID)
}

func (s *CRMService) GetChallan(id, bizID int) (*models.DeliveryChallan, error) {
	return s.store.GetChallan(id, bizID)
}

func (s *CRMService) MarkDelivered(id, bizID int) error {
	order, _ := s.store.GetChallan(id, bizID)
	if order == nil {
		return errors.New("challan not found")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now()
	if _, err = tx.Exec(`UPDATE delivery_challans SET status='delivered', delivery_date=?, updated_at=CURRENT_TIMESTAMP WHERE id=? AND business_id=?`, now, id, bizID); err != nil {
		return err
	}
	// Mark order as delivered/completed
	if _, err = tx.Exec(`UPDATE sales_orders SET status='delivered', updated_at=CURRENT_TIMESTAMP WHERE id=? AND business_id=?`, order.OrderID, bizID); err != nil {
		return err
	}
	return tx.Commit()
}

// ── Customer Payments ─────────────────────────────────────────────────────────

func (s *CRMService) RecordPayment(bizID, customerID, orderID int, amount float64, method, payType, reference, notes string) (*models.CustomerPayment, error) {
	if customerID <= 0 {
		return nil, errors.New("please select a customer")
	}
	if amount <= 0 {
		return nil, errors.New("payment amount must be greater than zero")
	}
	validMethods := map[string]bool{"cash": true, "card": true, "upi": true, "bank_transfer": true, "cheque": true}
	if !validMethods[method] {
		method = "cash"
	}
	validTypes := map[string]bool{"advance": true, "regular": true, "refund": true}
	if !validTypes[payType] {
		payType = "regular"
	}

	cust, err := s.store.GetCustomer(customerID, bizID)
	if err != nil {
		return nil, errors.New("customer not found")
	}
	orderNumber := ""
	if orderID > 0 {
		if o, err := s.store.GetOrder(orderID, bizID); err == nil {
			orderNumber = o.OrderNumber
		}
	}
	return s.store.CreatePayment(&models.CustomerPayment{
		BusinessID: bizID, CustomerID: customerID, CustomerName: cust.Name,
		OrderID: orderID, OrderNumber: orderNumber,
		Amount: math.Round(amount*100) / 100, PaymentMethod: method, PaymentType: payType,
		Reference: strings.TrimSpace(reference), Notes: strings.TrimSpace(notes),
	})
}

func (s *CRMService) ListPayments(bizID, customerID int) ([]models.CustomerPayment, error) {
	return s.store.ListPayments(bizID, customerID)
}

// ── Dashboard ─────────────────────────────────────────────────────────────────

func (s *CRMService) Stats(bizID int) (models.CRMStats, error) {
	return s.store.Stats(bizID)
}
