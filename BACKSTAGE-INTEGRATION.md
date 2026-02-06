# RCA-App with Backstage Integration Guide

## Overview

This guide explains how to integrate **Backstage** as the web UI for RCA-App, leveraging its powerful Kubernetes plugins, software catalog, and extensible plugin architecture to create a comprehensive observability platform.

## Why Backstage for RCA-App?

### Built-in Kubernetes Integration
- **@backstage/plugin-kubernetes**: Native Kubernetes monitoring
- **Real-time pod/deployment visibility**
- **Multi-cluster support out-of-the-box**
- **Health status indicators**

### Extensible Plugin Ecosystem
- **100+ community plugins** available
### Unified Developer Portal
- **Spotify's Backstage**: Built on React for a modern, extensible developer experience.
- **Service catalog** for managing all applications
- **Software templates** for standardization
- **TechDocs** for documentation

### Developer-Friendly
- **Single pane of glass** for all services
- **Role-based access control**
- **Golden paths** for common tasks
- **Self-service capabilities**

## Architecture with Backstage

```
┌─────────────────────────────────────────────────────────────┐
│                    Backstage Frontend                       │
│  ┌──────────────┬──────────────┬──────────────────────┐    │
│  │  Software    │  Kubernetes  │  RCA-App             │    │
│  │  Catalog     │  Plugin      │  Custom Plugins      │    │
│  └──────────────┴──────────────┴──────────────────────┘    │
│  ┌──────────────┬──────────────┬──────────────────────┐    │
│  │  Service Map │  Metrics     │  Logs & Traces       │    │
│  │  Visualizer  │  Dashboards  │  Viewer              │    │
│  └──────────────┴──────────────┴──────────────────────┘    │
└───────────────────────────┬─────────────────────────────────┘
                            │
┌───────────────────────────┴─────────────────────────────────┐
│              Backstage Backend + RCA-App Backend            │
│  ┌──────────────┬──────────────┬──────────────────────┐    │
│  │  Catalog     │  Kubernetes  │  RCA-App API         │    │
│  │  Backend     │  Backend     │  Integration         │    │
│  └──────────────┴──────────────┴──────────────────────┘    │
└───────────────────────────┬─────────────────────────────────┘
                            │
┌───────────────────────────┴─────────────────────────────────┐
│                   RCA-App Core Services                     │
│  ┌──────────────┬──────────────┬──────────────────────┐    │
│  │  Node Agent  │  ML Service  │  Inspection Engine   │    │
│  │  (eBPF)      │  (AI/ML)     │  (Health Checks)     │    │
│  └──────────────┴──────────────┴──────────────────────┘    │
└─────────────────────────────────────────────────────────────┘
                            │
┌───────────────────────────┴─────────────────────────────────┐
│                    Storage Layer                            │
│   ClickHouse | Prometheus | Redis                           │
└─────────────────────────────────────────────────────────────┘
```

## Custom Backstage Plugins for RCA-App

We'll create custom plugins to integrate RCA-App's unique features:

### 1. RCA-App Service Map Plugin
**Package**: `@rca-app/plugin-service-map`

Displays the real-time service dependency graph with health indicators.

### 2. RCA-App AI Analysis Plugin
**Package**: `@rca-app/plugin-ai-analysis`

Shows AI-powered root cause analysis results and incident insights.

### 3. RCA-App Inspections Plugin
**Package**: `@rca-app/plugin-inspections`

Displays predefined inspection results and SLO tracking.

### 4. RCA-App Profiling Plugin
**Package**: `@rca-app/plugin-profiling`

Shows continuous profiling data with flamegraphs.

### 5. RCA-App Cost Monitoring Plugin
**Package**: `@rca-app/plugin-cost-monitoring`

Displays cloud cost attribution per service.

## Setup Instructions

### Step 1: Create Backstage App

```bash
npx @backstage/create-app@latest

# Choose a name (e.g., rca-app-portal)
cd rca-app-portal
```

