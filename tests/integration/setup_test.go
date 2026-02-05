package integration

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

    "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/clickhouse"
    "github.com/testcontainers/testcontainers-go/modules/kafka"
    "github.com/testcontainers/testcontainers-go/modules/redis"
)

var (
	clickHouseAddr string
    kafkaBrokers   []string
    redisAddr      string
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	// 1. Start ClickHouse
	chContainer, err := clickhouse.RunContainer(ctx,
		testcontainers.WithImage("clickhouse/clickhouse-server:latest"),
        clickhouse.WithDatabase("rca"),
        clickhouse.WithUsername("default"),
        clickhouse.WithPassword(""),
        clickhouse.WithInitScripts("../../deploy/clickhouse/init.sql"),
	)
	if err != nil {
		log.Fatalf("failed to start clickhouse: %v", err)
	}
    
    // Get host:port
    chHost, _ := chContainer.Host(ctx)
    chPort, _ := chContainer.MappedPort(ctx, "9000")
    clickHouseAddr = fmt.Sprintf("%s:%s", chHost, chPort.Port())

    // 2. Start Kafka
    kafkaContainer, err := kafka.RunContainer(ctx, 
        kafka.WithClusterID("test-cluster"),
        testcontainers.WithImage("confluentinc/cp-kafka:7.3.0"),
    )
    if err != nil {
        log.Fatalf("failed to start kafka: %v", err)
    }
    
    kafkaBrokers, _ = kafkaContainer.Brokers(ctx)

    // 3. Start Redis
    redisContainer, err := redis.RunContainer(ctx, 
        testcontainers.WithImage("redis:7.0"),
    )
    if err != nil {
        log.Fatalf("failed to start redis: %v", err)
    }
    
    redisHost, _ := redisContainer.Host(ctx)
    redisPort, _ := redisContainer.MappedPort(ctx, "6379")
    redisAddr = fmt.Sprintf("%s:%s", redisHost, redisPort.Port())

	// Run tests
	code := m.Run()

	// Teardown
    chContainer.Terminate(ctx)
    kafkaContainer.Terminate(ctx)
    redisContainer.Terminate(ctx)

	os.Exit(code)
}
