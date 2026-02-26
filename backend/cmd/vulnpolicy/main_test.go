//go:build tools

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

type fakeSeverityResolver struct {
	byID  map[string]severityAssessment
	errID map[string]error
	calls []string
}

func (resolver *fakeSeverityResolver) Resolve(_ context.Context, vuln vulnAssessment) (severityAssessment, error) {
	resolver.calls = append(resolver.calls, vuln.ID)
	if assessment, ok := resolver.byID[vuln.ID]; ok {
		return assessment, resolver.errID[vuln.ID]
	}
	if err := resolver.errID[vuln.ID]; err != nil {
		return severityAssessment{Severity: severityUnknown}, err
	}
	return severityAssessment{Severity: severityUnknown}, nil
}

func TestParseGovulncheckOutput(t *testing.T) {
	t.Parallel()
	input := strings.Join([]string{
		`{"osv":{"id":"GO-1","aliases":["CVE-1","CVE-1"],"summary":"first vuln","database_specific":{"url":"https://example.test/GO-1"}}}`,
		`{"finding":{"osv":"GO-1","fixed_version":"v1.2.3","trace":[{}]}}`,
		`{"finding":{"osv":"GO-1","fixed_version":"v1.2.4","trace":[{"package":"crypto/tls","function":"HandshakeContext"},{"package":"plato/backend/cmd/plato","function":"main"}]}}`,
		`{"finding":{"osv":"GO-2","fixed_version":"v2.0.0","trace":[{"package":"net/url"}]}}`,
		`{"osv":{"id":"GO-2","aliases":["CVE-2"],"summary":"second vuln","database_specific":{"url":"https://example.test/GO-2"}}}`,
	}, "\n")

	vulns, err := parseGovulncheckOutput(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parseGovulncheckOutput returned error: %v", err)
	}

	if len(vulns) != 2 {
		t.Fatalf("expected 2 vulnerabilities, got %d", len(vulns))
	}

	first := vulns[0]
	if first.ID != "GO-1" {
		t.Fatalf("expected first vulnerability GO-1, got %s", first.ID)
	}
	if !first.Reachable {
		t.Fatal("expected GO-1 to be reachable")
	}
	if !reflect.DeepEqual(first.Aliases, []string{"CVE-1"}) {
		t.Fatalf("unexpected aliases for GO-1: %#v", first.Aliases)
	}
	if !reflect.DeepEqual(first.FixedVersions, []string{"v1.2.3", "v1.2.4"}) {
		t.Fatalf("unexpected fixed versions for GO-1: %#v", first.FixedVersions)
	}

	second := vulns[1]
	if second.ID != "GO-2" {
		t.Fatalf("expected second vulnerability GO-2, got %s", second.ID)
	}
	if second.Reachable {
		t.Fatal("expected GO-2 to be not reachable")
	}
}

func TestLoadOverrides(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "overrides.json")

	validContent := `{
  "overrides": [
    {
      "id": "go-2026-4340",
      "reason": "accepted short term until toolchain update",
      "expires_on": "2026-03-15"
    }
  ]
}`
	if err := os.WriteFile(path, []byte(validContent), 0o644); err != nil {
		t.Fatalf("write override file: %v", err)
	}

	overrides, err := loadOverrides(path)
	if err != nil {
		t.Fatalf("loadOverrides returned error: %v", err)
	}

	override, ok := overrides["GO-2026-4340"]
	if !ok {
		t.Fatalf("expected GO-2026-4340 override")
	}
	if override.Reason == "" {
		t.Fatal("expected non-empty reason")
	}
	if override.ExpiresOn.Format("2006-01-02") != "2026-03-15" {
		t.Fatalf("unexpected expiry date: %s", override.ExpiresOn.Format("2006-01-02"))
	}

	t.Run("missing expires_on", func(t *testing.T) {
		invalidPath := filepath.Join(tempDir, "invalid-missing-expires.json")
		invalidContent := `{"overrides":[{"id":"GO-1","reason":"x"}]}`
		if err := os.WriteFile(invalidPath, []byte(invalidContent), 0o644); err != nil {
			t.Fatalf("write invalid override file: %v", err)
		}
		if _, err := loadOverrides(invalidPath); err == nil {
			t.Fatal("expected error for missing expires_on")
		}
	})

	t.Run("duplicate override IDs", func(t *testing.T) {
		invalidPath := filepath.Join(tempDir, "invalid-duplicate.json")
		invalidContent := `{
  "overrides": [
    {"id": "GO-1", "reason": "a", "expires_on": "2026-03-01"},
    {"id": "go-1", "reason": "b", "expires_on": "2026-03-10"}
  ]
}`
		if err := os.WriteFile(invalidPath, []byte(invalidContent), 0o644); err != nil {
			t.Fatalf("write invalid override file: %v", err)
		}
		if _, err := loadOverrides(invalidPath); err == nil {
			t.Fatal("expected error for duplicate override IDs")
		}
	})

	t.Run("missing reason", func(t *testing.T) {
		invalidPath := filepath.Join(tempDir, "invalid-missing-reason.json")
		invalidContent := `{"overrides":[{"id":"GO-1","expires_on":"2026-03-01"}]}`
		if err := os.WriteFile(invalidPath, []byte(invalidContent), 0o644); err != nil {
			t.Fatalf("write invalid override file: %v", err)
		}
		if _, err := loadOverrides(invalidPath); err == nil {
			t.Fatal("expected error for missing reason")
		}
	})
}

