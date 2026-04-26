# Quick Start Guide

Get the Telemetry Streaming Service running in 5 minutes!

## Option 1: Docker (Fastest)

### Prerequisites
- Docker installed
- Message broker accessible (Kafka/RabbitMQ)
- CSV file ready

### Steps

```bash
# 1. Clone/navigate to project
cd telemetry-streaming

# 2. Build Docker image
docker build -t telemetry-streaming .

# 3. Prepare data
mkdir -p data
cp your-data.csv data/telemetry.csv

# 4. Run container
docker run -d \
  -p 8080:8080 \
  -v $(pwd)/data:/data \
  -e BROKER_ADDRESSES=kafka:9092 \
  -e CSV_FILE_PATH=/data/telemetry.csv \
  -e LOG_LEVEL=info \
  -e READ_SPEED=100 \
  --name telemetry-stream \
  telemetry-streaming

# 5. Verify
curl http://localhost:8080/health
curl http://localhost:8080/metrics

# View logs
docker logs -f telemetry-stream
```

## Option 2: Kubernetes (Recommended for Production)

### Prerequisites
- kubectl configured
- Helm 3+ installed
- Kubernetes cluster accessible
- Message broker in cluster

### Steps

```bash
# 1. Create namespace
kubectl create namespace telemetry

# 2. Create storage for data
cat > pvc.yaml <<EOF
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: telemetry-data
  namespace: telemetry
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi
EOF

kubectl apply -f pvc.yaml

# 3. Copy CSV data
kubectl cp data.csv telemetry/telemetry-data-pvc:/data/telemetry.csv

# 4. Install Helm chart
helm install telemetry-streaming deployment/helm/telemetry-streaming \
  -n telemetry \
  --set config.brokerAddresses="kafka-broker:9092" \
  --set config.readSpeed=100

# 5. Verify deployment
kubectl get pods -n telemetry
kubectl logs -n telemetry -f -l app=telemetry-streaming

# 6. Access metrics
kubectl port-forward -n telemetry svc/telemetry-streaming 8080:8080
curl http://localhost:8080/metrics
```

## Option 3: Local Binary

### Prerequisites
- Go 1.21+
- Message broker running
- CSV file

### Steps

```bash
# 1. Build
make build

# 2. Configure environment
export LOG_LEVEL=info
export CSV_FILE_PATH=./sample-data.csv
export BROKER_ADDRESSES=localhost:9092
export READ_SPEED=100

# 3. Run
./bin/telemetry-streamer

# 4. In another terminal, check health
curl http://localhost:8080/health

# View metrics
curl http://localhost:8080/metrics
```

## Testing

### Test with Sample Data

```bash
# Use provided sample CSV
./bin/telemetry-streamer \
  -CSV_FILE_PATH=sample-data.csv \
  -READ_SPEED=1

# Or with Docker
docker run -d \
  -v $(pwd)/sample-data.csv:/data/telemetry.csv \
  -e CSV_FILE_PATH=/data/telemetry.csv \
  -e READ_SPEED=1 \
  telemetry-streaming
```

### Monitor Progress

```bash
# Watch metrics in real-time
watch -n 1 'curl -s http://localhost:8080/metrics | jq .'

# Check logs
# Docker
docker logs -f telemetry-stream

# Kubernetes
kubectl logs -f -n telemetry deployment/telemetry-streaming

# Local
tail -f app.log  # if configured
```

## Common Configurations

### Development (Low Data Volume)
```bash
READ_SPEED=10
BATCH_SIZE=10
BATCH_TIMEOUT_MS=500
```

### Production (Medium Volume)
```bash
READ_SPEED=500
BATCH_SIZE=100
BATCH_TIMEOUT_MS=1000
```

### High Throughput
```bash
READ_SPEED=5000
BATCH_SIZE=500
BATCH_TIMEOUT_MS=100
```

## Troubleshooting

### Issue: "Connection refused"
```bash
# Verify broker is running
# For Kafka:
kafka-broker-api-versions.sh --bootstrap-server localhost:9092

# For RabbitMQ:
rabbitmq-diagnostics status
```

### Issue: "CSV file not found"
```bash
# Verify file path
ls -la /data/telemetry.csv

# Check file permissions
chmod 644 /data/telemetry.csv
```

### Issue: No data being sent
```bash
# Check logs for errors
curl http://localhost:8080/metrics

# Try slower read speed
READ_SPEED=1

# Check circuit breaker status in logs
LOG_LEVEL=debug
```

## Next Steps

1. **Read Configuration**: See [CONFIGURATION.md](CONFIGURATION.md)
2. **Learn Deployment**: See [DEPLOYMENT.md](DEPLOYMENT.md)
3. **Review Full README**: See [README.md](README.md)
4. **Setup Monitoring**: Configure Prometheus scraping
5. **Scale horizontally**: Increase replicas in Kubernetes

## Useful Commands

### Docker
```bash
# View container logs
docker logs telemetry-stream

# Execute command in container
docker exec telemetry-stream curl http://localhost:8080/health

# Stop container
docker stop telemetry-stream

# Remove container
docker rm telemetry-stream
```

### Kubernetes
```bash
# View pod logs
kubectl logs -n telemetry telemetry-streaming-0

# Port forward for local access
kubectl port-forward -n telemetry pod/telemetry-streaming-0 8080:8080

# Describe pod for events
kubectl describe pod -n telemetry telemetry-streaming-0

# Scale deployment
kubectl scale deployment telemetry-streaming -n telemetry --replicas=5

# Update configuration
kubectl edit configmap -n telemetry telemetry-streaming-config
```

### Helm
```bash
# List releases
helm list -n telemetry

# View values
helm get values telemetry-streaming -n telemetry

# Upgrade release
helm upgrade telemetry-streaming deployment/helm/telemetry-streaming \
  -n telemetry \
  --set config.readSpeed=1000

# Uninstall
helm uninstall telemetry-streaming -n telemetry
```

## Health Endpoints

Access these endpoints to verify service health:

```bash
# Health status
curl http://localhost:8080/health

# Readiness probe
curl http://localhost:8080/ready

# Metrics
curl http://localhost:8080/metrics | jq .
```

## Support

Having issues? Check:
1. [README.md](README.md) - Full documentation
2. [CONFIGURATION.md](CONFIGURATION.md) - Configuration options
3. [DEPLOYMENT.md](DEPLOYMENT.md) - Deployment guides
4. Logs: `docker logs` or `kubectl logs`
5. Metrics: `curl http://localhost:8080/metrics`
