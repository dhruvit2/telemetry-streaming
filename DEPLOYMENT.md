# Deployment Guide

## Overview

This guide covers deploying the Telemetry Streaming Service to various environments.

## Prerequisites

### All Environments
- Service binary built and tested
- Message broker (Kafka, RabbitMQ, etc.) configured and accessible
- CSV data file prepared
- Network connectivity between service and broker

### Kubernetes
- kubectl configured
- Helm 3+ installed
- Kubernetes cluster 1.21+ with RBAC enabled
- Persistent storage provisioner available
- Container registry access (if using private images)

### Machine (Ansible)
- Ansible 2.9+ installed
- SSH access to all target machines
- Linux OS (Ubuntu 20.04+, CentOS 8+, or similar)
- Go 1.21+ runtime installed on targets
- sudo/root privileges for systemd management

## Quick Start: Docker

```bash
# Build image
docker build -t telemetry-streaming:1.0.0 .

# Prepare data volume
mkdir -p ./data
cp your-telemetry-data.csv ./data/telemetry.csv

# Run container
docker run -d \
  -p 8080:8080 \
  -v ./data:/data \
  -e BROKER_ADDRESSES=kafka:9092 \
  -e CSV_FILE_PATH=/data/telemetry.csv \
  -e LOG_LEVEL=info \
  --name telemetry-streamer \
  telemetry-streaming:1.0.0

# Check health
curl http://localhost:8080/health
```

## Kubernetes Deployment

### Step 1: Prepare Image

```bash
# Build and tag
docker build -t myregistry.azurecr.io/telemetry-streaming:1.0.0 .

# Push to registry
docker push myregistry.azurecr.io/telemetry-streaming:1.0.0
```

### Step 2: Create Namespace and Secrets

```bash
# Create namespace
kubectl create namespace telemetry

# Create image pull secret (if using private registry)
kubectl create secret docker-registry regcred \
  --docker-server=myregistry.azurecr.io \
  --docker-username=<username> \
  --docker-password=<password> \
  -n telemetry
```

### Step 3: Prepare Storage

```bash
# Create storage class (if not already exist)
cat > storage-class.yaml <<EOF
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: fast-ssd
provisioner: kubernetes.io/aws-ebs  # or your provisioner
parameters:
  type: gp3
  iops: "1000"
  throughput: "250"
EOF

kubectl apply -f storage-class.yaml
```

### Step 4: Configure Values

Create `custom-values.yaml`:

```yaml
replicaCount: 3

image:
  repository: myregistry.azurecr.io/telemetry-streaming
  tag: "1.0.0"

imagePullSecrets:
  - name: regcred

config:
  serviceId: "1"
  logLevel: "info"
  readSpeed: 500
  brokerAddresses: "kafka-broker-0.kafka:9092,kafka-broker-1.kafka:9092,kafka-broker-2.kafka:9092"
  topic: "telemetry"
  replicationFactor: 3
  batchSize: 100
  maxRetries: 5

persistence:
  enabled: true
  storageClass: "fast-ssd"
  size: 20Gi

autoscaling:
  enabled: true
  minReplicas: 3
  maxReplicas: 10
  targetCPUUtilizationPercentage: 70

resources:
  requests:
    cpu: 500m
    memory: 256Mi
  limits:
    cpu: 2000m
    memory: 1Gi
```

### Step 5: Install Helm Chart

```bash
# Create namespace
kubectl create namespace telemetry

# Install
helm install telemetry-streaming deployment/helm/telemetry-streaming \
  -n telemetry \
  -f custom-values.yaml

# Verify
kubectl get pods -n telemetry -o wide
kubectl logs -n telemetry -l app=telemetry-streaming
```

### Step 6: Load CSV Data

```bash
# Copy CSV to persistent volume
kubectl cp local-data.csv telemetry/telemetry-streaming-0:/data/telemetry.csv

# Or mount ConfigMap with data
kubectl create configmap telemetry-data --from-file=telemetry.csv -n telemetry
```

## Machine Deployment with Ansible

### Step 1: Prepare Inventory

Create `inventory.ini`:

```ini
[telemetry_hosts]
streamer-1 ansible_host=10.0.1.10 ansible_user=ubuntu
streamer-2 ansible_host=10.0.1.11 ansible_user=ubuntu
streamer-3 ansible_host=10.0.1.12 ansible_user=ubuntu

[telemetry_hosts:vars]
ansible_ssh_private_key_file=~/.ssh/id_rsa
ansible_become=yes
broker_addresses=kafka-broker-1:9092,kafka-broker-2:9092
topic=telemetry
csv_file_path=/var/lib/telemetry/data.csv
read_speed=500
batch_size=100
max_retries=5
```

### Step 2: Prepare Configuration

Create `group_vars/telemetry_hosts.yml`:

```yaml
---
# Service configuration
telemetry_user: telemetry
telemetry_group: telemetry
telemetry_home: /home/telemetry
telemetry_data: /var/lib/telemetry

# Service configuration
service_config:
  log_level: info
  read_speed: 500
  batch_size: 100
  batch_timeout_ms: 1000
  max_retries: 5
  retry_backoff_ms: 100
  circuit_breaker_threshold: 10
  graceful_shutdown_timeout: 30

# Broker configuration
broker_config:
  addresses: "kafka-1:9092,kafka-2:9092"
  topic: telemetry
  replication_factor: 3
  acks: all
  compression_type: snappy

# Health check
health_port: 8080
health_check_interval: 30
```

