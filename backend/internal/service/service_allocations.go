package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"plato/backend/internal/domain"
	"plato/backend/internal/ports"
)

func (s *Service) ListAllocations(ctx context.Context, auth ports.AuthContext) ([]domain.Allocation, error) {
	if err := requireAnyRole(auth, domain.RoleOrgAdmin, domain.RoleOrgUser); err != nil {
		return nil, err
	}
	organisationID, err := requiredOrganisationID(auth)
	if err != nil {
		return nil, err
	}
	return s.repo.ListAllocations(ctx, organisationID)
}

func (s *Service) GetAllocation(ctx context.Context, auth ports.AuthContext, allocationID string) (domain.Allocation, error) {
	if err := requireAnyRole(auth, domain.RoleOrgAdmin, domain.RoleOrgUser); err != nil {
		return domain.Allocation{}, err
	}
	organisationID, err := requiredOrganisationID(auth)
	if err != nil {
		return domain.Allocation{}, err
	}
	return s.repo.GetAllocation(ctx, organisationID, allocationID)
}

func (s *Service) CreateAllocation(ctx context.Context, auth ports.AuthContext, input domain.Allocation) (domain.Allocation, error) {
	if err := requireAnyRole(auth, domain.RoleOrgAdmin); err != nil {
		return domain.Allocation{}, err
	}
	organisationID, err := requiredOrganisationID(auth)
	if err != nil {
		return domain.Allocation{}, err
	}
	input = normalizeAllocationInput(input)
	err = validateAllocation(input)
	if err != nil {
		return domain.Allocation{}, err
	}
	project, err := s.repo.GetProject(ctx, organisationID, input.ProjectID)
	if err != nil {
		return domain.Allocation{}, err
	}
	err = validateAllocationWithinProjectRange(input, project)
	if err != nil {
		return domain.Allocation{}, err
	}

	targetPersonIDs, err := s.resolveAllocationTargetPersons(ctx, organisationID, input.TargetType, input.TargetID)
	if err != nil {
		return domain.Allocation{}, err
	}
	err = s.validateAllocationLimit(ctx, organisationID, input, targetPersonIDs, "")
	if err != nil {
		return domain.Allocation{}, err
	}

	allocation := domain.Allocation{
		OrganisationID: organisationID,
		TargetType:     input.TargetType,
		TargetID:       input.TargetID,
		ProjectID:      input.ProjectID,
		StartDate:      input.StartDate,
		EndDate:        input.EndDate,
		Percent:        input.Percent,
	}
	if input.TargetType == domain.AllocationTargetPerson {
		allocation.PersonID = input.TargetID
	}

	created, err := s.repo.CreateAllocation(ctx, allocation)
	if err != nil {
		return domain.Allocation{}, err
	}

	s.telemetry.Record("allocation.created", map[string]string{"allocation_id": created.ID})
	return created, nil
}

func (s *Service) UpdateAllocation(ctx context.Context, auth ports.AuthContext, allocationID string, input domain.Allocation) (domain.Allocation, error) {
	if err := requireAnyRole(auth, domain.RoleOrgAdmin); err != nil {
		return domain.Allocation{}, err
	}
	organisationID, err := requiredOrganisationID(auth)
	if err != nil {
		return domain.Allocation{}, err
	}
	input = normalizeAllocationInput(input)
	err = validateAllocation(input)
	if err != nil {
		return domain.Allocation{}, err
	}

	allocation, err := s.repo.GetAllocation(ctx, organisationID, allocationID)
	if err != nil {
		return domain.Allocation{}, err
	}
	project, err := s.repo.GetProject(ctx, organisationID, input.ProjectID)
	if err != nil {
		return domain.Allocation{}, err
	}
	err = validateAllocationWithinProjectRange(input, project)
	if err != nil {
		return domain.Allocation{}, err
	}

	targetPersonIDs, err := s.resolveAllocationTargetPersons(ctx, organisationID, input.TargetType, input.TargetID)
	if err != nil {
		return domain.Allocation{}, err
	}
	err = s.validateAllocationLimit(ctx, organisationID, input, targetPersonIDs, allocationID)
	if err != nil {
		return domain.Allocation{}, err
	}

	allocation.TargetType = input.TargetType
	allocation.TargetID = input.TargetID
	allocation.ProjectID = input.ProjectID
	allocation.StartDate = input.StartDate
	allocation.EndDate = input.EndDate
	allocation.Percent = input.Percent
	if input.TargetType == domain.AllocationTargetPerson {
		allocation.PersonID = input.TargetID
	} else {
		allocation.PersonID = ""
	}

	updated, err := s.repo.UpdateAllocation(ctx, allocation)
	if err != nil {
		return domain.Allocation{}, err
	}

	s.telemetry.Record("allocation.updated", map[string]string{"allocation_id": updated.ID})
	return updated, nil
}

func (s *Service) DeleteAllocation(ctx context.Context, auth ports.AuthContext, allocationID string) error {
	if err := requireAnyRole(auth, domain.RoleOrgAdmin); err != nil {
		return err
	}
	organisationID, err := requiredOrganisationID(auth)
	if err != nil {
		return err
	}

	err = s.repo.DeleteAllocation(ctx, organisationID, allocationID)
	if err != nil {
		return err
	}

	s.telemetry.Record("allocation.deleted", map[string]string{"allocation_id": allocationID})
	return nil
}

