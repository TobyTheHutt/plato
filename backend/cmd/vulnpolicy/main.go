//go:build tools

package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultNVDAPIBaseURL  = "https://services.nvd.nist.gov/rest/json/cves/2.0"
	defaultGHSAAPIBaseURL = "https://api.github.com/advisories"
	scanModeSource        = "source"
	scanModeBinary        = "binary"
	nvd401ErrorMessage    = "Missing or invalid NVD API key. Please configure a valid API key."
	nvd403ErrorMessage    = "NVD API key valid but lacks required permissions. Please check your API key configuration."
	ghsa401ErrorMessage   = "Missing or invalid GHSA token. Remove GHSA_TOKEN_FILE to use unauthenticated access, or configure a valid token."
	ghsa403ErrorMessage   = "GHSA token is valid but access is forbidden. Check token scope and account permissions."
)

type severity string

const (
	severityUnknown  severity = "UNKNOWN"
	severityLow      severity = "LOW"
	severityMedium   severity = "MEDIUM"
	severityHigh     severity = "HIGH"
	severityCritical severity = "CRITICAL"
)

type severityMethod string

const (
	severityMethodUnknown severityMethod = "unknown"
	severityMethodOSV     severityMethod = "osv"
	severityMethodGHSA    severityMethod = "ghsa"
	severityMethodNVD     severityMethod = "nvd"
)

type vulnAssessment struct {
	ID            string
	Aliases       []string
	Summary       string
	URL           string
	FixedVersions []string
	Reachable     bool
	OSVSeverity   severityAssessment
}

type severityAssessment struct {
	Severity severity
	Score    float64
	Source   string
	Method   severityMethod
	Reason   string
}

type evaluatedVuln struct {
	Vuln          vulnAssessment
	Severity      severityAssessment
	Override      *riskOverride
	MatchedByID   string
	ResolverError error
}

type evaluationResult struct {
	Fail     []evaluatedVuln
	Warn     []evaluatedVuln
	Info     []evaluatedVuln
	Accepted []evaluatedVuln
	Expired  []evaluatedVuln
}

type excludedVulnerabilityIDs struct {
	all       map[string]struct{}
	reachable map[string]struct{}
}

type severityResolver interface {
	Resolve(ctx context.Context, vuln vulnAssessment) (severityAssessment, error)
}

type nvdSeverityResolver struct {
	client      *http.Client
	baseURL     string
	apiKey      string
	ghsaBaseURL string
	ghsaToken   string
	offline     bool
	snapshot    map[string]severityAssessment
	mu          sync.RWMutex
	cache       map[string]severityAssessment
	errorMap    map[string]error
}

type govulnEvent struct {
	OSV     *govulnOSV     `json:"osv"`
	Finding *govulnFinding `json:"finding"`
}

type govulnOSV struct {
	ID               string      `json:"id"`
	Aliases          []string    `json:"aliases"`
	Summary          string      `json:"summary"`
	Severity         interface{} `json:"severity"`
	DatabaseSpecific struct {
		URL      string  `json:"url"`
		Severity string  `json:"severity"`
		Score    float64 `json:"score"`
	} `json:"database_specific"`
}

type govulnFinding struct {
	OSV          string             `json:"osv"`
	FixedVersion string             `json:"fixed_version"`
	Trace        []govulnTraceFrame `json:"trace"`
}

type govulnTraceFrame struct {
	Package  string `json:"package"`
	Function string `json:"function"`
}

type overrideConfig struct {
	Overrides []overrideInput `json:"overrides"`
}

type overrideInput struct {
	ID        string `json:"id"`
	Reason    string `json:"reason"`
	ExpiresOn string `json:"expires_on"`
}

type riskOverride struct {
	ID        string
	Reason    string
	ExpiresOn time.Time
}

type nvdResponse struct {
	Vulnerabilities []struct {
		CVE struct {
			Metrics struct {
				CVSSMetricV31 []nvdMetric `json:"cvssMetricV31"`
				CVSSMetricV30 []nvdMetric `json:"cvssMetricV30"`
				CVSSMetricV2  []nvdMetric `json:"cvssMetricV2"`
			} `json:"metrics"`
		} `json:"cve"`
	} `json:"vulnerabilities"`
}

type nvdMetric struct {
	CVSSData struct {
		BaseScore    float64 `json:"baseScore"`
		BaseSeverity string  `json:"baseSeverity"`
	} `json:"cvssData"`
}

type ghsaResponse struct {
	GHSAID         string      `json:"ghsa_id"`
	Severity       string      `json:"severity"`
	CVSS           ghsaCVSS    `json:"cvss"`
	CVSSSeverities ghsaSevData `json:"cvss_severities"`
}

type ghsaCVSS struct {
	Score interface{} `json:"score"`
}

type ghsaSevData struct {
	CVSSV3 ghsaCVSSData `json:"cvss_v3"`
	CVSSV4 ghsaCVSSData `json:"cvss_v4"`
}

type ghsaCVSSData struct {
	Score    interface{} `json:"score"`
	Severity string      `json:"severity"`
}

type severitySnapshotFile struct {
	CVEs map[string]severitySnapshotEntry `json:"cves"`
}

type severitySnapshotEntry struct {
	Severity string  `json:"severity"`
	Score    float64 `json:"score"`
}

