package service

import (
	"errors"

	"plato/backend/internal/ports"
)

// Service coordinates business logic across repository and adapter ports.
type Service struct {
	repo      ports.Repository
	telemetry ports.Telemetry
	importer  ports.ImportExport
}

// New returns a Service from the required repository and adapter dependencies.
func New(repo ports.Repository, telemetry ports.Telemetry, importer ports.ImportExport) (*Service, error) {
	if repo == nil {
		return nil, errors.New("new service: repository is nil")
	}
	if telemetry == nil {
		return nil, errors.New("new service: telemetry is nil")
	}
	if importer == nil {
		return nil, errors.New("new service: import/export is nil")
	}
	return &Service{repo: repo, telemetry: telemetry, importer: importer}, nil
}
