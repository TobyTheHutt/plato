package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"plato/backend/internal/adapters/auth"
	"plato/backend/internal/adapters/impexp"
	"plato/backend/internal/adapters/persistence"
	"plato/backend/internal/adapters/telemetry"
	"plato/backend/internal/domain"
	"plato/backend/internal/ports"
	"plato/backend/internal/service"
)

const maxJSONBodyBytes int64 = 1 << 20

type API struct {
	authProvider ports.AuthProvider
	service      *service.Service
}

func NewRouter() http.Handler {
	dataFile := strings.TrimSpace(os.Getenv("PLATO_DATA_FILE"))
	repo, err := persistence.NewFileRepository(dataFile)
	if err != nil {
		panic(fmt.Sprintf("create repository (%q): %v", dataFile, err))
	}
	svc, err := service.New(repo, telemetry.NewNoopTelemetry(), impexp.NewNoopImportExport())
	if err != nil {
		panic(fmt.Sprintf("create service (%q): %v", dataFile, err))
	}

	api := &API{
		authProvider: auth.NewDevAuthProvider(),
		service:      svc,
	}

	return api
}

func NewRouterWithDependencies(authProvider ports.AuthProvider, svc *service.Service) http.Handler {
	return &API{authProvider: authProvider, service: svc}
}

func (a *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.URL.Path == "/healthz" {
		healthz(w, r)
		return
	}

	if !strings.HasPrefix(r.URL.Path, "/api/") {
		notFound(w)
		return
	}

	authCtx, err := a.authProvider.FromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication failed")
		return
	}

	segments := splitPath(r.URL.Path)
	switch {
	case len(segments) == 2 && segments[1] == "organisations":
		a.handleOrganisations(w, r, authCtx)
	case len(segments) >= 3 && segments[1] == "organisations":
		a.handleOrganisationByID(w, r, authCtx, segments)
	case len(segments) == 2 && segments[1] == "persons":
		a.handlePersons(w, r, authCtx)
	case len(segments) >= 3 && segments[1] == "persons":
		a.handlePersonByID(w, r, authCtx, segments)
	case len(segments) == 2 && segments[1] == "projects":
		a.handleProjects(w, r, authCtx)
	case len(segments) >= 3 && segments[1] == "projects":
		a.handleProjectByID(w, r, authCtx, segments)
	case len(segments) == 2 && segments[1] == "groups":
		a.handleGroups(w, r, authCtx)
	case len(segments) >= 3 && segments[1] == "groups":
		a.handleGroupByID(w, r, authCtx, segments)
	case len(segments) == 2 && segments[1] == "allocations":
		a.handleAllocations(w, r, authCtx)
	case len(segments) >= 3 && segments[1] == "allocations":
		a.handleAllocationByID(w, r, authCtx, segments)
	case len(segments) == 3 && segments[1] == "reports" && segments[2] == "availability-load":
		a.handleReportAvailabilityLoad(w, r, authCtx)
	default:
		notFound(w)
	}
}

func (a *API) handleOrganisations(w http.ResponseWriter, r *http.Request, authCtx ports.AuthContext) {
	switch r.Method {
	case http.MethodGet:
		organisations, err := a.service.ListOrganisations(r.Context(), authCtx)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, organisations)
	case http.MethodPost:
		var input domain.Organisation
		if err := decodeJSON(w, r, &input); err != nil {
			writeDecodeError(w, err)
			return
		}

		created, err := a.service.CreateOrganisation(r.Context(), authCtx, input)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, created)
	default:
		methodNotAllowed(w)
	}
}