### Step 2: Install Core Plugins

```bash
# Kubernetes plugins
yarn --cwd packages/app add @backstage/plugin-kubernetes
yarn --cwd packages/backend add @backstage/plugin-kubernetes-backend

# Other useful plugins
yarn --cwd packages/app add @backstage/plugin-prometheus
yarn --cwd packages/app add @backstage/plugin-grafana
yarn --cwd packages/app add @backstage/plugin-cost-insights

# For Dynatrace integration (optional)
yarn --cwd packages/app add @dynatrace/backstage-plugin-dql
yarn --cwd packages/backend add @dynatrace/backstage-plugin-dql-backend
```

### Step 3: Configure Kubernetes Plugin

**app-config.yaml**:
```yaml
kubernetes:
  serviceLocatorMethod:
    type: 'multiTenant'
  clusterLocatorMethods:
    - type: 'config'
      clusters:
        - url: ${K8S_CLUSTER_URL}
          name: ${K8S_CLUSTER_NAME}
          authProvider: 'serviceAccount'
          skipTLSVerify: false
          serviceAccountToken: ${K8S_SERVICE_ACCOUNT_TOKEN}
          
  # Custom resources for RCA-App
  customResources:
    - group: 'rca-app.io'
      apiVersion: 'v1'
      plural: 'servicemaps'
    - group: 'rca-app.io'
      apiVersion: 'v1'
      plural: 'inspections'
```

### Step 4: Create Custom RCA-App Plugins

#### Plugin 1: Service Map Plugin

**Create plugin structure**:
```bash
cd packages
yarn new --select plugin
# Name: @rca-app/plugin-service-map
```

**plugins/service-map/src/components/ServiceMapCard/ServiceMapCard.tsx**:
```typescript
import React, { useEffect, useState } from 'react';
import { InfoCard } from '@backstage/core-components';
import { useEntity } from '@backstage/plugin-catalog-react';
import Cytoscape from 'cytoscape';
import CytoscapeComponent from 'react-cytoscapejs';

export const ServiceMapCard = () => {
  const { entity } = useEntity();
  const [serviceMap, setServiceMap] = useState(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    // Fetch service map from RCA-App backend
    const fetchServiceMap = async () => {
      try {
        const response = await fetch(
          `http://rca-app-server:8080/api/v1/servicemap/${entity.metadata.name}`
        );
        const data = await response.json();
        setServiceMap(data);
      } catch (error) {
        console.error('Failed to fetch service map:', error);
      } finally {
        setLoading(false);
      }
    };

    fetchServiceMap();
  }, [entity]);

  if (loading) return <div>Loading service map...</div>;

  // Transform service map data to Cytoscape format
  const elements = serviceMap?.connections?.map(conn => ({
    data: {
      id: `${conn.source}-${conn.destination}`,
      source: conn.source,
      target: conn.destination,
      label: `${conn.requestRate} req/s`,
      errorRate: conn.errorRate,
    }
  })) || [];

  const nodes = Array.from(new Set([
    ...elements.map(e => e.data.source),
    ...elements.map(e => e.data.target)
  ])).map(id => ({
    data: { id, label: id }
  }));

  return (
    <InfoCard title="Service Dependencies">
      <CytoscapeComponent
        elements={[...nodes, ...elements]}
        style={{ width: '100%', height: '400px' }}
        layout={{ name: 'cose' }}
        stylesheet={[
          {
            selector: 'node',
            style: {
              'background-color': '#666',
              'label': 'data(label)',
            }
          },
          {
            selector: 'edge',
            style: {
              'width': 3,
              'line-color': '#ccc',
              'target-arrow-color': '#ccc',
              'target-arrow-shape': 'triangle',
              'curve-style': 'bezier',
              'label': 'data(label)',
            }
          }
        ]}
      />
    </InfoCard>
  );
};
```

#### Plugin 2: AI Analysis Plugin

**plugins/ai-analysis/src/components/AIAnalysisCard/AIAnalysisCard.tsx**:
```typescript
import React, { useState } from 'react';
import {
  InfoCard,
  Progress,
  StructuredMetadataTable,
} from '@backstage/core-components';
import { Button, Chip } from '@material-ui/core';
import { useEntity } from '@backstage/plugin-catalog-react';
import { Alert } from '@material-ui/lab';

