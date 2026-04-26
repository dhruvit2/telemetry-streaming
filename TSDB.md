# TSDB Integration Documentation

This document describes the TSDB (Time Series Database) integration in the telemetry-streaming service.

## Overview

The telemetry-streaming service now supports writing GPU telemetry metrics to InfluxDB TSDB in addition to message brokers. This enables real-time querying and historical analysis of GPU metrics.

## Features

- **Dual Write**: Data can be sent to both message brokers and TSDB simultaneously
- **Fault Tolerant**: Circuit breaker pattern for resilience
- **Async Processing**: Non-blocking writes with retry logic
- **Metrics Tracking**: Built-in metrics collection for monitoring

## Configuration

The TSDB feature is controlled by environment variables:

### Environment Variables

```bash
# Enable TSDB writing
TSDB_ENABLED=true

# InfluxDB connection details
TSDB_URL=http://influxdb-tsdb-influxdb:8086
TSDB_TOKEN=your-influxdb-token
TSDB_ORG=telemetry
TSDB_BUCKET=gpu_metrics_raw

# Retry configuration
MAX_RETRIES=5
```

### Example Configuration

```bash
export SERVICE_NAME=telemetry-streamer
export SERVICE_ID=1
export CSV_FILE_PATH=/data/telemetry.csv
export READ_SPEED=100

# Broker configuration (optional)
export BROKER_ADDRESSES=kafka:9092
export TOPIC=telemetry

# TSDB configuration
export TSDB_ENABLED=true
export TSDB_URL=http://localhost:8086
export TSDB_TOKEN=your-token
export TSDB_ORG=telemetry
export TSDB_BUCKET=gpu_metrics_raw
```

## Data Schema

GPU telemetry data is stored in InfluxDB with the following structure:

### Measurement: `gpu_metrics`

#### Tags (indexed fields for fast queries)
- `metric_name` - Name of the metric (e.g., "utilization", "memory", "temperature")
- `gpu_id` - GPU device ID (e.g., "0", "1")
- `device_id` - Physical device identifier
- `uuid` - Unique GPU identifier
- `model_name` - GPU model (e.g., "A100", "V100", "H100")
- `host_name` - Server hostname
- `container` - Kubernetes pod/container name
- Custom labels (comma-separated key=value pairs)

#### Fields (unindexed data values)
- `value` (float64) - The metric value

#### Timestamp
- RFC3339Nano format or Unix timestamp (converted to nanoseconds)

### Example Data Point

```
gpu_metrics,metric_name=utilization,gpu_id=0,device_id=dev001,uuid=abc-123,model_name=A100,host_name=server1,container=pod-1,env=production value=85.5 1704067200000000000
```

## CSV Input Format

The input CSV should have columns that map to the GPU metrics schema:

```
timestamp,metric_name,gpu_id,device_id,uuid,modelName,hostName,container,value,labels_raw
2024-01-15T10:00:00Z,utilization,0,dev001,abc-123,A100,server1,pod-1,85.5,env=production
2024-01-15T10:00:01Z,memory_usage,0,dev001,abc-123,A100,server1,pod-1,45.2,env=production
```

**Note**: Column names are flexible - the converter will extract data by column name or use defaults.

## Running the Service

### Docker

```bash
docker run -d \
  --name telemetry-streaming \
  -e TSDB_ENABLED=true \
  -e TSDB_URL=http://host.docker.internal:8086 \
  -e TSDB_TOKEN=my-token \
  -e TSDB_ORG=telemetry \
  -e TSDB_BUCKET=gpu_metrics_raw \
  -v /path/to/data:/data \
  telemetry-streaming:latest
```

### Kubernetes

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: telemetry-streaming
spec:
  replicas: 2
  template:
    spec:
      containers:
      - name: streamer
        image: telemetry-streaming:latest
        env:
        - name: TSDB_ENABLED
          value: "true"
        - name: TSDB_URL
          value: "http://influxdb-tsdb-influxdb:8086"
        - name: TSDB_TOKEN
          valueFrom:
            secretKeyRef:
              name: influxdb-credentials
              key: token
        - name: TSDB_ORG
          value: "telemetry"
        - name: TSDB_BUCKET
          value: "gpu_metrics_raw"
