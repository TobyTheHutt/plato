package persistence

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"plato/backend/internal/domain"
)

func TestFileRepositoryCRUDAndCascade(t *testing.T) {
	ctx := context.Background()
	repo, err := NewFileRepository(filepath.Join(t.TempDir(), "repo.json"))
	if err != nil {
		t.Fatalf("new repo: %v", err)
	}

	orgA, err := repo.CreateOrganisation(ctx, domain.Organisation{Name: "Org A", HoursPerDay: 8, HoursPerWeek: 40, HoursPerYear: 2080})
	if err != nil {
		t.Fatalf("create org A: %v", err)
	}
	orgB, err := repo.CreateOrganisation(ctx, domain.Organisation{Name: "Org B", HoursPerDay: 8, HoursPerWeek: 40, HoursPerYear: 2080})
	if err != nil {
		t.Fatalf("create org B: %v", err)
	}

	organisations, err := repo.ListOrganisations(ctx)
	if err != nil {
		t.Fatalf("list organisations: %v", err)
	}
	if len(organisations) != 2 {
		t.Fatalf("expected 2 organisations, got %d", len(organisations))
	}

	orgA.Name = "Org A Updated"
	orgA, err = repo.UpdateOrganisation(ctx, orgA)
	if err != nil {
		t.Fatalf("update organisation: %v", err)
	}
	if orgA.Name != "Org A Updated" {
		t.Fatalf("unexpected organisation update: %#v", orgA)
	}

	personA1, err := repo.CreatePerson(ctx, domain.Person{OrganisationID: orgA.ID, Name: "Alice", EmploymentPct: 100})
	if err != nil {
		t.Fatalf("create person A1: %v", err)
	}
	personA2, err := repo.CreatePerson(ctx, domain.Person{OrganisationID: orgA.ID, Name: "Bob", EmploymentPct: 60})
	if err != nil {
		t.Fatalf("create person A2: %v", err)
	}
	personA3, err := repo.CreatePerson(ctx, domain.Person{OrganisationID: orgA.ID, Name: "Bob", EmploymentPct: 40})
	if err != nil {
		t.Fatalf("create person A3: %v", err)
	}
	_, err = repo.CreatePerson(ctx, domain.Person{OrganisationID: orgB.ID, Name: "Other", EmploymentPct: 100})
	if err != nil {
		t.Fatalf("create person B: %v", err)
	}
	personsInA, err := repo.ListPersons(ctx, orgA.ID)
	if err != nil {
		t.Fatalf("list persons in org A: %v", err)
	}
	if len(personsInA) != 3 {
		t.Fatalf("expected 3 persons in org A, got %d", len(personsInA))
	}

	personA1.EmploymentPct = 90
	personA1, err = repo.UpdatePerson(ctx, personA1)
	if err != nil {
		t.Fatalf("update person: %v", err)
	}
	if personA1.EmploymentPct != 90 {
		t.Fatalf("expected employment update, got %v", personA1.EmploymentPct)
	}

	if _, err := repo.GetPerson(ctx, orgB.ID, personA1.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected not found across tenant, got %v", err)
	}

	projectA1, err := repo.CreateProject(ctx, domain.Project{OrganisationID: orgA.ID, Name: "Project A1"})
	if err != nil {
		t.Fatalf("create project A1: %v", err)
	}
	projectA2, err := repo.CreateProject(ctx, domain.Project{OrganisationID: orgA.ID, Name: "Project A2"})
	if err != nil {
		t.Fatalf("create project A2: %v", err)
	}
	_, err = repo.CreateProject(ctx, domain.Project{OrganisationID: orgB.ID, Name: "Project B1"})
	if err != nil {
		t.Fatalf("create project B1: %v", err)
	}

	projectA1.Name = "Project A1 Updated"
	projectA1, err = repo.UpdateProject(ctx, projectA1)
	if err != nil {
		t.Fatalf("update project: %v", err)
	}
	if projectA1.Name != "Project A1 Updated" {
		t.Fatalf("project update failed: %#v", projectA1)
	}
	projects, err := repo.ListProjects(ctx, orgA.ID)
	if err != nil {
		t.Fatalf("list projects: %v", err)
	}
	if len(projects) != 2 {
		t.Fatalf("expected 2 projects in org A, got %d", len(projects))
	}

	groupA, err := repo.CreateGroup(ctx, domain.Group{OrganisationID: orgA.ID, Name: "Team A", MemberIDs: []string{personA1.ID, personA1.ID}})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	if len(groupA.MemberIDs) != 1 {
		t.Fatalf("expected de-duplicated members, got %v", groupA.MemberIDs)
	}
	_, err = repo.CreateGroup(ctx, domain.Group{OrganisationID: orgA.ID, Name: "Team A", MemberIDs: []string{personA3.ID}})
	if err != nil {
		t.Fatalf("create second group: %v", err)
	}

	groupA.MemberIDs = []string{personA1.ID, personA2.ID}
	groupA, err = repo.UpdateGroup(ctx, groupA)
	if err != nil {
		t.Fatalf("update group: %v", err)
	}
	if len(groupA.MemberIDs) != 2 {
		t.Fatalf("expected 2 members, got %v", groupA.MemberIDs)
	}
	groups, err := repo.ListGroups(ctx, orgA.ID)
	if err != nil {
		t.Fatalf("list groups: %v", err)
	}
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups in org A, got %d", len(groups))
	}

	allocationA1, err := repo.CreateAllocation(ctx, domain.Allocation{OrganisationID: orgA.ID, PersonID: personA1.ID, ProjectID: projectA1.ID, Percent: 40})
	if err != nil {
		t.Fatalf("create allocation A1: %v", err)
	}
	allocationA2, err := repo.CreateAllocation(ctx, domain.Allocation{OrganisationID: orgA.ID, PersonID: personA2.ID, ProjectID: projectA2.ID, Percent: 30})
	if err != nil {
		t.Fatalf("create allocation A2: %v", err)
	}
	_, err = repo.CreateAllocation(ctx, domain.Allocation{OrganisationID: orgA.ID, PersonID: personA1.ID, ProjectID: projectA1.ID, Percent: 5})
	if err != nil {
		t.Fatalf("create allocation A3: %v", err)
	}
	groupAllocation, err := repo.CreateAllocation(ctx, domain.Allocation{
		OrganisationID: orgA.ID,
		TargetType:     domain.AllocationTargetGroup,
		TargetID:       groupA.ID,
		ProjectID:      projectA1.ID,
		StartDate:      "2026-01-01",
		EndDate:        "2026-01-31",
		Percent:        10,
	})
	if err != nil {
		t.Fatalf("create group allocation: %v", err)
	}

	allocationA1.Percent = 45
	allocationA1, err = repo.UpdateAllocation(ctx, allocationA1)
	if err != nil {
		t.Fatalf("update allocation: %v", err)
	}
	if allocationA1.Percent != 45 {
		t.Fatalf("expected allocation update")
	}
	allocationRead, err := repo.GetAllocation(ctx, orgA.ID, allocationA1.ID)
	if err != nil {
		t.Fatalf("get allocation: %v", err)
	}
	if allocationRead.Percent != 45 {
		t.Fatalf("unexpected allocation read: %+v", allocationRead)
	}

	holiday, err := repo.CreateOrgHoliday(ctx, domain.OrgHoliday{OrganisationID: orgA.ID, Date: "2026-01-01", Hours: 8})
	if err != nil {
		t.Fatalf("create holiday: %v", err)
	}
	_, err = repo.CreateOrgHoliday(ctx, domain.OrgHoliday{OrganisationID: orgA.ID, Date: "2026-01-01", Hours: 4})
	if err != nil {
		t.Fatalf("create second holiday: %v", err)
	}
	groupUnavailability, err := repo.CreateGroupUnavailability(ctx, domain.GroupUnavailability{OrganisationID: orgA.ID, GroupID: groupA.ID, Date: "2026-01-03", Hours: 4})
	if err != nil {
		t.Fatalf("create group unavailability: %v", err)
	}
	_, err = repo.CreateGroupUnavailability(ctx, domain.GroupUnavailability{OrganisationID: orgA.ID, GroupID: groupA.ID, Date: "2026-01-03", Hours: 1})
	if err != nil {
		t.Fatalf("create second group unavailability: %v", err)
	}
	personUnavailability, err := repo.CreatePersonUnavailability(ctx, domain.PersonUnavailability{OrganisationID: orgA.ID, PersonID: personA1.ID, Date: "2026-01-04", Hours: 2})
	if err != nil {
		t.Fatalf("create person unavailability: %v", err)
	}
	_, err = repo.CreatePersonUnavailability(ctx, domain.PersonUnavailability{OrganisationID: orgA.ID, PersonID: personA1.ID, Date: "2026-01-04", Hours: 1})
	if err != nil {
		t.Fatalf("create second person unavailability: %v", err)
	}
	personUnavailabilityScoped, err := repo.CreatePersonUnavailabilityWithDailyLimit(ctx, domain.PersonUnavailability{OrganisationID: orgA.ID, PersonID: personA2.ID, Date: "2026-01-05", Hours: 1}, 8)
	if err != nil {
		t.Fatalf("create scoped person unavailability: %v", err)
	}
	if _, err := repo.CreatePersonUnavailabilityWithDailyLimit(ctx, domain.PersonUnavailability{OrganisationID: orgA.ID, PersonID: personA2.ID, Date: "2026-01-05", Hours: 8}, 8); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected scoped person unavailability daily cap validation failure, got %v", err)
	}
	allocations, err := repo.ListAllocations(ctx, orgA.ID)
	if err != nil {
		t.Fatalf("list allocations: %v", err)
	}
	if len(allocations) != 4 {
		t.Fatalf("expected 4 allocations, got %d", len(allocations))
	}
	if allocations[0].TargetType == "" || allocations[0].TargetID == "" {
		t.Fatalf("expected allocations to carry normalized targets, got %+v", allocations[0])
	}

	holidays, err := repo.ListOrgHolidays(ctx, orgA.ID)
	if err != nil {
		t.Fatalf("list holidays: %v", err)
	}
	if len(holidays) != 2 {
		t.Fatalf("expected 2 holidays, got %d", len(holidays))
	}
	groupUnavailableEntries, err := repo.ListGroupUnavailability(ctx, orgA.ID)
	if err != nil {
		t.Fatalf("list group unavailability: %v", err)
	}
	if len(groupUnavailableEntries) != 2 {
		t.Fatalf("expected 2 group unavailability entries, got %d", len(groupUnavailableEntries))
	}
	personUnavailableEntries, err := repo.ListPersonUnavailability(ctx, orgA.ID)
	if err != nil {
		t.Fatalf("list person unavailability: %v", err)
	}
	if len(personUnavailableEntries) != 3 {
		t.Fatalf("expected 3 person unavailability entries, got %d", len(personUnavailableEntries))
	}
	personUnavailableForA1, err := repo.ListPersonUnavailabilityByPerson(ctx, orgA.ID, personA1.ID)
	if err != nil {
		t.Fatalf("list person unavailability by person: %v", err)
	}
	if len(personUnavailableForA1) != 2 {
		t.Fatalf("expected 2 person unavailability entries for person A1, got %d", len(personUnavailableForA1))
	}
	personUnavailableForA1Date, err := repo.ListPersonUnavailabilityByPersonAndDate(ctx, orgA.ID, personA1.ID, "2026-01-04")
	if err != nil {
		t.Fatalf("list person unavailability by person and date: %v", err)
	}
	if len(personUnavailableForA1Date) != 2 {
		t.Fatalf("expected 2 person unavailability entries for person A1 on date, got %d", len(personUnavailableForA1Date))
	}

	if err := repo.DeleteAllocation(ctx, orgA.ID, allocationA2.ID); err != nil {
		t.Fatalf("delete allocation A2: %v", err)
	}
	if err := repo.DeleteGroupUnavailability(ctx, orgA.ID, groupUnavailability.ID); err != nil {
		t.Fatalf("delete group unavailability: %v", err)
	}
	if err := repo.DeletePersonUnavailabilityByPerson(ctx, orgA.ID, personA1.ID, personUnavailabilityScoped.ID); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected person-scoped delete forbidden for mismatched person, got %v", err)
	}
	if err := repo.DeletePersonUnavailabilityByPerson(ctx, orgA.ID, personA2.ID, personUnavailabilityScoped.ID); err != nil {
		t.Fatalf("delete scoped person unavailability: %v", err)
	}
	if err := repo.DeletePersonUnavailability(ctx, orgA.ID, personUnavailability.ID); err != nil {
		t.Fatalf("delete person unavailability: %v", err)
	}
	if err := repo.DeleteOrgHoliday(ctx, orgA.ID, holiday.ID); err != nil {
		t.Fatalf("delete holiday: %v", err)
	}

	if err := repo.DeletePerson(ctx, orgA.ID, personA1.ID); err != nil {
		t.Fatalf("delete person A1: %v", err)
	}
	groupAfterDelete, err := repo.GetGroup(ctx, orgA.ID, groupA.ID)
	if err != nil {
		t.Fatalf("get group after person delete: %v", err)
	}
	if len(groupAfterDelete.MemberIDs) != 1 || groupAfterDelete.MemberIDs[0] != personA2.ID {
		t.Fatalf("expected remaining member Bob, got %v", groupAfterDelete.MemberIDs)
	}

	projectWithAllocation, err := repo.CreateProject(ctx, domain.Project{OrganisationID: orgA.ID, Name: "Project With Allocation"})
	if err != nil {
		t.Fatalf("create project with allocation: %v", err)
	}
	_, err = repo.CreateAllocation(ctx, domain.Allocation{OrganisationID: orgA.ID, PersonID: personA2.ID, ProjectID: projectWithAllocation.ID, Percent: 20})
	if err != nil {
		t.Fatalf("create project allocation: %v", err)
	}
	if err := repo.DeleteProject(ctx, orgA.ID, projectWithAllocation.ID); err != nil {
		t.Fatalf("delete project with allocations: %v", err)
	}
	if err := repo.DeleteProject(ctx, orgA.ID, projectA2.ID); err != nil {
		t.Fatalf("delete project A2: %v", err)
	}
	if err := repo.DeleteGroup(ctx, orgA.ID, groupA.ID); err != nil {
		t.Fatalf("delete group: %v", err)
	}
	if _, err := repo.GetAllocation(ctx, orgA.ID, groupAllocation.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected group allocation to be deleted with group, got %v", err)
	}

	if err := repo.DeleteOrganisation(ctx, orgA.ID); err != nil {
		t.Fatalf("delete organisation A: %v", err)
	}

	personsLeft, err := repo.ListPersons(ctx, orgA.ID)
	if err != nil {
		t.Fatalf("list persons after delete org: %v", err)
	}
	if len(personsLeft) != 0 {
		t.Fatalf("expected no persons in org A, got %d", len(personsLeft))
	}

	orgBFromRepo, err := repo.GetOrganisation(ctx, orgB.ID)
	if err != nil {
		t.Fatalf("expected org B to remain: %v", err)
	}
	if orgBFromRepo.ID != orgB.ID {
		t.Fatalf("unexpected org B id: %s", orgBFromRepo.ID)
	}

	reloaded, err := NewFileRepository(repo.path)
	if err != nil {
		t.Fatalf("reload repository: %v", err)
	}
	remaining, err := reloaded.ListOrganisations(ctx)
	if err != nil {
		t.Fatalf("list organisations in reloaded repo: %v", err)
	}
	if len(remaining) != 1 || remaining[0].ID != orgB.ID {
		t.Fatalf("expected only org B after reload, got %+v", remaining)
	}
}

