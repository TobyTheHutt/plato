package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"plato/backend/internal/adapters/auth"
	"plato/backend/internal/adapters/impexp"
	"plato/backend/internal/adapters/persistence"
	"plato/backend/internal/adapters/telemetry"
	"plato/backend/internal/domain"
	"plato/backend/internal/ports"
	"plato/backend/internal/service"
)

func TestHealthz(t *testing.T) {
	router := newTestRouter(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", http.NoBody)

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("unexpected response: %v", body)
	}
}

func TestRBACOrgUserCannotMutate(t *testing.T) {
	router := newTestRouter(t)
	orgID := createOrganisation(t, router, map[string]string{"X-Role": "org_admin"})

	rec := doJSONRequest(t, router, http.MethodPost, "/api/projects", map[string]any{"name": "Hidden"}, map[string]string{"X-Role": "org_user", "X-Org-ID": orgID})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestTenantScopingForPersonAndOrganisation(t *testing.T) {
	router := newTestRouter(t)
	orgA := createOrganisation(t, router, map[string]string{"X-Role": "org_admin"})
	orgB := createOrganisation(t, router, map[string]string{"X-Role": "org_admin"})

	personID := createPerson(t, router, orgA, "Alice", 100)

	resPerson := doJSONRequest(t, router, http.MethodGet, "/api/persons/"+personID, nil, map[string]string{"X-Role": "org_admin", "X-Org-ID": orgB})
	if resPerson.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for cross-tenant person, got %d body=%s", resPerson.Code, resPerson.Body.String())
	}

	resOrg := doJSONRequest(t, router, http.MethodGet, "/api/organisations/"+orgA, nil, map[string]string{"X-Role": "org_admin", "X-Org-ID": orgB})
	if resOrg.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for cross-tenant organisation, got %d body=%s", resOrg.Code, resOrg.Body.String())
	}

	resHoliday := doJSONRequest(t, router, http.MethodPost, "/api/organisations/"+orgA+"/holidays", map[string]any{"date": "2026-01-01", "hours": 8}, map[string]string{"X-Role": "org_admin", "X-Org-ID": orgB})
	if resHoliday.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for cross-tenant holiday path, got %d body=%s", resHoliday.Code, resHoliday.Body.String())
	}
}

func TestAllocationValidationAndReportEndpoint(t *testing.T) {
	router := newTestRouter(t)
	orgID := createOrganisation(t, router, map[string]string{"X-Role": "org_admin"})
	personID := createPerson(t, router, orgID, "Bob", 50)
	projectID := createProject(t, router, orgID, "Phoenix")

	firstAllocation := doJSONRequest(t, router, http.MethodPost, "/api/allocations", personAllocationPayload(personID, projectID, 40), map[string]string{"X-Role": "org_admin", "X-Org-ID": orgID})
	if firstAllocation.Code != http.StatusCreated {
		t.Fatalf("expected first allocation to pass, got %d body=%s", firstAllocation.Code, firstAllocation.Body.String())
	}

	overEmploymentAllocation := doJSONRequest(t, router, http.MethodPost, "/api/allocations", personAllocationPayload(personID, projectID, 20), map[string]string{"X-Role": "org_admin", "X-Org-ID": orgID})
	if overEmploymentAllocation.Code != http.StatusCreated {
		t.Fatalf("expected over-employment allocation to pass, got %d body=%s", overEmploymentAllocation.Code, overEmploymentAllocation.Body.String())
	}

	overTheoreticalLimitAllocation := doJSONRequest(t, router, http.MethodPost, "/api/allocations", personAllocationPayload(personID, projectID, 250), map[string]string{"X-Role": "org_admin", "X-Org-ID": orgID})
	if overTheoreticalLimitAllocation.Code != http.StatusBadRequest {
		t.Fatalf("expected theoretical limit allocation error, got %d body=%s", overTheoreticalLimitAllocation.Code, overTheoreticalLimitAllocation.Body.String())
	}
	var validationBody map[string]string
	if err := json.Unmarshal(overTheoreticalLimitAllocation.Body.Bytes(), &validationBody); err != nil {
		t.Fatalf("decode validation response: %v", err)
	}
	if validationBody["error"] != "allocation exceeds 24 hours/day theoretical limit" {
		t.Fatalf("unexpected theoretical limit validation body: %+v", validationBody)
	}

	report := doJSONRequest(t, router, http.MethodPost, "/api/reports/availability-load", map[string]any{"scope": "organisation", "from_date": "2026-01-01", "to_date": "2026-01-01", "granularity": "day"}, map[string]string{"X-Role": "org_user", "X-Org-ID": orgID})
	if report.Code != http.StatusOK {
		t.Fatalf("expected report success, got %d body=%s", report.Code, report.Body.String())
	}

	var payload struct {
		Buckets []domain.ReportBucket `json:"buckets"`
	}
	if err := json.Unmarshal(report.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode report: %v", err)
	}
	if len(payload.Buckets) != 1 {
		t.Fatalf("expected one report bucket, got %d", len(payload.Buckets))
	}
	if payload.Buckets[0].LoadHours != 4.8 {
		t.Fatalf("unexpected load hours: %v", payload.Buckets[0].LoadHours)
	}
}

func TestMethodAndJSONErrors(t *testing.T) {
	router := newTestRouter(t)

	badMethod := doRawRequest(t, router, http.MethodPatch, "/api/persons", nil, map[string]string{"X-Role": "org_admin"})
	if badMethod.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", badMethod.Code)
	}

	badJSON := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/organisations", bytes.NewBufferString("{"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Role", "org_admin")
	router.ServeHTTP(badJSON, req)
	if badJSON.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad json, got %d", badJSON.Code)
	}
}