func main() {
	inputPath := flag.String("input", "", "path to govulncheck JSON output")
	overridesPath := flag.String("overrides", "", "path to vulnerability override config")
	scanMode := flag.String("scan-mode", scanModeSource, "govulncheck scan mode used by the input: source or binary")
	excludeInput := flag.String("exclude-input", "", "optional path to govulncheck JSON output whose vulnerabilities should be excluded")
	nvdAPIBaseURL := flag.String("nvd-api-base-url", defaultNVDAPIBaseURL, "NVD CVE API base URL")
	nvdAPIKeyFile := flag.String("nvd-api-key-file", "", "path to file containing NVD API key")
	ghsaAPIBaseURL := flag.String("ghsa-api-base-url", defaultGHSAAPIBaseURL, "GHSA advisory API base URL")
	ghsaTokenFile := flag.String("ghsa-token-file", "", "path to file containing optional GHSA API token")
	severitySnapshot := flag.String("severity-snapshot", "", "path to pinned NVD severity snapshot JSON")
	offlineMode := flag.Bool("offline", false, "disable live GHSA and NVD lookups and use pinned snapshot data only")
	nvdTimeout := flag.Duration("nvd-timeout", 15*time.Second, "timeout per severity API request")
	flag.Parse()

	if strings.TrimSpace(*inputPath) == "" {
		exitf("error: -input is required")
	}
	if strings.TrimSpace(*overridesPath) == "" {
		exitf("error: -overrides is required")
	}
	normalizedScanMode, err := normalizeScanMode(*scanMode)
	if err != nil {
		exitf("error: %v", err)
	}

	inputFile, err := os.Open(*inputPath)
	if err != nil {
		exitf("error: open govulncheck output: %v", err)
	}

	vulns, err := parseGovulncheckOutputWithMode(inputFile, normalizedScanMode)
	closeErr := inputFile.Close()
	if err != nil {
		exitf("error: parse govulncheck output: %v", err)
	}
	if closeErr != nil {
		exitf("error: close govulncheck output: %v", closeErr)
	}

	if strings.TrimSpace(*excludeInput) != "" {
		excludedIDs, excludeErr := collectExcludedIDs(*excludeInput)
		if excludeErr != nil {
			exitf("error: load exclude-input: %v", excludeErr)
		}
		vulns = filterExcludedVulnerabilities(vulns, excludedIDs)
	}

	overrides, err := loadOverrides(*overridesPath)
	if err != nil {
		exitf("error: load overrides: %v", err)
	}

	apiKey, err := resolveNVDAPIKey(*nvdAPIKeyFile)
	if err != nil {
		exitf("error: resolve NVD API key: %v", err)
	}

	ghsaToken, err := resolveGHSAToken(*ghsaTokenFile)
	if err != nil {
		exitf("error: resolve GHSA token: %v", err)
	}

	snapshot, err := loadSeveritySnapshot(*severitySnapshot)
	if err != nil {
		exitf("error: load severity snapshot: %v", err)
	}

	if *offlineMode && len(snapshot) == 0 {
		exitf("error: -offline requires -severity-snapshot")
	}

	resolver := &nvdSeverityResolver{
		client: &http.Client{
			Timeout: *nvdTimeout,
		},
		baseURL:     *nvdAPIBaseURL,
		apiKey:      apiKey,
		ghsaBaseURL: *ghsaAPIBaseURL,
		ghsaToken:   ghsaToken,
		offline:     *offlineMode,
		snapshot:    snapshot,
		cache:       make(map[string]severityAssessment),
		errorMap:    make(map[string]error),
	}

	result := evaluateVulnerabilities(context.Background(), vulns, overrides, resolver, time.Now().UTC())
	printResult(normalizedScanMode, result)

	if len(result.Fail) > 0 || len(result.Expired) > 0 {
		os.Exit(1)
	}
}

func exitf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func parseGovulncheckOutput(reader io.Reader) ([]vulnAssessment, error) {
	return parseGovulncheckOutputWithMode(reader, scanModeSource)
}

func parseGovulncheckOutputWithMode(reader io.Reader, scanMode string) ([]vulnAssessment, error) {
	decoder := json.NewDecoder(reader)
	vulnByID := make(map[string]*vulnAssessment)

	for {
		var event govulnEvent
		if err := decoder.Decode(&event); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}

		if event.OSV != nil {
			entry := ensureVuln(vulnByID, event.OSV.ID)
			entry.Aliases = uniqueStrings(append(entry.Aliases, event.OSV.Aliases...))
			if strings.TrimSpace(event.OSV.Summary) != "" {
				entry.Summary = strings.TrimSpace(event.OSV.Summary)
			}
			if strings.TrimSpace(event.OSV.DatabaseSpecific.URL) != "" {
				entry.URL = strings.TrimSpace(event.OSV.DatabaseSpecific.URL)
			}
			if severityValue, ok := resolveOSVSeverity(*event.OSV); ok && betterSeverity(severityValue, entry.OSVSeverity) {
				entry.OSVSeverity = severityValue
			}
		}

		if event.Finding != nil {
			entry := ensureVuln(vulnByID, event.Finding.OSV)
			fixed := strings.TrimSpace(event.Finding.FixedVersion)
			if fixed != "" {
				entry.FixedVersions = uniqueStrings(append(entry.FixedVersions, fixed))
			}
			if scanMode == scanModeBinary || findingIsReachable(event.Finding) {
				entry.Reachable = true
			}
		}
	}

	result := make([]vulnAssessment, 0, len(vulnByID))
	for _, vuln := range vulnByID {
		sort.Strings(vuln.Aliases)
		sort.Strings(vuln.FixedVersions)
		result = append(result, *vuln)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})

	return result, nil
}

func normalizeScanMode(value string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case scanModeSource, scanModeBinary:
		return normalized, nil
	default:
		return "", fmt.Errorf("unsupported -scan-mode %q (valid values: %s, %s)", value, scanModeSource, scanModeBinary)
	}
}

func collectExcludedIDs(path string) (excludedVulnerabilityIDs, error) {
	file, err := os.Open(strings.TrimSpace(path))
	if err != nil {
		return excludedVulnerabilityIDs{}, err
	}
	vulns, parseErr := parseGovulncheckOutput(file)
	closeErr := file.Close()
	if parseErr != nil {
		return excludedVulnerabilityIDs{}, parseErr
	}
	if closeErr != nil {
		return excludedVulnerabilityIDs{}, closeErr
	}

	excludedIDs := excludedVulnerabilityIDs{
		all:       make(map[string]struct{}, len(vulns)),
		reachable: make(map[string]struct{}, len(vulns)),
	}
	for _, vuln := range vulns {
		candidateIDs := append([]string{vuln.ID}, vuln.Aliases...)
		for _, candidateID := range candidateIDs {
			normalizedID := normalizeID(candidateID)
			if normalizedID == "" {
				continue
			}
			excludedIDs.all[normalizedID] = struct{}{}
			if vuln.Reachable {
				excludedIDs.reachable[normalizedID] = struct{}{}
			}
		}
	}
	return excludedIDs, nil
}