func TestFileRepositoryNotFoundCases(t *testing.T) {
	ctx := context.Background()
	repo, err := NewFileRepository(filepath.Join(t.TempDir(), "repo.json"))
	if err != nil {
		t.Fatalf("new repo: %v", err)
	}

	if _, err := repo.GetOrganisation(ctx, "missing"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected not found for organisation, got %v", err)
	}
	if err := repo.DeleteOrganisation(ctx, "missing"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected not found for delete organisation, got %v", err)
	}
	if _, err := repo.GetProject(ctx, "org", "missing"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected not found for project, got %v", err)
	}
	if err := repo.DeletePerson(ctx, "org", "missing"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected not found for person delete, got %v", err)
	}
	if err := repo.DeleteGroup(ctx, "org", "missing"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected not found for group delete, got %v", err)
	}
	if err := repo.DeleteAllocation(ctx, "org", "missing"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected not found for allocation delete, got %v", err)
	}
	if err := repo.DeleteOrgHoliday(ctx, "org", "missing"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected not found for holiday delete, got %v", err)
	}
	if err := repo.DeleteGroupUnavailability(ctx, "org", "missing"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected not found for group unavailability delete, got %v", err)
	}
	if err := repo.DeletePersonUnavailability(ctx, "org", "missing"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected not found for person unavailability delete, got %v", err)
	}
	if err := repo.DeletePersonUnavailabilityByPerson(ctx, "org", "person", "missing"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected not found for person-scoped person unavailability delete, got %v", err)
	}
}