func TestDefaultAuthValuesEnableDevFlow(t *testing.T) {
	router := newTestRouter(t)
	rec := doJSONRequest(t, router, http.MethodPost, "/api/organisations", map[string]any{"name": "Org dev", "hours_per_day": 8, "hours_per_week": 40, "hours_per_year": 2080}, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected create organisation with default auth, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAPICloseRunsCleanupOnceAcrossConcurrentCallers(t *testing.T) {
	expected := errors.New("cleanup failed")
	callCount := 0
	var countMu sync.Mutex

	api := &API{
		cleanup: func() error {
			countMu.Lock()
			callCount++
			countMu.Unlock()
			return expected
		},
	}

	const callerCount = 8
	var wg sync.WaitGroup
	errs := make(chan error, callerCount)
	for i := 0; i < callerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- api.Close()
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		if !errors.Is(err, expected) {
			t.Fatalf("expected close error %v, got %v", expected, err)
		}
	}

	countMu.Lock()
	defer countMu.Unlock()
	if callCount != 1 {
		t.Fatalf("expected cleanup to run once, got %d calls", callCount)
	}
}

func TestEndToEndCRUDRoutes(t *testing.T) {
	router := newTestRouter(t)
	state := setupEndToEndCRUDRoutesState(t, router)
	validateEndToEndCRUDOrganisationPersonsProjects(t, router, state)
	validateEndToEndCRUDGroupsAllocationsCalendar(t, router, state)
	validateEndToEndCRUDReportsAndDeletion(t, router, state)
}

type endToEndCRUDRoutesState struct {
	orgID                  string
	adminHeaders           map[string]string
	personA                string
	personB                string
	projectA               string
	projectB               string
	groupID                string
	allocationID           string
	holidayID              string
	personUnavailabilityID string
	groupUnavailabilityID  string
}

func setupEndToEndCRUDRoutesState(t *testing.T, router http.Handler) *endToEndCRUDRoutesState {
	t.Helper()
	orgID := createOrganisation(t, router, map[string]string{"X-Role": "org_admin"})
	return &endToEndCRUDRoutesState{
		orgID:        orgID,
		adminHeaders: map[string]string{"X-Role": "org_admin", "X-Org-ID": orgID},
	}
}

func validateEndToEndCRUDOrganisationPersonsProjects(t *testing.T, router http.Handler, state *endToEndCRUDRoutesState) {
	t.Helper()

	organisations := doJSONRequest(t, router, http.MethodGet, "/api/organisations", nil, state.adminHeaders)
	if organisations.Code != http.StatusOK {
		t.Fatalf("expected list organisations success, got %d", organisations.Code)
	}

	updateOrg := doJSONRequest(t, router, http.MethodPut, "/api/organisations/"+state.orgID, map[string]any{
		"name":           "Updated Org",
		"hours_per_day":  7.5,
		"hours_per_week": 37.5,
		"hours_per_year": 1950,
	}, state.adminHeaders)
	if updateOrg.Code != http.StatusOK {
		t.Fatalf("expected update organisation success, got %d body=%s", updateOrg.Code, updateOrg.Body.String())
	}

	state.personA = createPerson(t, router, state.orgID, "Alice", 100)
	state.personB = createPerson(t, router, state.orgID, "Bob", 80)

	getPerson := doJSONRequest(t, router, http.MethodGet, "/api/persons/"+state.personA, nil, state.adminHeaders)
	if getPerson.Code != http.StatusOK {
		t.Fatalf("expected get person success, got %d", getPerson.Code)
	}

	updatePerson := doJSONRequest(t, router, http.MethodPut, "/api/persons/"+state.personB, map[string]any{"name": "Bob Updated", "employment_pct": 75}, state.adminHeaders)
	if updatePerson.Code != http.StatusOK {
		t.Fatalf("expected update person success, got %d body=%s", updatePerson.Code, updatePerson.Body.String())
	}

	listPersons := doJSONRequest(t, router, http.MethodGet, "/api/persons", nil, state.adminHeaders)
	if listPersons.Code != http.StatusOK {
		t.Fatalf("expected list persons success, got %d", listPersons.Code)
	}

	state.projectA = createProject(t, router, state.orgID, "Project A")
	state.projectB = createProject(t, router, state.orgID, "Project B")

	getProject := doJSONRequest(t, router, http.MethodGet, "/api/projects/"+state.projectA, nil, state.adminHeaders)
	if getProject.Code != http.StatusOK {
		t.Fatalf("expected get project success, got %d", getProject.Code)
	}

	updateProject := doJSONRequest(t, router, http.MethodPut, "/api/projects/"+state.projectB, projectPayload("Project B Updated"), state.adminHeaders)
	if updateProject.Code != http.StatusOK {
		t.Fatalf("expected update project success, got %d body=%s", updateProject.Code, updateProject.Body.String())
	}

	listProjects := doJSONRequest(t, router, http.MethodGet, "/api/projects", nil, state.adminHeaders)
	if listProjects.Code != http.StatusOK {
		t.Fatalf("expected list projects success, got %d", listProjects.Code)
	}
}

func validateEndToEndCRUDGroupsAllocationsCalendar(t *testing.T, router http.Handler, state *endToEndCRUDRoutesState) {
	t.Helper()

	createAndVerifyEndToEndGroupRoutes(t, router, state)
	createAndVerifyEndToEndAllocationRoutes(t, router, state)
	createAndVerifyEndToEndCalendarRoutes(t, router, state)
}

func createAndVerifyEndToEndGroupRoutes(t *testing.T, router http.Handler, state *endToEndCRUDRoutesState) {
	t.Helper()

	createGroup := doJSONRequest(t, router, http.MethodPost, "/api/groups", map[string]any{"name": "Team One", "member_ids": []string{state.personA}}, state.adminHeaders)
	if createGroup.Code != http.StatusCreated {
		t.Fatalf("expected create group success, got %d body=%s", createGroup.Code, createGroup.Body.String())
	}
	var group domain.Group
	if err := json.Unmarshal(createGroup.Body.Bytes(), &group); err != nil {
		t.Fatalf("decode group: %v", err)
	}
	state.groupID = group.ID

	getGroup := doJSONRequest(t, router, http.MethodGet, "/api/groups/"+state.groupID, nil, state.adminHeaders)
	if getGroup.Code != http.StatusOK {
		t.Fatalf("expected get group success, got %d", getGroup.Code)
	}
	if code := doJSONRequest(t, router, http.MethodGet, "/api/groups", nil, state.adminHeaders).Code; code != http.StatusOK {
		t.Fatalf("expected list groups success, got %d", code)
	}

	if code := doJSONRequest(t, router, http.MethodPost, "/api/groups/"+state.groupID+"/members", map[string]any{"person_id": state.personB}, state.adminHeaders).Code; code != http.StatusOK {
		t.Fatalf("expected add group member success, got %d", code)
	}
	if code := doJSONRequest(t, router, http.MethodDelete, "/api/groups/"+state.groupID+"/members/"+state.personB, nil, state.adminHeaders).Code; code != http.StatusOK {
		t.Fatalf("expected remove group member success, got %d", code)
	}
}

func createAndVerifyEndToEndAllocationRoutes(t *testing.T, router http.Handler, state *endToEndCRUDRoutesState) {
	t.Helper()

	createAllocation := doJSONRequest(t, router, http.MethodPost, "/api/allocations", personAllocationPayload(state.personA, state.projectA, 50), state.adminHeaders)
	if createAllocation.Code != http.StatusCreated {
		t.Fatalf("expected create allocation success, got %d body=%s", createAllocation.Code, createAllocation.Body.String())
	}
	var allocation domain.Allocation
	if err := json.Unmarshal(createAllocation.Body.Bytes(), &allocation); err != nil {
		t.Fatalf("decode allocation: %v", err)
	}
	state.allocationID = allocation.ID

	if code := doJSONRequest(t, router, http.MethodGet, "/api/allocations/"+state.allocationID, nil, state.adminHeaders).Code; code != http.StatusOK {
		t.Fatalf("expected get allocation success, got %d", code)
	}
	if code := doJSONRequest(t, router, http.MethodPut, "/api/allocations/"+state.allocationID, personAllocationPayload(state.personA, state.projectA, 45), state.adminHeaders).Code; code != http.StatusOK {
		t.Fatalf("expected update allocation success, got %d", code)
	}
	if code := doJSONRequest(t, router, http.MethodGet, "/api/allocations", nil, state.adminHeaders).Code; code != http.StatusOK {
		t.Fatalf("expected list allocations success, got %d", code)
	}
}

func createAndVerifyEndToEndCalendarRoutes(t *testing.T, router http.Handler, state *endToEndCRUDRoutesState) {
	t.Helper()

	createHoliday := doJSONRequest(t, router, http.MethodPost, "/api/organisations/"+state.orgID+"/holidays", map[string]any{"date": "2026-01-01", "hours": 7.5}, state.adminHeaders)
	if createHoliday.Code != http.StatusCreated {
		t.Fatalf("expected create holiday success, got %d body=%s", createHoliday.Code, createHoliday.Body.String())
	}
	var holiday domain.OrgHoliday
	if err := json.Unmarshal(createHoliday.Body.Bytes(), &holiday); err != nil {
		t.Fatalf("decode holiday: %v", err)
	}
	state.holidayID = holiday.ID

	if code := doJSONRequest(t, router, http.MethodGet, "/api/organisations/"+state.orgID+"/holidays", nil, state.adminHeaders).Code; code != http.StatusOK {
		t.Fatalf("expected list holidays success, got %d", code)
	}

	createPersonUnavailable := doJSONRequest(t, router, http.MethodPost, "/api/persons/"+state.personA+"/unavailability", map[string]any{"date": "2026-01-02", "hours": 2}, state.adminHeaders)
	if createPersonUnavailable.Code != http.StatusCreated {
		t.Fatalf("expected create person unavailability success, got %d body=%s", createPersonUnavailable.Code, createPersonUnavailable.Body.String())
	}
	var personUnavailable domain.PersonUnavailability
	if err := json.Unmarshal(createPersonUnavailable.Body.Bytes(), &personUnavailable); err != nil {
		t.Fatalf("decode person unavailability: %v", err)
	}
	state.personUnavailabilityID = personUnavailable.ID

	if code := doJSONRequest(t, router, http.MethodGet, "/api/persons/"+state.personA+"/unavailability", nil, state.adminHeaders).Code; code != http.StatusOK {
		t.Fatalf("expected list person unavailability success, got %d", code)
	}

	createGroupUnavailable := doJSONRequest(t, router, http.MethodPost, "/api/groups/"+state.groupID+"/unavailability", map[string]any{"date": "2026-01-03", "hours": 3}, state.adminHeaders)
	if createGroupUnavailable.Code != http.StatusCreated {
		t.Fatalf("expected create group unavailability success, got %d body=%s", createGroupUnavailable.Code, createGroupUnavailable.Body.String())
	}
	var groupUnavailable domain.GroupUnavailability
	if err := json.Unmarshal(createGroupUnavailable.Body.Bytes(), &groupUnavailable); err != nil {
		t.Fatalf("decode group unavailability: %v", err)
	}
	state.groupUnavailabilityID = groupUnavailable.ID

	if code := doJSONRequest(t, router, http.MethodGet, "/api/groups/"+state.groupID+"/unavailability", nil, state.adminHeaders).Code; code != http.StatusOK {
		t.Fatalf("expected list group unavailability success, got %d", code)
	}
}

func validateEndToEndCRUDReportsAndDeletion(t *testing.T, router http.Handler, state *endToEndCRUDRoutesState) {
	t.Helper()

	verifyEndToEndReportResponse(t, router, state)
	verifyEndToEndRouterMetaEndpoints(t, router, state)
	executeEndToEndDeletionFlow(t, router, state)
}

func verifyEndToEndReportResponse(t *testing.T, router http.Handler, state *endToEndCRUDRoutesState) {
	t.Helper()

	report := doJSONRequest(t, router, http.MethodPost, "/api/reports/availability-load", map[string]any{
		"scope":       "project",
		"ids":         []string{state.projectA},
		"from_date":   "2026-01-01",
		"to_date":     "2026-01-03",
		"granularity": "month",
	}, map[string]string{"X-Role": "org_user", "X-Org-ID": state.orgID})
	if report.Code != http.StatusOK {
		t.Fatalf("expected report success, got %d body=%s", report.Code, report.Body.String())
	}

	var reportPayload struct {
		Buckets []domain.ReportBucket `json:"buckets"`
	}
	if err := json.Unmarshal(report.Body.Bytes(), &reportPayload); err != nil {
		t.Fatalf("decode report response: %v", err)
	}
	if len(reportPayload.Buckets) != 1 {
		t.Fatalf("expected one report bucket, got %d", len(reportPayload.Buckets))
	}
	if reportPayload.Buckets[0].ProjectEstimation <= 0 {
		t.Fatalf("expected project estimation in report, got %v", reportPayload.Buckets[0].ProjectEstimation)
	}
	if reportPayload.Buckets[0].ProjectLoadHours <= 0 {
		t.Fatalf("expected project load hours in report, got %v", reportPayload.Buckets[0].ProjectLoadHours)
	}
	if reportPayload.Buckets[0].CompletionPct <= 0 {
		t.Fatalf("expected project completion percent in report, got %v", reportPayload.Buckets[0].CompletionPct)
	}
}

func verifyEndToEndRouterMetaEndpoints(t *testing.T, router http.Handler, state *endToEndCRUDRoutesState) {
	t.Helper()

	if code := doRawRequest(t, router, http.MethodOptions, "/api/persons", nil, nil).Code; code != http.StatusNoContent {
		t.Fatalf("expected options status 204, got %d", code)
	}
	if code := doJSONRequest(t, router, http.MethodGet, "/api/missing", nil, state.adminHeaders).Code; code != http.StatusNotFound {
		t.Fatalf("expected not found status, got %d", code)
	}
	if code := doJSONRequest(t, router, http.MethodGet, "/api/reports/availability-load", nil, state.adminHeaders).Code; code != http.StatusMethodNotAllowed {
		t.Fatalf("expected report wrong method 405, got %d", code)
	}
}

func executeEndToEndDeletionFlow(t *testing.T, router http.Handler, state *endToEndCRUDRoutesState) {
	t.Helper()

	if code := doJSONRequest(t, router, http.MethodDelete, "/api/groups/"+state.groupID+"/unavailability/"+state.groupUnavailabilityID, nil, state.adminHeaders).Code; code != http.StatusNoContent {
		t.Fatalf("expected delete group unavailability success, got %d", code)
	}
	if code := doJSONRequest(t, router, http.MethodDelete, "/api/persons/"+state.personA+"/unavailability/missing", nil, state.adminHeaders).Code; code != http.StatusNotFound {
		t.Fatalf("expected delete person unavailability with missing entry to be not found, got %d", code)
	}
	if code := doJSONRequest(t, router, http.MethodDelete, "/api/persons/"+state.personB+"/unavailability/"+state.personUnavailabilityID, nil, state.adminHeaders).Code; code != http.StatusForbidden {
		t.Fatalf("expected delete person unavailability with mismatched person path to be forbidden, got %d", code)
	}
	if code := doJSONRequest(t, router, http.MethodDelete, "/api/persons/"+state.personA+"/unavailability/"+state.personUnavailabilityID, nil, state.adminHeaders).Code; code != http.StatusNoContent {
		t.Fatalf("expected delete person unavailability success, got %d", code)
	}
	if code := doJSONRequest(t, router, http.MethodDelete, "/api/organisations/"+state.orgID+"/holidays/"+state.holidayID, nil, state.adminHeaders).Code; code != http.StatusNoContent {
		t.Fatalf("expected delete holiday success, got %d", code)
	}
	if code := doJSONRequest(t, router, http.MethodDelete, "/api/allocations/"+state.allocationID, nil, state.adminHeaders).Code; code != http.StatusNoContent {
		t.Fatalf("expected delete allocation success, got %d", code)
	}
	if code := doJSONRequest(t, router, http.MethodDelete, "/api/projects/"+state.projectB, nil, state.adminHeaders).Code; code != http.StatusNoContent {
		t.Fatalf("expected delete project success, got %d", code)
	}
	if code := doJSONRequest(t, router, http.MethodDelete, "/api/projects/"+state.projectA, nil, state.adminHeaders).Code; code != http.StatusNoContent {
		t.Fatalf("expected delete project A success, got %d", code)
	}
	if code := doJSONRequest(t, router, http.MethodDelete, "/api/groups/"+state.groupID, nil, state.adminHeaders).Code; code != http.StatusNoContent {
		t.Fatalf("expected delete group success, got %d", code)
	}
	if code := doJSONRequest(t, router, http.MethodDelete, "/api/persons/"+state.personB, nil, state.adminHeaders).Code; code != http.StatusNoContent {
		t.Fatalf("expected delete person B success, got %d", code)
	}
	if code := doJSONRequest(t, router, http.MethodDelete, "/api/persons/"+state.personA, nil, state.adminHeaders).Code; code != http.StatusNoContent {
		t.Fatalf("expected delete person A success, got %d", code)
	}
}

func TestRouterNewRouterAndAuthFailure(t *testing.T) {
	t.Setenv("DEV_MODE", "true")
	t.Setenv("PLATO_DATA_FILE", filepath.Join(t.TempDir(), "router-data.json"))
	router, err := NewRouterFromEnv()
	if err != nil {
		t.Fatalf("create router: %v", err)
	}
	health := doRawRequest(t, router, http.MethodGet, "/healthz", nil, nil)
	if health.Code != http.StatusOK {
		t.Fatalf("expected health from NewRouter, got %d", health.Code)
	}

	repo, err := persistence.NewFileRepository(filepath.Join(t.TempDir(), "auth-fail-data.json"))
	if err != nil {
		t.Fatalf("create repo: %v", err)
	}
	svc, err := service.New(repo, telemetry.NewNoopTelemetry(), impexp.NewNoopImportExport())
	if err != nil {
		t.Fatalf("create service: %v", err)
	}
	routerWithFailingAuth := NewRouterWithDependencies(failingAuthProvider{}, svc)

	response := doRawRequest(t, routerWithFailingAuth, http.MethodGet, "/api/organisations", nil, nil)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized for failing auth provider, got %d", response.Code)
	}
}

