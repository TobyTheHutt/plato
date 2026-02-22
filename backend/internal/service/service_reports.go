package service

import (
	"context"
	"fmt"

	"plato/backend/internal/domain"
	"plato/backend/internal/ports"
)

func (s *Service) ReportAvailabilityAndLoad(ctx context.Context, auth ports.AuthContext, request domain.ReportRequest) ([]domain.ReportBucket, error) {
	if err := requireAnyRole(auth, domain.RoleOrgAdmin, domain.RoleOrgUser); err != nil {
		return nil, err
	}
	organisationID, err := requiredOrganisationID(auth)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateScope(request.Scope); err != nil {
		return nil, err
	}
	if err := domain.ValidateGranularity(request.Granularity); err != nil {
		return nil, err
	}
	fromDate, err := domain.ValidateDate(request.FromDate)
	if err != nil {
		return nil, fmt.Errorf("from date: %v: %w", err, domain.ErrValidation)
	}
	toDate, err := domain.ValidateDate(request.ToDate)
	if err != nil {
		return nil, fmt.Errorf("to date: %v: %w", err, domain.ErrValidation)
	}
	if fromDate > toDate {
		return nil, domain.ErrValidation
	}

	organisation, err := s.repo.GetOrganisation(ctx, organisationID)
	if err != nil {
		return nil, err
	}
	persons, err := s.repo.ListPersons(ctx, organisationID)
	if err != nil {
		return nil, err
	}
	projects, err := s.repo.ListProjects(ctx, organisationID)
	if err != nil {
		return nil, err
	}
	groups, err := s.repo.ListGroups(ctx, organisationID)
	if err != nil {
		return nil, err
	}
	allocations, err := s.repo.ListAllocations(ctx, organisationID)
	if err != nil {
		return nil, err
	}
	orgHolidays, err := s.repo.ListOrgHolidays(ctx, organisationID)
	if err != nil {
		return nil, err
	}
	groupUnavailability, err := s.repo.ListGroupUnavailability(ctx, organisationID)
	if err != nil {
		return nil, err
	}
	personUnavailability, err := s.repo.ListPersonUnavailability(ctx, organisationID)
	if err != nil {
		return nil, err
	}

	if err := validateScopeIDs(request, persons, groups, projects); err != nil {
		return nil, err
	}

	result, err := domain.CalculateAvailabilityLoad(domain.CalculationInput{
		Organisation:         organisation,
		Persons:              persons,
		Projects:             projects,
		Groups:               groups,
		Allocations:          allocations,
		OrgHolidays:          orgHolidays,
		GroupUnavailability:  groupUnavailability,
		PersonUnavailability: personUnavailability,
		Request:              request,
	})
	if err != nil {
		return nil, err
	}

	s.telemetry.Record("report.generated", map[string]string{"scope": request.Scope})
	return result, nil
}

func validateScopeIDs(request domain.ReportRequest, persons []domain.Person, groups []domain.Group, projects []domain.Project) error {
	if len(request.IDs) == 0 {
		return nil
	}

	lookup := map[string]bool{}
	switch request.Scope {
	case domain.ScopePerson:
		for _, person := range persons {
			lookup[person.ID] = true
		}
	case domain.ScopeGroup:
		for _, group := range groups {
			lookup[group.ID] = true
		}
	case domain.ScopeProject:
		for _, project := range projects {
			lookup[project.ID] = true
		}
	case domain.ScopeOrganisation:
		return nil
	default:
		return domain.ErrValidation
	}

	for _, id := range request.IDs {
		if !lookup[id] {
			return domain.ErrNotFound
		}
	}

	return nil
}
