# Build Instructions

This document provides detailed steps to compile and build the RCA-App modules.

## Prerequisites

- **Go**: Version 1.21 or higher
- **Python**: Version 3.10 or higher
- **Docker**: Version 20.10 or higher
- **Clang/LLVM**: Required for compiling eBPF programs (node-agent)
- **Make**: For running automation scripts

## 1. Node Agent (Go + eBPF)

### Native Packaging (RPM/DEB)

The Node Agent supports native packaging using `nfpm`:

1.  **Build amd64 Binary**:
    ```bash
    cd node-agent
    make build-amd64
    ```

2.  **Generate Packages**:
    ```bash
    make package-deb
    make package-rpm
    ```
    Alternatively, use the orchestration script:
    ```bash
    ./scripts/package.sh
    ```

---

## 2. Server (Go)

### Local Build (Requires CGO)
The server uses `go-sqlite3`, which requires CGO and a working C compiler (`gcc`).

```bash
cd server
export CGO_ENABLED=1
go build -o server main.go
```

---

## 3. ML Service (Python)
... (existing content) ...

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