package service

import (
	"context"
	"errors"
	"math"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"plato/backend/internal/adapters/impexp"
	"plato/backend/internal/adapters/persistence"
	"plato/backend/internal/adapters/telemetry"
	"plato/backend/internal/domain"
	"plato/backend/internal/ports"
)

func TestServiceOrganisationCRUDAndTenantEnforcement(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	globalAdmin := ports.AuthContext{UserID: "admin", Roles: []string{domain.RoleOrgAdmin}}

	created, err := svc.CreateOrganisation(ctx, globalAdmin, domain.Organisation{
		Name:         "Org One",
		HoursPerDay:  8,
		HoursPerWeek: 40,
		HoursPerYear: 2080,
	})
	if err != nil {
		t.Fatalf("create organisation: %v", err)
	}

	list, err := svc.ListOrganisations(ctx, globalAdmin)
	if err != nil {
		t.Fatalf("list organisations: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected one organisation, got %d", len(list))
	}

	tenantAdmin := ports.AuthContext{UserID: "tenant-admin", OrganisationID: created.ID, Roles: []string{domain.RoleOrgAdmin}}
	read, err := svc.GetOrganisation(ctx, tenantAdmin, created.ID)
	if err != nil {
		t.Fatalf("get organisation: %v", err)
	}
	if read.ID != created.ID {
		t.Fatalf("unexpected organisation read: %+v", read)
	}

	updated, err := svc.UpdateOrganisation(ctx, tenantAdmin, created.ID, domain.Organisation{
		Name:         "Org One Updated",
		HoursPerDay:  7.5,
		HoursPerWeek: 37.5,
		HoursPerYear: 1950,
	})
	if err != nil {
		t.Fatalf("update organisation: %v", err)
	}
	if updated.Name != "Org One Updated" {
		t.Fatalf("unexpected update result: %+v", updated)
	}

	wrongTenant := ports.AuthContext{UserID: "other", OrganisationID: "org_other", Roles: []string{domain.RoleOrgAdmin}}
	_, err = svc.GetOrganisation(ctx, wrongTenant, created.ID)
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected forbidden cross tenant, got %v", err)
	}

	filteredList, err := svc.ListOrganisations(ctx, ports.AuthContext{UserID: "u1", OrganisationID: created.ID, Roles: []string{domain.RoleOrgUser}})
	if err != nil {
		t.Fatalf("tenant list: %v", err)
	}
	if len(filteredList) != 1 || filteredList[0].ID != created.ID {
		t.Fatalf("expected filtered list to contain only tenant org, got %+v", filteredList)
	}
	emptyFilteredList, err := svc.ListOrganisations(ctx, ports.AuthContext{UserID: "u2", OrganisationID: "missing-org", Roles: []string{domain.RoleOrgUser}})
	if err != nil {
		t.Fatalf("tenant list for missing org: %v", err)
	}
	if len(emptyFilteredList) != 0 {
		t.Fatalf("expected empty filtered list for missing tenant org, got %+v", emptyFilteredList)
	}

	err = svc.DeleteOrganisation(ctx, tenantAdmin, created.ID)
	if err != nil {
		t.Fatalf("delete organisation: %v", err)
	}

	_, err = svc.GetOrganisation(ctx, globalAdmin, created.ID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected organisation not found after delete, got %v", err)
	}
}

func TestNewServiceRequiresDependencies(t *testing.T) {
	repo, err := persistence.NewFileRepository(filepath.Join(t.TempDir(), "deps-data.json"))
	if err != nil {
		t.Fatalf("create repository: %v", err)
	}
	telemetryAdapter := telemetry.NewNoopTelemetry()
	importAdapter := impexp.NewNoopImportExport()

	_, err = New(nil, telemetryAdapter, importAdapter)
	if err == nil {
		t.Fatal("expected nil repository to fail")
	}
	_, err = New(repo, nil, importAdapter)
	if err == nil {
		t.Fatal("expected nil telemetry to fail")
	}
	_, err = New(repo, telemetryAdapter, nil)
	if err == nil {
		t.Fatal("expected nil import/export to fail")
	}
	_, err = New(repo, telemetryAdapter, importAdapter)
	if err != nil {
		t.Fatalf("expected valid dependencies to succeed, got %v", err)
	}
}

func TestServiceResourceFlowAndReport(t *testing.T) {
	state := setupServiceResourceFlowState(t)
	validateServiceResourceFlowPeopleProjectsGroups(t, state)
	validateServiceResourceFlowAllocationsAndCalendar(t, state)
	validateServiceResourceFlowReportsAndCleanup(t, state)
}

type serviceResourceFlowState struct {
	svc                       *Service
	admin                     ports.AuthContext
	user                      ports.AuthContext
	person1                   domain.Person
	person2                   domain.Person
	project1                  domain.Project
	project2                  domain.Project
	group                     domain.Group
	allocation1               domain.Allocation
	allocation2               domain.Allocation
	groupAllocation           domain.Allocation
	holiday                   domain.OrgHoliday
	groupUnavailability       domain.GroupUnavailability
	personUnavailability      domain.PersonUnavailability
	personUnavailabilityOther domain.PersonUnavailability
}

func setupServiceResourceFlowState(t *testing.T) *serviceResourceFlowState {
	t.Helper()
	svc := newTestService(t)
	ctx := context.Background()
	globalAdmin := ports.AuthContext{UserID: "admin", Roles: []string{domain.RoleOrgAdmin}}
	organisation := createOrganisationForService(t, svc, ctx, globalAdmin, "Org Flow")

	return &serviceResourceFlowState{
		svc:   svc,
		admin: ports.AuthContext{UserID: "admin1", OrganisationID: organisation.ID, Roles: []string{domain.RoleOrgAdmin}},
		user:  ports.AuthContext{UserID: "user1", OrganisationID: organisation.ID, Roles: []string{domain.RoleOrgUser}},
	}
}

func validateServiceResourceFlowPeopleProjectsGroups(t *testing.T, state *serviceResourceFlowState) {
	t.Helper()
	ctx := context.Background()

	createServiceResourceFlowPeople(t, state, ctx)
	createServiceResourceFlowProjects(t, state, ctx)
	createServiceResourceFlowGroupAndLists(t, state, ctx)
}

func createServiceResourceFlowPeople(t *testing.T, state *serviceResourceFlowState, ctx context.Context) {
	t.Helper()

	var err error
	state.person1, err = state.svc.CreatePerson(ctx, state.admin, domain.Person{Name: "Alice", EmploymentPct: 100})
	if err != nil {
		t.Fatalf("create person1: %v", err)
	}
	state.person2, err = state.svc.CreatePerson(ctx, state.admin, domain.Person{Name: "Bob", EmploymentPct: 60})
	if err != nil {
		t.Fatalf("create person2: %v", err)
	}

	person2Read, err := state.svc.GetPerson(ctx, state.admin, state.person2.ID)
	if err != nil {
		t.Fatalf("get person2: %v", err)
	}
	if person2Read.Name != "Bob" {
		t.Fatalf("unexpected person read: %+v", person2Read)
	}

	state.person2, err = state.svc.UpdatePerson(ctx, state.admin, state.person2.ID, domain.Person{Name: "Bob Updated", EmploymentPct: 70})
	if err != nil {
		t.Fatalf("update person2: %v", err)
	}
	if state.person2.Name != "Bob Updated" {
		t.Fatalf("unexpected person update: %+v", state.person2)
	}
}

func createServiceResourceFlowProjects(t *testing.T, state *serviceResourceFlowState, ctx context.Context) {
	t.Helper()

	var err error
	state.project1, err = state.svc.CreateProject(ctx, state.admin, testProjectInput("Project One"))
	if err != nil {
		t.Fatalf("create project1: %v", err)
	}
	state.project2, err = state.svc.CreateProject(ctx, state.admin, testProjectInput("Project Two"))
	if err != nil {
		t.Fatalf("create project2: %v", err)
	}

	project1Read, err := state.svc.GetProject(ctx, state.admin, state.project1.ID)
	if err != nil {
		t.Fatalf("get project1: %v", err)
	}
	if project1Read.Name != "Project One" {
		t.Fatalf("unexpected project read: %+v", project1Read)
	}

	state.project2, err = state.svc.UpdateProject(ctx, state.admin, state.project2.ID, testProjectInput("Project Two Updated"))
	if err != nil {
		t.Fatalf("update project2: %v", err)
	}
	if state.project2.Name != "Project Two Updated" {
		t.Fatalf("unexpected project2 update: %+v", state.project2)
	}

	projectList, err := state.svc.ListProjects(ctx, state.user)
	if err != nil {
		t.Fatalf("list projects as user: %v", err)
	}
	if len(projectList) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(projectList))
	}
}

