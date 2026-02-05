FROM golang:1.21-alpine AS builder

# Install build dependencies for CGO (SQLite)
RUN apk add --no-cache gcc musl-dev

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Build binary with CGO enabled for SQLite
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w" -o server main.go

FROM alpine:latest
# Install sqlite-libs in the runtime stage just in case
RUN apk add --no-cache sqlite-libs

WORKDIR /app
COPY --from=builder /app/server .

EXPOSE 8080
CMD ["./server"]