func filterExcludedVulnerabilities(vulns []vulnAssessment, excludedIDs excludedVulnerabilityIDs) []vulnAssessment {
	if len(excludedIDs.all) == 0 {
		return vulns
	}

	result := make([]vulnAssessment, 0, len(vulns))
	for _, vuln := range vulns {
		matchedAll, matchedReachable := matchesExcludedIDs(vuln, excludedIDs)
		if matchedReachable || (matchedAll && !vuln.Reachable) {
			continue
		}
		result = append(result, vuln)
	}
	return result
}

func matchesExcludedIDs(vuln vulnAssessment, excludedIDs excludedVulnerabilityIDs) (bool, bool) {
	matchedAll := false
	matchedReachable := false

	candidateIDs := append([]string{vuln.ID}, vuln.Aliases...)
	for _, candidateID := range candidateIDs {
		normalizedID := normalizeID(candidateID)
		if normalizedID == "" {
			continue
		}
		if _, exists := excludedIDs.all[normalizedID]; exists {
			matchedAll = true
		}
		if _, exists := excludedIDs.reachable[normalizedID]; exists {
			matchedReachable = true
		}
	}

	return matchedAll, matchedReachable
}

func ensureVuln(vulnByID map[string]*vulnAssessment, id string) *vulnAssessment {
	normalizedID := strings.TrimSpace(id)
	if existing, ok := vulnByID[normalizedID]; ok {
		return existing
	}
	entry := &vulnAssessment{ID: normalizedID}
	vulnByID[normalizedID] = entry
	return entry
}

func findingIsReachable(finding *govulnFinding) bool {
	if finding == nil {
		return false
	}
	if len(finding.Trace) > 1 {
		return true
	}
	if len(finding.Trace) == 1 {
		frame := finding.Trace[0]
		return strings.TrimSpace(frame.Package) != "" && strings.TrimSpace(frame.Function) != ""
	}
	return false
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		normalized := strings.TrimSpace(value)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result
}

func loadOverrides(path string) (map[string]riskOverride, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config overrideConfig
	unmarshalErr := json.Unmarshal(data, &config)
	if unmarshalErr != nil {
		return nil, unmarshalErr
	}

	overrides := make(map[string]riskOverride, len(config.Overrides))
	for _, item := range config.Overrides {
		id := normalizeID(item.ID)
		if id == "" {
			return nil, fmt.Errorf("override id is required")
		}
		if _, exists := overrides[id]; exists {
			return nil, fmt.Errorf("duplicate override id: %s", id)
		}
		reason := strings.TrimSpace(item.Reason)
		if reason == "" {
			return nil, fmt.Errorf("override %s must include a reason", id)
		}
		expiresOn := strings.TrimSpace(item.ExpiresOn)
		if expiresOn == "" {
			return nil, fmt.Errorf("override %s must include expires_on", id)
		}
		expiryDate, parseErr := time.Parse("2006-01-02", expiresOn)
		if parseErr != nil {
			return nil, fmt.Errorf("override %s has invalid expires_on %q: %w", id, expiresOn, parseErr)
		}
		overrides[id] = riskOverride{
			ID:        id,
			Reason:    reason,
			ExpiresOn: expiryDate.UTC(),
		}
	}

	return overrides, nil
}

func normalizeID(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

func evaluateVulnerabilities(
	ctx context.Context,
	vulns []vulnAssessment,
	overrides map[string]riskOverride,
	resolver severityResolver,
	now time.Time,
) evaluationResult {
	result := evaluationResult{
		Fail:     make([]evaluatedVuln, 0),
		Warn:     make([]evaluatedVuln, 0),
		Info:     make([]evaluatedVuln, 0),
		Accepted: make([]evaluatedVuln, 0),
		Expired:  make([]evaluatedVuln, 0),
	}

	for _, vuln := range vulns {
		override, matchedByID := matchOverride(vuln, overrides)
		if override != nil {
			evaluated := evaluatedVuln{
				Vuln:        vuln,
				Override:    override,
				MatchedByID: matchedByID,
			}
			if overrideExpired(*override, now) {
				result.Expired = append(result.Expired, evaluated)
				continue
			}
			result.Accepted = append(result.Accepted, evaluated)
			continue
		}

		if !vuln.Reachable {
			result.Info = append(result.Info, evaluatedVuln{Vuln: vuln})
			continue
		}

		severityDetails, err := resolver.Resolve(ctx, vuln)
		evaluated := evaluatedVuln{
			Vuln:          vuln,
			Severity:      severityDetails,
			ResolverError: err,
		}
		switch severityDetails.Severity {
		case severityCritical, severityHigh:
			result.Fail = append(result.Fail, evaluated)
		case severityMedium, severityLow:
			result.Warn = append(result.Warn, evaluated)
		default:
			result.Fail = append(result.Fail, evaluated)
		}
	}

	sortEvaluated(result.Fail)
	sortEvaluated(result.Warn)
	sortEvaluated(result.Info)
	sortEvaluated(result.Accepted)
	sortEvaluated(result.Expired)

	return result
}

func matchOverride(vuln vulnAssessment, overrides map[string]riskOverride) (*riskOverride, string) {
	candidateIDs := append([]string{vuln.ID}, vuln.Aliases...)
	for _, candidate := range candidateIDs {
		normalized := normalizeID(candidate)
		if override, ok := overrides[normalized]; ok {
			overrideCopy := override
			return &overrideCopy, normalized
		}
	}
	return nil, ""
}

func overrideExpired(override riskOverride, now time.Time) bool {
	currentDate := time.Date(now.UTC().Year(), now.UTC().Month(), now.UTC().Day(), 0, 0, 0, 0, time.UTC)
	return currentDate.After(override.ExpiresOn)
}

func sortEvaluated(items []evaluatedVuln) {
	sort.Slice(items, func(i, j int) bool {
		left := items[i]
		right := items[j]
		if left.Severity.Severity != right.Severity.Severity {
			return severityRank(left.Severity.Severity) > severityRank(right.Severity.Severity)
		}
		return left.Vuln.ID < right.Vuln.ID
	})
}

func resolveNVDAPIKey(apiKeyFile string) (string, error) {
	trimmedPath := strings.TrimSpace(apiKeyFile)
	if trimmedPath == "" {
		return strings.TrimSpace(os.Getenv("NVD_API_KEY")), nil
	}

	rawValue, err := os.ReadFile(trimmedPath)
	if err != nil {
		return "", err
	}
	apiKey := strings.TrimSpace(string(rawValue))
	if apiKey == "" {
		return "", fmt.Errorf("NVD API key file %q is empty", trimmedPath)
	}
	return apiKey, nil
}

func resolveGHSAToken(tokenFile string) (string, error) {
	trimmedPath := strings.TrimSpace(tokenFile)
	if trimmedPath == "" {
		token := strings.TrimSpace(os.Getenv("GHSA_TOKEN"))
		if token != "" {
			return token, nil
		}
		return strings.TrimSpace(os.Getenv("GITHUB_TOKEN")), nil
	}

	rawValue, err := os.ReadFile(trimmedPath)
	if err != nil {
		return "", err
	}
	token := strings.TrimSpace(string(rawValue))
	if token == "" {
		return "", fmt.Errorf("GHSA token file %q is empty", trimmedPath)
	}
	return token, nil
}

func loadSeveritySnapshot(path string) (map[string]severityAssessment, error) {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return map[string]severityAssessment{}, nil
	}

	rawValue, err := os.ReadFile(trimmedPath)
	if err != nil {
		return nil, err
	}

	var file severitySnapshotFile
	unmarshalErr := json.Unmarshal(rawValue, &file)
	if unmarshalErr != nil {
		return nil, unmarshalErr
	}

	result := make(map[string]severityAssessment, len(file.CVEs))
	for rawID, entry := range file.CVEs {
		normalizedID := normalizeID(rawID)
		if !strings.HasPrefix(normalizedID, "CVE-") {
			return nil, fmt.Errorf("snapshot id must start with CVE-: %s", rawID)
		}
		severityValue := normalizeSeverity(entry.Severity, entry.Score)
		result[normalizedID] = severityAssessment{
			Severity: severityValue,
			Score:    entry.Score,
			Source:   normalizedID,
			Method:   severityMethodNVD,
		}
	}

	return result, nil
}

