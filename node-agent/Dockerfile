FROM golang:1.21-bullseye AS builder

# Install dependencies for eBPF compilation
RUN apt-get update && apt-get install -y \
    clang \
    llvm \
    libbpf-dev \
    gcc \
    make \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Compile eBPF objects
RUN cd ebpf && make

# Build Go binary with ebpf tag
RUN go build -tags ebpf -o node-agent .

FROM debian:bullseye-slim
WORKDIR /app
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/node-agent .
# Copy compiled eBPF objects so the loader can find them at runtime
COPY --from=builder /app/ebpf/*.o ./ebpf/

CMD ["./node-agent"]