func TestEvaluateVulnerabilities(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.February, 22, 12, 0, 0, 0, time.UTC)

	vulns := []vulnAssessment{
		{ID: "GO-A", Reachable: true},
		{ID: "GO-B", Reachable: true},
		{ID: "GO-C", Reachable: false},
		{ID: "GO-D", Reachable: true},
		{ID: "GO-E", Reachable: true, Aliases: []string{"CVE-E"}},
	}

	overrides := map[string]riskOverride{
		"GO-D": {
			ID:        "GO-D",
			Reason:    "accepted while patch in progress",
			ExpiresOn: time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC),
		},
		"CVE-E": {
			ID:        "CVE-E",
			Reason:    "temporary acceptance",
			ExpiresOn: time.Date(2026, time.January, 15, 0, 0, 0, 0, time.UTC),
		},
	}

	resolver := &fakeSeverityResolver{
		byID: map[string]severityAssessment{
			"GO-A": {Severity: severityHigh, Score: 8.1},
			"GO-B": {Severity: severityMedium, Score: 5.0},
		},
		errID: map[string]error{
			"GO-A": errors.New("resolver partial warning"),
		},
	}

	result := evaluateVulnerabilities(context.Background(), vulns, overrides, resolver, now)

	if len(result.Fail) != 1 || result.Fail[0].Vuln.ID != "GO-A" {
		t.Fatalf("unexpected fail list: %#v", result.Fail)
	}
	if len(result.Warn) != 1 || result.Warn[0].Vuln.ID != "GO-B" {
		t.Fatalf("unexpected warn list: %#v", result.Warn)
	}
	if len(result.Info) != 1 || result.Info[0].Vuln.ID != "GO-C" {
		t.Fatalf("unexpected info list: %#v", result.Info)
	}
	if len(result.Accepted) != 1 || result.Accepted[0].Vuln.ID != "GO-D" {
		t.Fatalf("unexpected accepted list: %#v", result.Accepted)
	}
	if len(result.Expired) != 1 || result.Expired[0].Vuln.ID != "GO-E" {
		t.Fatalf("unexpected expired list: %#v", result.Expired)
	}

	if !reflect.DeepEqual(resolver.calls, []string{"GO-A", "GO-B"}) {
		t.Fatalf("unexpected resolver calls: %#v", resolver.calls)
	}
}

func TestCollectCVEIDs(t *testing.T) {
	t.Parallel()
	vuln := vulnAssessment{
		ID:      "go-1234",
		Aliases: []string{"CVE-2026-1000", "cve-2026-1000", "GHSA-1", "CVE-2026-1001"},
	}
	actual := collectCVEIDs(vuln)
	expected := []string{"CVE-2026-1000", "CVE-2026-1001"}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("unexpected CVE IDs: got %#v want %#v", actual, expected)
	}
}

func TestBestNVDSeverity(t *testing.T) {
	t.Parallel()
	payload := nvdResponse{
		Vulnerabilities: []struct {
			CVE struct {
				Metrics struct {
					CVSSMetricV31 []nvdMetric `json:"cvssMetricV31"`
					CVSSMetricV30 []nvdMetric `json:"cvssMetricV30"`
					CVSSMetricV2  []nvdMetric `json:"cvssMetricV2"`
				} `json:"metrics"`
			} `json:"cve"`
		}{
			{},
		},
	}

	mediumMetric := nvdMetric{}
	mediumMetric.CVSSData.BaseScore = 5.1
	mediumMetric.CVSSData.BaseSeverity = "MEDIUM"

	highMetric := nvdMetric{}
	highMetric.CVSSData.BaseScore = 7.8
	highMetric.CVSSData.BaseSeverity = "HIGH"

	payload.Vulnerabilities[0].CVE.Metrics.CVSSMetricV31 = []nvdMetric{mediumMetric}
	payload.Vulnerabilities[0].CVE.Metrics.CVSSMetricV30 = []nvdMetric{highMetric}

	severityValue, score := bestNVDSeverity(payload)
	if severityValue != severityHigh {
		t.Fatalf("expected HIGH severity, got %s", severityValue)
	}
	if score != 7.8 {
		t.Fatalf("expected score 7.8, got %.1f", score)
	}
}

func TestNormalizeSeverity(t *testing.T) {
	t.Parallel()
	if normalizeSeverity("", 9.2) != severityCritical {
		t.Fatal("expected score fallback to CRITICAL")
	}
}