func createServiceResourceFlowGroupAndLists(t *testing.T, state *serviceResourceFlowState, ctx context.Context) {
	t.Helper()

	var err error
	state.group, err = state.svc.CreateGroup(ctx, state.admin, domain.Group{Name: "Team", MemberIDs: []string{state.person1.ID}})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}

	groupRead, err := state.svc.GetGroup(ctx, state.admin, state.group.ID)
	if err != nil {
		t.Fatalf("get group: %v", err)
	}
	if len(groupRead.MemberIDs) != 1 || groupRead.MemberIDs[0] != state.person1.ID {
		t.Fatalf("unexpected group read: %+v", groupRead)
	}

	state.group, err = state.svc.UpdateGroup(ctx, state.admin, state.group.ID, domain.Group{Name: "Team Updated", MemberIDs: []string{state.person1.ID}})
	if err != nil {
		t.Fatalf("update group: %v", err)
	}
	if state.group.Name != "Team Updated" {
		t.Fatalf("unexpected group update: %+v", state.group)
	}

	groupList, err := state.svc.ListGroups(ctx, state.user)
	if err != nil {
		t.Fatalf("list groups as user: %v", err)
	}
	if len(groupList) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groupList))
	}

	personList, err := state.svc.ListPersons(ctx, state.user)
	if err != nil {
		t.Fatalf("list persons as user: %v", err)
	}
	if len(personList) != 2 {
		t.Fatalf("expected 2 persons, got %d", len(personList))
	}
}

func validateServiceResourceFlowAllocationsAndCalendar(t *testing.T, state *serviceResourceFlowState) {
	t.Helper()
	ctx := context.Background()

	updateServiceResourceFlowGroupMembership(t, state, ctx)
	createServiceResourceFlowAllocations(t, state, ctx)
	createServiceResourceFlowCalendarEntries(t, state, ctx)
	verifyServiceResourceFlowCalendarLists(t, state, ctx)
}

func updateServiceResourceFlowGroupMembership(t *testing.T, state *serviceResourceFlowState, ctx context.Context) {
	t.Helper()

	var err error
	state.group, err = state.svc.AddGroupMember(ctx, state.admin, state.group.ID, state.person2.ID)
	if err != nil {
		t.Fatalf("add group member: %v", err)
	}
	if len(state.group.MemberIDs) != 2 {
		t.Fatalf("expected 2 members after add, got %v", state.group.MemberIDs)
	}

	state.group, err = state.svc.AddGroupMember(ctx, state.admin, state.group.ID, state.person2.ID)
	if err != nil {
		t.Fatalf("add duplicate group member: %v", err)
	}
	if len(state.group.MemberIDs) != 2 {
		t.Fatalf("expected duplicate add to keep 2 members, got %v", state.group.MemberIDs)
	}

	state.group, err = state.svc.RemoveGroupMember(ctx, state.admin, state.group.ID, state.person2.ID)
	if err != nil {
		t.Fatalf("remove group member: %v", err)
	}
	if len(state.group.MemberIDs) != 1 {
		t.Fatalf("expected 1 member after remove, got %v", state.group.MemberIDs)
	}
}

func createServiceResourceFlowAllocations(t *testing.T, state *serviceResourceFlowState, ctx context.Context) {
	t.Helper()

	var err error
	state.groupAllocation, err = state.svc.CreateAllocation(ctx, state.admin, testGroupAllocationInput(state.group.ID, state.project2.ID, 20))
	if err != nil {
		t.Fatalf("create group allocation: %v", err)
	}
	if state.groupAllocation.TargetType != domain.AllocationTargetGroup {
		t.Fatalf("expected group allocation target type, got %s", state.groupAllocation.TargetType)
	}

	state.allocation1, err = state.svc.CreateAllocation(ctx, state.admin, testPersonAllocationInput(state.person1.ID, state.project1.ID, 60))
	if err != nil {
		t.Fatalf("create allocation1: %v", err)
	}
	allocationRead, err := state.svc.GetAllocation(ctx, state.admin, state.allocation1.ID)
	if err != nil {
		t.Fatalf("get allocation1: %v", err)
	}
	if allocationRead.Percent != 60 {
		t.Fatalf("unexpected allocation read: %+v", allocationRead)
	}

	state.allocation2, err = state.svc.CreateAllocation(ctx, state.admin, testPersonAllocationInput(state.person1.ID, state.project2.ID, 50))
	if err != nil {
		t.Fatalf("create allocation2: %v", err)
	}

	state.allocation1, err = state.svc.UpdateAllocation(ctx, state.admin, state.allocation1.ID, testPersonAllocationInput(state.person1.ID, state.project1.ID, 40))
	if err != nil {
		t.Fatalf("update allocation: %v", err)
	}
	if state.allocation1.Percent != 40 {
		t.Fatalf("expected updated allocation percent, got %v", state.allocation1.Percent)
	}

	allocationList, err := state.svc.ListAllocations(ctx, state.user)
	if err != nil {
		t.Fatalf("list allocations as user: %v", err)
	}
	if len(allocationList) != 3 {
		t.Fatalf("expected 3 allocations, got %d", len(allocationList))
	}
}

func createServiceResourceFlowCalendarEntries(t *testing.T, state *serviceResourceFlowState, ctx context.Context) {
	t.Helper()

	var err error
	state.holiday, err = state.svc.CreateOrgHoliday(ctx, state.admin, domain.OrgHoliday{Date: "2026-01-01", Hours: 8})
	if err != nil {
		t.Fatalf("create org holiday: %v", err)
	}
	state.groupUnavailability, err = state.svc.CreateGroupUnavailability(ctx, state.admin, domain.GroupUnavailability{GroupID: state.group.ID, Date: "2026-01-03", Hours: 4})
	if err != nil {
		t.Fatalf("create group unavailability: %v", err)
	}
	state.personUnavailability, err = state.svc.CreatePersonUnavailability(ctx, state.admin, domain.PersonUnavailability{PersonID: state.person1.ID, Date: "2026-01-04", Hours: 2})
	if err != nil {
		t.Fatalf("create person unavailability: %v", err)
	}
	state.personUnavailabilityOther, err = state.svc.CreatePersonUnavailability(ctx, state.admin, domain.PersonUnavailability{PersonID: state.person1.ID, Date: "2026-01-06", Hours: 1})
	if err != nil {
		t.Fatalf("create second person unavailability: %v", err)
	}
}

func verifyServiceResourceFlowCalendarLists(t *testing.T, state *serviceResourceFlowState, ctx context.Context) {
	t.Helper()

	holidayList, err := state.svc.ListOrgHolidays(ctx, state.user)
	if err != nil {
		t.Fatalf("list holidays as user: %v", err)
	}
	if len(holidayList) != 1 {
		t.Fatalf("expected 1 holiday, got %d", len(holidayList))
	}

	groupUnavailabilityList, err := state.svc.ListGroupUnavailability(ctx, state.user)
	if err != nil {
		t.Fatalf("list group unavailability as user: %v", err)
	}
	if len(groupUnavailabilityList) != 1 {
		t.Fatalf("expected 1 group unavailability, got %d", len(groupUnavailabilityList))
	}

	personUnavailabilityList, err := state.svc.ListPersonUnavailability(ctx, state.user)
	if err != nil {
		t.Fatalf("list person unavailability as user: %v", err)
	}
	if len(personUnavailabilityList) != 2 {
		t.Fatalf("expected 2 person unavailability entries, got %d", len(personUnavailabilityList))
	}

	person1UnavailabilityList, err := state.svc.ListPersonUnavailabilityByPerson(ctx, state.user, state.person1.ID)
	if err != nil {
		t.Fatalf("list person unavailability for person as user: %v", err)
	}
	if len(person1UnavailabilityList) != 2 {
		t.Fatalf("expected 2 person unavailability entries for person1, got %d", len(person1UnavailabilityList))
	}
}

func validateServiceResourceFlowReportsAndCleanup(t *testing.T, state *serviceResourceFlowState) {
	t.Helper()
	ctx := context.Background()

	report, err := state.svc.ReportAvailabilityAndLoad(ctx, state.user, domain.ReportRequest{
		Scope:       domain.ScopePerson,
		IDs:         []string{state.person1.ID},
		FromDate:    "2026-01-01",
		ToDate:      "2026-01-02",
		Granularity: domain.GranularityDay,
	})
	if err != nil {
		t.Fatalf("report: %v", err)
	}
	if len(report) != 2 {
		t.Fatalf("expected 2 buckets, got %d", len(report))
	}

	reportByGroup, err := state.svc.ReportAvailabilityAndLoad(ctx, state.user, domain.ReportRequest{
		Scope:       domain.ScopeGroup,
		IDs:         []string{state.group.ID},
		FromDate:    "2026-01-01",
		ToDate:      "2026-01-01",
		Granularity: domain.GranularityDay,
	})
	if err != nil {
		t.Fatalf("group report: %v", err)
	}
	if len(reportByGroup) != 1 {
		t.Fatalf("expected 1 group report bucket, got %d", len(reportByGroup))
	}

	_, err = state.svc.ReportAvailabilityAndLoad(ctx, state.user, domain.ReportRequest{Scope: domain.ScopeProject, IDs: []string{"missing"}, FromDate: "2026-01-01", ToDate: "2026-01-01", Granularity: domain.GranularityDay})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected not found for missing report scope id, got %v", err)
	}

	err = state.svc.DeleteOrgHoliday(ctx, state.admin, state.holiday.ID)
	if err != nil {
		t.Fatalf("delete holiday: %v", err)
	}
	err = state.svc.DeleteGroupUnavailability(ctx, state.admin, state.groupUnavailability.ID)
	if err != nil {
		t.Fatalf("delete group unavailability: %v", err)
	}
	err = state.svc.DeletePersonUnavailabilityByPerson(ctx, state.admin, state.person2.ID, state.personUnavailability.ID)
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected person-scoped delete with mismatched person to fail, got %v", err)
	}
	err = state.svc.DeletePersonUnavailabilityByPerson(ctx, state.admin, state.person1.ID, state.personUnavailability.ID)
	if err != nil {
		t.Fatalf("delete person unavailability: %v", err)
	}
	err = state.svc.DeletePersonUnavailability(ctx, state.admin, state.personUnavailabilityOther.ID)
	if err != nil {
		t.Fatalf("delete second person unavailability: %v", err)
	}
	err = state.svc.DeleteAllocation(ctx, state.admin, state.allocation1.ID)
	if err != nil {
		t.Fatalf("delete allocation: %v", err)
	}
	err = state.svc.DeleteAllocation(ctx, state.admin, state.allocation2.ID)
	if err != nil {
		t.Fatalf("delete allocation2: %v", err)
	}
	err = state.svc.DeleteAllocation(ctx, state.admin, state.groupAllocation.ID)
	if err != nil {
		t.Fatalf("delete group allocation: %v", err)
	}
	err = state.svc.DeleteGroup(ctx, state.admin, state.group.ID)
	if err != nil {
		t.Fatalf("delete group: %v", err)
	}
	err = state.svc.DeleteProject(ctx, state.admin, state.project2.ID)
	if err != nil {
		t.Fatalf("delete project2: %v", err)
	}
	err = state.svc.DeleteProject(ctx, state.admin, state.project1.ID)
	if err != nil {
		t.Fatalf("delete project1: %v", err)
	}
	err = state.svc.DeletePerson(ctx, state.admin, state.person2.ID)
	if err != nil {
		t.Fatalf("delete person2: %v", err)
	}
	err = state.svc.DeletePerson(ctx, state.admin, state.person1.ID)
	if err != nil {
		t.Fatalf("delete person1: %v", err)
	}
}

