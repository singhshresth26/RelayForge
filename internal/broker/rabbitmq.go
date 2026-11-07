package broker

import (
	"fmt"
	"log/slog"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
)

type RabbitMQClient struct {
	url     string
	logger  *slog.Logger
	conn    *amqp.Connection
	channel *amqp.Channel
	mu      sync.Mutex
	isClosed bool
}

func NewRabbitMQClient(url string, logger *slog.Logger) (*RabbitMQClient, error) {
	if logger == nil {
		logger = slog.Default()
	}

	client := &RabbitMQClient{
		url:    url,
		logger: logger.With(slog.String("component", "rabbitmq")),
	}

	if err := client.connect(); err != nil {
		return nil, err
	}

	return client, nil
}

func (c *RabbitMQClient) connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	conn, err := amqp.Dial(c.url)
	if err != nil {
		return fmt.Errorf("failed to connect to rabbitmq at %s: %w", c.url, err)
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("failed to open rabbitmq channel: %w", err)
	}

	c.conn = conn
	c.channel = ch
	c.isClosed = false
	c.logger.Info("Connected to RabbitMQ successfully")
	return nil
}

func (c *RabbitMQClient) Channel() *amqp.Channel {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.channel
}

func (c *RabbitMQClient) Connection() *amqp.Connection {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn
}

func (c *RabbitMQClient) IsHealthy() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn != nil && !c.conn.IsClosed() && c.channel != nil && !c.channel.IsClosed()
}

func (c *RabbitMQClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.isClosed = true
	var chErr, connErr error
	if c.channel != nil && !c.channel.IsClosed() {
		chErr = c.channel.Close()
	}
	if c.conn != nil && !c.conn.IsClosed() {
		connErr = c.conn.Close()
	}

	if chErr != nil {
		return chErr
	}
	return connErr
}
