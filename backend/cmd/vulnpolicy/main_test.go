//go:build tools

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
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

func TestParseGovulncheckOutputWithModeBinaryFindingsAreReachable(t *testing.T) {
	t.Parallel()

	input := `{"osv":{"id":"GO-1","summary":"binary finding"}}` + "\n" +
		`{"finding":{"osv":"GO-1","trace":[{}]}}`

	fromSource, sourceErr := parseGovulncheckOutputWithMode(strings.NewReader(input), scanModeSource)
	if sourceErr != nil {
		t.Fatalf("parseGovulncheckOutputWithMode source returned error: %v", sourceErr)
	}
	if len(fromSource) != 1 {
		t.Fatalf("expected one vulnerability in source mode, got %d", len(fromSource))
	}
	if fromSource[0].Reachable {
		t.Fatal("expected source mode finding with empty trace details to remain not reachable")
	}

	fromBinary, binaryErr := parseGovulncheckOutputWithMode(strings.NewReader(input), scanModeBinary)
	if binaryErr != nil {
		t.Fatalf("parseGovulncheckOutputWithMode binary returned error: %v", binaryErr)
	}
	if len(fromBinary) != 1 {
		t.Fatalf("expected one vulnerability in binary mode, got %d", len(fromBinary))
	}
	if !fromBinary[0].Reachable {
		t.Fatal("expected binary mode finding to be treated as reachable")
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
	if err := os.WriteFile(path, []byte(validContent), 0o600); err != nil {
		t.Fatalf("write override file: %v", err)
	}

	overrides, overridesErr := loadOverrides(path)
	if overridesErr != nil {
		t.Fatalf("loadOverrides returned error: %v", overridesErr)
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
		if err := os.WriteFile(invalidPath, []byte(invalidContent), 0o600); err != nil {
			t.Fatalf("write invalid override file: %v", err)
		}
		if _, loadErr := loadOverrides(invalidPath); loadErr == nil {
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
		if err := os.WriteFile(invalidPath, []byte(invalidContent), 0o600); err != nil {
			t.Fatalf("write invalid override file: %v", err)
		}
		if _, loadErr := loadOverrides(invalidPath); loadErr == nil {
			t.Fatal("expected error for duplicate override IDs")
		}
	})

	t.Run("missing reason", func(t *testing.T) {
		invalidPath := filepath.Join(tempDir, "invalid-missing-reason.json")
		invalidContent := `{"overrides":[{"id":"GO-1","expires_on":"2026-03-01"}]}`
		if err := os.WriteFile(invalidPath, []byte(invalidContent), 0o600); err != nil {
			t.Fatalf("write invalid override file: %v", err)
		}
		if _, loadErr := loadOverrides(invalidPath); loadErr == nil {
			t.Fatal("expected error for missing reason")
		}
	})
}

func TestNormalizeScanMode(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "source", input: "source", want: scanModeSource},
		{name: "binary uppercase", input: "BINARY", want: scanModeBinary},
		{name: "trim", input: " source ", want: scanModeSource},
		{name: "invalid", input: "extract", wantErr: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got, err := normalizeScanMode(testCase.input)
			if testCase.wantErr {
				if err == nil {
					t.Fatalf("expected error for input %q", testCase.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeScanMode returned error for %q: %v", testCase.input, err)
			}
			if got != testCase.want {
				t.Fatalf("normalizeScanMode(%q) = %q, want %q", testCase.input, got, testCase.want)
			}
		})
	}
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

func TestCollectExcludedIDsAndFilterExcludedVulnerabilities(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	exclusionsPath := filepath.Join(tempDir, "source.json")
	exclusionsContent := strings.Join([]string{
		`{"osv":{"id":"GO-100","aliases":["CVE-2026-9000"]}}`,
		`{"finding":{"osv":"GO-100","trace":[{"package":"pkg","function":"f"}]}}`,
		`{"osv":{"id":"GO-UNREACHABLE","aliases":["CVE-2026-9999"]}}`,
	}, "\n")
	if err := os.WriteFile(exclusionsPath, []byte(exclusionsContent), 0o600); err != nil {
		t.Fatalf("write exclusions file: %v", err)
	}

	excludedIDs, err := collectExcludedIDs(exclusionsPath)
	if err != nil {
		t.Fatalf("collectExcludedIDs returned error: %v", err)
	}
	if _, exists := excludedIDs.all["GO-100"]; !exists {
		t.Fatal("expected GO-100 in exclusion set")
	}
	if _, exists := excludedIDs.all["CVE-2026-9000"]; !exists {
		t.Fatal("expected CVE alias in exclusion set")
	}
	if _, exists := excludedIDs.reachable["GO-100"]; !exists {
		t.Fatal("expected GO-100 in reachable exclusion set")
	}
	if _, exists := excludedIDs.all["GO-UNREACHABLE"]; !exists {
		t.Fatal("expected unreachable source vulnerability in all exclusion set")
	}
	if _, exists := excludedIDs.reachable["GO-UNREACHABLE"]; exists {
		t.Fatal("did not expect unreachable source vulnerability in reachable exclusion set")
	}

	vulns := []vulnAssessment{
		{ID: "GO-100", Reachable: true},
		{ID: "GO-UNREACHABLE"},
		{ID: "GO-UNREACHABLE", Reachable: true},
		{ID: "GO-200", Aliases: []string{"CVE-2026-9000"}, Reachable: true},
		{ID: "GO-300", Aliases: []string{"CVE-2026-9001"}},
	}
	filtered := filterExcludedVulnerabilities(vulns, excludedIDs)
	if len(filtered) != 2 || filtered[0].ID != "GO-UNREACHABLE" || !filtered[0].Reachable || filtered[1].ID != "GO-300" {
		t.Fatalf("unexpected filtered vulnerabilities: %#v", filtered)
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
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
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
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
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

	if got := cached.Source; got != "CVE-2026-1001" {
		t.Fatalf("unexpected cached source: %s", got)
	}
}

func TestParseGovulncheckOutputMalformedJSON(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name  string
		input string
	}{
		{
			name:  "truncated object",
			input: `{"osv":{"id":"GO-1"}`,
		},
		{
			name:  "invalid second line",
			input: `{"osv":{"id":"GO-1"}}` + "\n" + `{"finding":`,
		},
		{
			name:  "unexpected array",
			input: `[]`,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if _, err := parseGovulncheckOutput(strings.NewReader(testCase.input)); err == nil {
				t.Fatalf("expected parse error for %s", testCase.name)
			}
		})
	}
}

