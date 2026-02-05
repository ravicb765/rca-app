package tracing

import (
	"context"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// Span represents a distributed tracing span
type Span struct {
	TraceID       string            `json:"trace_id"`
	SpanID        string            `json:"span_id"`
	ParentSpanID  string            `json:"parent_span_id"`
	OperationName string            `json:"operation_name"`
	StartTime     time.Time         `json:"start_time"`
	Duration      uint64            `json:"duration"` // nanoseconds
	Application   string            `json:"application"`
	Attributes    map[string]string `json:"attributes"`
	Status        string            `json:"status"`
}

// Collector handles trace ingestion and retrieval
type Collector struct {
	db driver.Conn
}

func NewCollector(db driver.Conn) *Collector {
	return &Collector{db: db}
}

// IngestSpans inserts a batch of spans into ClickHouse
func (c *Collector) IngestSpans(ctx context.Context, spans []Span) error {
	batch, err := c.db.PrepareBatch(ctx, "INSERT INTO traces")
	if err != nil {
		return err
	}
	for _, s := range spans {
		// Schema: trace_id, span_id, parent_span_id, operation_name, start_time, duration, application, attributes, events, status
		// Note: events are passed as empty slice for this implementation
		err := batch.Append(
			s.TraceID,
			s.SpanID,
			s.ParentSpanID,
			s.OperationName,
			s.StartTime,
			s.Duration,
			s.Application,
			s.Attributes,
			[]interface{}{}, // events
			s.Status,
		)
		if err != nil {
			return err
		}
	}
	return batch.Send()
}

// GetTrace retrieves all spans for a given trace ID
func (c *Collector) GetTrace(ctx context.Context, traceID string) ([]Span, error) {
	rows, err := c.db.Query(ctx, "SELECT trace_id, span_id, parent_span_id, operation_name, start_time, duration, application, attributes, status FROM traces WHERE trace_id = ? ORDER BY start_time", traceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var spans []Span
	for rows.Next() {
		var s Span
		if err := rows.Scan(&s.TraceID, &s.SpanID, &s.ParentSpanID, &s.OperationName, &s.StartTime, &s.Duration, &s.Application, &s.Attributes, &s.Status); err != nil {
			return nil, err
		}
		spans = append(spans, s)
	}
	return spans, nil
}