func TestFileRepositoryNormalizesLegacyAllocationTargets(t *testing.T) {
	ctx := context.Background()
	state := `{
  "organisations": {
    "org_1": {
      "id": "org_1",
      "name": "Org One",
      "hours_per_day": 8,
      "hours_per_week": 40,
      "hours_per_year": 2080
    }
  },
  "allocations": {
    "allocation_1": {
      "id": "allocation_1",
      "organisation_id": "org_1",
      "person_id": "person_1",
      "project_id": "project_1",
      "percent": 25
    }
  }
}`

	path := filepath.Join(t.TempDir(), "legacy-allocations.json")
	if err := os.WriteFile(path, []byte(state), 0o644); err != nil {
		t.Fatalf("write legacy state: %v", err)
	}

	repo, err := NewFileRepository(path)
	if err != nil {
		t.Fatalf("open legacy state repository: %v", err)
	}

	allocations, err := repo.ListAllocations(ctx, "org_1")
	if err != nil {
		t.Fatalf("list allocations: %v", err)
	}
	if len(allocations) != 1 {
		t.Fatalf("expected one allocation, got %d", len(allocations))
	}
	if allocations[0].TargetType != domain.AllocationTargetPerson || allocations[0].TargetID != "person_1" {
		t.Fatalf("expected legacy allocation normalization, got %+v", allocations[0])
	}
}

