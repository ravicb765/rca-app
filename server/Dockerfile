FROM golang:1.21-alpine
WORKDIR /app
COPY go.mod .
COPY main.go .
# Ensure dependencies are resolved and go.sum is present so `go build` succeeds in clean build environments
RUN go mod tidy
RUN go build -o server main.go
EXPOSE 8080
CMD ["./server"]
