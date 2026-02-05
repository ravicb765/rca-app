# RCA-App Node Agent

The Node Agent is an eBPF-powered telemetry collector designed to run as a DaemonSet on Kubernetes nodes.

## Features

- **Network Tracing**: Captures TCP connect, accept, and close events to build dependency maps.
- **L7 Visbility**: Parses HTTP headers to extract methods, paths, and status codes (userspace buffering).
- **Container Metrics**: Tracks CPU, Memory, and I/O usage per cgroup.
- **Continuous Profiling**: Low-overhead stack sampling using `perf_events`.

## Pre-requisites

- Linux Kernel 5.4+ (BTF support recommended)
- `clang` and `llvm` for compiling eBPF programs.
- Privileged access (required for loading BPF maps).

## Development

### Compile eBPF Programs

```bash
# Requires clang/llvm installed
go generate ./...
```

### Run Locally

```bash
# Must be run as root
sudo go run main.go
```

## Configuration

| Environment Variable | Description | Default |
|----------------------|-------------|---------|
| `RCA_APP_ENDPOINT` | URL of the central Backend Server | `http://localhost:8080` |
| `LOG_LEVEL` | Logging verbosity | `info` |

## Packaging & Distribution

The agent supports native Linux packaging for easy distribution to VMs:

```bash
make package-deb # Generates .deb for Debian/Ubuntu
make package-rpm # Generates .rpm for RHEL/CentOS
```

Packages include a `systemd` service unit for automated lifecycle management.

## Architecture

The agent uses `cilium/ebpf` to load C programs into the kernel. It reads events from `BPF_MAP_TYPE_PERF_EVENT_ARRAY` and forwards them to the backend server via a buffered channel.
