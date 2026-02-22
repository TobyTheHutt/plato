package httpapi

import (
	"net/http"

	"plato/backend/internal/domain"
	"plato/backend/internal/ports"
)

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
