FROM golang:1.21-alpine AS builder

# Install build dependencies for eBPF compilation
RUN apk add --no-cache clang llvm make

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Compile the eBPF C program
RUN make -C ebpf

# Build the Go binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o node-agent main.go

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/node-agent .

# Copy the compiled eBPF object to the expected location
COPY --from=builder /app/ebpf/network_tracer.o ./ebpf/network_tracer.o

CMD ["./node-agent"]