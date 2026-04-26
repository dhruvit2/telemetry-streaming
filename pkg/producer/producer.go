package producer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	pb "github.com/dhruvit2/messagebroker/pkg/pb"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// MessageRecord represents a message to send to broker
type MessageRecord struct {
	Topic    string
	Key      []byte
	Value    []byte
	Metadata map[string]string
}

// ProducerConfig holds producer configuration
type ProducerConfig struct {
	BrokerAddresses         []string
	Topic                   string
	BatchSize               int
	BatchTimeoutMs          int
	CompressionType         string
	Acks                    string
	ReplicationFactor       int
	MaxRetries              int
	RetryBackoffMs          int
	CircuitBreakerThreshold int
}

// CircuitBreaker implements circuit breaker pattern for fault tolerance
type CircuitBreaker struct {
	failureThreshold int32
	failures         int32
	successCount     int32
	lastFailureTime  time.Time
	state            string // "closed", "open", "half-open"
	mu               sync.RWMutex
	resetTimeout     time.Duration
}

// NewCircuitBreaker creates new circuit breaker
func NewCircuitBreaker(failureThreshold int32, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		failureThreshold: failureThreshold,
		failures:         0,
		state:            "closed",
		resetTimeout:     resetTimeout,
	}
}

// RecordSuccess records successful operation
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == "half-open" {
		atomic.StoreInt32(&cb.successCount, atomic.LoadInt32(&cb.successCount)+1)
		if atomic.LoadInt32(&cb.successCount) >= 5 {
			cb.state = "closed"
			atomic.StoreInt32(&cb.failures, 0)
			atomic.StoreInt32(&cb.successCount, 0)
		}
	}
}

// RecordFailure records failed operation
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.lastFailureTime = time.Now()
	newFailures := atomic.AddInt32(&cb.failures, 1)

	if newFailures >= cb.failureThreshold && cb.state == "closed" {
		cb.state = "open"
	}
}

// IsOpen checks if circuit is open
func (cb *CircuitBreaker) IsOpen() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	if cb.state == "open" {
		if time.Since(cb.lastFailureTime) > cb.resetTimeout {
			cb.mu.RUnlock()
			cb.mu.Lock()
			cb.state = "half-open"
			atomic.StoreInt32(&cb.successCount, 0)
			cb.mu.Unlock()
			cb.mu.RLock()
			return false
		}
		return true
	}

	return false
}