func (a *API) handleOrganisationByID(w http.ResponseWriter, r *http.Request, authCtx ports.AuthContext, segments []string) {
	organisationID := segments[2]
	if len(segments) == 3 {
		switch r.Method {
		case http.MethodGet:
			organisation, err := a.service.GetOrganisation(r.Context(), authCtx, organisationID)
			if err != nil {
				writeServiceError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, organisation)
		case http.MethodPut:
			var input domain.Organisation
			if err := decodeJSON(w, r, &input); err != nil {
				writeDecodeError(w, err)
				return
			}
			updated, err := a.service.UpdateOrganisation(r.Context(), authCtx, organisationID, input)
			if err != nil {
				writeServiceError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, updated)
		case http.MethodDelete:
			if err := a.service.DeleteOrganisation(r.Context(), authCtx, organisationID); err != nil {
				writeServiceError(w, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			methodNotAllowed(w)
		}
		return
	}

	if len(segments) >= 4 && segments[3] == "holidays" {
		if err := enforcePathTenant(authCtx, organisationID); err != nil {
			writeServiceError(w, err)
			return
		}

		if len(segments) == 4 {
			switch r.Method {
			case http.MethodGet:
				holidays, err := a.service.ListOrgHolidays(r.Context(), authCtx)
				if err != nil {
					writeServiceError(w, err)
					return
				}
				writeJSON(w, http.StatusOK, holidays)
			case http.MethodPost:
				var input domain.OrgHoliday
				if err := decodeJSON(w, r, &input); err != nil {
					writeDecodeError(w, err)
					return
				}
				created, err := a.service.CreateOrgHoliday(r.Context(), authCtx, input)
				if err != nil {
					writeServiceError(w, err)
					return
				}
				writeJSON(w, http.StatusCreated, created)
			default:
				methodNotAllowed(w)
			}
			return
		}

		if len(segments) == 5 && r.Method == http.MethodDelete {
			holidayID := segments[4]
			if err := a.service.DeleteOrgHoliday(r.Context(), authCtx, holidayID); err != nil {
				writeServiceError(w, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}

	notFound(w)
}

func (a *API) handlePersons(w http.ResponseWriter, r *http.Request, authCtx ports.AuthContext) {
	switch r.Method {
	case http.MethodGet:
		persons, err := a.service.ListPersons(r.Context(), authCtx)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, persons)
	case http.MethodPost:
		var input domain.Person
		if err := decodeJSON(w, r, &input); err != nil {
			writeDecodeError(w, err)
			return
		}
		created, err := a.service.CreatePerson(r.Context(), authCtx, input)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, created)
	default:
		methodNotAllowed(w)
	}
}

func (a *API) handlePersonByID(w http.ResponseWriter, r *http.Request, authCtx ports.AuthContext, segments []string) {
	personID := segments[2]
	if len(segments) == 3 {
		switch r.Method {
		case http.MethodGet:
			person, err := a.service.GetPerson(r.Context(), authCtx, personID)
			if err != nil {
				writeServiceError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, person)
		case http.MethodPut:
			var input domain.Person
			if err := decodeJSON(w, r, &input); err != nil {
				writeDecodeError(w, err)
				return
			}
			updated, err := a.service.UpdatePerson(r.Context(), authCtx, personID, input)
			if err != nil {
				writeServiceError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, updated)
		case http.MethodDelete:
			if err := a.service.DeletePerson(r.Context(), authCtx, personID); err != nil {
				writeServiceError(w, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			methodNotAllowed(w)
		}
		return
	}

	if len(segments) >= 4 && segments[3] == "unavailability" {
		if len(segments) == 4 {
			switch r.Method {
			case http.MethodGet:
				entries, err := a.service.ListPersonUnavailability(r.Context(), authCtx)
				if err != nil {
					writeServiceError(w, err)
					return
				}
				filtered := make([]domain.PersonUnavailability, 0)
				for _, entry := range entries {
					if entry.PersonID == personID {
						filtered = append(filtered, entry)
					}
				}
				writeJSON(w, http.StatusOK, filtered)
			case http.MethodPost:
				var input domain.PersonUnavailability
				if err := decodeJSON(w, r, &input); err != nil {
					writeDecodeError(w, err)
					return
				}
				input.PersonID = personID
				created, err := a.service.CreatePersonUnavailability(r.Context(), authCtx, input)
				if err != nil {
					writeServiceError(w, err)
					return
				}
				writeJSON(w, http.StatusCreated, created)
			default:
				methodNotAllowed(w)
			}
			return
		}

		if len(segments) == 5 && r.Method == http.MethodDelete {
			entryID := segments[4]
			if err := a.service.DeletePersonUnavailability(r.Context(), authCtx, entryID); err != nil {
				writeServiceError(w, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}

	notFound(w)
}

func (a *API) handleProjects(w http.ResponseWriter, r *http.Request, authCtx ports.AuthContext) {
	switch r.Method {
	case http.MethodGet:
		projects, err := a.service.ListProjects(r.Context(), authCtx)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, projects)
	case http.MethodPost:
		var input domain.Project
		if err := decodeJSON(w, r, &input); err != nil {
			writeDecodeError(w, err)
			return
		}
		created, err := a.service.CreateProject(r.Context(), authCtx, input)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, created)
	default:
		methodNotAllowed(w)
	}
}

func (a *API) handleProjectByID(w http.ResponseWriter, r *http.Request, authCtx ports.AuthContext, segments []string) {
	projectID := segments[2]
	switch r.Method {
	case http.MethodGet:
		project, err := a.service.GetProject(r.Context(), authCtx, projectID)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, project)
	case http.MethodPut:
		var input domain.Project
		if err := decodeJSON(w, r, &input); err != nil {
			writeDecodeError(w, err)
			return
		}
		updated, err := a.service.UpdateProject(r.Context(), authCtx, projectID, input)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, updated)
	case http.MethodDelete:
		if err := a.service.DeleteProject(r.Context(), authCtx, projectID); err != nil {
			writeServiceError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		methodNotAllowed(w)
	}
}

func (a *API) handleGroups(w http.ResponseWriter, r *http.Request, authCtx ports.AuthContext) {
	switch r.Method {
	case http.MethodGet:
		groups, err := a.service.ListGroups(r.Context(), authCtx)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, groups)
	case http.MethodPost:
		var input domain.Group
		if err := decodeJSON(w, r, &input); err != nil {
			writeDecodeError(w, err)
			return
		}
		created, err := a.service.CreateGroup(r.Context(), authCtx, input)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, created)
	default:
		methodNotAllowed(w)
	}
}

func (a *API) handleGroupByID(w http.ResponseWriter, r *http.Request, authCtx ports.AuthContext, segments []string) {
	groupID := segments[2]
	if len(segments) == 3 {
		switch r.Method {
		case http.MethodGet:
			group, err := a.service.GetGroup(r.Context(), authCtx, groupID)
			if err != nil {
				writeServiceError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, group)
		case http.MethodPut:
			var input domain.Group
			if err := decodeJSON(w, r, &input); err != nil {
				writeDecodeError(w, err)
				return
			}
			updated, err := a.service.UpdateGroup(r.Context(), authCtx, groupID, input)
			if err != nil {
				writeServiceError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, updated)
		case http.MethodDelete:
			if err := a.service.DeleteGroup(r.Context(), authCtx, groupID); err != nil {
				writeServiceError(w, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			methodNotAllowed(w)
		}
		return
	}

	if len(segments) >= 4 && segments[3] == "members" {
		if len(segments) == 4 && r.Method == http.MethodPost {
			var payload struct {
				PersonID string `json:"person_id"`
			}
			if err := decodeJSON(w, r, &payload); err != nil {
				writeDecodeError(w, err)
				return
			}
			updated, err := a.service.AddGroupMember(r.Context(), authCtx, groupID, payload.PersonID)
			if err != nil {
				writeServiceError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, updated)
			return
		}

		if len(segments) == 5 && r.Method == http.MethodDelete {
			personID := segments[4]
			updated, err := a.service.RemoveGroupMember(r.Context(), authCtx, groupID, personID)
			if err != nil {
				writeServiceError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, updated)
			return
		}
	}

	if len(segments) >= 4 && segments[3] == "unavailability" {
		if len(segments) == 4 {
			switch r.Method {
			case http.MethodGet:
				entries, err := a.service.ListGroupUnavailability(r.Context(), authCtx)
				if err != nil {
					writeServiceError(w, err)
					return
				}
				filtered := make([]domain.GroupUnavailability, 0)
				for _, entry := range entries {
					if entry.GroupID == groupID {
						filtered = append(filtered, entry)
					}
				}
				writeJSON(w, http.StatusOK, filtered)
			case http.MethodPost:
				var input domain.GroupUnavailability
				if err := decodeJSON(w, r, &input); err != nil {
					writeDecodeError(w, err)
					return
				}
				input.GroupID = groupID
				created, err := a.service.CreateGroupUnavailability(r.Context(), authCtx, input)
				if err != nil {
					writeServiceError(w, err)
					return
				}
				writeJSON(w, http.StatusCreated, created)
			default:
				methodNotAllowed(w)
			}
			return
		}

		if len(segments) == 5 && r.Method == http.MethodDelete {
			entryID := segments[4]
			if err := a.service.DeleteGroupUnavailability(r.Context(), authCtx, entryID); err != nil {
				writeServiceError(w, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}

	notFound(w)
}

func (a *API) handleAllocations(w http.ResponseWriter, r *http.Request, authCtx ports.AuthContext) {
	switch r.Method {
	case http.MethodGet:
		allocations, err := a.service.ListAllocations(r.Context(), authCtx)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, allocations)
	case http.MethodPost:
		var input domain.Allocation
		if err := decodeJSON(w, r, &input); err != nil {
			writeDecodeError(w, err)
			return
		}
		created, err := a.service.CreateAllocation(r.Context(), authCtx, input)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, created)
	default:
		methodNotAllowed(w)
	}
}

func (a *API) handleAllocationByID(w http.ResponseWriter, r *http.Request, authCtx ports.AuthContext, segments []string) {
	allocationID := segments[2]
	switch r.Method {
	case http.MethodGet:
		allocation, err := a.service.GetAllocation(r.Context(), authCtx, allocationID)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, allocation)
	case http.MethodPut:
		var input domain.Allocation
		if err := decodeJSON(w, r, &input); err != nil {
			writeDecodeError(w, err)
			return
		}
		updated, err := a.service.UpdateAllocation(r.Context(), authCtx, allocationID, input)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, updated)
	case http.MethodDelete:
		if err := a.service.DeleteAllocation(r.Context(), authCtx, allocationID); err != nil {
			writeServiceError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		methodNotAllowed(w)
	}
}

func (a *API) handleReportAvailabilityLoad(w http.ResponseWriter, r *http.Request, authCtx ports.AuthContext) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}

	var request domain.ReportRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeDecodeError(w, err)
		return
	}

	buckets, err := a.service.ReportAvailabilityAndLoad(r.Context(), authCtx, request)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"buckets": buckets})
}

