// Package collector provides system metrics collection for Linux nodes.
// It abstracts hardware-specific details while optimizing for edge/resource-constrained environments.
package collector

import (
	"context"
	"sync"
	"time"
)

// NodeMetrics represents a snapshot of node health metrics.
type NodeMetrics struct {
	Timestamp time.Time `json:"timestamp"`
	NodeName  string    `json:"node_name"`

	// CPU metrics
	CPUTemperature    float64 `json:"cpu_temperature_celsius"`
	CPUUsagePercent   float64 `json:"cpu_usage_percent"`
	CPUThrottled      bool    `json:"cpu_throttled"`
	CPUFrequencyMHz   float64 `json:"cpu_frequency_mhz"`
	LoadAverage1Min   float64 `json:"load_average_1min"`
	LoadAverage5Min   float64 `json:"load_average_5min"`
	LoadAverage15Min  float64 `json:"load_average_15min"`

	// Memory metrics
	MemoryTotalBytes     uint64  `json:"memory_total_bytes"`
	MemoryAvailableBytes uint64  `json:"memory_available_bytes"`
	MemoryUsagePercent   float64 `json:"memory_usage_percent"`
	SwapTotalBytes       uint64  `json:"swap_total_bytes"`
	SwapUsedBytes        uint64  `json:"swap_used_bytes"`
	OOMKillCount         uint64  `json:"oom_kill_count"`

	// Disk metrics
	DiskTotalBytes      uint64  `json:"disk_total_bytes"`
	DiskUsedBytes       uint64  `json:"disk_used_bytes"`
	DiskUsagePercent    float64 `json:"disk_usage_percent"`
	DiskIOReadBytes     uint64  `json:"disk_io_read_bytes"`
	DiskIOWriteBytes    uint64  `json:"disk_io_write_bytes"`
	DiskIOLatencyMs     float64 `json:"disk_io_latency_ms"`

	// Network metrics
	NetworkRxBytes   uint64  `json:"network_rx_bytes"`
	NetworkTxBytes   uint64  `json:"network_tx_bytes"`
	NetworkRxErrors  uint64  `json:"network_rx_errors"`
	NetworkTxErrors  uint64  `json:"network_tx_errors"`
	NetworkLatencyMs float64 `json:"network_latency_ms"`

	// Collection metadata
	CollectionDurationMs float64 `json:"collection_duration_ms"`
	Errors               []string `json:"errors,omitempty"`
}

// Collector gathers system metrics from a Linux node.
type Collector struct {
	nodeName       string
	procPath       string
	sysPath        string
	thermalZones   []string
	primaryDisk    string
	networkIface   string

	// Previous values for delta calculations
	mu             sync.Mutex
	lastCollect    time.Time
}

// Option configures the Collector.
type Option func(*Collector)

// WithProcPath sets a custom /proc path (useful for testing).
func WithProcPath(path string) Option {
	return func(c *Collector) {
		c.procPath = path
	}
}

// WithSysPath sets a custom /sys path (useful for testing).
func WithSysPath(path string) Option {
	return func(c *Collector) {
		c.sysPath = path
	}
}

// WithDisk sets the primary disk device to monitor.
func WithDisk(device string) Option {
	return func(c *Collector) {
		c.primaryDisk = device
	}
}

// WithNetworkInterface sets the primary network interface to monitor.
func WithNetworkInterface(iface string) Option {
	return func(c *Collector) {
		c.networkIface = iface
	}
}

// New creates a new Collector for the given node.
func New(nodeName string, opts ...Option) (*Collector, error) {
	c := &Collector{
		nodeName: nodeName,
		procPath: "/proc",
		sysPath:  "/sys",
	}

	for _, opt := range opts {
		opt(c)
	}

	return c, nil
}

// Collect gathers all metrics from the node.
func (c *Collector) Collect(ctx context.Context) (*NodeMetrics, error) {
	start := time.Now()

	m := &NodeMetrics{
		Timestamp: start,
		NodeName:  c.nodeName,
	}

	// TODO: Implement metric collection
	m.CollectionDurationMs = float64(time.Since(start).Microseconds()) / 1000.0
	return m, nil
}
