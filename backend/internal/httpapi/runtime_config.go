package httpapi

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	envDevMode            = "DEV_MODE"
	envProductionMode     = "PRODUCTION_MODE"
	envCORSAllowedOrigins = "PLATO_CORS_ALLOWED_ORIGINS"
)

type RuntimeMode string

const (
	RuntimeModeDevelopment RuntimeMode = "development"
	RuntimeModeProduction  RuntimeMode = "production"
)

type RuntimeConfig struct {
	Mode               RuntimeMode
	CORSAllowedOrigins []string
	AllowAnyCORSOrigin bool
}

func (m RuntimeMode) IsDevelopment() bool {
	return m == RuntimeModeDevelopment
}

func (m RuntimeMode) IsProduction() bool {
	return m == RuntimeModeProduction
}

func DefaultListenAddr(mode RuntimeMode) string {
	if mode.IsDevelopment() {
		return "127.0.0.1:8070"
	}
	return ":8070"
}

func LoadRuntimeConfigFromEnv() (RuntimeConfig, error) {
	mode, err := runtimeModeFromEnv()
	if err != nil {
		return RuntimeConfig{}, err
	}

	allowedOrigins := parseCSV(os.Getenv(envCORSAllowedOrigins))
	if mode.IsProduction() {
		for _, origin := range allowedOrigins {
			if origin == "*" {
				return RuntimeConfig{}, fmt.Errorf("%s cannot include wildcard origin in production mode", envCORSAllowedOrigins)
			}
		}
		return RuntimeConfig{
			Mode:               mode,
			CORSAllowedOrigins: allowedOrigins,
		}, nil
	}

	if len(allowedOrigins) == 0 {
		return RuntimeConfig{
			Mode:               mode,
			CORSAllowedOrigins: []string{"*"},
			AllowAnyCORSOrigin: true,
		}, nil
	}
	for _, origin := range allowedOrigins {
		if origin == "*" {
			return RuntimeConfig{
				Mode:               mode,
				CORSAllowedOrigins: []string{"*"},
				AllowAnyCORSOrigin: true,
			}, nil
		}
	}

	return RuntimeConfig{
		Mode:               mode,
		CORSAllowedOrigins: allowedOrigins,
	}, nil
}

func runtimeModeFromEnv() (RuntimeMode, error) {
	devMode, _, err := parseOptionalBoolEnv(envDevMode)
	if err != nil {
		return "", err
	}
	productionMode, _, err := parseOptionalBoolEnv(envProductionMode)
	if err != nil {
		return "", err
	}
	if devMode && productionMode {
		return "", fmt.Errorf("%s and %s cannot both be true", envDevMode, envProductionMode)
	}
	if devMode {
		return RuntimeModeDevelopment, nil
	}
	if productionMode {
		return RuntimeModeProduction, nil
	}

	return RuntimeModeProduction, nil
}

func parseOptionalBoolEnv(key string) (value bool, set bool, err error) {
	rawValue, exists := os.LookupEnv(key)
	if !exists {
		return false, false, nil
	}
	trimmedValue := strings.TrimSpace(rawValue)
	if trimmedValue == "" {
		return false, false, nil
	}
	parsedValue, parseErr := strconv.ParseBool(trimmedValue)
	if parseErr != nil {
		return false, true, fmt.Errorf("%s must be a boolean value: %w", key, parseErr)
	}
	return parsedValue, true, nil
}

func parseCSV(rawValue string) []string {
	parts := strings.Split(rawValue, ",")
	values := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, part := range parts {
		trimmedPart := strings.TrimSpace(part)
		if trimmedPart == "" {
			continue
		}
		if _, exists := seen[trimmedPart]; exists {
			continue
		}
		seen[trimmedPart] = struct{}{}
		values = append(values, trimmedPart)
	}
	return values
}
