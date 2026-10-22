package inventory

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"warehouse/internal/inventory/domain"
	"warehouse/pkg"
)

type inventoryRepository struct {
	db *gorm.DB
}

func NewInventoryRepository(db *gorm.DB) domain.InventoryRepository {
	return &inventoryRepository{db: db}
}

func (r *inventoryRepository) GetBalances(ctx context.Context, search string, page, limit int) ([]domain.BalanceEntity, int64, error) {
	rows := make([]domain.BalanceEntity, 0)
	var total int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Table("inventory_balances b").Joins("JOIN products p ON p.id=b.product_id")
		if search != "" {
			query = query.Where("p.sku ILIKE ? OR p.name ILIKE ?", pkg.SearchPattern(search), pkg.SearchPattern(search))
		}
		if err := query.Count(&total).Error; err != nil {
			return err
		}
		return query.Select("b.*, p.sku, p.name").Order("p.sku ASC, b.id ASC").Limit(limit).Offset((page - 1) * limit).Scan(&rows).Error
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return rows, total, pkg.HandleDBError(err)
}

func (r *inventoryRepository) GetMovements(ctx context.Context, productID *uuid.UUID, page, limit int) ([]domain.MovementEntity, int64, error) {
	rows := make([]domain.MovementEntity, 0)
	var total int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Table("inventory_transactions m").
			Joins("JOIN receiving_transaction_items ti ON ti.id=m.receiving_transaction_item_id").
			Joins("JOIN receiving_transactions t ON t.id=ti.receiving_transaction_id")
		if productID != nil {
			query = query.Where("m.product_id = ?", *productID)
		}
		if err := query.Count(&total).Error; err != nil {
			return err
		}
		return query.Select("m.*, ti.receiving_transaction_id, t.receiving_document_id").Order("m.created_at DESC, m.id ASC").Limit(limit).Offset((page - 1) * limit).Scan(&rows).Error
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return rows, total, pkg.HandleDBError(err)
}