func TestFileRepositoryLoadAndDefaultPathBranches(t *testing.T) {
	ctx := context.Background()

	invalidFilePath := filepath.Join(t.TempDir(), "invalid.json")
	if err := os.WriteFile(invalidFilePath, []byte("{bad json"), 0o644); err != nil {
		t.Fatalf("write invalid file: %v", err)
	}
	if _, err := NewFileRepository(invalidFilePath); err == nil {
		t.Fatal("expected decode error for invalid repository file")
	}

	emptyStateFile := filepath.Join(t.TempDir(), "empty-state.json")
	if err := os.WriteFile(emptyStateFile, []byte("{}"), 0o644); err != nil {
		t.Fatalf("write empty state file: %v", err)
	}
	repo, err := NewFileRepository(emptyStateFile)
	if err != nil {
		t.Fatalf("open empty state repository: %v", err)
	}
	if _, err := repo.ListOrganisations(ctx); err != nil {
		t.Fatalf("list organisations from empty state: %v", err)
	}

	tempDir := t.TempDir()
	repoDefault, err := NewFileRepository(filepath.Join(tempDir, "plato_runtime_data.json"))
	if err != nil {
		t.Fatalf("new repository with default path: %v", err)
	}
	if _, err := repoDefault.CreateOrganisation(ctx, domain.Organisation{Name: "Default Path Org", HoursPerDay: 8, HoursPerWeek: 40, HoursPerYear: 2080}); err != nil {
		t.Fatalf("create org in default path repo: %v", err)
	}

	if runtime.GOOS != "windows" {
		if _, err := NewFileRepository(filepath.Join(os.DevNull, "repo.json")); err == nil {
			t.Fatal("expected path error for unwritable directory")
		}
	}
}