func TestRouterNewRouterProductionModeRequiresJWTSecret(t *testing.T) {
	t.Setenv("PRODUCTION_MODE", "true")
	t.Setenv("PLATO_CORS_ALLOWED_ORIGINS", "https://app.example.com")
	t.Setenv("PLATO_AUTH_JWT_HS256_SIGNING_KEY", "")
	t.Setenv("PLATO_DATA_FILE", filepath.Join(t.TempDir(), "prod-data.json"))

	if _, err := NewRouterFromEnv(); err == nil {
		t.Fatal("expected router creation to fail without production JWT secret")
	}
}

func TestRouterNewRouterProductionModeCORSAllowlistAndAuth(t *testing.T) {
	t.Setenv("PRODUCTION_MODE", "true")
	t.Setenv("PLATO_AUTH_JWT_HS256_SIGNING_KEY", "test-secret")
	t.Setenv("PLATO_CORS_ALLOWED_ORIGINS", "https://app.example.com")
	t.Setenv("PLATO_DATA_FILE", filepath.Join(t.TempDir(), "prod-router-data.json"))

	router, err := NewRouterFromEnv()
	if err != nil {
		t.Fatalf("create production router: %v", err)
	}

	allowlistedOriginResponse := doRawRequest(t, router, http.MethodGet, "/api/organisations", nil, map[string]string{
		"Origin": "https://app.example.com",
	})
	if allowlistedOriginResponse.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized without bearer token, got %d", allowlistedOriginResponse.Code)
	}
	if got := allowlistedOriginResponse.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Fatalf("expected allowlisted origin header, got %q", got)
	}

	blockedOriginResponse := doRawRequest(t, router, http.MethodGet, "/api/organisations", nil, map[string]string{
		"Origin": "https://blocked.example.com",
	})
	if got := blockedOriginResponse.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected blocked origin to be omitted from CORS header, got %q", got)
	}
}

