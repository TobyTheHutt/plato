package telemetry

import "testing"

func TestNoopTelemetryRecord(t *testing.T) {
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("unexpected panic: %v", recovered)
		}
	}()

	adapter := NewNoopTelemetry()
	adapter.Record("event.name", map[string]string{"k": "v"})
}