func (resolver *nvdSeverityResolver) Resolve(ctx context.Context, vuln vulnAssessment) (severityAssessment, error) {
	if osvSeverity, ok := resolvedOSVSeverity(vuln); ok {
		return osvSeverity, nil
	}

	ghsaCandidates := collectGHSAIDs(vuln)
	ghsaResult := resolver.resolveBestFromCandidates(ctx, ghsaCandidates, resolver.resolveGHSA)
	if ghsaResult.Resolved {
		return ghsaResult.Best, ghsaResult.LookupErr
	}

	cveCandidates := collectCVEIDs(vuln)
	nvdResult := resolver.resolveBestFromCandidates(ctx, cveCandidates, resolver.resolveCVE)
	joinedErr := errors.Join(ghsaResult.LookupErr, nvdResult.LookupErr)
	if nvdResult.Resolved {
		return nvdResult.Best, joinedErr
	}

	reason := buildUnknownSeverityReason(ghsaResult, nvdResult)
	source := unknownSeveritySource(vuln, ghsaCandidates, cveCandidates)
	assessment := unknownSeverityAssessmentWithReason(source, reason, severityMethodUnknown)
	return assessment, joinedErr
}

type sourceResolutionResult struct {
	Best            severityAssessment
	LookupErr       error
	HasCandidates   bool
	Resolved        bool
	ReturnedUnknown bool
}

func (resolver *nvdSeverityResolver) resolveBestFromCandidates(
	ctx context.Context,
	candidates []string,
	lookup func(context.Context, string) (severityAssessment, error),
) sourceResolutionResult {
	result := sourceResolutionResult{
		Best:          severityAssessment{Severity: severityUnknown},
		HasCandidates: len(candidates) > 0,
	}
	for _, candidateID := range candidates {
		assessment, err := lookup(ctx, candidateID)
		if err != nil {
			result.LookupErr = errors.Join(result.LookupErr, err)
			continue
		}
		if assessment.Severity == severityUnknown {
			result.ReturnedUnknown = true
			continue
		}
		if !result.Resolved || betterSeverity(assessment, result.Best) {
			result.Best = assessment
		}
		result.Resolved = true
	}
	return result
}

func buildUnknownSeverityReason(ghsaResult, nvdResult sourceResolutionResult) string {
	if !ghsaResult.HasCandidates && !nvdResult.HasCandidates {
		return "OSV severity unavailable in govulncheck input, no CVE/GHSA aliases found"
	}

	reasons := []string{"OSV severity unavailable in govulncheck input"}
	ghsaReason := sourceUnknownReason("GHSA", ghsaResult, "no GHSA aliases found")
	if ghsaReason != "" {
		reasons = append(reasons, ghsaReason)
	}
	nvdReason := sourceUnknownReason("NVD", nvdResult, "no CVE aliases found")
	if nvdReason != "" {
		reasons = append(reasons, nvdReason)
	}
	return strings.Join(reasons, ", ")
}

func sourceUnknownReason(sourceName string, result sourceResolutionResult, noAliasMessage string) string {
	if !result.HasCandidates {
		return noAliasMessage
	}
	if result.ReturnedUnknown {
		return fmt.Sprintf("%s lookup returned no severity data", sourceName)
	}
	return fmt.Sprintf("%s lookup failed", sourceName)
}

func unknownSeveritySource(vuln vulnAssessment, ghsaCandidates, cveCandidates []string) string {
	if len(ghsaCandidates) > 0 {
		return ghsaCandidates[0]
	}
	if len(cveCandidates) > 0 {
		return cveCandidates[0]
	}
	return normalizeID(vuln.ID)
}

