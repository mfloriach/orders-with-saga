package app

import (
	"context"
	"fmt"

	"paymentgateway/internal/config"
	"paymentgateway/internal/webhooks"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Webhook topology aliases. Declared here for the broker; the worker
// consumes via the webhooks package constants.
const (
	WebhookExchange = webhooks.Exchange
	WebhookDLX      = webhooks.DLX
	DeliverQueue    = webhooks.DeliverQueue
	DLQueue         = webhooks.DLQueue
	WebhookRoute    = webhooks.RouteKey
)

// Broker is an AMQP connection with the webhook topology declared.
// Channels are opened per operation: publish and consume each use their
// own channel because amqp091 channels are not safe for concurrent use.
type Broker struct {
	conn *amqp.Connection
}

// OpenBroker dials the broker, declares the webhook topology, and verifies
// it. On any failure the connection is closed: boot stays fail-closed.
func OpenBroker(cfg config.Config) (*Broker, error) {
	conn, err := amqp.Dial(cfg.AMQPURL)
	if err != nil {
		return nil, fmt.Errorf("dial rabbitmq: %w", err)
	}
	b := &Broker{conn: conn}
	if err := b.declare(); err != nil {
		conn.Close()
		return nil, err
	}
	return b, nil
}

func (b *Broker) declare() error {
	ch, err := b.conn.Channel()
	if err != nil {
		return fmt.Errorf("open rabbitmq channel: %w", err)
	}
	defer ch.Close()

	if err := ch.ExchangeDeclare(WebhookExchange, "topic", true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare exchange %s: %w", WebhookExchange, err)
	}
	if err := ch.ExchangeDeclare(WebhookDLX, "topic", true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare exchange %s: %w", WebhookDLX, err)
	}
	if _, err := ch.QueueDeclare(DeliverQueue, true, false, false, false, amqp.Table{
		"x-dead-letter-exchange": WebhookDLX,
	}); err != nil {
		return fmt.Errorf("declare queue %s: %w", DeliverQueue, err)
	}
	if _, err := ch.QueueDeclare(DLQueue, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare queue %s: %w", DLQueue, err)
	}
	if err := ch.QueueBind(DeliverQueue, "webhook.*", WebhookExchange, false, nil); err != nil {
		return fmt.Errorf("bind queue %s: %w", DeliverQueue, err)
	}
	if err := ch.QueueBind(DLQueue, "webhook.*", WebhookDLX, false, nil); err != nil {
		return fmt.Errorf("bind queue %s: %w", DLQueue, err)
	}
	return nil
}

// Publish sends one persistent message on its own channel.
func (b *Broker) Publish(ctx context.Context, exchange, routingKey string, body []byte, headers amqp.Table) error {
	ch, err := b.conn.Channel()
	if err != nil {
		return fmt.Errorf("open rabbitmq channel: %w", err)
	}
	defer ch.Close()

	return ch.PublishWithContext(ctx, exchange, routingKey, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Headers:      headers,
		Body:         body,
	})
}

// Channel opens a dedicated channel for a consumer.
func (b *Broker) Channel() (*amqp.Channel, error) {
	ch, err := b.conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("open rabbitmq channel: %w", err)
	}
	return ch, nil
}

// Ping verifies the connection with a throwaway channel.
func (b *Broker) Ping(ctx context.Context) error {
	if b == nil || b.conn == nil || b.conn.IsClosed() {
		return fmt.Errorf("ping rabbitmq: not connected")
	}
	done := make(chan error, 1)
	go func() {
		ch, err := b.conn.Channel()
		if err != nil {
			done <- err
			return
		}
		done <- ch.Close()
	}()
	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("ping rabbitmq: %w", err)
		}
		return nil
	case <-ctx.Done():
		return fmt.Errorf("ping rabbitmq: %w", ctx.Err())
	}
}

// Close shuts the connection. Safe to call on a nil receiver.
func (b *Broker) Close() error {
	if b == nil || b.conn == nil {
		return nil
	}
	return b.conn.Close()
}