export const AIAnalysisCard = () => {
  const { entity } = useEntity();
  const [analysis, setAnalysis] = useState(null);
  const [loading, setLoading] = useState(false);

  const runAnalysis = async () => {
    setLoading(true);
    try {
      const response = await fetch(
        'http://rca-app-server:8080/api/v1/analyze',
        {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            application_id: entity.metadata.name,
            start_time: new Date(Date.now() - 3600000).toISOString(),
            end_time: new Date().toISOString(),
          }),
        }
      );
      const data = await response.json();
      setAnalysis(data);
    } catch (error) {
      console.error('Analysis failed:', error);
    } finally {
      setLoading(false);
    }
  };

  return (
    <InfoCard
      title="AI Root Cause Analysis"
      action={
        <Button
          variant="contained"
          color="primary"
          onClick={runAnalysis}
          disabled={loading}
        >
          Analyze Now
        </Button>
      }
    >
      {loading && <Progress />}
      
      {analysis && (
        <>
          <Alert severity={analysis.confidence > 0.8 ? 'info' : 'warning'}>
            <strong>Root Cause:</strong> {analysis.root_cause}
            <Chip
              label={`${(analysis.confidence * 100).toFixed(0)}% confidence`}
              size="small"
              style={{ marginLeft: 8 }}
            />
          </Alert>

          <StructuredMetadataTable
            metadata={{
              'Reasoning': analysis.reasoning,
              'Symptoms': analysis.symptoms?.join(', '),
              'Detected At': new Date(analysis.timestamp).toLocaleString(),
            }}
          />

          <div style={{ marginTop: 16 }}>
            <h4>Remediation Steps:</h4>
            <ol>
              {analysis.remediation?.map((step, idx) => (
                <li key={idx}>{step}</li>
              ))}
            </ol>
          </div>
        </>
      )}
      
      {!analysis && !loading && (
        <p>Click "Analyze Now" to run AI-powered root cause analysis</p>
      )}
    </InfoCard>
  );
};
```

#### Plugin 3: Inspections Plugin

**plugins/inspections/src/components/InspectionsCard/InspectionsCard.tsx**:
```typescript
import React, { useEffect, useState } from 'react';
import {
  InfoCard,
  Table,
  TableColumn,
} from '@backstage/core-components';
import { Chip } from '@material-ui/core';
import { useEntity } from '@backstage/plugin-catalog-react';
import ErrorIcon from '@material-ui/icons/Error';
import WarningIcon from '@material-ui/icons/Warning';
import CheckCircleIcon from '@material-ui/icons/CheckCircle';

interface Inspection {
  name: string;
  category: string;
  severity: 'critical' | 'warning' | 'info';
  status: 'pass' | 'fail';
  description: string;
  remediation: string;
}

