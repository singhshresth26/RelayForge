package broker

import (
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	// Events Exchange & Queues
	ExchangeEvents           = "relayforge.events"
	QueueEventsFanout         = "relayforge.events.fanout"
	RoutingKeyEventsWildcard  = "event.#"

	// Deliveries Exchange & Queues
	ExchangeDeliveries        = "relayforge.deliveries"
	QueueDeliveriesProcess    = "relayforge.deliveries.process"
	RoutingKeyDeliveryProcess = "delivery.process"

	// Retries Exchange & Queues
	ExchangeRetries           = "relayforge.retries"
	QueueRetriesWait          = "relayforge.retries.wait"
	RoutingKeyDeliveryRetry   = "delivery.retry"

	// Dead Letter Queue
	ExchangeDLQ               = "relayforge.dlq"
	QueueDeliveriesDLQ        = "relayforge.deliveries.dlq"
	RoutingKeyDeliveryDLQ     = "delivery.dlq"
)

// SetupTopology declares all required exchanges, queues, and bindings
func SetupTopology(ch *amqp.Channel) error {
	// 1. Events Topic Exchange
	if err := ch.ExchangeDeclare(
		ExchangeEvents,
		amqp.ExchangeTopic,
		true,  // durable
		false, // auto-deleted
		false, // internal
		false, // no-wait
		nil,   // arguments
	); err != nil {
		return fmt.Errorf("failed to declare exchange %s: %w", ExchangeEvents, err)
	}

	// 2. Events Fan-Out Queue
	qFanout, err := ch.QueueDeclare(
		QueueEventsFanout,
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare queue %s: %w", QueueEventsFanout, err)
	}

	if err := ch.QueueBind(
		qFanout.Name,
		RoutingKeyEventsWildcard,
		ExchangeEvents,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("failed to bind queue %s to %s: %w", qFanout.Name, ExchangeEvents, err)
	}

	// 3. Deliveries Direct Exchange
	if err := ch.ExchangeDeclare(
		ExchangeDeliveries,
		amqp.ExchangeDirect,
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("failed to declare exchange %s: %w", ExchangeDeliveries, err)
	}

	// 4. Deliveries Processing Queue
	qDeliveries, err := ch.QueueDeclare(
		QueueDeliveriesProcess,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to declare queue %s: %w", QueueDeliveriesProcess, err)
	}

	if err := ch.QueueBind(
		qDeliveries.Name,
		RoutingKeyDeliveryProcess,
		ExchangeDeliveries,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("failed to bind queue %s: %w", qDeliveries.Name, err)
	}

	// 5. Retries Exchange & TTL Delayed Queue
	if err := ch.ExchangeDeclare(
		ExchangeRetries,
		amqp.ExchangeDirect,
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("failed to declare exchange %s: %w", ExchangeRetries, err)
	}

	// Queue with DLX pointing back to Deliveries Exchange
	retryArgs := amqp.Table{
		"x-dead-letter-exchange":    ExchangeDeliveries,
		"x-dead-letter-routing-key": RoutingKeyDeliveryProcess,
	}
	qRetries, err := ch.QueueDeclare(
		QueueRetriesWait,
		true,
		false,
		false,
		false,
		retryArgs,
	)
	if err != nil {
		return fmt.Errorf("failed to declare queue %s: %w", QueueRetriesWait, err)
	}

	if err := ch.QueueBind(
		qRetries.Name,
		RoutingKeyDeliveryRetry,
		ExchangeRetries,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("failed to bind retry queue: %w", err)
	}

	// 6. DLQ Exchange & Queue
	if err := ch.ExchangeDeclare(
		ExchangeDLQ,
		amqp.ExchangeDirect,
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("failed to declare exchange %s: %w", ExchangeDLQ, err)
	}

	qDLQ, err := ch.QueueDeclare(
		QueueDeliveriesDLQ,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to declare DLQ %s: %w", QueueDeliveriesDLQ, err)
	}

	if err := ch.QueueBind(
		qDLQ.Name,
		RoutingKeyDeliveryDLQ,
		ExchangeDLQ,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("failed to bind DLQ queue: %w", err)
	}

	return nil
}