func TestMethodNotAllowedAndInternalErrorBranches(t *testing.T) {
	router := newTestRouter(t)
	orgID := createOrganisation(t, router, map[string]string{"X-Role": "org_admin"})
	adminHeaders := map[string]string{"X-Role": "org_admin", "X-Org-ID": orgID}
	personID := createPerson(t, router, orgID, "Method Tester", 100)
	projectID := createProject(t, router, orgID, "Method Project")

	createGroup := doJSONRequest(t, router, http.MethodPost, "/api/groups", map[string]any{"name": "Method Group", "member_ids": []string{personID}}, adminHeaders)
	if createGroup.Code != http.StatusCreated {
		t.Fatalf("setup group failed: %d body=%s", createGroup.Code, createGroup.Body.String())
	}
	var group domain.Group
	if err := json.Unmarshal(createGroup.Body.Bytes(), &group); err != nil {
		t.Fatalf("decode setup group: %v", err)
	}

	createAllocation := doJSONRequest(t, router, http.MethodPost, "/api/allocations", personAllocationPayload(personID, projectID, 20), adminHeaders)
	if createAllocation.Code != http.StatusCreated {
		t.Fatalf("setup allocation failed: %d body=%s", createAllocation.Code, createAllocation.Body.String())
	}
	var allocation domain.Allocation
	if err := json.Unmarshal(createAllocation.Body.Bytes(), &allocation); err != nil {
		t.Fatalf("decode setup allocation: %v", err)
	}

	hits := []struct {
		method     string
		path       string
		statusCode int
	}{
		{http.MethodPatch, "/api/organisations", http.StatusMethodNotAllowed},
		{http.MethodPost, "/api/organisations/" + orgID, http.StatusMethodNotAllowed},
		{http.MethodPatch, "/api/organisations/" + orgID + "/holidays", http.StatusMethodNotAllowed},
		{http.MethodPatch, "/api/persons", http.StatusMethodNotAllowed},
		{http.MethodPost, "/api/persons/" + personID, http.StatusMethodNotAllowed},
		{http.MethodPatch, "/api/persons/" + personID + "/unavailability", http.StatusMethodNotAllowed},
		{http.MethodPatch, "/api/projects", http.StatusMethodNotAllowed},
		{http.MethodPatch, "/api/projects/" + projectID, http.StatusMethodNotAllowed},
		{http.MethodPatch, "/api/groups", http.StatusMethodNotAllowed},
		{http.MethodPatch, "/api/groups/" + group.ID, http.StatusMethodNotAllowed},
		{http.MethodGet, "/api/groups/" + group.ID + "/members", http.StatusMethodNotAllowed},
		{http.MethodPatch, "/api/groups/" + group.ID + "/unavailability", http.StatusMethodNotAllowed},
		{http.MethodPatch, "/api/allocations", http.StatusMethodNotAllowed},
		{http.MethodPatch, "/api/allocations/" + allocation.ID, http.StatusMethodNotAllowed},
		{http.MethodGet, "/api/reports/availability-load", http.StatusMethodNotAllowed},
	}

	for _, hit := range hits {
		rec := doJSONRequest(t, router, hit.method, hit.path, nil, adminHeaders)
		if rec.Code != hit.statusCode {
			t.Fatalf("expected %d for %s %s got %d body=%s", hit.statusCode, hit.method, hit.path, rec.Code, rec.Body.String())
		}
	}

	notFound := doJSONRequest(t, router, http.MethodDelete, "/api/groups/"+group.ID+"/members/"+personID+"/extra", nil, adminHeaders)
	if notFound.Code != http.StatusNotFound {
		t.Fatalf("expected nested path not found, got %d", notFound.Code)
	}

	repo, err := persistence.NewFileRepository(filepath.Join(t.TempDir(), "error-repo.json"))
	if err != nil {
		t.Fatalf("create error repo: %v", err)
	}
	errSvc, err := service.New(errorRepository{Repository: repo}, telemetry.NewNoopTelemetry(), impexp.NewNoopImportExport())
	if err != nil {
		t.Fatalf("create service: %v", err)
	}
	errRouter := NewRouterWithDependencies(auth.NewDevAuthProvider(), errSvc)
	res := doJSONRequest(t, errRouter, http.MethodGet, "/api/organisations", nil, map[string]string{"X-Role": "org_admin"})
	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 from repository failure, got %d body=%s", res.Code, res.Body.String())
	}
}