func TestServiceValidationAndHelpers(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()
	globalAdmin := ports.AuthContext{UserID: "admin", Roles: []string{domain.RoleOrgAdmin}}
	assertServiceValidationOrganisationGuards(t, svc, ctx, globalAdmin)

	organisation := createOrganisationForService(t, svc, ctx, globalAdmin, "Org Validate")
	admin := ports.AuthContext{UserID: "admin1", OrganisationID: organisation.ID, Roles: []string{domain.RoleOrgAdmin}}
	user := ports.AuthContext{UserID: "user1", OrganisationID: organisation.ID, Roles: []string{domain.RoleOrgUser}}

	assertServiceValidationPersonGuards(t, svc, ctx, admin, user)
	assertServiceValidationProjectAndGroupGuards(t, svc, ctx, admin)
	assertServiceValidationTenantListGuards(t, svc, ctx)
	assertServiceValidationErrorHelpers(t)
}

func assertServiceValidationOrganisationGuards(t *testing.T, svc *Service, ctx context.Context, globalAdmin ports.AuthContext) {
	t.Helper()

	if _, err := svc.CreateOrganisation(ctx, ports.AuthContext{Roles: []string{domain.RoleOrgUser}}, domain.Organisation{Name: "Bad", HoursPerDay: 8, HoursPerWeek: 40, HoursPerYear: 2080}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected forbidden create organisation for org_user, got %v", err)
	}
	if _, err := svc.CreateOrganisation(ctx, globalAdmin, domain.Organisation{Name: "", HoursPerDay: 8, HoursPerWeek: 40, HoursPerYear: 2080}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected validation for create organisation, got %v", err)
	}
	if _, err := svc.CreateOrganisation(ctx, globalAdmin, domain.Organisation{Name: "Bad Zero", HoursPerDay: 0, HoursPerWeek: 40, HoursPerYear: 2080}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected validation for non-positive hours, got %v", err)
	}
	if _, err := svc.ListOrganisations(ctx, ports.AuthContext{}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected forbidden list organisations without role, got %v", err)
	}
}

func assertServiceValidationPersonGuards(t *testing.T, svc *Service, ctx context.Context, admin, user ports.AuthContext) {
	t.Helper()

	if _, err := svc.CreatePerson(ctx, user, domain.Person{Name: "X", EmploymentPct: 50}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected forbidden create person for user role, got %v", err)
	}

	if _, err := svc.CreatePerson(ctx, admin, domain.Person{Name: "", EmploymentPct: 50}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected validation for empty name, got %v", err)
	}
	if _, err := svc.CreatePerson(ctx, admin, domain.Person{Name: "Bad", EmploymentPct: 150}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected validation for employment percent, got %v", err)
	}
}

func assertServiceValidationProjectAndGroupGuards(t *testing.T, svc *Service, ctx context.Context, admin ports.AuthContext) {
	t.Helper()

	if _, err := svc.CreateProject(ctx, ports.AuthContext{Roles: []string{domain.RoleOrgAdmin}}, testProjectInput("No Org")); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected forbidden for missing tenant org, got %v", err)
	}
	invalidProject := testProjectInput("")
	if _, err := svc.CreateProject(ctx, admin, invalidProject); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected validation for empty project name, got %v", err)
	}

	if _, err := svc.CreateOrgHoliday(ctx, admin, domain.OrgHoliday{Date: "bad", Hours: 20}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected date validation for org holiday, got %v", err)
	}

	if _, err := svc.CreateGroup(ctx, admin, domain.Group{Name: "Team", MemberIDs: []string{"missing"}}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected member not found, got %v", err)
	}
	if _, err := svc.CreateGroup(ctx, admin, domain.Group{Name: "", MemberIDs: nil}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected validation for empty group name, got %v", err)
	}
}

func assertServiceValidationTenantListGuards(t *testing.T, svc *Service, ctx context.Context) {
	t.Helper()

	if _, err := svc.ListPersons(ctx, ports.AuthContext{Roles: []string{domain.RoleOrgAdmin}}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected forbidden when tenant missing for list persons, got %v", err)
	}
	if _, err := svc.ListProjects(ctx, ports.AuthContext{Roles: []string{domain.RoleOrgUser}}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected forbidden when tenant missing for list projects, got %v", err)
	}
	if _, err := svc.ListGroups(ctx, ports.AuthContext{Roles: []string{domain.RoleOrgUser}}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected forbidden when tenant missing for list groups, got %v", err)
	}
	if _, err := svc.ListAllocations(ctx, ports.AuthContext{Roles: []string{domain.RoleOrgUser}}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected forbidden when tenant missing for list allocations, got %v", err)
	}
	if _, err := svc.ListOrgHolidays(ctx, ports.AuthContext{Roles: []string{domain.RoleOrgUser}}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected forbidden when tenant missing for list holidays, got %v", err)
	}
	if _, err := svc.ListGroupUnavailability(ctx, ports.AuthContext{Roles: []string{domain.RoleOrgUser}}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected forbidden when tenant missing for list group unavailability, got %v", err)
	}
	if _, err := svc.ListPersonUnavailability(ctx, ports.AuthContext{Roles: []string{domain.RoleOrgUser}}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected forbidden when tenant missing for list person unavailability, got %v", err)
	}
	if _, err := svc.ListPersonUnavailabilityByPerson(ctx, ports.AuthContext{Roles: []string{domain.RoleOrgUser}}, "person_1"); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected forbidden when tenant missing for list person unavailability by person, got %v", err)
	}
}

func assertServiceValidationErrorHelpers(t *testing.T) {
	t.Helper()

	if !IsValidationError(domain.ErrValidation) {
		t.Fatal("expected IsValidationError to match")
	}
	if !IsForbiddenError(domain.ErrForbidden) {
		t.Fatal("expected IsForbiddenError to match")
	}
	if !IsNotFoundError(domain.ErrNotFound) {
		t.Fatal("expected IsNotFoundError to match")
	}
}

func TestServiceRemainingErrorBranches(t *testing.T) {
	state := setupServiceRemainingErrorBranchesState(t)
	validateServiceRemainingErrorBranchesForbiddenReads(t, state)
	validateServiceRemainingErrorBranchesUpdates(t, state)
	validateServiceRemainingErrorBranchesDeleteAndCreate(t, state)
	validateServiceRemainingErrorBranchesReports(t, state)
}

type serviceRemainingErrorBranchesState struct {
	svc          *Service
	organisation domain.Organisation
	admin        ports.AuthContext
	user         ports.AuthContext
	person       domain.Person
	project      domain.Project
	group        domain.Group
	allocation   domain.Allocation
}

