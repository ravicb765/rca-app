# RCA-App: Observability Platform with AI-Powered Root Cause Analysis

Note: This repository includes a CI workflow that compiles eBPF sources and (on self-hosted runners labeled `ebpf`) attempts to verify loading compiled objects. See `.github/workflows/ebpf-verify.yml` for details.

### Agent event format

Agents may post connection telemetry in multiple formats to the server `/api/v1/agent/event` endpoint:
- Top-level envelope: `{"connections":[ ... ]}`
- An array of connections: `[ {...}, {...} ]`
- A single connection object: `{...}`
- Perf/map events: `{"map":"name","data":"0x..."}` (accepted by server for forwarding to specialized consumers)

The service map builder also contains simple heuristics to classify services (e.g., name hints like `postgres`, `redis`, `mysql`, or protocol hints such as `protocol: "http"`).

A production-ready observability and APM platform with automated root cause analysis.

## Project Structure

```
rca-app/
├── node-agent/          # eBPF-based data collection agent
│   ├── main.go
│   ├── Dockerfile
│   └── go.mod
├── server/              # Main backend application
│   ├── main.go
│   ├── Dockerfile
│   └── go.mod
├── ml-service/          # AI/ML service for root cause analysis
│   ├── main.py
│   ├── Dockerfile
│   └── requirements.txt
├── backstage-portal/    # Backstage developer portal (Web UI)
│   ├── packages/
│   │   ├── app/        # Frontend application
│   │   └── backend/    # Backend API
│   └── plugins/        # Custom RCA-App plugins
│       ├── service-map/
│       ├── ai-analysis/
│       ├── inspections/
│       ├── profiling/
│       └── cost-monitoring/
├── docker-compose.yml   # Local development setup
└── README.md
```

## Features

### Core Features
- ✅ **Zero-instrumentation observability** with eBPF
- ✅ **Service map generation** from network traffic
- ✅ **Metrics collection** (Prometheus-compatible)
- ✅ **Log aggregation** with pattern clustering
- ✅ **Distributed tracing** (OpenTelemetry-compatible)
- ✅ **Continuous profiling**
- ✅ **AI-powered root cause analysis**

### Advanced Features
- 🚧 **Predefined inspections** (health checks)
- 🚧 **SLO tracking**
- 🚧 **Cost monitoring**
- 🚧 **Deployment tracking**
- 🚧 **Alerting** (Slack, PagerDuty, etc.)

## Prerequisites

- **Docker** and **Docker Compose**
- **Go** 1.21+ (for building node-agent and server)
- **Python** 3.10+ (for ML service)
- **Kubernetes** cluster (for production deployment)
- Linux kernel 5.4+ with eBPF support

## Quick Start

### 1. Clone and Setup

```bash
cd rca-app

# Create necessary directories
mkdir -p clickhouse/init.sql prometheus web-ui
```

### 2. Create Configuration Files

**prometheus/prometheus.yml**:
```yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: 'prometheus'
    static_configs:
      - targets: ['localhost:9090']

  - job_name: 'node-agents'
    static_configs:
      - targets: ['localhost:9100']
```

**clickhouse/init.sql**:
```sql
CREATE DATABASE IF NOT EXISTS observability;

USE observability;

-- Logs table
CREATE TABLE IF NOT EXISTS logs (
    timestamp DateTime64(9),
    application String,
    instance String,
    level String,
    message String,
    pattern_id UInt64,
    attributes Map(String, String)
) ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(timestamp)
ORDER BY (application, timestamp);

-- Traces table
CREATE TABLE IF NOT EXISTS traces (
    trace_id String,
    span_id String,
    parent_span_id String,
    operation_name String,
    start_time DateTime64(9),
    duration UInt64,
    application String
) ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(start_time)
ORDER BY (trace_id, start_time);
```

### 3. Build and Run

```bash
# Start all services
docker-compose up -d

# Check logs
docker-compose logs -f

# Access the UI
open http://localhost:8080
```

### 4. Test the Services

**Test ML Service**:
```bash
curl -X POST http://localhost:5000/analyze \
  -H "Content-Type: application/json" \
  -d '{
    "application_id": "test-app",
    "metrics": {
      "error_rate": 0.05,
      "latency_p95": 1200,
      "cpu_usage": 0.85,
      "memory_usage": 0.75
    }
  }'
```

**Test Server API**:
```bash
# Get service map
curl http://localhost:8080/api/v1/servicemap

# List applications
curl http://localhost:8080/api/v1/applications
```

## Development

**Developer guide:** For detailed developer workflows, eBPF debugging tips, CI/test notes and PR conventions see `rca-app-guide.md` (developer guide) — e.g., `less rca-app-guide.md` or open the file in your editor.

### Node Agent (Go)

```bash
cd node-agent

# Initialize Go module
go mod init github.com/ravicb765/rca-app/node-agent

# Add dependencies
go get github.com/cilium/ebpf
go get github.com/prometheus/client_golang/prometheus

# Build
go build -o node-agent main.go

# Run (requires sudo for eBPF)
sudo ./node-agent
```
### Server (Go)