export const InspectionsCard = () => {
  const { entity } = useEntity();
  const [inspections, setInspections] = useState<Inspection[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const fetchInspections = async () => {
      try {
        const response = await fetch(
          `http://rca-app-server:8080/api/v1/applications/${entity.metadata.name}/inspections`
        );
        const data = await response.json();
        setInspections(data.inspections || []);
      } catch (error) {
        console.error('Failed to fetch inspections:', error);
      } finally {
        setLoading(false);
      }
    };

    fetchInspections();
    const interval = setInterval(fetchInspections, 30000); // Refresh every 30s
    return () => clearInterval(interval);
  }, [entity]);

  const getSeverityIcon = (severity: string) => {
    switch (severity) {
      case 'critical':
        return <ErrorIcon color="error" />;
      case 'warning':
        return <WarningIcon color="secondary" />;
      default:
        return <CheckCircleIcon color="primary" />;
    }
  };

  const columns: TableColumn[] = [
    {
      title: 'Status',
      field: 'status',
      render: (row: Inspection) =>
        row.status === 'pass' ? (
          <CheckCircleIcon style={{ color: 'green' }} />
        ) : (
          getSeverityIcon(row.severity)
        ),
    },
    { title: 'Inspection', field: 'name' },
    { title: 'Category', field: 'category' },
    {
      title: 'Severity',
      field: 'severity',
      render: (row: Inspection) => (
        <Chip
          label={row.severity}
          color={row.severity === 'critical' ? 'secondary' : 'default'}
          size="small"
        />
      ),
    },
    { title: 'Description', field: 'description' },
  ];

  const failedInspections = inspections.filter(i => i.status === 'fail');

  return (
    <InfoCard
      title={`Health Inspections (${failedInspections.length} issues)`}
    >
      <Table
        options={{ paging: false, search: false }}
        columns={columns}
        data={inspections}
        isLoading={loading}
      />
    </InfoCard>
  );
};
```

### Step 5: Register Plugins in Entity Page

**packages/app/src/components/catalog/EntityPage.tsx**:
```typescript
import { ServiceMapCard } from '@rca-app/plugin-service-map';
import { AIAnalysisCard } from '@rca-app/plugin-ai-analysis';
import { InspectionsCard } from '@rca-app/plugin-inspections';
import { EntityKubernetesContent } from '@backstage/plugin-kubernetes';

const serviceEntityPage = (
  <EntityLayout>
    <EntityLayout.Route path="/" title="Overview">
      <Grid container spacing={3}>
        <Grid item md={6}>
          <EntityAboutCard variant="gridItem" />
        </Grid>
        <Grid item md={6}>
          <InspectionsCard />
        </Grid>
        <Grid item md={12}>
          <ServiceMapCard />
        </Grid>
      </Grid>
    </EntityLayout.Route>

    <EntityLayout.Route path="/kubernetes" title="Kubernetes">
      <EntityKubernetesContent refreshIntervalMs={30000} />
    </EntityLayout.Route>

    <EntityLayout.Route path="/ai-analysis" title="AI Analysis">
      <Grid container spacing={3}>
        <Grid item md={12}>
          <AIAnalysisCard />
        </Grid>
      </Grid>
    </EntityLayout.Route>

    <EntityLayout.Route path="/ci-cd" title="CI/CD">
      <EntitySwitch>
        <EntitySwitch.Case if={isGithubActionsAvailable}>
          <EntityGithubActionsContent />
        </EntitySwitch.Case>
      </EntitySwitch>
    </EntityLayout.Route>
  </EntityLayout>
);
```

### Step 6: Create Component Catalog Entries

**catalog-info.yaml** (example for a service):
```yaml
apiVersion: backstage.io/v1alpha1
kind: Component
metadata:
  name: payment-service
  description: Payment processing microservice
  annotations:
    # Backstage Kubernetes plugin
    backstage.io/kubernetes-id: payment-service
    backstage.io/kubernetes-namespace: production
    
    # RCA-App specific annotations
    rca-app.io/slo-availability: '99.9'
    rca-app.io/slo-latency: '200ms'
    rca-app.io/enable-ai-analysis: 'true'
    rca-app.io/cost-tracking: 'enabled'
    
    # Links
    prometheus.io/rule: 'payment-service-alerts'
    grafana/dashboard-selector: 'payment-service'
    
  tags:
    - go
    - microservice
    - payments
  links:
    - url: https://github.com/myorg/payment-service
      title: Source Code
      icon: github
    - url: https://grafana.example.com/d/payment-service
      title: Grafana Dashboard
      icon: dashboard

spec:
  type: service
  lifecycle: production
  owner: payments-team
  system: payments-platform
  
  providesApis:
    - payment-api
  
  consumesApis:
    - user-api
    - billing-api
