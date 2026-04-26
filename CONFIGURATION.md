# Configuration Guide

## Environment Variables

All configuration is managed through environment variables. The service loads configuration on startup.

### Service Configuration

#### SERVICE_NAME
- **Description**: Identifies the service in logs
- **Default**: `telemetry-streamer`
- **Example**: `SERVICE_NAME=telemetry-streaming-prod`

#### SERVICE_ID
- **Description**: Unique identifier for each instance in a multi-replica deployment
- **Default**: `1`
- **Example**: `SERVICE_ID=3`
- **Note**: Set different values for each pod/instance

#### LOG_LEVEL
- **Description**: Controls logging verbosity
- **Options**: `debug`, `info`, `warn`, `error`
- **Default**: `info`
- **Example**: `LOG_LEVEL=debug`

#### HEALTH_PORT
- **Description**: Port for health check endpoints
- **Default**: `8080`
- **Example**: `HEALTH_PORT=9090`

### CSV Configuration

#### CSV_FILE_PATH
- **Description**: Path to CSV file to stream
- **Default**: `/data/telemetry.csv`
- **Example**: `CSV_FILE_PATH=/mnt/data/metrics.csv`
- **Requirements**: 
  - File must be readable
  - Must have headers in first row
  - Comma-delimited format

#### READ_SPEED
- **Description**: Number of CSV records to read per second
- **Default**: `100`
- **Example**: `READ_SPEED=1000`
- **Tuning**:
  - Lower for testing: 10-50
  - Moderate for production: 100-500
  - High for large-scale: 1000+
- **Note**: Actual throughput depends on batch size and broker capacity

### Message Broker Configuration

#### BROKER_ADDRESSES
- **Description**: Comma-separated list of broker addresses
- **Default**: `localhost:9092`
- **Examples**:
  - Kafka: `BROKER_ADDRESSES=kafka-1:9092,kafka-2:9092,kafka-3:9092`
  - RabbitMQ: `BROKER_ADDRESSES=rabbitmq-1:5672,rabbitmq-2:5672`
- **Format**: `host1:port1,host2:port2,...`

#### TOPIC
- **Description**: Message topic/queue name
- **Default**: `telemetry`
- **Example**: `TOPIC=metrics-stream`

#### REPLICATION_FACTOR
- **Description**: Broker replication factor (Kafka)
- **Default**: `3`
- **Range**: `1` to `broker_count`
- **Note**: Higher values increase reliability but reduce throughput

#### ACKS
- **Description**: Acknowledgment level (Kafka)
- **Options**:
  - `none`: No acknowledgment (fastest, least reliable)
  - `leader`: Wait for leader confirmation (balanced)
  - `all`: Wait for all replicas (slowest, most reliable)
- **Default**: `all`
- **Example**: `ACKS=leader`

#### COMPRESSION_TYPE
- **Description**: Message compression algorithm
- **Options**: `none`, `snappy`, `lz4`, `gzip`
- **Default**: `snappy`
- **Trade-offs**:
  - `none`: Fastest, uses most bandwidth
  - `snappy`: Fast compression, good ratio
  - `lz4`: Very fast, lower ratio
  - `gzip`: Slow compression, best ratio

### Producer Configuration

#### BATCH_SIZE
- **Description**: Number of messages to batch before sending
- **Default**: `50`
- **Range**: `1` to `1000`
- **Tuning**:
  - Small (10-20): Lower latency, higher overhead
  - Medium (50-100): Balanced (recommended)
  - Large (200+): Higher throughput, higher latency

#### BATCH_TIMEOUT_MS
- **Description**: Time to wait before flushing batch (milliseconds)
- **Default**: `1000`
- **Range**: `10` to `10000`
- **Tuning**:
  - Low (10-100): Low latency, high broker load
  - Medium (500-1000): Balanced (recommended)
  - High (2000+): High throughput, higher latency

#### MAX_RETRIES
- **Description**: Maximum retry attempts for failed sends
- **Default**: `5`
- **Range**: `0` to `20`
- **Tuning**:
  - Low (0-2): Fast failure, data loss risk
  - Medium (3-5): Balanced (recommended)
  - High (10+): High reliability, slower recovery

#### RETRY_BACKOFF_MS
- **Description**: Initial backoff delay for retries (exponential)
- **Default**: `100`
- **Range**: `10` to `5000`
- **Tuning**:
  - Low (10-50): Fast retry, may overload broker
  - Medium (100-500): Balanced (recommended)
  - High (1000+): Slow retry, gives broker time to recover

### Resilience Configuration

#### CIRCUIT_BREAKER_THRESHOLD
- **Description**: Number of failures before opening circuit
- **Default**: `10`
- **Range**: `1` to `100`
- **Tuning**:
  - Low (1-5): Aggressive, quick fail
  - Medium (5-20): Balanced (recommended)
  - High (20+): Tolerant, slower to fail

#### HEALTH_CHECK_INTERVAL
- **Description**: Health check frequency (seconds)
- **Default**: `30`
- **Example**: `HEALTH_CHECK_INTERVAL=60`

#### GRACEFUL_SHUTDOWN_TIMEOUT
- **Description**: Time to wait for messages to flush on shutdown (seconds)
- **Default**: `30`
- **Range**: `5` to `300`
- **Tuning**:
  - Low (5-10): Quick shutdown, may lose messages
  - Medium (30): Balanced (recommended)
  - High (60+): Ensures all messages sent, slower shutdown

## Configuration Profiles

