package telemetry

import "testing"

func TestNoopTelemetryRecord(t *testing.T) {
	adapter := NewNoopTelemetry()
	adapter.Record("event.name", map[string]string{"k": "v"})
}
