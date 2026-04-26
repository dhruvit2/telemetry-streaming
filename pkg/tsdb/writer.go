package tsdb
package tsdb

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"go.uber.org/zap"
)

// TSDBConfig holds TSDB configuration
type TSDBConfig struct {
	URL       string
	Token     string
	Org       string
	Bucket    string
	MaxRetries int
}

// TSDBWriter writes telemetry data to InfluxDB TSDB
type TSDBWriter struct {
	config   *TSDBConfig
	logger   *zap.Logger
	client   influxdb2.Client
	writeAPI api.WriteAPIBlocking
	
	// Metrics
	totalWritten int64
	totalFailed  int64
	batchSize    int
	batchMu      sync.Mutex
	batch        []byte
	batchPoints  int
	
	done chan struct{}
	wg   sync.WaitGroup
}

// NewTSDBWriter creates new TSDB writer
func NewTSDBWriter(config *TSDBConfig, logger *zap.Logger) (*TSDBWriter, error) {
	if config == nil {
		return nil, fmt.Errorf("tsdb config cannot be nil")
	}
	if config.URL == "" {
		return nil, fmt.Errorf("tsdb url cannot be empty")
	}
	if config.Token == "" {
		return nil, fmt.Errorf("tsdb token cannot be empty")
	}
	if config.Org == "" {
		return nil, fmt.Errorf("tsdb org cannot be empty")
	}
	if config.Bucket == "" {
		return nil, fmt.Errorf("tsdb bucket cannot be empty")
	}
	if config.MaxRetries <= 0 {
		config.MaxRetries = 3
	}

	// Create InfluxDB client
	client := influxdb2.NewClient(config.URL, config.Token)
	
	// Test connection
	health, err := client.Health(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to TSDB: %w", err)
	}
	
	if health.Status != "ok" {
		return nil, fmt.Errorf("tsdb health status not ok: %s", health.Message)
	}

	writer := &TSDBWriter{
		config:   config,
		logger:   logger,
		client:   client,
		writeAPI: client.WriteAPIBlocking(config.Org, config.Bucket),
		batch:    make([]byte, 0, 1024*1024), // 1MB initial capacity
		done:     make(chan struct{}),
	}

	logger.Info("TSDB writer initialized",
		zap.String("url", config.URL),
		zap.String("bucket", config.Bucket),
		zap.String("org", config.Org))

	return writer, nil
}

// WriteMetric writes a single metric to TSDB
func (w *TSDBWriter) WriteMetric(ctx context.Context, record map[string]interface{}) error {
	// Convert map to InfluxDB Line Protocol
	lineProtocol, err := w.convertToLineProtocol(record)
	if err != nil {
		w.logger.Error("failed to convert record to line protocol", zap.Error(err))
		atomic.AddInt64(&w.totalFailed, 1)
		return err
	}

	// Write to TSDB with retries
	err = w.writeWithRetries(ctx, lineProtocol)
	if err != nil {
		atomic.AddInt64(&w.totalFailed, 1)
		return err
	}

	atomic.AddInt64(&w.totalWritten, 1)
	return nil
}

// writeWithRetries writes data with exponential backoff retries
func (w *TSDBWriter) writeWithRetries(ctx context.Context, lineProtocol string) error {
	var lastErr error
	backoff := 100 * time.Millisecond

	for attempt := 0; attempt <= w.config.MaxRetries; attempt++ {
		err := w.writeAPI.WriteRecord(ctx, lineProtocol)
		if err == nil {
			return nil
		}

		lastErr = err
		if attempt < w.config.MaxRetries {
			w.logger.Warn("failed to write to TSDB, retrying",
				zap.Int("attempt", attempt+1),
				zap.Int("max_retries", w.config.MaxRetries),
				zap.Duration("backoff", backoff),
				zap.Error(err))
			time.Sleep(backoff)
			backoff *= 2 // Exponential backoff
		}
	}

	return fmt.Errorf("failed to write to TSDB after %d retries: %w", w.config.MaxRetries, lastErr)
}

