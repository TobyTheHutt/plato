package httpapi

import (
	"reflect"
	"testing"
)

func TestLoadRuntimeConfigFromEnvDefaultsToProductionMode(t *testing.T) {
	t.Setenv(envDevMode, "")
	t.Setenv(envProductionMode, "")
	t.Setenv(envCORSAllowedOrigins, "")

	config, err := LoadRuntimeConfigFromEnv()
	if err != nil {
		t.Fatalf("load runtime config: %v", err)
	}
	if !config.Mode.IsProduction() {
		t.Fatalf("expected production mode, got %s", config.Mode)
	}
	if config.AllowAnyCORSOrigin {
		t.Fatal("expected no wildcard CORS in production mode")
	}
	if len(config.CORSAllowedOrigins) != 0 {
		t.Fatalf("expected empty production CORS allowlist by default, got %v", config.CORSAllowedOrigins)
	}
}

func TestLoadRuntimeConfigFromEnvDevelopmentModeEnablesWildcardCORS(t *testing.T) {
	t.Setenv(envDevMode, "true")
	t.Setenv(envProductionMode, "")
	t.Setenv(envCORSAllowedOrigins, "")

	config, err := LoadRuntimeConfigFromEnv()
	if err != nil {
		t.Fatalf("load runtime config: %v", err)
	}
	if !config.Mode.IsDevelopment() {
		t.Fatalf("expected development mode, got %s", config.Mode)
	}
	if !config.AllowAnyCORSOrigin {
		t.Fatal("expected wildcard CORS for development mode defaults")
	}
}

func TestLoadRuntimeConfigFromEnvProductionModeParsesAllowlist(t *testing.T) {
	t.Setenv(envDevMode, "")
	t.Setenv(envProductionMode, "true")
	t.Setenv(envCORSAllowedOrigins, "https://app.example.com, https://admin.example.com, https://app.example.com")

	config, err := LoadRuntimeConfigFromEnv()
	if err != nil {
		t.Fatalf("load runtime config: %v", err)
	}
	if !config.Mode.IsProduction() {
		t.Fatalf("expected production mode, got %s", config.Mode)
	}
	if len(config.CORSAllowedOrigins) != 2 {
		t.Fatalf("expected two unique allowed origins, got %v", config.CORSAllowedOrigins)
	}
	expectedOrigins := []string{"https://app.example.com", "https://admin.example.com"}
	if !reflect.DeepEqual(config.CORSAllowedOrigins, expectedOrigins) {
		t.Fatalf("expected production allowlist %v, got %v", expectedOrigins, config.CORSAllowedOrigins)
	}
	if config.AllowAnyCORSOrigin {
		t.Fatal("expected allowAnyCORSOrigin to be false in production mode")
	}
}

func TestLoadRuntimeConfigFromEnvProductionModeRejectsWildcardOrigin(t *testing.T) {
	t.Setenv(envDevMode, "")
	t.Setenv(envProductionMode, "true")
	t.Setenv(envCORSAllowedOrigins, "*")

	if _, err := LoadRuntimeConfigFromEnv(); err == nil {
		t.Fatal("expected wildcard origin error in production mode")
	}
}

func TestLoadRuntimeConfigFromEnvRejectsInvalidBooleanValues(t *testing.T) {
	t.Setenv(envDevMode, "nope")
	t.Setenv(envProductionMode, "")
	t.Setenv(envCORSAllowedOrigins, "")

	if _, err := LoadRuntimeConfigFromEnv(); err == nil {
		t.Fatal("expected boolean parse error")
	}
}

func TestLoadRuntimeConfigFromEnvRejectsConflictingModeBooleans(t *testing.T) {
	t.Setenv(envDevMode, "true")
	t.Setenv(envProductionMode, "true")
	t.Setenv(envCORSAllowedOrigins, "")

	if _, err := LoadRuntimeConfigFromEnv(); err == nil {
		t.Fatal("expected conflicting mode error")
	}
}

func TestDefaultListenAddr(t *testing.T) {
	if got := DefaultListenAddr(RuntimeModeDevelopment); got != "127.0.0.1:8070" {
		t.Fatalf("unexpected development default listen addr: %s", got)
	}
	if got := DefaultListenAddr(RuntimeModeProduction); got != ":8070" {
		t.Fatalf("unexpected production default listen addr: %s", got)
	}
}