// GetState returns current circuit breaker state
func (cb *CircuitBreaker) GetState() string {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// TelemetryProducer handles sending telemetry data to message broker
type TelemetryProducer struct {
	config         *ProducerConfig
	logger         *zap.Logger
	circuitBreaker *CircuitBreaker

	// Metrics
	metrics *ProducerMetrics

	// State
	batch      [][]byte
	batchBytes int64
	batchMu    sync.Mutex
	done       chan struct{}
	wg         sync.WaitGroup

	// gRPC client
	grpcConn   *grpc.ClientConn
	grpcClient pb.MessageBrokerClient

	// Topic management
	topicCreated bool
	topicMu      sync.Mutex

	// Mock broker for testing/demo
	sentMessages int64
}

// NewTelemetryProducer creates new telemetry producer
func NewTelemetryProducer(config *ProducerConfig, logger *zap.Logger) (*TelemetryProducer, error) {
	if config == nil {
		return nil, fmt.Errorf("producer config cannot be nil")
	}
	if len(config.BrokerAddresses) == 0 {
		return nil, fmt.Errorf("broker addresses cannot be empty")
	}
	if config.BatchSize <= 0 {
		config.BatchSize = 50
	}
	if config.BatchTimeoutMs <= 0 {
		config.BatchTimeoutMs = 1000
	}
	if config.MaxRetries <= 0 {
		config.MaxRetries = 5
	}
	if config.CircuitBreakerThreshold <= 0 {
		config.CircuitBreakerThreshold = 10
	}

	// Connect to messagebroker via gRPC
	brokerAddr := config.BrokerAddresses[0]
	conn, err := grpc.Dial(brokerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to messagebroker at %s: %v", brokerAddr, err)
	}

	logger.Info("Connected to messagebroker", zap.String("address", brokerAddr))

	return &TelemetryProducer{
		config:         config,
		logger:         logger,
		circuitBreaker: NewCircuitBreaker(int32(config.CircuitBreakerThreshold), 30*time.Second),
		batch:          make([][]byte, 0, config.BatchSize),
		metrics:        &ProducerMetrics{},
		done:           make(chan struct{}),
		grpcConn:       conn,
		grpcClient:     pb.NewMessageBrokerClient(conn),
		topicCreated:   false,
	}, nil
}

// Start starts the producer batch flush goroutine
func (p *TelemetryProducer) Start(ctx context.Context) {
	p.wg.Add(1)
	go p.flushBatchLoop(ctx)
}

// SendMessage sends single message (queues in batch)
func (p *TelemetryProducer) SendMessage(record map[string]string) error {
	// Check circuit breaker
	if p.circuitBreaker.IsOpen() {
		p.logger.Warn("circuit breaker is open, rejecting message",
			zap.String("state", p.circuitBreaker.GetState()))
		p.metrics.RecordMessageFailed()
		return fmt.Errorf("circuit breaker open")
	}

	// Marshal record to JSON
	data, err := json.Marshal(record)
	if err != nil {
		p.logger.Error("failed to marshal record", zap.Error(err))
		p.metrics.RecordMessageFailed()
		return err
	}

	p.batchMu.Lock()
	defer p.batchMu.Unlock()

	p.batch = append(p.batch, data)
	p.batchBytes += int64(len(data))

	// Auto-flush if batch is full
	if len(p.batch) >= p.config.BatchSize {
		return p.flushBatchLocked()
	}

	return nil
}

// flushBatchLoop periodically flushes batches
func (p *TelemetryProducer) flushBatchLoop(ctx context.Context) {
	defer p.wg.Done()

	ticker := time.NewTicker(time.Duration(p.config.BatchTimeoutMs) * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("batch flush loop stopped")
			return
		case <-ticker.C:
			p.batchMu.Lock()
			if len(p.batch) > 0 {
				_ = p.flushBatchLocked()
			}
			p.batchMu.Unlock()
		case <-p.done:
			return
		}
	}
}

// flushBatchLocked flushes current batch (must hold batchMu)
func (p *TelemetryProducer) flushBatchLocked() error {
	if len(p.batch) == 0 {
		return nil
	}

	batchSize := len(p.batch)
	batchBytes := p.batchBytes
	p.batchBytes = 0

	// Create batch copy to avoid holding lock during send
	batchCopy := make([][]byte, len(p.batch))
	copy(batchCopy, p.batch)
	p.batch = p.batch[:0]

	// Send batch with retries
	err := p.sendBatchWithRetries(batchCopy)

	if err != nil {
		p.logger.Error("failed to send batch after retries",
			zap.Int("batch_size", batchSize),
			zap.Int64("batch_bytes", batchBytes),
			zap.Error(err))
		p.metrics.RecordMessageFailed()
		p.circuitBreaker.RecordFailure()
		return err
	}

	p.metrics.RecordBatchSent(int(batchBytes))
	p.circuitBreaker.RecordSuccess()

	p.logger.Debug("batch flushed successfully",
		zap.Int("batch_size", batchSize),
		zap.Int64("batch_bytes", batchBytes))

	return nil
}

// sendBatchWithRetries sends batch with exponential backoff retries
func (p *TelemetryProducer) sendBatchWithRetries(batch [][]byte) error {
	var lastErr error
	backoff := time.Duration(p.config.RetryBackoffMs) * time.Millisecond

	for attempt := 0; attempt <= p.config.MaxRetries; attempt++ {
		err := p.sendBatch(batch)
		if err == nil {
			for i := 0; i < len(batch); i++ {
				p.metrics.RecordMessageSent()
			}
			return nil
		}

		lastErr = err
		p.metrics.RecordRetryAttempt()

		if attempt < p.config.MaxRetries {
			p.logger.Warn("batch send failed, retrying",
				zap.Int("attempt", attempt+1),
				zap.Int("max_retries", p.config.MaxRetries),
				zap.Duration("backoff", backoff),
				zap.Error(err))
			time.Sleep(backoff)
			backoff *= 2 // Exponential backoff
		}
	}

	return fmt.Errorf("failed to send batch after %d retries: %w", p.config.MaxRetries, lastErr)
}

