package csv

import (
	"bufio"
	"context"
	"encoding/csv"
	"os"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Record represents a single CSV row as key-value pairs
type Record struct {
	Headers []string
	Values  []string
	LineNum int64
}

// ToMap converts record to map
func (r *Record) ToMap() map[string]string {
	result := make(map[string]string)
	for i, header := range r.Headers {
		if i < len(r.Values) {
			result[header] = r.Values[i]
		}
	}
	return result
}

// Reader reads CSV file and streams records with configurable speed
type Reader struct {
	filePath  string
	readSpeed int // records per second
	logger    *zap.Logger
	mu        sync.RWMutex
	closed    bool
}

// NewReader creates new CSV reader
func NewReader(filePath string, readSpeed int, logger *zap.Logger) *Reader {
	return &Reader{
		filePath:  filePath,
		readSpeed: readSpeed,
		logger:    logger,
		closed:    false,
	}
}

// StreamRecords reads CSV and sends records to channel with rate limiting
func (r *Reader) StreamRecords(ctx context.Context) <-chan *Record {
	recordsChan := make(chan *Record, 100)
	
	go func() {
		defer close(recordsChan)
		
		file, err := os.Open(r.filePath)
		if err != nil {
			r.logger.Error("failed to open CSV file", 
				zap.String("path", r.filePath), 
				zap.Error(err))
			return
		}
		defer file.Close()
		
		reader := csv.NewReader(bufio.NewReader(file))
		
		// Read headers
		headers, err := reader.Read()
		if err != nil {
			r.logger.Error("failed to read CSV headers", zap.Error(err))
			return
		}
		
		r.logger.Info("CSV headers read", zap.Int("column_count", len(headers)))
		
		// Calculate throttle duration
		throttleDuration := time.Duration(1000000000 / r.readSpeed) // nanoseconds
		ticker := time.NewTicker(throttleDuration)
		defer ticker.Stop()
		
		lineNum := int64(1)
		totalProcessed := int64(0)
		
		for {
			select {
			case <-ctx.Done():
				r.logger.Info("CSV stream cancelled", zap.Int64("records_processed", totalProcessed))
				return
			case <-ticker.C:
				values, err := reader.Read()
				if err != nil {
					// EOF is normal, end of file reached
					if totalProcessed > 0 {
						r.logger.Info("EOF reached, CSV streaming complete", 
							zap.Int64("total_records", totalProcessed))
					}
					return
				}
				
				record := &Record{
					Headers: headers,
					Values:  values,
					LineNum: lineNum,
				}
				
				select {
				case recordsChan <- record:
					lineNum++
					totalProcessed++
				case <-ctx.Done():
					r.logger.Info("CSV stream cancelled during send", 
						zap.Int64("records_processed", totalProcessed))
					return
				}
			}
		}
	}()
	
	return recordsChan
}

// Close closes the reader
func (r *Reader) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = true
	r.logger.Debug("CSV reader closed")
	return nil
}

// IsClosed returns whether reader is closed
func (r *Reader) IsClosed() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.closed
}
