package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all configuration for telemetry streamer
type Config struct {
	// Streamer configuration
	ServiceName string
	ServiceID   string
	HealthPort  int
	LogLevel    string

	// CSV configuration
	CSVFilePath string
	ReadSpeed   int // messages per second

	// Message Broker configuration
	BrokerAddresses   []string
	Topic             string
	ReplicationFactor int

	// Producer configuration
	BatchSize       int
	BatchTimeoutMs  int
	CompressionType string
	Acks            string // "none", "leader", "all"

	// Resilience configuration
	MaxRetries              int
	RetryBackoffMs          int
	CircuitBreakerThreshold int
	HealthCheckInterval     time.Duration

	// Shutdown configuration
	GracefulShutdownTimeout time.Duration
}

// LoadConfig loads configuration from environment variables
func LoadConfig() *Config {
	cfg := &Config{
		ServiceName: getEnv("SERVICE_NAME", "telemetry-streamer"),
		ServiceID:   getEnv("SERVICE_ID", "1"),
		HealthPort:  getEnvInt("HEALTH_PORT", 8080),
		LogLevel:    getEnv("LOG_LEVEL", "info"),

		CSVFilePath: getEnv("CSV_FILE_PATH", "/data/telemetry.csv"),
		ReadSpeed:   getEnvInt("READ_SPEED", 100), // 100 msgs/sec by default

		BrokerAddresses:   parseAddresses(getEnv("BROKER_ADDRESSES", "localhost:9092")),
		Topic:             getEnv("TOPIC", "telemetry"),
		ReplicationFactor: getEnvInt("REPLICATION_FACTOR", 3),

		BatchSize:       getEnvInt("BATCH_SIZE", 50),
		BatchTimeoutMs:  getEnvInt("BATCH_TIMEOUT_MS", 1000),
		CompressionType: getEnv("COMPRESSION_TYPE", "snappy"),
		Acks:            getEnv("ACKS", "all"),

		MaxRetries:              getEnvInt("MAX_RETRIES", 5),
		RetryBackoffMs:          getEnvInt("RETRY_BACKOFF_MS", 100),
		CircuitBreakerThreshold: getEnvInt("CIRCUIT_BREAKER_THRESHOLD", 10),
		HealthCheckInterval:     time.Duration(getEnvInt("HEALTH_CHECK_INTERVAL", 30)) * time.Second,

		GracefulShutdownTimeout: time.Duration(getEnvInt("GRACEFUL_SHUTDOWN_TIMEOUT", 30)) * time.Second,
	}

	return cfg
}

// Helper functions
func getEnv(key, defaultVal string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	valStr := getEnv(key, "")
	if val, err := strconv.Atoi(valStr); err == nil {
		return val
	}
	return defaultVal
}

// parseAddresses parses broker addresses from comma-separated string
// Supports formats: "host1:9092,host2:9092,host3:9092" or "localhost:9092"
func parseAddresses(addressStr string) []string {
	addresses := []string{}
	if addressStr == "" {
		addresses = append(addresses, "localhost:9092")
		return addresses
	}

	parts := strings.Split(addressStr, ",")
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			addresses = append(addresses, trimmed)
		}
	}

	if len(addresses) == 0 {
		addresses = append(addresses, "localhost:9092")
	}

	return addresses
}
