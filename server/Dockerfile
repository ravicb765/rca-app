FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Build static binary with debug information stripped for smaller size
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o server main.go

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/server .

EXPOSE 8080
CMD ["./server"]