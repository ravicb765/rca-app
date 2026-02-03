# RCA-App Quick Reference Card

## What is RCA-App?

RCA-App is an open-source observability and APM platform with AI-powered root cause analysis. It automatically collects metrics, logs, traces, and profiles using eBPF technology, then uses machine learning to identify and explain issues.

## Key Features

✅ **Zero-Instrumentation** - No code changes required  
✅ **AI Root Cause Analysis** - Automated incident diagnosis  
✅ **Service Map** - Real-time dependency visualization  
✅ **Unified Platform** - Metrics, logs, traces, profiles in one place  
✅ **Built-in Inspections** - 80%+ automated issue detection  
✅ **Cost Monitoring** - Track cloud costs per service  

## Architecture Components

```
┌─────────────────────────────────────────────────┐
│  RCA-App Platform                               │
├─────────────────────────────────────────────────┤
│  Frontend (Vue/React)                           │
│  Backend (Go)                                   │
│  ML Service (Python)                            │
├─────────────────────────────────────────────────┤
│  Storage Layer                                  │
│  - ClickHouse (logs, traces, profiles)         │
│  - Prometheus (metrics)                         │
│  - Redis (cache)                                │
├─────────────────────────────────────────────────┤
│  Collection Agents                              │
│  - Node Agent (DaemonSet, eBPF)                │
│  - Cluster Agent (Kubernetes metadata)         │
└─────────────────────────────────────────────────┘
```

## Quick Start

### Prerequisites
- Docker & Docker Compose
- Kubernetes cluster (for production)
- Linux kernel 5.4+ with eBPF support

### Local Development

```bash
# Clone the repository
git clone https://github.com/ravicb765/rca-app
cd rca-app

# Start all services
docker-compose up -d

# Access the UI
open http://localhost:8080

# View logs
docker-compose logs -f
```

### Kubernetes Deployment

```bash
# Create namespace
kubectl create namespace observability

# Deploy RCA-App
kubectl apply -f k8s/

# Verify deployment
kubectl get pods -n observability
```

## API Endpoints

### Service Map
```bash
GET /api/v1/servicemap
GET /api/v1/applications
GET /api/v1/applications/:id
```

### Health & Inspections
```bash
GET /api/v1/applications/:id/health
GET /api/v1/applications/:id/inspections
```

### Observability Data
```bash
GET /api/v1/metrics/query
GET /api/v1/logs
GET /api/v1/traces
GET /api/v1/traces/:trace_id
```

### AI Analysis
```bash
POST /api/v1/analyze
{
  "application_id": "app-name",
  "start_time": "2024-01-01T00:00:00Z",
  "end_time": "2024-01-01T01:00:00Z"
}
```

## Configuration

### Environment Variables

```bash
# Server
CLICKHOUSE_URL=clickhouse:9000
PROMETHEUS_URL=http://prometheus:9090
ML_SERVICE_URL=http://ml-service:5000
REDIS_URL=redis:6379

# Node Agent
RCA_APP_ENDPOINT=http://rca-app-server:8080

# ML Service
OPENAI_API_KEY=sk-...  # Optional, for GPT integration
```

### Prometheus Config

```yaml
global:
  scrape_interval: 15s

scrape_configs:
  - job_name: 'node-agents'
    static_configs:
      - targets: ['node-agent:9100']
```

### ClickHouse Schema

```sql
-- Logs
CREATE TABLE logs (
    timestamp DateTime64(9),
    application String,
    level String,
    message String,
    pattern_id UInt64
) ENGINE = MergeTree()
ORDER BY (application, timestamp);

-- Traces
CREATE TABLE traces (
    trace_id String,
    span_id String,
    operation_name String,
    start_time DateTime64(9),
    duration UInt64
) ENGINE = MergeTree()
ORDER BY (trace_id, start_time);
```

## Common Commands

### Docker Compose

```bash
# Start services
docker-compose up -d

# Stop services
docker-compose down

# View logs
docker-compose logs -f [service_name]

# Rebuild images
docker-compose build

# Scale ML service
docker-compose up -d --scale ml-service=3
```

### Kubernetes

```bash
# Deploy node agent
kubectl apply -f k8s/node-agent-daemonset.yaml

# Check agent status
kubectl get pods -n observability -l app=node-agent

# View agent logs
kubectl logs -n observability -l app=node-agent -f

# Port forward to UI
kubectl port-forward -n observability svc/rca-app-server 8080:8080
```

### Building from Source

