# RCA-App: Root Cause Analysis Observability Platform

RCA-App is a comprehensive observability platform designed to provide deep visibility into microservices, identify root causes of incidents using AI, and integrate seamlessly with developer portals.

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

- `/node-agent`: eBPF C code and Go userspace agent.
- `/server`: Core API, Service Map Builder, Inspection Engine.
- `/ml-service`: Python ML models and RAG implementation.
- `/backstage-portal`: Backstage app with custom plugins.
- `/deploy`: Kubernetes manifests and Helm charts.
- `/docs`: Architecture and Operations documentation.

## 🤝 Contributing

Please read the specific module READMEs for detailed development instructions.
