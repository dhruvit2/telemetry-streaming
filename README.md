# Telemetry Streaming Service

A high-performance, fault-tolerant Go service for streaming CSV data to message brokers with configurable throughput, batch processing, and resilience patterns.

## Features

### Core Capabilities
- **CSV Reader**: Configurable streaming of CSV files with rate limiting
- **Message Broker Integration**: Send data to Kafka, RabbitMQ, or other brokers
- **Batch Processing**: Efficient batching with configurable size and timeout
- **Fault Tolerance**: Circuit breaker pattern, exponential backoff retries, graceful degradation
- **Scalability**: Horizontal scaling with Kubernetes and configurable replication
- **Health Monitoring**: Real-time metrics and health endpoints

### Resilience Patterns
- **Circuit Breaker**: Prevents cascading failures when broker is unavailable
- **Exponential Backoff Retries**: Configurable retry logic with progressive delays
- **Graceful Shutdown**: Flush pending messages before termination
- **Health Checks**: Liveness and readiness probes for Kubernetes
- **Rate Limiting**: Configurable read speed per pod

### Configuration
All features are configurable via environment variables:
- `READ_SPEED`: Messages per second (default: 100)
- `BATCH_SIZE`: Messages per batch (default: 50)
- `BATCH_TIMEOUT_MS`: Batch flush timeout (default: 1000ms)
- `MAX_RETRIES`: Maximum retry attempts (default: 5)
- `RETRY_BACKOFF_MS`: Initial retry delay (default: 100ms)
- `CIRCUIT_BREAKER_THRESHOLD`: Failures to open circuit (default: 10)
- `GRACEFUL_SHUTDOWN_TIMEOUT`: Shutdown wait time (default: 30s)

## Architecture

```
┌─────────────┐
│   CSV File  │
└──────┬──────┘
       │
       ▼
┌─────────────────────────┐
│  CSV Reader (Go)        │
│  - Rate Limiting        │
│  - Stream Records       │
└──────────┬──────────────┘
           │
           ▼
┌─────────────────────────┐
│  Telemetry Producer     │
│  - Batching             │
│  - Serialization        │
│  - Resilience Patterns  │
│  - Metrics              │
└──────────┬──────────────┘
           │
           ▼
┌─────────────────────────┐
│  Message Broker         │
│  (Kafka/RabbitMQ/etc)   │
└─────────────────────────┘
```

## Building

### Prerequisites
- Go 1.21+
- Docker (for container builds)
- kubectl (for Kubernetes deployments)
- Helm 3+ (for Kubernetes deployments)
- Ansible 2.9+ (for machine deployments)

### Build Locally
```bash
# Build Go binary
make build

# Run locally
./bin/telemetry-streamer
```

### Docker Build
```bash
# Build Docker image
make docker-build

# Run Docker container
make docker-run

# Stop container
make docker-stop
```

### Verify Build
```bash
# Check binary
./bin/telemetry-streamer --version  # (if implemented)

# Check Docker image
docker images | grep telemetry-streaming

# Helm lint
make helm-lint
```

## Deployment

### Kubernetes with Helm

**Prerequisites:**
- Kubernetes 1.21+
- Helm 3+
- Kafka or message broker accessible from cluster
- Persistent storage provisioner

**Install:**
```bash
# Package Helm chart
make helm-package

# Create namespace
kubectl create namespace telemetry

# Install release
helm install telemetry-streaming deployment/helm/telemetry-streaming \
  --namespace telemetry \
  --set config.brokerAddresses="kafka-broker-0:9092,kafka-broker-1:9092" \
  --set config.topic="telemetry" \
  --set replicaCount=3

# Verify installation
kubectl get pods -n telemetry
kubectl logs -n telemetry -l app=telemetry-streaming
```

**Configuration:**
```bash
# Override configuration
helm upgrade telemetry-streaming deployment/helm/telemetry-streaming \
  --namespace telemetry \
  --set config.readSpeed=500 \
  --set config.batchSize=100 \
  --set autoscaling.enabled=true \
  --set autoscaling.maxReplicas=10
```

