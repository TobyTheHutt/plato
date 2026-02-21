package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"plato/backend/internal/domain"
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

func (s *Service) ListProjects(ctx context.Context, auth ports.AuthContext) ([]domain.Project, error) {
	if err := requireAnyRole(auth, domain.RoleOrgAdmin, domain.RoleOrgUser); err != nil {
		return nil, err
	}
	organisationID, err := requiredOrganisationID(auth)
	if err != nil {
		return nil, err
	}
	return s.repo.ListProjects(ctx, organisationID)
}

func (s *Service) GetProject(ctx context.Context, auth ports.AuthContext, projectID string) (domain.Project, error) {
	if err := requireAnyRole(auth, domain.RoleOrgAdmin, domain.RoleOrgUser); err != nil {
		return domain.Project{}, err
	}
	organisationID, err := requiredOrganisationID(auth)
	if err != nil {
		return domain.Project{}, err
	}
	return s.repo.GetProject(ctx, organisationID, projectID)
}

func (s *Service) CreateProject(ctx context.Context, auth ports.AuthContext, input domain.Project) (domain.Project, error) {
	if err := requireAnyRole(auth, domain.RoleOrgAdmin); err != nil {
		return domain.Project{}, err
	}
	organisationID, err := requiredOrganisationID(auth)
	if err != nil {
		return domain.Project{}, err
	}
	if err := validateProject(input); err != nil {
		return domain.Project{}, err
	}

	project := domain.Project{
		OrganisationID:       organisationID,
		Name:                 strings.TrimSpace(input.Name),
		StartDate:            input.StartDate,
		EndDate:              input.EndDate,
		EstimatedEffortHours: input.EstimatedEffortHours,
	}

	created, err := s.repo.CreateProject(ctx, project)
	if err != nil {
		return domain.Project{}, err
	}

	s.telemetry.Record("project.created", map[string]string{"project_id": created.ID})
	return created, nil
}

func (s *Service) UpdateProject(ctx context.Context, auth ports.AuthContext, projectID string, input domain.Project) (domain.Project, error) {
	if err := requireAnyRole(auth, domain.RoleOrgAdmin); err != nil {
		return domain.Project{}, err
	}
	organisationID, err := requiredOrganisationID(auth)
	if err != nil {
		return domain.Project{}, err
	}
	if err := validateProject(input); err != nil {
		return domain.Project{}, err
	}

	project, err := s.repo.GetProject(ctx, organisationID, projectID)
	if err != nil {
		return domain.Project{}, err
	}
	project.Name = strings.TrimSpace(input.Name)
	project.StartDate = input.StartDate
	project.EndDate = input.EndDate
	project.EstimatedEffortHours = input.EstimatedEffortHours

	updated, err := s.repo.UpdateProject(ctx, project)
	if err != nil {
		return domain.Project{}, err
	}

	s.telemetry.Record("project.updated", map[string]string{"project_id": updated.ID})
	return updated, nil
}

func (s *Service) DeleteProject(ctx context.Context, auth ports.AuthContext, projectID string) error {
	if err := requireAnyRole(auth, domain.RoleOrgAdmin); err != nil {
		return err
	}
	organisationID, err := requiredOrganisationID(auth)
	if err != nil {
		return err
	}

	if err := s.repo.DeleteProject(ctx, organisationID, projectID); err != nil {
		return err
	}

	s.telemetry.Record("project.deleted", map[string]string{"project_id": projectID})
	return nil
}

func (s *Service) ListGroups(ctx context.Context, auth ports.AuthContext) ([]domain.Group, error) {
	if err := requireAnyRole(auth, domain.RoleOrgAdmin, domain.RoleOrgUser); err != nil {
		return nil, err
	}
	organisationID, err := requiredOrganisationID(auth)
	if err != nil {
		return nil, err
	}
	return s.repo.ListGroups(ctx, organisationID)
}

func (s *Service) GetGroup(ctx context.Context, auth ports.AuthContext, groupID string) (domain.Group, error) {
	if err := requireAnyRole(auth, domain.RoleOrgAdmin, domain.RoleOrgUser); err != nil {
		return domain.Group{}, err
	}
	organisationID, err := requiredOrganisationID(auth)
	if err != nil {
		return domain.Group{}, err
	}
	return s.repo.GetGroup(ctx, organisationID, groupID)
}

