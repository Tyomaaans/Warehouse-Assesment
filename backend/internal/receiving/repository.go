package receiving

import (
	"context"
	"database/sql"
	"encoding/json"
	"sort"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"warehouse/internal/receiving/domain"
	"warehouse/pkg"
)

type receivingRepository struct{ db *gorm.DB }
type receivingTx struct {
	db       *gorm.DB
	document *domain.DocumentEntity
}

func NewReceivingRepository(db *gorm.DB) domain.ReceivingRepository {
	return &receivingRepository{db: db}
}

func (r *receivingRepository) CreateDocument(ctx context.Context, e *domain.DocumentEntity, audit domain.AuditEntity) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		storage := DocumentStorage{ID: e.ID, OrderReference: e.OrderReference, Status: e.Status, CreatedBy: e.CreatedBy, CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt}
		if err := tx.Create(&storage).Error; err != nil {
			return err
		}
		for _, item := range e.Items {
			row := DocumentItemStorage{ID: item.ID, ReceivingDocumentID: e.ID, ProductID: item.ProductID, ExpectedQuantity: item.ExpectedQuantity}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		}
		loaded, err := loadDocument(tx, e.ID, false)
		if err != nil {
			return err
		}
		*e = *loaded
		audit.After = ToReceivingResponse(e)
		return (&receivingTx{db: tx, document: e}).AppendAudit(audit)
	})
	return pkg.HandleDBError(err)
}

func (r *receivingRepository) GetDocuments(ctx context.Context, status string, page, limit int) ([]domain.DocumentEntity, int64, error) {
	query := r.db.WithContext(ctx).Model(&DocumentStorage{})
	if status != "" {
		query = query.Where("status=?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, pkg.HandleDBError(err)
	}
	var rows []DocumentStorage
	if err := query.Order("created_at DESC,id ASC").Limit(limit).Offset((page - 1) * limit).Find(&rows).Error; err != nil {
		return nil, 0, pkg.HandleDBError(err)
	}
	result := make([]domain.DocumentEntity, len(rows))
	for i, row := range rows {
		result[i] = ToDocumentEntity(row)
	}
	return result, total, nil
}

func (r *receivingRepository) GetDocument(ctx context.Context, id uuid.UUID) (*domain.DocumentEntity, error) {
	var result *domain.DocumentEntity
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		result, err = loadDocument(tx, id, false)
		return err
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return result, pkg.HandleDBError(err)
}

func (r *receivingRepository) GetTransactions(ctx context.Context, id uuid.UUID, page, limit int) ([]domain.TransactionEntity, int64, error) {
	var parent DocumentStorage
	if err := r.db.WithContext(ctx).First(&parent, "id=?", id).Error; err != nil {
		return nil, 0, pkg.HandleDBError(err)
	}
	query := r.db.WithContext(ctx).Model(&TransactionStorage{}).Where("receiving_document_id=?", id)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, pkg.HandleDBError(err)
	}
	var rows []TransactionStorage
	if err := query.Order("created_at DESC,id ASC").Limit(limit).Offset((page - 1) * limit).Find(&rows).Error; err != nil {
		return nil, 0, pkg.HandleDBError(err)
	}
	result := make([]domain.TransactionEntity, len(rows))
	for i, row := range rows {
		result[i] = ToTransactionEntity(row)
		if err := loadTransactionItems(r.db.WithContext(ctx), &result[i]); err != nil {
			return nil, 0, pkg.HandleDBError(err)
		}
	}
	return result, total, nil
}

func (r *receivingRepository) GetTransaction(ctx context.Context, id uuid.UUID) (*domain.TransactionEntity, error) {
	var result *domain.TransactionEntity
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row TransactionStorage
		if err := tx.First(&row, "id=?", id).Error; err != nil {
			return err
		}
		e := ToTransactionEntity(row)
		result = &e
		return loadTransactionItems(tx, result)
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return result, pkg.HandleDBError(err)
}

func (r *receivingRepository) WithDocument(ctx context.Context, id uuid.UUID, fn func(domain.ReceivingTx) error) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		document, err := loadDocument(tx, id, true)
		if err != nil {
			return err
		}
		return fn(&receivingTx{db: tx, document: document})
	})
	return pkg.HandleDBError(err)
}

func loadDocument(db *gorm.DB, id uuid.UUID, lock bool) (*domain.DocumentEntity, error) {
	var row DocumentStorage
	query := db
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := query.First(&row, "id=?", id).Error; err != nil {
		return nil, err
	}
	result := ToDocumentEntity(row)
	result.Items = make([]domain.DocumentItemEntity, 0)
	err := db.Raw(`SELECT di.*,p.sku,p.name,
COALESCE(SUM(ti.actual_quantity) FILTER (WHERE t.status='APPROVED'),0)::bigint AS approved_quantity
FROM receiving_document_items di JOIN products p ON p.id=di.product_id
LEFT JOIN receiving_transaction_items ti ON ti.receiving_document_item_id=di.id
LEFT JOIN receiving_transactions t ON t.id=ti.receiving_transaction_id
WHERE di.receiving_document_id=? GROUP BY di.id,p.id ORDER BY di.id`, id).Scan(&result.Items).Error
	return &result, err
}

func loadTransactionItems(db *gorm.DB, e *domain.TransactionEntity) error {
	e.Items = make([]domain.TransactionItemEntity, 0)
	return db.Table("receiving_transaction_items").Where("receiving_transaction_id=?", e.ID).Order("id ASC").Scan(&e.Items).Error
}

func (r *receivingTx) Document() *domain.DocumentEntity { return r.document }