**Scaling:**
```bash
# Manual scaling
kubectl scale deployment telemetry-streaming -n telemetry --replicas=5

# Or via Helm
helm upgrade telemetry-streaming deployment/helm/telemetry-streaming \
  --namespace telemetry \
  --set replicaCount=5
```

**Monitoring:**
```bash
# Check pod status
kubectl get pods -n telemetry -o wide

# View metrics
kubectl logs -n telemetry -f -l app=telemetry-streaming

# Port forward health endpoint
kubectl port-forward -n telemetry svc/telemetry-streaming 8080:8080
curl http://localhost:8080/health
curl http://localhost:8080/metrics
```

### Machine Deployment with Ansible

**Prerequisites:**
- Ansible 2.9+
- SSH access to target machines
- Go runtime installed on targets
- Message broker accessible from targets

**Inventory File (`inventory.ini`):**
```ini
[telemetry_hosts]
telemetry-node-1 ansible_host=10.0.1.10 ansible_user=ubuntu
telemetry-node-2 ansible_host=10.0.1.11 ansible_user=ubuntu
telemetry-node-3 ansible_host=10.0.1.12 ansible_user=ubuntu

[telemetry_hosts:vars]
broker_addresses=kafka-broker-1:9092,kafka-broker-2:9092
topic=telemetry
read_speed=500
replica_count=3
```

**Deploy:**
```bash
# Dry run
ansible-playbook -i inventory.ini deployment/ansible/deploy.yml --check

# Deploy
ansible-playbook -i inventory.ini deployment/ansible/deploy.yml

# Deploy with custom variables
ansible-playbook -i inventory.ini deployment/ansible/deploy.yml \
  -e broker_addresses="kafka-1:9092,kafka-2:9092" \
  -e read_speed=1000
```

**Verify:**
```bash
# Check service status
ansible telemetry_hosts -i inventory.ini -m command -a "sudo systemctl status telemetry-streaming"

# Check metrics
ansible telemetry_hosts -i inventory.ini -m uri -a "url=http://localhost:8080/metrics"
```

## Configuration

### Environment Variables

```bash
# Service Configuration
SERVICE_NAME=telemetry-streamer          # Service identifier
SERVICE_ID=1                              # Instance ID for multi-replica
LOG_LEVEL=info                            # debug, info, warn, error
HEALTH_PORT=8080                          # Health check port

# CSV Configuration
CSV_FILE_PATH=/data/telemetry.csv        # Path to CSV file
READ_SPEED=100                            # Records per second

# Message Broker Configuration
BROKER_ADDRESSES=kafka:9092               # Comma-separated broker list
TOPIC=telemetry                           # Message topic
REPLICATION_FACTOR=3                      # Broker replication factor
ACKS=all                                  # Ack level: none, leader, all

# Producer Configuration
BATCH_SIZE=50                             # Messages per batch
BATCH_TIMEOUT_MS=1000                     # Batch flush timeout
COMPRESSION_TYPE=snappy                   # Compression: none, snappy, lz4, gzip
MAX_RETRIES=5                             # Retry attempts
RETRY_BACKOFF_MS=100                      # Initial backoff (exponential)

# Resilience Configuration
CIRCUIT_BREAKER_THRESHOLD=10              # Failures to trip circuit
GRACEFUL_SHUTDOWN_TIMEOUT=30              # Shutdown wait (seconds)
HEALTH_CHECK_INTERVAL=30                  # Health check frequency
```

### Tuning for Different Workloads

**High Throughput:**
```bash
READ_SPEED=5000
BATCH_SIZE=500
BATCH_TIMEOUT_MS=100
COMPRESSION_TYPE=lz4
```

**Low Latency:**
```bash
READ_SPEED=100
BATCH_SIZE=10
BATCH_TIMEOUT_MS=100
COMPRESSION_TYPE=none
```

**Reliability (Best Effort):**
```bash
MAX_RETRIES=10
RETRY_BACKOFF_MS=500
CIRCUIT_BREAKER_THRESHOLD=20
ACKS=all
```

## Health Endpoints

### `/health`
```bash
curl http://localhost:8080/health
# Response: {"status":"healthy","timestamp":"2024-01-15T10:30:45.123Z"}
```

### `/ready`
```bash
curl http://localhost:8080/ready
# Response: {"ready":true}
```