func TestNormalizeSeverityMatrix(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name  string
		raw   string
		score float64
		want  severity
	}{
		{name: "explicit critical", raw: "critical", score: 0.0, want: severityCritical},
		{name: "explicit high", raw: " HIGH ", score: 0.0, want: severityHigh},
		{name: "explicit medium", raw: "MeDiUm", score: 0.0, want: severityMedium},
		{name: "explicit low", raw: "low", score: 10.0, want: severityLow},
		{name: "score critical", raw: "", score: 9.0, want: severityCritical},
		{name: "score high", raw: "", score: 7.0, want: severityHigh},
		{name: "score medium", raw: "", score: 4.0, want: severityMedium},
		{name: "score low", raw: "", score: 0.1, want: severityLow},
		{name: "score unknown zero", raw: "", score: 0.0, want: severityUnknown},
		{name: "score unknown negative", raw: "", score: -1.0, want: severityUnknown},
		{name: "unknown text with score fallback", raw: "unknown-text", score: 8.2, want: severityHigh},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if got := normalizeSeverity(testCase.raw, testCase.score); got != testCase.want {
				t.Fatalf("normalizeSeverity(%q, %.1f) = %s, want %s", testCase.raw, testCase.score, got, testCase.want)
			}
		})
	}
}

func TestLoadOverridesErrorPaths(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	t.Run("missing file", func(t *testing.T) {
		t.Parallel()
		if _, loadErr := loadOverrides(filepath.Join(tempDir, "missing.json")); loadErr == nil {
			t.Fatal("expected missing file error")
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(tempDir, "invalid-json.json")
		if err := os.WriteFile(path, []byte(`{"overrides":[`), 0o600); err != nil {
			t.Fatalf("write invalid file: %v", err)
		}
		if _, loadErr := loadOverrides(path); loadErr == nil {
			t.Fatal("expected invalid json error")
		}
	})

	t.Run("missing id", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(tempDir, "missing-id.json")
		content := `{"overrides":[{"reason":"x","expires_on":"2026-03-01"}]}`
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("write invalid file: %v", err)
		}
		if _, loadErr := loadOverrides(path); loadErr == nil {
			t.Fatal("expected missing id error")
		}
	})

	t.Run("invalid expires_on format", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(tempDir, "invalid-date.json")
		content := `{"overrides":[{"id":"GO-1","reason":"x","expires_on":"03/01/2026"}]}`
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("write invalid file: %v", err)
		}
		if _, loadErr := loadOverrides(path); loadErr == nil {
			t.Fatal("expected invalid expires_on error")
		}
	})
}