func enforcePathTenant(authCtx ports.AuthContext, organisationID string) error {
	if strings.TrimSpace(authCtx.OrganisationID) == "" {
		return nil
	}
	if strings.TrimSpace(authCtx.OrganisationID) != strings.TrimSpace(organisationID) {
		return domain.ErrForbidden
	}
	return nil
}

func splitPath(path string) []string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return []string{}
	}
	return strings.Split(trimmed, "/")
}

func methodNotAllowed(w http.ResponseWriter) {
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}

func notFound(w http.ResponseWriter) {
	writeError(w, http.StatusNotFound, "not found")
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Printf("write json failed: status=%d body_type=%T err=%v", status, body, err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeDecodeError(w http.ResponseWriter, err error) {
	message := "invalid JSON"
	if strings.Contains(err.Error(), "request body too large") {
		message = fmt.Sprintf("request body too large (max %d bytes)", maxJSONBodyBytes)
	}
	writeError(w, http.StatusBadRequest, message)
}

func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden")
	case errors.Is(err, domain.ErrValidation):
		message := "validation failed"
		detailed := strings.TrimSpace(err.Error())
		suffix := ": " + domain.ErrValidation.Error()
		if strings.HasSuffix(detailed, suffix) {
			detailed = strings.TrimSuffix(detailed, suffix)
		}
		if detailed != "" && detailed != domain.ErrValidation.Error() {
			message = detailed
		}
		writeError(w, http.StatusBadRequest, message)
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, "not found")
	default:
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}

func setCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-User-ID, X-Org-ID, X-Role")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
}

func healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
