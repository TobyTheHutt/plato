package httpapi

import (
	"net/http"
	"strings"

	"plato/backend/internal/domain"
	"plato/backend/internal/ports"
)

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

func enforcePathTenant(authCtx ports.AuthContext, organisationID string) error {
	if strings.TrimSpace(authCtx.OrganisationID) == "" {
		return nil
	}
	if strings.TrimSpace(authCtx.OrganisationID) != strings.TrimSpace(organisationID) {
		return domain.ErrForbidden
	}
	return nil
}
