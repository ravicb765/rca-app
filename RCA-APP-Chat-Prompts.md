# RCA-App Development Chat Prompts

Based on the comprehensive rc-app-guide.md, here are structured chat prompts to guide development of each component. Use these prompts with AI coding assistants (Claude, GPT-4, etc.) to build RCA-App incrementally.

---

## 📋 Table of Contents

1. [Phase 1: eBPF Node Agent Development](#phase-1-ebpf-node-agent)
2. [Phase 2: Backend Server Development](#phase-2-backend-server)
3. [Phase 3: ML Service Development](#phase-3-ml-service)
4. [Phase 4: Backstage Integration](#phase-4-backstage-integration)
5. [Phase 5: Deployment & DevOps](#phase-5-deployment-devops)

---

## Phase 1: eBPF Node Agent Development

### Prompt 1.1: Basic eBPF Network Tracer

```
I'm building an observability platform called RCA-App. I need to create an eBPF-based network tracer for a Go application.

Requirements:
- Use cilium/ebpf library in Go
- Capture TCP connection events (connect, accept, close)
- Track the following per connection:
  * Source IP and port
  * Destination IP and port
  * Connection state
  * Timestamps
  * Process ID and name
- Store events in a ring buffer for userspace processing
- Export events via a Go channel

Please provide:
1. eBPF C code for kprobes on tcp_connect and tcp_accept
2. Go code to load and manage the eBPF program
3. Event structure definitions
4. Basic event processing loop

Technical constraints:
- Linux kernel 5.4+
- Must handle high-frequency events efficiently
- Include error handling and cleanup
```

### Prompt 1.2: HTTP Request Parser

```
Building on the network tracer, I need to add HTTP request parsing capability to RCA-App's eBPF agent.

Requirements:
- Parse HTTP requests from TCP payload data
- Extract:
  * HTTP method (GET, POST, etc.)
  * URL path
  * Status code from responses
  * Request/response timestamps for latency calculation
- Associate requests with responses to calculate latency
- Support HTTP/1.1 (HTTP/2 optional for future)

Please provide:
1. eBPF code to read TCP socket buffers
2. HTTP parsing logic (method, path, status extraction)
3. Request-response correlation logic
4. Performance optimization techniques for high-throughput scenarios

Include sample output format showing parsed HTTP events.
```

### Prompt 1.3: Container Metrics Collection

```
For RCA-App, I need to collect container-level metrics using eBPF and cgroups.

Requirements:
- Collect metrics per container:
  * CPU usage (percentage and time)
  * Memory usage (RSS, cache, swap)
  * Network I/O (bytes sent/received)
  * Disk I/O (read/write bytes and operations)
- Identify containers by cgroup ID
- Map cgroup IDs to container names using /proc filesystem
- Export metrics in Prometheus format

Please provide:
1. eBPF programs for CPU and I/O tracking
2. Go code to read cgroup metrics
3. Container identification logic
4. Prometheus metrics exporter
5. Example metrics output

The agent should handle 100+ containers per node efficiently.
```

### Prompt 1.4: Continuous Profiling with eBPF

```
I need to implement continuous CPU profiling for RCA-App using eBPF stack sampling.

Requirements:
- Sample call stacks at 100 Hz frequency
- Support both kernel and user space stacks
- Symbolize stack traces using DWARF debug info
- Aggregate samples into a flamegraph-compatible format
- Target Go and compiled applications
- Minimize performance overhead (<1% CPU)

Please provide:
1. eBPF code for stack trace sampling
2. Go code to process and aggregate stack samples
3. Symbol resolution logic
4. Output format compatible with flamegraph tools
5. Performance tuning recommendations

Include handling for missing symbols gracefully.
```

---

## Phase 2: Backend Server Development

### Prompt 2.1: Service Map Builder

```
I'm building the backend server for RCA-App. I need a service map builder that creates a dependency graph from network connection data.

Requirements:
- Input: Stream of TCP connection events from eBPF agents
- Process:
  * Identify listening processes as "services"
  * Map client connections to server services
  * Build directed graph of dependencies
  * Classify service types (HTTP, gRPC, database, etc.) by port and protocol
  * Track connection statistics (request rate, error rate, latency)
- Output: JSON service map with nodes and edges

Please provide:
1. Go data structures for Service, Connection, ServiceMap
2. Algorithm to build the graph from connection events
3. Service type classification logic
4. Real-time update mechanism (handle new connections/disconnections)
5. REST API endpoint to expose the service map

The system should handle 10,000+ services and 100,000+ connections efficiently.
```

### Prompt 2.2: Inspection Engine

```
For RCA-App, I need an inspection engine that runs predefined health checks on applications.

Requirements:
- Define inspections as:
  * Name and category
  * Evaluation rule (error rate threshold, latency check, etc.)
  * Severity (critical, warning, info)
  * Remediation advice
- Support multiple inspection types:
  * Threshold-based (error rate > 1%)
  * Trend-based (memory continuously increasing)
  * Baseline comparison (latency > baseline * 1.5)
- Run inspections periodically (every 30 seconds)
- Store results with timestamps

Please provide:
1. Inspection interface and data structures
2. Built-in inspections for common issues:
   - High error rate
   - High latency
   - Memory leak detection
   - Database connection pool exhaustion
   - High CPU usage
3. Inspection executor that runs all checks
4. Result storage and querying API
5. Configuration format (YAML)

Include 5-10 example inspection definitions.
```

### Prompt 2.3: Metrics Query Engine

```
RCA-App needs a metrics query engine that aggregates time-series data from Prometheus.

Requirements:
- Query Prometheus for metrics
- Support aggregation functions (avg, max, min, sum, rate)
- Time range queries with downsampling
- Multi-dimensional queries with labels
- Cache results for 30 seconds to reduce load
- Support batch queries for dashboards

Please provide:
1. Go client for Prometheus API
2. Query builder with fluent interface
3. Caching layer using Redis
4. Aggregation functions
5. REST API endpoints:
   - /api/v1/metrics/query - instant query
   - /api/v1/metrics/query_range - range query
   - /api/v1/metrics/series - list series

Include error handling for Prometheus unavailability.
```

### Prompt 2.4: Log Pattern Clustering

```
I need to implement log pattern clustering using the Drain algorithm for RCA-App.

Requirements:
- Input: Raw log messages from containers
- Process using Drain algorithm:
  * Tokenize log messages
  * Build log tree structure (depth 4)
  * Group similar logs into clusters
  * Extract log templates (patterns)
- Assign pattern IDs to each log
- Store in ClickHouse with pattern_id for efficient querying

Please provide:
1. Drain algorithm implementation in Go or Python
2. Log tokenization and preprocessing
3. Pattern extraction logic
4. ClickHouse schema for logs with patterns
5. API to query logs by pattern

Example:
Input: "User 123 logged in from 192.168.1.1"
       "User 456 logged in from 10.0.0.1"
Pattern: "User <*> logged in from <*>"

The system should handle 100,000+ logs per second.
```

---

## Phase 3: ML Service Development

### Prompt 3.1: Anomaly Detection Model

```
For RCA-App's AI capabilities, I need an anomaly detection model using Isolation Forest.

Requirements:
- Features: error_rate, latency_p50, latency_p95, latency_p99, cpu_usage, memory_usage, request_rate
- Model: Isolation Forest with contamination=0.01
- Training: Offline on historical "normal" data
- Inference: Real-time on current metrics
- Output: Anomaly score and binary prediction (normal/anomaly)

Please provide:
1. Python code using scikit-learn
2. Training script with data preprocessing
3. Model persistence (save/load with joblib)
4. Real-time prediction API endpoint (Flask/FastAPI)
5. Feature engineering and scaling
6. Evaluation metrics (precision, recall, F1)

Include example training data generation and API usage.
```

### Prompt 3.2: Incident Classification

```
I need a multi-class classifier for RCA-App to categorize incidents into root cause types.

Requirements:
- Classes: 
  * database_slowdown
  * memory_leak
  * network_issue
  * high_traffic
  * dependency_failure
  * resource_exhaustion
  * configuration_error
- Features: Same as anomaly detection plus symptom flags
- Model: Random Forest or XGBoost
- Output: Predicted class with confidence score

Please provide:
1. Training data structure and feature engineering
2. Model training code with hyperparameter tuning
3. Cross-validation for model evaluation
4. Inference API with confidence thresholds
5. Feature importance analysis
6. Example synthetic training data generation

The model should achieve >80% accuracy on test data.
```

### Prompt 3.3: Time Series Forecasting

```
RCA-App needs time series forecasting to predict future metric values for proactive alerting.

Requirements:
- Forecast metrics (e.g., request rate, error rate) 1-24 hours ahead
- Model: Facebook Prophet or LSTM
- Handle seasonality (hourly, daily, weekly patterns)
- Detect anomalies as deviations from forecast
- Provide confidence intervals

Please provide:
1. Forecasting model implementation
2. Training on historical time series data
3. Multi-step ahead predictions
4. Anomaly detection using forecast bounds
5. API endpoint for forecast queries
6. Visualization-ready output format

Include example for CPU usage forecasting with daily patterns.
```

### Prompt 3.4: LLM-Based Root Cause Explainer

```
I need an LLM-based system for RCA-App to generate human-readable root cause explanations.

Requirements:
- Input: Incident context (metrics, logs, traces, correlations)
- Process: Use GPT-4 or claude or gemini or  qwen or deepseek or local LLM (Llama 2) with prompt engineering
- Output: 
  * Root cause summary
  * Reasoning/evidence
  * Immediate remediation steps
  * Long-term prevention recommendations
- Use RAG to retrieve similar past incidents

Please provide:
1. LangChain integration for LLM orchestration
2. Prompt template for root cause analysis
3. RAG implementation with vector database (ChromaDB)
4. Historical incident knowledge base
5. API endpoint for analysis requests
6. Confidence scoring for explanations

Include example prompt and response for a database connection pool issue.
```

---

## Phase 4: Backstage Integration

### Prompt 4.1: Service Map Backstage Plugin

```
I need to create a custom Backstage plugin for RCA-App that displays the service dependency map.

Requirements:
- Plugin name: @rca-app/plugin-service-map
- Fetch service map from RCA-App backend API
- Display using Cytoscape.js for graph visualization
- Show real-time metrics on edges (request rate, error rate, latency)
- Color-code nodes by health status
- Interactive: click node to see details

Please provide:
1. Backstage plugin scaffolding
2. React component with Cytoscape integration
3. API client to fetch service map
4. Graph styling configuration
5. Entity page integration code
6. Package.json with dependencies

The plugin should refresh every 30 seconds.
```

### Prompt 4.2: AI Analysis Backstage Plugin

```
Create a Backstage plugin for RCA-App that triggers and displays AI-powered root cause analysis.

Requirements:
- Plugin name: @rca-app/plugin-ai-analysis
- "Analyze Now" button to trigger analysis
- Display:
  * Root cause summary
  * Confidence score with visual indicator
  * Reasoning and evidence
  * Remediation steps (numbered list)
  * Timestamp
- Loading state during analysis
- Error handling if analysis fails

Please provide:
1. Plugin structure
2. React component with Material-UI
3. API integration for analysis endpoint
4. Result display component
5. Loading and error states
6. Entity page integration

Include TypeScript types for analysis response.
```

### Prompt 4.3: Inspections Backstage Plugin

```
Build a Backstage plugin to display RCA-App health inspections in a table format.

Requirements:
- Plugin name: @rca-app/plugin-inspections
- Table showing:
  * Status icon (pass/fail)
  * Inspection name
  * Category
  * Severity badge
  * Description
  * Remediation (expandable)
- Auto-refresh every 30 seconds
- Filter by status or severity
- Show count of failed inspections in header

Please provide:
1. Plugin setup
2. Table component using Backstage Table
3. API client for inspections
4. Status icons and severity badges
5. Filter controls
6. Auto-refresh mechanism

Use Backstage design system for consistency.
```

### Prompt 4.4: Software Template for Observed Services

```
Create a Backstage software template that scaffolds new microservices with RCA-App observability built-in.

Requirements:
- Template name: microservice-with-observability
- Generate:
  * Basic service code (Go/Python/Node.js options)
  * Dockerfile with proper labels
  * Kubernetes manifests with RCA-App annotations
  * catalog-info.yaml with observability metadata
  * CI/CD pipeline with health checks
- Automatically register with RCA-App on creation

Please provide:
1. Template YAML definition
2. Skeleton files for each language
3. Kubernetes manifest templates
4. catalog-info.yaml template with annotations:
   - rca-app.io/slo-availability
   - rca-app.io/slo-latency
   - rca-app.io/enable-ai-analysis
5. GitHub Actions workflow for CI/CD

The template should work with Backstage's scaffolder.
```

---

## Phase 5: Deployment & DevOps

### Prompt 5.1: Kubernetes DaemonSet for Node Agent

```
Create Kubernetes manifests to deploy RCA-App's node agent as a DaemonSet.

Requirements:
- DaemonSet configuration:
  * Run on every node
  * Privileged mode for eBPF
  * Host network and PID namespace access
  * Mount /sys and /sys/kernel/debug
- Resource limits: 200m CPU, 256Mi memory
- Environment variables for configuration
- Health checks and readiness probes
- Service account with minimal required permissions

Please provide:
1. DaemonSet YAML
2. ServiceAccount and RBAC configuration
3. ConfigMap for agent configuration
4. Service for metrics exposure
5. NetworkPolicy for security

Include deployment instructions and verification steps.
```

### Prompt 5.2: Helm Chart for RCA-App

```
Create a production-ready Helm chart for deploying the complete RCA-App platform on Kubernetes.

Requirements:
- Components to deploy:
  * Node agent (DaemonSet)
  * Cluster agent (Deployment)
  * RCA-App server (Deployment with 2+ replicas)
  * ML service (Deployment)
  * ClickHouse (StatefulSet)
  * Prometheus (use subchart)
  * Redis (use subchart)
- Values for customization:
  * Resource limits
  * Storage sizes
  * Ingress configuration
  * TLS certificates
  * Authentication settings

Please provide:
1. Chart.yaml with dependencies
2. values.yaml with sensible defaults
3. Templates for all components
4. NOTES.txt with deployment instructions
5. README.md with configuration guide

Support both dev and production configurations.
```

### Prompt 5.3: Observability Stack Integration

```
Configure RCA-App to integrate with existing observability tools (Prometheus, Grafana, Jaeger).

Requirements:
- Prometheus:
  * ServiceMonitor for scraping node agents
  * Recording rules for aggregations
  * AlertManager rules for critical issues
- Grafana:
  * Pre-built dashboards for RCA-App
  * Dashboard for service map
  * Dashboard for AI analysis results
- Jaeger:
  * Trace export from RCA-App
  * Trace correlation with logs

Please provide:
1. Prometheus ServiceMonitor YAML
2. PrometheusRule with recording and alerting rules
3. Grafana dashboard JSON (5+ panels)
4. Jaeger configuration for trace export
5. Docker Compose with full stack for testing

Include screenshots or ASCII art of dashboard layouts.
```

### Prompt 5.4: CI/CD Pipeline

```
Set up a complete CI/CD pipeline for RCA-App using GitHub Actions.

Requirements:
- Stages:
  * Lint and test (Go, Python)
  * Build Docker images
  * Scan images for vulnerabilities
  * Run integration tests
  * Deploy to staging
  * Deploy to production (manual approval)
- Multi-architecture builds (amd64, arm64)
- Semantic versioning
- Helm chart packaging and publishing

Please provide:
1. .github/workflows/ci.yml for PR checks
2. .github/workflows/release.yml for releases
3. Dockerfile optimizations (multi-stage builds)
4. Integration test suite
5. Deployment automation scripts

Include branch protection and required checks configuration.
```

---

## Advanced Prompts

### Advanced 1: Real-Time Stream Processing

```
Implement a real-time stream processing pipeline for RCA-App to handle high-volume telemetry data.

Requirements:
- Use Apache Kafka or NATS for message bus
- Process streams:
  * Metrics aggregation (1-minute rollups)
  * Log pattern matching
  * Trace assembly from spans
- Windowing and stateful operations
- Exactly-once processing semantics
- Scalable to 1M+ events/second

Please provide:
1. Kafka/NATS topic design
2. Stream processor implementation (Go or Java)
3. Stateful aggregation logic
4. Consumer group configuration
5. Backpressure handling

Include performance benchmarks and tuning guide.
```

### Advanced 2: Multi-Tenancy Implementation

```
Add multi-tenancy support to RCA-App for enterprise use.

Requirements:
- Tenant isolation:
  * Data separation in ClickHouse (separate databases)
  * Namespace-based isolation in Kubernetes
  * Authentication per tenant (OAuth2/OIDC)
- RBAC:
  * Admin, developer, viewer roles
  * Fine-grained permissions per resource
- Quotas and limits per tenant
- Cost allocation and chargeback

Please provide:
1. Tenant management API
2. Database schema for multi-tenancy
3. Authentication middleware
4. RBAC implementation
5. Resource quota enforcement
6. Billing/usage tracking

Include migration path from single-tenant to multi-tenant.
```

### Advanced 3: Edge Deployment Support

```
Extend RCA-App to support edge computing scenarios with intermittent connectivity.

Requirements:
- Edge agents:
  * Lightweight footprint (<50MB memory)
  * Local buffering during disconnection
  * Sync when connected
  * Delta compression for bandwidth efficiency
- Central aggregation:
  * Merge data from multiple edges
  * Handle clock skew
  * Conflict resolution

Please provide:
1. Edge agent implementation (optimized)
2. Local SQLite buffer
3. Sync protocol design
4. Data compression algorithms
5. Clock synchronization handling
6. Central aggregator service

Target: Support 1000+ edge nodes with 4G/LTE connectivity.
```

---

## Testing Prompts

### Testing 1: Unit Test Suite

```
Create comprehensive unit tests for RCA-App's service map builder component.

Requirements:
- Test coverage >80%
- Test cases:
  * Adding/removing services
  * Connection tracking
  * Service type classification
  * Concurrent updates
  * Edge cases (cycles, disconnected graphs)
- Use table-driven tests
- Mock external dependencies

Please provide:
1. Test file structure
2. Test helper functions
3. Mock implementations
4. Benchmark tests
5. Coverage report configuration

Use Go testing framework and testify for assertions.
```

### Testing 2: Integration Tests

```
Build integration tests for RCA-App that verify end-to-end flows.

Requirements:
- Test scenarios:
  * Agent → Server → Storage → API
  * Service map generation from live traffic
  * AI analysis triggered by incident
  * Backstage plugin data retrieval
- Use testcontainers for dependencies
- Parallel execution
- Test data fixtures

Please provide:
1. Integration test framework setup
2. Test scenarios implementation
3. Testcontainers configuration
4. Test data generation
5. CI integration

Include both happy path and error scenarios.
```

---

## Documentation Prompts

### Documentation 1: API Reference

```
Generate OpenAPI/Swagger documentation for RCA-App's REST API.

Requirements:
- Document all endpoints:
  * Service map APIs
  * Metrics query APIs
  * Logs and traces APIs
  * AI analysis APIs
  * Admin APIs
- Include:
  * Request/response schemas
  * Authentication requirements
  * Rate limits
  * Examples
- Generate interactive documentation

Please provide:
1. OpenAPI 3.0 YAML specification
2. Example requests/responses
3. Swagger UI integration
4. SDK generation configuration (Go, Python, TypeScript)

Use realistic example data.
```

### Documentation 2: Operations Guide

```
Write a comprehensive operations guide for RCA-App administrators.

Requirements:
- Cover:
  * Installation and setup
  * Configuration reference
  * Scaling guidelines
  * Backup and restore
  * Troubleshooting
  * Performance tuning
  * Security hardening
- Include runbooks for common issues
- Monitoring and alerting setup

Please provide:
1. Markdown documentation structure
2. Step-by-step procedures
3. Configuration examples
4. Troubleshooting decision trees
5. Performance tuning checklist

Target audience: SRE/DevOps engineers.
```

---

## Usage Instructions

1. **Start with Phase 1** if you're building from scratch
2. **Use prompts sequentially** within each phase
3. **Customize prompts** based on your specific needs
4. **Iterate** - refine prompts based on results
5. **Combine prompts** for complex features

## Tips for Best Results

- Be specific about your technology stack
- Include example input/output when possible
- Specify performance requirements
- Ask for error handling explicitly
- Request tests along with implementation
- Ask for documentation comments

## Example Workflow

```
Day 1: Prompts 1.1, 1.2 - Basic eBPF network tracer
Day 2: Prompt 1.3 - Container metrics
Day 3: Prompt 2.1 - Service map builder
Day 4: Prompts 2.2, 2.3 - Inspection engine and metrics
Week 2: Phase 3 - ML service development
Week 3: Phase 4 - Backstage integration
Week 4: Phase 5 - Deployment
```

---

## Additional Resources

- Reference: `rca-app-guide.md` - Comprehensive architecture guide
- Reference: `BACKSTAGE-INTEGRATION.md` - Backstage setup details
- Reference: `RCA-APP-QUICK-REFERENCE.md` - Quick command reference

---

**Note**: These prompts are designed to work with AI coding assistants like Claude, GPT-4, or GitHub Copilot. Adjust the level of detail based on the assistant's capabilities and your experience level.

**Recommended**: Start with prompts marked as foundational and iterate based on results. Each prompt is designed to produce working code that can be integrated into the larger RCA-App system.
