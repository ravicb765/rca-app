# Build stage
FROM golang:1.24-bookworm AS builder

# Install dependencies for eBPF compilation
RUN apt-get update && apt-get install -y \
    clang \
    llvm \
    make \
    libbpf-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy dependency files first for caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary and eBPF object
RUN make build 

# Runtime stage
FROM alpine:3.22

WORKDIR /app

# Copy binary and eBPF object from builder
COPY --from=builder /app/node-agent .
COPY --from=builder /app/ebpf/network_tracer.o ./ebpf/

# The agent requires privileged access to load eBPF programs
CMD ["./node-agent"]
