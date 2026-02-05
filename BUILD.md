# Build Instructions

This document provides detailed steps to compile and build the RCA-App modules.

## Prerequisites

- **Go**: Version 1.21 or higher
- **Python**: Version 3.10 or higher
- **Docker**: Version 20.10 or higher
- **Clang/LLVM**: Required for compiling eBPF programs (node-agent)
- **Make**: For running automation scripts

## 1. Node Agent (Go + eBPF)

The Node Agent consists of a Go userspace program and C-based eBPF kernel probes.

### Local Build

1.  **Compile eBPF Object**:
    ```bash
    cd node-agent/ebpf
    make
    ```
    This generates `network_tracer.o`.

2.  **Build Go Binary**:
    ```bash
    cd node-agent
    go mod tidy
    go build -o node-agent main.go
    ```

3.  **Run**:
    ```bash
    sudo ./node-agent
    ```
    *Note: Root privileges are required to load eBPF programs.*

### Docker Build

The Dockerfile handles eBPF compilation automatically.

```bash
cd node-agent
docker build -t rca-app-node-agent:latest .
```

## 2. Server (Go)

### Local Build

```bash
cd server
go mod tidy
go build -o server main.go
./server
```

### Docker Build

```bash
cd server
docker build -t rca-app-server:latest .
```

## 3. ML Service (Python)

### Local Build

```bash
cd ml-service
python3 -m venv venv
source venv/bin/activate
pip install -r requirements.txt
python main.py
```

### Docker Build

```bash
cd ml-service
docker build -t rca-app-ml-service:latest .
```

## 4. Automation (Makefile)

A root `Makefile` is provided to manage all modules simultaneously.

- **Build all binaries locally**:
    ```bash
    make build
    ```

- **Build all Docker images**:
    ```bash
    make docker-build
    ```

- **Clean build artifacts**:
    ```bash
    make clean
    ```