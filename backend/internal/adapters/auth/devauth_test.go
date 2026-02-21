package auth

import (
	"net/http/httptest"
	"testing"
)

func TestDevAuthProviderFromRequest(t *testing.T) {
	t.Setenv("PLATO_DEV_USER_ID", "fallback-user")
	t.Setenv("PLATO_DEV_ORG_ID", "fallback-org")
	t.Setenv("PLATO_DEV_ROLES", "org_user")

	provider := NewDevAuthProvider()
	request := httptest.NewRequest("GET", "/", nil)
	request.Header.Set("X-User-ID", "request-user")
	request.Header.Set("X-Org-ID", "request-org")
	request.Header.Set("X-Role", "org_admin, org_user")

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

func TestDevAuthProviderDefaults(t *testing.T) {
	t.Setenv("PLATO_DEV_USER_ID", "")
	t.Setenv("PLATO_DEV_ORG_ID", "")
	t.Setenv("PLATO_DEV_ROLES", "")

	provider := NewDevAuthProvider()
	request := httptest.NewRequest("GET", "/", nil)
	ctx, err := provider.FromRequest(request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.UserID != "dev-user" {
		t.Fatalf("expected default user, got %s", ctx.UserID)
	}
	if len(ctx.Roles) == 0 || ctx.Roles[0] != "org_admin" {
		t.Fatalf("expected default role org_admin, got %v", ctx.Roles)
	}
}

func TestParseRoles(t *testing.T) {
	roles := parseRoles(" org_admin, , org_user ")
	if len(roles) != 2 {
		t.Fatalf("expected 2 roles, got %d", len(roles))
	}
	if roles[0] != "org_admin" || roles[1] != "org_user" {
		t.Fatalf("unexpected roles: %v", roles)
	}
}

func TestGetenv(t *testing.T) {
	if getenv("NOT_SET", "fallback") != "fallback" {
		t.Fatal("expected fallback env value")
	}

	t.Setenv("TRIMMED", "  value  ")
	if getenv("TRIMMED", "fallback") != "value" {
		t.Fatal("expected getenv to trim whitespace")
	}
}
