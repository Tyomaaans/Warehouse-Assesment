package notifications

import (
	"context"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"warehouse/internal/infrastructure/rabbitmq"
	"warehouse/internal/notifications/domain"
	"warehouse/pkg"
)

const ExchangeName = "warehouse.notifications"
const QueueName = "warehouse.notifications"
const DeadQueueName = "warehouse.notifications.dead"

type Publisher interface {
	Publish(context.Context, *domain.OutboxEntity) error
}

type Broker struct {
	client  *rabbitmq.RabbitMQ
	returns <-chan amqp.Return
	stop    func() bool
}

func NewBroker(ctx context.Context, dsn string) (*Broker, error) {
	client, err := rabbitmq.NewRabbitMQClient(ctx, dsn)
	if err != nil {
		return nil, err
	}
	b := &Broker{client: client, stop: context.AfterFunc(ctx, client.Close)}
	setup, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	stop := context.AfterFunc(setup, client.Close)
	defer stop()
	if err := b.setup(); err != nil {
		b.Close()
		return nil, fmt.Errorf("notification broker setup failed")
	}
	return b, nil
}

func (b *Broker) Close() { b.stop(); b.client.Close() }

func (b *Broker) setup() error {
	ch := b.client.Channel()
	if err := ch.ExchangeDeclare(ExchangeName, "direct", true, false, false, false, nil); err != nil {
		return err
	}
	if _, err := ch.QueueDeclare(DeadQueueName, true, false, false, false, nil); err != nil {
		return err
	}
	if _, err := ch.QueueDeclare(QueueName, true, false, false, false, amqp.Table{
		"x-dead-letter-exchange": "", "x-dead-letter-routing-key": DeadQueueName,
	}); err != nil {
		return err
	}
	for _, kind := range eventTypes {
		if err := ch.QueueBind(QueueName, kind, ExchangeName, false, nil); err != nil {
			return err
		}
	}
	if err := ch.Confirm(false); err != nil {
		return err
	}
	b.returns = ch.NotifyReturn(make(chan amqp.Return, 1))
	return ch.Qos(1, 0, false)
}

func (b *Broker) Publish(ctx context.Context, event *domain.OutboxEntity) error {
	// amqp091-go checks context before Publish, but its write is not context-aware.
	// Closing the connection on timeout also bounds blocked socket writes.
	stop := context.AfterFunc(ctx, b.client.Close)
	defer stop()
	confirmation, err := b.client.Channel().PublishWithDeferredConfirmWithContext(ctx, ExchangeName, event.EventType, true, false, amqp.Publishing{
		ContentType: "application/json", DeliveryMode: amqp.Persistent, MessageId: pkg.EncodeID(event.ID),
		Type: event.EventType, Timestamp: event.CreatedAt, Body: event.Payload,
	})
	if err != nil {
		return fmt.Errorf("notification publish failed")
	}
	accepted, err := confirmation.WaitContext(ctx)
	if err != nil || !accepted {
		return fmt.Errorf("notification publish unconfirmed")
	}
	// A mandatory unroutable message is returned before its confirmation.
	select {
	case _, ok := <-b.returns:
		if !ok {
			return fmt.Errorf("notification publisher closed")
		}
		return fmt.Errorf("notification event was unroutable")
	default:
		return nil
	}
}

func (b *Broker) Consume(ctx context.Context) (<-chan amqp.Delivery, error) {
	return b.client.Channel().ConsumeWithContext(ctx, QueueName, "", false, false, false, false, nil)
}
