FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go env -w GOPROXY=https://proxy.golang.org,direct
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /cluster-agent ./

FROM alpine:3.22
RUN apk --no-cache add ca-certificates
COPY --from=builder /cluster-agent /usr/local/bin/cluster-agent
EXPOSE 9100
ENTRYPOINT ["/usr/local/bin/cluster-agent"]