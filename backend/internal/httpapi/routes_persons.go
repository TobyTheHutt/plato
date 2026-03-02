package httpapi

import (
	"net/http"

	"plato/backend/internal/domain"
	"plato/backend/internal/ports"
)

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
	personID, ok := parseResourceID(segments)
	if !ok {
		notFound(w)
		return
	}

	if len(segments) == 3 {
		a.dispatchPersonByIDMethod(w, r, authCtx, personID)
		return
	}

	if isSubresourceRoute(segments, "unavailability") {
		a.handlePersonUnavailabilityRoute(w, r, authCtx, personID, segments)
		return
	}

	notFound(w)
}

func (a *API) dispatchPersonByIDMethod(w http.ResponseWriter, r *http.Request, authCtx ports.AuthContext, personID string) {
	switch r.Method {
	case http.MethodGet:
		a.getPersonByID(w, r, authCtx, personID)
	case http.MethodPut:
		a.updatePersonByID(w, r, authCtx, personID)
	case http.MethodDelete:
		a.deletePersonByID(w, r, authCtx, personID)
	default:
		methodNotAllowed(w)
	}
}

func (a *API) getPersonByID(w http.ResponseWriter, r *http.Request, authCtx ports.AuthContext, personID string) {
	person, err := a.service.GetPerson(r.Context(), authCtx, personID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, person)
}

func (a *API) updatePersonByID(w http.ResponseWriter, r *http.Request, authCtx ports.AuthContext, personID string) {
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
}

func (a *API) deletePersonByID(w http.ResponseWriter, r *http.Request, authCtx ports.AuthContext, personID string) {
	if err := a.service.DeletePerson(r.Context(), authCtx, personID); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) handlePersonUnavailabilityRoute(w http.ResponseWriter, r *http.Request, authCtx ports.AuthContext, personID string, segments []string) {
	switch len(segments) {
	case 4:
		a.dispatchPersonUnavailabilityMethod(w, r, authCtx, personID)
	case 5:
		if r.Method == http.MethodDelete {
			a.deletePersonUnavailabilityEntry(w, r, authCtx, personID, segments)
			return
		}
		notFound(w)
	default:
		notFound(w)
	}
}

func (a *API) dispatchPersonUnavailabilityMethod(w http.ResponseWriter, r *http.Request, authCtx ports.AuthContext, personID string) {
	switch r.Method {
	case http.MethodGet:
		a.listPersonUnavailability(w, r, authCtx, personID)
	case http.MethodPost:
		a.createPersonUnavailability(w, r, authCtx, personID)
	default:
		methodNotAllowed(w)
	}
}

func (a *API) listPersonUnavailability(w http.ResponseWriter, r *http.Request, authCtx ports.AuthContext, personID string) {
	entries, err := a.service.ListPersonUnavailabilityByPerson(r.Context(), authCtx, personID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, entries)
}

func (a *API) createPersonUnavailability(w http.ResponseWriter, r *http.Request, authCtx ports.AuthContext, personID string) {
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
}

func (a *API) deletePersonUnavailabilityEntry(w http.ResponseWriter, r *http.Request, authCtx ports.AuthContext, personID string, segments []string) {
	entryID, ok := parseSubresourceID(segments)
	if !ok {
		notFound(w)
		return
	}
	if err := a.service.DeletePersonUnavailabilityByPerson(r.Context(), authCtx, personID, entryID); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