func (s *Service) CreateGroup(ctx context.Context, auth ports.AuthContext, input domain.Group) (domain.Group, error) {
	if err := requireAnyRole(auth, domain.RoleOrgAdmin); err != nil {
		return domain.Group{}, err
	}
	organisationID, err := requiredOrganisationID(auth)
	if err != nil {
		return domain.Group{}, err
	}
	if err := validateGroup(input); err != nil {
		return domain.Group{}, err
	}
	if err := s.ensureMembersBelongToOrg(ctx, organisationID, input.MemberIDs); err != nil {
		return domain.Group{}, err
	}

	group := domain.Group{
		OrganisationID: organisationID,
		Name:           strings.TrimSpace(input.Name),
		MemberIDs:      input.MemberIDs,
	}

	created, err := s.repo.CreateGroup(ctx, group)
	if err != nil {
		return domain.Group{}, err
	}

	s.telemetry.Record("group.created", map[string]string{"group_id": created.ID})
	return created, nil
}

func (s *Service) UpdateGroup(ctx context.Context, auth ports.AuthContext, groupID string, input domain.Group) (domain.Group, error) {
	if err := requireAnyRole(auth, domain.RoleOrgAdmin); err != nil {
		return domain.Group{}, err
	}
	organisationID, err := requiredOrganisationID(auth)
	if err != nil {
		return domain.Group{}, err
	}
	if err := validateGroup(input); err != nil {
		return domain.Group{}, err
	}
	if err := s.ensureMembersBelongToOrg(ctx, organisationID, input.MemberIDs); err != nil {
		return domain.Group{}, err
	}

	group, err := s.repo.GetGroup(ctx, organisationID, groupID)
	if err != nil {
		return domain.Group{}, err
	}
	group.Name = strings.TrimSpace(input.Name)
	group.MemberIDs = input.MemberIDs

	updated, err := s.repo.UpdateGroup(ctx, group)
	if err != nil {
		return domain.Group{}, err
	}

	s.telemetry.Record("group.updated", map[string]string{"group_id": updated.ID})
	return updated, nil
}

func (s *Service) DeleteGroup(ctx context.Context, auth ports.AuthContext, groupID string) error {
	if err := requireAnyRole(auth, domain.RoleOrgAdmin); err != nil {
		return err
	}
	organisationID, err := requiredOrganisationID(auth)
	if err != nil {
		return err
	}

	if err := s.repo.DeleteGroup(ctx, organisationID, groupID); err != nil {
		return err
	}

	s.telemetry.Record("group.deleted", map[string]string{"group_id": groupID})
	return nil
}

func (s *Service) AddGroupMember(ctx context.Context, auth ports.AuthContext, groupID, personID string) (domain.Group, error) {
	if err := requireAnyRole(auth, domain.RoleOrgAdmin); err != nil {
		return domain.Group{}, err
	}
	organisationID, err := requiredOrganisationID(auth)
	if err != nil {
		return domain.Group{}, err
	}
	if _, err := s.repo.GetPerson(ctx, organisationID, personID); err != nil {
		return domain.Group{}, err
	}

	group, err := s.repo.GetGroup(ctx, organisationID, groupID)
	if err != nil {
		return domain.Group{}, err
	}
	for _, memberID := range group.MemberIDs {
		if memberID == personID {
			return group, nil
		}
	}
	group.MemberIDs = append(group.MemberIDs, personID)
	return s.repo.UpdateGroup(ctx, group)
}

