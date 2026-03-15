package telemetry

import "testing"

// TestNoopTelemetryRecord verifies the no-op telemetry record scenario.
func TestNoopTelemetryRecord(t *testing.T) {
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("unexpected panic: %v", recovered)
		}
	}()

	adapter := NewNoopTelemetry()
	adapter.Record("event.name", map[string]string{"k": "v"})
}