func TestInvalidJSONAcrossMutatingRoutes(t *testing.T) {
	router := newTestRouter(t)
	orgID := createOrganisation(t, router, map[string]string{"X-Role": "org_admin"})
	headers := map[string]string{"X-Role": "org_admin", "X-Org-ID": orgID}

	personID := createPerson(t, router, orgID, "Bad JSON Person", 100)
	projectID := createProject(t, router, orgID, "Bad JSON Project")
	groupResp := doJSONRequest(t, router, http.MethodPost, "/api/groups", map[string]any{"name": "Bad JSON Group", "member_ids": []string{personID}}, headers)
	if groupResp.Code != http.StatusCreated {
		t.Fatalf("create group for bad json test failed: %d body=%s", groupResp.Code, groupResp.Body.String())
	}
	var group domain.Group
	if err := json.Unmarshal(groupResp.Body.Bytes(), &group); err != nil {
		t.Fatalf("decode group for bad json test: %v", err)
	}
	allocResp := doJSONRequest(t, router, http.MethodPost, "/api/allocations", personAllocationPayload(personID, projectID, 10), headers)
	if allocResp.Code != http.StatusCreated {
		t.Fatalf("create allocation for bad json test failed: %d body=%s", allocResp.Code, allocResp.Body.String())
	}
	var allocation domain.Allocation
	if err := json.Unmarshal(allocResp.Body.Bytes(), &allocation); err != nil {
		t.Fatalf("decode allocation for bad json test: %v", err)
	}

	paths := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/organisations"},
		{http.MethodPut, "/api/organisations/" + orgID},
		{http.MethodPost, "/api/organisations/" + orgID + "/holidays"},
		{http.MethodPost, "/api/persons"},
		{http.MethodPut, "/api/persons/" + personID},
		{http.MethodPost, "/api/persons/" + personID + "/unavailability"},
		{http.MethodPost, "/api/projects"},
		{http.MethodPut, "/api/projects/" + projectID},
		{http.MethodPost, "/api/groups"},
		{http.MethodPut, "/api/groups/" + group.ID},
		{http.MethodPost, "/api/groups/" + group.ID + "/members"},
		{http.MethodPost, "/api/groups/" + group.ID + "/unavailability"},
		{http.MethodPost, "/api/allocations"},
		{http.MethodPut, "/api/allocations/" + allocation.ID},
		{http.MethodPost, "/api/reports/availability-load"},
	}

	for _, entry := range paths {
		response := doRawRequest(t, router, entry.method, entry.path, []byte("{"), headers)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for invalid json on %s %s got %d body=%s", entry.method, entry.path, response.Code, response.Body.String())
		}
	}
}