func (s *Service) RemoveGroupMember(ctx context.Context, auth ports.AuthContext, groupID, personID string) (domain.Group, error) {
	if err := requireAnyRole(auth, domain.RoleOrgAdmin); err != nil {
		return domain.Group{}, err
	}
	organisationID, err := requiredOrganisationID(auth)
	if err != nil {
		return domain.Group{}, err
	}

	group, err := s.repo.GetGroup(ctx, organisationID, groupID)
	if err != nil {
		return domain.Group{}, err
	}
	members := make([]string, 0, len(group.MemberIDs))
	for _, memberID := range group.MemberIDs {
		if memberID != personID {
			members = append(members, memberID)
		}
	}
	group.MemberIDs = members
	return s.repo.UpdateGroup(ctx, group)
}

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
	if err := validateAllocation(input); err != nil {
		return domain.Allocation{}, err
	}
	project, err := s.repo.GetProject(ctx, organisationID, input.ProjectID)
	if err != nil {
		return domain.Allocation{}, err
	}
	if err := validateAllocationWithinProjectRange(input, project); err != nil {
		return domain.Allocation{}, err
	}

	targetPersonIDs, err := s.resolveAllocationTargetPersons(ctx, organisationID, input.TargetType, input.TargetID)
	if err != nil {
		return domain.Allocation{}, err
	}
	if err := s.validateAllocationLimit(ctx, organisationID, input, targetPersonIDs, ""); err != nil {
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
	if err := validateAllocation(input); err != nil {
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
	if err := validateAllocationWithinProjectRange(input, project); err != nil {
		return domain.Allocation{}, err
	}

	targetPersonIDs, err := s.resolveAllocationTargetPersons(ctx, organisationID, input.TargetType, input.TargetID)
	if err != nil {
		return domain.Allocation{}, err
	}
	if err := s.validateAllocationLimit(ctx, organisationID, input, targetPersonIDs, allocationID); err != nil {
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

	if err := s.repo.DeleteAllocation(ctx, organisationID, allocationID); err != nil {
		return err
	}

	s.telemetry.Record("allocation.deleted", map[string]string{"allocation_id": allocationID})
	return nil
}

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
		return domain.PersonUnavailability{}, domain.ErrValidation
	}
	personDailyHours := organisation.HoursPerDay * employmentPct / 100
	if err := validateDateHours(input.Date, input.Hours, personDailyHours); err != nil {
		return domain.PersonUnavailability{}, err
	}

	existingEntries, err := s.repo.ListPersonUnavailability(ctx, organisationID)
	if err != nil {
		return domain.PersonUnavailability{}, err
	}
	existingTotal := 0.0
	for _, entry := range existingEntries {
		if entry.PersonID == input.PersonID && entry.Date == input.Date {
			existingTotal += entry.Hours
		}
	}
	if existingTotal+input.Hours > personDailyHours+1e-9 {
		return domain.PersonUnavailability{}, domain.ErrValidation
	}

	entry := domain.PersonUnavailability{
		OrganisationID: organisationID,
		PersonID:       input.PersonID,
		Date:           input.Date,
		Hours:          input.Hours,
	}

	created, err := s.repo.CreatePersonUnavailability(ctx, entry)
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
	if _, err := domain.ValidateDate(request.FromDate); err != nil {
		return nil, domain.ErrValidation
	}
	if _, err := domain.ValidateDate(request.ToDate); err != nil {
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
	}

	for _, id := range request.IDs {
		if !lookup[id] {
			return domain.ErrNotFound
		}
	}

	return nil
}