### Development Profile
```bash
LOG_LEVEL=debug
READ_SPEED=10
BATCH_SIZE=10
BATCH_TIMEOUT_MS=500
MAX_RETRIES=2
COMPRESSION_TYPE=none
ACKS=leader
CIRCUIT_BREAKER_THRESHOLD=3
```

### Production Profile (Balanced)
```bash
LOG_LEVEL=info
READ_SPEED=500
BATCH_SIZE=50
BATCH_TIMEOUT_MS=1000
MAX_RETRIES=5
COMPRESSION_TYPE=snappy
ACKS=all
CIRCUIT_BREAKER_THRESHOLD=10
GRACEFUL_SHUTDOWN_TIMEOUT=60
```

### High-Throughput Profile
```bash
LOG_LEVEL=warn
READ_SPEED=5000
BATCH_SIZE=500
BATCH_TIMEOUT_MS=100
MAX_RETRIES=3
COMPRESSION_TYPE=lz4
ACKS=leader
CIRCUIT_BREAKER_THRESHOLD=20
```

### High-Reliability Profile
```bash
LOG_LEVEL=info
READ_SPEED=100
BATCH_SIZE=20
BATCH_TIMEOUT_MS=2000
MAX_RETRIES=10
COMPRESSION_TYPE=gzip
ACKS=all
CIRCUIT_BREAKER_THRESHOLD=5
GRACEFUL_SHUTDOWN_TIMEOUT=120
```

## Kubernetes Configuration

### Via values.yaml
```yaml
config:
  logLevel: info
  readSpeed: 500
  brokerAddresses: "kafka-1:9092,kafka-2:9092"
  topic: telemetry
  batchSize: 100
  maxRetries: 5
```

### Via --set
```bash
helm install telemetry-streaming chart/ \
  --set config.readSpeed=1000 \
  --set config.batchSize=200 \
  --set autoscaling.maxReplicas=20
```

### Via ConfigMap
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: telemetry-config
data:
  LOG_LEVEL: "info"
  READ_SPEED: "500"
  BATCH_SIZE: "100"
```

## Machine Configuration

### Systemd Service File
```ini
[Service]
Environment="LOG_LEVEL=info"
Environment="READ_SPEED=500"
Environment="BATCH_SIZE=100"
Environment="BROKER_ADDRESSES=kafka-1:9092,kafka-2:9092"
Environment="CSV_FILE_PATH=/var/lib/telemetry/data.csv"
```

### Shell Script
```bash
#!/bin/bash
export LOG_LEVEL=info
export READ_SPEED=500
export BATCH_SIZE=100
export BROKER_ADDRESSES=kafka-1:9092,kafka-2:9092
export CSV_FILE_PATH=/var/lib/telemetry/data.csv
exec /opt/telemetry-streaming/telemetry-streamer
```

## Performance Tuning Guide

### For Maximum Throughput
1. Increase `READ_SPEED` (e.g., 5000)
2. Increase `BATCH_SIZE` (e.g., 500)
3. Decrease `BATCH_TIMEOUT_MS` (e.g., 100)
4. Use `COMPRESSION_TYPE=lz4`
5. Set `ACKS=leader` or `ACKS=none`
6. Increase pod replicas and CPU/memory

### For Minimum Latency
1. Decrease `BATCH_SIZE` (e.g., 10)
2. Decrease `BATCH_TIMEOUT_MS` (e.g., 100)
3. Use `COMPRESSION_TYPE=none`
4. Set `ACKS=leader` or `ACKS=none`
5. Reduce network latency to broker

### For Maximum Reliability
1. Increase `MAX_RETRIES` (e.g., 10)
2. Increase `RETRY_BACKOFF_MS` (e.g., 1000)
3. Set `ACKS=all`
4. Increase `REPLICATION_FACTOR` (e.g., 3+)
5. Set `CIRCUIT_BREAKER_THRESHOLD` low (e.g., 5)
6. Increase `GRACEFUL_SHUTDOWN_TIMEOUT`

### For Cost Optimization
1. Decrease `READ_SPEED` to match demand
2. Use smaller batch sizes when possible
3. Set `COMPRESSION_TYPE=gzip` for bandwidth savings
4. Use fewer replicas (minimum 1 for HA)
5. Configure HPA with lower target utilization

## Validation and Testing

### Verify Configuration
```bash
# Check loaded configuration
curl http://localhost:8080/metrics

# Expected output shows configured values in action
# e.g., batch sizes, retry counts
```

### Load Testing
```bash
# Generate test data
head -1000 sample-data.csv > test-data.csv

# Test configuration
docker run \
  -e CSV_FILE_PATH=/data/test.csv \
  -e READ_SPEED=1000 \
  -e BATCH_SIZE=100 \
  -v $(pwd)/test-data.csv:/data/test.csv \
  telemetry-streaming:latest
```

## Troubleshooting Configuration

### Issue: "CSV file not found"
- Verify `CSV_FILE_PATH` is correct
- Check file permissions
- In Kubernetes, verify PVC is mounted

### Issue: "Connection refused"
- Verify `BROKER_ADDRESSES` is correct
- Check broker is running
- Verify network connectivity

### Issue: "Circuit breaker open"
- Reduce `CIRCUIT_BREAKER_THRESHOLD`
- Verify broker capacity
- Increase `RETRY_BACKOFF_MS`

### Issue: "High memory usage"
- Reduce `BATCH_SIZE`
- Reduce `READ_SPEED`
- Decrease pod replica count