func TestLoadSeveritySnapshotErrorPaths(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	t.Run("missing file", func(t *testing.T) {
		t.Parallel()
		if _, err := loadSeveritySnapshot(filepath.Join(tempDir, "missing-snapshot.json")); err == nil {
			t.Fatal("expected missing snapshot error")
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(tempDir, "snapshot-invalid.json")
		if err := os.WriteFile(path, []byte(`{"cves":`), 0o600); err != nil {
			t.Fatalf("write snapshot file: %v", err)
		}
		if _, err := loadSeveritySnapshot(path); err == nil {
			t.Fatal("expected invalid snapshot json error")
		}
	})
}

func TestEvaluateVulnerabilitiesUnknownSeverity(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.February, 22, 12, 0, 0, 0, time.UTC)
	vulns := []vulnAssessment{
		{ID: "GO-LOW", Reachable: true},
		{ID: "GO-UNKNOWN", Reachable: true},
	}

	resolver := &fakeSeverityResolver{
		byID: map[string]severityAssessment{
			"GO-LOW":     {Severity: severityLow, Score: 1.0},
			"GO-UNKNOWN": {Severity: severityUnknown},
		},
		errID: map[string]error{},
	}

	result := evaluateVulnerabilities(context.Background(), vulns, nil, resolver, now)

	if len(result.Warn) != 1 || result.Warn[0].Vuln.ID != "GO-LOW" {
		t.Fatalf("unexpected warn list: %#v", result.Warn)
	}
	if len(result.Fail) != 1 || result.Fail[0].Vuln.ID != "GO-UNKNOWN" {
		t.Fatalf("unexpected fail list: %#v", result.Fail)
	}
}

func TestSortEvaluated(t *testing.T) {
	t.Parallel()

	items := []evaluatedVuln{
		{Vuln: vulnAssessment{ID: "GO-3"}, Severity: severityAssessment{Severity: severityLow}},
		{Vuln: vulnAssessment{ID: "GO-1"}, Severity: severityAssessment{Severity: severityHigh}},
		{Vuln: vulnAssessment{ID: "GO-2"}, Severity: severityAssessment{Severity: severityHigh}},
		{Vuln: vulnAssessment{ID: "GO-4"}, Severity: severityAssessment{Severity: severityCritical}},
	}

	sortEvaluated(items)

	got := []string{items[0].Vuln.ID, items[1].Vuln.ID, items[2].Vuln.ID, items[3].Vuln.ID}
	want := []string{"GO-4", "GO-1", "GO-2", "GO-3"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected sort order: got %#v want %#v", got, want)
	}
}

func TestResolveUsesCachedCVEsAndJoinedErrors(t *testing.T) {
	t.Parallel()

	resolver := &nvdSeverityResolver{
		snapshot: map[string]severityAssessment{},
		cache: map[string]severityAssessment{
			"CVE-2026-1000": {Severity: severityMedium, Score: 6.0, Source: "CVE-2026-1000"},
			"CVE-2026-1001": {Severity: severityUnknown, Source: "CVE-2026-1001"},
			"CVE-2026-1002": {Severity: severityHigh, Score: 8.1, Source: "CVE-2026-1002"},
		},
		errorMap: map[string]error{
			"CVE-2026-1001": errors.New("lookup failed"),
		},
	}

	vuln := vulnAssessment{
		ID:      "GO-TEST-1",
		Aliases: []string{"CVE-2026-1001", "cve-2026-1000", "CVE-2026-1002"},
	}

	assessment, err := resolver.Resolve(context.Background(), vuln)
	if err == nil {
		t.Fatal("expected joined resolver error")
	}
	if assessment.Severity != severityHigh || assessment.Score != 8.1 || assessment.Source != "CVE-2026-1002" {
		t.Fatalf("unexpected best assessment: %#v", assessment)
	}
	if !strings.Contains(err.Error(), "lookup failed") {
		t.Fatalf("unexpected resolver error: %v", err)
	}
}