func (s *Service) validateAllocationLimit(
	ctx context.Context,
	organisationID string,
	candidate domain.Allocation,
	candidatePersonIDs []string,
	allocationID string,
) error {
	candidateStart, candidateEnd, err := parseDateRange(candidate.StartDate, candidate.EndDate)
	if err != nil {
		return domain.ErrValidation
	}

	allocations, err := s.repo.ListAllocations(ctx, organisationID)
	if err != nil {
		return err
	}
	groups, err := s.repo.ListGroups(ctx, organisationID)
	if err != nil {
		return err
	}
	organisation, err := s.repo.GetOrganisation(ctx, organisationID)
	if err != nil {
		return err
	}
	if organisation.HoursPerDay <= 0 {
		return domain.ErrValidation
	}
	maxPercentPerDay := (24.0 * 100.0) / organisation.HoursPerDay

	groupsByID := make(map[string]domain.Group, len(groups))
	for _, group := range groups {
		groupsByID[group.ID] = group
	}

	for _, personID := range candidatePersonIDs {
		_, err = s.repo.GetPerson(ctx, organisationID, personID)
		if err != nil {
			return err
		}

		total := candidate.Percent
		if total > maxPercentPerDay+1e-9 {
			return fmt.Errorf("allocation exceeds 24 hours/day theoretical limit: %w", domain.ErrValidation)
		}

		events := make(map[time.Time]float64)
		for _, allocation := range allocations {
			if allocation.ID == allocationID {
				continue
			}
			if !allocationTargetsPerson(allocation, personID, groupsByID) {
				continue
			}

			var existingStart, existingEnd time.Time
			existingStart, existingEnd, err = parseDateRange(allocation.StartDate, allocation.EndDate)
			if err != nil {
				return domain.ErrValidation
			}

			overlapStart := existingStart
			if overlapStart.Before(candidateStart) {
				overlapStart = candidateStart
			}
			overlapEnd := existingEnd
			if overlapEnd.After(candidateEnd) {
				overlapEnd = candidateEnd
			}
			if overlapEnd.Before(overlapStart) {
				continue
			}

			events[overlapStart] += allocation.Percent
			events[overlapEnd.AddDate(0, 0, 1)] -= allocation.Percent
		}

		eventDates := make([]time.Time, 0, len(events))
		for eventDate := range events {
			eventDates = append(eventDates, eventDate)
		}
		sort.Slice(eventDates, func(i int, j int) bool {
			return eventDates[i].Before(eventDates[j])
		})

		for _, eventDate := range eventDates {
			if eventDate.After(candidateEnd) {
				break
			}
			total += events[eventDate]
			if total > maxPercentPerDay+1e-9 {
				return fmt.Errorf("allocation exceeds 24 hours/day theoretical limit: %w", domain.ErrValidation)
			}
		}
	}

	return nil
}

func normalizeAllocationInput(input domain.Allocation) domain.Allocation {
	input.TargetType = strings.TrimSpace(input.TargetType)
	input.TargetID = strings.TrimSpace(input.TargetID)
	if input.TargetType == "" && strings.TrimSpace(input.PersonID) != "" {
		input.TargetType = domain.AllocationTargetPerson
		input.TargetID = strings.TrimSpace(input.PersonID)
	}
	if input.TargetType == domain.AllocationTargetPerson {
		input.PersonID = input.TargetID
	}
	return input
}

func (s *Service) resolveAllocationTargetPersons(
	ctx context.Context,
	organisationID string,
	targetType string,
	targetID string,
) ([]string, error) {
	switch targetType {
	case domain.AllocationTargetPerson:
		if _, err := s.repo.GetPerson(ctx, organisationID, targetID); err != nil {
			return nil, err
		}
		return []string{targetID}, nil
	case domain.AllocationTargetGroup:
		group, err := s.repo.GetGroup(ctx, organisationID, targetID)
		if err != nil {
			return nil, err
		}
		if len(group.MemberIDs) == 0 {
			return nil, domain.ErrValidation
		}
		return uniqueStringIDs(group.MemberIDs), nil
	default:
		return nil, domain.ErrValidation
	}
}

func allocationTargetsPerson(allocation domain.Allocation, personID string, groupsByID map[string]domain.Group) bool {
	targetType, targetID := normalizedAllocationTarget(allocation)
	switch targetType {
	case domain.AllocationTargetPerson:
		return targetID == personID
	case domain.AllocationTargetGroup:
		group, ok := groupsByID[targetID]
		if !ok {
			return false
		}
		for _, memberID := range group.MemberIDs {
			if memberID == personID {
				return true
			}
		}
		return false
	default:
		return false
	}
}

func normalizedAllocationTarget(allocation domain.Allocation) (string, string) {
	targetType := strings.TrimSpace(allocation.TargetType)
	targetID := strings.TrimSpace(allocation.TargetID)
	if targetType == "" && strings.TrimSpace(allocation.PersonID) != "" {
		return domain.AllocationTargetPerson, strings.TrimSpace(allocation.PersonID)
	}
	return targetType, targetID
}

func validateAllocationWithinProjectRange(allocation domain.Allocation, project domain.Project) error {
	projectStart, projectEnd, err := parseDateRange(project.StartDate, project.EndDate)
	if err != nil {
		return domain.ErrValidation
	}
	allocationStart, allocationEnd, err := parseDateRange(allocation.StartDate, allocation.EndDate)
	if err != nil {
		return domain.ErrValidation
	}
	if allocationStart.Before(projectStart) || allocationEnd.After(projectEnd) {
		return domain.ErrValidation
	}
	return nil
}

func uniqueStringIDs(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}
