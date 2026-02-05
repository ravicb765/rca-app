# RCA-App Backstage Portal

This is a customized [Backstage](https://backstage.io) instance integrated with the RCA-App plugins.

## Plugins Included

- **Service Map**: Visualizes the RCA-App dependency graph using Cytoscape.js.
- **AI Analysis**: Provides an interface to trigger and view Root Cause Analysis.
- **Inspections**: Displays a health dashboard for services.

## Setup

### Install Dependencies

```bash
yarn install
```

### Run Locally

```bash
yarn dev
```

The portal runs on `http://localhost:3000`.

## Configuration

See `app-config.yaml` to configure the RCA-App Backend URL.

```yaml
rcaApp:
  baseUrl: http://localhost:8080
```

## Scaffolder Templates


## Docker

### Build for Production

To build the production-ready Docker image (which serves the frontend via the backend):

```bash
docker build -t rca-app/backstage-portal:latest .
```

### Run Tests in Docker

To run the full test suite in an isolated environment:

```bash
docker build -f Dockerfile.test -t backstage-tests .
docker run backstage-tests
```
