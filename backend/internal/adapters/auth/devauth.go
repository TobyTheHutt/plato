package auth

import (
	"errors"
	"net/http"
	"os"
	"strings"

	"plato/backend/internal/ports"
)

const (
	headerUserID = "X-User-ID"
	headerOrgID  = "X-Org-ID"
	headerRoles  = "X-Role"
)

type DevAuthProvider struct {
	defaultUserID string
	defaultOrgID  string
	defaultRoles  []string
}

func NewDevAuthProvider() *DevAuthProvider {
	userID := getenv("PLATO_DEV_USER_ID", "dev-user")
	orgID := getenv("PLATO_DEV_ORG_ID", "")
	roles := parseRoles(getenv("PLATO_DEV_ROLES", "org_admin"))
	if len(roles) == 0 {
		roles = []string{"org_admin"}
	}

	return &DevAuthProvider{
		defaultUserID: userID,
		defaultOrgID:  orgID,
		defaultRoles:  roles,
	}
}

func (p *DevAuthProvider) FromRequest(r *http.Request) (ports.AuthContext, error) {
	if p == nil {
		return ports.AuthContext{}, errors.New("auth provider is nil")
	}

	userID := strings.TrimSpace(r.Header.Get(headerUserID))
	if userID == "" {
		userID = p.defaultUserID
	}

	orgID := strings.TrimSpace(r.Header.Get(headerOrgID))
	if orgID == "" {
		orgID = p.defaultOrgID
	}

	rolesHeader := r.Header.Get(headerRoles)
	roles := parseRoles(rolesHeader)
	if len(roles) == 0 {
		roles = append([]string{}, p.defaultRoles...)
	}

	return ports.AuthContext{
		UserID:         userID,
		OrganisationID: orgID,
		Roles:          roles,
	}, nil
}

func parseRoles(raw string) []string {
	parts := strings.Split(raw, ",")
	roles := make([]string, 0, len(parts))
	for _, part := range parts {
		role := strings.TrimSpace(part)
		if role == "" {
			continue
		}
		roles = append(roles, role)
	}
	return roles
}

func getenv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}
