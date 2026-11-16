package system

import (
	"testing"
	"time"
)

func TestMonitorNetSpeedReasonable(t *testing.T) {
	m := NewMonitor(500 * time.Millisecond)
	recv, sent, ok := readNetTotals()
	if !ok {
		t.Skip("no network interfaces found")
	}
	m.lastBytesRecv = recv
	m.lastBytesSent = sent
	m.lastNetSample = time.Now()

	time.Sleep(500 * time.Millisecond)
	stats := m.Collect()
	t.Logf("Down: %.1f KB/s (%s), Up: %.1f KB/s (%s)", stats.NetDownKB, stats.NetDownText, stats.NetUpKB, stats.NetUpText)
	if stats.NetDownKB > 1250000 || stats.NetUpKB > 1250000 {
		t.Fatalf("unexpectedly huge network speed: down=%f KB/s up=%f KB/s", stats.NetDownKB, stats.NetUpKB)
	}
}
