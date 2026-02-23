package auth

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewJWTAuthProviderFromEnv(t *testing.T) {
	t.Setenv("DEV_MODE", "")
	t.Setenv("PLATO_AUTH_JWT_HS256_SIGNING_KEY", "")
	t.Setenv("PLATO_AUTH_JWT_HS256_SECRET", "")
	if _, err := NewJWTAuthProviderFromEnv(); err == nil {
		t.Fatal("expected missing secret error")
	}

	t.Setenv("PLATO_AUTH_JWT_HS256_SIGNING_KEY", "dev-secret")
	provider, err := NewJWTAuthProviderFromEnv()
	if err != nil {
		t.Fatalf("create provider from env: %v", err)
	}
	if provider == nil {
		t.Fatal("expected provider")
	}

	if _, err := NewJWTAuthProvider(""); err == nil {
		t.Fatal("expected error for empty secret")
	}
}

func TestNewJWTAuthProviderFromEnvGeneratesSecretInDevelopmentMode(t *testing.T) {
	t.Setenv("DEV_MODE", "true")
	t.Setenv("PLATO_AUTH_JWT_HS256_SIGNING_KEY", "")
	t.Setenv("PLATO_AUTH_JWT_HS256_SECRET", "")

	firstProvider, err := NewJWTAuthProviderFromEnv()
	if err != nil {
		t.Fatalf("create development provider: %v", err)
	}
	secondProvider, err := NewJWTAuthProviderFromEnv()
	if err != nil {
		t.Fatalf("create second development provider: %v", err)
	}

	if len(firstProvider.signingKey) == 0 {
		t.Fatal("expected generated secret for first provider")
	}
	if len(secondProvider.signingKey) == 0 {
		t.Fatal("expected generated secret for second provider")
	}
	if bytes.Equal(firstProvider.signingKey, secondProvider.signingKey) {
		t.Fatal("expected generated development secrets to differ across provider instances")
	}
}

func TestNewJWTAuthProviderFromEnvUsesLegacySigningKey(t *testing.T) {
	t.Setenv("DEV_MODE", "")
	t.Setenv("PLATO_AUTH_JWT_HS256_SIGNING_KEY", "")
	t.Setenv("PLATO_AUTH_JWT_HS256_SECRET", "legacy-secret")

	provider, err := NewJWTAuthProviderFromEnv()
	if err != nil {
		t.Fatalf("create provider from legacy env var: %v", err)
	}
	if provider == nil {
		t.Fatal("expected provider")
	}
	if !bytes.Equal(provider.signingKey, []byte("legacy-secret")) {
		t.Fatalf("expected signing key to come from legacy env var")
	}
}

func TestJWTAuthProviderFromRequest(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	provider, err := NewJWTAuthProvider("test-secret")
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}
	provider.now = func() time.Time {
		return now
	}

	token := makeTestJWT(t, "test-secret", map[string]any{
		"sub":    "user_1",
		"org_id": "org_1",
		"roles":  []string{"org_admin", "org_user"},
		"exp":    now.Add(time.Hour).Unix(),
	})

	request := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	request.Header.Set(headerAuthorization, bearerPrefix+token)

	context, err := provider.FromRequest(request)
	if err != nil {
		t.Fatalf("authenticate request: %v", err)
	}
	if context.UserID != "user_1" {
		t.Fatalf("expected user_1, got %s", context.UserID)
	}
	if context.OrganisationID != "org_1" {
		t.Fatalf("expected org_1, got %s", context.OrganisationID)
	}
	if len(context.Roles) != 2 {
		t.Fatalf("expected two roles, got %v", context.Roles)
	}
}

func TestJWTAuthProviderFromRequestClaimFallbacks(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	provider, err := NewJWTAuthProvider("fallback-secret")
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}
	provider.now = func() time.Time {
		return now
	}

	token := makeTestJWT(t, "fallback-secret", map[string]any{
		"user_id":         "user_2",
		"organisation_id": "org_2",
		"roles":           "org_user,org_admin",
		"exp":             now.Add(time.Hour).Unix(),
	})

	request := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	request.Header.Set(headerAuthorization, bearerPrefix+token)

	context, err := provider.FromRequest(request)
	if err != nil {
		t.Fatalf("authenticate request: %v", err)
	}
	if context.UserID != "user_2" {
		t.Fatalf("expected user_2, got %s", context.UserID)
	}
	if context.OrganisationID != "org_2" {
		t.Fatalf("expected org_2, got %s", context.OrganisationID)
	}
	if len(context.Roles) != 2 {
		t.Fatalf("expected two roles, got %v", context.Roles)
	}
}

