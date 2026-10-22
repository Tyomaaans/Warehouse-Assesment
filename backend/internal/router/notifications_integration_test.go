package router_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog"

	"warehouse/internal/infrastructure/rabbitmq"
	"warehouse/internal/notifications"
	"warehouse/internal/notifications/domain"
	"warehouse/pkg"
)

func eventually(t *testing.T, check func() bool) {
	t.Helper()
	deadline := time.Now().Add(12 * time.Second)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("notification condition did not become true")
}

func nextDelivery(t *testing.T, deliveries <-chan amqp.Delivery) amqp.Delivery {
	t.Helper()
	select {
	case delivery, ok := <-deliveries:
		if !ok {
			t.Fatal("consumer disconnected")
		}
		return delivery
	case <-time.After(5 * time.Second):
		t.Fatal("notification delivery timed out")
	}
	return amqp.Delivery{}
}

func TestNotificationDeliveryIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_RABBITMQ_URL")
	if dsn == "" {
		t.Skip("set TEST_RABBITMQ_URL to a dedicated disposable broker/vhost for RabbitMQ integration")
	}
	f := newWorkflowFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	broker, err := notifications.NewBroker(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer broker.Close()
	consumer, err := notifications.NewBroker(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer consumer.Close()
	deliveries, err := consumer.Consume(ctx)
	if err != nil {
		t.Fatal(err)
	}
	admin, err := rabbitmq.NewRabbitMQClient(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	repo := notifications.NewOutboxRepository(f.db)
	notificationRepo := notifications.NewNotificationRepository(f.db)
	doc := newReceiving(t, f, 10)
	transaction := submitTransaction(t, f, doc, 5)
	var source domain.OutboxEntity
	if err := f.db.Table("notification_outbox").Take(&source).Error; err != nil {
		t.Fatal(err)
	}

	t.Run("short_claim_concurrency_crash_recovery_and_fencing", func(t *testing.T) {
		claimed := make(chan *domain.OutboxEntity, 8)
		errorsCh := make(chan error, 8)
		var wg sync.WaitGroup
		for range 8 {
			wg.Go(func() { row, err := repo.Claim(ctx, 30*time.Second); claimed <- row; errorsCh <- err })
		}
		wg.Wait()
		close(claimed)
		close(errorsCh)
		for err := range errorsCh {
			if err != nil {
				t.Fatal(err)
			}
		}
		var owner *domain.OutboxEntity
		for row := range claimed {
			if row != nil {
				if owner != nil {
					t.Fatal("multiple claim owners")
				}
				owner = row
			}
		}
		if owner == nil || owner.ID != source.ID || owner.Attempts != 1 {
			t.Fatal("pending event not claimed once")
		}
		// The claim statement has committed. A separate connection can obtain this row lock immediately.
		if err := f.db.Exec("SELECT id FROM notification_outbox WHERE id=? FOR UPDATE NOWAIT", source.ID).Error; err != nil {
			t.Fatal("broker work would hold a SQL row lock", err)
		}
		if err := f.db.Exec("UPDATE notification_outbox SET lease_until=clock_timestamp()-interval '1 second' WHERE id=?", source.ID).Error; err != nil {
			t.Fatal(err)
		}
		recovered, err := repo.Claim(ctx, 30*time.Second)
		if err != nil || recovered == nil || recovered.LeaseToken == owner.LeaseToken || recovered.Attempts != 2 {
			t.Fatalf("restart recovery=%v", err)
		}
		if err := repo.Published(ctx, owner); !errors.Is(err, pkg.ErrConflict) {
			t.Fatal("stale owner marked a new lease", err)
		}
		if err := repo.Retry(ctx, recovered, 0); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("mandatory_return_not_marked_then_confirmed_retry", func(t *testing.T) {
		ch := admin.Channel()
		if err := ch.QueueUnbind(notifications.QueueName, source.EventType, notifications.ExchangeName, nil); err != nil {
			t.Fatal(err)
		}
		worked, err := notifications.PublishPending(ctx, repo, broker)
		if !worked || err == nil {
			t.Fatal("unroutable broker confirmation was accepted")
		}
		var row struct {
			PublishedAt   *time.Time
			LeaseToken    *uuid.UUID
			NextAttemptAt time.Time
			Attempts      int64
		}
		if err := f.db.Table("notification_outbox").Where("id=?", source.ID).Scan(&row).Error; err != nil {
			t.Fatal(err)
		}
		if row.PublishedAt != nil || row.LeaseToken != nil || !row.NextAttemptAt.After(time.Now()) || row.Attempts != 3 {
			t.Fatal("failed publish lost retry state")
		}
		if err := ch.QueueBind(notifications.QueueName, source.EventType, notifications.ExchangeName, false, nil); err != nil {
			t.Fatal(err)
		}
		if err := f.db.Exec("UPDATE notification_outbox SET next_attempt_at=clock_timestamp() WHERE id=?", source.ID).Error; err != nil {
			t.Fatal(err)
		}
		if worked, err := notifications.PublishPending(ctx, repo, broker); err != nil || !worked {
			t.Fatal("confirmed retry failed", err)
		}
		var count int64
		if err := f.db.Table("notification_outbox").Where("id=? AND published_at IS NOT NULL", source.ID).Count(&count).Error; err != nil || count != 1 {
			t.Fatal("confirmed event not marked")
		}
	})
	t.Run("database_failure_nack_redelivery_atomic_fanout", func(t *testing.T) {
		// Fail the second recipient so the first insert must roll back too.
		if err := f.db.Exec(`CREATE FUNCTION task2e_fail_notification() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN IF EXISTS(SELECT 1 FROM notifications WHERE source_event_id=NEW.source_event_id) THEN
RAISE EXCEPTION 'injected database failure' USING ERRCODE='P0001'; END IF; RETURN NEW; END; $$;
CREATE TRIGGER task2e_fail_notification BEFORE INSERT ON notifications FOR EACH ROW EXECUTE FUNCTION task2e_fail_notification()`).Error; err != nil {
			t.Fatal(err)
		}
		first := nextDelivery(t, deliveries)
		if first.MessageId != pkg.EncodeID(source.ID) || first.DeliveryMode != amqp.Persistent {
			t.Fatal("stable event identity/persistence lost")
		}
		if err := notifications.HandleDelivery(ctx, notificationRepo, first); err == nil {
			t.Fatal("database failure acknowledged")
		}
		if tableCount(t, f.db, "notifications") != 0 {
			t.Fatal("partial fanout survived database failure")
		}
		if err := f.db.Exec("DROP TRIGGER task2e_fail_notification ON notifications; DROP FUNCTION task2e_fail_notification()").Error; err != nil {
			t.Fatal(err)
		}
		redelivery := nextDelivery(t, deliveries)
		if !redelivery.Redelivered || redelivery.MessageId != first.MessageId {
			t.Fatal("failed delivery did not return with same ID")
		}
		if err := notifications.HandleDelivery(ctx, notificationRepo, redelivery); err != nil {
			t.Fatal(err)
		}
		if tableCount(t, f.db, "notifications") != 2 {
			t.Fatal("recipient fanout missing")
		}
	})
	t.Run("recipient_owned_API_pagination_and_idempotent_read", func(t *testing.T) {
		for _, role := range []string{pkg.Operator, pkg.Supervisor} {
			response := f.request("GET", "/api/v1/notifications?limit=1", nil, role)
			if response.Code != 200 {
				t.Fatalf("notification list=%d", response.Code)
			}
			page := responseData[notifications.PaginatedNotificationResponse](t, response)
			if len(page.Notifications) != 1 || page.Meta.TotalData != 1 || page.Notifications[0].RecipientID != pkg.EncodeID(f.users[role].ID) || page.Notifications[0].ReadAt != nil {
				t.Fatal("recipient isolation/list shape failed")
			}
			n := page.Notifications[0]
			otherRole := pkg.Supervisor
			if role == otherRole {
				otherRole = pkg.Operator
			}
			workflowError(t, f.request("PATCH", "/api/v1/notifications/"+n.ID+"/read", nil, otherRole), 404, "not_found")
			first := f.request("PATCH", "/api/v1/notifications/"+n.ID+"/read", nil, role)
			if first.Code != 200 {
				t.Fatal("own read failed")
			}
			read := responseData[notifications.NotificationResponse](t, first)
			again := responseData[notifications.NotificationResponse](t, f.request("PATCH", "/api/v1/notifications/"+n.ID+"/read", nil, role))
			if read.ReadAt == nil || again.ReadAt == nil || !read.ReadAt.Equal(*again.ReadAt) {
				t.Fatal("repeated read changed first read time")
			}
			page2 := responseData[notifications.PaginatedNotificationResponse](t, f.request("GET", "/api/v1/notifications?page=2&limit=1", nil, role))
			if len(page2.Notifications) != 0 || page2.Meta.TotalData != 1 {
				t.Fatal("out-of-range page lost totals")
			}
		}
		workflowError(t, f.request("GET", "/api/v1/notifications", nil, ""), 401, "unauthorized")
		workflowError(t, f.request("PATCH", "/api/v1/notifications/"+pkg.EncodeID(uuid.New())+"/read", nil, ""), 401, "unauthorized")
		for _, path := range []string{"/api/v1/notifications?limit=0", "/api/v1/notifications?page=10001", "/api/v1/notifications?limit=101"} {
			workflowError(t, f.request("GET", path, nil, pkg.Operator), 400, "invalid_input")
		}
		workflowError(t, f.request("PATCH", "/api/v1/notifications/"+uuid.NewString()+"/read", nil, pkg.Operator), 400, "invalid_input")
		workflowError(t, f.request("PATCH", "/api/v1/notifications/"+pkg.EncodeID(uuid.New())+"/read", nil, pkg.Operator), 404, "not_found")
	})
	t.Run("duplicate_delivery_preserves_notification_identity_and_read_state", func(t *testing.T) {
		for range 3 {
			if err := broker.Publish(ctx, &source); err != nil {
				t.Fatal(err)
			}
			if err := notifications.HandleDelivery(ctx, notificationRepo, nextDelivery(t, deliveries)); err != nil {
				t.Fatal(err)
			}
		}
		var unread int64
		if err := f.db.Table("notifications").Where("read_at IS NULL").Count(&unread).Error; err != nil || unread != 0 || tableCount(t, f.db, "notifications") != 2 {
			t.Fatal("duplicate created notifications or reset read state")
		}
	})
	t.Run("consumer_crash_after_commit_before_ack_recovers", func(t *testing.T) {
		if err := broker.Publish(ctx, &source); err != nil {
			t.Fatal(err)
		}
		delivery := nextDelivery(t, deliveries)
		var event domain.Event
		if err := json.Unmarshal(delivery.Body, &event); err != nil {
			t.Fatal(err)
		}
		if err := notificationRepo.PersistEvent(ctx, event, delivery.Body); err != nil {
			t.Fatal(err)
		}
		// Deliberately close with an unacknowledged committed event.
		consumer.Close()
		restarted, err := notifications.NewBroker(ctx, dsn)
		if err != nil {
			t.Fatal(err)
		}
		defer restarted.Close()
		stream, err := restarted.Consume(ctx)
		if err != nil {
			t.Fatal(err)
		}
		redelivery := nextDelivery(t, stream)
		if !redelivery.Redelivered {
			t.Fatal("unacked event was lost on restart")
		}
		if err := notifications.HandleDelivery(ctx, notificationRepo, redelivery); err != nil {
			t.Fatal(err)
		}
		if tableCount(t, f.db, "notifications") != 2 {
			t.Fatal("restart duplicated logical delivery")
		}
	})
	t.Run("concurrent_workers_approval_unchanged_and_graceful_shutdown", func(t *testing.T) {
		approveTransaction(t, f, transaction)
		for range 4 {
			submitTransaction(t, f, newReceiving(t, f, 10), 1)
		}
		workers, cancelWorkers := context.WithCancel(ctx)
		var wg sync.WaitGroup
		for range 2 {
			wg.Go(func() { notifications.RunWorker(workers, repo, notificationRepo, dsn, zerolog.Nop()) })
		}
		defer func() { cancelWorkers(); wg.Wait() }()
		eventually(t, func() bool {
			var pending, count int64
			if f.db.Table("notification_outbox").Where("published_at IS NULL").Count(&pending).Error != nil {
				return false
			}
			if f.db.Table("notifications").Count(&count).Error != nil {
				return false
			}
			return pending == 0 && count == 11 // submission fanout 2 + approval 1 + four submissions fanout 8
		})
		if stockFor(t, f, doc.Items[0]) != 5 || len(movementsFor(t, f, doc.Items[0].ProductID)) != 1 {
			t.Fatal("notifications altered inventory")
		}
		var duplicates int64
		if err := f.db.Raw(`SELECT count(*) FROM (SELECT source_event_id,recipient_id FROM notifications GROUP BY source_event_id,recipient_id HAVING count(*)>1) d`).Scan(&duplicates).Error; err != nil || duplicates != 0 {
			t.Fatal("duplicate logical deliveries")
		}
		for _, role := range []string{pkg.Operator, pkg.Supervisor} {
			page := responseData[notifications.PaginatedNotificationResponse](t, f.request("GET", "/api/v1/notifications?limit=100", nil, role))
			want := int64(5)
			if role == pkg.Operator {
				want = 6
			}
			if page.Meta.TotalData != want {
				t.Fatalf("%s notifications=%d want=%d", role, page.Meta.TotalData, want)
			}
			for _, n := range page.Notifications {
				if n.RecipientID != pkg.EncodeID(f.users[role].ID) {
					t.Fatal("recipient leaked")
				}
			}
		}
		cancelWorkers()
		done := make(chan struct{})
		go func() { wg.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("worker shutdown hung")
		}
	})
}
