package telemetry

import "plato/backend/internal/ports"

type NoopTelemetry struct{}

var _ ports.Telemetry = (*NoopTelemetry)(nil)

func NewNoopTelemetry() *NoopTelemetry {
	return &NoopTelemetry{}
}

func (n *NoopTelemetry) Record(_ string, _ map[string]string) {}
