# Monitoring HOWTO

This short HOWTO explains how to enable Prometheus scraping for the RCA-App components and some operational notes for RBAC and CI verification.

## Apply ServiceMonitors and probes
1. Ensure your cluster has the Prometheus Operator (kube-prometheus-stack or equivalent) installed.
2. Deploy the RCA-App manifests (namespace: `observability`):

```bash
kubectl apply -f cluster-agent/k8s/deployment.yaml
kubectl apply -f cluster-agent/k8s/service.yaml
kubectl apply -f server/k8s/deployment.yaml
kubectl apply -f server/k8s/service.yaml
```

3. Apply ServiceMonitors (requires the `monitoring.coreos.com` CRDs provided by the operator):

```bash
kubectl apply -f monitoring/servicemonitor-cluster-agent.yaml
kubectl apply -f monitoring/servicemonitor-server.yaml
```

4. Verify Prometheus discovered ServiceMonitors:

```bash
kubectl -n observability get servicemonitor
# Example output should list 'cluster-agent-sm' and 'rca-server-sm'
```

## RBAC notes (ServiceMonitor & Prometheus Operator)
- The Prometheus operator needs RBAC to read `services`, `endpoints`, and `pods` in the `observability` namespace so it can scrape targets referenced by ServiceMonitors.
- If your cluster enforces restricted RBAC, ensure the Prometheus service account (example: `prometheus-kube-prometheus-prometheus` in the `monitoring` namespace for kube-prometheus-stack) has a `ClusterRole` or `Role` granting the following verbs on the required resources:
  - apiGroups: [""], resources: ["services","endpoints","pods"], verbs: ["get","list","watch"]
  - apiGroup: ["monitoring.coreos.com"], resources: ["servicemonitors"], verbs: ["get","list","watch"]

See `monitoring/servicemonitor-rbac.yaml` for an example that can be adapted to your environment. For convenience you can generate a ClusterRoleBinding targeted to your Prometheus service account with:

```bash
SA_NAME=prometheus-kube-prometheus-prometheus SA_NAMESPACE=monitoring \
  ./monitoring/create-servicemonitor-rbac.sh | kubectl apply -f -
```

This avoids manual edits and ensures the binding is applied for the exact service account used by your Prometheus deployment.

## CI integration (optional)
We provide a manual GitHub Actions workflow `monitoring-integration.yml` that can be triggered to create a local k3d cluster, install the Prometheus operator (kube-prometheus-stack), apply the RCA-App manifests and ServiceMonitors, and check that ServiceMonitors are present. This is intended for CI environments that have docker-in-docker or k3d available and runs only when manually triggered (workflow_dispatch).

---

If you'd like, I can adapt the RBAC example to your specific Prometheus deployment (e.g., different service account name or namespace).