func resolvedOSVSeverity(vuln vulnAssessment) (severityAssessment, bool) {
	if vuln.OSVSeverity.Severity == "" || vuln.OSVSeverity.Severity == severityUnknown {
		return severityAssessment{}, false
	}
	assessment := vuln.OSVSeverity
	if assessment.Source == "" {
		assessment.Source = normalizeID(vuln.ID)
	}
	assessment.Method = severityMethodOSV
	return assessment, true
}

func (resolver *nvdSeverityResolver) resolveCVE(ctx context.Context, cveID string) (severityAssessment, error) {
	normalizedCVE := normalizeID(cveID)
	if cached, cachedErr, ok := resolver.readCache(normalizedCVE); ok {
		return cached, cachedErr
	}

	if snapshotSeverity, ok := resolver.snapshot[normalizedCVE]; ok {
		resolver.writeCache(normalizedCVE, snapshotSeverity, nil)
		return snapshotSeverity, nil
	}

	if resolver.offline {
		assessment := unknownSeverityAssessment(normalizedCVE)
		err := fmt.Errorf("offline mode enabled and %s is missing from severity snapshot", normalizedCVE)
		resolver.writeCache(normalizedCVE, assessment, err)
		return assessment, err
	}

	requestURL, err := addQueryParam(resolver.baseURL, "cveId", normalizedCVE)
	if err != nil {
		assessment := unknownSeverityAssessment(normalizedCVE)
		resolver.writeCache(normalizedCVE, assessment, err)
		return assessment, err
	}

	maxAttempts := 3
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		request, requestErr := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
		if requestErr != nil {
			assessment := unknownSeverityAssessment(normalizedCVE)
			resolver.writeCache(normalizedCVE, assessment, requestErr)
			return assessment, requestErr
		}
		request.Header.Set("Accept", "application/json")
		request.Header.Set("User-Agent", "plato-govuln-policy/1.0")

		apiKeyConfigured := false
		if resolver.apiKey != "" {
			request.Header.Set("apiKey", resolver.apiKey)
			apiKeyConfigured = true
		}

		response, responseErr := resolver.client.Do(request)
		if responseErr != nil {
			if attempt < maxAttempts {
				sleepErr := sleepWithBackoff(ctx, attempt, apiKeyConfigured)
				if sleepErr != nil {
					return unknownSeverityAssessment(normalizedCVE), sleepErr
				}
				continue
			}

			assessment := unknownSeverityAssessment(normalizedCVE)
			resolver.writeCache(normalizedCVE, assessment, responseErr)
			return assessment, responseErr
		}

		if response.StatusCode == http.StatusUnauthorized {
			response.Body.Close()
			assessment := unknownSeverityAssessment(normalizedCVE)
			finalErr := errors.New(nvd401ErrorMessage)
			resolver.writeCache(normalizedCVE, assessment, finalErr)
			return assessment, finalErr
		}

		if response.StatusCode == http.StatusForbidden {
			response.Body.Close()
			assessment := unknownSeverityAssessment(normalizedCVE)
			finalErr := errors.New(nvd403ErrorMessage)
			resolver.writeCache(normalizedCVE, assessment, finalErr)
			return assessment, finalErr
		}

		if response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= http.StatusInternalServerError {
			response.Body.Close()
			if attempt < maxAttempts {
				sleepErr := sleepWithBackoff(ctx, attempt, apiKeyConfigured)
				if sleepErr != nil {
					return unknownSeverityAssessment(normalizedCVE), sleepErr
				}
				continue
			}

			assessment := unknownSeverityAssessment(normalizedCVE)
			finalErr := retryableNVDStatusError(response.StatusCode, normalizedCVE)
			resolver.writeCache(normalizedCVE, assessment, finalErr)
			return assessment, finalErr
		}

		if response.StatusCode != http.StatusOK {
			response.Body.Close()
			assessment := unknownSeverityAssessment(normalizedCVE)
			finalErr := fmt.Errorf("NVD API returned HTTP %d for %s", response.StatusCode, normalizedCVE)
			resolver.writeCache(normalizedCVE, assessment, finalErr)
			return assessment, finalErr
		}

		var payload nvdResponse
		decodeErr := json.NewDecoder(response.Body).Decode(&payload)
		response.Body.Close()
		if decodeErr != nil {
			assessment := unknownSeverityAssessment(normalizedCVE)
			resolver.writeCache(normalizedCVE, assessment, decodeErr)
			return assessment, decodeErr
		}

		severityValue, score := bestNVDSeverity(payload)
		assessment := severityAssessment{
			Severity: severityValue,
			Score:    score,
			Source:   normalizedCVE,
			Method:   severityMethodNVD,
		}
		if severityValue == severityUnknown {
			finalErr := fmt.Errorf("NVD API returned no severity data for %s", normalizedCVE)
			resolver.writeCache(normalizedCVE, assessment, finalErr)
			return assessment, finalErr
		}
		resolver.writeCache(normalizedCVE, assessment, nil)
		return assessment, nil
	}

	assessment := unknownSeverityAssessment(normalizedCVE)
	finalErr := fmt.Errorf("exhausted NVD resolution attempts for %s", normalizedCVE)
	resolver.writeCache(normalizedCVE, assessment, finalErr)
	return assessment, finalErr
}

