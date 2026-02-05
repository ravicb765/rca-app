package metrics

import (
	"context"
	"time"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

type QueryEngine struct {
	api v1.API
}

func NewQueryEngine(api v1.API) *QueryEngine {
	return &QueryEngine{api: api}
}

func (q *QueryEngine) Query(ctx context.Context, query string, ts time.Time) (model.Value, error) {
	val, _, err := q.api.Query(ctx, query, ts)
	return val, err
}

func (q *QueryEngine) QueryRange(ctx context.Context, query string, r v1.Range) (model.Value, error) {
	val, _, err := q.api.QueryRange(ctx, query, r)
	return val, err
}
