package tsdb
package tsdb

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"
)

// MockInfluxDBWriter is a mock TSDB writer for testing
type MockTSDBWriter struct {
	WrittenMetrics []map[string]interface{}
	FailureCount   int
	ShouldFail     bool
}

func NewMockTSDBWriter() *MockTSDBWriter {
	return &MockTSDBWriter{
		WrittenMetrics: make([]map[string]interface{}, 0),
	}
}

func (m *MockTSDBWriter) WriteMetric(ctx context.Context, record map[string]interface{}) error {
	if m.ShouldFail {
		m.FailureCount++
		return nil // Mock doesn't track errors for simplicity
	}
	m.WrittenMetrics = append(m.WrittenMetrics, record)
	return nil
}

func (m *MockTSDBWriter) Close() error {
	return nil
}

func TestConvertToLineProtocol(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	tests := []struct {
		name     string
		record   map[string]interface{}
		wantErr  bool
		validate func(string) bool
	}{
		{
			name: "valid GPU metric",
			record: map[string]interface{}{
				"timestamp":  time.Now().Format(time.RFC3339Nano),
				"metric_name": "gpu_utilization",
				"gpu_id":      "0",
				"device_id":   "dev001",
				"uuid":        "abc-123-def",
				"modelName":   "A100",
				"hostName":    "server1",
				"container":   "pod-1",
				"value":       85.5,
				"labels_raw":  "",
			},
			wantErr: false,
			validate: func(lp string) bool {
				return len(lp) > 0
			},
		},
		{
			name: "with labels",
			record: map[string]interface{}{
				"timestamp":   time.Now().Format(time.RFC3339Nano),
				"metric_name": "gpu_memory",
				"gpu_id":      "1",
				"device_id":   "dev002",
				"uuid":        "xyz-789",
				"modelName":   "V100",
				"hostName":    "server2",
				"container":   "pod-2",
				"value":       45.0,
				"labels_raw":   "env=production,zone=us-east",
			},
			wantErr: false,
			validate: func(lp string) bool {
				return len(lp) > 0
			},
		},
		{
			name: "missing optional fields",
			record: map[string]interface{}{
				"value": 100.0,
			},
			wantErr: false,
			validate: func(lp string) bool {
				return len(lp) > 0
			},
		},
		{
			name: "string value as field",
			record: map[string]interface{}{
				"value": "42.5",
			},
			wantErr: false,
			validate: func(lp string) bool {
				return len(lp) > 0
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writer := &TSDBWriter{
				logger: logger,
			}
			
			lp, err := writer.convertToLineProtocol(tt.record)
			if (err != nil) != tt.wantErr {
				t.Errorf("convertToLineProtocol() error = %v, wantErr %v", err, tt.wantErr)
			}
			
			if !tt.wantErr && !tt.validate(lp) {
				t.Errorf("convertToLineProtocol() returned invalid line protocol: %s", lp)
			}
		})
	}
}

func TestGetStringField(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()
	
	writer := &TSDBWriter{logger: logger}
	
	tests := []struct {
		name        string
		m           map[string]interface{}
		key         string
		defaultVal  string
		expected    string
	}{
		{
			name:       "field exists",
			m:          map[string]interface{}{"gpu_id": "0"},
			key:        "gpu_id",
			defaultVal: "unknown",
			expected:   "0",
		},
		{
			name:       "field missing",
			m:          map[string]interface{}{},
			key:        "gpu_id",
			defaultVal: "unknown",
			expected:   "unknown",
		},
		{
			name:       "field wrong type",
			m:          map[string]interface{}{"gpu_id": 123},
			key:        "gpu_id",
			defaultVal: "unknown",
			expected:   "unknown",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := writer.getStringField(tt.m, tt.key, tt.defaultVal)
			if got != tt.expected {
				t.Errorf("getStringField() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGetFloatField(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()
	
	writer := &TSDBWriter{logger: logger}
	
	tests := []struct {
		name       string
		m          map[string]interface{}
		key        string
		defaultVal float64
		expected   float64
	}{
		{
			name:       "float64 field",
			m:          map[string]interface{}{"value": 85.5},
			key:        "value",
			defaultVal: 0.0,
			expected:   85.5,
		},
		{
			name:       "int field",
			m:          map[string]interface{}{"value": 100},
			key:        "value",
			defaultVal: 0.0,
			expected:   100.0,
		},
		{
			name:       "string field",
			m:          map[string]interface{}{"value": "42.5"},
			key:        "value",
			defaultVal: 0.0,
			expected:   42.5,
		},
		{
			name:       "missing field",
			m:          map[string]interface{}{},
			key:        "value",
			defaultVal: 10.0,
			expected:   10.0,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := writer.getFloatField(tt.m, tt.key, tt.defaultVal)
			if got != tt.expected {
				t.Errorf("getFloatField() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestFormatLabels(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()
	
	writer := &TSDBWriter{logger: logger}
	
	tests := []struct {
		name      string
		labelsRaw string
		expected  string
	}{
		{
			name:      "empty labels",
			labelsRaw: "",
			expected:  "",
		},
		{
			name:      "with labels",
			labelsRaw: "env=prod,zone=us-east",
			expected:  ",env=prod,zone=us-east",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := writer.formatLabels(tt.labelsRaw)
			if got != tt.expected {
				t.Errorf("formatLabels() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestTSDBWriterMetrics(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()
	
	writer := &TSDBWriter{
		config: &TSDBConfig{
			MaxRetries: 3,
		},
		logger: logger,
	}
	
	// Simulate writes
	writer.totalWritten = 100
	writer.totalFailed = 5
	
	metrics := writer.GetMetrics()
	
	if metrics["total_written"] != 100 {
		t.Errorf("Expected total_written=100, got %d", metrics["total_written"])
	}
	
	if metrics["total_failed"] != 5 {
		t.Errorf("Expected total_failed=5, got %d", metrics["total_failed"])
	}
}