func TestPersistenceHelperBranches(t *testing.T) {
	repo := &FileRepository{}
	repo.ensureMapsLocked()
	if repo.state.Organisations == nil || repo.state.Persons == nil || repo.state.Projects == nil || repo.state.Groups == nil {
		t.Fatal("expected maps to be initialized")
	}
	if repo.state.Allocations == nil || repo.state.OrgHolidays == nil || repo.state.GroupUnavailability == nil || repo.state.PersonUnavailability == nil {
		t.Fatal("expected all state maps to be initialized")
	}

	baseDir := t.TempDir()
	blockerPath := filepath.Join(baseDir, "blocked")
	if err := os.WriteFile(blockerPath, []byte("x"), 0o644); err != nil {
		t.Fatalf("create blocker file: %v", err)
	}
	repo.path = filepath.Join(blockerPath, "repo.json")
	if err := repo.persistLocked(); err == nil {
		t.Fatal("expected persist error when parent path is a file")
	}

	renameFailureDir := filepath.Join(baseDir, "rename-failure-target")
	if err := os.Mkdir(renameFailureDir, 0o755); err != nil {
		t.Fatalf("create rename failure target directory: %v", err)
	}
	repo.path = renameFailureDir
	if err := repo.persistLocked(); err == nil {
		t.Fatal("expected persist error when rename target is a directory")
	}
	if _, err := os.Stat(renameFailureDir + ".tmp"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected temp file cleanup after rename failure, got stat err: %v", err)
	}
}

func TestFileRepositoryRollsBackStateOnPersistFailure(t *testing.T) {
	ctx := context.Background()
	repo, err := NewFileRepository(filepath.Join(t.TempDir(), "rollback-state.json"))
	if err != nil {
		t.Fatalf("create repository: %v", err)
	}

	initial, err := repo.CreateOrganisation(ctx, domain.Organisation{Name: "Persisted Org", HoursPerDay: 8, HoursPerWeek: 40, HoursPerYear: 2080})
	if err != nil {
		t.Fatalf("create initial org: %v", err)
	}

	renameFailureTarget := filepath.Join(t.TempDir(), "rename-target-dir")
	if err := os.Mkdir(renameFailureTarget, 0o755); err != nil {
		t.Fatalf("create rename target directory: %v", err)
	}
	repo.path = renameFailureTarget

	if _, err := repo.CreateOrganisation(ctx, domain.Organisation{Name: "Should Rollback", HoursPerDay: 8, HoursPerWeek: 40, HoursPerYear: 2080}); err == nil {
		t.Fatal("expected create organisation to fail when persist cannot rename to directory path")
	}

	organisations, err := repo.ListOrganisations(ctx)
	if err != nil {
		t.Fatalf("list organisations after failed persist: %v", err)
	}
	if len(organisations) != 1 || organisations[0].ID != initial.ID {
		t.Fatalf("expected state rollback to keep only initial org, got %+v", organisations)
	}
}