```

### Step 7: Configure Backend Integration

**packages/backend/src/plugins/rca-app.ts**:
```typescript
import { createRouter } from '@backstage/plugin-proxy-backend';
import { Router } from 'express';
import { PluginEnvironment } from '../types';

export default async function createPlugin(
  env: PluginEnvironment,
): Promise<Router> {
  return await createRouter({
    logger: env.logger,
    config: env.config,
    discovery: env.discovery,
    routes: {
      '/rca-app': {
        target: 'http://rca-app-server:8080',
        pathRewrite: { '^/api/proxy/rca-app': '/' },
        allowedMethods: ['GET', 'POST'],
        allowedHeaders: ['Content-Type', 'Authorization'],
      },
    },
  });
}
```

**packages/backend/src/index.ts**:
```typescript
import rcaApp from './plugins/rca-app';

async function main() {
  // ... existing code
  
  const rcaAppEnv = useHotMemoize(module, () => createEnv('rca-app'));
  apiRouter.use('/rca-app', await rcaApp(rcaAppEnv));
  
  // ... rest of code
}
```

### Step 8: Add RCA-App Backend as Microservice

**app-config.yaml**:
```yaml
proxy:
  endpoints:
    '/rca-app':
      target: 'http://rca-app-server:8080'
      changeOrigin: true
      pathRewrite:
        '^/api/proxy/rca-app': '/'

# RCA-App specific configuration
rcaApp:
  baseUrl: 'http://rca-app-server:8080'
  apiKey: ${RCA_APP_API_KEY}
  
  # AI/ML Service
  mlService:
    url: 'http://rca-app-ml-service:5000'
  
  # Storage
  clickhouse:
    host: 'clickhouse'
    port: 9000
    database: 'observability'
  
  prometheus:
    url: 'http://prometheus:9090'
```

## Deployment

### Docker Compose for Development

**docker-compose.backstage.yml**:
```yaml
version: '3.8'

services:
  backstage:
    build:
      context: ./rca-app-portal
      dockerfile: packages/backend/Dockerfile
    ports:
      - "3000:3000"
      - "7007:7007"
    environment:
      - NODE_ENV=development
      - POSTGRES_HOST=postgres
      - POSTGRES_USER=backstage
      - POSTGRES_PASSWORD=backstage
      - K8S_CLUSTER_URL=${K8S_CLUSTER_URL}
      - K8S_SERVICE_ACCOUNT_TOKEN=${K8S_TOKEN}
    depends_on:
      - postgres
      - rca-app-server
    networks:
      - rca-app-network

  postgres:
    image: postgres:15
    environment:
      - POSTGRES_USER=backstage
      - POSTGRES_PASSWORD=backstage
      - POSTGRES_DB=backstage
    volumes:
      - postgres-data:/var/lib/postgresql/data
    networks:
      - rca-app-network

  # RCA-App services (from previous docker-compose.yml)
  rca-app-server:
    build: ./rca-app/server
    ports:
      - "8080:8080"
    networks:
      - rca-app-network

  # ... other RCA-App services

volumes:
  postgres-data:

networks:
  rca-app-network:
    external: true
```

### Kubernetes Deployment

**k8s/backstage-deployment.yaml**:
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: backstage
  namespace: observability
spec:
  replicas: 1
  selector:
    matchLabels:
      app: backstage
  template:
    metadata:
      labels:
        app: backstage
    spec:
      serviceAccountName: backstage
      containers:
      - name: backstage
        image: your-registry/backstage:latest
        ports:
        - containerPort: 7007
        env:
        - name: POSTGRES_HOST
          value: postgres
        - name: POSTGRES_USER
          value: backstage
        - name: POSTGRES_PASSWORD
          valueFrom:
            secretKeyRef:
              name: backstage-secrets
              key: postgres-password
---
apiVersion: v1
kind: Service
metadata:
  name: backstage
  namespace: observability
spec:
  selector:
    app: backstage
  ports:
  - port: 80
    targetPort: 7007
  type: LoadBalancer
```

