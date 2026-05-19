package services

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"go-monolith/models"
)

const defaultAlertDays = 30

// BatchService manages batch lifecycle, FEFO deduction, and expiry tracking.
type BatchService struct {
	db         *sql.DB
	store      *models.BatchStore
	whStore    *models.WarehouseStore
	prodStore  *models.ProductStore
	AlertDays  int
}

func NewBatchService(db *sql.DB, store *models.BatchStore, wh *models.WarehouseStore, prod *models.ProductStore) *BatchService {
	return &BatchService{db: db, store: store, whStore: wh, prodStore: prod, AlertDays: defaultAlertDays}
}

// ── Batch CRUD ────────────────────────────────────────────────────────────────

func (s *BatchService) List(bizID, warehouseID, productID int) ([]models.Batch, error) {
	return s.store.List(bizID, warehouseID, productID)
}

func (s *BatchService) Get(id, bizID int) (*models.Batch, error) {
	if id <= 0 {
		return nil, errors.New("invalid batch ID")
	}
	return s.store.Get(id, bizID)
}

// ReceiveBatch creates a batch record AND adjusts warehouse stock in one transaction.
// This is called when goods arrive with known batch/lot/expiry info.
func (s *BatchService) ReceiveBatch(bizID, productID, warehouseID, qty int,
	batchNumber, lotNumber, notes string,
	mfgDate, expiryDate *time.Time,
	changeNote string,
) (*models.Batch, error) {
	if bizID <= 0 || productID <= 0 || warehouseID <= 0 {
		return nil, errors.New("invalid IDs")
	}
	if qty <= 0 {
		return nil, errors.New("batch quantity must be greater than zero")
	}
	batchNumber = strings.TrimSpace(batchNumber)

	// Validate product + warehouse belong to business.
	if _, err := s.prodStore.Get(productID, bizID); err != nil {
		return nil, fmt.Errorf("product not found: %w", err)
	}
	if _, err := s.whStore.Get(warehouseID, bizID); err != nil {
		return nil, fmt.Errorf("warehouse not found: %w", err)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Create the batch record.
	batch := &models.Batch{
		BusinessID:  bizID,
		ProductID:   productID,
		WarehouseID: warehouseID,
		BatchNumber: batchNumber,
		LotNumber:   strings.TrimSpace(lotNumber),
		Quantity:    qty,
		Notes:       strings.TrimSpace(notes),
		MfgDate:     mfgDate,
		ExpiryDate:  expiryDate,
	}
	// Insert batch inside transaction.
	res, err := tx.Exec(
		`INSERT INTO batches
		 (business_id, product_id, warehouse_id, batch_number, lot_number, mfg_date, expiry_date, quantity, status, notes)
		 VALUES (?,?,?,?,?,?,?,?,'active',?)`,
		batch.BusinessID, batch.ProductID, batch.WarehouseID, batch.BatchNumber, batch.LotNumber,
		batch.MfgDate, batch.ExpiryDate, batch.Quantity, batch.Notes,
	)
	if err != nil {
		return nil, err
	}
	batchID, _ := res.LastInsertId()
	_, _ = tx.Exec(
		`INSERT INTO batch_logs (batch_id, product_id, warehouse_id, business_id, change_type, qty_before, qty_change, qty_after, note)
		 VALUES (?,?,?,?,'purchase_in',0,?,?,?)`,
		batchID, productID, warehouseID, bizID, qty, qty, "Batch received",
	)

	// Also adjust warehouse stock.
	if changeNote == "" {
		changeNote = "Batch " + batchNumber + " received"
	}
	if err = s.whStore.AdjustWarehouseStockTx(tx, warehouseID, productID, bizID, qty, "purchase", changeNote); err != nil {
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return s.store.Get(int(batchID), bizID)
}

func (s *BatchService) UpdateBatch(id, bizID int, batchNumber, lotNumber, notes string, mfgDate, expiryDate *time.Time) error {
	if id <= 0 {
		return errors.New("invalid batch ID")
	}
	b, err := s.store.Get(id, bizID)
	if err != nil {
		return errors.New("batch not found")
	}
	b.BatchNumber = strings.TrimSpace(batchNumber)
	b.LotNumber = strings.TrimSpace(lotNumber)
	b.Notes = strings.TrimSpace(notes)
	b.MfgDate = mfgDate
	b.ExpiryDate = expiryDate
	return s.store.Update(b)
}

func (s *BatchService) WriteOffExpired(bizID int) (int, error) {
	return s.store.WriteOffExpiredBatches(bizID)
}

// ── FEFO deduction (called by POS and Returns services) ───────────────────────

// DeductFEFO deducts `qty` units from the given product+warehouse using FEFO ordering.
// Returns the batch deductions made (for receipt/logging).
// If no batches exist, returns (nil, nil) → caller should fall back to plain stock deduction.
func (s *BatchService) DeductFEFO(tx *sql.Tx, productID, warehouseID, bizID, qty int, changeType, refType string, refID int, note string) ([]models.BatchDeduction, error) {
	hasBatches, err := s.store.HasBatches(tx, productID, warehouseID, bizID)
	if err != nil {
		return nil, err
	}
	if !hasBatches {
		return nil, nil // no batches → use plain warehouse_stock
	}

	deductions, err := s.store.SelectFEFOTx(tx, productID, warehouseID, bizID, qty)
	if err != nil {
		return nil, err
	}

	for _, d := range deductions {
		if err = s.store.DeductBatchTx(tx, d.BatchID, d.Quantity, changeType, refType, refID, note); err != nil {
			return nil, err
		}
	}
	return deductions, nil
}

// AddToBatch restores qty to a specific batch (for sales returns with resalable condition).
func (s *BatchService) AddToBatch(tx *sql.Tx, batchID, qty int, changeType, note string) error {
	return s.store.AddToBatchTx(tx, batchID, qty, changeType, note)
}

// ── Expiry ────────────────────────────────────────────────────────────────────

func (s *BatchService) ExpiryStats(bizID int) (models.ExpiryStats, error) {
	return s.store.ExpiryStats(bizID, s.AlertDays)
}

func (s *BatchService) ExpiringList(bizID int) ([]models.Batch, error) {
	return s.store.ExpiringList(bizID, s.AlertDays)
}

func (s *BatchService) ExpiredList(bizID int) ([]models.Batch, error) {
	return s.store.ExpiredList(bizID)
}

func (s *BatchService) BatchLogs(bizID, productID int) ([]models.BatchLog, error) {
	return s.store.BatchLogs(bizID, productID, 200)
}
