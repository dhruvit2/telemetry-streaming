package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"telemetry-streaming/pkg/config"
	"telemetry-streaming/pkg/csv"
	"telemetry-streaming/pkg/producer"
)

func main() {
	// Load configuration
	cfg := config.LoadConfig()

	// Setup logger
	logger := setupLogger(cfg.LogLevel)
	defer logger.Sync()

	logger.Info("Starting telemetry streaming service",
		zap.String("service_name", cfg.ServiceName),
		zap.String("service_id", cfg.ServiceID),
		zap.String("csv_file", cfg.CSVFilePath),
		zap.Int("read_speed", cfg.ReadSpeed),
		zap.Int("broker_count", len(cfg.BrokerAddresses)),
		zap.String("topic", cfg.Topic),
		zap.Int("replication_factor", cfg.ReplicationFactor),
		zap.Int("batch_size", cfg.BatchSize),
		zap.Int("max_retries", cfg.MaxRetries))

	// Verify CSV file exists
	if _, err := os.Stat(cfg.CSVFilePath); os.IsNotExist(err) {
		logger.Fatal("CSV file not found", zap.String("path", cfg.CSVFilePath))
	}

	// Setup CSV reader
	csvReader := csv.NewReader(cfg.CSVFilePath, cfg.ReadSpeed, logger)

	// Setup producer (for message broker)
	producerConfig := &producer.ProducerConfig{
		BrokerAddresses:         cfg.BrokerAddresses,
		Topic:                   cfg.Topic,
		BatchSize:               cfg.BatchSize,
		BatchTimeoutMs:          cfg.BatchTimeoutMs,
		CompressionType:         cfg.CompressionType,
		Acks:                    cfg.Acks,
		ReplicationFactor:       cfg.ReplicationFactor,
		MaxRetries:              cfg.MaxRetries,
		RetryBackoffMs:          cfg.RetryBackoffMs,
		CircuitBreakerThreshold: cfg.CircuitBreakerThreshold,
	}

	telemetryProducer, err := producer.NewTelemetryProducer(producerConfig, logger)
	if err != nil {
		logger.Fatal("failed to create producer", zap.Error(err))
	}

	// Start producer batch flushing
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	telemetryProducer.Start(ctx)

	// Start health check endpoint
	go startHealthCheckServer(cfg.HealthPort, logger, telemetryProducer)

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Create CSV record stream
	stopChan := make(chan struct{})
	recordsChan := csvReader.StreamRecords(ctx)

	// Process records
	recordCount := 0
	errorCount := 0
	startTime := time.Now()
	lastReportTime := startTime

	logger.Info("starting to stream records from CSV")

	for {
		select {
		case record, ok := <-recordsChan:
			if !ok {
				logger.Info("CSV reading completed")
				goto cleanup
			}

			// Convert record to map and send
			recordMap := record.ToMap()
			recordMap["_timestamp"] = time.Now().Format(time.RFC3339Nano)
			recordMap["_line_number"] = fmt.Sprintf("%d", record.LineNum)
			recordMap["_service_id"] = cfg.ServiceID

			// Send to message broker
			if err := telemetryProducer.SendMessage(recordMap); err != nil {
				logger.Error("failed to queue message to broker", zap.Error(err))
				errorCount++
			} else {
				recordCount++
			}

			// Report progress every 10 seconds
			if time.Since(lastReportTime) > 10*time.Second {
				elapsed := time.Since(startTime)
				throughput := float64(recordCount) / elapsed.Seconds()
				metrics := telemetryProducer.GetMetrics()

				logger.Info("progress report",
					zap.Int("total_records_queued", recordCount),
					zap.Int("queue_errors", errorCount),
					zap.Float64("throughput_rps", throughput),
					zap.Duration("elapsed", elapsed),
					zap.Int64("messages_sent", metrics.MessagesSent),
					zap.Int64("messages_failed", metrics.MessagesFailed),
					zap.Int64("batches_sent", metrics.BatchesSent),
					zap.Int64("total_bytes_sent", metrics.BytesSent))

				lastReportTime = time.Now()
			}

		case <-sigChan:
			logger.Info("received shutdown signal")
			goto cleanup
		}
	}

cleanup:
	logger.Info("cleaning up and shutting down gracefully...")

	// Signal CSV reader to stop
	close(stopChan)
	_ = csvReader.Close()

	// Create graceful shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.GracefulShutdownTimeout)
	defer shutdownCancel()

	// Flush remaining messages
	if err := telemetryProducer.Flush(shutdownCtx); err != nil {
		logger.Error("final flush failed", zap.Error(err))
	}

	// Close producer
	if err := telemetryProducer.Close(); err != nil {
		logger.Error("producer close failed", zap.Error(err))
	}

	// Final metrics
	metrics := telemetryProducer.GetMetrics()
	totalTime := time.Since(startTime)
	var throughput float64
	if totalTime.Seconds() > 0 {
		throughput = float64(recordCount) / totalTime.Seconds()
	}

	logger.Info("telemetry streaming completed",
		zap.Int("total_records_read", recordCount),
		zap.Int("total_queue_errors", errorCount),
		zap.Duration("total_time", totalTime),
		zap.Float64("average_throughput_rps", throughput),
		zap.Int64("messages_sent", metrics.MessagesSent),
		zap.Int64("messages_failed", metrics.MessagesFailed),
		zap.Int64("batches_sent", metrics.BatchesSent),
		zap.Int64("total_bytes_sent", metrics.BytesSent))

	cancel()
}

// setupLogger creates and configures logger
func setupLogger(level string) *zap.Logger {
	var logLevel zapcore.Level
	switch level {
	case "debug":
		logLevel = zapcore.DebugLevel
	case "info":
		logLevel = zapcore.InfoLevel
	case "warn":
		logLevel = zapcore.WarnLevel
	case "error":
		logLevel = zapcore.ErrorLevel
	default:
		logLevel = zapcore.InfoLevel
	}

	config := zap.Config{
		Level:       zap.NewAtomicLevelAt(logLevel),
		Development: false,
		Encoding:    "json",
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "timestamp",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			MessageKey:     "message",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.RFC3339NanoTimeEncoder,
			EncodeDuration: zapcore.StringDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	logger, err := config.Build()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create logger: %v\n", err)
		os.Exit(1)
	}

	return logger
}

// startHealthCheckServer starts the health check HTTP endpoint
func startHealthCheckServer(port int, logger *zap.Logger, prod *producer.TelemetryProducer) {
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		metrics := prod.GetMetrics()
		status := "healthy"

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status": "%s", "timestamp": "%s", "messages_sent": %d, "messages_failed": %d}`+"\n",
			status, time.Now().Format(time.RFC3339Nano), metrics.MessagesSent, metrics.MessagesFailed)
	})

	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		metrics := prod.GetMetrics()
		w.Header().Set("Content-Type", "application/json")

		fmt.Fprintf(w, `{"messages_sent":%d,"messages_failed":%d,"batches_sent":%d,"bytes_sent":%d}`+"\n",
			metrics.MessagesSent, metrics.MessagesFailed, metrics.BatchesSent, metrics.BytesSent)
	})

	http.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, "{\"ready\": true}\n")
	})

	addr := fmt.Sprintf(":%d", port)
	logger.Info("starting health check server", zap.String("address", addr))

	if err := http.ListenAndServe(addr, nil); err != nil && err != http.ErrServerClosed {
		logger.Error("health check server error", zap.Error(err))
	}
}
