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
	groupID, ok := parseResourceID(segments)
	if !ok {
		notFound(w)
		return
	}

	if len(segments) == 3 {
		a.dispatchGroupByIDMethod(w, r, authCtx, groupID)
		return
	}

	if isSubresourceRoute(segments, "members") {
		a.handleGroupMembersRoute(w, r, authCtx, groupID, segments)
		return
	}

	if isSubresourceRoute(segments, "unavailability") {
		a.handleGroupUnavailabilityRoute(w, r, authCtx, groupID, segments)
		return
	}

	notFound(w)
}

func (a *API) dispatchGroupByIDMethod(w http.ResponseWriter, r *http.Request, authCtx ports.AuthContext, groupID string) {
	switch r.Method {
	case http.MethodGet:
		a.getGroupByID(w, r, authCtx, groupID)
	case http.MethodPut:
		a.updateGroupByID(w, r, authCtx, groupID)
	case http.MethodDelete:
		a.deleteGroupByID(w, r, authCtx, groupID)
	default:
		methodNotAllowed(w)
	}
}

func (a *API) getGroupByID(w http.ResponseWriter, r *http.Request, authCtx ports.AuthContext, groupID string) {
	group, err := a.service.GetGroup(r.Context(), authCtx, groupID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, group)
}

func (a *API) updateGroupByID(w http.ResponseWriter, r *http.Request, authCtx ports.AuthContext, groupID string) {
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
}

func (a *API) deleteGroupByID(w http.ResponseWriter, r *http.Request, authCtx ports.AuthContext, groupID string) {
	if err := a.service.DeleteGroup(r.Context(), authCtx, groupID); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) handleGroupMembersRoute(w http.ResponseWriter, r *http.Request, authCtx ports.AuthContext, groupID string, segments []string) {
	switch len(segments) {
	case 4:
		a.addGroupMember(w, r, authCtx, groupID)
	case 5:
		a.removeGroupMember(w, r, authCtx, groupID, segments)
	default:
		notFound(w)
	}
}

func (a *API) addGroupMember(w http.ResponseWriter, r *http.Request, authCtx ports.AuthContext, groupID string) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		methodNotAllowed(w)
		return
	}

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
}

func (a *API) removeGroupMember(w http.ResponseWriter, r *http.Request, authCtx ports.AuthContext, groupID string, segments []string) {
	if r.Method != http.MethodDelete {
		w.Header().Set("Allow", http.MethodDelete)
		methodNotAllowed(w)
		return
	}

	personID, ok := parseSubresourceID(segments)
	if !ok {
		notFound(w)
		return
	}
	updated, err := a.service.RemoveGroupMember(r.Context(), authCtx, groupID, personID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (a *API) handleGroupUnavailabilityRoute(w http.ResponseWriter, r *http.Request, authCtx ports.AuthContext, groupID string, segments []string) {
	switch len(segments) {
	case 4:
		a.dispatchGroupUnavailabilityMethod(w, r, authCtx, groupID)
	case 5:
		if r.Method == http.MethodDelete {
			a.deleteGroupUnavailabilityEntry(w, r, authCtx, segments)
			return
		}
		notFound(w)
	default:
		notFound(w)
	}
}

func (a *API) dispatchGroupUnavailabilityMethod(w http.ResponseWriter, r *http.Request, authCtx ports.AuthContext, groupID string) {
	switch r.Method {
	case http.MethodGet:
		a.listGroupUnavailability(w, r, authCtx, groupID)
	case http.MethodPost:
		a.createGroupUnavailability(w, r, authCtx, groupID)
	default:
		methodNotAllowed(w)
	}
}

func (a *API) listGroupUnavailability(w http.ResponseWriter, r *http.Request, authCtx ports.AuthContext, groupID string) {
	entries, err := a.service.ListGroupUnavailability(r.Context(), authCtx)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, filterGroupUnavailabilityByGroup(entries, groupID))
}

func (a *API) createGroupUnavailability(w http.ResponseWriter, r *http.Request, authCtx ports.AuthContext, groupID string) {
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
}

func (a *API) deleteGroupUnavailabilityEntry(w http.ResponseWriter, r *http.Request, authCtx ports.AuthContext, segments []string) {
	entryID, ok := parseSubresourceID(segments)
	if !ok {
		notFound(w)
		return
	}
	if err := a.service.DeleteGroupUnavailability(r.Context(), authCtx, entryID); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func filterGroupUnavailabilityByGroup(entries []domain.GroupUnavailability, groupID string) []domain.GroupUnavailability {
	filtered := make([]domain.GroupUnavailability, 0, len(entries))
	for _, entry := range entries {
		if entry.GroupID == groupID {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}