// convertToLineProtocol converts telemetry record to InfluxDB line protocol format
// Expected fields in record:
// timestamp, metric_name, gpu_id, device_id, uuid, modelName, hostName, container, value, labels_raw
func (w *TSDBWriter) convertToLineProtocol(record map[string]interface{}) (string, error) {
	// Extract required fields
	timestamp := w.getStringField(record, "timestamp", "")
	metricName := w.getStringField(record, "metric_name", "gpu_metric")
	gpuID := w.getStringField(record, "gpu_id", "unknown")
	deviceID := w.getStringField(record, "device_id", "unknown")
	uuid := w.getStringField(record, "uuid", "unknown")
	modelName := w.getStringField(record, "modelName", "unknown")
	hostName := w.getStringField(record, "hostName", "unknown")
	container := w.getStringField(record, "container", "unknown")
	value := w.getFloatField(record, "value", 0.0)
	labelsRaw := w.getStringField(record, "labels_raw", "")

	// Parse timestamp
	var ts int64
	if timestamp != "" {
		t, err := time.Parse(time.RFC3339Nano, timestamp)
		if err != nil {
			// Try to parse as Unix timestamp
			if unixTS, err2 := strconv.ParseInt(timestamp, 10, 64); err2 == nil {
				ts = unixTS * 1_000_000_000 // Convert to nanoseconds
			} else {
				// Use current time if parsing fails
				ts = time.Now().UnixNano()
			}
		} else {
			ts = t.UnixNano()
		}
	} else {
		ts = time.Now().UnixNano()
	}

	// Build line protocol
	// Format: measurement[,tag=value[,tag=value]] field=value[,field=value] timestamp
	lineProtocol := fmt.Sprintf(
		"gpu_metrics,metric_name=%s,gpu_id=%s,device_id=%s,uuid=%s,model_name=%s,host_name=%s,container=%s%s value=%.2f %d",
		metricName,
		gpuID,
		deviceID,
		uuid,
		modelName,
		hostName,
		container,
		w.formatLabels(labelsRaw),
		value,
		ts,
	)

	return lineProtocol, nil
}

// formatLabels converts labels_raw to line protocol format
func (w *TSDBWriter) formatLabels(labelsRaw string) string {
	if labelsRaw == "" {
		return ""
	}
	// labelsRaw is expected to be comma-separated key=value pairs
	// Convert to InfluxDB tag format
	if labelsRaw != "" {
		return "," + labelsRaw
	}
	return ""
}

// getStringField safely gets string field from map
func (w *TSDBWriter) getStringField(m map[string]interface{}, key, defaultVal string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return defaultVal
}

// getFloatField safely gets float field from map
func (w *TSDBWriter) getFloatField(m map[string]interface{}, key string, defaultVal float64) float64 {
	if val, ok := m[key]; ok {
		switch v := val.(type) {
		case float64:
			return v
		case float32:
			return float64(v)
		case int:
			return float64(v)
		case int64:
			return float64(v)
		case string:
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				return f
			}
		}
	}
	return defaultVal
}

// GetMetrics returns writer metrics
func (w *TSDBWriter) GetMetrics() map[string]int64 {
	return map[string]int64{
		"total_written": atomic.LoadInt64(&w.totalWritten),
		"total_failed":  atomic.LoadInt64(&w.totalFailed),
	}
}

// Close closes the TSDB connection
func (w *TSDBWriter) Close() error {
	w.client.Close()
	close(w.done)
	w.wg.Wait()
	return nil
}

// Health checks TSDB health
func (w *TSDBWriter) Health(ctx context.Context) (bool, error) {
	health, err := w.client.Health(ctx)
	if err != nil {
		return false, err
	}
	return health.Status == "ok", nil
}