func setupServiceRemainingErrorBranchesState(t *testing.T) *serviceRemainingErrorBranchesState {
	t.Helper()
	svc := newTestService(t)
	ctx := context.Background()
	globalAdmin := ports.AuthContext{UserID: "admin", Roles: []string{domain.RoleOrgAdmin}}
	organisation := createOrganisationForService(t, svc, ctx, globalAdmin, "Org Errors")
	admin := ports.AuthContext{UserID: "admin1", OrganisationID: organisation.ID, Roles: []string{domain.RoleOrgAdmin}}
	user := ports.AuthContext{UserID: "user1", OrganisationID: organisation.ID, Roles: []string{domain.RoleOrgUser}}

	person, err := svc.CreatePerson(ctx, admin, domain.Person{Name: "Error Person", EmploymentPct: 80})
	if err != nil {
		t.Fatalf("setup person: %v", err)
	}
	project, err := svc.CreateProject(ctx, admin, testProjectInput("Error Project"))
	if err != nil {
		t.Fatalf("setup project: %v", err)
	}
	group, err := svc.CreateGroup(ctx, admin, domain.Group{Name: "Error Group", MemberIDs: []string{person.ID}})
	if err != nil {
		t.Fatalf("setup group: %v", err)
	}
	allocation, err := svc.CreateAllocation(ctx, admin, testPersonAllocationInput(person.ID, project.ID, 20))
	if err != nil {
		t.Fatalf("setup allocation: %v", err)
	}

	return &serviceRemainingErrorBranchesState{
		svc:          svc,
		organisation: organisation,
		admin:        admin,
		user:         user,
		person:       person,
		project:      project,
		group:        group,
		allocation:   allocation,
	}
}

func validateServiceRemainingErrorBranchesForbiddenReads(t *testing.T, state *serviceRemainingErrorBranchesState) {
	t.Helper()
	ctx := context.Background()

	if _, err := state.svc.GetPerson(ctx, ports.AuthContext{OrganisationID: state.organisation.ID, Roles: []string{}}, state.person.ID); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected forbidden get person, got %v", err)
	}
	if _, err := state.svc.GetProject(ctx, ports.AuthContext{OrganisationID: state.organisation.ID, Roles: []string{}}, state.project.ID); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected forbidden get project, got %v", err)
	}
	if _, err := state.svc.GetGroup(ctx, ports.AuthContext{OrganisationID: state.organisation.ID, Roles: []string{}}, state.group.ID); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected forbidden get group, got %v", err)
	}
	if _, err := state.svc.GetAllocation(ctx, ports.AuthContext{OrganisationID: state.organisation.ID, Roles: []string{}}, state.allocation.ID); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected forbidden get allocation, got %v", err)
	}

	if _, err := state.svc.ListPersons(ctx, ports.AuthContext{OrganisationID: state.organisation.ID, Roles: []string{}}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected forbidden list persons without role, got %v", err)
	}
	if _, err := state.svc.ListProjects(ctx, ports.AuthContext{OrganisationID: state.organisation.ID, Roles: []string{}}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected forbidden list projects without role, got %v", err)
	}
	if _, err := state.svc.ListGroups(ctx, ports.AuthContext{OrganisationID: state.organisation.ID, Roles: []string{}}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected forbidden list groups without role, got %v", err)
	}
	if _, err := state.svc.ListAllocations(ctx, ports.AuthContext{OrganisationID: state.organisation.ID, Roles: []string{}}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected forbidden list allocations without role, got %v", err)
	}
	if _, err := state.svc.ListOrgHolidays(ctx, ports.AuthContext{OrganisationID: state.organisation.ID, Roles: []string{}}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected forbidden list holidays without role, got %v", err)
	}
	if _, err := state.svc.ListGroupUnavailability(ctx, ports.AuthContext{OrganisationID: state.organisation.ID, Roles: []string{}}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected forbidden list group unavailability without role, got %v", err)
	}
	if _, err := state.svc.ListPersonUnavailability(ctx, ports.AuthContext{OrganisationID: state.organisation.ID, Roles: []string{}}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected forbidden list person unavailability without role, got %v", err)
	}
	if _, err := state.svc.ListPersonUnavailabilityByPerson(ctx, ports.AuthContext{OrganisationID: state.organisation.ID, Roles: []string{}}, state.person.ID); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected forbidden list person unavailability by person without role, got %v", err)
	}
}

func validateServiceRemainingErrorBranchesUpdates(t *testing.T, state *serviceRemainingErrorBranchesState) {
	t.Helper()
	ctx := context.Background()

	if _, err := state.svc.UpdatePerson(ctx, state.admin, "missing", domain.Person{Name: "x", EmploymentPct: 80}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected person not found on update, got %v", err)
	}
	if _, err := state.svc.UpdatePerson(ctx, state.admin, state.person.ID, domain.Person{Name: "x", EmploymentPct: 120}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected person validation failure on update, got %v", err)
	}
	if _, err := state.svc.UpdatePerson(ctx, state.admin, state.person.ID, domain.Person{Name: "x", EmploymentPct: 80, EmploymentEffectiveFromMonth: "bad"}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected person month validation failure on update, got %v", err)
	}
	if _, err := state.svc.UpdateProject(ctx, state.admin, "missing", testProjectInput("x")); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected project not found on update, got %v", err)
	}
	if _, err := state.svc.UpdateProject(ctx, state.admin, state.project.ID, testProjectInput("")); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected project validation failure on update, got %v", err)
	}
	if _, err := state.svc.UpdateGroup(ctx, state.admin, "missing", domain.Group{Name: "x"}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected group not found on update, got %v", err)
	}
	if _, err := state.svc.UpdateGroup(ctx, state.admin, state.group.ID, domain.Group{Name: "", MemberIDs: []string{state.person.ID}}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected group validation failure on update, got %v", err)
	}
	if _, err := state.svc.UpdateAllocation(ctx, state.admin, "missing", testPersonAllocationInput(state.person.ID, state.project.ID, 10)); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected allocation not found on update, got %v", err)
	}
	if _, err := state.svc.UpdateAllocation(ctx, state.admin, state.allocation.ID, testPersonAllocationInput("", state.project.ID, 10)); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected allocation validation failure on update, got %v", err)
	}
	if _, err := state.svc.UpdateOrganisation(ctx, state.admin, state.organisation.ID, domain.Organisation{Name: "bad", HoursPerDay: -1, HoursPerWeek: 40, HoursPerYear: 2080}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected organisation validation failure on update, got %v", err)
	}
}

func validateServiceRemainingErrorBranchesDeleteAndCreate(t *testing.T, state *serviceRemainingErrorBranchesState) {
	t.Helper()
	ctx := context.Background()

	if err := state.svc.DeletePerson(ctx, state.admin, "missing"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected delete missing person not found, got %v", err)
	}
	if err := state.svc.DeleteProject(ctx, state.admin, "missing"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected delete missing project not found, got %v", err)
	}
	if err := state.svc.DeleteGroup(ctx, state.admin, "missing"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected delete missing group not found, got %v", err)
	}
	if err := state.svc.DeleteAllocation(ctx, state.admin, "missing"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected delete missing allocation not found, got %v", err)
	}
	if err := state.svc.DeleteOrgHoliday(ctx, state.admin, "missing"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected delete missing holiday not found, got %v", err)
	}
	if err := state.svc.DeleteGroupUnavailability(ctx, state.admin, "missing"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected delete missing group unavailability not found, got %v", err)
	}
	if err := state.svc.DeletePersonUnavailability(ctx, state.admin, "missing"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected delete missing person unavailability not found, got %v", err)
	}
	if err := state.svc.DeletePersonUnavailabilityByPerson(ctx, state.admin, state.person.ID, "missing"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected delete missing person-scoped unavailability not found, got %v", err)
	}

	if _, err := state.svc.CreateAllocation(ctx, state.admin, testPersonAllocationInput("", state.project.ID, 10)); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected validation for missing person id in allocation, got %v", err)
	}
	if _, err := state.svc.CreateAllocation(ctx, state.admin, testPersonAllocationInput(state.person.ID, "", 10)); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected validation for missing project id in allocation, got %v", err)
	}

	if _, err := state.svc.AddGroupMember(ctx, state.admin, state.group.ID, "missing"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected add missing member not found, got %v", err)
	}
	if _, err := state.svc.RemoveGroupMember(ctx, state.admin, "missing", state.person.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected remove member from missing group not found, got %v", err)
	}
	if _, err := state.svc.CreateGroupUnavailability(ctx, state.admin, domain.GroupUnavailability{GroupID: "missing", Date: "2026-01-01", Hours: 2}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected group unavailability missing group not found, got %v", err)
	}
	if _, err := state.svc.CreateGroupUnavailability(ctx, state.admin, domain.GroupUnavailability{GroupID: state.group.ID, Date: "2026-01-01", Hours: 99}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected group unavailability hours validation failure, got %v", err)
	}
	if _, err := state.svc.CreatePersonUnavailability(ctx, state.admin, domain.PersonUnavailability{PersonID: "missing", Date: "2026-01-01", Hours: 2}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected person unavailability missing person not found, got %v", err)
	}
	if _, err := state.svc.CreatePersonUnavailability(ctx, state.admin, domain.PersonUnavailability{PersonID: state.person.ID, Date: "bad", Hours: 2}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected person unavailability bad date validation failure, got %v", err)
	}
	if _, err := state.svc.CreatePersonUnavailability(ctx, state.admin, domain.PersonUnavailability{PersonID: state.person.ID, Date: "2026-01-01", Hours: 99}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected person unavailability hours validation failure, got %v", err)
	}
	if err := state.svc.DeletePersonUnavailabilityByPerson(ctx, ports.AuthContext{Roles: []string{domain.RoleOrgAdmin}}, state.person.ID, "missing"); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected delete person-scoped unavailability without tenant to fail, got %v", err)
	}
	if _, err := state.svc.CreateOrgHoliday(ctx, state.admin, domain.OrgHoliday{Date: "2026-01-01", Hours: 99}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected org holiday hours validation failure, got %v", err)
	}
}