func TestResolveWithNoCVECandidates(t *testing.T) {
	t.Parallel()

	resolver := &nvdSeverityResolver{
		snapshot: map[string]severityAssessment{},
		cache:    map[string]severityAssessment{},
		errorMap: map[string]error{},
	}

	assessment, err := resolver.Resolve(context.Background(), vulnAssessment{
		ID:      "GO-ONLY",
		Aliases: []string{"GHSA-123"},
	})
	if err != nil {
		t.Fatalf("expected nil error when no CVEs are available: %v", err)
	}
	if assessment.Severity != severityUnknown {
		t.Fatalf("expected UNKNOWN severity, got %#v", assessment)
	}
}

func newTestResolver(client *http.Client, baseURL, apiKey string) *nvdSeverityResolver {
	return &nvdSeverityResolver{
		client:   client,
		baseURL:  baseURL,
		apiKey:   apiKey,
		snapshot: map[string]severityAssessment{},
		cache:    map[string]severityAssessment{},
		errorMap: map[string]error{},
	}
}

func TestResolveCVEInvalidBaseURL(t *testing.T) {
	t.Parallel()

	resolver := newTestResolver(&http.Client{Timeout: time.Second}, "://bad-url", "")
	assessment, err := resolver.resolveCVE(context.Background(), "cve-2026-2000")
	if err == nil {
		t.Fatal("expected URL parse error")
	}
	if assessment.Severity != severityUnknown || assessment.Source != "CVE-2026-2000" {
		t.Fatalf("unexpected assessment: %#v", assessment)
	}
}

func TestResolveCVENon200Status(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusTeapot)
	}))
	t.Cleanup(server.Close)

	resolver := newTestResolver(server.Client(), server.URL, "")
	assessment, err := resolver.resolveCVE(context.Background(), "CVE-2026-2001")
	if err == nil {
		t.Fatal("expected HTTP status error")
	}
	if !strings.Contains(err.Error(), "HTTP 418") {
		t.Fatalf("unexpected status error: %v", err)
	}
	if assessment.Severity != severityUnknown {
		t.Fatalf("unexpected assessment: %#v", assessment)
	}
}

func TestResolveCVEDecodeError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, writeErr := writer.Write([]byte("{not-json"))
		if writeErr != nil {
			return
		}
	}))
	t.Cleanup(server.Close)

	resolver := newTestResolver(server.Client(), server.URL, "")
	assessment, err := resolver.resolveCVE(context.Background(), "CVE-2026-2002")
	if err == nil {
		t.Fatal("expected decode error")
	}
	if assessment.Severity != severityUnknown {
		t.Fatalf("unexpected assessment: %#v", assessment)
	}
}

func TestResolveCVESuccessfulLookupIsCached(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		calls.Add(1)
		response := nvdResponse{
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

		metric := nvdMetric{}
		metric.CVSSData.BaseSeverity = "HIGH"
		metric.CVSSData.BaseScore = 7.4
		response.Vulnerabilities[0].CVE.Metrics.CVSSMetricV31 = []nvdMetric{metric}

		writer.Header().Set("Content-Type", "application/json")
		encodeErr := json.NewEncoder(writer).Encode(response)
		if encodeErr != nil {
			http.Error(writer, encodeErr.Error(), http.StatusInternalServerError)
			return
		}
	}))
	t.Cleanup(server.Close)

	resolver := newTestResolver(server.Client(), server.URL, "")
	first, firstErr := resolver.resolveCVE(context.Background(), "CVE-2026-2003")
	if firstErr != nil {
		t.Fatalf("unexpected first lookup error: %v", firstErr)
	}
	if first.Severity != severityHigh || first.Score != 7.4 {
		t.Fatalf("unexpected first assessment: %#v", first)
	}

	second, secondErr := resolver.resolveCVE(context.Background(), "CVE-2026-2003")
	if secondErr != nil {
		t.Fatalf("unexpected cached lookup error: %v", secondErr)
	}
	if !reflect.DeepEqual(second, first) {
		t.Fatalf("expected cached assessment to match first lookup: got %#v want %#v", second, first)
	}
	if calls.Load() != 1 {
		t.Fatalf("expected one upstream call, got %d", calls.Load())
	}
}

