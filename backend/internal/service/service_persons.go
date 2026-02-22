package service

import (
	"context"
	"strings"

	"plato/backend/internal/domain"
	"plato/backend/internal/ports"
)

func (s *Service) ListPersons(ctx context.Context, auth ports.AuthContext) ([]domain.Person, error) {
	if err := requireAnyRole(auth, domain.RoleOrgAdmin, domain.RoleOrgUser); err != nil {
		return nil, err
	}
	organisationID, err := requiredOrganisationID(auth)
	if err != nil {
		return nil, err
	}
	return s.repo.ListPersons(ctx, organisationID)
}

func (s *Service) GetPerson(ctx context.Context, auth ports.AuthContext, personID string) (domain.Person, error) {
	if err := requireAnyRole(auth, domain.RoleOrgAdmin, domain.RoleOrgUser); err != nil {
		return domain.Person{}, err
	}
	organisationID, err := requiredOrganisationID(auth)
	if err != nil {
		return domain.Person{}, err
	}
	return s.repo.GetPerson(ctx, organisationID, personID)
}

func (s *Service) CreatePerson(ctx context.Context, auth ports.AuthContext, input domain.Person) (domain.Person, error) {
	if err := requireAnyRole(auth, domain.RoleOrgAdmin); err != nil {
		return domain.Person{}, err
	}
	organisationID, err := requiredOrganisationID(auth)
	if err != nil {
		return domain.Person{}, err
	}
	if err := validatePerson(input); err != nil {
		return domain.Person{}, err
	}
	if _, err := s.repo.GetOrganisation(ctx, organisationID); err != nil {
		return domain.Person{}, err
	}

	person := domain.Person{
		OrganisationID:               organisationID,
		Name:                         strings.TrimSpace(input.Name),
		EmploymentPct:                input.EmploymentPct,
		EmploymentEffectiveFromMonth: "",
	}

	created, err := s.repo.CreatePerson(ctx, person)
	if err != nil {
		return domain.Person{}, err
	}

	s.telemetry.Record("person.created", map[string]string{"person_id": created.ID})
	return created, nil
}

func (s *Service) UpdatePerson(ctx context.Context, auth ports.AuthContext, personID string, input domain.Person) (domain.Person, error) {
	if err := requireAnyRole(auth, domain.RoleOrgAdmin); err != nil {
		return domain.Person{}, err
	}
	organisationID, err := requiredOrganisationID(auth)
	if err != nil {
		return domain.Person{}, err
	}
	if err := validatePerson(input); err != nil {
		return domain.Person{}, err
	}

	person, err := s.repo.GetPerson(ctx, organisationID, personID)
	if err != nil {
		return domain.Person{}, err
	}
	person.Name = strings.TrimSpace(input.Name)
	effectiveFromMonth := strings.TrimSpace(input.EmploymentEffectiveFromMonth)
	if effectiveFromMonth == "" {
		person.EmploymentPct = input.EmploymentPct
	} else {
		normalizedMonth, err := domain.ValidateMonth(effectiveFromMonth)
		if err != nil {
			return domain.Person{}, domain.ErrValidation
		}
		person.EmploymentChanges = upsertEmploymentChange(person.EmploymentChanges, normalizedMonth, input.EmploymentPct)
	}
	person.EmploymentEffectiveFromMonth = ""
	if err := validatePerson(person); err != nil {
		return domain.Person{}, err
	}

	updated, err := s.repo.UpdatePerson(ctx, person)
	if err != nil {
		return domain.Person{}, err
	}

	s.telemetry.Record("person.updated", map[string]string{"person_id": updated.ID})
	return updated, nil
}

func (s *Service) DeletePerson(ctx context.Context, auth ports.AuthContext, personID string) error {
	if err := requireAnyRole(auth, domain.RoleOrgAdmin); err != nil {
		return err
	}
	organisationID, err := requiredOrganisationID(auth)
	if err != nil {
		return err
	}

	if err := s.repo.DeletePerson(ctx, organisationID, personID); err != nil {
		return err
	}

	s.telemetry.Record("person.deleted", map[string]string{"person_id": personID})
	return nil
}
