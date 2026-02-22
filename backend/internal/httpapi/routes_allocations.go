package httpapi

import (
	"net/http"

	"plato/backend/internal/domain"
	"plato/backend/internal/ports"
)

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
