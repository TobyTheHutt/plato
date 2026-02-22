package httpapi

import (
	"net/http"

	"plato/backend/internal/domain"
	"plato/backend/internal/ports"
)

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
		if len(segments) == 4 {
			if r.Method == http.MethodPost {
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
			w.Header().Set("Allow", http.MethodPost)
			methodNotAllowed(w)
			return
		}

		if len(segments) == 5 {
			if r.Method == http.MethodDelete {
				personID := segments[4]
				updated, err := a.service.RemoveGroupMember(r.Context(), authCtx, groupID, personID)
				if err != nil {
					writeServiceError(w, err)
					return
				}
				writeJSON(w, http.StatusOK, updated)
				return
			}
			w.Header().Set("Allow", http.MethodDelete)
			methodNotAllowed(w)
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
