# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies (gcc, musl-dev for CGO)
RUN apk add --no-cache build-base

WORKDIR /app

# Copy dependency files first for caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build with CGO enabled, using musl tags for sqlite3
RUN CGO_ENABLED=1 GOOS=linux CGO_CFLAGS="-D_LARGEFILE64_SOURCE" go build -tags musl -ldflags="-s -w" -o server main.go

# Runtime stage
FROM alpine:3.22

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/server .

CMD ["./server"]