### Step 3: Run Deployment

```bash
# Dry run
ansible-playbook -i inventory.ini deployment/ansible/deploy.yml --check

# Deploy
ansible-playbook -i inventory.ini deployment/ansible/deploy.yml -v

# Deploy with tags
ansible-playbook -i inventory.ini deployment/ansible/deploy.yml \
  --tags install-dependencies,deploy \
  -v
```

### Step 4: Verify Deployment

```bash
# Check service status
ansible telemetry_hosts -i inventory.ini -m systemd -a "name=telemetry-streaming state=started"

# Check health
ansible telemetry_hosts -i inventory.ini -m uri \
  -a "url=http://localhost:8080/health"

# Check logs
ansible telemetry_hosts -i inventory.ini -m command \
  -a "journalctl -u telemetry-streaming -n 50"
```

## Rolling Updates

### Kubernetes

```bash
# Update image
kubectl set image deployment/telemetry-streaming \
  telemetry-streaming=myregistry.azurecr.io/telemetry-streaming:1.1.0 \
  -n telemetry

# Wait for rollout
kubectl rollout status deployment/telemetry-streaming -n telemetry

# Rollback if needed
kubectl rollout undo deployment/telemetry-streaming -n telemetry
```

### Machine (Ansible)

```bash
# Create rolling update playbook
cat > rolling-update.yml <<EOF
---
- name: Rolling Update Telemetry Streaming
  hosts: telemetry_hosts
  serial: 1  # Update one host at a time
  become: yes
  tasks:
    - name: Download new binary
      get_url:
        url: "https://releases.example.com/telemetry-streamer-1.1.0"
        dest: /tmp/telemetry-streamer-new
        mode: '0755'
    
    - name: Stop service
      systemd:
        name: telemetry-streaming
        state: stopped
    
    - name: Backup old binary
      copy:
        src: /opt/telemetry-streaming/telemetry-streamer
        dest: /opt/telemetry-streaming/telemetry-streamer.backup
        remote_src: yes
    
    - name: Install new binary
      copy:
        src: /tmp/telemetry-streamer-new
        dest: /opt/telemetry-streaming/telemetry-streamer
        mode: '0755'
        remote_src: yes
    
    - name: Start service
      systemd:
        name: telemetry-streaming
        state: started
    
    - name: Wait for health
      uri:
        url: "http://localhost:8080/health"
        status_code: 200
      retries: 5
      delay: 10
EOF

# Run rolling update
ansible-playbook -i inventory.ini rolling-update.yml
```

## Scaling

### Kubernetes Horizontal Scaling

```bash
# Manual scale
kubectl scale deployment telemetry-streaming \
  --replicas=5 \
  -n telemetry

# Automatic scaling
kubectl autoscale deployment telemetry-streaming \
  --min=3 --max=10 \
  --cpu-percent=80 \
  -n telemetry

# Check HPA
kubectl get hpa -n telemetry
```

### Machine Scaling

For machine deployments, replicate the deployment process across multiple hosts using Ansible groups.

## Monitoring and Logging

### Kubernetes

```bash
# Real-time logs
kubectl logs -n telemetry -l app=telemetry-streaming -f

# Metrics
kubectl port-forward -n telemetry svc/telemetry-streaming 8080:8080
curl http://localhost:8080/metrics

# Pod events
kubectl describe pod <pod-name> -n telemetry

# Resource usage
kubectl top pods -n telemetry
```

### Machine

```bash
# Service logs
sudo journalctl -u telemetry-streaming -f

# System metrics
top -p $(pgrep -f telemetry-streamer)

# Network connections
sudo netstat -tulnp | grep telemetry-streamer
```

## Troubleshooting

### Issue: Pods not starting

```bash
# Check pod status
kubectl describe pod <pod-name> -n telemetry

# Check events
kubectl get events -n telemetry

# Common solutions:
# 1. Image not found: Check image registry and credentials
# 2. PVC pending: Check storage provisioner
# 3. Memory limit: Increase resource limits
```

### Issue: High error rate

```bash
# Check broker connectivity
kubectl exec -it <pod-name> -n telemetry -- \
  nc -zv kafka-broker-0.kafka 9092

# Check logs for errors
kubectl logs <pod-name> -n telemetry | grep ERROR

# Solutions:
# - Verify broker is running
# - Check network policies
# - Increase MAX_RETRIES
```

### Issue: Performance degradation

```bash
# Check resource usage
kubectl top pods -n telemetry

# Check metrics
curl http://<pod-ip>:8080/metrics

# Solutions:
# - Increase replicas
# - Increase resource limits
# - Optimize batch size
```

## Cleanup

### Kubernetes

```bash
# Delete Helm release
helm uninstall telemetry-streaming -n telemetry

# Delete namespace
kubectl delete namespace telemetry

# Delete PVCs
kubectl delete pvc -n telemetry --all
```

### Machine

```bash
# Stop service
sudo systemctl stop telemetry-streaming

# Disable service
sudo systemctl disable telemetry-streaming

# Remove files
sudo rm -rf /opt/telemetry-streaming
sudo rm -rf /var/lib/telemetry
```