func TestSortingHelpers(t *testing.T) {
	organisations := []domain.Organisation{
		{ID: "org_2", Name: "Z"},
		{ID: "org_1", Name: "A"},
		{ID: "org_3", Name: "A"},
	}
	sortedOrganisations(organisations)
	if organisations[0].ID != "org_1" || organisations[1].ID != "org_3" {
		t.Fatalf("unexpected organisation sort order: %+v", organisations)
	}

	persons := []domain.Person{{ID: "person_2", Name: "Bob"}, {ID: "person_1", Name: "Bob"}, {ID: "person_3", Name: "Alice"}}
	sortedPersons(persons)
	if persons[0].ID != "person_3" || persons[1].ID != "person_1" {
		t.Fatalf("unexpected person sort order: %+v", persons)
	}

	projects := []domain.Project{{ID: "project_2", Name: "Core"}, {ID: "project_1", Name: "Core"}, {ID: "project_3", Name: "Alpha"}}
	sortedProjects(projects)
	if projects[0].ID != "project_3" || projects[1].ID != "project_1" {
		t.Fatalf("unexpected project sort order: %+v", projects)
	}

	groups := []domain.Group{{ID: "group_2", Name: "Team"}, {ID: "group_1", Name: "Team"}, {ID: "group_3", Name: "Alpha"}}
	sortedGroups(groups)
	if groups[0].ID != "group_3" || groups[1].ID != "group_1" {
		t.Fatalf("unexpected group sort order: %+v", groups)
	}

	allocations := []domain.Allocation{{ID: "a3", PersonID: "p1", ProjectID: "pr1"}, {ID: "a1", PersonID: "p1", ProjectID: "pr1"}, {ID: "a2", PersonID: "p2", ProjectID: "pr2"}}
	sortedAllocations(allocations)
	if allocations[0].ID != "a1" || allocations[1].ID != "a3" {
		t.Fatalf("unexpected allocation sort order: %+v", allocations)
	}

	holidays := []domain.OrgHoliday{{ID: "h2", Date: "2026-01-01"}, {ID: "h1", Date: "2026-01-01"}, {ID: "h3", Date: "2026-01-02"}}
	sortedOrgHolidays(holidays)
	if holidays[0].ID != "h1" || holidays[1].ID != "h2" {
		t.Fatalf("unexpected holiday sort order: %+v", holidays)
	}

	groupUnavailable := []domain.GroupUnavailability{{ID: "gu2", Date: "2026-01-01"}, {ID: "gu1", Date: "2026-01-01"}, {ID: "gu3", Date: "2026-01-03"}}
	sortedGroupUnavailability(groupUnavailable)
	if groupUnavailable[0].ID != "gu1" || groupUnavailable[1].ID != "gu2" {
		t.Fatalf("unexpected group unavailability sort order: %+v", groupUnavailable)
	}

	personUnavailable := []domain.PersonUnavailability{{ID: "pu2", Date: "2026-01-01"}, {ID: "pu1", Date: "2026-01-01"}, {ID: "pu3", Date: "2026-01-03"}}
	sortedPersonUnavailability(personUnavailable)
	if personUnavailable[0].ID != "pu1" || personUnavailable[1].ID != "pu2" {
		t.Fatalf("unexpected person unavailability sort order: %+v", personUnavailable)
	}
}

func TestFileRepositoryClose(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "repo.json")

	repo, err := NewFileRepository(path)
	if err != nil {
		t.Fatalf("create repository: %v", err)
	}

	created, err := repo.CreateOrganisation(ctx, domain.Organisation{
		Name:         "Close Persisted Org",
		HoursPerDay:  8,
		HoursPerWeek: 40,
		HoursPerYear: 2080,
	})
	if err != nil {
		t.Fatalf("create organisation: %v", err)
	}

	if err := repo.Close(); err != nil {
		t.Fatalf("close repository: %v", err)
	}

	if err := repo.Close(); err != nil {
		t.Fatalf("close repository second time: %v", err)
	}

	reopened, err := NewFileRepository(path)
	if err != nil {
		t.Fatalf("reopen repository: %v", err)
	}
	organisations, err := reopened.ListOrganisations(ctx)
	if err != nil {
		t.Fatalf("list organisations after reopen: %v", err)
	}
	if len(organisations) != 1 || organisations[0].ID != created.ID {
		t.Fatalf("expected persisted organisation after close, got %+v", organisations)
	}
}