func (s *Service) ensureMembersBelongToOrg(ctx context.Context, organisationID string, memberIDs []string) error {
	for _, memberID := range memberIDs {
		if _, err := s.repo.GetPerson(ctx, organisationID, memberID); err != nil {
			return err
		}
	}
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
		if _, err := s.repo.GetPerson(ctx, organisationID, personID); err != nil {
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

			existingStart, existingEnd, err := parseDateRange(allocation.StartDate, allocation.EndDate)
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

func validateOrganisation(organisation domain.Organisation) error {
	if err := domain.ValidateName(organisation.Name); err != nil {
		return domain.ErrValidation
	}
	if organisation.HoursPerDay <= 0 || organisation.HoursPerWeek <= 0 || organisation.HoursPerYear <= 0 {
		return domain.ErrValidation
	}
	return nil
}

func validatePerson(person domain.Person) error {
	if err := domain.ValidateName(person.Name); err != nil {
		return domain.ErrValidation
	}
	if err := domain.ValidatePercent(person.EmploymentPct); err != nil {
		return domain.ErrValidation
	}
	if strings.TrimSpace(person.EmploymentEffectiveFromMonth) != "" {
		if _, err := domain.ValidateMonth(strings.TrimSpace(person.EmploymentEffectiveFromMonth)); err != nil {
			return domain.ErrValidation
		}
	}
	for _, change := range person.EmploymentChanges {
		if _, err := domain.ValidateMonth(change.EffectiveMonth); err != nil {
			return domain.ErrValidation
		}
		if err := domain.ValidatePercent(change.EmploymentPct); err != nil {
			return domain.ErrValidation
		}
	}
	return nil
}

func upsertEmploymentChange(changes []domain.EmploymentChange, month string, employmentPct float64) []domain.EmploymentChange {
	normalized := make([]domain.EmploymentChange, 0, len(changes))
	updated := false
	for _, change := range changes {
		if change.EffectiveMonth == month {
			normalized = append(normalized, domain.EmploymentChange{
				EffectiveMonth: month,
				EmploymentPct:  employmentPct,
			})
			updated = true
			continue
		}
		normalized = append(normalized, change)
	}
	if !updated {
		normalized = append(normalized, domain.EmploymentChange{
			EffectiveMonth: month,
			EmploymentPct:  employmentPct,
		})
	}

	sort.Slice(normalized, func(i, j int) bool {
		if normalized[i].EffectiveMonth == normalized[j].EffectiveMonth {
			return i < j
		}
		return normalized[i].EffectiveMonth < normalized[j].EffectiveMonth
	})

	return normalized
}

func validateProject(project domain.Project) error {
	if err := domain.ValidateName(project.Name); err != nil {
		return domain.ErrValidation
	}
	if project.EstimatedEffortHours <= 0 {
		return domain.ErrValidation
	}
	if strings.TrimSpace(project.StartDate) == "" || strings.TrimSpace(project.EndDate) == "" {
		return domain.ErrValidation
	}
	if _, _, err := parseDateRange(project.StartDate, project.EndDate); err != nil {
		return domain.ErrValidation
	}
	return nil
}

func validateGroup(group domain.Group) error {
	if err := domain.ValidateName(group.Name); err != nil {
		return domain.ErrValidation
	}
	return nil
}

func validateAllocation(allocation domain.Allocation) error {
	if err := domain.ValidateAllocationTargetType(allocation.TargetType); err != nil {
		return domain.ErrValidation
	}
	if strings.TrimSpace(allocation.TargetID) == "" {
		return domain.ErrValidation
	}
	if strings.TrimSpace(allocation.ProjectID) == "" {
		return domain.ErrValidation
	}
	if strings.TrimSpace(allocation.StartDate) == "" || strings.TrimSpace(allocation.EndDate) == "" {
		return domain.ErrValidation
	}
	if _, _, err := parseDateRange(allocation.StartDate, allocation.EndDate); err != nil {
		return domain.ErrValidation
	}
	if math.IsNaN(allocation.Percent) || math.IsInf(allocation.Percent, 0) || allocation.Percent < 0 {
		return domain.ErrValidation
	}
	return nil
}

func validateDateHours(date string, hours float64, maxHours float64) error {
	if _, err := domain.ValidateDate(date); err != nil {
		return domain.ErrValidation
	}
	if hours < 0 || hours > maxHours {
		return domain.ErrValidation
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

func parseDateRange(startDate, endDate string) (time.Time, time.Time, error) {
	startDate = strings.TrimSpace(startDate)
	endDate = strings.TrimSpace(endDate)

	if startDate == "" {
		startDate = "1970-01-01"
	}
	if endDate == "" {
		endDate = "9999-12-31"
	}

	start, err := domain.ValidateDate(startDate)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	end, err := domain.ValidateDate(endDate)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}

	startParsed, err := time.Parse(domain.DateLayout, start)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	endParsed, err := time.Parse(domain.DateLayout, end)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	if endParsed.Before(startParsed) {
		return time.Time{}, time.Time{}, domain.ErrValidation
	}

	return startParsed, endParsed, nil
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

func requiredOrganisationID(auth ports.AuthContext) (string, error) {
	organisationID := strings.TrimSpace(auth.OrganisationID)
	if organisationID == "" {
		return "", domain.ErrForbidden
	}
	return organisationID, nil
}

func requireAnyRole(auth ports.AuthContext, roles ...string) error {
	if len(roles) == 0 {
		return domain.ErrForbidden
	}
	for _, role := range roles {
		if auth.HasRole(role) {
			return nil
		}
	}
	return domain.ErrForbidden
}

func enforceTenant(auth ports.AuthContext, targetOrganisationID string) error {
	organisationID := strings.TrimSpace(auth.OrganisationID)
	if organisationID == "" {
		return nil
	}
	if organisationID != strings.TrimSpace(targetOrganisationID) {
		return domain.ErrForbidden
	}
	return nil
}

func IsValidationError(err error) bool {
	return errors.Is(err, domain.ErrValidation)
}

func IsForbiddenError(err error) bool {
	return errors.Is(err, domain.ErrForbidden)
}

func IsNotFoundError(err error) bool {
	return errors.Is(err, domain.ErrNotFound)
}
