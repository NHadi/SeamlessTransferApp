package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"internal-transfers/account-service/internal/domain"
	"os"

	amqp "github.com/rabbitmq/amqp091-go"
)

// MessageBroker defines the interface for message broker operations
type MessageBroker interface {
	// PublishAccountCreated publishes an account created event
	PublishAccountCreated(ctx context.Context, account *domain.Account) error
	// PublishTransactionSubmitted publishes a transaction submitted event
	PublishTransactionSubmitted(ctx context.Context, event domain.TransactionEvent) error
	// PublishTransactionCompleted publishes a transaction completed event
	PublishTransactionCompleted(ctx context.Context, event domain.TransactionEvent) error
	// PublishTransactionFailed publishes a transaction failed event
	PublishTransactionFailed(ctx context.Context, event domain.TransactionEvent) error
	// SubscribeToTransactionEvents subscribes to transaction events
	SubscribeToTransactionEvents(ctx context.Context, handler func(ctx context.Context, event domain.TransactionEvent) error) error
	// Close closes the message broker connection
	Close() error
}

// RabbitMQBroker implements MessageBroker using RabbitMQ
type RabbitMQBroker struct {
	conn    *amqp.Connection
	channel *amqp.Channel
}

// NewRabbitMQBroker creates a new RabbitMQ broker instance
func NewRabbitMQBroker() (*RabbitMQBroker, error) {
	// Get RabbitMQ connection details from environment
	user := os.Getenv("RABBITMQ_USER")
	password := os.Getenv("RABBITMQ_PASSWORD")
	host := os.Getenv("RABBITMQ_HOST")
	port := os.Getenv("RABBITMQ_PORT")

	// Create connection URL
	url := fmt.Sprintf("amqp://%s:%s@%s:%s/", user, password, host, port)

	// Connect to RabbitMQ
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	// Create channel
	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to open channel: %w", err)
	}

	// Declare exchange
	err = ch.ExchangeDeclare(
		"transactions", // name
		"topic",        // type
		true,           // durable
		false,          // auto-deleted
		false,          // internal
		false,          // no-wait
		nil,            // arguments
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to declare exchange: %w", err)
	}

	return &RabbitMQBroker{
		conn:    conn,
		channel: ch,
	}, nil
}

// PublishAccountCreated publishes an account created event
func (b *RabbitMQBroker) PublishAccountCreated(ctx context.Context, account *domain.Account) error {
	body, err := json.Marshal(account)
	if err != nil {
		return fmt.Errorf("failed to marshal account: %w", err)
	}

	return b.channel.PublishWithContext(ctx,
		"transactions",    // exchange
		"account.created", // routing key
		false,             // mandatory
		false,             // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	)
}

// PublishTransactionSubmitted publishes a transaction submitted event
func (b *RabbitMQBroker) PublishTransactionSubmitted(ctx context.Context, event domain.TransactionEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	return b.channel.PublishWithContext(ctx,
		"transactions",          // exchange
		"transaction.submitted", // routing key
		false,                   // mandatory
		false,                   // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	)
}

// PublishTransactionCompleted publishes a transaction completed event
func (b *RabbitMQBroker) PublishTransactionCompleted(ctx context.Context, event domain.TransactionEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	return b.channel.PublishWithContext(ctx,
		"transactions",                   // exchange
		domain.EventTransactionCompleted, // routing key
		false,                            // mandatory
		false,                            // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	)
}

// PublishTransactionFailed publishes a transaction failed event
func (b *RabbitMQBroker) PublishTransactionFailed(ctx context.Context, event domain.TransactionEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	return b.channel.PublishWithContext(ctx,
		"transactions",                // exchange
		domain.EventTransactionFailed, // routing key
		false,                         // mandatory
		false,                         // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	)
}

// SubscribeToTransactionEvents subscribes to transaction events
func (b *RabbitMQBroker) SubscribeToTransactionEvents(ctx context.Context, handler func(ctx context.Context, event domain.TransactionEvent) error) error {
	// Declare dead letter queue
	dlq, err := b.channel.QueueDeclare(
		"account_transaction_events_dlq", // name
		true,                             // durable
		false,                            // delete when unused
		false,                            // exclusive
		false,                            // no-wait
		nil,                              // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare DLQ: %w", err)
	}

	// Declare main queue with DLQ binding
	q, err := b.channel.QueueDeclare(
		"account_transaction_events", // name
		true,                         // durable
		false,                        // delete when unused
		false,                        // exclusive
		false,                        // no-wait
		amqp.Table{
			"x-dead-letter-exchange":    "", // Use default exchange
			"x-dead-letter-routing-key": dlq.Name,
			"x-message-ttl":             30000, // 30 seconds
		},
	)
	if err != nil {
		return fmt.Errorf("failed to declare queue: %w", err)
	}

	// Bind queue to exchange for transaction submitted events only
	err = b.channel.QueueBind(
		q.Name,                           // queue name
		domain.EventTransactionSubmitted, // routing key
		"transactions",                   // exchange
		false,                            // no-wait
		nil,                              // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to bind queue: %w", err)
	}

	// Consume messages
	msgs, err := b.channel.Consume(
		q.Name, // queue
		"",     // consumer
		false,  // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	if err != nil {
		return fmt.Errorf("failed to register consumer: %w", err)
	}

	// Process messages
	go func() {
		for msg := range msgs {
			var event domain.TransactionEvent
			if err := json.Unmarshal(msg.Body, &event); err != nil {
				fmt.Printf("Failed to unmarshal event: %v\n", err)
				msg.Nack(false, false) // Reject without requeue
				continue
			}

			// Initialize headers if nil
			if msg.Headers == nil {
				msg.Headers = make(amqp.Table)
			}

			// Get retry count from headers
			retryCount := 0
			if retries, ok := msg.Headers["x-retry-count"].(int32); ok {
				retryCount = int(retries)
			}

			// Check if max retries reached
			if retryCount >= 3 {
				fmt.Printf("Max retries reached for transaction %d, moving to DLQ\n", event.TransactionID)
				msg.Nack(false, false) // Move to DLQ
				continue
			}

			if err := handler(ctx, event); err != nil {
				fmt.Printf("Failed to handle event: %v\n", err)

				// Increment retry count
				retryCount++

				// Publish the message again with updated retry count
				headers := amqp.Table{
					"x-retry-count": retryCount,
				}

				if retryCount >= 3 {
					fmt.Printf("Max retries reached for transaction %d, moving to DLQ\n", event.TransactionID)
					msg.Nack(false, false) // Move to DLQ
				} else {
					fmt.Printf("Retrying transaction %d (attempt %d/3)\n", event.TransactionID, retryCount)

					// Publish the message again with updated headers
					err = b.channel.PublishWithContext(ctx,
						"transactions",                   // exchange
						domain.EventTransactionSubmitted, // routing key
						false,                            // mandatory
						false,                            // immediate
						amqp.Publishing{
							ContentType: "application/json",
							Body:        msg.Body,
							Headers:     headers,
						},
					)
					if err != nil {
						fmt.Printf("Failed to republish message: %v\n", err)
					}

					msg.Ack(false) // Acknowledge the original message
				}
				continue
			}

			msg.Ack(false) // Acknowledge successful processing
		}
	}()

	return nil
}

// Close closes the RabbitMQ connection
func (b *RabbitMQBroker) Close() error {
	if err := b.channel.Close(); err != nil {
		return fmt.Errorf("failed to close channel: %w", err)
	}
	if err := b.conn.Close(); err != nil {
		return fmt.Errorf("failed to close connection: %w", err)
	}
	return nil
}
