package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/ravicb765/rca-app/logs"
	"github.com/ravicb765/rca-app/metrics"
	"github.com/ravicb765/rca-app/profiling"
	"github.com/ravicb765/rca-app/tracing"
)

type Server struct {
	MetricsEngine *metrics.QueryEngine
	Drain         *logs.Drain
	Tracer        *tracing.Collector
	Profiler      *profiling.Service
}

// MetricsQueryHandler handles instant queries
// GET /api/v1/metrics/query?query=up&time=1234567890
func (s *Server) MetricsQueryHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	tsStr := r.URL.Query().Get("time")

	if query == "" {
		http.Error(w, "missing query parameter", http.StatusBadRequest)
		return
	}

	ts := time.Now()
	if tsStr != "" {
		if t, err := strconv.ParseInt(tsStr, 10, 64); err == nil {
			ts = time.Unix(t, 0)
		}
	}

	val, err := s.MetricsEngine.Query(r.Context(), query, ts)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"data":   val,
	})
}

// MetricsQueryRangeHandler handles range queries
// GET /api/v1/metrics/query_range?query=up&start=...&end=...&step=...
func (s *Server) MetricsQueryRangeHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")
	stepStr := r.URL.Query().Get("step")

	if query == "" || startStr == "" || endStr == "" || stepStr == "" {
		http.Error(w, "missing required parameters", http.StatusBadRequest)
		return
	}

	start, _ := strconv.ParseInt(startStr, 10, 64)
	end, _ := strconv.ParseInt(endStr, 10, 64)
	step, _ := strconv.ParseInt(stepStr, 10, 64) // seconds

	rng := v1.Range{
		Start: time.Unix(start, 0),
		End:   time.Unix(end, 0),
		Step:  time.Duration(step) * time.Second,
	}

	val, err := s.MetricsEngine.QueryRange(r.Context(), query, rng)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"data":   val,
	})
}

// IngestLogHandler simulates log ingestion and clustering
// POST /api/v1/logs
func (s *Server) IngestLogHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	id, pattern := s.Drain.AddLog(req.Message)

	// In a real app, we would now insert into ClickHouse with the ID

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"pattern_id": id,
		"pattern":    pattern,
	})
}

// LogsByPatternHandler queries logs by pattern ID (Mock implementation)
// GET /api/v1/logs/pattern/:id
func (s *Server) LogsByPatternHandler(w http.ResponseWriter, r *http.Request) {
	// Extract ID from path (using a router like mux or gin in real app)
	// For simplicity, assuming query param here
	idStr := r.URL.Query().Get("id")
	id, _ := strconv.Atoi(idStr)

	pattern := s.Drain.GetPattern(id)
	if pattern == "" {
		http.Error(w, "pattern not found", http.StatusNotFound)
		return
	}

	// Mock ClickHouse query result
	logs := []map[string]interface{}{
		{
			"timestamp": time.Now(),
			"message":   "Mock log matching pattern " + pattern,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"pattern_id": id,
		"pattern":    pattern,
		"logs":       logs,
	})
}

// IngestTraceHandler accepts a batch of spans
// POST /api/v1/traces
func (s *Server) IngestTraceHandler(w http.ResponseWriter, r *http.Request) {
	var spans []tracing.Span
	if err := json.NewDecoder(r.Body).Decode(&spans); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.Tracer.IngestSpans(r.Context(), spans); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

// GetTraceHandler retrieves a trace by ID
// GET /api/v1/traces/:id
func (s *Server) GetTraceHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}

	spans, err := s.Tracer.GetTrace(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(spans)
}

// IngestProfileHandler accepts pprof data
// POST /api/v1/profiles?app=foo&instance=bar&type=cpu
func (s *Server) IngestProfileHandler(w http.ResponseWriter, r *http.Request) {
	app := r.URL.Query().Get("app")
	instance := r.URL.Query().Get("instance")
	pType := r.URL.Query().Get("type")

	data, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.Profiler.IngestProfile(r.Context(), app, instance, pType, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

// GetFlamegraphHandler generates a flamegraph
// GET /api/v1/profiles/flamegraph?app=foo&start=...&end=...
func (s *Server) GetFlamegraphHandler(w http.ResponseWriter, r *http.Request) {
	app := r.URL.Query().Get("app")
	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")

	start, _ := time.Parse(time.RFC3339, startStr)
	end, _ := time.Parse(time.RFC3339, endStr)

	fg, err := s.Profiler.GetFlamegraph(r.Context(), app, start, end)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(fg)
}

// SetupRoutes configures the mux
func SetupRoutes(mux *http.ServeMux, s *Server) {
	mux.HandleFunc("/api/v1/metrics/query", s.MetricsQueryHandler)
	mux.HandleFunc("/api/v1/metrics/query_range", s.MetricsQueryRangeHandler)
	mux.HandleFunc("/api/v1/logs", s.IngestLogHandler)
	mux.HandleFunc("/api/v1/logs/pattern", s.LogsByPatternHandler)
	mux.HandleFunc("/api/v1/traces", s.IngestTraceHandler)
	mux.HandleFunc("/api/v1/traces/get", s.GetTraceHandler)
	mux.HandleFunc("/api/v1/profiles", s.IngestProfileHandler)
	mux.HandleFunc("/api/v1/profiles/flamegraph", s.GetFlamegraphHandler)
}
```

<!--
[PROMPT_SUGGESTION]Implement a distributed tracing collector that accepts OpenTelemetry spans and stores them in ClickHouse.[/PROMPT_SUGGESTION]
[PROMPT_SUGGESTION]Add a continuous profiling service that aggregates pprof profiles from agents and generates flamegraphs.[/PROMPT_SUGGESTION]