// sendBatch sends a batch of messages to the broker via gRPC
func (p *TelemetryProducer) sendBatch(batch [][]byte) error {
	if p.circuitBreaker.IsOpen() {
		return fmt.Errorf("circuit breaker is open")
	}

	if len(batch) == 0 {
		return nil
	}

	// Ensure topic exists on first send
	if !p.topicCreated {
		if err := p.ensureTopicExists(context.Background()); err != nil {
			return err
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	successCount := 0
	failCount := 0

	for _, msgData := range batch {
		req := &pb.ProduceRequest{
			Topic:     p.config.Topic,
			Partition: 0,
			Key:       []byte(fmt.Sprintf("key-%d", atomic.LoadInt64(&p.sentMessages))),
			Value:     msgData,
		}

		resp, err := p.grpcClient.ProduceMessage(ctx, req)
		if err != nil {
			p.logger.Error("failed to send message to broker",
				zap.String("topic", p.config.Topic),
				zap.Error(err))
			failCount++
			continue
		}

		p.logger.Debug("message sent successfully",
			zap.String("topic", resp.Topic),
			zap.Int32("partition", resp.Partition),
			zap.Int64("offset", resp.Offset))

		successCount++
		atomic.AddInt64(&p.sentMessages, 1)
	}

	if failCount > 0 {
		return fmt.Errorf("failed to send %d/%d messages", failCount, len(batch))
	}

	return nil
}

// ensureTopicExists creates the topic if it doesn't already exist
func (p *TelemetryProducer) ensureTopicExists(ctx context.Context) error {
	p.topicMu.Lock()
	defer p.topicMu.Unlock()

	if p.topicCreated {
		return nil
	}

	req := &pb.CreateTopicRequest{
		Topic:             p.config.Topic,
		NumPartitions:     3,
		ReplicationFactor: int32(p.config.ReplicationFactor),
	}

	resp, err := p.grpcClient.CreateTopic(ctx, req)
	if err != nil {
		// Check if it's "already exists" error (benign)
		errMsg := strings.ToLower(err.Error())
		if strings.Contains(errMsg, "already exists") {
			p.logger.Info("topic already exists", zap.String("topic", p.config.Topic))
			p.topicCreated = true
			return nil
		}
		return fmt.Errorf("failed to create topic %s: %v", p.config.Topic, err)
	}

	if resp.Success {
		p.logger.Info("topic created successfully",
			zap.String("topic", resp.Topic),
			zap.Int32("partitions", resp.Partitions))
		p.topicCreated = true
		return nil
	}

	return fmt.Errorf("failed to create topic %s: broker returned success=false", p.config.Topic)
}

// Flush flushes all remaining messages with context timeout
func (p *TelemetryProducer) Flush(ctx context.Context) error {
	flushDone := make(chan error, 1)

	go func() {
		p.batchMu.Lock()
		defer p.batchMu.Unlock()

		if len(p.batch) > 0 {
			flushDone <- p.flushBatchLocked()
		} else {
			flushDone <- nil
		}
	}()

	select {
	case err := <-flushDone:
		return err
	case <-ctx.Done():
		return fmt.Errorf("flush timeout exceeded")
	}
}

// Close closes the producer
func (p *TelemetryProducer) Close() error {
	close(p.done)
	p.wg.Wait()

	// Close gRPC connection
	if p.grpcConn != nil {
		if err := p.grpcConn.Close(); err != nil {
			p.logger.Error("error closing gRPC connection", zap.Error(err))
		}
	}

	p.logger.Info("producer closed")
	return nil
}

// GetMetrics returns current metrics
func (p *TelemetryProducer) GetMetrics() Metrics {
	return p.metrics.GetSnapshot()
}