func TestResolveCVERetryableStatusEventuallyFails(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		calls.Add(1)
		writer.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	resolver := newTestResolver(server.Client(), server.URL, "configured")
	assessment, err := resolver.resolveCVE(context.Background(), "CVE-2026-2004")
	if err == nil {
		t.Fatal("expected error after retries")
	}
	if !strings.Contains(err.Error(), "HTTP 500") {
		t.Fatalf("unexpected retry error: %v", err)
	}
	if assessment.Severity != severityUnknown {
		t.Fatalf("unexpected assessment: %#v", assessment)
	}
	if calls.Load() != 3 {
		t.Fatalf("expected three attempts, got %d", calls.Load())
	}
}

func TestResolveCVERetryableStatusReturnsContextCancellation(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		calls.Add(1)
		writer.WriteHeader(http.StatusTooManyRequests)
		cancel()
	}))
	t.Cleanup(server.Close)

	resolver := newTestResolver(server.Client(), server.URL, "configured")
	assessment, err := resolver.resolveCVE(ctx, "CVE-2026-2005")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got: %v", err)
	}
	if assessment.Severity != severityUnknown {
		t.Fatalf("unexpected assessment: %#v", assessment)
	}
	if calls.Load() != 1 {
		t.Fatalf("expected one request before cancellation, got %d", calls.Load())
	}
}

func TestResolveCVETransportErrorWithCanceledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	resolver := newTestResolver(
		&http.Client{
			Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
				return nil, errors.New("transport down")
			}),
		},
		"https://example.test",
		"configured",
	)

	assessment, err := resolver.resolveCVE(ctx, "CVE-2026-2006")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got: %v", err)
	}
	if assessment.Severity != severityUnknown {
		t.Fatalf("unexpected assessment: %#v", assessment)
	}
}

func TestBestNVDSeverityNoMetrics(t *testing.T) {
	t.Parallel()

	severityValue, score := bestNVDSeverity(nvdResponse{})
	if severityValue != severityUnknown || score != 0 {
		t.Fatalf("unexpected empty payload severity: %s %.1f", severityValue, score)
	}
}

func TestBestNVDSeverityPrefersHigherScoreOnTies(t *testing.T) {
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

	first := nvdMetric{}
	first.CVSSData.BaseScore = 7.1
	first.CVSSData.BaseSeverity = "HIGH"

	second := nvdMetric{}
	second.CVSSData.BaseScore = 8.3
	second.CVSSData.BaseSeverity = "HIGH"

	payload.Vulnerabilities[0].CVE.Metrics.CVSSMetricV30 = []nvdMetric{first, second}

	severityValue, score := bestNVDSeverity(payload)
	if severityValue != severityHigh || score != 8.3 {
		t.Fatalf("unexpected tie-break severity: %s %.1f", severityValue, score)
	}
}

func TestBetterSeverity(t *testing.T) {
	t.Parallel()

	if !betterSeverity(
		severityAssessment{Severity: severityHigh, Score: 7.0},
		severityAssessment{Severity: severityMedium, Score: 9.5},
	) {
		t.Fatal("expected higher rank severity to win")
	}

	if !betterSeverity(
		severityAssessment{Severity: severityHigh, Score: 8.1},
		severityAssessment{Severity: severityHigh, Score: 7.1},
	) {
		t.Fatal("expected higher score on equal rank to win")
	}

	if betterSeverity(
		severityAssessment{Severity: severityLow, Score: 1.0},
		severityAssessment{Severity: severityLow, Score: 2.0},
	) {
		t.Fatal("expected lower score to lose on equal rank")
	}
}

func TestSeverityRankMatrix(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		value severity
		want  int
	}{
		{value: severityCritical, want: 4},
		{value: severityHigh, want: 3},
		{value: severityMedium, want: 2},
		{value: severityLow, want: 1},
		{value: severity("other"), want: 0},
	}

	for _, testCase := range testCases {
		t.Run(string(testCase.value), func(t *testing.T) {
			t.Parallel()
			if got := severityRank(testCase.value); got != testCase.want {
				t.Fatalf("severityRank(%s) = %d, want %d", testCase.value, got, testCase.want)
			}
		})
	}
}

