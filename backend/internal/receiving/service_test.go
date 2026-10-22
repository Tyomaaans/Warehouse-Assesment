package receiving

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"warehouse/internal/receiving/domain"
	"warehouse/pkg"
)

type receivingRepositoryStub struct {
	document    *domain.DocumentEntity
	audit       domain.AuditEntity
	err         error
	transaction *domain.TransactionEntity
	tx          domain.ReceivingTx
}

func (r *receivingRepositoryStub) CreateDocument(_ context.Context, document *domain.DocumentEntity, audit domain.AuditEntity) error {
	r.document, r.audit = document, audit
	return r.err
}

func (r *receivingRepositoryStub) GetDocuments(context.Context, string, int, int) ([]domain.DocumentEntity, int64, error) {
	return nil, 0, r.err
}

func (r *receivingRepositoryStub) GetDocument(context.Context, uuid.UUID) (*domain.DocumentEntity, error) {
	return r.document, r.err
}

func (*receivingRepositoryStub) GetTransactions(context.Context, uuid.UUID, int, int) ([]domain.TransactionEntity, int64, error) {
	return nil, 0, pkg.ErrInternal
}

func (r *receivingRepositoryStub) GetTransaction(context.Context, uuid.UUID) (*domain.TransactionEntity, error) {
	return r.transaction, r.err
}

func (r *receivingRepositoryStub) WithDocument(_ context.Context, _ uuid.UUID, fn func(domain.ReceivingTx) error) error {
	if r.tx != nil {
		return fn(r.tx)
	}
	return pkg.ErrInternal
}

func (*receivingRepositoryStub) GetAuditLogs(context.Context, *uuid.UUID, int, int) ([]domain.AuditRecordEntity, int64, error) {
	return nil, 0, nil
}

func TestCreateDocumentUsesTrustedActorAndSnapshots(t *testing.T) {
	repo := &receivingRepositoryStub{}
	service := NewReceivingService(repo, pkg.NewValidator())
	actor := pkg.Actor{ID: uuid.New(), SessionID: uuid.New(), Role: pkg.Operator}
	ctx := pkg.WithActor(context.Background(), actor)
	productID := pkg.EncodeID(uuid.New())
	req := CreateReceivingRequest{OrderReference: "ORD-001", Items: []ExpectedItemRequest{{ProductID: productID, ExpectedQuantity: 130}}}
	response, err := service.CreateDocument(ctx, req)
	if err != nil {
		t.Fatalf("CreateDocument() error = %v", err)
	}
	if response.Status != domain.Incomplete || response.CompletedAt != nil || response.CreatedBy != pkg.EncodeID(actor.ID) || response.Items[0].ExpectedQuantity != 130 || response.Items[0].ApprovedQuantity != 0 || response.Items[0].RemainingQuantity != 130 {
		t.Fatalf("incorrect initial receiving state = %#v", response)
	}
	if repo.document.CreatedBy != actor.ID || repo.audit.ActorID == nil || *repo.audit.ActorID != actor.ID || repo.audit.EventType != "RECEIVING_CREATED" || repo.audit.TransactionID != nil || repo.audit.CreatedAt.IsZero() {
		t.Fatal("creation identity and audit must be server supplied")
	}
	for _, id := range []string{response.ID, response.Items[0].ID, response.Items[0].ProductID} {
		if _, err := pkg.ParseID(id); err != nil {
			t.Fatal("response ID is not canonical UUID v4")
		}
	}
	req.Items[0].ExpectedQuantity = 999
	if repo.document.Items[0].ExpectedQuantity != 130 {
		t.Fatal("expected quantity retained a mutable request reference")
	}
}

func TestReceivingServiceAuthorizationAndValidation(t *testing.T) {
	repo := &receivingRepositoryStub{}
	service := NewReceivingService(repo, pkg.NewValidator())
	operator := pkg.WithActor(context.Background(), pkg.Actor{ID: uuid.New(), SessionID: uuid.New(), Role: pkg.Operator})
	supervisor := pkg.WithActor(context.Background(), pkg.Actor{ID: uuid.New(), SessionID: uuid.New(), Role: pkg.Supervisor})
	valid := CreateReceivingRequest{OrderReference: "ORD-001", Items: []ExpectedItemRequest{{ProductID: pkg.EncodeID(uuid.New()), ExpectedQuantity: 1}}}
	if _, err := service.CreateDocument(context.Background(), valid); !errors.Is(err, pkg.ErrUnauthorized) {
		t.Fatalf("anonymous create error = %v", err)
	}
	if _, err := service.CreateDocument(supervisor, valid); !errors.Is(err, pkg.ErrForbidden) || repo.document != nil {
		t.Fatalf("Supervisor create error = %v, want forbidden before persistence", err)
	}
	for _, req := range []CreateReceivingRequest{
		{OrderReference: " "}, {OrderReference: "ORD-001"},
		{OrderReference: "ORD-001", Items: []ExpectedItemRequest{{ProductID: uuid.NewString(), ExpectedQuantity: 1}}},
		{OrderReference: "ORD-001", Items: []ExpectedItemRequest{{ProductID: valid.Items[0].ProductID, ExpectedQuantity: 0}}},
		{OrderReference: "ORD-001", Items: []ExpectedItemRequest{valid.Items[0], valid.Items[0]}},
	} {
		if _, err := service.CreateDocument(operator, req); !errors.Is(err, pkg.ErrInvalidInput) || repo.document != nil {
			t.Fatalf("invalid document error = %v, want invalid input before persistence", err)
		}
	}
	if _, err := service.GetDocument(context.Background(), valid.Items[0].ProductID); !errors.Is(err, pkg.ErrUnauthorized) {
		t.Fatalf("anonymous detail error = %v", err)
	}
	if _, err := service.GetDocuments(context.Background(), "", 1, 10); !errors.Is(err, pkg.ErrUnauthorized) {
		t.Fatalf("anonymous list error = %v", err)
	}
	if _, err := service.GetDocument(operator, uuid.NewString()); !errors.Is(err, pkg.ErrInvalidInput) {
		t.Fatalf("text UUID detail error = %v", err)
	}
	if _, err := service.GetDocuments(operator, "REJECTED", 1, 10); !errors.Is(err, pkg.ErrInvalidInput) {
		t.Fatalf("invalid status error = %v", err)
	}
	if _, err := service.GetDocuments(operator, "", 1, 0); !errors.Is(err, pkg.ErrInvalidInput) {
		t.Fatalf("invalid pagination error = %v", err)
	}
	for _, ctx := range []context.Context{operator, supervisor} {
		page, err := service.GetDocuments(ctx, "", 1, 10)
		if err != nil || page.Receivings == nil || page.Meta.TotalPages != 0 {
			t.Fatalf("authenticated empty list error = %v", err)
		}
	}
}
