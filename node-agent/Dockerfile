FROM golang:1.21-alpine
WORKDIR /app
COPY go.mod .
COPY main.go .
RUN go build -o node-agent main.go
EXPOSE 9100
CMD ["./node-agent"]
