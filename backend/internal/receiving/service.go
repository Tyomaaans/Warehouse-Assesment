package receiving

import (
	"context"
	"fmt"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"

	"warehouse/internal/receiving/domain"
	"warehouse/pkg"
)

// Interface
type ReceivingService interface {
	CreateDocument(context.Context, CreateReceivingRequest) (*ReceivingResponse, error)
	GetDocuments(context.Context, string, int, int) (*PaginatedReceivingResponse, error)
	GetDocument(context.Context, string) (*ReceivingResponse, error)
	CreateTransaction(context.Context, string, CreateTransactionRequest) (*TransactionResponse, error)
	GetTransactions(context.Context, string, int, int) (*PaginatedTransactionResponse, error)
	GetTransaction(context.Context, string) (*TransactionResponse, error)
	CorrectTransaction(context.Context, string, CorrectTransactionRequest) (*TransactionResponse, error)
	ReviewTransaction(context.Context, string, ReviewTransactionRequest) (*TransactionResponse, error)
	ApproveTransaction(context.Context, string) (*TransactionResponse, error)
	GetAuditLogs(context.Context, string, int, int) (*PaginatedAuditResponse, error)
}

// Implementation
type receivingService struct {
	receivingRepo domain.ReceivingRepository
	validate      *validator.Validate
}

func NewReceivingService(receivingRepo domain.ReceivingRepository, validate *validator.Validate) ReceivingService {
	return &receivingService{receivingRepo: receivingRepo, validate: validate}
}

func (s *receivingService) CreateDocument(ctx context.Context, req CreateReceivingRequest) (*ReceivingResponse, error) {
	actor, err := pkg.RequireActor(ctx, pkg.Operator)
	if err != nil {
		return nil, err
	}
	if err := pkg.ValidateStruct(s.validate, req); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	document := &domain.DocumentEntity{
		ID: uuid.New(), OrderReference: req.OrderReference, Status: domain.Incomplete,
		CreatedBy: actor.ID, CreatedAt: now, UpdatedAt: now,
		Items: make([]domain.DocumentItemEntity, len(req.Items)),
	}
	products := make(map[uuid.UUID]bool, len(req.Items))
	for i, item := range req.Items {
		productID, err := pkg.ParseID(item.ProductID)
		if err != nil {
			return nil, err
		}
		if products[productID] {
			return nil, fmt.Errorf("%w: items[%d].product_id duplicates another SKU line", pkg.ErrInvalidInput, i)
		}
		products[productID] = true
		document.Items[i] = domain.DocumentItemEntity{
			ID: uuid.New(), ReceivingDocumentID: document.ID,
			ProductID: productID, ExpectedQuantity: item.ExpectedQuantity,
		}
	}
	audit := domain.AuditEntity{
		DocumentID: document.ID, ActorID: &actor.ID,
		EventType: "RECEIVING_CREATED", CreatedAt: now,
	}
	if err := s.receivingRepo.CreateDocument(ctx, document, audit); err != nil {
		return nil, err
	}
	return ToReceivingResponse(document), nil
}

func (s *receivingService) GetDocuments(ctx context.Context, status string, page, limit int) (*PaginatedReceivingResponse, error) {
	if _, err := pkg.RequireActor(ctx, ""); err != nil {
		return nil, err
	}
	if err := pkg.ValidatePagination(page, limit); err != nil {
		return nil, err
	}
	if status != "" && status != domain.Incomplete && status != domain.Completed {
		return nil, fmt.Errorf("%w: status must be INCOMPLETE or COMPLETED", pkg.ErrInvalidInput)
	}
	list, total, err := s.receivingRepo.GetDocuments(ctx, status, page, limit)
	if err != nil {
		return nil, err
	}
	result := &PaginatedReceivingResponse{
		Receivings: make([]ReceivingSummary, len(list)), Meta: pkg.NewMeta(page, limit, total),
	}
	for i := range list {
		result.Receivings[i] = ToReceivingSummary(&list[i])
	}
	return result, nil
}

func (s *receivingService) GetDocument(ctx context.Context, rawID string) (*ReceivingResponse, error) {
	if _, err := pkg.RequireActor(ctx, ""); err != nil {
		return nil, err
	}
	id, err := pkg.ParseID(rawID)
	if err != nil {
		return nil, err
	}
	document, err := s.receivingRepo.GetDocument(ctx, id)
	if err != nil {
		return nil, err
	}
	return ToReceivingResponse(document), nil
}
