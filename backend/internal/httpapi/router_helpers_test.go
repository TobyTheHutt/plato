package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSetCORS(t *testing.T) {
	t.Run("wildcard policy", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "/api/organisations", http.NoBody)
		recorder := httptest.NewRecorder()
		setCORS(recorder, request, corsPolicy{
			allowAnyOrigin: true,
			allowHeaders:   "Content-Type",
			allowMethods:   "GET",
		})

		if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "*" {
			t.Fatalf("expected wildcard origin header, got %q", got)
		}
	})

	t.Run("allowlisted origin", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "/api/organisations", http.NoBody)
		request.Header.Set("Origin", "https://app.example.com")
		recorder := httptest.NewRecorder()
		setCORS(recorder, request, corsPolicy{
			allowedOrigins: map[string]struct{}{
				"https://app.example.com": {},
			},
			allowHeaders: "Content-Type",
			allowMethods: "GET",
		})

		if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
			t.Fatalf("expected allowlisted origin header, got %q", got)
		}
		if got := recorder.Header().Get("Vary"); got != "Origin" {
			t.Fatalf("expected Vary header for origin, got %q", got)
		}
	})

	t.Run("blocked origin", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "/api/organisations", http.NoBody)
		request.Header.Set("Origin", "https://blocked.example.com")
		recorder := httptest.NewRecorder()
		setCORS(recorder, request, corsPolicy{
			allowedOrigins: map[string]struct{}{
				"https://app.example.com": {},
			},
			allowHeaders: "Content-Type",
			allowMethods: "GET",
		})

		if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Fatalf("expected no allow-origin header for blocked origin, got %q", got)
		}
	})
}