func TestAddQueryParam(t *testing.T) {
	t.Parallel()

	updated, err := addQueryParam("https://example.test/path?existing=1", "cveId", "CVE-2026-9999")
	if err != nil {
		t.Fatalf("addQueryParam returned error: %v", err)
	}
	if !strings.Contains(updated, "existing=1") || !strings.Contains(updated, "cveId=CVE-2026-9999") {
		t.Fatalf("unexpected query update: %s", updated)
	}

	if _, queryErr := addQueryParam("://bad-url", "k", "v"); queryErr == nil {
		t.Fatal("expected invalid URL error")
	}
}

func TestPrintResult(t *testing.T) {
	now := time.Date(2026, time.February, 22, 12, 0, 0, 0, time.UTC)
	result := evaluationResult{
		Fail: []evaluatedVuln{
			{
				Vuln: vulnAssessment{
					ID:            "GO-FAIL",
					Summary:       "fails policy",
					URL:           "https://example.test/fail",
					FixedVersions: []string{"v1.2.3"},
				},
				Severity:      severityAssessment{Severity: severityHigh, Score: 8.1, Source: "CVE-2026-3000"},
				ResolverError: errors.New("nvd fallback used"),
			},
		},
		Warn: []evaluatedVuln{
			{
				Vuln:     vulnAssessment{ID: "GO-WARN", Summary: "warn only"},
				Severity: severityAssessment{Severity: severityLow, Score: 1.1, Source: "CVE-2026-3001"},
			},
		},
		Accepted: []evaluatedVuln{
			{
				Vuln:        vulnAssessment{ID: "GO-ACCEPT"},
				MatchedByID: "GO-ACCEPT",
				Override: &riskOverride{
					ID:        "GO-ACCEPT",
					Reason:    "temporary",
					ExpiresOn: now.Add(24 * time.Hour),
				},
			},
		},
		Expired: []evaluatedVuln{
			{
				Vuln:        vulnAssessment{ID: "GO-EXPIRED"},
				MatchedByID: "GO-EXPIRED",
				Override: &riskOverride{
					ID:        "GO-EXPIRED",
					Reason:    "needs renewal",
					ExpiresOn: now.Add(-24 * time.Hour),
				},
			},
		},
	}

	for index := 0; index < 12; index++ {
		result.Info = append(result.Info, evaluatedVuln{
			Vuln: vulnAssessment{
				ID:      fmt.Sprintf("GO-INFO-%02d", index),
				Summary: "not reachable",
			},
		})
	}

	output := captureStdout(t, func() {
		printResult(scanModeSource, result)
	})

	expectedSnippets := []string{
		"govulncheck policy results (source)",
		"Expired overrides",
		"Failing vulnerabilities",
		"Warning vulnerabilities",
		"Accepted risk overrides",
		"Not reachable vulnerabilities",
		"severity source: CVE-2026-3000",
		"resolver warning: nvd fallback used",
		"... and 2 more not reachable vulnerabilities",
	}
	for _, snippet := range expectedSnippets {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected output to contain %q, got:\n%s", snippet, output)
		}
	}
}

func TestPrintResultBinaryInfoHeading(t *testing.T) {
	t.Parallel()

	output := captureStdout(t, func() {
		printResult(scanModeBinary, evaluationResult{
			Info: []evaluatedVuln{
				{Vuln: vulnAssessment{ID: "GO-1", Summary: "binary info"}},
			},
		})
	})

	if !strings.Contains(output, "govulncheck policy results (binary)") {
		t.Fatalf("expected binary scan header, got:\n%s", output)
	}
	if !strings.Contains(output, "Informational vulnerabilities") {
		t.Fatalf("expected binary info heading, got:\n%s", output)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	originalStdout := os.Stdout
	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe failed: %v", err)
	}

	type copyResult struct {
		value string
		err   error
	}

	output := make(chan copyResult, 1)
	go func() {
		var buffer bytes.Buffer
		_, copyErr := io.Copy(&buffer, readPipe)
		output <- copyResult{value: buffer.String(), err: copyErr}
	}()

	os.Stdout = writePipe
	defer func() {
		os.Stdout = originalStdout
		_ = writePipe.Close()
		_ = readPipe.Close()
	}()

	fn()

	if closeErr := writePipe.Close(); closeErr != nil {
		t.Fatalf("closing stdout writer failed: %v", closeErr)
	}
	result := <-output
	if result.err != nil {
		t.Fatalf("capturing stdout failed: %v", result.err)
	}
	return result.value
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}
