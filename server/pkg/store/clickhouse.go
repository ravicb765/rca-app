package store

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

type ClickHouseStore struct {
	conn driver.Conn
}

func NewClickHouseStore(addr string) (*ClickHouseStore, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{addr},
		Auth: clickhouse.Auth{
			Database: "rca",
			Username: "default",
			Password: "",
		},
		Settings: clickhouse.Settings{
			"max_execution_time": 60,
		},
		Compression: &clickhouse.Compression{
			Method: clickhouse.CompressionLZ4,
		},
	})
	if err != nil {
		return nil, err
	}

	if err := conn.Ping(context.Background()); err != nil {
		return nil, err
	}

	return &ClickHouseStore{conn: conn}, nil
}

func (s *ClickHouseStore) InsertLog(ctx context.Context, service, severity, body, traceID string) error {
	batch, err := s.conn.PrepareBatch(ctx, "INSERT INTO rca.logs")
	if err != nil {
		return err
	}
	if err := batch.Append(time.Now(), service, severity, body, traceID, "", map[string]string{}); err != nil {
		return err
	}
	return batch.Send()
}
