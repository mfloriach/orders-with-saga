package config

import (
	"context"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

// BrokerExchange is the topic exchange capturing payment lifecycle events
// that async consumers bind to. The relay publishes generic captured-payment
// events here; delivery consumers subscribe in later slices.
const BrokerExchange = "payments.events"

// Broker is an AMQP connection with the relay exchange declared. Channels
// are opened per operation because amqp091 channels are not safe for
// concurrent use.
type Broker struct {
	conn *amqp.Connection
}

// OpenBroker dials the broker, declares the events exchange, and verifies
// the connection. On any failure the connection is closed: boot stays
// fail-closed.
func OpenBroker(cfg Config) (*Broker, error) {
	conn, err := amqp.Dial(cfg.RabbitMQ.URL)
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

	if err := ch.ExchangeDeclare(BrokerExchange, "topic", true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare exchange %s: %w", BrokerExchange, err)
	}
	return nil
}

// Publish sends one persistent message on its own channel.
func (b *Broker) Publish(ctx context.Context, routingKey string, body []byte) error {
	ch, err := b.conn.Channel()
	if err != nil {
		return fmt.Errorf("open rabbitmq channel: %w", err)
	}
	defer ch.Close()

	return ch.PublishWithContext(ctx, BrokerExchange, routingKey, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Body:         body,
	})
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

// Close shuts the connection down. Safe to call on a nil receiver.
func (b *Broker) Close() error {
	if b == nil || b.conn == nil {
		return nil
	}
	return b.conn.Close()
}