## Advanced Features

### 1. Custom Software Templates

Create templates for new services that automatically register with RCA-App:

**templates/microservice-template.yaml**:
```yaml
apiVersion: scaffolder.backstage.io/v1beta3
kind: Template
metadata:
  name: microservice-with-observability
  title: Microservice with RCA-App Observability
  description: Create a new microservice with full observability setup
spec:
  owner: platform-team
  type: service

  parameters:
    - title: Service Information
      required:
        - name
        - description
      properties:
        name:
          title: Name
          type: string
        description:
          title: Description
          type: string
        owner:
          title: Owner
          type: string
          ui:field: OwnerPicker

  steps:
    - id: template
      name: Fetch Skeleton
      action: fetch:template
      input:
        url: ./skeleton
        values:
          name: ${{ parameters.name }}
          description: ${{ parameters.description }}

    - id: publish
      name: Publish to GitHub
      action: publish:github
      input:
        repoUrl: github.com?repo=${{ parameters.name }}&owner=myorg

    - id: register
      name: Register with Backstage
      action: catalog:register
      input:
        repoContentsUrl: ${{ steps.publish.output.repoContentsUrl }}
        catalogInfoPath: '/catalog-info.yaml'

    - id: register-rca-app
      name: Register with RCA-App
      action: http:backstage:request
      input:
        method: POST
        path: '/api/proxy/rca-app/api/v1/applications'
        body:
          name: ${{ parameters.name }}
          owner: ${{ parameters.owner }}
```

### 2. TechDocs Integration

Document your observability practices:

**docs/observability.md**:
```markdown
# Observability Guide

## Service Health

Our platform automatically monitors service health using RCA-App's inspections:

- **High Error Rate**: Triggers when error rate > 1%
- **High Latency**: Triggers when p95 > 500ms
- **Resource Usage**: Monitors CPU and memory

## AI Analysis

Click the "AI Analysis" tab to run root cause analysis on any incident.

## SLO Tracking

We track the following SLOs:
- Availability: 99.9%
- Latency (p95): < 200ms
```

### 3. Scorecards for Service Quality

**app-config.yaml**:
```yaml
scorecards:
  - name: Production Readiness
    description: Measures production readiness of services
    checks:
      - name: Has SLO Defined
        rule: rca-app.io/slo-availability exists
      - name: Observability Enabled
        rule: rca-app.io/enable-ai-analysis == 'true'
      - name: Cost Tracking
        rule: rca-app.io/cost-tracking == 'enabled'
      - name: No Critical Issues
        rule: inspection.critical.count == 0
```

## Benefits of Backstage + RCA-App

1. **Unified Experience**: Single UI for all observability needs
2. **Service Catalog**: Central registry of all services
3. **Self-Service**: Developers can access insights without asking SREs
4. **Standardization**: Golden paths for observability
5. **Extensibility**: Easy to add new features via plugins
6. **Community**: Large ecosystem of existing plugins
7. **Documentation**: Co-located docs with code
8. **Kubernetes Native**: Built for cloud-native environments

## Next Steps

1. **Deploy Backstage**: Follow setup instructions
2. **Create Custom Plugins**: Build RCA-App integration plugins
3. **Populate Catalog**: Register your services
4. **Configure Kubernetes**: Connect to your clusters
5. **Train Team**: Onboard developers to the platform
6. **Iterate**: Add more features based on feedback

## Resources

- **Backstage Documentation**: https://backstage.io/docs
- **Plugin Development**: https://backstage.io/docs/plugins/
- **Kubernetes Plugin**: https://backstage.io/docs/features/kubernetes/
- **Software Templates**: https://backstage.io/docs/features/software-templates/

---

**RCA-App + Backstage = Complete Developer Portal with AI-Powered Observability**
