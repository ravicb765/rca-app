package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type MetricsCache struct {
	client *redis.Client
}

func NewMetricsCache(addr string) *MetricsCache {
	rdb := redis.NewClient(&redis.Options{
		Addr: addr,
	})
	return &MetricsCache{client: rdb}
}

func (c *MetricsCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return c.client.Set(ctx, key, value, ttl).Err()
}

func (c *MetricsCache) Get(ctx context.Context, key string) ([]byte, error) {
	return c.client.Get(ctx, key).Bytes()
}
