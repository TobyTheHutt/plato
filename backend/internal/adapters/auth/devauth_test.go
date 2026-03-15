package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

const testFallbackValue = "fallback"

// TestDevAuthProviderFromRequest verifies the dev auth provider from request scenario.
func TestDevAuthProviderFromRequest(t *testing.T) {
	t.Setenv(devUserIDEnvVar, "fallback-user")
	t.Setenv(devOrgIDEnvVar, "fallback-org")
	t.Setenv(devRolesEnvVar, "org_user")

	provider := NewDevAuthProvider()
	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
	request.Header.Set(headerUserID, "request-user")
	request.Header.Set(headerOrgID, "request-org")
	request.Header.Set(headerRoles, "org_admin, org_user")

	ctx, err := provider.FromRequest(request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.UserID != "request-user" {
		t.Fatalf("unexpected user id: %s", ctx.UserID)
	}
	if ctx.OrganisationID != "request-org" {
		t.Fatalf("unexpected org id: %s", ctx.OrganisationID)
	}
	if len(ctx.Roles) != 2 {
		t.Fatalf("expected two roles, got %d", len(ctx.Roles))
	}
}

// TestDevAuthProviderDefaults verifies the dev auth provider defaults scenario.
func TestDevAuthProviderDefaults(t *testing.T) {
	t.Setenv(devUserIDEnvVar, "")
	t.Setenv(devOrgIDEnvVar, "")
	t.Setenv(devRolesEnvVar, "")

	provider := NewDevAuthProvider()
	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
	ctx, err := provider.FromRequest(request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.UserID != defaultDevUserID {
		t.Fatalf("expected default user, got %s", ctx.UserID)
	}
	if len(ctx.Roles) == 0 || ctx.Roles[0] != defaultDevAdminRole {
		t.Fatalf("expected default role org_admin, got %v", ctx.Roles)
	}
}

// TestParseRoles verifies the parse roles scenario.
func TestParseRoles(t *testing.T) {
	roles := parseRoles(" org_admin, , org_user ")
	if len(roles) != 2 {
		t.Fatalf("expected 2 roles, got %d", len(roles))
	}
	if roles[0] != "org_admin" || roles[1] != "org_user" {
		t.Fatalf("unexpected roles: %v", roles)
	}
}

// TestGetenv verifies the getenv scenario.
func TestGetenv(t *testing.T) {
	if getenv("NOT_SET", testFallbackValue) != testFallbackValue {
		t.Fatal("expected fallback env value")
	}

	t.Setenv("TRIMMED", "  value  ")
	if getenv("TRIMMED", testFallbackValue) != "value" {
		t.Fatal("expected getenv to trim whitespace")
	}
}
