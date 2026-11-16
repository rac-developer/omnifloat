package system

import (
	"context"
	"fmt"
	"math"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	psnet "github.com/shirou/gopsutil/v3/net"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Stats holds real-time telemetry for CPU, RAM, Network I/O, and Clock.
type Stats struct {
	CPUPercent  float64 `json:"cpuPercent"`
	RAMPercent  float64 `json:"ramPercent"`
	RAMUsedMB   uint64  `json:"ramUsedMB"`
	RAMTotalMB  uint64  `json:"ramTotalMB"`
	NetDownKB   float64 `json:"netDownKB"`
	NetUpKB     float64 `json:"netUpKB"`
	NetDownText string  `json:"netDownText"`
	NetUpText   string  `json:"netUpText"`
	ClockText   string  `json:"clockText"`
	Timestamp   int64   `json:"timestamp"`
}

// Monitor manages the decoupled background goroutine that polls system metrics.
type Monitor struct {
	ctx           context.Context
	interval      time.Duration
	stopCh        chan struct{}
	mu            sync.RWMutex
	lastStats     Stats
	lastBytesRecv uint64
	lastBytesSent uint64
	lastNetSample time.Time
	running       bool
}

// NewMonitor creates an instance of the system resource monitor.
func NewMonitor(interval time.Duration) *Monitor {
	if interval <= 0 {
		interval = 1200 * time.Millisecond
	}
	return &Monitor{
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// Start launches the decoupled goroutine with a time.Ticker.
func (m *Monitor) Start(ctx context.Context) {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.ctx = ctx
	m.running = true
	m.stopCh = make(chan struct{})

	// Warm-up initial network counters
	if recv, sent, ok := readNetTotals(); ok {
		m.lastBytesRecv = recv
		m.lastBytesSent = sent
		m.lastNetSample = time.Now()
	}
	m.mu.Unlock()

	go m.run()
}

func (m *Monitor) run() {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	m.sampleAndEmit()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.sampleAndEmit()
		}
	}
}

func (m *Monitor) sampleAndEmit() {
	stats := m.Collect()

	m.mu.Lock()
	m.lastStats = stats
	m.mu.Unlock()

	if m.ctx != nil {
		wailsRuntime.EventsEmit(m.ctx, "system:stats", stats)
	}
}

// Collect reads the instant hardware and network metrics.
func (m *Monitor) Collect() Stats {
	// 1. CPU Percentage
	var cpuVal float64
	cpuPercentages, err := cpu.Percent(0, false)
	if err == nil && len(cpuPercentages) > 0 {
		cpuVal = math.Round(cpuPercentages[0]*10) / 10
	}

	// 2. RAM Usage
	var ramPercent float64
	var ramUsedMB, ramTotalMB uint64
	vMem, err := mem.VirtualMemory()
	if err == nil {
		ramPercent = math.Round(vMem.UsedPercent*10) / 10
		ramUsedMB = vMem.Used / (1024 * 1024)
		ramTotalMB = vMem.Total / (1024 * 1024)
	}

	// 3. Network I/O Speed
	var netDownKB, netUpKB float64
	now := time.Now()
	curRecv, curSent, ok := readNetTotals()

	m.mu.Lock()
	if ok && !m.lastNetSample.IsZero() {
		elapsed := now.Sub(m.lastNetSample).Seconds()
		if elapsed >= 0.05 {
			if curRecv >= m.lastBytesRecv {
				netDownKB = float64(curRecv-m.lastBytesRecv) / (1024.0 * elapsed)
			}
			if curSent >= m.lastBytesSent {
				netUpKB = float64(curSent-m.lastBytesSent) / (1024.0 * elapsed)
			}
			// Guard against counter resets / bogus overflow spikes (> 10 Gbps)
			const maxReasonableKBPerSec = 1250000.0
			if netDownKB > maxReasonableKBPerSec || math.IsNaN(netDownKB) || math.IsInf(netDownKB, 0) {
				netDownKB = 0
			}
			if netUpKB > maxReasonableKBPerSec || math.IsNaN(netUpKB) || math.IsInf(netUpKB, 0) {
				netUpKB = 0
			}
		}
	}
	if ok {
		m.lastBytesRecv = curRecv
		m.lastBytesSent = curSent
		m.lastNetSample = now
	}
	m.mu.Unlock()

	return Stats{
		CPUPercent:  cpuVal,
		RAMPercent:  ramPercent,
		RAMUsedMB:   ramUsedMB,
		RAMTotalMB:  ramTotalMB,
		NetDownKB:   math.Round(netDownKB*10) / 10,
		NetUpKB:     math.Round(netUpKB*10) / 10,
		NetDownText: formatSpeed(netDownKB),
		NetUpText:   formatSpeed(netUpKB),
		ClockText:   now.Format("15:04:05"),
		Timestamp:   now.Unix(),
	}
}

func readNetTotals() (recv uint64, sent uint64, ok bool) {
	counters, err := psnet.IOCounters(true)
	if err != nil || len(counters) == 0 {
		return 0, 0, false
	}
	for _, c := range counters {
		nameLower := strings.ToLower(c.Name)
		if strings.Contains(nameLower, "loopback") || nameLower == "lo" {
			continue
		}
		recv += normalizeWin386Counter(c.BytesRecv)
		sent += normalizeWin386Counter(c.BytesSent)
	}
	return recv, sent, true
}

// normalizeWin386Counter corrects the 4-byte struct alignment offset in MibIfRow2
// when compiled under 32-bit Windows (GOARCH=386), where 64-bit octet counters
// are shifted into the upper 32 bits.
func normalizeWin386Counter(v uint64) uint64 {
	if runtime.GOOS == "windows" && runtime.GOARCH == "386" {
		if v > 0xFFFFFFFF && uint32(v) == 0 {
			return v >> 32
		}
	}
	return v
}

func formatSpeed(kbPerSec float64) string {
	if kbPerSec < 0.1 {
		return "0 KB/s"
	}
	if kbPerSec < 1000 {
		return fmt.Sprintf("%.0f KB/s", kbPerSec)
	}
	mbPerSec := kbPerSec / 1024.0
	return fmt.Sprintf("%.1f MB/s", mbPerSec)
}

// GetLastStats returns the most recently collected system metrics.
func (m *Monitor) GetLastStats() Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastStats
}

// Stop cleanly terminates the background monitoring ticker.
func (m *Monitor) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return
	}
	close(m.stopCh)
	m.running = false
}
