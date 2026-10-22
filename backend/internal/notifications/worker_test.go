package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"

	"warehouse/internal/notifications/domain"
	"warehouse/pkg"
)

type outboxStub struct {
	event              *domain.OutboxEntity
	published, retried bool
}

func (s *outboxStub) Claim(context.Context, time.Duration) (*domain.OutboxEntity, error) {
	return s.event, nil
}
func (s *outboxStub) Published(context.Context, *domain.OutboxEntity) error {
	s.published = true
	return nil
}
func (s *outboxStub) Retry(context.Context, *domain.OutboxEntity, time.Duration) error {
	s.retried = true
	return nil
}

type publisherFunc func(context.Context, *domain.OutboxEntity) error

func (p publisherFunc) Publish(ctx context.Context, e *domain.OutboxEntity) error { return p(ctx, e) }

func TestPublisherMarksOnlyConfirmedEvents(t *testing.T) {
	for _, confirmed := range []bool{false, true} {
		repo := &outboxStub{event: &domain.OutboxEntity{ID: uuid.New(), LeaseToken: uuid.New(), Attempts: 1}}
		_, err := PublishPending(context.Background(), repo, publisherFunc(func(ctx context.Context, e *domain.OutboxEntity) error {
			if repo.published {
				t.Fatal("marked before confirmation")
			}
			if _, ok := ctx.Deadline(); !ok {
				t.Fatal("broker call is unbounded")
			}
			if !confirmed {
				return errors.New("broker did not confirm")
			}
			return nil
		}))
		if (err == nil) != confirmed || repo.published != confirmed || repo.retried == confirmed {
			t.Fatalf("confirmed=%v published=%v retry=%v err=%v", confirmed, repo.published, repo.retried, err)
		}
	}
}

type notificationStub struct {
	persistErr error
	persisted  bool
}

func (*notificationStub) GetNotifications(context.Context, uuid.UUID, int, int) ([]domain.NotificationEntity, int64, error) {
	return nil, 0, nil
}
func (*notificationStub) MarkRead(context.Context, uuid.UUID, uuid.UUID) (*domain.NotificationEntity, error) {
	return nil, pkg.ErrNotFound
}
func (s *notificationStub) PersistEvent(context.Context, domain.Event, []byte) error {
	s.persisted = s.persistErr == nil
	return s.persistErr
}

type acknowledger struct {
	repo                   *notificationStub
	acked, nacked, requeue bool
}

func (a *acknowledger) Ack(uint64, bool) error {
	if !a.repo.persisted {
		return errors.New("ack before commit")
	}
	a.acked = true
	return nil
}
func (*acknowledger) Reject(uint64, bool) error { return errors.New("unexpected reject") }
func (a *acknowledger) Nack(_ uint64, _ bool, requeue bool) error {
	a.nacked = true
	a.requeue = requeue
	return nil
}

func TestConsumerCommitBeforeAckAndRetry(t *testing.T) {
	event := domain.Event{SourceEventID: pkg.EncodeID(uuid.New()), ReceivingTransactionID: pkg.EncodeID(uuid.New()), ReceivingDocumentID: pkg.EncodeID(uuid.New()), EventType: "TRANSACTION_APPROVED", Recipients: []string{pkg.EncodeID(uuid.New())}, Message: "approved"}
	body, _ := json.Marshal(event)
	for _, failure := range []error{nil, errors.New("temporary database failure"), domain.ErrInvalidEvent} {
		repo := &notificationStub{persistErr: failure}
		ack := &acknowledger{repo: repo}
		err := HandleDelivery(context.Background(), repo, amqp.Delivery{Acknowledger: ack, DeliveryTag: 1, Body: body, MessageId: event.SourceEventID, Type: event.EventType})
		if failure == nil && (err != nil || !ack.acked || ack.nacked) {
			t.Fatalf("commit/ack error=%v", err)
		}
		if failure != nil && (err == nil || ack.acked || !ack.nacked || ack.requeue == errors.Is(failure, domain.ErrInvalidEvent)) {
			t.Fatalf("failure disposition error=%v", err)
		}
	}
	repo := &notificationStub{}
	ack := &acknowledger{repo: repo}
	if err := HandleDelivery(context.Background(), repo, amqp.Delivery{Acknowledger: ack, Body: body, MessageId: pkg.EncodeID(uuid.New()), Type: event.EventType}); !errors.Is(err, domain.ErrInvalidEvent) || repo.persisted || !ack.nacked || ack.requeue {
		t.Fatal("mismatched event ID was not quarantined")
	}
}

func TestNotificationServiceAuthorizationAndCanonicalIDs(t *testing.T) {
	service := NewNotificationService(&notificationStub{})
	if _, err := service.GetNotifications(context.Background(), 1, 10); !errors.Is(err, pkg.ErrUnauthorized) {
		t.Fatal(err)
	}
	if _, err := service.MarkRead(context.Background(), pkg.EncodeID(uuid.New())); !errors.Is(err, pkg.ErrUnauthorized) {
		t.Fatal(err)
	}
	for _, role := range []string{pkg.Operator, pkg.Supervisor} {
		ctx := pkg.WithActor(context.Background(), pkg.Actor{ID: uuid.New(), Role: role})
		page, err := service.GetNotifications(ctx, 1, 10)
		if err != nil || page.Notifications == nil || page.Meta.TotalData != 0 {
			t.Fatalf("list error=%v", err)
		}
		if _, err := service.GetNotifications(ctx, 1, 0); !errors.Is(err, pkg.ErrInvalidInput) {
			t.Fatal(err)
		}
		if _, err := service.MarkRead(ctx, uuid.NewString()); !errors.Is(err, pkg.ErrInvalidInput) {
			t.Fatal(err)
		}
	}
}
