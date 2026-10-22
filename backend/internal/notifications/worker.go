package notifications

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog"

	"warehouse/internal/notifications/domain"
)

const publishTimeout = 5 * time.Second
const leaseDuration = 30 * time.Second

// PublishPending claims only one event so no batch waits behind a broker call.
func PublishPending(ctx context.Context, repo domain.OutboxRepository, publisher Publisher) (bool, error) {
	query, cancel := context.WithTimeout(ctx, 3*time.Second)
	event, err := repo.Claim(query, leaseDuration)
	cancel()
	if err != nil || event == nil {
		return false, err
	}
	publishing, cancel := context.WithTimeout(ctx, publishTimeout)
	err = publisher.Publish(publishing, event)
	cancel()
	if err != nil {
		// Best-effort release on shutdown; a crash or DB outage recovers by lease expiry.
		retry, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		_ = repo.Retry(retry, event, time.Second*time.Duration(1<<min(event.Attempts-1, 5)))
		return true, err
	}
	query, cancel = context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	// A lost mark after confirmation deliberately permits redelivery of the same ID.
	return true, repo.Published(query, event)
}

func HandleDelivery(ctx context.Context, repo domain.NotificationRepository, delivery amqp.Delivery) error {
	event, err := decodeEvent(delivery.Body, delivery.MessageId)
	if err == nil && delivery.Type != event.EventType {
		err = domain.ErrInvalidEvent
	}
	if err == nil {
		persist, cancel := context.WithTimeout(ctx, 5*time.Second)
		err = repo.PersistEvent(persist, event, delivery.Body)
		cancel()
	}
	if errors.Is(err, domain.ErrInvalidEvent) {
		// Invalid/untrusted payloads are retained in a durable dead-letter queue.
		if nack := delivery.Nack(false, false); nack != nil {
			return nack
		}
		return domain.ErrInvalidEvent
	}
	if err != nil {
		_ = delivery.Nack(false, true)
		return err
	}
	return delivery.Ack(false)
}

func RunWorker(ctx context.Context, outbox domain.OutboxRepository, notifications domain.NotificationRepository, dsn string, logger zerolog.Logger) {
	var workers sync.WaitGroup
	workers.Go(func() { runPublisher(ctx, outbox, dsn, logger) })
	workers.Go(func() { runConsumer(ctx, notifications, dsn, logger) })
	workers.Wait()
	logger.Info().Str("event", "notification_worker_stopped").Msg("notification workers stopped")
}

func runPublisher(ctx context.Context, repo domain.OutboxRepository, dsn string, logger zerolog.Logger) {
	for ctx.Err() == nil {
		broker, err := NewBroker(ctx, dsn)
		if err == nil {
			logger.Info().Str("event", "notification_publisher_connected").Msg("outbox publisher connected")
			for ctx.Err() == nil {
				var worked bool
				worked, err = PublishPending(ctx, repo, broker)
				if err != nil {
					break
				}
				if !worked && !wait(ctx, 250*time.Millisecond) {
					break
				}
			}
			broker.Close()
		}
		if ctx.Err() != nil {
			return
		}
		logger.Warn().Str("event", "notification_publisher_retry").Msg("outbox retained; publisher will retry")
		if !wait(ctx, time.Second) {
			return
		}
	}
}

func runConsumer(ctx context.Context, repo domain.NotificationRepository, dsn string, logger zerolog.Logger) {
	for ctx.Err() == nil {
		broker, err := NewBroker(ctx, dsn)
		if err == nil {
			err = consume(ctx, repo, broker, logger)
			broker.Close()
		}
		if ctx.Err() != nil {
			return
		}
		logger.Warn().Str("event", "notification_consumer_retry").Msg("unacknowledged deliveries retained; consumer will retry")
		if !wait(ctx, time.Second) {
			return
		}
	}
}

func consume(ctx context.Context, repo domain.NotificationRepository, broker *Broker, logger zerolog.Logger) error {
	deliveries, err := broker.Consume(ctx)
	if err != nil {
		return err
	}
	logger.Info().Str("event", "notification_consumer_connected").Msg("notification consumer connected")
	for {
		select {
		case <-ctx.Done():
			return nil
		case delivery, ok := <-deliveries:
			if !ok {
				return fmt.Errorf("notification consumer disconnected")
			}
			err := HandleDelivery(ctx, repo, delivery)
			if errors.Is(err, domain.ErrInvalidEvent) {
				logger.Warn().Str("event", "notification_invalid_event").Msg("invalid delivery retained in dead-letter queue")
				continue
			}
			if err != nil {
				return err
			}
			logger.Info().Str("event", "notification_persisted").Str("source_event_id", delivery.MessageId).Bool("redelivered", delivery.Redelivered).Msg("notification persisted or duplicate confirmed")
		}
	}
}

func wait(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