func (resolver *nvdSeverityResolver) resolveGHSA(ctx context.Context, ghsaID string) (severityAssessment, error) {
	normalizedGHSA := normalizeID(ghsaID)
	if cached, cachedErr, ok := resolver.readCache(normalizedGHSA); ok {
		return cached, cachedErr
	}

	if resolver.offline {
		assessment := unknownSeverityAssessment(normalizedGHSA)
		err := fmt.Errorf("offline mode enabled and %s requires live GHSA lookup", normalizedGHSA)
		resolver.writeCache(normalizedGHSA, assessment, err)
		return assessment, err
	}

	requestURL, err := advisoryLookupURL(resolver.ghsaBaseURL, normalizedGHSA)
	if err != nil {
		assessment := unknownSeverityAssessment(normalizedGHSA)
		resolver.writeCache(normalizedGHSA, assessment, err)
		return assessment, err
	}

	maxAttempts := 3
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		request, requestErr := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
		if requestErr != nil {
			assessment := unknownSeverityAssessment(normalizedGHSA)
			resolver.writeCache(normalizedGHSA, assessment, requestErr)
			return assessment, requestErr
		}
		request.Header.Set("Accept", "application/vnd.github+json")
		request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
		request.Header.Set("User-Agent", "plato-govuln-policy/1.0")

		tokenConfigured := false
		if resolver.ghsaToken != "" {
			request.Header.Set("Authorization", "Bearer "+resolver.ghsaToken)
			tokenConfigured = true
		}

		response, responseErr := resolver.client.Do(request)
		if responseErr != nil {
			if attempt < maxAttempts {
				sleepErr := sleepWithBackoff(ctx, attempt, tokenConfigured)
				if sleepErr != nil {
					return unknownSeverityAssessment(normalizedGHSA), sleepErr
				}
				continue
			}

			assessment := unknownSeverityAssessment(normalizedGHSA)
			resolver.writeCache(normalizedGHSA, assessment, responseErr)
			return assessment, responseErr
		}

		if response.StatusCode == http.StatusUnauthorized {
			response.Body.Close()
			assessment := unknownSeverityAssessment(normalizedGHSA)
			finalErr := errors.New(ghsa401ErrorMessage)
			resolver.writeCache(normalizedGHSA, assessment, finalErr)
			return assessment, finalErr
		}

		if response.StatusCode == http.StatusForbidden {
			response.Body.Close()
			assessment := unknownSeverityAssessment(normalizedGHSA)
			finalErr := errors.New(ghsa403ErrorMessage)
			resolver.writeCache(normalizedGHSA, assessment, finalErr)
			return assessment, finalErr
		}

		if response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= http.StatusInternalServerError {
			response.Body.Close()
			if attempt < maxAttempts {
				sleepErr := sleepWithBackoff(ctx, attempt, tokenConfigured)
				if sleepErr != nil {
					return unknownSeverityAssessment(normalizedGHSA), sleepErr
				}
				continue
			}

			assessment := unknownSeverityAssessment(normalizedGHSA)
			finalErr := retryableGHSAStatusError(response.StatusCode, normalizedGHSA)
			resolver.writeCache(normalizedGHSA, assessment, finalErr)
			return assessment, finalErr
		}

		if response.StatusCode != http.StatusOK {
			response.Body.Close()
			assessment := unknownSeverityAssessment(normalizedGHSA)
			finalErr := fmt.Errorf("GHSA API returned HTTP %d for %s", response.StatusCode, normalizedGHSA)
			resolver.writeCache(normalizedGHSA, assessment, finalErr)
			return assessment, finalErr
		}

		var payload ghsaResponse
		decodeErr := json.NewDecoder(response.Body).Decode(&payload)
		response.Body.Close()
		if decodeErr != nil {
			assessment := unknownSeverityAssessment(normalizedGHSA)
			resolver.writeCache(normalizedGHSA, assessment, decodeErr)
			return assessment, decodeErr
		}

		assessment := bestGHSASeverity(payload, normalizedGHSA)
		if assessment.Severity == severityUnknown {
			finalErr := fmt.Errorf("GHSA API returned no severity data for %s", normalizedGHSA)
			resolver.writeCache(normalizedGHSA, assessment, finalErr)
			return assessment, finalErr
		}
		resolver.writeCache(normalizedGHSA, assessment, nil)
		return assessment, nil
	}

	assessment := unknownSeverityAssessment(normalizedGHSA)
	finalErr := fmt.Errorf("exhausted GHSA resolution attempts for %s", normalizedGHSA)
	resolver.writeCache(normalizedGHSA, assessment, finalErr)
	return assessment, finalErr
}

func retryableNVDStatusError(statusCode int, cveID string) error {
	if statusCode == http.StatusTooManyRequests {
		return fmt.Errorf(
			"NVD API returned HTTP 429 for %s. This indicates rate limiting. "+
				"Retry later, or configure NVD_API_KEY_FILE or NVD_API_KEY for higher request limits",
			cveID,
		)
	}
	return fmt.Errorf("NVD API returned HTTP %d for %s", statusCode, cveID)
}

func retryableGHSAStatusError(statusCode int, ghsaID string) error {
	if statusCode == http.StatusTooManyRequests {
		return fmt.Errorf(
			"GHSA API returned HTTP 429 for %s. This indicates rate limiting. "+
				"Retry later, use unauthenticated fallback, or configure GHSA_TOKEN_FILE for higher request limits",
			ghsaID,
		)
	}
	return fmt.Errorf("GHSA API returned HTTP %d for %s", statusCode, ghsaID)
}

func unknownSeverityAssessment(source string) severityAssessment {
	return unknownSeverityAssessmentWithReason(source, "", severityMethodUnknown)
}

func unknownSeverityAssessmentWithReason(source, reason string, method severityMethod) severityAssessment {
	return severityAssessment{
		Severity: severityUnknown,
		Source:   source,
		Method:   method,
		Reason:   reason,
	}
}

func advisoryLookupURL(baseURL, advisoryID string) (string, error) {
	trimmedBase := strings.TrimSpace(baseURL)
	if trimmedBase == "" {
		return "", fmt.Errorf("advisory base URL is required")
	}

	parsedURL, err := url.Parse(trimmedBase)
	if err != nil {
		return "", err
	}

	basePath := strings.TrimRight(parsedURL.Path, "/")
	parsedURL.Path = basePath + "/" + url.PathEscape(advisoryID)
	return parsedURL.String(), nil
}

