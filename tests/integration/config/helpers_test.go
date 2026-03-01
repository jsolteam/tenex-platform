package config_integration

import (
	"testing"
	"time"
)

// waitFor блокирует до тех пор, пока cond() не вернёт true или не истечёт timeout.
func waitFor(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
}
