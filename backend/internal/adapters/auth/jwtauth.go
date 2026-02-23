package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"plato/backend/internal/ports"
)

const (
	headerAuthorization = "Authorization"
	bearerPrefix        = "Bearer "
)

const generatedDevJWTSecretBytes = 48

type JWTAuthProvider struct {
	signingKey []byte
	now        func() time.Time
}

func NewJWTAuthProviderFromEnv() (*JWTAuthProvider, error) {
	configuredEnvKey, signingKey := jwtSigningKeyFromEnv()
	secret := signingKey
	if secret == "" {
		if !isDevModeEnabled() {
			return nil, fmt.Errorf("%s is required in production mode", configuredEnvKey)
		}

		generatedSecret, err := generateJWTSecret(generatedDevJWTSecretBytes)
		if err != nil {
			return nil, fmt.Errorf("generate development jwt secret: %w", err)
		}
		secret = generatedSecret
	}
	return NewJWTAuthProvider(secret)
}

func NewJWTAuthProvider(secret string) (*JWTAuthProvider, error) {
	trimmedSecret := strings.TrimSpace(secret)
	if trimmedSecret == "" {
		return nil, errors.New("jwt secret is required")
	}

	return &JWTAuthProvider{
		signingKey: []byte(trimmedSecret),
		now:        time.Now,
	}, nil
}

func (p *JWTAuthProvider) FromRequest(r *http.Request) (ports.AuthContext, error) {
	if p == nil {
		return ports.AuthContext{}, errors.New("auth provider is nil")
	}

	authorizationParts := strings.Fields(strings.TrimSpace(r.Header.Get(headerAuthorization)))
	if len(authorizationParts) == 0 || !strings.EqualFold(authorizationParts[0], strings.TrimSpace(bearerPrefix)) {
		return ports.AuthContext{}, errors.New("missing bearer token")
	}
	if len(authorizationParts) == 1 {
		return ports.AuthContext{}, errors.New("empty bearer token")
	}
	if len(authorizationParts) > 2 {
		return ports.AuthContext{}, errors.New("invalid bearer token format")
	}

	token := strings.TrimSpace(authorizationParts[1])
	if token == "" {
		return ports.AuthContext{}, errors.New("empty bearer token")
	}

	claims, err := p.parseAndValidateToken(token)
	if err != nil {
		return ports.AuthContext{}, err
	}

	userID := claimString(claims, "sub")
	if userID == "" {
		userID = claimString(claims, "user_id")
	}
	if userID == "" {
		return ports.AuthContext{}, errors.New("token subject is required")
	}

	organisationID := claimString(claims, "org_id")
	if organisationID == "" {
		organisationID = claimString(claims, "organisation_id")
	}

	roles, err := parseRolesClaim(claims["roles"])
	if err != nil {
		return ports.AuthContext{}, err
	}
	if len(roles) == 0 {
		return ports.AuthContext{}, errors.New("token roles are required")
	}

	return ports.AuthContext{
		UserID:         userID,
		OrganisationID: organisationID,
		Roles:          roles,
	}, nil
}

func (p *JWTAuthProvider) parseAndValidateToken(token string) (map[string]any, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("token must have three segments")
	}

	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("decode token header: %w", err)
	}
	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode token payload: %w", err)
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, fmt.Errorf("decode token signature: %w", err)
	}

	var header map[string]any
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return nil, fmt.Errorf("parse token header: %w", err)
	}
	if claimString(header, "alg") != "HS256" {
		return nil, errors.New("token alg must be HS256")
	}

	expectedSignature := signJWT(parts[0]+"."+parts[1], p.signingKey)
	if !hmac.Equal(signature, expectedSignature) {
		return nil, errors.New("token signature is invalid")
	}

	var claims map[string]any
	if err := json.Unmarshal(payloadJSON, &claims); err != nil {
		return nil, fmt.Errorf("parse token payload: %w", err)
	}

	now := p.now().UTC().Unix()
	if err := validateExpiration(claims, now); err != nil {
		return nil, err
	}
	if err := validateNotBefore(claims, now); err != nil {
		return nil, err
	}

	return claims, nil
}

