package service

import (
	"context"
	"errors"
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
	if validationErr := validateReportRequest(request); validationErr != nil {
		return nil, validationErr
	}

	calculationInput, err := s.loadReportCalculationInput(ctx, organisationID, request)
	if err != nil {
		return nil, err
	}

	result, err := domain.CalculateAvailabilityLoad(calculationInput)
	if err != nil {
		return nil, err
	}

	s.telemetry.Record("report.generated", map[string]string{"scope": request.Scope})
	return result, nil
}

func validateReportRequest(request domain.ReportRequest) error {
	if err := domain.ValidateScope(request.Scope); err != nil {
		return err
	}
	if err := domain.ValidateGranularity(request.Granularity); err != nil {
		return err
	}
	fromDate, err := domain.ValidateDate(request.FromDate)
	if err != nil {
		return errors.Join(domain.ErrValidation, fmt.Errorf("from date: %w", err))
	}
	toDate, err := domain.ValidateDate(request.ToDate)
	if err != nil {
		return errors.Join(domain.ErrValidation, fmt.Errorf("to date: %w", err))
	}
	if fromDate > toDate {
		return errors.Join(domain.ErrValidation, fmt.Errorf("invalid date range: from %s is after to %s", fromDate, toDate))
	}
	return nil
}

func (s *Service) loadReportCalculationInput(ctx context.Context, organisationID string, request domain.ReportRequest) (domain.CalculationInput, error) {
	organisation, err := s.repo.GetOrganisation(ctx, organisationID)
	if err != nil {
		return domain.CalculationInput{}, fmt.Errorf("get organisation %s: %w", organisationID, err)
	}
	persons, err := s.repo.ListPersons(ctx, organisationID)
	if err != nil {
		return domain.CalculationInput{}, fmt.Errorf("list persons for organisation %s: %w", organisationID, err)
	}
	projects, err := s.repo.ListProjects(ctx, organisationID)
	if err != nil {
		return domain.CalculationInput{}, fmt.Errorf("list projects for organisation %s: %w", organisationID, err)
	}
	groups, err := s.repo.ListGroups(ctx, organisationID)
	if err != nil {
		return domain.CalculationInput{}, fmt.Errorf("list groups for organisation %s: %w", organisationID, err)
	}
	allocations, err := s.repo.ListAllocations(ctx, organisationID)
	if err != nil {
		return domain.CalculationInput{}, fmt.Errorf("list allocations for organisation %s: %w", organisationID, err)
	}
	orgHolidays, err := s.repo.ListOrgHolidays(ctx, organisationID)
	if err != nil {
		return domain.CalculationInput{}, fmt.Errorf("list organisation holidays for organisation %s: %w", organisationID, err)
	}
	groupUnavailability, err := s.repo.ListGroupUnavailability(ctx, organisationID)
	if err != nil {
		return domain.CalculationInput{}, fmt.Errorf("list group unavailability for organisation %s: %w", organisationID, err)
	}
	personUnavailability, err := s.repo.ListPersonUnavailability(ctx, organisationID)
	if err != nil {
		return domain.CalculationInput{}, fmt.Errorf("list person unavailability for organisation %s: %w", organisationID, err)
	}
	if scopeErr := validateScopeIDs(request, persons, groups, projects); scopeErr != nil {
		return domain.CalculationInput{}, scopeErr
	}

	return domain.CalculationInput{
		Organisation:         organisation,
		Persons:              persons,
		Projects:             projects,
		Groups:               groups,
		Allocations:          allocations,
		OrgHolidays:          orgHolidays,
		GroupUnavailability:  groupUnavailability,
		PersonUnavailability: personUnavailability,
		Request:              request,
	}, nil
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
