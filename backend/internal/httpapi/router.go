package httpapi

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"plato/backend/internal/adapters/auth"
	"plato/backend/internal/adapters/impexp"
	"plato/backend/internal/adapters/persistence"
	"plato/backend/internal/adapters/telemetry"
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
