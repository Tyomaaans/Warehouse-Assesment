package products

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"warehouse/internal/products/domain"
	"warehouse/pkg"
)

type productRepository struct{ db *gorm.DB }

func NewProductRepository(db *gorm.DB) domain.ProductRepository { return &productRepository{db: db} }

func (r *productRepository) CreateProduct(ctx context.Context, e *domain.ProductEntity) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		storage := ProductStorage{ID: e.ID, SKU: e.SKU, Name: e.Name, CreatedAt: e.CreatedAt}
		if err := tx.Create(&storage).Error; err != nil {
			return err
		}
		return tx.Exec("INSERT INTO inventory_balances(id,product_id,quantity,updated_at) VALUES (?,?,0,?)", uuid.New(), e.ID, e.CreatedAt).Error
	})
	return pkg.HandleDBError(err)
}

func (r *productRepository) GetProducts(ctx context.Context, search string, page, limit int) ([]domain.ProductEntity, int64, error) {
	query := r.db.WithContext(ctx).Model(&ProductStorage{})
	if search != "" {
		query = query.Where("sku ILIKE ? OR name ILIKE ?", pkg.SearchPattern(search), pkg.SearchPattern(search))
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, pkg.HandleDBError(err)
	}
	var storages []ProductStorage
	if err := query.Order("created_at DESC, id ASC").Offset((page - 1) * limit).Limit(limit).Find(&storages).Error; err != nil {
		return nil, 0, pkg.HandleDBError(err)
	}
	result := make([]domain.ProductEntity, len(storages))
	for i, row := range storages {
		result[i] = domain.ProductEntity{ID: row.ID, SKU: row.SKU, Name: row.Name, CreatedAt: row.CreatedAt}
	}
	return result, total, nil
}
