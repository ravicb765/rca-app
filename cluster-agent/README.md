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
   - `GET /api/v1/cluster/services` - list services
   - `GET /api/v1/cluster/pods` - list pods