func validateServiceRemainingErrorBranchesReports(t *testing.T, state *serviceRemainingErrorBranchesState) {
	t.Helper()
	ctx := context.Background()

	if _, err := state.svc.ReportAvailabilityAndLoad(ctx, state.user, domain.ReportRequest{Scope: "bad", FromDate: "2026-01-01", ToDate: "2026-01-01", Granularity: domain.GranularityDay}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected invalid scope validation, got %v", err)
	}
	if _, err := state.svc.ReportAvailabilityAndLoad(ctx, state.user, domain.ReportRequest{Scope: domain.ScopeOrganisation, FromDate: "2026-01-01", ToDate: "2026-01-01", Granularity: "bad"}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected invalid granularity validation, got %v", err)
	}
	if _, err := state.svc.ReportAvailabilityAndLoad(ctx, state.user, domain.ReportRequest{Scope: domain.ScopeOrganisation, FromDate: "bad", ToDate: "2026-01-01", Granularity: domain.GranularityDay}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected invalid from date validation, got %v", err)
	}
	if _, err := state.svc.ReportAvailabilityAndLoad(ctx, state.user, domain.ReportRequest{Scope: domain.ScopeOrganisation, FromDate: "2026-01-01", ToDate: "bad", Granularity: domain.GranularityDay}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected invalid to date validation, got %v", err)
	}
	if _, err := state.svc.ReportAvailabilityAndLoad(ctx, ports.AuthContext{Roles: []string{domain.RoleOrgUser}}, domain.ReportRequest{Scope: domain.ScopeOrganisation, FromDate: "2026-01-01", ToDate: "2026-01-01", Granularity: domain.GranularityDay}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected report forbidden without tenant, got %v", err)
	}
	if _, err := state.svc.ReportAvailabilityAndLoad(ctx, ports.AuthContext{OrganisationID: state.organisation.ID, Roles: []string{}}, domain.ReportRequest{Scope: domain.ScopeOrganisation, FromDate: "2026-01-01", ToDate: "2026-01-01", Granularity: domain.GranularityDay}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected report forbidden without role, got %v", err)
	}
}

func TestServicePersonUnavailabilityEmploymentDailyCap(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	globalAdmin := ports.AuthContext{UserID: "admin", Roles: []string{domain.RoleOrgAdmin}}
	organisation := createOrganisationForService(t, svc, ctx, globalAdmin, "Org Capacity")
	admin := ports.AuthContext{UserID: "admin1", OrganisationID: organisation.ID, Roles: []string{domain.RoleOrgAdmin}}

	person, err := svc.CreatePerson(ctx, admin, domain.Person{Name: "Part Time", EmploymentPct: 80})
	if err != nil {
		t.Fatalf("setup person: %v", err)
	}

	_, err = svc.CreatePersonUnavailability(ctx, admin, domain.PersonUnavailability{PersonID: person.ID, Date: "2026-03-01", Hours: 4})
	if err != nil {
		t.Fatalf("expected valid person unavailability within capacity, got %v", err)
	}
	_, err = svc.CreatePersonUnavailability(ctx, admin, domain.PersonUnavailability{PersonID: person.ID, Date: "2026-03-02", Hours: 6.5})
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected over-capacity daily unavailability to fail, got %v", err)
	}

	_, err = svc.CreatePersonUnavailability(ctx, admin, domain.PersonUnavailability{PersonID: person.ID, Date: "2026-03-03", Hours: 3.5})
	if err != nil {
		t.Fatalf("expected first same-day unavailability entry to pass, got %v", err)
	}
	_, err = svc.CreatePersonUnavailability(ctx, admin, domain.PersonUnavailability{PersonID: person.ID, Date: "2026-03-03", Hours: 3})
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected cumulative same-day unavailability over capacity to fail, got %v", err)
	}
}

func TestServicePersonEmploymentChangesByMonth(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	globalAdmin := ports.AuthContext{UserID: "admin", Roles: []string{domain.RoleOrgAdmin}}
	organisation := createOrganisationForService(t, svc, ctx, globalAdmin, "Org Employment Timeline")
	admin := ports.AuthContext{UserID: "admin1", OrganisationID: organisation.ID, Roles: []string{domain.RoleOrgAdmin}}

	person, err := svc.CreatePerson(ctx, admin, domain.Person{Name: "Timeline Person", EmploymentPct: 80})
	if err != nil {
		t.Fatalf("setup person: %v", err)
	}

	person = applyTimelineEmploymentChanges(t, svc, ctx, admin, person)
	if len(person.EmploymentChanges) != 2 {
		t.Fatalf("expected 2 employment changes, got %+v", person.EmploymentChanges)
	}

	assertEmploymentPctOnDate(t, person, "2026-04-15", 80, "expected april employment percent")
	assertEmploymentPctOnDate(t, person, "2026-06-15", 70, "expected june employment percent")
	assertEmploymentPctOnDate(t, person, "2026-09-01", 50, "expected september employment percent")

	person, err = svc.UpdatePerson(ctx, admin, person.ID, domain.Person{Name: "Timeline Person", EmploymentPct: 90})
	if err != nil {
		t.Fatalf("update baseline employment percent: %v", err)
	}

	assertEmploymentPctOnDate(t, person, "2026-04-15", 90, "expected april employment percent after baseline change")
	assertEmploymentPctOnDate(t, person, "2026-06-15", 70, "expected june employment percent after baseline change")
	assertEmploymentPctOnDate(t, person, "2026-09-01", 50, "expected september employment percent after baseline change")

	assertEmploymentTimelineUnavailabilityCaps(t, svc, ctx, admin, person.ID)
}

func applyTimelineEmploymentChanges(t *testing.T, svc *Service, ctx context.Context, admin ports.AuthContext, person domain.Person) domain.Person {
	t.Helper()

	if _, err := svc.UpdatePerson(ctx, admin, person.ID, domain.Person{
		Name:                         "Timeline Person",
		EmploymentPct:                60,
		EmploymentEffectiveFromMonth: "2026-05",
	}); err != nil {
		t.Fatalf("set first monthly employment change: %v", err)
	}
	if _, err := svc.UpdatePerson(ctx, admin, person.ID, domain.Person{
		Name:                         "Timeline Person",
		EmploymentPct:                50,
		EmploymentEffectiveFromMonth: "2026-08",
	}); err != nil {
		t.Fatalf("set second monthly employment change: %v", err)
	}
	updatedPerson, err := svc.UpdatePerson(ctx, admin, person.ID, domain.Person{
		Name:                         "Timeline Person",
		EmploymentPct:                70,
		EmploymentEffectiveFromMonth: "2026-05",
	})
	if err != nil {
		t.Fatalf("replace monthly employment change for same month: %v", err)
	}
	return updatedPerson
}

func assertEmploymentPctOnDate(t *testing.T, person domain.Person, date string, want float64, contextLabel string) {
	t.Helper()

	got, err := domain.EmploymentPctOnDate(person, date)
	if err != nil {
		t.Fatalf("%s, got %v", contextLabel, err)
	}
	if got != want {
		t.Fatalf("%s %v, got %v", contextLabel, want, got)
	}
}

func assertEmploymentTimelineUnavailabilityCaps(t *testing.T, svc *Service, ctx context.Context, admin ports.AuthContext, personID string) {
	t.Helper()

	_, err := svc.CreatePersonUnavailability(ctx, admin, domain.PersonUnavailability{PersonID: personID, Date: "2026-04-07", Hours: 7.2})
	if err != nil {
		t.Fatalf("expected april unavailability to use 90 pct cap, got %v", err)
	}
	_, err = svc.CreatePersonUnavailability(ctx, admin, domain.PersonUnavailability{PersonID: personID, Date: "2026-06-07", Hours: 6})
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected june unavailability above 70 pct cap to fail, got %v", err)
	}
	_, err = svc.CreatePersonUnavailability(ctx, admin, domain.PersonUnavailability{PersonID: personID, Date: "2026-06-08", Hours: 5.6})
	if err != nil {
		t.Fatalf("expected june unavailability within 70 pct cap, got %v", err)
	}
	_, err = svc.CreatePersonUnavailability(ctx, admin, domain.PersonUnavailability{PersonID: personID, Date: "2026-09-02", Hours: 4.1})
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected september unavailability above 50 pct cap to fail, got %v", err)
	}
}

