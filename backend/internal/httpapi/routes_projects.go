package httpapi

import (
	"net/http"

	"plato/backend/internal/domain"
	"plato/backend/internal/ports"
)

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
