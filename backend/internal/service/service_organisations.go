package service

import (
	"context"
	"strings"

	"plato/backend/internal/domain"
	"plato/backend/internal/ports"
)

func (s *Service) ListOrganisations(ctx context.Context, auth ports.AuthContext) ([]domain.Organisation, error) {
	if err := requireAnyRole(auth, domain.RoleOrgAdmin, domain.RoleOrgUser); err != nil {
		return nil, err
	}

	organisations, err := s.repo.ListOrganisations(ctx)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(auth.OrganisationID) == "" {
		return organisations, nil
	}

	for _, organisation := range organisations {
		if organisation.ID == auth.OrganisationID {
			return []domain.Organisation{organisation}, nil
		}
	}

	return []domain.Organisation{}, nil
}

func (s *Service) GetOrganisation(ctx context.Context, auth ports.AuthContext, organisationID string) (domain.Organisation, error) {
	if err := requireAnyRole(auth, domain.RoleOrgAdmin, domain.RoleOrgUser); err != nil {
		return domain.Organisation{}, err
	}
	if err := enforceTenant(auth, organisationID); err != nil {
		return domain.Organisation{}, err
	}

	organisation, err := s.repo.GetOrganisation(ctx, organisationID)
	if err != nil {
		return domain.Organisation{}, err
	}

	return organisation, nil
}

func (s *Service) CreateOrganisation(ctx context.Context, auth ports.AuthContext, input domain.Organisation) (domain.Organisation, error) {
	if err := requireAnyRole(auth, domain.RoleOrgAdmin); err != nil {
		return domain.Organisation{}, err
	}
	if err := validateOrganisation(input); err != nil {
		return domain.Organisation{}, err
	}

	created, err := s.repo.CreateOrganisation(ctx, domain.Organisation{
		Name:         strings.TrimSpace(input.Name),
		HoursPerDay:  input.HoursPerDay,
		HoursPerWeek: input.HoursPerWeek,
		HoursPerYear: input.HoursPerYear,
	})
	if err != nil {
		return domain.Organisation{}, err
	}

	s.telemetry.Record("organisation.created", map[string]string{"organisation_id": created.ID})
	return created, nil
}

func (s *Service) UpdateOrganisation(ctx context.Context, auth ports.AuthContext, organisationID string, input domain.Organisation) (domain.Organisation, error) {
	if err := requireAnyRole(auth, domain.RoleOrgAdmin); err != nil {
		return domain.Organisation{}, err
	}
	if err := enforceTenant(auth, organisationID); err != nil {
		return domain.Organisation{}, err
	}
	if err := validateOrganisation(input); err != nil {
		return domain.Organisation{}, err
	}

	current, err := s.repo.GetOrganisation(ctx, organisationID)
	if err != nil {
		return domain.Organisation{}, err
	}

	current.Name = strings.TrimSpace(input.Name)
	current.HoursPerDay = input.HoursPerDay
	current.HoursPerWeek = input.HoursPerWeek
	current.HoursPerYear = input.HoursPerYear

	updated, err := s.repo.UpdateOrganisation(ctx, current)
	if err != nil {
		return domain.Organisation{}, err
	}

	s.telemetry.Record("organisation.updated", map[string]string{"organisation_id": updated.ID})
	return updated, nil
}

func (s *Service) DeleteOrganisation(ctx context.Context, auth ports.AuthContext, organisationID string) error {
	if err := requireAnyRole(auth, domain.RoleOrgAdmin); err != nil {
		return err
	}
	if err := enforceTenant(auth, organisationID); err != nil {
		return err
	}

	if err := s.repo.DeleteOrganisation(ctx, organisationID); err != nil {
		return err
	}

	s.telemetry.Record("organisation.deleted", map[string]string{"organisation_id": organisationID})
	return nil
}