```bash
cd server

# Initialize Go module
go mod init github.com/ravicb765/rca-app/server

# Add dependencies
go get github.com/gin-gonic/gin

# Build
go build -o server main.go

# Run
./server
```

## Monitoring quick HOWTO
If you operate Prometheus via the Operator (kube-prometheus-stack), apply the ServiceMonitor manifests to let Prometheus discover RCA-App metrics:

```bash
kubectl apply -f monitoring/servicemonitor-cluster-agent.yaml
kubectl apply -f monitoring/servicemonitor-server.yaml
```

- If your cluster enforces RBAC, adapt and apply `monitoring/servicemonitor-rbac.yaml` to ensure Prometheus has rights to `get,list,watch` the ServiceMonitor CRD and `services/endpoints/pods` in the `observability` namespace. You can generate a tailored binding with `monitoring/create-servicemonitor-rbac.sh`:

```bash
SA_NAME=prometheus-kube-prometheus-prometheus SA_NAMESPACE=monitoring \
  ./monitoring/create-servicemonitor-rbac.sh | kubectl apply -f -
```

For CI verification, there are two monitoring integration workflows:
- `.github/workflows/monitoring-integration.yml` — runs in a disposable k3d cluster on GitHub-hosted runners (manual `workflow_dispatch`).
- `.github/workflows/monitoring-integration-selfhosted.yml` — runs on a self-hosted runner (label it with `monitoring`) and can be triggered on PRs touching monitoring manifests or manually. This workflow is useful if you have a privileged runner with `kubectl`/`helm` preinstalled and want a PR gate for ServiceMonitor discovery.

### ML Service (Python)

```bash
cd ml-service

# Create virtual environment
python3 -m venv venv
source venv/bin/activate

# Install dependencies
pip install -r requirements.txt

# Run
python main.py
```

## Kubernetes Deployment

### Deploy with Helm

```bash
# Create namespace
kubectl create namespace observability

# Deploy ClickHouse
helm repo add clickhouse https://charts.clickhouse.com
helm install clickhouse clickhouse/clickhouse -n observability

# Deploy Prometheus
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm install prometheus prometheus-community/prometheus -n observability

# Deploy RCA-App components
kubectl apply -f k8s/
```

### Deploy Node Agent as DaemonSet

```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: node-agent
  namespace: observability
spec:
  selector:
    matchLabels:
      app: node-agent
  template:
    metadata:
      labels:
        app: node-agent
    spec:
      hostNetwork: true
      hostPID: true
      containers:
      - name: agent
        image: your-registry/node-agent:latest
        securityContext:
          privileged: true
        env:
        - name: RCA_APP_ENDPOINT
          value: "http://rca-app-server:8080"
```

## Architecture

```
┌─────────────────────────────────────────┐
│      Backstage Developer Portal         │
│   (Web UI with Custom Plugins)          │
│   ┌─────────────────────────────────┐   │
│   │ Service Map │ AI Analysis       │   │
│   │ Inspections │ K8s Dashboard     │   │
│   └─────────────────────────────────┘   │
└──────────────┬──────────────────────────┘
               │
┌──────────────┴──────────────────────────┐
│         RCA-App Server (Go)             │
│         - API Gateway                   │
│         - Service Map Builder           │
│         - Inspection Engine             │
└──┬────────┬────────┬─────────┬──────────┘
   │        │        │         │
   │        │        │         └───────────┐
   │        │        │                     │
┌──┴────┐ ┌┴──────┐ ┌┴────────┐  ┌────────┴────────┐
│ Click │ │Prome- │ │ Redis   │  │   ML Service    │
│ House │ │theus  │ │         │  │   (Python)      │
└───────┘ └───────┘ └─────────┘  └─────────────────┘
               │
┌──────────────┴──────────────────────────┐
│         Node Agents (DaemonSet)         │
│         - eBPF Data Collection          │
│         - Metrics Export                │
└─────────────────────────────────────────┘
```

### Component Interactions

**Backstage** serves as the frontend, consuming RCA-App APIs:
- Software Catalog manages service metadata
- Kubernetes plugin shows real-time cluster state
- Custom plugins display RCA-App insights (service map, AI analysis, inspections)

**RCA-App Server** aggregates data from agents and provides REST APIs for Backstage

**Node Agents** collect telemetry using eBPF and forward to the server

**ML Service** provides AI-powered root cause analysis

## Key Technologies

- **Backend**: Go (Gin framework)
- **ML/AI**: Python (scikit-learn, Flask)
- **eBPF**: cilium/ebpf library
- **Storage**: 
  - ClickHouse (logs, traces, profiles)
  - Prometheus (metrics)
  - Redis (caching)
- **Frontend**: Backstage (Developer Portal)
  - Built-in Kubernetes plugin
  - Custom RCA-App plugins
  - Software catalog integration