func TestServiceForbiddenMutationsForOrgUser(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()
	globalAdmin := ports.AuthContext{UserID: "admin", Roles: []string{domain.RoleOrgAdmin}}
	organisation := createOrganisationForService(t, svc, ctx, globalAdmin, "Org Forbidden")
	admin := ports.AuthContext{UserID: "admin1", OrganisationID: organisation.ID, Roles: []string{domain.RoleOrgAdmin}}
	user := ports.AuthContext{UserID: "user1", OrganisationID: organisation.ID, Roles: []string{domain.RoleOrgUser}}

	person, err := svc.CreatePerson(ctx, admin, domain.Person{Name: "Lock Person", EmploymentPct: 100})
	if err != nil {
		t.Fatalf("setup person: %v", err)
	}
	project, err := svc.CreateProject(ctx, admin, testProjectInput("Lock Project"))
	if err != nil {
		t.Fatalf("setup project: %v", err)
	}
	group, err := svc.CreateGroup(ctx, admin, domain.Group{Name: "Lock Group", MemberIDs: []string{person.ID}})
	if err != nil {
		t.Fatalf("setup group: %v", err)
	}
	allocation, err := svc.CreateAllocation(ctx, admin, testPersonAllocationInput(person.ID, project.ID, 10))
	if err != nil {
		t.Fatalf("setup allocation: %v", err)
	}
	holiday, err := svc.CreateOrgHoliday(ctx, admin, domain.OrgHoliday{Date: "2026-02-01", Hours: 8})
	if err != nil {
		t.Fatalf("setup holiday: %v", err)
	}
	groupUnavailable, err := svc.CreateGroupUnavailability(ctx, admin, domain.GroupUnavailability{GroupID: group.ID, Date: "2026-02-02", Hours: 1})
	if err != nil {
		t.Fatalf("setup group unavailability: %v", err)
	}
	personUnavailable, err := svc.CreatePersonUnavailability(ctx, admin, domain.PersonUnavailability{PersonID: person.ID, Date: "2026-02-03", Hours: 1})
	if err != nil {
		t.Fatalf("setup person unavailability: %v", err)
	}

	_, err = svc.UpdateOrganisation(ctx, user, organisation.ID, domain.Organisation{Name: "x", HoursPerDay: 8, HoursPerWeek: 40, HoursPerYear: 2080})
	expectForbiddenError(t, err)
	expectForbiddenError(t, svc.DeleteOrganisation(ctx, user, organisation.ID))
	_, err = svc.CreatePerson(ctx, user, domain.Person{Name: "x", EmploymentPct: 100})
	expectForbiddenError(t, err)
	_, err = svc.UpdatePerson(ctx, user, person.ID, domain.Person{Name: "x", EmploymentPct: 100})
	expectForbiddenError(t, err)
	expectForbiddenError(t, svc.DeletePerson(ctx, user, person.ID))
	_, err = svc.CreateProject(ctx, user, testProjectInput("x"))
	expectForbiddenError(t, err)
	_, err = svc.UpdateProject(ctx, user, project.ID, testProjectInput("x"))
	expectForbiddenError(t, err)
	expectForbiddenError(t, svc.DeleteProject(ctx, user, project.ID))
	_, err = svc.CreateGroup(ctx, user, domain.Group{Name: "x"})
	expectForbiddenError(t, err)
	_, err = svc.UpdateGroup(ctx, user, group.ID, domain.Group{Name: "x"})
	expectForbiddenError(t, err)
	expectForbiddenError(t, svc.DeleteGroup(ctx, user, group.ID))
	_, err = svc.AddGroupMember(ctx, user, group.ID, person.ID)
	expectForbiddenError(t, err)
	_, err = svc.RemoveGroupMember(ctx, user, group.ID, person.ID)
	expectForbiddenError(t, err)
	_, err = svc.CreateAllocation(ctx, user, testPersonAllocationInput(person.ID, project.ID, 1))
	expectForbiddenError(t, err)
	_, err = svc.UpdateAllocation(ctx, user, allocation.ID, testPersonAllocationInput(person.ID, project.ID, 1))
	expectForbiddenError(t, err)
	expectForbiddenError(t, svc.DeleteAllocation(ctx, user, allocation.ID))
	_, err = svc.CreateOrgHoliday(ctx, user, domain.OrgHoliday{Date: "2026-01-01", Hours: 8})
	expectForbiddenError(t, err)
	expectForbiddenError(t, svc.DeleteOrgHoliday(ctx, user, holiday.ID))
	_, err = svc.CreateGroupUnavailability(ctx, user, domain.GroupUnavailability{GroupID: group.ID, Date: "2026-01-01", Hours: 1})
	expectForbiddenError(t, err)
	expectForbiddenError(t, svc.DeleteGroupUnavailability(ctx, user, groupUnavailable.ID))
	_, err = svc.CreatePersonUnavailability(ctx, user, domain.PersonUnavailability{PersonID: person.ID, Date: "2026-01-01", Hours: 1})
	expectForbiddenError(t, err)
	expectForbiddenError(t, svc.DeletePersonUnavailability(ctx, user, personUnavailable.ID))
	expectForbiddenError(t, svc.DeletePersonUnavailabilityByPerson(ctx, user, person.ID, personUnavailable.ID))
}

func expectForbiddenError(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected forbidden error, got %v", err)
	}
}

func TestServiceAdditionalBranchCoverage(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()
	globalAdmin := ports.AuthContext{UserID: "admin", Roles: []string{domain.RoleOrgAdmin}}
	organisation := createOrganisationForService(t, svc, ctx, globalAdmin, "Org Extra")
	admin := ports.AuthContext{UserID: "admin1", OrganisationID: organisation.ID, Roles: []string{domain.RoleOrgAdmin}}

	person, err := svc.CreatePerson(ctx, admin, domain.Person{Name: "Extra Person", EmploymentPct: 50})
	if err != nil {
		t.Fatalf("setup person: %v", err)
	}
	project, err := svc.CreateProject(ctx, admin, testProjectInput("Extra Project"))
	if err != nil {
		t.Fatalf("setup project: %v", err)
	}
	group, err := svc.CreateGroup(ctx, admin, domain.Group{Name: "Extra Group", MemberIDs: []string{person.ID}})
	if err != nil {
		t.Fatalf("setup group: %v", err)
	}
	allocation, err := svc.CreateAllocation(ctx, admin, testPersonAllocationInput(person.ID, project.ID, 20))
	if err != nil {
		t.Fatalf("setup allocation: %v", err)
	}

	assertServiceAdditionalForbiddenReads(t, svc, ctx, person.ID, project.ID, group.ID, allocation.ID)
	assertServiceAdditionalMissingOrgCreates(t, svc, ctx, group.ID, person.ID)
	assertServiceAdditionalUpdateScenarios(t, svc, ctx, admin, group.ID, project.ID, person.ID, allocation.ID)
	assertServiceAdditionalTenantGuards(t, svc, ctx, organisation.ID, group.ID, person.ID)
	assertServiceAdditionalReportAndHelperValidation(t, svc, ctx, admin, project.ID, person.ID)
}

func assertServiceAdditionalForbiddenReads(t *testing.T, svc *Service, ctx context.Context, personID, projectID, groupID, allocationID string) {
	t.Helper()

	_, err := svc.GetPerson(ctx, ports.AuthContext{Roles: []string{domain.RoleOrgAdmin}}, personID)
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected forbidden get person without org scope, got %v", err)
	}
	_, err = svc.GetProject(ctx, ports.AuthContext{Roles: []string{domain.RoleOrgAdmin}}, projectID)
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected forbidden get project without org scope, got %v", err)
	}
	_, err = svc.GetGroup(ctx, ports.AuthContext{Roles: []string{domain.RoleOrgAdmin}}, groupID)
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected forbidden get group without org scope, got %v", err)
	}
	_, err = svc.GetAllocation(ctx, ports.AuthContext{Roles: []string{domain.RoleOrgAdmin}}, allocationID)
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected forbidden get allocation without org scope, got %v", err)
	}
}

func assertServiceAdditionalMissingOrgCreates(t *testing.T, svc *Service, ctx context.Context, groupID, personID string) {
	t.Helper()

	_, err := svc.CreatePerson(ctx, ports.AuthContext{OrganisationID: "missing", Roles: []string{domain.RoleOrgAdmin}}, domain.Person{Name: "x", EmploymentPct: 50})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected create person with missing org to fail, got %v", err)
	}
	_, err = svc.CreateOrgHoliday(ctx, ports.AuthContext{OrganisationID: "missing", Roles: []string{domain.RoleOrgAdmin}}, domain.OrgHoliday{Date: "2026-01-01", Hours: 8})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected create holiday with missing org to fail, got %v", err)
	}
	_, err = svc.CreateGroupUnavailability(ctx, ports.AuthContext{OrganisationID: "missing", Roles: []string{domain.RoleOrgAdmin}}, domain.GroupUnavailability{GroupID: groupID, Date: "2026-01-01", Hours: 1})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected create group unavailability with missing org to fail, got %v", err)
	}
	_, err = svc.CreatePersonUnavailability(ctx, ports.AuthContext{OrganisationID: "missing", Roles: []string{domain.RoleOrgAdmin}}, domain.PersonUnavailability{PersonID: personID, Date: "2026-01-01", Hours: 1})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected create person unavailability with missing org to fail, got %v", err)
	}
}