func bestGHSASeverity(payload ghsaResponse, fallbackSource string) severityAssessment {
	source := normalizeID(payload.GHSAID)
	if source == "" {
		source = normalizeID(fallbackSource)
	}

	best := severityAssessment{
		Severity: severityUnknown,
		Source:   source,
		Method:   severityMethodGHSA,
	}

	topLevelScore, hasTopLevelScore := parseScore(payload.CVSS.Score)
	if hasTopLevelScore {
		candidate := severityAssessment{
			Severity: normalizeSeverity(payload.Severity, topLevelScore),
			Score:    topLevelScore,
			Source:   source,
			Method:   severityMethodGHSA,
		}
		if betterSeverity(candidate, best) {
			best = candidate
		}
	} else {
		candidate := severityAssessment{
			Severity: normalizeSeverity(payload.Severity, 0),
			Source:   source,
			Method:   severityMethodGHSA,
		}
		if betterSeverity(candidate, best) {
			best = candidate
		}
	}

	for _, candidate := range []ghsaCVSSData{payload.CVSSSeverities.CVSSV4, payload.CVSSSeverities.CVSSV3} {
		score, _ := parseScore(candidate.Score)
		next := severityAssessment{
			Severity: normalizeSeverity(candidate.Severity, score),
			Score:    score,
			Source:   source,
			Method:   severityMethodGHSA,
		}
		if betterSeverity(next, best) {
			best = next
		}
	}

	return best
}

func parseScore(value interface{}) (float64, bool) {
	switch typedValue := value.(type) {
	case float64:
		return typedValue, true
	case float32:
		return float64(typedValue), true
	case int:
		return float64(typedValue), true
	case int32:
		return float64(typedValue), true
	case int64:
		return float64(typedValue), true
	case json.Number:
		parsedValue, err := typedValue.Float64()
		if err != nil {
			return 0, false
		}
		return parsedValue, true
	case string:
		parsedValue, err := strconv.ParseFloat(strings.TrimSpace(typedValue), 64)
		if err != nil {
			return 0, false
		}
		return parsedValue, true
	default:
		return 0, false
	}
}

func resolveOSVSeverity(osv govulnOSV) (severityAssessment, bool) {
	source := normalizeID(osv.ID)
	best := severityAssessment{
		Severity: severityUnknown,
		Source:   source,
		Method:   severityMethodOSV,
	}

	pushCandidate := func(rawSeverity string, score float64) {
		candidate := severityAssessment{
			Severity: normalizeSeverity(rawSeverity, score),
			Score:    score,
			Source:   source,
			Method:   severityMethodOSV,
		}
		if betterSeverity(candidate, best) {
			best = candidate
		}
	}

	pushCandidate(osv.DatabaseSpecific.Severity, osv.DatabaseSpecific.Score)
	for _, severityValue := range extractOSVSeverityCandidates(osv.Severity) {
		pushCandidate(severityValue.Severity, severityValue.Score)
	}

	if best.Severity == severityUnknown {
		return severityAssessment{}, false
	}
	return best, true
}

type severityCandidate struct {
	Severity string
	Score    float64
}

func extractOSVSeverityCandidates(value interface{}) []severityCandidate {
	switch typedValue := value.(type) {
	case string:
		return []severityCandidate{{Severity: typedValue}}
	case map[string]interface{}:
		return []severityCandidate{candidateFromMap(typedValue)}
	case []interface{}:
		candidates := make([]severityCandidate, 0, len(typedValue))
		for _, item := range typedValue {
			switch nested := item.(type) {
			case string:
				candidates = append(candidates, severityCandidate{Severity: nested})
			case map[string]interface{}:
				candidates = append(candidates, candidateFromMap(nested))
			}
		}
		return candidates
	default:
		return nil
	}
}

func candidateFromMap(value map[string]interface{}) severityCandidate {
	rawSeverity := ""
	rawSeverityValue, hasRawSeverity := value["severity"].(string)
	if hasRawSeverity {
		rawSeverity = rawSeverityValue
	}
	score, hasScore := parseScore(value["score"])
	if hasScore {
		return severityCandidate{Severity: rawSeverity, Score: score}
	}
	scoreText, hasScoreText := value["score"].(string)
	if !hasScoreText {
		return severityCandidate{Severity: rawSeverity}
	}
	return severityCandidate{Severity: rawSeverity, Score: scoreFromCVSSText(scoreText)}
}

func scoreFromCVSSText(rawValue string) float64 {
	parts := strings.Split(rawValue, "/")
	for _, part := range parts {
		trimmedPart := strings.TrimSpace(part)
		if !strings.HasPrefix(trimmedPart, "SCORE:") {
			continue
		}
		parsed, err := strconv.ParseFloat(strings.TrimPrefix(trimmedPart, "SCORE:"), 64)
		if err != nil {
			continue
		}
		return parsed
	}
	return 0
}

