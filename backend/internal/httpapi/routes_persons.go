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
				entries, err := a.service.ListPersonUnavailabilityByPerson(r.Context(), authCtx, personID)
				if err != nil {
					writeServiceError(w, err)
					return
				}
				writeJSON(w, http.StatusOK, entries)
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
			if err := a.service.DeletePersonUnavailabilityByPerson(r.Context(), authCtx, personID, entryID); err != nil {
				writeServiceError(w, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}

	notFound(w)
}
