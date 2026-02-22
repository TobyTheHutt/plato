package service

import (
	"context"
	"fmt"

	"plato/backend/internal/domain"
	"plato/backend/internal/ports"
)

func (s *Service) ListOrgHolidays(ctx context.Context, auth ports.AuthContext) ([]domain.OrgHoliday, error) {
	if err := requireAnyRole(auth, domain.RoleOrgAdmin, domain.RoleOrgUser); err != nil {
		return nil, err
	}
	organisationID, err := requiredOrganisationID(auth)
	if err != nil {
		return nil, err
	}
	return s.repo.ListOrgHolidays(ctx, organisationID)
}

func (s *Service) CreateOrgHoliday(ctx context.Context, auth ports.AuthContext, input domain.OrgHoliday) (domain.OrgHoliday, error) {
	if err := requireAnyRole(auth, domain.RoleOrgAdmin); err != nil {
		return domain.OrgHoliday{}, err
	}
	organisationID, err := requiredOrganisationID(auth)
	if err != nil {
		return domain.OrgHoliday{}, err
	}
	organisation, err := s.repo.GetOrganisation(ctx, organisationID)
	if err != nil {
		return domain.OrgHoliday{}, err
	}
	if err := validateDateHours(input.Date, input.Hours, organisation.HoursPerDay); err != nil {
		return domain.OrgHoliday{}, err
	}

	entry := domain.OrgHoliday{
		OrganisationID: organisationID,
		Date:           input.Date,
		Hours:          input.Hours,
	}

	created, err := s.repo.CreateOrgHoliday(ctx, entry)
	if err != nil {
		return domain.OrgHoliday{}, err
	}

	s.telemetry.Record("holiday.created", map[string]string{"holiday_id": created.ID})
	return created, nil
}

func (s *Service) DeleteOrgHoliday(ctx context.Context, auth ports.AuthContext, holidayID string) error {
	if err := requireAnyRole(auth, domain.RoleOrgAdmin); err != nil {
		return err
	}
	organisationID, err := requiredOrganisationID(auth)
	if err != nil {
		return err
	}

	if err := s.repo.DeleteOrgHoliday(ctx, organisationID, holidayID); err != nil {
		return err
	}

	s.telemetry.Record("holiday.deleted", map[string]string{"holiday_id": holidayID})
	return nil
}

func (s *Service) ListGroupUnavailability(ctx context.Context, auth ports.AuthContext) ([]domain.GroupUnavailability, error) {
	if err := requireAnyRole(auth, domain.RoleOrgAdmin, domain.RoleOrgUser); err != nil {
		return nil, err
	}
	organisationID, err := requiredOrganisationID(auth)
	if err != nil {
		return nil, err
	}
	return s.repo.ListGroupUnavailability(ctx, organisationID)
}

func (s *Service) CreateGroupUnavailability(ctx context.Context, auth ports.AuthContext, input domain.GroupUnavailability) (domain.GroupUnavailability, error) {
	if err := requireAnyRole(auth, domain.RoleOrgAdmin); err != nil {
		return domain.GroupUnavailability{}, err
	}
	organisationID, err := requiredOrganisationID(auth)
	if err != nil {
		return domain.GroupUnavailability{}, err
	}
	organisation, err := s.repo.GetOrganisation(ctx, organisationID)
	if err != nil {
		return domain.GroupUnavailability{}, err
	}
	if _, err := s.repo.GetGroup(ctx, organisationID, input.GroupID); err != nil {
		return domain.GroupUnavailability{}, err
	}
	if err := validateDateHours(input.Date, input.Hours, organisation.HoursPerDay); err != nil {
		return domain.GroupUnavailability{}, err
	}

	entry := domain.GroupUnavailability{
		OrganisationID: organisationID,
		GroupID:        input.GroupID,
		Date:           input.Date,
		Hours:          input.Hours,
	}

	created, err := s.repo.CreateGroupUnavailability(ctx, entry)
	if err != nil {
		return domain.GroupUnavailability{}, err
	}

	s.telemetry.Record("group_unavailability.created", map[string]string{"entry_id": created.ID})
	return created, nil
}

