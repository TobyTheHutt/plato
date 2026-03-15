package service

import (
	"errors"
	"strings"

	"plato/backend/internal/domain"
	"plato/backend/internal/ports"
)

func requiredOrganisationID(auth ports.AuthContext) (string, error) {
	organisationID := strings.TrimSpace(auth.OrganisationID)
	if organisationID == "" {
		return "", domain.ErrForbidden
	}
	return organisationID, nil
}

func requireAnyRole(auth ports.AuthContext, roles ...string) error {
	if len(roles) == 0 {
		return domain.ErrForbidden
	}
	for _, role := range roles {
		if auth.HasRole(role) {
			return nil
		}
	}
	return domain.ErrForbidden
}

func enforceTenant(auth ports.AuthContext, targetOrganisationID string) error {
	organisationID := strings.TrimSpace(auth.OrganisationID)
	if organisationID == "" {
		return nil
	}
	if organisationID != strings.TrimSpace(targetOrganisationID) {
		return domain.ErrForbidden
	}
	return nil
}

// IsValidationError reports whether err matches the validation sentinel.
func IsValidationError(err error) bool {
	return errors.Is(err, domain.ErrValidation)
}

// IsForbiddenError reports whether err matches the forbidden sentinel.
func IsForbiddenError(err error) bool {
	return errors.Is(err, domain.ErrForbidden)
}

// IsNotFoundError reports whether err matches the not-found sentinel.
func IsNotFoundError(err error) bool {
	return errors.Is(err, domain.ErrNotFound)
}
