package inventory

import (
	"warehouse/internal/inventory/domain"
	"warehouse/pkg"
)

func ToBalanceResponse(e domain.BalanceEntity) BalanceResponse {
	return BalanceResponse{
		ID: pkg.EncodeID(e.ID), ProductID: pkg.EncodeID(e.ProductID),
		SKU: e.SKU, Name: e.Name, Quantity: e.Quantity, UpdatedAt: e.UpdatedAt,
	}
}

func ToMovementResponse(e domain.MovementEntity) MovementResponse {
	return MovementResponse{
		ID: pkg.EncodeID(e.ID), ProductID: pkg.EncodeID(e.ProductID),
		ReceivingTransactionItemID: pkg.EncodeID(e.ReceivingTransactionItemID),
		ReceivingTransactionID:     pkg.EncodeID(e.ReceivingTransactionID), ReceivingDocumentID: pkg.EncodeID(e.ReceivingDocumentID),
		QuantityDelta: e.QuantityDelta, CreatedAt: e.CreatedAt,
	}
}
