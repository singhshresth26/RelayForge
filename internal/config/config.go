package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	// Server
	Port         string
	Environment  string
	MasterKey    string // 32-byte key for AES-256 endpoint secret encryption

	// Database
	DatabaseURL string

	// Redis
	RedisURL string

	// RabbitMQ
	RabbitMQURL string

	// Outbox Publisher
	OutboxBatchSize     int
	OutboxPollInterval  time.Duration

	// Worker Pool
	WorkerConcurrency   int
	DeliveryTimeout     time.Duration
	AllowPrivateIPs     bool
	EnforceHTTPS        bool
}

func Load() *Config {
	return &Config{
		Port:                getEnv("PORT", "8080"),
		Environment:         getEnv("ENV", "development"),
		MasterKey:           getEnv("RELAYFORGE_MASTER_KEY", "01234567890123456789012345678901"), // Default 32-byte key for dev
		DatabaseURL:         getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/relayforge?sslmode=disable"),
		RedisURL:            getEnv("REDIS_URL", "redis://localhost:6379/0"),
		RabbitMQURL:         getEnv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		OutboxBatchSize:     getEnvInt("OUTBOX_BATCH_SIZE", 100),
		OutboxPollInterval:  getEnvDuration("OUTBOX_POLL_INTERVAL", 50*time.Millisecond),
		WorkerConcurrency:   getEnvInt("WORKER_CONCURRENCY", 20),
		DeliveryTimeout:     getEnvDuration("DELIVERY_TIMEOUT", 10*time.Second),
		AllowPrivateIPs:     getEnvBool("ALLOW_PRIVATE_IPS", true), // Default true in dev, false in prod
		EnforceHTTPS:        getEnvBool("ENFORCE_HTTPS", false),    // Default false in dev
	}
}

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val, ok := os.LookupEnv(key); ok {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}

func getEnvDuration(key string, defaultVal time.Duration) time.Duration {
	if val, ok := os.LookupEnv(key); ok {
		if d, err := time.ParseDuration(val); err == nil {
			return d
		}
	}
	return defaultVal
}

func getEnvBool(key string, defaultVal bool) bool {
	if val, ok := os.LookupEnv(key); ok {
		if b, err := strconv.ParseBool(val); err == nil {
			return b
		}
	}
	return defaultVal
}