func sleepWithBackoff(ctx context.Context, attempt int, apiKeyConfigured bool) error {
	baseDelay := 300 * time.Millisecond
	if !apiKeyConfigured {
		baseDelay = 750 * time.Millisecond
	}
	backoffDelay := baseDelay * time.Duration(1<<(attempt-1))
	// #nosec G404 -- non-cryptographic jitter is sufficient for retry backoff.
	jitter := time.Duration(rand.Int63n(int64(baseDelay / 2)))
	waitFor := backoffDelay + jitter

	timer := time.NewTimer(waitFor)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (resolver *nvdSeverityResolver) readCache(cveID string) (severityAssessment, error, bool) {
	resolver.mu.RLock()
	defer resolver.mu.RUnlock()
	cached, ok := resolver.cache[cveID]
	if !ok {
		return severityAssessment{}, nil, false
	}
	return cached, resolver.errorMap[cveID], true
}

func (resolver *nvdSeverityResolver) writeCache(cveID string, assessment severityAssessment, lookupErr error) {
	resolver.mu.Lock()
	defer resolver.mu.Unlock()
	resolver.cache[cveID] = assessment
	resolver.errorMap[cveID] = lookupErr
}

func addQueryParam(rawURL, key, value string) (string, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	query := parsedURL.Query()
	query.Set(key, value)
	parsedURL.RawQuery = query.Encode()
	return parsedURL.String(), nil
}

func collectCVEIDs(vuln vulnAssessment) []string {
	return collectIDsWithPrefix(vuln, "CVE-")
}

func collectGHSAIDs(vuln vulnAssessment) []string {
	return collectIDsWithPrefix(vuln, "GHSA-")
}

func collectIDsWithPrefix(vuln vulnAssessment, prefix string) []string {
	candidates := make([]string, 0, len(vuln.Aliases)+1)
	candidates = append(candidates, vuln.ID)
	candidates = append(candidates, vuln.Aliases...)

	result := make([]string, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		normalized := normalizeID(candidate)
		if !strings.HasPrefix(normalized, prefix) {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	sort.Strings(result)
	return result
}

func bestNVDSeverity(payload nvdResponse) (severity, float64) {
	bestSeverity := severityUnknown
	bestScore := -1.0

	for _, vulnerability := range payload.Vulnerabilities {
		metrics := make([]nvdMetric, 0)
		metrics = append(metrics, vulnerability.CVE.Metrics.CVSSMetricV31...)
		metrics = append(metrics, vulnerability.CVE.Metrics.CVSSMetricV30...)
		metrics = append(metrics, vulnerability.CVE.Metrics.CVSSMetricV2...)

		for _, metric := range metrics {
			severityValue := normalizeSeverity(metric.CVSSData.BaseSeverity, metric.CVSSData.BaseScore)
			if severityRank(severityValue) > severityRank(bestSeverity) {
				bestSeverity = severityValue
				bestScore = metric.CVSSData.BaseScore
				continue
			}
			if severityRank(severityValue) == severityRank(bestSeverity) && metric.CVSSData.BaseScore > bestScore {
				bestScore = metric.CVSSData.BaseScore
			}
		}
	}

	if bestScore < 0 {
		bestScore = 0
	}
	return bestSeverity, bestScore
}

func normalizeSeverity(raw string, score float64) severity {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case string(severityCritical):
		return severityCritical
	case string(severityHigh):
		return severityHigh
	case string(severityMedium):
		return severityMedium
	case string(severityLow):
		return severityLow
	}

	if score >= 9.0 {
		return severityCritical
	}
	if score >= 7.0 {
		return severityHigh
	}
	if score >= 4.0 {
		return severityMedium
	}
	if score > 0 {
		return severityLow
	}
	return severityUnknown
}

func betterSeverity(left, right severityAssessment) bool {
	leftRank := severityRank(left.Severity)
	rightRank := severityRank(right.Severity)
	if leftRank != rightRank {
		return leftRank > rightRank
	}
	return left.Score > right.Score
}

func severityRank(value severity) int {
	switch value {
	case severityCritical:
		return 4
	case severityHigh:
		return 3
	case severityMedium:
		return 2
	case severityLow:
		return 1
	default:
		return 0
	}
}

func printResult(scanMode string, result evaluationResult) {
	fmt.Printf("govulncheck policy results (%s)\n", scanMode)
	fmt.Printf("  fail: %d\n", len(result.Fail)+len(result.Expired))
	fmt.Printf("  warn: %d\n", len(result.Warn))
	fmt.Printf("  accepted: %d\n", len(result.Accepted))
	fmt.Printf("  info: %d\n", len(result.Info))

	if len(result.Expired) > 0 {
		fmt.Println("")
		fmt.Println("Expired overrides")
		for _, item := range result.Expired {
			fmt.Printf("  - %s override %s expired on %s\n", item.Vuln.ID, item.MatchedByID, item.Override.ExpiresOn.Format("2006-01-02"))
			fmt.Printf("    reason: %s\n", item.Override.Reason)
		}
	}

	if len(result.Fail) > 0 {
		fmt.Println("")
		fmt.Println("Failing vulnerabilities")
		for _, item := range result.Fail {
			printEvaluated(item)
		}
	}

	if len(result.Warn) > 0 {
		fmt.Println("")
		fmt.Println("Warning vulnerabilities")
		for _, item := range result.Warn {
			printEvaluated(item)
		}
	}

	if len(result.Accepted) > 0 {
		fmt.Println("")
		fmt.Println("Accepted risk overrides")
		for _, item := range result.Accepted {
			fmt.Printf("  - %s accepted by %s until %s\n", item.Vuln.ID, item.MatchedByID, item.Override.ExpiresOn.Format("2006-01-02"))
			fmt.Printf("    reason: %s\n", item.Override.Reason)
		}
	}

	if len(result.Info) > 0 {
		fmt.Println("")
		infoLabel := infoHeading(scanMode)
		fmt.Println(infoLabel)
		limit := len(result.Info)
		if limit > 10 {
			limit = 10
		}
		for index := 0; index < limit; index++ {
			item := result.Info[index]
			fmt.Printf("  - %s %s\n", item.Vuln.ID, item.Vuln.Summary)
			if item.Vuln.URL != "" {
				fmt.Printf("    more info: %s\n", item.Vuln.URL)
			}
		}
		if len(result.Info) > limit {
			fmt.Printf("  ... and %d more %s\n", len(result.Info)-limit, strings.ToLower(infoLabel))
		}
	}
}

func infoHeading(scanMode string) string {
	if scanMode == scanModeBinary {
		return "Informational vulnerabilities"
	}
	return "Not reachable vulnerabilities"
}

func printEvaluated(item evaluatedVuln) {
	fmt.Printf("  - %s [%s] %s\n", item.Vuln.ID, item.Severity.Severity, item.Vuln.Summary)
	if item.Severity.Score > 0 {
		fmt.Printf("    cvss score: %.1f\n", item.Severity.Score)
	}
	if item.Severity.Source != "" {
		fmt.Printf("    severity source: %s\n", item.Severity.Source)
	}
	if item.Severity.Method != "" {
		fmt.Printf("    severity method: %s\n", item.Severity.Method)
	}
	if item.Severity.Reason != "" {
		fmt.Printf("    severity reason: %s\n", item.Severity.Reason)
	}
	if len(item.Vuln.FixedVersions) > 0 {
		fmt.Printf("    fixed versions: %s\n", strings.Join(item.Vuln.FixedVersions, ", "))
	}
	if item.Vuln.URL != "" {
		fmt.Printf("    more info: %s\n", item.Vuln.URL)
	}
	if item.ResolverError != nil {
		fmt.Printf("    resolver warning: %v\n", item.ResolverError)
	}
}