func assertServiceAdditionalUpdateScenarios(t *testing.T, svc *Service, ctx context.Context, admin ports.AuthContext, groupID, projectID, personID, allocationID string) {
	t.Helper()

	_, err := svc.UpdateGroup(ctx, admin, groupID, domain.Group{Name: "x", MemberIDs: []string{"missing"}})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected update group member validation not found, got %v", err)
	}
	_, err = svc.UpdateAllocation(ctx, admin, allocationID, testPersonAllocationInput("missing", projectID, 10))
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected update allocation missing person to fail, got %v", err)
	}
	_, err = svc.UpdateAllocation(ctx, admin, allocationID, testPersonAllocationInput(personID, "missing", 10))
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected update allocation missing project to fail, got %v", err)
	}
	updatedAllocation, err := svc.UpdateAllocation(ctx, admin, allocationID, testPersonAllocationInput(personID, projectID, 60))
	if err != nil {
		t.Fatalf("expected update allocation to allow over-employment load, got %v", err)
	}
	if updatedAllocation.Percent != 60 {
		t.Fatalf("expected updated allocation percent 60, got %v", updatedAllocation.Percent)
	}
}

func assertServiceAdditionalTenantGuards(t *testing.T, svc *Service, ctx context.Context, organisationID, groupID, personID string) {
	t.Helper()

	_, err := svc.AddGroupMember(ctx, ports.AuthContext{Roles: []string{domain.RoleOrgAdmin}}, groupID, personID)
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected add group member without org scope to fail, got %v", err)
	}
	_, err = svc.UpdateOrganisation(ctx, ports.AuthContext{OrganisationID: "other", Roles: []string{domain.RoleOrgAdmin}}, organisationID, domain.Organisation{Name: "x", HoursPerDay: 8, HoursPerWeek: 40, HoursPerYear: 2080})
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected update organisation cross tenant to fail, got %v", err)
	}
	err = svc.DeleteOrganisation(ctx, ports.AuthContext{OrganisationID: "other", Roles: []string{domain.RoleOrgAdmin}}, organisationID)
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected delete organisation cross tenant to fail, got %v", err)
	}
}

func assertServiceAdditionalReportAndHelperValidation(t *testing.T, svc *Service, ctx context.Context, admin ports.AuthContext, projectID, personID string) {
	t.Helper()

	_, err := svc.ReportAvailabilityAndLoad(ctx, admin, domain.ReportRequest{Scope: domain.ScopePerson, IDs: []string{"missing"}, FromDate: "2026-01-01", ToDate: "2026-01-02", Granularity: domain.GranularityDay})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected report missing person id to fail, got %v", err)
	}
	_, err = svc.ReportAvailabilityAndLoad(ctx, admin, domain.ReportRequest{Scope: domain.ScopeGroup, IDs: []string{"missing"}, FromDate: "2026-01-01", ToDate: "2026-01-02", Granularity: domain.GranularityDay})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected report missing group id to fail, got %v", err)
	}
	_, err = svc.ReportAvailabilityAndLoad(ctx, admin, domain.ReportRequest{Scope: domain.ScopeOrganisation, IDs: nil, FromDate: "2026-01-02", ToDate: "2026-01-01", Granularity: domain.GranularityDay})
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected report reversed date range to fail, got %v", err)
	}

	err = requireAnyRole(ports.AuthContext{})
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected requireAnyRole with no roles to fail, got %v", err)
	}
	err = validateAllocation(testPersonAllocationInput(personID, projectID, 101))
	if err != nil {
		t.Fatalf("expected validate allocation percent above 100 to pass, got %v", err)
	}
	err = validateAllocation(testPersonAllocationInput(personID, projectID, -1))
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected validate allocation negative percent to fail, got %v", err)
	}
}

func TestAllocationValidationHelpers(t *testing.T) {
	validProject := testProjectInput("Helper Project")
	if err := validateProject(validProject); err != nil {
		t.Fatalf("expected valid project, got %v", err)
	}

	noEffortProject := testProjectInput("No Effort")
	noEffortProject.EstimatedEffortHours = 0
	if err := validateProject(noEffortProject); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected no-effort project validation error, got %v", err)
	}

	missingDateProject := testProjectInput("Missing Date")
	missingDateProject.StartDate = ""
	if err := validateProject(missingDateProject); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected missing date validation error, got %v", err)
	}

	reversedDateProject := testProjectInput("Reversed Date")
	reversedDateProject.StartDate = "2026-02-01"
	reversedDateProject.EndDate = "2026-01-01"
	if err := validateProject(reversedDateProject); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected reversed date validation error, got %v", err)
	}

	validAllocation := testPersonAllocationInput("person_1", "project_1", 10)
	if err := validateAllocation(validAllocation); err != nil {
		t.Fatalf("expected valid allocation, got %v", err)
	}
	if err := validateDateHours("2026-01-01", math.NaN(), 8); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected NaN hours validation error, got %v", err)
	}

	badTargetType := validAllocation
	badTargetType.TargetType = "bad"
	if err := validateAllocation(badTargetType); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected bad target type validation error, got %v", err)
	}

	missingTarget := validAllocation
	missingTarget.TargetID = ""
	if err := validateAllocation(missingTarget); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected missing target validation error, got %v", err)
	}

	missingProjectID := validAllocation
	missingProjectID.ProjectID = ""
	if err := validateAllocation(missingProjectID); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected missing project validation error, got %v", err)
	}

	missingDates := validAllocation
	missingDates.StartDate = ""
	if err := validateAllocation(missingDates); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected missing dates validation error, got %v", err)
	}

	normalized := normalizeAllocationInput(domain.Allocation{PersonID: "person_legacy"})
	if normalized.TargetType != domain.AllocationTargetPerson || normalized.TargetID != "person_legacy" {
		t.Fatalf("expected legacy person id normalization, got %+v", normalized)
	}

	openStart, openEnd, err := parseDateRange("", "")
	if err != nil {
		t.Fatalf("expected open date range to parse, got %v", err)
	}
	if !openStart.Before(openEnd) {
		t.Fatalf("expected open range start before end, got %v %v", openStart, openEnd)
	}
	_, _, err = parseDateRange("2026-02-01", "2026-01-01")
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected parseDateRange reversed date error, got %v", err)
	}

	project := testProjectInput("Project Range")
	allocationWithin := testPersonAllocationInput("person_1", "project_1", 10)
	err = validateAllocationWithinProjectRange(allocationWithin, project)
	if err != nil {
		t.Fatalf("expected allocation inside project range, got %v", err)
	}

	allocationOutside := testPersonAllocationInput("person_1", "project_1", 10)
	allocationOutside.StartDate = "2025-12-31"
	err = validateAllocationWithinProjectRange(allocationOutside, project)
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected allocation outside project range validation error, got %v", err)
	}
}

func TestOverlapDateRanges(t *testing.T) {
	tests := []struct {
		name          string
		rangeStartA   string
		rangeEndA     string
		rangeStartB   string
		rangeEndB     string
		expectOverlap bool
		expectStart   string
		expectEnd     string
	}{
		{
			name:          "full overlap",
			rangeStartA:   "2026-01-01",
			rangeEndA:     "2026-01-10",
			rangeStartB:   "2026-01-03",
			rangeEndB:     "2026-01-08",
			expectOverlap: true,
			expectStart:   "2026-01-03",
			expectEnd:     "2026-01-08",
		},
		{
			name:          "left trimmed overlap",
			rangeStartA:   "2026-01-05",
			rangeEndA:     "2026-01-10",
			rangeStartB:   "2026-01-01",
			rangeEndB:     "2026-01-07",
			expectOverlap: true,
			expectStart:   "2026-01-05",
			expectEnd:     "2026-01-07",
		},
		{
			name:          "right trimmed overlap",
			rangeStartA:   "2026-01-05",
			rangeEndA:     "2026-01-10",
			rangeStartB:   "2026-01-08",
			rangeEndB:     "2026-01-15",
			expectOverlap: true,
			expectStart:   "2026-01-08",
			expectEnd:     "2026-01-10",
		},
		{
			name:          "single day overlap",
			rangeStartA:   "2026-01-05",
			rangeEndA:     "2026-01-10",
			rangeStartB:   "2026-01-10",
			rangeEndB:     "2026-01-15",
			expectOverlap: true,
			expectStart:   "2026-01-10",
			expectEnd:     "2026-01-10",
		},
		{
			name:          "no overlap",
			rangeStartA:   "2026-01-05",
			rangeEndA:     "2026-01-10",
			rangeStartB:   "2026-01-11",
			rangeEndB:     "2026-01-15",
			expectOverlap: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rangeStartA := mustParseTestDate(t, tc.rangeStartA)
			rangeEndA := mustParseTestDate(t, tc.rangeEndA)
			rangeStartB := mustParseTestDate(t, tc.rangeStartB)
			rangeEndB := mustParseTestDate(t, tc.rangeEndB)

			start, end, overlaps := overlapDateRanges(rangeStartA, rangeEndA, rangeStartB, rangeEndB)
			if overlaps != tc.expectOverlap {
				t.Fatalf("expected overlap=%v, got %v", tc.expectOverlap, overlaps)
			}
			if !overlaps {
				return
			}
			if start.Format(domain.DateLayout) != tc.expectStart {
				t.Fatalf("expected overlap start %s, got %s", tc.expectStart, start.Format(domain.DateLayout))
			}
			if end.Format(domain.DateLayout) != tc.expectEnd {
				t.Fatalf("expected overlap end %s, got %s", tc.expectEnd, end.Format(domain.DateLayout))
			}
		})
	}
}

