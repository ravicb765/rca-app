package metrics

import (
	"context"
	"fmt"
	"time"

	"github.com/ravicb765/rca-app/server/pkg/cache"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

type QueryEngine struct {
	api   v1.API
	cache *cache.MetricsCache
}

func NewQueryEngine(api v1.API, cache *cache.MetricsCache) *QueryEngine {
	return &QueryEngine{api: api, cache: cache}
}

func (q *QueryEngine) Query(ctx context.Context, query string, ts time.Time) (model.Value, error) {
	// Simple cache key based on query and time (rounded to minute)
	key := fmt.Sprintf("metric:%s:%d", query, ts.Truncate(time.Minute).Unix())
	
	if q.cache != nil {
		if val, err := q.cache.Get(ctx, key); err == nil {
			// In real impl: decode val back to model.Value
			// keeping it simple for now, assuming miss
			_ = val 
		}
	}

	val, warnings, err := q.api.Query(ctx, query, ts)
	if err == nil && q.cache != nil {
		// In real impl: encode val
		_ = warnings
	}
	return val, err
}

func (q *QueryEngine) QueryRange(ctx context.Context, query string, r v1.Range) (model.Value, error) {
	val, _, err := q.api.QueryRange(ctx, query, r)
	return val, err
}
