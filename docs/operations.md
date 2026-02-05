# RCA-App Operations Guide

## 1. Installation

### Kubernetes (Helm)
```bash
helm repo add rca-charts https://charts.rca-app.io
helm install rca-app rca-charts/rca-app --namespace rca-app --create-namespace
```

### Docker Compose (Local)
```bash
docker-compose up -d
```

## 2. Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `agent.cpuLimit` | CPU limit for node agent | `200m` |
| `server.replicas` | Number of backend replicas | `2` |
| `ml.openaiKey` | API Key for LLM Explainer | `""` |

## 3. Troubleshooting

### Node Agent Fails to Start
**Symptom**: `CrashLoopBackOff` on `rca-node-agent`
**Check**:
1. Verify privileged mode is enabled (required for eBPF).
2. Check kernel version (`uname -r`). Must be >= 5.4.
3. Check logs: `kubectl logs -l app=rca-node-agent`

### Missing Service Map Data
**Symptom**: Empty graph in UI.
**Check**:
1. Ensure traffic is flowing between services.
2. Verify agent connectivity to server: `curl http://rca-server:8080/health`
3. Check server logs for ingestion errors.

## 4. Maintenance
- **Backup**: Run `scripts/backup_clickhouse.sh` daily.
- **Pruning**: Data older than 7 days is auto-pruned by default. Adjust in `values.yaml`.