func (s *Service) DeleteGroupUnavailability(ctx context.Context, auth ports.AuthContext, entryID string) error {
	if err := requireAnyRole(auth, domain.RoleOrgAdmin); err != nil {
		return err
	}
	organisationID, err := requiredOrganisationID(auth)
	if err != nil {
		return err
	}

	if err := s.repo.DeleteGroupUnavailability(ctx, organisationID, entryID); err != nil {
		return err
	}

	s.telemetry.Record("group_unavailability.deleted", map[string]string{"entry_id": entryID})
	return nil
}

func (s *Service) ListPersonUnavailability(ctx context.Context, auth ports.AuthContext) ([]domain.PersonUnavailability, error) {
	if err := requireAnyRole(auth, domain.RoleOrgAdmin, domain.RoleOrgUser); err != nil {
		return nil, err
	}
	organisationID, err := requiredOrganisationID(auth)
	if err != nil {
		return nil, err
	}
	return s.repo.ListPersonUnavailability(ctx, organisationID)
}

func (s *Service) ListPersonUnavailabilityByPerson(ctx context.Context, auth ports.AuthContext, personID string) ([]domain.PersonUnavailability, error) {
	if err := requireAnyRole(auth, domain.RoleOrgAdmin, domain.RoleOrgUser); err != nil {
		return nil, err
	}
	organisationID, err := requiredOrganisationID(auth)
	if err != nil {
		return nil, err
	}
	return s.repo.ListPersonUnavailabilityByPerson(ctx, organisationID, personID)
}

func (s *Service) CreatePersonUnavailability(ctx context.Context, auth ports.AuthContext, input domain.PersonUnavailability) (domain.PersonUnavailability, error) {
	if err := requireAnyRole(auth, domain.RoleOrgAdmin); err != nil {
		return domain.PersonUnavailability{}, err
	}
	organisationID, err := requiredOrganisationID(auth)
	if err != nil {
		return domain.PersonUnavailability{}, err
	}
	organisation, err := s.repo.GetOrganisation(ctx, organisationID)
	if err != nil {
		return domain.PersonUnavailability{}, err
	}
	person, err := s.repo.GetPerson(ctx, organisationID, input.PersonID)
	if err != nil {
		return domain.PersonUnavailability{}, err
	}

	employmentPct, err := domain.EmploymentPctOnDate(person, input.Date)
	if err != nil {
		return domain.PersonUnavailability{}, fmt.Errorf("person employment on date: %w", err)
	}
	personDailyHours := organisation.HoursPerDay * employmentPct / 100
	if err := validateDateHours(input.Date, input.Hours, personDailyHours); err != nil {
		return domain.PersonUnavailability{}, err
	}

	entry := domain.PersonUnavailability{
		OrganisationID: organisationID,
		PersonID:       input.PersonID,
		Date:           input.Date,
		Hours:          input.Hours,
	}

	created, err := s.repo.CreatePersonUnavailabilityWithDailyLimit(ctx, entry, personDailyHours)
	if err != nil {
		return domain.PersonUnavailability{}, err
	}

	s.telemetry.Record("person_unavailability.created", map[string]string{"entry_id": created.ID})
	return created, nil
}

func (s *Service) DeletePersonUnavailability(ctx context.Context, auth ports.AuthContext, entryID string) error {
	if err := requireAnyRole(auth, domain.RoleOrgAdmin); err != nil {
		return err
	}
	organisationID, err := requiredOrganisationID(auth)
	if err != nil {
		return err
	}

	if err := s.repo.DeletePersonUnavailability(ctx, organisationID, entryID); err != nil {
		return err
	}

	s.telemetry.Record("person_unavailability.deleted", map[string]string{"entry_id": entryID})
	return nil
}

func (s *Service) DeletePersonUnavailabilityByPerson(ctx context.Context, auth ports.AuthContext, personID, entryID string) error {
	if err := requireAnyRole(auth, domain.RoleOrgAdmin); err != nil {
		return err
	}
	organisationID, err := requiredOrganisationID(auth)
	if err != nil {
		return err
	}

	if err := s.repo.DeletePersonUnavailabilityByPerson(ctx, organisationID, personID, entryID); err != nil {
		return err
	}

	s.telemetry.Record("person_unavailability.deleted", map[string]string{"entry_id": entryID})
	return nil
}
