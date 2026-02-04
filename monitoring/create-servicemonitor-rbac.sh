#!/usr/bin/env bash
set -euo pipefail

: ${SA_NAME:?"SA_NAME must be set (service account name for Prometheus)")}
: ${SA_NAMESPACE:?"SA_NAMESPACE must be set (namespace of the Prometheus service account)")}
: ${ROLE_NAME:=prometheus-observability-reader}

cat <<EOF
# Generated ClusterRoleBinding for Prometheus ServiceAccount ${SA_NAMESPACE}/${SA_NAME}
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: ${ROLE_NAME}
rules:
  - apiGroups: ["monitoring.coreos.com"]
    resources: ["servicemonitors"]
    verbs: ["get", "list", "watch"]
  - apiGroups: [""]
    resources: ["services", "endpoints", "pods"]
    verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: ${ROLE_NAME}-binding
subjects:
  - kind: ServiceAccount
    name: ${SA_NAME}
    namespace: ${SA_NAMESPACE}
roleRef:
  kind: ClusterRole
  name: ${ROLE_NAME}
  apiGroup: rbac.authorization.k8s.io
EOF
