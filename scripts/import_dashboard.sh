#!/bin/bash
# Script to import Grafana dashboard via API

GRAFANA_URL=${GRAFANA_URL:-http://localhost:3000}
GRAFANA_USER=${GRAFANA_USER:-admin}
GRAFANA_PASS=${GRAFANA_PASS:-admin}
DASHBOARD_FILE=${1:-scripts/dashboard.json}

if [ ! -f "$DASHBOARD_FILE" ]; then
    echo "Error: Dashboard file $DASHBOARD_FILE not found."
    exit 1
fi

echo "Importing dashboard from $DASHBOARD_FILE to $GRAFANA_URL..."

# Wrap the dashboard JSON in the required structure for the import API
PAYLOAD=$(python3 -c "import json; print(json.dumps({'dashboard': json.load(open('$DASHBOARD_FILE')), 'overwrite': True}))")

response=$(curl -s -X POST -H "Content-Type: application/json" \
     -u "$GRAFANA_USER:$GRAFANA_PASS" \
     "$GRAFANA_URL/api/dashboards/db" \
     -d "$PAYLOAD")

if echo "$response" | grep -q '"status":"success"'; then
    echo "Dashboard imported successfully."
else
    echo "Failed to import dashboard. Response:"
    echo "$response"
    exit 1
fi