```

## Health Checks

The service exposes health check endpoints:

### `/health`
Returns the service health status, including TSDB health:
```bash
curl http://localhost:8080/health
```

**Response**:
```json
{
  "status": "healthy",
  "timestamp": "2024-01-15T10:00:00Z",
  "messages_sent": 1000,
  "messages_failed": 2
}
```

If TSDB is unhealthy, status will be "degraded".

### `/metrics`
Returns performance metrics for both broker and TSDB:
```bash
curl http://localhost:8080/metrics
```

**Response**:
```json
{
  "messages_sent": 1000,
  "messages_failed": 2,
  "batches_sent": 20,
  "bytes_sent": 102400,
  "tsdb_written": 1000,
  "tsdb_failed": 0
}
```

### `/ready`
Returns readiness status:
```bash
curl http://localhost:8080/ready
```

## Query Examples

### Using Flux

```flux
from(bucket: "gpu_metrics_raw")
  |> range(start: -24h)
  |> filter(fn: (r) => r._measurement == "gpu_metrics")
  |> filter(fn: (r) => r.gpu_id == "0")
  |> filter(fn: (r) => r.metric_name == "utilization")
```

### Using InfluxQL

```sql
SELECT value FROM gpu_metrics 
WHERE gpu_id='0' AND metric_name='utilization' 
AND time > now() - 24h
```

### Using telemetry-api

See the telemetry-api documentation for REST API endpoints.

## Troubleshooting

### TSDB Connection Fails

**Error**: "failed to connect to TSDB"

**Solution**:
1. Check TSDB URL is correct: `curl http://influxdb:8086/health`
2. Verify token is valid and has write permissions
3. Check network connectivity between services
4. Review TSDB logs for authentication issues

### High Failure Rate

**Symptoms**: `tsdb_failed` metric increasing

**Solutions**:
1. Check TSDB disk space: `df -h`
2. Verify bucket hasn't reached retention limit
3. Check TSDB performance: `influx stats`
4. Increase MAX_RETRIES if experiencing transient failures

### Data Not Appearing in TSDB

**Debug steps**:
1. Check `/metrics` endpoint to confirm writes are attempted
2. Query TSDB directly for the measurement:
   ```flux
   from(bucket: "gpu_metrics_raw")
     |> range(start: -1h)
     |> filter(fn: (r) => r._measurement == "gpu_metrics")
   ```
3. Review service logs for conversion errors
4. Verify CSV data format matches expected schema

## Performance Tuning

### For High Throughput

```bash
# Increase read speed
READ_SPEED=1000

# Increase batch size for broker
BATCH_SIZE=500

# Adjust batch timeout (ms)
BATCH_TIMEOUT_MS=500
```

### For Low Latency

```bash
# Decrease batch timeout
BATCH_TIMEOUT_MS=100

# Keep batch size reasonable
BATCH_SIZE=50
```

## Monitoring

### Prometheus Metrics

The service can expose Prometheus-compatible metrics. Configure in telemetry-api for scraping.

### Key Metrics

- `telemetry_streaming_records_total` - Total records processed
- `telemetry_streaming_tsdb_writes_total` - Total TSDB writes
- `telemetry_streaming_tsdb_write_errors_total` - Total TSDB write errors
- `telemetry_streaming_broker_sends_total` - Total broker sends
- `telemetry_streaming_throughput_rps` - Records per second

## Migration from Broker-only

To enable TSDB on existing deployments:

1. Set `TSDB_ENABLED=true`
2. Configure TSDB connection parameters
3. Deploy TSDB (using provided Helm chart)
4. Restart telemetry-streaming service
5. Monitor `/metrics` endpoint to confirm TSDB writes

Data will be written to both broker and TSDB after restart. No migration of existing broker data is needed.

## Best Practices

1. **Validate CSV Schema**: Ensure CSV columns match expected names
2. **Monitor Metrics**: Regular check `/metrics` and `/health` endpoints
3. **Set Appropriate Retention**: Configure TSDB retention policies based on storage
4. **Test Before Production**: Run in dual-write mode before fully migrating to TSDB
5. **Use Helm Chart**: Deploy TSDB using provided Helm chart for HA setup
6. **Enable Network Policies**: Restrict TSDB access to telemetry services only
7. **Backup Regularly**: Configure TSDB backups to S3 or other storage

## Advanced Configuration

### Custom Retention Policies

Update values.yaml before deploying Helm chart:

```yaml
influxdb:
  retentionPolicies:
    - name: "gpu_metrics_7d"
      duration: "7d"
      replication: 3
    - name: "gpu_metrics_30d"
      duration: "30d"
      replication: 3
```

### Multi-Bucket Setup

Configure telemetry-streaming to write to different buckets based on metric type:

```bash
# For raw metrics
TSDB_BUCKET=gpu_metrics_raw

# Aggregation happens in TSDB or via separate service
```

## Related Documentation

- [InfluxDB Documentation](https://docs.influxdata.com/influxdb/)
- [Helm Chart README](../helm-tsdb/README.md)
- [telemetry-api Documentation](../telemetry-api/README.md)
- [Architecture Design](./DESIGN.md)
