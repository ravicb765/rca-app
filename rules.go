package inspections

import (
	"fmt"

	"github.com/ravicb765/rca-app/server/servicemap"
)

type ErrorRateRule struct {
	Threshold float64
}

func (r *ErrorRateRule) Evaluate(app *servicemap.Application, metrics AppMetrics) (bool, string) {
	if metrics.ErrorRate > r.Threshold {
		return false, fmt.Sprintf("Error rate %.2f%% exceeds threshold %.2f%%", metrics.ErrorRate*100, r.Threshold*100)
	}
	return true, fmt.Sprintf("Error rate %.2f%% is within limits", metrics.ErrorRate*100)
}

type LatencyRule struct {
	Threshold float64 // in ms
}

func (r *LatencyRule) Evaluate(app *servicemap.Application, metrics AppMetrics) (bool, string) {
	if metrics.Latency > r.Threshold {
		return false, fmt.Sprintf("Latency %.2fms exceeds threshold %.2fms", metrics.Latency, r.Threshold)
	}
	return true, fmt.Sprintf("Latency %.2fms is within limits", metrics.Latency)
}

type MemoryLeakRule struct {
	Threshold float64 // in MB
}

func (r *MemoryLeakRule) Evaluate(app *servicemap.Application, metrics AppMetrics) (bool, string) {
	if metrics.MemoryUsage > r.Threshold {
		return false, fmt.Sprintf("Memory usage %.2fMB exceeds threshold %.2fMB", metrics.MemoryUsage, r.Threshold)
	}
	return true, fmt.Sprintf("Memory usage %.2fMB is within limits", metrics.MemoryUsage)
}

type CPUUsageRule struct {
	Threshold float64 // percentage
}

func (r *CPUUsageRule) Evaluate(app *servicemap.Application, metrics AppMetrics) (bool, string) {
	if metrics.CPUUsage > r.Threshold {
		return false, fmt.Sprintf("CPU usage %.2f%% exceeds threshold %.2f%%", metrics.CPUUsage, r.Threshold)
	}
	return true, fmt.Sprintf("CPU usage %.2f%% is within limits", metrics.CPUUsage)
}

type DiskUsageRule struct {
	Threshold float64 // percentage
}

func (r *DiskUsageRule) Evaluate(app *servicemap.Application, metrics AppMetrics) (bool, string) {
	if metrics.DiskUsage > r.Threshold {
		return false, fmt.Sprintf("Disk usage %.2f%% exceeds threshold %.2f%%", metrics.DiskUsage, r.Threshold)
	}
	return true, fmt.Sprintf("Disk usage %.2f%% is within limits", metrics.DiskUsage)
}

type IOLoadRule struct {
	Threshold float64
}

func (r *IOLoadRule) Evaluate(app *servicemap.Application, metrics AppMetrics) (bool, string) {
	if metrics.IOLoad > r.Threshold {
		return false, fmt.Sprintf("IO load %.2f exceeds threshold %.2f", metrics.IOLoad, r.Threshold)
	}
	return true, fmt.Sprintf("IO load %.2f is within limits", metrics.IOLoad)
}

type ConnectionPoolExhaustionRule struct {
	Threshold float64
}

func (r *ConnectionPoolExhaustionRule) Evaluate(app *servicemap.Application, metrics AppMetrics) (bool, string) {
	if metrics.ActiveConnections > r.Threshold {
		return false, fmt.Sprintf("Active connections %.0f exceeds threshold %.0f", metrics.ActiveConnections, r.Threshold)
	}
	return true, fmt.Sprintf("Active connections %.0f is within limits", metrics.ActiveConnections)
}

type PacketLossRule struct {
	Threshold float64 // percentage
}

func (r *PacketLossRule) Evaluate(app *servicemap.Application, metrics AppMetrics) (bool, string) {
	if metrics.PacketLoss > r.Threshold {
		return false, fmt.Sprintf("Packet loss %.2f%% exceeds threshold %.2f%%", metrics.PacketLoss, r.Threshold)
	}
	return true, fmt.Sprintf("Packet loss %.2f%% is within limits", metrics.PacketLoss)
}

type Http5xxRateRule struct {
	Threshold float64
}

func (r *Http5xxRateRule) Evaluate(app *servicemap.Application, metrics AppMetrics) (bool, string) {
	if metrics.Http5xxRate > r.Threshold {
		return false, fmt.Sprintf("HTTP 5xx rate %.2f%% exceeds threshold %.2f%%", metrics.Http5xxRate*100, r.Threshold*100)
	}
	return true, fmt.Sprintf("HTTP 5xx rate %.2f%% is within limits", metrics.Http5xxRate*100)
}

type IOWaitRule struct {
	Threshold float64 // percentage
}

func (r *IOWaitRule) Evaluate(app *servicemap.Application, metrics AppMetrics) (bool, string) {
	if metrics.IOWait > r.Threshold {
		return false, fmt.Sprintf("Disk I/O wait %.2f%% exceeds threshold %.2f%%", metrics.IOWait, r.Threshold)
	}
	return true, fmt.Sprintf("Disk I/O wait %.2f%% is within limits", metrics.IOWait)
}

type SwapUsageRule struct {
	Threshold float64 // percentage
}

func (r *SwapUsageRule) Evaluate(app *servicemap.Application, metrics AppMetrics) (bool, string) {
	if metrics.SwapUsage > r.Threshold {
		return false, fmt.Sprintf("Swap usage %.2f%% exceeds threshold %.2f%%", metrics.SwapUsage, r.Threshold)
	}
	return true, fmt.Sprintf("Swap usage %.2f%% is within limits", metrics.SwapUsage)
}