func TestResourceNotFoundAndTenantRequiredResponses(t *testing.T) {
	router := newTestRouter(t)
	orgID := createOrganisation(t, router, map[string]string{"X-Role": "org_admin"})
	headers := map[string]string{"X-Role": "org_admin", "X-Org-ID": orgID}

	personID := createPerson(t, router, orgID, "Missing Paths Person", 100)
	projectID := createProject(t, router, orgID, "Missing Paths Project")
	groupResp := doJSONRequest(t, router, http.MethodPost, "/api/groups", map[string]any{"name": "Missing Group", "member_ids": []string{personID}}, headers)
	if groupResp.Code != http.StatusCreated {
		t.Fatalf("setup group failed: %d body=%s", groupResp.Code, groupResp.Body.String())
	}
	allocResp := doJSONRequest(t, router, http.MethodPost, "/api/allocations", personAllocationPayload(personID, projectID, 10), headers)
	if allocResp.Code != http.StatusCreated {
		t.Fatalf("setup allocation failed: %d body=%s", allocResp.Code, allocResp.Body.String())
	}

	missingHits := []struct {
		method string
		path   string
		body   any
	}{
		{http.MethodGet, "/api/persons/missing", nil},
		{http.MethodPut, "/api/persons/missing", map[string]any{"name": "x", "employment_pct": 100}},
		{http.MethodDelete, "/api/persons/missing", nil},
		{http.MethodGet, "/api/projects/missing", nil},
		{http.MethodPut, "/api/projects/missing", projectPayload("x")},
		{http.MethodDelete, "/api/projects/missing", nil},
		{http.MethodGet, "/api/groups/missing", nil},
		{http.MethodPut, "/api/groups/missing", map[string]any{"name": "x", "member_ids": []string{}}},
		{http.MethodDelete, "/api/groups/missing", nil},
		{http.MethodDelete, "/api/groups/missing/members/" + personID, nil},
		{http.MethodGet, "/api/allocations/missing", nil},
		{http.MethodPut, "/api/allocations/missing", personAllocationPayload(personID, projectID, 10)},
		{http.MethodDelete, "/api/allocations/missing", nil},
		{http.MethodDelete, "/api/persons/" + personID + "/unavailability/missing", nil},
	}

	for _, hit := range missingHits {
		rec := doJSONRequest(t, router, hit.method, hit.path, hit.body, headers)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404 for %s %s got %d body=%s", hit.method, hit.path, rec.Code, rec.Body.String())
		}
	}

	tenantRequiredHits := []string{"/api/persons", "/api/projects", "/api/groups", "/api/allocations"}
	for _, path := range tenantRequiredHits {
		rec := doJSONRequest(t, router, http.MethodGet, path, nil, map[string]string{"X-Role": "org_admin"})
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for tenant-required path %s got %d body=%s", path, rec.Code, rec.Body.String())
		}
	}
}

