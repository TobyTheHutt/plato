package telemetry

import "plato/backend/internal/ports"

// NoopTelemetry is a telemetry adapter that discards all events.
type NoopTelemetry struct{}

var _ ports.Telemetry = (*NoopTelemetry)(nil)

// NewNoopTelemetry returns a telemetry adapter that records nothing.
func NewNoopTelemetry() *NoopTelemetry {
	return &NoopTelemetry{}
}

// Record discards the provided telemetry event.
func (n *NoopTelemetry) Record(_ string, _ map[string]string) {}
