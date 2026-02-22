package service

import (
	"fmt"

	"plato/backend/internal/ports"
)

type Service struct {
	repo      ports.Repository
	telemetry ports.Telemetry
	importer  ports.ImportExport
}

func New(repo ports.Repository, telemetry ports.Telemetry, importer ports.ImportExport) (*Service, error) {
	if repo == nil {
		return nil, fmt.Errorf("new service: repository is nil")
	}
	if telemetry == nil {
		return nil, fmt.Errorf("new service: telemetry is nil")
	}
	if importer == nil {
		return nil, fmt.Errorf("new service: import/export is nil")
	}
	return &Service{repo: repo, telemetry: telemetry, importer: importer}, nil
}
