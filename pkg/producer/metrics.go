package producer

import (
	"sync/atomic"
)

// Metrics holds producer metrics
type Metrics struct {
	MessagesSent   int64
	MessagesFailed int64
	BatchesSent    int64
	BytesSent      int64
	RetryAttempts  int64
}

// ProducerMetrics tracks producer statistics
type ProducerMetrics struct {
	messagesSent   int64
	messagesFailed int64
	batchesSent    int64
	bytesSent      int64
	retryAttempts  int64
	totalBatchTime int64
}

// RecordMessageSent increments sent message counter
func (pm *ProducerMetrics) RecordMessageSent() {
	atomic.AddInt64(&pm.messagesSent, 1)
}

// RecordMessageFailed increments failed message counter
func (pm *ProducerMetrics) RecordMessageFailed() {
	atomic.AddInt64(&pm.messagesFailed, 1)
}

// RecordBatchSent increments batch counter
func (pm *ProducerMetrics) RecordBatchSent(byteCount int) {
	atomic.AddInt64(&pm.batchesSent, 1)
	atomic.AddInt64(&pm.bytesSent, int64(byteCount))
}

// RecordRetryAttempt increments retry counter
func (pm *ProducerMetrics) RecordRetryAttempt() {
	atomic.AddInt64(&pm.retryAttempts, 1)
}

// GetSnapshot returns current metrics snapshot
func (pm *ProducerMetrics) GetSnapshot() Metrics {
	return Metrics{
		MessagesSent:   atomic.LoadInt64(&pm.messagesSent),
		MessagesFailed: atomic.LoadInt64(&pm.messagesFailed),
		BatchesSent:    atomic.LoadInt64(&pm.batchesSent),
		BytesSent:      atomic.LoadInt64(&pm.bytesSent),
		RetryAttempts:  atomic.LoadInt64(&pm.retryAttempts),
	}
}

// Reset resets all metrics
func (pm *ProducerMetrics) Reset() {
	atomic.StoreInt64(&pm.messagesSent, 0)
	atomic.StoreInt64(&pm.messagesFailed, 0)
	atomic.StoreInt64(&pm.batchesSent, 0)
	atomic.StoreInt64(&pm.bytesSent, 0)
	atomic.StoreInt64(&pm.retryAttempts, 0)
}