func validateExpiration(claims map[string]any, nowUnixSeconds int64) error {
	expirationClaim, exists := claims["exp"]
	if !exists {
		return errors.New("token exp claim is required")
	}
	expirationUnixSeconds, err := parseNumericDateClaim(expirationClaim)
	if err != nil {
		return fmt.Errorf("token exp claim is invalid: %w", err)
	}
	if nowUnixSeconds >= expirationUnixSeconds {
		return errors.New("token is expired")
	}
	return nil
}

func validateNotBefore(claims map[string]any, nowUnixSeconds int64) error {
	notBeforeClaim, exists := claims["nbf"]
	if !exists {
		return nil
	}
	notBeforeUnixSeconds, err := parseNumericDateClaim(notBeforeClaim)
	if err != nil {
		return fmt.Errorf("token nbf claim is invalid: %w", err)
	}
	if nowUnixSeconds < notBeforeUnixSeconds {
		return errors.New("token is not valid yet")
	}
	return nil
}

func parseNumericDateClaim(value any) (int64, error) {
	switch typedValue := value.(type) {
	case float64:
		return int64(typedValue), nil
	case int64:
		return typedValue, nil
	case int:
		return int64(typedValue), nil
	case json.Number:
		return typedValue.Int64()
	case string:
		parsedValue, err := strconv.ParseInt(strings.TrimSpace(typedValue), 10, 64)
		if err != nil {
			return 0, err
		}
		return parsedValue, nil
	default:
		return 0, fmt.Errorf("unsupported claim type %T", value)
	}
}

func parseRolesClaim(value any) ([]string, error) {
	switch typedValue := value.(type) {
	case nil:
		return nil, nil
	case string:
		return parseJWTCommaSeparatedRoles(typedValue), nil
	case []any:
		roles := make([]string, 0, len(typedValue))
		for _, entry := range typedValue {
			role, ok := entry.(string)
			if !ok {
				return nil, errors.New("token roles must be strings")
			}
			trimmedRole := strings.TrimSpace(role)
			if trimmedRole == "" {
				continue
			}
			roles = append(roles, trimmedRole)
		}
		return roles, nil
	default:
		return nil, errors.New("token roles claim has unsupported type")
	}
}

func parseJWTCommaSeparatedRoles(rawValue string) []string {
	parts := strings.Split(rawValue, ",")
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

func claimString(claims map[string]any, claimName string) string {
	rawValue, exists := claims[claimName]
	if !exists {
		return ""
	}
	stringValue, ok := rawValue.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(stringValue)
}

func signJWT(payload string, signingKey []byte) []byte {
	mac := hmac.New(sha256.New, signingKey)
	mac.Write([]byte(payload))
	return mac.Sum(nil)
}

func isDevModeEnabled() bool {
	rawValue := strings.TrimSpace(os.Getenv("DEV_MODE"))
	if rawValue == "" {
		return false
	}

	devModeEnabled, err := strconv.ParseBool(rawValue)
	if err != nil {
		return false
	}

	return devModeEnabled
}

func jwtSigningKeyFromEnv() (string, string) {
	const currentEnvVarName = "PLATO_AUTH_JWT_HS256_SIGNING_KEY"
	const legacyEnvVarName = "PLATO_AUTH_JWT_HS256_SECRET"

	signingKey := strings.TrimSpace(os.Getenv(currentEnvVarName))
	if signingKey != "" {
		return currentEnvVarName, signingKey
	}

	legacySigningKey := strings.TrimSpace(os.Getenv(legacyEnvVarName))
	if legacySigningKey != "" {
		return legacyEnvVarName, legacySigningKey
	}

	return currentEnvVarName, ""
}

func generateJWTSecret(size int) (string, error) {
	if size <= 0 {
		return "", errors.New("secret size must be positive")
	}

	randomBytes := make([]byte, size)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}

	return hex.EncodeToString(randomBytes), nil
}
