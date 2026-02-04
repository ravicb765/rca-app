# Cluster Agent

Small service that collects cluster-wide metadata (Kubernetes services, pods)
and provides endpoints for other services to consume.

Usage:

1. Build
   ```bash
   cd cluster-agent
   go build ./...
   ```

2. Run (developer mode)
   ```bash
   KUBECONFIG=~/.kube/config go run main.go
   ```

3. Endpoints
   - `GET /healthz` - health check
   - `GET /readyz` - readiness
   - `GET /metrics` - Prometheus metrics
   - `GET /api/v1/cluster/services` - list services
   - `GET /api/v1/cluster/pods` - list pods

## Prometheus & ServiceMonitor HOWTO
If you run Prometheus via the operator (kube-prometheus-stack), apply the `monitoring/servicemonitor-cluster-agent.yaml` manifest in the `observability` namespace to have Prometheus scrape the cluster agent's `/metrics` endpoint.

If your cluster uses RBAC restrictions, ensure the Prometheus service account has permission to `get/list/watch` `services`, `endpoints`, and `pods` in the `observability` namespace and to `get/list/watch` `servicemonitors` (see `monitoring/servicemonitor-rbac.yaml` for an example).
