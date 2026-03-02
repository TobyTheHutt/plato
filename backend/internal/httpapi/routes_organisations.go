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
	organisationID, ok := parseResourceID(segments)
	if !ok {
		notFound(w)
		return
	}

	if len(segments) == 3 {
		a.dispatchOrganisationByIDMethod(w, r, authCtx, organisationID)
		return
	}

	if isSubresourceRoute(segments, "holidays") {
		a.handleOrganisationHolidaysRoute(w, r, authCtx, organisationID, segments)
		return
	}

	notFound(w)
}

func (a *API) dispatchOrganisationByIDMethod(w http.ResponseWriter, r *http.Request, authCtx ports.AuthContext, organisationID string) {
	switch r.Method {
	case http.MethodGet:
		a.getOrganisationByID(w, r, authCtx, organisationID)
	case http.MethodPut:
		a.updateOrganisationByID(w, r, authCtx, organisationID)
	case http.MethodDelete:
		a.deleteOrganisationByID(w, r, authCtx, organisationID)
	default:
		methodNotAllowed(w)
	}
}

func (a *API) getOrganisationByID(w http.ResponseWriter, r *http.Request, authCtx ports.AuthContext, organisationID string) {
	organisation, err := a.service.GetOrganisation(r.Context(), authCtx, organisationID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, organisation)
}

func (a *API) updateOrganisationByID(w http.ResponseWriter, r *http.Request, authCtx ports.AuthContext, organisationID string) {
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
}

func (a *API) deleteOrganisationByID(w http.ResponseWriter, r *http.Request, authCtx ports.AuthContext, organisationID string) {
	if err := a.service.DeleteOrganisation(r.Context(), authCtx, organisationID); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) handleOrganisationHolidaysRoute(w http.ResponseWriter, r *http.Request, authCtx ports.AuthContext, organisationID string, segments []string) {
	if err := enforcePathTenant(authCtx, organisationID); err != nil {
		writeServiceError(w, err)
		return
	}

	switch len(segments) {
	case 4:
		a.dispatchOrganisationHolidaysMethod(w, r, authCtx, organisationID)
	case 5:
		if r.Method == http.MethodDelete {
			a.deleteOrganisationHolidayByID(w, r, authCtx, segments)
			return
		}
		notFound(w)
	default:
		notFound(w)
	}
}

func (a *API) dispatchOrganisationHolidaysMethod(w http.ResponseWriter, r *http.Request, authCtx ports.AuthContext, organisationID string) {
	switch r.Method {
	case http.MethodGet:
		a.listOrganisationHolidays(w, r, authCtx)
	case http.MethodPost:
		a.createOrganisationHoliday(w, r, authCtx, organisationID)
	default:
		methodNotAllowed(w)
	}
}

func (a *API) listOrganisationHolidays(w http.ResponseWriter, r *http.Request, authCtx ports.AuthContext) {
	holidays, err := a.service.ListOrgHolidays(r.Context(), authCtx)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, holidays)
}

func (a *API) createOrganisationHoliday(w http.ResponseWriter, r *http.Request, authCtx ports.AuthContext, organisationID string) {
	var input domain.OrgHoliday
	if err := decodeJSON(w, r, &input); err != nil {
		writeDecodeError(w, err)
		return
	}
	input.OrganisationID = organisationID

	created, err := a.service.CreateOrgHoliday(r.Context(), authCtx, input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (a *API) deleteOrganisationHolidayByID(w http.ResponseWriter, r *http.Request, authCtx ports.AuthContext, segments []string) {
	holidayID, ok := parseSubresourceID(segments)
	if !ok {
		notFound(w)
		return
	}
	if err := a.service.DeleteOrgHoliday(r.Context(), authCtx, holidayID); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
