package collector

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNew(t *testing.T) {
	c, err := New("test-node")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if c.nodeName != "test-node" {
		t.Errorf("nodeName = %v, want test-node", c.nodeName)
	}
}

func TestNewWithOptions(t *testing.T) {
	c, err := New("test-node",
		WithProcPath("/custom/proc"),
		WithSysPath("/custom/sys"),
		WithDisk("sda"),
		WithNetworkInterface("eth0"),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if c.procPath != "/custom/proc" {
		t.Errorf("procPath = %v, want /custom/proc", c.procPath)
	}
	if c.sysPath != "/custom/sys" {
		t.Errorf("sysPath = %v, want /custom/sys", c.sysPath)
	}
	if c.primaryDisk != "sda" {
		t.Errorf("primaryDisk = %v, want sda", c.primaryDisk)
	}
	if c.networkIface != "eth0" {
		t.Errorf("networkIface = %v, want eth0", c.networkIface)
	}
}

func TestCollect(t *testing.T) {
	// Skip if not on Linux
	if _, err := os.Stat("/proc/stat"); os.IsNotExist(err) {
		t.Skip("Skipping test: /proc/stat not available (not Linux)")
	}

	c, err := New("test-node")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := context.Background()
	m, err := c.Collect(ctx)
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}

	// Basic sanity checks
	if m.NodeName != "test-node" {
		t.Errorf("NodeName = %v, want test-node", m.NodeName)
	}
	if m.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}

	// CPU usage should be between 0 and 100 (may be 0 on first collection)
	if m.CPUUsagePercent < 0 || m.CPUUsagePercent > 100 {
		t.Errorf("CPUUsagePercent = %v, want 0-100", m.CPUUsagePercent)
	}

	// Memory should be > 0
	if m.MemoryTotalBytes == 0 {
		t.Error("MemoryTotalBytes should be > 0")
	}

	// Collection duration should be recorded
	if m.CollectionDurationMs <= 0 {
		t.Errorf("CollectionDurationMs = %v, want > 0", m.CollectionDurationMs)
	}
}