func TestFileRepositoryContextCancellation(t *testing.T) {
	repo, err := NewFileRepository(filepath.Join(t.TempDir(), "context-cancel-repo.json"))
	if err != nil {
		t.Fatalf("create repository: %v", err)
	}

	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := repo.ListOrganisations(cancelledCtx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled from list organisations, got %v", err)
	}
	if _, err := repo.CreateOrganisation(cancelledCtx, domain.Organisation{
		Name:         "Canceled Org",
		HoursPerDay:  8,
		HoursPerWeek: 40,
		HoursPerYear: 2080,
	}); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled from create organisation, got %v", err)
	}

	organisations, err := repo.ListOrganisations(context.Background())
	if err != nil {
		t.Fatalf("list organisations after canceled create: %v", err)
	}
	if len(organisations) != 0 {
		t.Fatalf("expected no organisations after canceled create, got %+v", organisations)
	}

	created, err := repo.CreateOrganisation(context.Background(), domain.Organisation{
		Name:         "Active Org",
		HoursPerDay:  8,
		HoursPerWeek: 40,
		HoursPerYear: 2080,
	})
	if err != nil {
		t.Fatalf("create organisation: %v", err)
	}

	if err := repo.DeleteOrganisation(cancelledCtx, created.ID); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled from delete organisation, got %v", err)
	}

	if _, err := repo.GetOrganisation(context.Background(), created.ID); err != nil {
		t.Fatalf("expected organisation to remain after canceled delete, got %v", err)
	}
}