func TestBuildAllocationEvents(t *testing.T) {
	candidateStart := mustParseTestDate(t, "2026-01-05")
	candidateEnd := mustParseTestDate(t, "2026-01-15")
	groupsByID := map[string]domain.Group{
		"group_1": {ID: "group_1", MemberIDs: []string{"person_1"}},
	}

	tests := []struct {
		name          string
		allocations   []domain.Allocation
		allocationID  string
		personID      string
		expectEvents  map[string]float64
		expectErrType error
	}{
		{
			name:         "tracks overlapping person and group allocations",
			allocationID: "skip_me",
			personID:     "person_1",
			allocations: []domain.Allocation{
				{
					ID:         "skip_me",
					TargetType: domain.AllocationTargetPerson,
					TargetID:   "person_1",
					StartDate:  "2026-01-01",
					EndDate:    "2026-01-10",
					Percent:    50,
				},
				{
					ID:         "person_overlap",
					TargetType: domain.AllocationTargetPerson,
					TargetID:   "person_1",
					StartDate:  "2026-01-01",
					EndDate:    "2026-01-10",
					Percent:    20,
				},
				{
					ID:         "group_overlap",
					TargetType: domain.AllocationTargetGroup,
					TargetID:   "group_1",
					StartDate:  "2026-01-10",
					EndDate:    "2026-01-20",
					Percent:    15,
				},
				{
					ID:         "other_person",
					TargetType: domain.AllocationTargetPerson,
					TargetID:   "person_2",
					StartDate:  "2026-01-05",
					EndDate:    "2026-01-15",
					Percent:    99,
				},
			},
			expectEvents: map[string]float64{
				"2026-01-05": 20,
				"2026-01-10": 15,
				"2026-01-11": -20,
				"2026-01-16": -15,
			},
		},
		{
			name:         "rejects invalid allocation dates",
			allocationID: "",
			personID:     "person_1",
			allocations: []domain.Allocation{
				{
					ID:         "bad_date",
					TargetType: domain.AllocationTargetPerson,
					TargetID:   "person_1",
					StartDate:  "bad-date",
					EndDate:    "2026-01-10",
					Percent:    20,
				},
			},
			expectErrType: domain.ErrValidation,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			events, err := buildAllocationEvents(tc.allocations, tc.allocationID, tc.personID, groupsByID, candidateStart, candidateEnd)
			if tc.expectErrType != nil {
				if !errors.Is(err, tc.expectErrType) {
					t.Fatalf("expected error %v, got %v", tc.expectErrType, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("build allocation events: %v", err)
			}
			formattedEvents := formatEventMapByDate(events)
			if !reflect.DeepEqual(formattedEvents, tc.expectEvents) {
				t.Fatalf("unexpected events, expected %+v got %+v", tc.expectEvents, formattedEvents)
			}
		})
	}
}

func TestAllocationTargetResolutionAndLimitRangeChecks(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()
	globalAdmin := ports.AuthContext{UserID: "admin", Roles: []string{domain.RoleOrgAdmin}}
	organisation := createOrganisationForService(t, svc, ctx, globalAdmin, "Org Limits")
	admin := ports.AuthContext{UserID: "admin1", OrganisationID: organisation.ID, Roles: []string{domain.RoleOrgAdmin}}

	person, err := svc.CreatePerson(ctx, admin, domain.Person{Name: "Range Person", EmploymentPct: 50})
	if err != nil {
		t.Fatalf("create person: %v", err)
	}
	project, err := svc.CreateProject(ctx, admin, testProjectInput("Range Project"))
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	group, err := svc.CreateGroup(ctx, admin, domain.Group{Name: "Range Group", MemberIDs: []string{person.ID}})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	emptyGroup, err := svc.CreateGroup(ctx, admin, domain.Group{Name: "Empty Group"})
	if err != nil {
		t.Fatalf("create empty group: %v", err)
	}

	personIDs, err := svc.resolveAllocationTargetPersons(ctx, organisation.ID, domain.AllocationTargetPerson, person.ID)
	if err != nil || len(personIDs) != 1 || personIDs[0] != person.ID {
		t.Fatalf("unexpected person target resolution result %v err=%v", personIDs, err)
	}

	groupPersonIDs, err := svc.resolveAllocationTargetPersons(ctx, organisation.ID, domain.AllocationTargetGroup, group.ID)
	if err != nil || len(groupPersonIDs) != 1 || groupPersonIDs[0] != person.ID {
		t.Fatalf("unexpected group target resolution result %v err=%v", groupPersonIDs, err)
	}
	_, err = svc.resolveAllocationTargetPersons(ctx, organisation.ID, "invalid", group.ID)
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected invalid target type error, got %v", err)
	}
	_, err = svc.resolveAllocationTargetPersons(ctx, organisation.ID, domain.AllocationTargetGroup, emptyGroup.ID)
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected empty group validation error, got %v", err)
	}

	groupsByID := map[string]domain.Group{group.ID: group}
	if !allocationTargetsPerson(testGroupAllocationInput(group.ID, project.ID, 10), person.ID, groupsByID) {
		t.Fatalf("expected group allocation to target member")
	}
	if allocationTargetsPerson(testGroupAllocationInput(group.ID, project.ID, 10), "other", groupsByID) {
		t.Fatalf("expected group allocation not to target non-member")
	}
	if !allocationTargetsPerson(testPersonAllocationInput(person.ID, project.ID, 10), person.ID, groupsByID) {
		t.Fatalf("expected person allocation to target person")
	}
	legacy := domain.Allocation{PersonID: person.ID}
	if !allocationTargetsPerson(legacy, person.ID, groupsByID) {
		t.Fatalf("expected legacy person allocation to target person")
	}

	_, err = svc.CreateAllocation(ctx, admin, testPersonAllocationInputForRange(person.ID, project.ID, 280, "2026-01-01", "2026-01-10"))
	if err != nil {
		t.Fatalf("create baseline allocation: %v", err)
	}

	nonOverlapping := testPersonAllocationInputForRange(person.ID, project.ID, 30, "2026-01-11", "2026-01-20")
	err = svc.validateAllocationLimit(ctx, organisation.ID, nonOverlapping, []string{person.ID}, "")
	if err != nil {
		t.Fatalf("expected non-overlapping allocation to pass limit, got %v", err)
	}

	overlapping := testPersonAllocationInputForRange(person.ID, project.ID, 30, "2026-01-05", "2026-01-15")
	err = svc.validateAllocationLimit(ctx, organisation.ID, overlapping, []string{person.ID}, "")
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected overlapping allocation to fail limit, got %v", err)
	}
}

func TestValidateScopeIDsRejectsUnknownScope(t *testing.T) {
	err := validateScopeIDs(domain.ReportRequest{Scope: "unknown", IDs: []string{"id_1"}}, nil, nil, nil)
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected invalid scope to fail validation, got %v", err)
	}
}

func testProjectInput(name string) domain.Project {
	return domain.Project{
		Name:                 name,
		StartDate:            "2026-01-01",
		EndDate:              "2026-12-31",
		EstimatedEffortHours: 1000,
	}
}

func testPersonAllocationInput(personID, projectID string, percent float64) domain.Allocation {
	return domain.Allocation{
		TargetType: domain.AllocationTargetPerson,
		TargetID:   personID,
		ProjectID:  projectID,
		StartDate:  "2026-01-01",
		EndDate:    "2026-12-31",
		Percent:    percent,
	}
}

func testPersonAllocationInputForRange(personID, projectID string, percent float64, startDate, endDate string) domain.Allocation {
	allocation := testPersonAllocationInput(personID, projectID, percent)
	allocation.StartDate = startDate
	allocation.EndDate = endDate
	return allocation
}

func testGroupAllocationInput(groupID, projectID string, percent float64) domain.Allocation {
	return domain.Allocation{
		TargetType: domain.AllocationTargetGroup,
		TargetID:   groupID,
		ProjectID:  projectID,
		StartDate:  "2026-01-01",
		EndDate:    "2026-12-31",
		Percent:    percent,
	}
}

func mustParseTestDate(t *testing.T, date string) time.Time {
	t.Helper()
	parsed, err := time.Parse(domain.DateLayout, date)
	if err != nil {
		t.Fatalf("parse test date %q: %v", date, err)
	}
	return parsed
}

func formatEventMapByDate(events map[time.Time]float64) map[string]float64 {
	formatted := make(map[string]float64, len(events))
	for date, delta := range events {
		formatted[date.Format(domain.DateLayout)] = delta
	}
	return formatted
}

func createOrganisationForService(t *testing.T, svc *Service, ctx context.Context, auth ports.AuthContext, name string) domain.Organisation {
	t.Helper()
	organisation, err := svc.CreateOrganisation(ctx, auth, domain.Organisation{
		Name:         name,
		HoursPerDay:  8,
		HoursPerWeek: 40,
		HoursPerYear: 2080,
	})
	if err != nil {
		t.Fatalf("create organisation helper: %v", err)
	}
	return organisation
}

func newTestService(t *testing.T) *Service {
	t.Helper()
	repo, err := persistence.NewFileRepository(filepath.Join(t.TempDir(), "service-data.json"))
	if err != nil {
		t.Fatalf("create repository: %v", err)
	}
	svc, err := New(repo, telemetry.NewNoopTelemetry(), impexp.NewNoopImportExport())
	if err != nil {
		t.Fatalf("create service: %v", err)
	}
	return svc
}