type failingAuthProvider struct{}

func (failingAuthProvider) FromRequest(_ *http.Request) (ports.AuthContext, error) {
	return ports.AuthContext{}, errors.New("forced auth failure")
}

type errorRepository struct {
	ports.Repository
}

func (e errorRepository) ListOrganisations(_ context.Context) ([]domain.Organisation, error) {
	return nil, errors.New("forced repository failure")
}

type personUnavailabilityDeleteErrorRepository struct {
	ports.Repository
}

func (e personUnavailabilityDeleteErrorRepository) DeletePersonUnavailabilityByPerson(_ context.Context, _, _, _ string) error {
	return errors.New("forced person unavailability delete failure")
}

func TestPathHelpers(t *testing.T) {
	if values := splitPath(""); len(values) != 0 {
		t.Fatalf("expected empty split path, got %v", values)
	}
	if values := splitPath("/api/projects"); len(values) != 2 {
		t.Fatalf("expected two split path entries, got %v", values)
	}

	if err := enforcePathTenant(ports.AuthContext{OrganisationID: ""}, "org_1"); err != nil {
		t.Fatalf("expected no tenant enforcement when auth tenant is empty, got %v", err)
	}
	if err := enforcePathTenant(ports.AuthContext{OrganisationID: "org_1"}, "org_1"); err != nil {
		t.Fatalf("expected no tenant mismatch error, got %v", err)
	}
	if err := enforcePathTenant(ports.AuthContext{OrganisationID: "org_2"}, "org_1"); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected tenant mismatch forbidden, got %v", err)
	}
}

func TestOrganisationAndReportExtraBranches(t *testing.T) {
	router := newTestRouter(t)
	orgID := createOrganisation(t, router, map[string]string{"X-Role": "org_admin"})
	headers := map[string]string{"X-Role": "org_admin", "X-Org-ID": orgID}

	holidayResp := doJSONRequest(t, router, http.MethodPost, "/api/organisations/"+orgID+"/holidays", map[string]any{"date": "2026-03-01", "hours": 8}, headers)
	if holidayResp.Code != http.StatusCreated {
		t.Fatalf("create holiday failed: %d body=%s", holidayResp.Code, holidayResp.Body.String())
	}
	var holiday domain.OrgHoliday
	if err := json.Unmarshal(holidayResp.Body.Bytes(), &holiday); err != nil {
		t.Fatalf("decode holiday: %v", err)
	}

	hits := []struct {
		method string
		path   string
		code   int
	}{
		{http.MethodGet, "/api/organisations/" + orgID + "/holidays/" + holiday.ID, http.StatusNotFound},
		{http.MethodPatch, "/api/organisations/" + orgID + "/holidays/" + holiday.ID, http.StatusNotFound},
		{http.MethodDelete, "/api/organisations/" + orgID + "/holidays/missing", http.StatusNotFound},
		{http.MethodGet, "/api/organisations/" + orgID + "/unknown", http.StatusNotFound},
		{http.MethodGet, "/api/organisations/" + orgID + "/holidays/extra/path", http.StatusNotFound},
	}

	for _, hit := range hits {
		response := doJSONRequest(t, router, hit.method, hit.path, nil, headers)
		if response.Code != hit.code {
			t.Fatalf("expected %d for %s %s got %d body=%s", hit.code, hit.method, hit.path, response.Code, response.Body.String())
		}
	}

	reportBadScope := doJSONRequest(t, router, http.MethodPost, "/api/reports/availability-load", map[string]any{
		"scope":       "unknown",
		"from_date":   "2026-01-01",
		"to_date":     "2026-01-02",
		"granularity": "day",
	}, headers)
	if reportBadScope.Code != http.StatusBadRequest {
		t.Fatalf("expected report bad scope validation status 400, got %d body=%s", reportBadScope.Code, reportBadScope.Body.String())
	}
}

