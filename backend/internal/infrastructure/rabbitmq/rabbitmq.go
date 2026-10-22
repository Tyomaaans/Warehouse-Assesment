package rabbitmq

import (
	"context"
	"fmt"
	"net"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog"
)

type RabbitMQ struct {
	conn *amqp.Connection
	ch   *amqp.Channel
}

func NewRabbitMQClient(ctx context.Context, dsn string) (*RabbitMQ, error) {
	dialCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	deadline, _ := dialCtx.Deadline()
	var socket net.Conn
	conn, err := amqp.DialConfig(dsn, amqp.Config{
		Heartbeat: 10 * time.Second,
		Dial: func(network, address string) (net.Conn, error) {
			var err error
			socket, err = (&net.Dialer{}).DialContext(dialCtx, network, address)
			if err != nil {
				return nil, err
			}
			if err := socket.SetDeadline(deadline); err != nil {
				_ = socket.Close()
				return nil, err
			}
			return socket, nil
		},
	})
	if err != nil {
		if socket != nil {
			_ = socket.Close()
		}
		return nil, fmt.Errorf("RabbitMQ unavailable")
	}
	if err := socket.SetDeadline(deadline); err != nil {
		_ = conn.CloseDeadline(time.Now().Add(time.Second))
		return nil, fmt.Errorf("RabbitMQ channel unavailable")
	}
	ch, err := conn.Channel()
	if err != nil {
		_ = conn.CloseDeadline(time.Now().Add(time.Second))
		return nil, fmt.Errorf("RabbitMQ channel unavailable")
	}
	if err := socket.SetDeadline(time.Time{}); err != nil {
		_ = conn.CloseDeadline(time.Now().Add(time.Second))
		return nil, fmt.Errorf("RabbitMQ connection unavailable")
	}
	return &RabbitMQ{conn: conn, ch: ch}, nil
}

func (r *RabbitMQ) Close() {
	_ = r.conn.CloseDeadline(time.Now().Add(time.Second))
}

func MaintainConnection(ctx context.Context, dsn string, logger zerolog.Logger) {
	for ctx.Err() == nil {
		client, err := NewRabbitMQClient(ctx, dsn)
		if err != nil {
			logger.Warn().Str("event", "rabbitmq_unavailable").Msg("RabbitMQ connection will be retried")
		} else {
			logger.Info().Str("event", "rabbitmq_connected").Msg("RabbitMQ connection and channel established")
			select {
			case <-ctx.Done():
			case <-client.conn.NotifyClose(make(chan *amqp.Error, 1)):
			}
			client.Close()
		}
		timer := time.NewTimer(5 * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}
