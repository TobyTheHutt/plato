package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

const (
	testOrganisationsPath = "/api/organisations"
	testAppOrigin         = "https://app.example.com"
)

func TestSetCORS(t *testing.T) {
	t.Run("wildcard policy", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, testOrganisationsPath, http.NoBody)
		recorder := httptest.NewRecorder()
		setCORS(recorder, request, corsPolicy{
			allowAnyOrigin: true,
			allowHeaders:   "Content-Type",
			allowMethods:   "GET",
		})

		if got := recorder.Header().Get(headerAccessControlAllowOrigin); got != "*" {
			t.Fatalf("expected wildcard origin header, got %q", got)
		}
	})

	t.Run("allowlisted origin", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, testOrganisationsPath, http.NoBody)
		request.Header.Set(headerOrigin, testAppOrigin)
		recorder := httptest.NewRecorder()
		setCORS(recorder, request, corsPolicy{
			allowedOrigins: map[string]struct{}{
				testAppOrigin: {},
			},
			allowHeaders: "Content-Type",
			allowMethods: "GET",
		})

		if got := recorder.Header().Get(headerAccessControlAllowOrigin); got != testAppOrigin {
			t.Fatalf("expected allowlisted origin header, got %q", got)
		}
		if got := recorder.Header().Get("Vary"); got != headerOrigin {
			t.Fatalf("expected Vary header for origin, got %q", got)
		}
	})

	t.Run("blocked origin", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, testOrganisationsPath, http.NoBody)
		request.Header.Set(headerOrigin, "https://blocked.example.com")
		recorder := httptest.NewRecorder()
		setCORS(recorder, request, corsPolicy{
			allowedOrigins: map[string]struct{}{
				testAppOrigin: {},
			},
			allowHeaders: "Content-Type",
			allowMethods: "GET",
		})

		if got := recorder.Header().Get(headerAccessControlAllowOrigin); got != "" {
			t.Fatalf("expected no allow-origin header for blocked origin, got %q", got)
		}
	})
}