func (r *receivingTx) HasActiveTransaction() (bool, error) {
	var count int64
	err := r.db.Model(&TransactionStorage{}).Where("receiving_document_id=? AND (status='PENDING_APPROVAL' OR (status='REJECTED_BY_SYSTEM' AND review_status='UNRESOLVED'))", r.document.ID).Count(&count).Error
	return count > 0, err
}

func (r *receivingTx) GetTransaction(id uuid.UUID) (*domain.TransactionEntity, error) {
	var row TransactionStorage
	if err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND receiving_document_id=?", id, r.document.ID).First(&row).Error; err != nil {
		return nil, err
	}
	e := ToTransactionEntity(row)
	if err := loadTransactionItems(r.db, &e); err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *receivingTx) SaveTransaction(e *domain.TransactionEntity, create, replaceItems bool) error {
	storage := ToTransactionStorage(e)
	if create {
		if err := r.db.Create(&storage).Error; err != nil {
			return err
		}
	}
	if replaceItems {
		ids := make([]uuid.UUID, len(e.Items))
		for i, item := range e.Items {
			ids[i] = item.ID
		}
		if err := r.db.Where("receiving_transaction_id=? AND id NOT IN ?", e.ID, ids).Delete(&TransactionItemStorage{}).Error; err != nil {
			return err
		}
		for _, item := range e.Items {
			row := TransactionItemStorage{ID: item.ID, ReceivingTransactionID: e.ID, ReceivingDocumentItemID: item.ReceivingDocumentItemID, ActualQuantity: item.ActualQuantity}
			if err := r.db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "id"}}, DoUpdates: clause.AssignmentColumns([]string{"actual_quantity"})}).Create(&row).Error; err != nil {
				return err
			}
		}
	}
	if !create {
		return r.db.Model(&storage).Select("status", "review_status", "rejection_reason", "review_note", "approved_by", "approved_at", "corrected_by", "corrected_at", "reviewed_by", "reviewed_at", "updated_at").Updates(&storage).Error
	}
	return nil
}

func (r *receivingTx) PostInventory(e *domain.TransactionEntity) error {
	products := make(map[uuid.UUID]uuid.UUID, len(r.document.Items))
	for _, item := range r.document.Items {
		products[item.ID] = item.ProductID
	}
	items := append([]domain.TransactionItemEntity(nil), e.Items...)
	sort.Slice(items, func(i, j int) bool {
		return products[items[i].ReceivingDocumentItemID].String() < products[items[j].ReceivingDocumentItemID].String()
	})
	for _, item := range items {
		productID := products[item.ReceivingDocumentItemID]
		if err := r.db.Exec(`INSERT INTO inventory_transactions(id,product_id,receiving_transaction_item_id,quantity_delta,created_at) VALUES (?,?,?,?,?)`, uuid.New(), productID, item.ID, item.ActualQuantity, e.UpdatedAt).Error; err != nil {
			return err
		}
		if err := r.db.Exec(`INSERT INTO inventory_balances(id,product_id,quantity,updated_at) VALUES (?,?,?,?)
ON CONFLICT(product_id) DO UPDATE SET quantity=inventory_balances.quantity+EXCLUDED.quantity,updated_at=EXCLUDED.updated_at`, uuid.New(), productID, item.ActualQuantity, e.UpdatedAt).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *receivingTx) CompleteDocument(now time.Time) error {
	r.document.Status, r.document.CompletedAt, r.document.UpdatedAt = domain.Completed, &now, now
	return r.db.Model(&DocumentStorage{}).Where("id=?", r.document.ID).Updates(map[string]any{"status": domain.Completed, "completed_at": now, "updated_at": now}).Error
}

func jsonValue(value any) (*string, error) {
	if value == nil {
		return nil, nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	result := string(data)
	return &result, nil
}

func (r *receivingTx) AppendAudit(e domain.AuditEntity) error {
	before, err := jsonValue(e.Before)
	if err != nil {
		return err
	}
	after, err := jsonValue(e.After)
	if err != nil {
		return err
	}
	return r.db.Exec(`INSERT INTO receiving_audit_logs(id,receiving_document_id,receiving_transaction_id,actor_id,event_type,before_data,after_data,reason,created_at)
VALUES (?,?,?,?,?,?::jsonb,?::jsonb,?,?)`, uuid.New(), e.DocumentID, e.TransactionID, e.ActorID, e.EventType, before, after, e.Reason, e.CreatedAt).Error
}

func (r *receivingTx) AddOutbox(event string, e *domain.TransactionEntity) error {
	var recipients []uuid.UUID
	query := r.db.Table("users").Select("id").Where("id=?", e.CreatedBy)
	if event != "TRANSACTION_APPROVED" {
		query = r.db.Table("users").Select("id").Where("role=? OR id=?", pkg.Supervisor, e.CreatedBy)
	}
	if err := query.Order("id").Scan(&recipients).Error; err != nil {
		return err
	}
	public := make([]string, len(recipients))
	for i, id := range recipients {
		public[i] = pkg.EncodeID(id)
	}
	id := uuid.New()
	payload, err := json.Marshal(map[string]any{"source_event_id": pkg.EncodeID(id), "receiving_transaction_id": pkg.EncodeID(e.ID), "receiving_document_id": pkg.EncodeID(e.ReceivingDocumentID), "event_type": event, "recipients": public, "message": event + ": " + e.Status})
	if err != nil {
		return err
	}
	return r.db.Exec(`INSERT INTO notification_outbox(id,event_type,aggregate_id,payload,created_at) VALUES (?,?,?,?::jsonb,?)`, id, event, e.ID, string(payload), e.UpdatedAt).Error
}