func TestFileRepositoryCancelledContextAcrossMethods(t *testing.T) {
	repo, err := NewFileRepository(filepath.Join(t.TempDir(), "context-cancel-all-methods.json"))
	if err != nil {
		t.Fatalf("create repository: %v", err)
	}

	backgroundCtx := context.Background()
	organisation, err := repo.CreateOrganisation(backgroundCtx, domain.Organisation{
		Name:         "Org",
		HoursPerDay:  8,
		HoursPerWeek: 40,
		HoursPerYear: 2080,
	})
	if err != nil {
		t.Fatalf("create organisation: %v", err)
	}
	person, err := repo.CreatePerson(backgroundCtx, domain.Person{
		OrganisationID: organisation.ID,
		Name:           "Person",
		EmploymentPct:  100,
	})
	if err != nil {
		t.Fatalf("create person: %v", err)
	}
	project, err := repo.CreateProject(backgroundCtx, domain.Project{
		OrganisationID: organisation.ID,
		Name:           "Project",
	})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	group, err := repo.CreateGroup(backgroundCtx, domain.Group{
		OrganisationID: organisation.ID,
		Name:           "Group",
		MemberIDs:      []string{person.ID},
	})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	allocation, err := repo.CreateAllocation(backgroundCtx, domain.Allocation{
		OrganisationID: organisation.ID,
		TargetType:     domain.AllocationTargetPerson,
		TargetID:       person.ID,
		ProjectID:      project.ID,
		StartDate:      "2026-01-01",
		EndDate:        "2026-01-02",
		Percent:        25,
	})
	if err != nil {
		t.Fatalf("create allocation: %v", err)
	}
	holiday, err := repo.CreateOrgHoliday(backgroundCtx, domain.OrgHoliday{
		OrganisationID: organisation.ID,
		Date:           "2026-01-01",
		Hours:          8,
	})
	if err != nil {
		t.Fatalf("create holiday: %v", err)
	}
	groupUnavailable, err := repo.CreateGroupUnavailability(backgroundCtx, domain.GroupUnavailability{
		OrganisationID: organisation.ID,
		GroupID:        group.ID,
		Date:           "2026-01-02",
		Hours:          4,
	})
	if err != nil {
		t.Fatalf("create group unavailability: %v", err)
	}
	personUnavailable, err := repo.CreatePersonUnavailability(backgroundCtx, domain.PersonUnavailability{
		OrganisationID: organisation.ID,
		PersonID:       person.ID,
		Date:           "2026-01-03",
		Hours:          2,
	})
	if err != nil {
		t.Fatalf("create person unavailability: %v", err)
	}

	cancelledCtx, cancel := context.WithCancel(backgroundCtx)
	cancel()
	expectCanceled := func(err error) {
		t.Helper()
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context canceled error, got %v", err)
		}
	}

	_, err = repo.ListOrganisations(cancelledCtx)
	expectCanceled(err)
	_, err = repo.GetOrganisation(cancelledCtx, organisation.ID)
	expectCanceled(err)
	_, err = repo.CreateOrganisation(cancelledCtx, domain.Organisation{})
	expectCanceled(err)
	_, err = repo.UpdateOrganisation(cancelledCtx, organisation)
	expectCanceled(err)
	err = repo.DeleteOrganisation(cancelledCtx, organisation.ID)
	expectCanceled(err)

	_, err = repo.ListPersons(cancelledCtx, organisation.ID)
	expectCanceled(err)
	_, err = repo.GetPerson(cancelledCtx, organisation.ID, person.ID)
	expectCanceled(err)
	_, err = repo.CreatePerson(cancelledCtx, person)
	expectCanceled(err)
	_, err = repo.UpdatePerson(cancelledCtx, person)
	expectCanceled(err)
	err = repo.DeletePerson(cancelledCtx, organisation.ID, person.ID)
	expectCanceled(err)

	_, err = repo.ListProjects(cancelledCtx, organisation.ID)
	expectCanceled(err)
	_, err = repo.GetProject(cancelledCtx, organisation.ID, project.ID)
	expectCanceled(err)
	_, err = repo.CreateProject(cancelledCtx, project)
	expectCanceled(err)
	_, err = repo.UpdateProject(cancelledCtx, project)
	expectCanceled(err)
	err = repo.DeleteProject(cancelledCtx, organisation.ID, project.ID)
	expectCanceled(err)

	_, err = repo.ListGroups(cancelledCtx, organisation.ID)
	expectCanceled(err)
	_, err = repo.GetGroup(cancelledCtx, organisation.ID, group.ID)
	expectCanceled(err)
	_, err = repo.CreateGroup(cancelledCtx, group)
	expectCanceled(err)
	_, err = repo.UpdateGroup(cancelledCtx, group)
	expectCanceled(err)
	err = repo.DeleteGroup(cancelledCtx, organisation.ID, group.ID)
	expectCanceled(err)

	_, err = repo.ListAllocations(cancelledCtx, organisation.ID)
	expectCanceled(err)
	_, err = repo.GetAllocation(cancelledCtx, organisation.ID, allocation.ID)
	expectCanceled(err)
	_, err = repo.CreateAllocation(cancelledCtx, allocation)
	expectCanceled(err)
	_, err = repo.UpdateAllocation(cancelledCtx, allocation)
	expectCanceled(err)
	err = repo.DeleteAllocation(cancelledCtx, organisation.ID, allocation.ID)
	expectCanceled(err)

	_, err = repo.ListOrgHolidays(cancelledCtx, organisation.ID)
	expectCanceled(err)
	_, err = repo.CreateOrgHoliday(cancelledCtx, holiday)
	expectCanceled(err)
	err = repo.DeleteOrgHoliday(cancelledCtx, organisation.ID, holiday.ID)
	expectCanceled(err)

	_, err = repo.ListGroupUnavailability(cancelledCtx, organisation.ID)
	expectCanceled(err)
	_, err = repo.CreateGroupUnavailability(cancelledCtx, groupUnavailable)
	expectCanceled(err)
	err = repo.DeleteGroupUnavailability(cancelledCtx, organisation.ID, groupUnavailable.ID)
	expectCanceled(err)

	_, err = repo.ListPersonUnavailability(cancelledCtx, organisation.ID)
	expectCanceled(err)
	_, err = repo.ListPersonUnavailabilityByPerson(cancelledCtx, organisation.ID, person.ID)
	expectCanceled(err)
	_, err = repo.ListPersonUnavailabilityByPersonAndDate(cancelledCtx, organisation.ID, person.ID, personUnavailable.Date)
	expectCanceled(err)
	_, err = repo.CreatePersonUnavailability(cancelledCtx, personUnavailable)
	expectCanceled(err)
	_, err = repo.CreatePersonUnavailabilityWithDailyLimit(cancelledCtx, personUnavailable, 8)
	expectCanceled(err)
	err = repo.DeletePersonUnavailability(cancelledCtx, organisation.ID, personUnavailable.ID)
	expectCanceled(err)
	err = repo.DeletePersonUnavailabilityByPerson(cancelledCtx, organisation.ID, person.ID, personUnavailable.ID)
	expectCanceled(err)
}
