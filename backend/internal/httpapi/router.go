package httpapi

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"

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
	corsPolicy   corsPolicy
	service      *service.Service
	cleanup      func() error
	closeOnce    sync.Once
	closeErr     error
}

type apiRouteMatcher func(api *API, w http.ResponseWriter, r *http.Request, authCtx ports.AuthContext, segments []string) bool

var apiRouteMatchers = []apiRouteMatcher{
	matchOrganisationsRoute,
	matchPersonsRoute,
	matchProjectsRoute,
	matchGroupsRoute,
	matchAllocationsRoute,
	matchReportsRoute,
}

func NewRouter(runtimeConfig RuntimeConfig) (http.Handler, error) {
	dataFile := strings.TrimSpace(os.Getenv("PLATO_DATA_FILE"))
	repo, err := persistence.NewFileRepository(dataFile)
	if err != nil {
		return nil, fmt.Errorf("create repository (%q): %w", dataFile, err)
	}
	cleanupOnError := func(cause error) error {
		if closeErr := repo.Close(); closeErr != nil {
			return fmt.Errorf("%w (cleanup repository: %s)", cause, closeErr.Error())
		}
		return cause
	}

	svc, err := service.New(repo, telemetry.NewNoopTelemetry(), impexp.NewNoopImportExport())
	if err != nil {
		return nil, cleanupOnError(fmt.Errorf("create service (%q): %w", dataFile, err))
	}

	authProvider, err := authProviderFromMode(runtimeConfig.Mode)
	if err != nil {
		return nil, cleanupOnError(err)
	}

	api := &API{
		authProvider: authProvider,
		corsPolicy:   newCORSPolicy(runtimeConfig),
		service:      svc,
		cleanup:      repo.Close,
	}

	return api, nil
}

func NewRouterFromEnv() (http.Handler, error) {
	runtimeConfig, err := LoadRuntimeConfigFromEnv()
	if err != nil {
		return nil, fmt.Errorf("load runtime config: %w", err)
	}
	return NewRouter(runtimeConfig)
}

func NewRouterWithDependencies(authProvider ports.AuthProvider, svc *service.Service) http.Handler {
	return &API{
		authProvider: authProvider,
		corsPolicy: newCORSPolicy(RuntimeConfig{
			Mode:               RuntimeModeDevelopment,
			AllowAnyCORSOrigin: true,
		}),
		service: svc,
	}
}

func authProviderFromMode(mode RuntimeMode) (ports.AuthProvider, error) {
	if mode.IsDevelopment() {
		return auth.NewDevAuthProvider(), nil
	}

	provider, err := auth.NewJWTAuthProviderFromEnv()
	if err != nil {
		return nil, fmt.Errorf("create production auth provider: %w", err)
	}

	return provider, nil
}

func (a *API) Close() error {
	a.closeOnce.Do(func() {
		if a.cleanup == nil {
			return
		}

		cleanup := a.cleanup
		a.cleanup = nil
		a.closeErr = cleanup()
	})

	return a.closeErr
}

func (a *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	setCORS(w, r, a.corsPolicy)
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
	if a.dispatchRoute(w, r, authCtx, segments) {
		return
	}

	notFound(w)
}

func (a *API) dispatchRoute(w http.ResponseWriter, r *http.Request, authCtx ports.AuthContext, segments []string) bool {
	for _, matcher := range apiRouteMatchers {
		if matcher(a, w, r, authCtx, segments) {
			return true
		}
	}
	return false
}

func matchOrganisationsRoute(api *API, w http.ResponseWriter, r *http.Request, authCtx ports.AuthContext, segments []string) bool {
	if isCollectionRoute(segments, "organisations") {
		api.handleOrganisations(w, r, authCtx)
		return true
	}
	if isItemRoute(segments, "organisations") {
		api.handleOrganisationByID(w, r, authCtx, segments)
		return true
	}
	return false
}

func matchPersonsRoute(api *API, w http.ResponseWriter, r *http.Request, authCtx ports.AuthContext, segments []string) bool {
	if isCollectionRoute(segments, "persons") {
		api.handlePersons(w, r, authCtx)
		return true
	}
	if isItemRoute(segments, "persons") {
		api.handlePersonByID(w, r, authCtx, segments)
		return true
	}
	return false
}

func matchProjectsRoute(api *API, w http.ResponseWriter, r *http.Request, authCtx ports.AuthContext, segments []string) bool {
	if isCollectionRoute(segments, "projects") {
		api.handleProjects(w, r, authCtx)
		return true
	}
	if isItemRoute(segments, "projects") {
		api.handleProjectByID(w, r, authCtx, segments)
		return true
	}
	return false
}

func matchGroupsRoute(api *API, w http.ResponseWriter, r *http.Request, authCtx ports.AuthContext, segments []string) bool {
	if isCollectionRoute(segments, "groups") {
		api.handleGroups(w, r, authCtx)
		return true
	}
	if isItemRoute(segments, "groups") {
		api.handleGroupByID(w, r, authCtx, segments)
		return true
	}
	return false
}

func matchAllocationsRoute(api *API, w http.ResponseWriter, r *http.Request, authCtx ports.AuthContext, segments []string) bool {
	if isCollectionRoute(segments, "allocations") {
		api.handleAllocations(w, r, authCtx)
		return true
	}
	if isItemRoute(segments, "allocations") {
		api.handleAllocationByID(w, r, authCtx, segments)
		return true
	}
	return false
}

func matchReportsRoute(api *API, w http.ResponseWriter, r *http.Request, authCtx ports.AuthContext, segments []string) bool {
	if !isExactRoute(segments, "api", "reports", "availability-load") {
		return false
	}

	api.handleReportAvailabilityLoad(w, r, authCtx)
	return true
}