func TestResolveNVDAPIKey(t *testing.T) {
	t.Run("from file", func(t *testing.T) {
		t.Setenv("NVD_API_KEY", "from-env")
		tempDir := t.TempDir()
		apiKeyPath := filepath.Join(tempDir, "nvd.key")
		if err := os.WriteFile(apiKeyPath, []byte("from-file\n"), 0o600); err != nil {
			t.Fatalf("write key file: %v", err)
		}

		apiKey, err := resolveNVDAPIKey(apiKeyPath)
		if err != nil {
			t.Fatalf("resolveNVDAPIKey returned error: %v", err)
		}
		if apiKey != "from-file" {
			t.Fatalf("unexpected api key: %q", apiKey)
		}
	})

	t.Run("empty file fails", func(t *testing.T) {
		tempDir := t.TempDir()
		apiKeyPath := filepath.Join(tempDir, "empty.key")
		if err := os.WriteFile(apiKeyPath, []byte("\n"), 0o600); err != nil {
			t.Fatalf("write key file: %v", err)
		}

		if _, err := resolveNVDAPIKey(apiKeyPath); err == nil {
			t.Fatal("expected error for empty key file")
		}
	})

	t.Run("fallback to env", func(t *testing.T) {
		t.Setenv("NVD_API_KEY", "from-env")
		apiKey, err := resolveNVDAPIKey("")
		if err != nil {
			t.Fatalf("resolveNVDAPIKey returned error: %v", err)
		}
		if apiKey != "from-env" {
			t.Fatalf("unexpected api key: %q", apiKey)
		}
	})
}

func TestLoadSeveritySnapshot(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()

	t.Run("loads valid snapshot", func(t *testing.T) {
		path := filepath.Join(tempDir, "snapshot.json")
		content := `{
  "cves": {
    "CVE-2026-1000": {"severity": "HIGH", "score": 8.1},
    "cve-2026-1001": {"severity": "", "score": 4.5}
  }
}`
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write snapshot file: %v", err)
		}

		snapshot, err := loadSeveritySnapshot(path)
		if err != nil {
			t.Fatalf("loadSeveritySnapshot returned error: %v", err)
		}

		high := snapshot["CVE-2026-1000"]
		if high.Severity != severityHigh || high.Score != 8.1 {
			t.Fatalf("unexpected severity for CVE-2026-1000: %#v", high)
		}

		medium := snapshot["CVE-2026-1001"]
		if medium.Severity != severityMedium || medium.Score != 4.5 {
			t.Fatalf("unexpected severity for CVE-2026-1001: %#v", medium)
		}
	})

	t.Run("invalid id fails", func(t *testing.T) {
		path := filepath.Join(tempDir, "invalid.json")
		content := `{"cves":{"GHSA-123":{"severity":"HIGH","score":8.0}}}`
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write snapshot file: %v", err)
		}

		if _, err := loadSeveritySnapshot(path); err == nil {
			t.Fatal("expected invalid id error")
		}
	})
}

func TestResolveCVEOfflineUsesSnapshot(t *testing.T) {
	t.Parallel()

	resolver := &nvdSeverityResolver{
		offline: true,
		snapshot: map[string]severityAssessment{
			"CVE-2026-1000": {
				Severity: severityHigh,
				Score:    7.5,
				Source:   "CVE-2026-1000",
			},
		},
		cache:    make(map[string]severityAssessment),
		errorMap: make(map[string]error),
	}

	fromSnapshot, err := resolver.resolveCVE(context.Background(), "CVE-2026-1000")
	if err != nil {
		t.Fatalf("resolveCVE returned error for snapshot hit: %v", err)
	}
	if fromSnapshot.Severity != severityHigh || fromSnapshot.Score != 7.5 {
		t.Fatalf("unexpected snapshot assessment: %#v", fromSnapshot)
	}

	missing, missingErr := resolver.resolveCVE(context.Background(), "CVE-2026-1001")
	if missingErr == nil {
		t.Fatal("expected error for missing snapshot CVE in offline mode")
	}
	if missing.Severity != severityUnknown {
		t.Fatalf("expected UNKNOWN severity in offline missing case, got %#v", missing)
	}
	if !strings.Contains(missingErr.Error(), "missing from severity snapshot") {
		t.Fatalf("unexpected missing snapshot error: %v", missingErr)
	}

	cached, cachedErr := resolver.resolveCVE(context.Background(), "CVE-2026-1001")
	if cachedErr == nil {
		t.Fatal("expected cached error for missing snapshot CVE")
	}
	if !errors.Is(cachedErr, missingErr) && cachedErr.Error() != missingErr.Error() {
		t.Fatalf("expected cached error to match original error: got %v want %v", cachedErr, missingErr)
	}
	if cached.Severity != severityUnknown {
		t.Fatalf("expected cached UNKNOWN severity, got %#v", cached)
	}

	if got := fmt.Sprintf("%s", cached.Source); got != "CVE-2026-1001" {
		t.Fatalf("unexpected cached source: %s", got)
	}
}