func TestJWTAuthProviderFromRequestErrors(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	provider, err := NewJWTAuthProvider("test-secret")
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}
	provider.now = func() time.Time {
		return now
	}

	tests := []struct {
		name        string
		headerValue string
	}{
		{
			name:        "missing bearer prefix",
			headerValue: "token-value",
		},
		{
			name:        "empty bearer token",
			headerValue: bearerPrefix + "   ",
		},
		{
			name:        "invalid token format",
			headerValue: bearerPrefix + "a.b",
		},
		{
			name: "invalid token signature",
			headerValue: bearerPrefix + makeTestJWT(t, "another-secret", map[string]any{
				"sub":   "user_1",
				"roles": []string{"org_admin"},
				"exp":   now.Add(time.Hour).Unix(),
			}),
		},
		{
			name: "missing subject",
			headerValue: bearerPrefix + makeTestJWT(t, "test-secret", map[string]any{
				"roles": []string{"org_admin"},
				"exp":   now.Add(time.Hour).Unix(),
			}),
		},
		{
			name: "expired token",
			headerValue: bearerPrefix + makeTestJWT(t, "test-secret", map[string]any{
				"sub":   "user_1",
				"roles": []string{"org_admin"},
				"exp":   now.Add(-time.Minute).Unix(),
			}),
		},
		{
			name: "not before in the future",
			headerValue: bearerPrefix + makeTestJWT(t, "test-secret", map[string]any{
				"sub":   "user_1",
				"roles": []string{"org_admin"},
				"nbf":   now.Add(time.Hour).Unix(),
				"exp":   now.Add(2 * time.Hour).Unix(),
			}),
		},
		{
			name: "missing roles",
			headerValue: bearerPrefix + makeTestJWT(t, "test-secret", map[string]any{
				"sub": "user_1",
				"exp": now.Add(time.Hour).Unix(),
			}),
		},
		{
			name: "unsupported algorithm",
			headerValue: bearerPrefix + makeTestJWTWithHeader(t, "test-secret", map[string]any{
				"alg": "HS384",
				"typ": "JWT",
			}, map[string]any{
				"sub":   "user_1",
				"roles": []string{"org_admin"},
				"exp":   now.Add(time.Hour).Unix(),
			}),
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			request.Header.Set(headerAuthorization, testCase.headerValue)

			if _, err := provider.FromRequest(request); err == nil {
				t.Fatal("expected authentication error")
			}
		})
	}
}

func TestJWTAuthProviderNilProvider(t *testing.T) {
	var provider *JWTAuthProvider
	request := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	if _, err := provider.FromRequest(request); err == nil {
		t.Fatal("expected error for nil provider")
	}
}

func TestParseNumericDateClaim(t *testing.T) {
	tests := []struct {
		name        string
		value       any
		expected    int64
		expectError bool
	}{
		{name: "float64", value: float64(42), expected: 42},
		{name: "int64", value: int64(21), expected: 21},
		{name: "int", value: int(7), expected: 7},
		{name: "json number", value: json.Number("15"), expected: 15},
		{name: "string", value: "9", expected: 9},
		{name: "invalid string", value: "nope", expectError: true},
		{name: "unsupported type", value: true, expectError: true},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			actual, err := parseNumericDateClaim(testCase.value)
			if testCase.expectError {
				if err == nil {
					t.Fatal("expected parse error")
				}
				return
			}
			if err != nil {
				t.Fatalf("parse numeric date claim: %v", err)
			}
			if actual != testCase.expected {
				t.Fatalf("expected %d, got %d", testCase.expected, actual)
			}
		})
	}
}

func TestParseRolesClaim(t *testing.T) {
	roles, err := parseRolesClaim(nil)
	if err != nil {
		t.Fatalf("parse nil roles claim: %v", err)
	}
	if len(roles) != 0 {
		t.Fatalf("expected no roles, got %v", roles)
	}

	roles, err = parseRolesClaim([]any{"org_admin", " org_user "})
	if err != nil {
		t.Fatalf("parse roles array claim: %v", err)
	}
	if len(roles) != 2 {
		t.Fatalf("expected two roles, got %v", roles)
	}

	if _, err := parseRolesClaim([]any{"org_admin", 123}); err == nil {
		t.Fatal("expected parse error for non-string role entry")
	}
	if _, err := parseRolesClaim(123); err == nil {
		t.Fatal("expected parse error for unsupported roles claim type")
	}
}

func TestValidateClaimHelpers(t *testing.T) {
	if err := validateExpiration(map[string]any{}, 100); err == nil {
		t.Fatal("expected missing exp claim error")
	}
	if err := validateNotBefore(map[string]any{"nbf": "invalid"}, 100); err == nil {
		t.Fatal("expected nbf parse error")
	}
	if err := validateExpiration(map[string]any{"exp": int64(101)}, 100); err != nil {
		t.Fatalf("expected valid expiration, got %v", err)
	}
	if err := validateNotBefore(map[string]any{"nbf": int64(100)}, 100); err != nil {
		t.Fatalf("expected valid not-before, got %v", err)
	}

	if _, err := parseRolesClaim([]any{"org_admin", map[string]any{}}); err == nil {
		t.Fatal("expected roles parse error")
	}
}

func TestDevelopmentModeAndSecretHelpers(t *testing.T) {
	t.Setenv("DEV_MODE", "true")
	if !isDevModeEnabled() {
		t.Fatal("expected development mode enabled")
	}

	t.Setenv("DEV_MODE", "invalid")
	if isDevModeEnabled() {
		t.Fatal("expected invalid dev mode value to be treated as disabled")
	}

	if _, err := generateJWTSecret(0); err == nil {
		t.Fatal("expected error for invalid secret size")
	}
	secret, err := generateJWTSecret(16)
	if err != nil {
		t.Fatalf("generate secret: %v", err)
	}
	if secret == "" {
		t.Fatal("expected generated secret")
	}
}

func makeTestJWT(t *testing.T, secret string, claims map[string]any) string {
	t.Helper()
	return makeTestJWTWithHeader(t, secret, map[string]any{
		"alg": "HS256",
		"typ": "JWT",
	}, claims)
}

func makeTestJWTWithHeader(t *testing.T, secret string, header map[string]any, claims map[string]any) string {
	t.Helper()

	headerJSON, err := json.Marshal(header)
	if err != nil {
		t.Fatalf("marshal header: %v", err)
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}

	headerSegment := base64.RawURLEncoding.EncodeToString(headerJSON)
	claimsSegment := base64.RawURLEncoding.EncodeToString(claimsJSON)
	signature := signJWT(headerSegment+"."+claimsSegment, []byte(secret))
	signatureSegment := base64.RawURLEncoding.EncodeToString(signature)
	return headerSegment + "." + claimsSegment + "." + signatureSegment
}
