package ports

import "testing"

func TestAuthContextHasRole(t *testing.T) {
	ctx := AuthContext{Roles: []string{"org_user"}}
	if !ctx.HasRole("org_user") {
		t.Fatal("expected org_user role")
	}
	if ctx.HasRole("org_admin") {
		t.Fatal("did not expect org_admin role")
	}
}