type RestartCountRule struct {
	Threshold float64
}

func (r *RestartCountRule) Evaluate(app *servicemap.Application, metrics AppMetrics) (bool, string) {
	if metrics.RestartCount > r.Threshold {
		return false, fmt.Sprintf("Container restart count %.0f exceeds threshold %.0f", metrics.RestartCount, r.Threshold)
	}
	return true, fmt.Sprintf("Container restart count %.0f is within limits", metrics.RestartCount)
}

type CPUThrottlingRule struct {
	Threshold float64
}

func (r *CPUThrottlingRule) Evaluate(app *servicemap.Application, metrics AppMetrics) (bool, string) {
	if metrics.CPUThrottling > r.Threshold {
		return false, fmt.Sprintf("CPU throttling %.2f%% exceeds threshold %.2f%%", metrics.CPUThrottling, r.Threshold)
	}
	return true, fmt.Sprintf("CPU throttling %.2f%% is within limits", metrics.CPUThrottling)
}

type GoroutineCountRule struct {
	Threshold float64
}

func (r *GoroutineCountRule) Evaluate(app *servicemap.Application, metrics AppMetrics) (bool, string) {
	if metrics.GoroutineCount > r.Threshold {
		return false, fmt.Sprintf("Goroutine count %.0f exceeds threshold %.0f", metrics.GoroutineCount, r.Threshold)
	}
	return true, fmt.Sprintf("Goroutine count %.0f is within limits", metrics.GoroutineCount)
}

type OpenFDCountRule struct {
	Threshold float64
}

func (r *OpenFDCountRule) Evaluate(app *servicemap.Application, metrics AppMetrics) (bool, string) {
	if metrics.OpenFDs > r.Threshold {
		return false, fmt.Sprintf("Open file descriptors %.0f exceeds threshold %.0f", metrics.OpenFDs, r.Threshold)
	}
	return true, fmt.Sprintf("Open file descriptors %.0f is within limits", metrics.OpenFDs)
}

type ThreadCountRule struct {
	Threshold float64
}

func (r *ThreadCountRule) Evaluate(app *servicemap.Application, metrics AppMetrics) (bool, string) {
	if metrics.ThreadCount > r.Threshold {
		return false, fmt.Sprintf("Thread count %.0f exceeds threshold %.0f", metrics.ThreadCount, r.Threshold)
	}
	return true, fmt.Sprintf("Thread count %.0f is within limits", metrics.ThreadCount)
}

type MemcachedLatencyRule struct {
	Threshold float64 // in ms
}

func (r *MemcachedLatencyRule) Evaluate(app *servicemap.Application, metrics AppMetrics) (bool, string) {
	if metrics.MemcachedLatency > r.Threshold {
		return false, fmt.Sprintf("Memcached latency %.2fms exceeds threshold %.2fms", metrics.MemcachedLatency, r.Threshold)
	}
	return true, fmt.Sprintf("Memcached latency %.2fms is within limits", metrics.MemcachedLatency)
}

type MysqlLatencyRule struct {
	Threshold float64 // in ms
}

func (r *MysqlLatencyRule) Evaluate(app *servicemap.Application, metrics AppMetrics) (bool, string) {
	if metrics.MysqlLatency > r.Threshold {
		return false, fmt.Sprintf("MySQL latency %.2fms exceeds threshold %.2fms", metrics.MysqlLatency, r.Threshold)
	}
	return true, fmt.Sprintf("MySQL latency %.2fms is within limits", metrics.MysqlLatency)
}

type MongoLatencyRule struct {
	Threshold float64 // in ms
}

func (r *MongoLatencyRule) Evaluate(app *servicemap.Application, metrics AppMetrics) (bool, string) {
	if metrics.MongoLatency > r.Threshold {
		return false, fmt.Sprintf("MongoDB latency %.2fms exceeds threshold %.2fms", metrics.MongoLatency, r.Threshold)
	}
	return true, fmt.Sprintf("MongoDB latency %.2fms is within limits", metrics.MongoLatency)
}

type RabbitMQLatencyRule struct {
	Threshold float64 // in ms
}

func (r *RabbitMQLatencyRule) Evaluate(app *servicemap.Application, metrics AppMetrics) (bool, string) {
	if metrics.RabbitMQLatency > r.Threshold {
		return false, fmt.Sprintf("RabbitMQ latency %.2fms exceeds threshold %.2fms", metrics.RabbitMQLatency, r.Threshold)
	}
	return true, fmt.Sprintf("RabbitMQ latency %.2fms is within limits", metrics.RabbitMQLatency)
}

type RabbitMQQueueLengthRule struct {
	Threshold float64 // count
}

func (r *RabbitMQQueueLengthRule) Evaluate(app *servicemap.Application, metrics AppMetrics) (bool, string) {
	if metrics.RabbitMQQueueLength > r.Threshold {
		return false, fmt.Sprintf("RabbitMQ queue length %.0f exceeds threshold %.0f", metrics.RabbitMQQueueLength, r.Threshold)
	}
	return true, fmt.Sprintf("RabbitMQ queue length %.0f is within limits", metrics.RabbitMQQueueLength)
}

type CassandraLatencyRule struct {
	Threshold float64 // in ms
}

func (r *CassandraLatencyRule) Evaluate(app *servicemap.Application, metrics AppMetrics) (bool, string) {
	if metrics.CassandraLatency > r.Threshold {
		return false, fmt.Sprintf("Cassandra latency %.2fms exceeds threshold %.2fms", metrics.CassandraLatency, r.Threshold)
	}
	return true, fmt.Sprintf("Cassandra latency %.2fms is within limits", metrics.CassandraLatency)
}
