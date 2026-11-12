package hotkey

import (
	"testing"
	"time"
)

func TestLowLevelHookLifecycle(t *testing.T) {
	called := false
	m := NewManager(func() {
		called = true
	})

	err := m.Start()
	if err != nil {
		t.Fatalf("Failed to start hotkey hook: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	m.Stop()
	t.Logf("Hook started and stopped cleanly, callback state: %v", called)
}