```bash
# Node Agent
cd node-agent
go build -o node-agent main.go
sudo ./node-agent

# Server
cd server
go build -o server main.go
./server

# ML Service
cd ml-service
pip install -r requirements.txt
python main.py
```

## Predefined Inspections

| Inspection | Category | Threshold |
|------------|----------|-----------|
| High Error Rate | Availability | error_rate > 1% |
| Slow Response | Performance | p95_latency > baseline * 1.5 |
| Memory Leak | Resources | memory continuously increasing |
| DB Pool Exhaustion | Database | active_connections > 90% |
| High CPU Usage | Resources | cpu_usage > 80% |
| Network Latency | Network | network_latency > 50ms |

## ML Models

### Anomaly Detection
- **Algorithm**: Isolation Forest
- **Input**: Error rate, latency, CPU, memory, request rate
- **Output**: Normal (1) or Anomaly (-1)

### Incident Classification
- **Algorithm**: Random Forest
- **Categories**: database_slowdown, memory_leak, network_issue, high_traffic, dependency_failure
- **Output**: Category + confidence score

### Root Cause Analysis
- **Method**: LLM-based (GPT-4 or local model)
- **Output**: Root cause, reasoning, remediation steps

## Troubleshooting

### Agent Not Collecting Data
```bash
# Check eBPF support
uname -r  # Should be 5.4+
ls /sys/kernel/debug/tracing

# Check agent logs
docker logs rca-app-node-agent

# Verify permissions
# Agent needs privileged mode for eBPF
```

### High Memory Usage
```bash
# Check ClickHouse retention
# Set TTL to reduce storage

# Adjust Prometheus retention
--storage.tsdb.retention.time=7d

# Enable data compression
```

### ML Service Not Responding
```bash
# Check service health
curl http://localhost:5000/health

# Verify dependencies
pip list | grep -E "sklearn|numpy|pandas"

# Check logs
docker logs rca-app-ml-service
```

## Performance Tips

### Node Agent
- Use adaptive sampling for high-traffic apps
- Batch events before sending
- Enable efficient eBPF map structures

### Storage
- Use materialized views in ClickHouse
- Enable query result caching
- Set appropriate retention policies

### Queries
- Use time-based partitioning
- Create indexes on frequently queried fields
- Implement incremental aggregation

## Security Best Practices

### Network
```yaml
# Kubernetes Network Policy
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: rca-app-policy
spec:
  podSelector:
    matchLabels:
      app: rca-app-server
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - podSelector:
        matchLabels:
          app: node-agent
```

### Authentication
- Enable OAuth2/OIDC for UI
- Use API keys for programmatic access
- Implement RBAC for multi-tenancy

### Data
- Enable TLS for all communication
- Encrypt data at rest in ClickHouse
- Mask sensitive information in logs

## Development Roadmap

### Phase 1: MVP ✅
- [x] Basic eBPF collection
- [x] Service map generation
- [x] Simple web UI
- [ ] Complete eBPF implementation

### Phase 2: Core Features 🚧
- [ ] Log pattern clustering
- [ ] Distributed tracing
- [ ] Continuous profiling
- [ ] Health inspections

### Phase 3: AI Integration 📋
- [ ] Train ML models
- [ ] LLM integration
- [ ] Automated RCA

### Phase 4: Enterprise 📋
- [ ] SLO tracking
- [ ] Cost monitoring
- [ ] Multi-tenancy
- [ ] Advanced alerting

## Resources

- **Documentation**: See `rca-app-guide.md`
- **GitHub**: https://github.com/ravicb765/rca-app
- **eBPF Learning**: https://ebpf.io/
- **Inspiration**: Coroot (https://github.com/coroot/coroot)

## Support

- **Issues**: https://github.com/ravicb765/rca-app/issues
- **Discussions**: https://github.com/ravicb765/rca-app/discussions

---

## Cheat Sheet

```bash
# Start everything
docker-compose up -d

# Check status
docker-compose ps

# Access UI
open http://localhost:8080

# Test ML service
curl -X POST http://localhost:5000/analyze \
  -H "Content-Type: application/json" \
  -d '{"application_id":"test","metrics":{"error_rate":0.05}}'

# Query metrics
curl "http://localhost:8080/api/v1/metrics/query?query=up"

# View service map
curl http://localhost:8080/api/v1/servicemap | jq

# Stop everything
docker-compose down
```

## License

Apache License 2.0

---

Built with ❤️ for the observability community