## 🎨 Web UI: Backstage Integration

RCA-App uses **Backstage** as its web interface, providing a comprehensive developer portal experience:

### Why Backstage?

✅ **Native Kubernetes Support** - Built-in plugins for K8s monitoring  
✅ **Service Catalog** - Centralized service registry  
✅ **Extensible** - Easy to add custom plugins  
✅ **Developer-Friendly** - Familiar UI for engineering teams  
✅ **100+ Plugins** - Rich ecosystem of integrations  

### Custom RCA-App Plugins

1. **Service Map Plugin** - Visualize service dependencies with health
2. **AI Analysis Plugin** - AI-powered root cause analysis interface
3. **Inspections Plugin** - Health checks and SLO tracking
4. **Profiling Plugin** - Continuous profiling with flamegraphs
5. **Cost Monitoring Plugin** - Cloud cost attribution per service

### Quick Setup with Backstage

```bash
# Create Backstage app
npx @backstage/create-app@latest

# Install Kubernetes plugin
yarn --cwd packages/app add @backstage/plugin-kubernetes
yarn --cwd packages/backend add @backstage/plugin-kubernetes-backend

# Install custom RCA-App plugins (coming soon)
yarn --cwd packages/app add @rca-app/plugin-service-map
yarn --cwd packages/app add @rca-app/plugin-ai-analysis
yarn --cwd packages/app add @rca-app/plugin-inspections
```

See [BACKSTAGE-INTEGRATION.md](../BACKSTAGE-INTEGRATION.md) for detailed setup instructions.

## Next Steps

### Phase 1: MVP (Current)
- [x] Basic project structure
- [x] Node agent skeleton
- [x] Server API skeleton
- [x] ML service skeleton
- [ ] Implement eBPF programs
- [ ] Service map generation
- [ ] Basic UI

### Phase 2: Core Features
- [ ] Log pattern clustering
- [ ] Distributed tracing
- [ ] Continuous profiling
- [ ] Health inspections

### Phase 3: AI Integration
- [ ] Train ML models
- [ ] LLM integration
- [ ] Automated RCA

### Phase 4: Advanced Features
- [ ] SLO tracking
- [ ] Cost monitoring
- [ ] Deployment tracking
- [ ] Multi-tenancy

## eBPF Development

[![eBPF CI](https://github.com/ravicb765/rca-app/actions/workflows/ebpf-ci.yml/badge.svg)](https://github.com/ravicb765/rca-app/actions/workflows/ebpf-ci.yml)

### Writing eBPF Programs

1. **Create C source file** (`node-agent/ebpf/network_tracer.c`):

```c
#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>

struct conn_event {
    __u32 saddr;
    __u32 daddr;
    __u16 sport;
    __u16 dport;
    __u64 timestamp;
};

struct {
    __uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
    __uint(key_size, sizeof(__u32));
    __uint(value_size, sizeof(__u32));
} events SEC(".maps");

SEC("kprobe/tcp_connect")
int trace_connect(struct pt_regs *ctx) {
    // Extract connection info and send to userspace
    struct conn_event event = {};
    // ... populate event
    bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, 
                          &event, sizeof(event));
    return 0;
}

char LICENSE[] SEC("license") = "GPL";
```

2. **Compile eBPF program**:

```bash
clang -O2 -target bpf -c network_tracer.c -o network_tracer.o
```

3. **Load in Go**:

```go
import "github.com/cilium/ebpf"

spec, err := ebpf.LoadCollectionSpec("network_tracer.o")
coll, err := ebpf.NewCollection(spec)
```

## Testing

### Unit Tests

```bash
# Go tests
cd server
go test ./...

# Python tests
cd ml-service
pytest tests/
```

### Integration Tests

```bash
# Start test environment
docker-compose -f docker-compose.test.yml up -d

# Run integration tests
./scripts/run-integration-tests.sh
```

## Troubleshooting

### eBPF Issues

**Error: "Operation not permitted"**
- Solution: Run with `sudo` or use privileged container

**Error: "Cannot find kernel BTF"**
- Solution: Install kernel headers or use BTF-enabled kernel

### Performance Issues

- Check agent resource usage: `docker stats rca-app-node-agent`
- Monitor ClickHouse disk usage
- Review Prometheus retention settings

## Contributing

1. Fork the repository
2. Create a feature branch
3. Commit your changes
4. Push to the branch
5. Create a Pull Request

## License

Apache License 2.0

## Resources

- [RCA-App Project](https://github.com/ravicb765/rca-app)
- [eBPF Documentation](https://ebpf.io/)
- [Prometheus Documentation](https://prometheus.io/docs/)
- [ClickHouse Documentation](https://clickhouse.com/docs/)

## Support

- GitHub Issues: [Create an issue](https://github.com/ravicb765/rca-app/issues)
- Community Discussions: [GitHub Discussions](https://github.com/ravicb765/rca-app/discussions)

---

Built with ❤️ for the observability community