func TestDeletePersonUnavailabilityByPersonError(t *testing.T) {
	repo, err := persistence.NewFileRepository(filepath.Join(t.TempDir(), "person-unavailability-list-error-data.json"))
	if err != nil {
		t.Fatalf("create repository: %v", err)
	}

	svc, err := service.New(personUnavailabilityDeleteErrorRepository{Repository: repo}, telemetry.NewNoopTelemetry(), impexp.NewNoopImportExport())
	if err != nil {
		t.Fatalf("create service: %v", err)
	}
	router := NewRouterWithDependencies(auth.NewDevAuthProvider(), svc)

	orgID := createOrganisation(t, router, map[string]string{"X-Role": "org_admin"})
	personID := createPerson(t, router, orgID, "List Error Person", 100)
	headers := map[string]string{"X-Role": "org_admin", "X-Org-ID": orgID}

	response := doJSONRequest(t, router, http.MethodDelete, "/api/persons/"+personID+"/unavailability/entry-1", nil, headers)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500 for person-scoped unavailability delete failure, got %d body=%s", response.Code, response.Body.String())
	}
}

func TestDecodeJSONRequestBodyTooLarge(t *testing.T) {
	router := newTestRouter(t)
	oversizedName := strings.Repeat("a", int(maxJSONBodyBytes))
	body := []byte(fmt.Sprintf(`{"name":%q,"hours_per_day":8,"hours_per_week":40,"hours_per_year":2080}`, oversizedName))
	response := doRawRequest(t, router, http.MethodPost, "/api/organisations", body, map[string]string{"X-Role": "org_admin"})
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d for oversized request body, got %d body=%s", http.StatusBadRequest, response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "request body too large") {
		t.Fatalf("expected oversized request body error, got %s", response.Body.String())
	}
}

func newTestRouter(t *testing.T) http.Handler {
	t.Helper()
	repo, err := persistence.NewFileRepository(filepath.Join(t.TempDir(), "test-data.json"))
	if err != nil {
		t.Fatalf("create repository: %v", err)
	}

	svc, err := service.New(repo, telemetry.NewNoopTelemetry(), impexp.NewNoopImportExport())
	if err != nil {
		t.Fatalf("create service: %v", err)
	}
	return NewRouterWithDependencies(auth.NewDevAuthProvider(), svc)
}

func createOrganisation(t *testing.T, router http.Handler, headers map[string]string) string {
	t.Helper()
	response := doJSONRequest(t, router, http.MethodPost, "/api/organisations", map[string]any{
		"name":           fmt.Sprintf("Org-%s", t.Name()),
		"hours_per_day":  8,
		"hours_per_week": 40,
		"hours_per_year": 2080,
	}, headers)
	if response.Code != http.StatusCreated {
		t.Fatalf("create organisation failed: %d body=%s", response.Code, response.Body.String())
	}

	var organisation domain.Organisation
	if err := json.Unmarshal(response.Body.Bytes(), &organisation); err != nil {
		t.Fatalf("decode organisation: %v", err)
	}
	return organisation.ID
}

func createPerson(t *testing.T, router http.Handler, organisationID, name string, employmentPct float64) string {
	t.Helper()
	response := doJSONRequest(t, router, http.MethodPost, "/api/persons", map[string]any{
		"name":           name,
		"employment_pct": employmentPct,
	}, map[string]string{"X-Role": "org_admin", "X-Org-ID": organisationID})
	if response.Code != http.StatusCreated {
		t.Fatalf("create person failed: %d body=%s", response.Code, response.Body.String())
	}

	var person domain.Person
	if err := json.Unmarshal(response.Body.Bytes(), &person); err != nil {
		t.Fatalf("decode person: %v", err)
	}
	return person.ID
}

func projectPayload(name string) map[string]any {
	return map[string]any{
		"name":                   name,
		"start_date":             "2026-01-01",
		"end_date":               "2026-12-31",
		"estimated_effort_hours": 1000,
	}
}

func personAllocationPayload(personID, projectID string, percent float64) map[string]any {
	return map[string]any{
		"target_type": "person",
		"target_id":   personID,
		"project_id":  projectID,
		"start_date":  "2026-01-01",
		"end_date":    "2026-12-31",
		"percent":     percent,
	}
}

func createProject(t *testing.T, router http.Handler, organisationID, name string) string {
	t.Helper()
	response := doJSONRequest(t, router, http.MethodPost, "/api/projects", projectPayload(name), map[string]string{"X-Role": "org_admin", "X-Org-ID": organisationID})
	if response.Code != http.StatusCreated {
		t.Fatalf("create project failed: %d body=%s", response.Code, response.Body.String())
	}

	var project domain.Project
	if err := json.Unmarshal(response.Body.Bytes(), &project); err != nil {
		t.Fatalf("decode project: %v", err)
	}
	return project.ID
}

func doJSONRequest(t *testing.T, handler http.Handler, method, path string, body any, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	var payload []byte
	var err error
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
	}

	return doRawRequest(t, handler, method, path, payload, headers)
}

func doRawRequest(t *testing.T, handler http.Handler, method, path string, body []byte, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(method, path, bytes.NewReader(body))
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
