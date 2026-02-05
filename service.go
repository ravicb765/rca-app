package profiling

import (
	"bytes"
	"context"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/google/pprof/profile"
)

// Service handles profile ingestion and flamegraph generation
type Service struct {
	db driver.Conn
}

func NewService(db driver.Conn) *Service {
	return &Service{db: db}
}

// IngestProfile parses a pprof binary and stores aggregated stacktraces in ClickHouse
func (s *Service) IngestProfile(ctx context.Context, app, instance, pType string, data []byte) error {
	p, err := profile.Parse(bytes.NewReader(data))
	if err != nil {
		return err
	}

	// Aggregate samples: stack -> value
	stackMap := make(map[string]uint64)
	for _, sample := range p.Sample {
		var frames []string
		// pprof locations are leaf-first, iterate backwards for root-first stack
		for i := len(sample.Location) - 1; i >= 0; i-- {
			loc := sample.Location[i]
			for _, line := range loc.Line {
				frames = append(frames, line.Function.Name)
			}
		}
		key := strings.Join(frames, ";")
		// Use the last value type (usually the main unit, e.g., cpu nanoseconds)
		if len(sample.Value) > 0 {
			val := uint64(sample.Value[len(sample.Value)-1])
			stackMap[key] += val
		}
	}

	// Convert to ClickHouse structure: Array(Tuple(Array(String), UInt64))
	var stacktraces []interface{}
	for stack, val := range stackMap {
		frames := strings.Split(stack, ";")
		stacktraces = append(stacktraces, []interface{}{frames, val})
	}

	// Insert into profiles table
	return s.db.Exec(ctx, `
		INSERT INTO profiles (timestamp, application, instance, profile_type, duration, sample_rate, stacktraces)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, time.Now(), app, instance, pType, uint64(p.DurationNanos), 100.0, stacktraces)
}

// FlamegraphNode represents a node in the flamegraph tree
type FlamegraphNode struct {
	Name     string            `json:"name"`
	Value    uint64            `json:"value"`
	Children []*FlamegraphNode `json:"children,omitempty"`
}

// GetFlamegraph aggregates profiles for a time range and returns a tree structure
func (s *Service) GetFlamegraph(ctx context.Context, app string, start, end time.Time) (*FlamegraphNode, error) {
	// ClickHouse query to aggregate stacktraces across multiple profile records
	query := `
		SELECT 
			tupleElement(st, 1) as frames, 
			sum(tupleElement(st, 2)) as val 
		FROM profiles 
		ARRAY JOIN stacktraces as st
		WHERE application = ? AND timestamp BETWEEN ? AND ?
		GROUP BY frames
	`
	rows, err := s.db.Query(ctx, query, app, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	root := &FlamegraphNode{Name: "root", Children: []*FlamegraphNode{}}

	for rows.Next() {
		var frames []string
		var val uint64
		if err := rows.Scan(&frames, &val); err != nil {
			return nil, err
		}
		s.addToTree(root, frames, val)
	}

	// Calculate root value sum
	for _, c := range root.Children {
		root.Value += c.Value
	}

	return root, nil
}

func (s *Service) addToTree(root *FlamegraphNode, frames []string, val uint64) {
	current := root
	for _, frame := range frames {
		var found *FlamegraphNode
		for _, child := range current.Children {
			if child.Name == frame {
				found = child
				break
			}
		}
		if found == nil {
			found = &FlamegraphNode{Name: frame, Children: []*FlamegraphNode{}}
			current.Children = append(current.Children, found)
		}
		current = found
		current.Value += val
	}
}