### `/metrics`
```bash
curl http://localhost:8080/metrics
# Response: {"messages_sent":10000,"messages_failed":5,"batches_sent":200,"bytes_sent":524288}
```

## Monitoring

### Prometheus Metrics (Kubernetes)
Pod annotations enable Prometheus scraping:
```yaml
annotations:
  prometheus.io/scrape: "true"
  prometheus.io/port: "8080"
  prometheus.io/path: "/metrics"
```

### Logging
- JSON structured logging with `go.uber.org/zap`
- Configurable log levels: debug, info, warn, error
- Fields: timestamp, level, service, message

### Performance Metrics
- Messages sent/failed
- Batches sent
- Total bytes transmitted
- Retry attempts
- Circuit breaker state

## Troubleshooting

### Pod Not Starting (Kubernetes)
```bash
# Check pod status
kubectl describe pod <pod-name> -n telemetry

# Check logs
kubectl logs <pod-name> -n telemetry

# Common issues:
# - CSV file not found: Verify CSV_FILE_PATH and persistent volume
# - Broker unreachable: Check BROKER_ADDRESSES and network connectivity
# - No startup permission: Verify PVC access and RBAC
```

### Circuit Breaker Open
- Check broker connectivity
- Verify broker health
- Review logs for broker errors
- Circuit resets after 30 seconds

### High Error Rate
- Check broker capacity
- Increase MAX_RETRIES
- Reduce READ_SPEED
- Check network connectivity

### Memory Usage High
- Reduce BATCH_SIZE
- Reduce BATCH_TIMEOUT_MS
- Check pod resource limits

## Performance Tuning

### Throughput
1. Increase `READ_SPEED`
2. Increase `BATCH_SIZE`
3. Use compression (snappy/lz4)
4. Increase pod CPU/memory limits
5. Increase broker replication factor

### Latency
1. Decrease `BATCH_TIMEOUT_MS`
2. Decrease `BATCH_SIZE`
3. Use `ACKS=leader` or `ACKS=none`
4. Disable compression
5. Increase read/write buffer sizes

### Reliability
1. Increase `MAX_RETRIES`
2. Increase `RETRY_BACKOFF_MS`
3. Set `ACKS=all`
4. Use higher `REPLICATION_FACTOR`
5. Add circuit breaker monitoring

## Development

### Project Structure
```
telemetry-streaming/
├── cmd/
│   └── telemetry-streamer/      # Main application
│       └── main.go
├── pkg/
│   ├── config/                  # Configuration management
│   │   └── config.go
│   ├── csv/                     # CSV reading logic
│   │   └── reader.go
│   └── producer/                # Message producer
│       ├── producer.go
│       └── metrics.go
├── deployment/
│   ├── helm/                    # Kubernetes Helm charts
│   ├── ansible/                 # Machine deployment playbooks
│   └── docker/                  # Docker configurations
├── go.mod                       # Go module definition
├── go.sum                       # Go module checksums
├── Dockerfile                   # Container image definition
├── Makefile                     # Build automation
└── README.md                    # This file
```

### Running Tests
```bash
# All tests
make test

# Specific package
go test ./pkg/config -v
go test ./pkg/csv -v
go test ./pkg/producer -v
```

### Code Quality
```bash
# Format code
make fmt

# Run linter
make lint

# Tidy modules
make mod-tidy
```

## Contributing

1. Fork the repository
2. Create feature branch: `git checkout -b feature/your-feature`
3. Commit changes: `git commit -am 'Add new feature'`
4. Push to branch: `git push origin feature/your-feature`
5. Submit pull request

## License

Copyright (c) 2024. All rights reserved.

## Support

For issues, questions, or contributions:
- GitHub Issues: [telemetry-streaming/issues](https://github.com/example/telemetry-streaming/issues)
- Documentation: [Wiki](https://github.com/example/telemetry-streaming/wiki)
- Slack: #telemetry-streaming

## Changelog

### v1.0.0 (2024-01-15)
- Initial release
- CSV streaming with rate limiting
- Message broker integration
- Circuit breaker pattern
- Exponential backoff retries
- Kubernetes deployment with Helm
- Machine deployment with Ansible
- Health check endpoints
- Structured logging
- Metrics collection