func TestCollectWithMockProc(t *testing.T) {
	// Create mock /proc structure
	tmpDir := t.TempDir()
	procDir := filepath.Join(tmpDir, "proc")
	sysDir := filepath.Join(tmpDir, "sys")

	// Create mock files
	os.MkdirAll(procDir, 0755)
	os.MkdirAll(filepath.Join(procDir, "net"), 0755)
	os.MkdirAll(filepath.Join(sysDir, "class/thermal"), 0755)
	os.MkdirAll(filepath.Join(sysDir, "devices/system/cpu/cpu0/cpufreq"), 0755)

	// Mock /proc/stat
	statContent := `cpu  10000 500 3000 80000 1000 100 50 0 0 0
cpu0 5000 250 1500 40000 500 50 25 0 0 0
`
	os.WriteFile(filepath.Join(procDir, "stat"), []byte(statContent), 0644)

	// Mock /proc/loadavg
	loadavgContent := "1.50 1.25 1.00 1/100 12345\n"
	os.WriteFile(filepath.Join(procDir, "loadavg"), []byte(loadavgContent), 0644)

	// Mock /proc/meminfo
	meminfoContent := `MemTotal:        8000000 kB
MemFree:         2000000 kB
MemAvailable:    4000000 kB
SwapTotal:       1000000 kB
SwapFree:         500000 kB
`
	os.WriteFile(filepath.Join(procDir, "meminfo"), []byte(meminfoContent), 0644)

	// Mock /proc/vmstat
	vmstatContent := "oom_kill 5\n"
	os.WriteFile(filepath.Join(procDir, "vmstat"), []byte(vmstatContent), 0644)

	// Mock /proc/diskstats
	diskstatsContent := "   8       0 sda 1000 0 20000 500 2000 0 40000 800 0 1000 1300\n"
	os.WriteFile(filepath.Join(procDir, "diskstats"), []byte(diskstatsContent), 0644)

	// Mock /proc/mounts
	mountsContent := "/dev/sda1 / ext4 rw 0 0\n"
	os.WriteFile(filepath.Join(procDir, "mounts"), []byte(mountsContent), 0644)

	// Mock /proc/net/dev
	netdevContent := `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
    lo: 1000000   10000    0    0    0     0          0         0  1000000   10000    0    0    0     0       0          0
  eth0: 5000000   50000   10    0    0     0          0         0  3000000   30000    5    0    0     0       0          0
`
	os.WriteFile(filepath.Join(procDir, "net/dev"), []byte(netdevContent), 0644)

	// Mock /proc/net/route
	routeContent := `Iface	Destination	Gateway		Flags	RefCnt	Use	Metric	Mask		MTU	Window	IRTT
eth0	00000000	0100A8C0	0003	0	0	100	00000000	0	0	0
`
	os.WriteFile(filepath.Join(procDir, "net/route"), []byte(routeContent), 0644)

	// Mock thermal zone
	thermalZone := filepath.Join(sysDir, "class/thermal/thermal_zone0")
	os.MkdirAll(thermalZone, 0755)
	os.WriteFile(filepath.Join(thermalZone, "temp"), []byte("45000\n"), 0644)

	// Create collector with mock paths
	c, err := New("test-node",
		WithProcPath(procDir),
		WithSysPath(sysDir),
		WithDisk("sda"),
		WithNetworkInterface("eth0"),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := context.Background()

	// First collection to establish baseline
	_, err = c.Collect(ctx)
	if err != nil {
		t.Fatalf("First Collect() error = %v", err)
	}

	// Second collection to get deltas
	m, err := c.Collect(ctx)
	if err != nil {
		t.Fatalf("Second Collect() error = %v", err)
	}

	// Verify parsed values
	if m.LoadAverage1Min != 1.50 {
		t.Errorf("LoadAverage1Min = %v, want 1.50", m.LoadAverage1Min)
	}
	if m.LoadAverage5Min != 1.25 {
		t.Errorf("LoadAverage5Min = %v, want 1.25", m.LoadAverage5Min)
	}
	if m.LoadAverage15Min != 1.00 {
		t.Errorf("LoadAverage15Min = %v, want 1.00", m.LoadAverage15Min)
	}

	// Memory (converted from KB to bytes)
	expectedMemTotal := uint64(8000000 * 1024)
	if m.MemoryTotalBytes != expectedMemTotal {
		t.Errorf("MemoryTotalBytes = %v, want %v", m.MemoryTotalBytes, expectedMemTotal)
	}

	expectedMemAvail := uint64(4000000 * 1024)
	if m.MemoryAvailableBytes != expectedMemAvail {
		t.Errorf("MemoryAvailableBytes = %v, want %v", m.MemoryAvailableBytes, expectedMemAvail)
	}

	// Memory usage should be 50% (4GB used out of 8GB)
	if m.MemoryUsagePercent < 49 || m.MemoryUsagePercent > 51 {
		t.Errorf("MemoryUsagePercent = %v, want ~50", m.MemoryUsagePercent)
	}

	// OOM kill count
	if m.OOMKillCount != 5 {
		t.Errorf("OOMKillCount = %v, want 5", m.OOMKillCount)
	}

	// Temperature (45000 millidegrees = 45°C)
	if m.CPUTemperature != 45.0 {
		t.Errorf("CPUTemperature = %v, want 45.0", m.CPUTemperature)
	}
}

func TestDetectPrimaryDisk(t *testing.T) {
	tmpDir := t.TempDir()
	procDir := filepath.Join(tmpDir, "proc")
	os.MkdirAll(procDir, 0755)

	tests := []struct {
		name     string
		mounts   string
		expected string
	}{
		{
			name:     "standard disk",
			mounts:   "/dev/sda1 / ext4 rw 0 0\n",
			expected: "sda",
		},
		{
			name:     "nvme disk",
			mounts:   "/dev/nvme0n1p1 / ext4 rw 0 0\n",
			expected: "nvme0n1",
		},
		{
			name:     "multiple mounts",
			mounts:   "/dev/sdb1 /boot ext4 rw 0 0\n/dev/sda1 / ext4 rw 0 0\n",
			expected: "sda",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.WriteFile(filepath.Join(procDir, "mounts"), []byte(tt.mounts), 0644)

			c := &Collector{procPath: procDir}
			result := c.detectPrimaryDisk()

			if result != tt.expected {
				t.Errorf("detectPrimaryDisk() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDetectPrimaryInterface(t *testing.T) {
	tmpDir := t.TempDir()
	procDir := filepath.Join(tmpDir, "proc")
	netDir := filepath.Join(procDir, "net")
	os.MkdirAll(netDir, 0755)

	routeContent := `Iface	Destination	Gateway		Flags	RefCnt	Use	Metric	Mask		MTU	Window	IRTT
eth0	00000000	0100A8C0	0003	0	0	100	00000000	0	0	0
eth0	0000A8C0	00000000	0001	0	0	100	00FFFFFF	0	0	0
`
	os.WriteFile(filepath.Join(netDir, "route"), []byte(routeContent), 0644)

	c := &Collector{procPath: procDir}
	result := c.detectPrimaryInterface()

	if result != "eth0" {
		t.Errorf("detectPrimaryInterface() = %v, want eth0", result)
	}
}

func BenchmarkCollect(b *testing.B) {
	// Skip if not on Linux
	if _, err := os.Stat("/proc/stat"); os.IsNotExist(err) {
		b.Skip("Skipping benchmark: /proc/stat not available (not Linux)")
	}

	c, err := New("bench-node")
	if err != nil {
		b.Fatalf("New() error = %v", err)
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := c.Collect(ctx)
		if err != nil {
			b.Fatalf("Collect() error = %v", err)
		}
	}
}
