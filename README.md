# RCA-App: Root Cause Analysis Observability Platform

RCA-App is a comprehensive observability platform designed to provide deep visibility into microservices, identify root causes of incidents using AI, and integrate seamlessly with developer portals.

## 🎯 Use Cases

### 1. Incident Response
- **Automatic Detection**: Real-time identification of performance degradation and service failures.
- **AI Root Cause Analysis**: Explains the root cause in plain English with high-confidence diagnostics.
- **Remediation Steps**: Provides immediate, actionable steps to resolve identified issues.
- **MTTR Reduction**: Significantly reduces Mean Time To Resolution by automating the investigation phase.

### 2. Proactive Monitoring
- **Early Warnings**: Anomaly detection identifies issues before they impact end-users.
- **Trend-Based Alerting**: Predictive alerts based on historical telemetry patterns.
- **Baseline Tracking**: Automatically learns and monitors "normal" system behavior.

### 3. Cost Optimization
- **Granular Attribution**: Track cloud infrastructure costs per microservice and namespace.
- **Waste Identification**: Spot underutilized resources and orphaned deployments.
- **Configuration Tuning**: Optimize Kubernetes deployment specs based on cost-to-performance data.

### 4. Performance Engineering
- **Production Profiling**: Low-overhead continuous profiling of live applications.
- **Cold-Path Detection**: Identify slow or inefficient code paths without instrumenting source code.
- **Deployment Comparison**: Compare performance characteristics across different versions.

### 5. Compliance & Audit
- **System Traceability**: Maintain a complete audit trail of system behavior and changes.
- **SLO Compliance**: Real-time tracking of Service Level Objectives and error budgets.
- **Automated Documentation**: Generate detailed root cause reports for post-mortems and audits.

---

## 🏗 Architecture

The platform consists of four main components:

1.  **[Node Agent](./node-agent)**: A lightweight eBPF-based agent that runs on every node, capturing network traffic, metrics, and profiles with low overhead.
2.  **[Backend Server](./server)**: A Go-based server that ingests telemetry, builds real-time service dependency graphs, and runs health inspections.
3.  **[ML Service](./ml-service)**: A Python service providing Anomaly Detection, Time-Series Forecasting, and LLM-based Root Cause Explanation.
4.  **[Backstage Portal](./backstage-portal)**: A developer portal integration to visualize service maps, health status, and AI insights.

## 🚀 Getting Started

### Prerequisites
- Docker & Docker Compose
- Go 1.21+
- Python 3.9+
- Node.js 18+ (for Backstage)
- Linux Kernel 5.4+ (for eBPF Node Agent)

### Quick Start (Local)

Run the entire stack using Docker Compose:

```bash
docker-compose up --build
```

This will start:
- **Server**: http://localhost:8080
- **ML Service**: http://localhost:5000
- **Backstage**: http://localhost:3000
- **Prometheus**: http://localhost:9090
- **ClickHouse**: TCP 9000
- **Jaeger**: http://localhost:16686

### Building from Source

You can build all components using the root Makefile:

```bash
make build
```

## 📂 Directory Structure

- `/node-agent`: eBPF C code and Go userspace agent (supports RPM/DEB).
- `/server`: Core API, Service Map Builder, Inspection Engine.
- `/ml-service`: Python ML models and RCA reasoning engine.
- `/backstage-portal`: Backstage app with custom plugins.
- `/deploy`: Kubernetes manifests and Helm charts.
- `/docs`: Architecture, Operations, and Technical Guides.
- `RCA-APP-QUICK-REFERENCE.md`: Quick reference card for operations.

## 🤝 Contributing

Please read the specific module READMEs for detailed development instructions.
