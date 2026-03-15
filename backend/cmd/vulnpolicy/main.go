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
	"path/filepath"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	exitProcess            = os.Exit
	stderrWriter io.Writer = os.Stderr
)

const (
	defaultNVDAPIBaseURL     = "https://services.nvd.nist.gov/rest/json/cves/2.0"
	defaultGHSAAPIBaseURL    = "https://api.github.com/advisories"
	scanModeSource           = "source"
	scanModeBinary           = "binary"
	consoleInfoDisplayCap    = 10
	reportFormatVersion      = "v1"
	reportToolName           = "vulnpolicy"
	unknownUnreachableReason = "Finding is not reachable so severity resolution is skipped by policy"
	unknownOverrideReason    = "Severity resolution is skipped because a risk override matched this finding"
	nvd401ErrorMessage       = "missing or invalid NVD API key, please configure a valid API key"
	nvd403ErrorMessage       = "NVD API key is valid but lacks required permissions, please check your API key configuration"
	ghsa401ErrorMessage      = "missing or invalid GHSA token, remove GHSA_TOKEN_FILE to use unauthenticated access or configure a valid token"
	ghsa403ErrorMessage      = "GHSA token is valid but access is forbidden, check token scope and account permissions"
	errorMessageFormat       = "error: %v"
	dateLayoutISO            = "2006-01-02"
	envNVDAPIKey             = "NVD_API_" + "KEY"
	envGHSAToken             = "GHSA_" + "TOKEN"
	envGitHubToken           = "GITHUB_" + "TOKEN"
	headerAccept             = "Accept"
	headerAuthorization      = "Authorization"
	contentTypeJSON          = "application/json"
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
	ID             string `json:"id"`
	Reason         string `json:"reason"`
	ExpiresOn      string `json:"expires_on"`
	Owner          string `json:"owner"`
	TrackingTicket string `json:"tracking_ticket"`
	Scope          string `json:"scope"`
	ApprovedBy     string `json:"approved_by"`
	ApprovedDate   string `json:"approved_date"`
	Severity       string `json:"severity"`
}

type riskOverride struct {
	ID             string
	Reason         string
	ExpiresOn      time.Time
	Owner          string
	TrackingTicket string
	Scope          string
	ApprovedBy     string
	ApprovedDate   *time.Time
	Severity       severity
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

type reportConfiguration struct {
	InputPath            string `json:"input_path"`
	OverridesPath        string `json:"overrides_path"`
	ExcludeInputPath     string `json:"exclude_input_path,omitempty"`
	SeveritySnapshotPath string `json:"severity_snapshot_path,omitempty"`
	NVDAPIBaseURL        string `json:"nvd_api_base_url"`
	GHSAAPIBaseURL       string `json:"ghsa_api_base_url"`
	NVDTimeout           string `json:"nvd_timeout"`
	Offline              bool   `json:"offline"`
	NVDAPIKeyConfigured  bool   `json:"nvd_api_key_configured"`
	GHSATokenConfigured  bool   `json:"ghsa_token_configured"`
}

type scanReport struct {
	ReportVersion string              `json:"report_version"`
	Metadata      reportMetadata      `json:"metadata"`
	Summary       reportSummary       `json:"summary"`
	Findings      reportFindingGroups `json:"findings"`
	Truncation    reportTruncation    `json:"truncation"`
}

type reportMetadata struct {
	Mode          string              `json:"mode"`
	GeneratedAt   time.Time           `json:"generated_at"`
	Tool          reportTool          `json:"tool"`
	Configuration reportConfiguration `json:"configuration"`
}

type reportTool struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type reportSummary struct {
	Fail     int `json:"fail"`
	Warn     int `json:"warn"`
	Info     int `json:"info"`
	Accepted int `json:"accepted"`
	Expired  int `json:"expired"`
	Blocking int `json:"blocking"`
}

type reportFindingGroups struct {
	Fail     []reportFinding `json:"fail"`
	Warn     []reportFinding `json:"warn"`
	Info     []reportFinding `json:"info"`
	Accepted []reportFinding `json:"accepted"`
	Expired  []reportFinding `json:"expired"`
}

type reportFinding struct {
	ID            string          `json:"id"`
	Aliases       []string        `json:"aliases,omitempty"`
	Summary       string          `json:"summary,omitempty"`
	URL           string          `json:"url,omitempty"`
	FixedVersions []string        `json:"fixed_versions,omitempty"`
	Reachable     bool            `json:"reachable"`
	Severity      *reportSeverity `json:"severity,omitempty"`
	Override      *reportOverride `json:"override,omitempty"`
	MatchedByID   string          `json:"matched_by_id,omitempty"`
	ResolverError string          `json:"resolver_error,omitempty"`
}

type reportSeverity struct {
	Level  severity       `json:"level"`
	Score  float64        `json:"score"`
	Source string         `json:"source,omitempty"`
	Method severityMethod `json:"method,omitempty"`
	Reason string         `json:"reason,omitempty"`
}

type reportOverride struct {
	ID             string     `json:"id"`
	Reason         string     `json:"reason"`
	ExpiresOn      time.Time  `json:"expires_on"`
	Owner          string     `json:"owner"`
	TrackingTicket string     `json:"tracking_ticket"`
	Scope          string     `json:"scope"`
	ApprovedBy     string     `json:"approved_by,omitempty"`
	ApprovedDate   *time.Time `json:"approved_date,omitempty"`
	Severity       severity   `json:"severity,omitempty"`
}

type reportTruncation struct {
	Applied    bool                     `json:"applied"`
	Categories []reportTruncationBucket `json:"categories"`
}

type reportTruncationBucket struct {
	Category     string   `json:"category"`
	ConsoleLabel string   `json:"console_label"`
	ConsoleLimit int      `json:"console_limit"`
	Total        int      `json:"total"`
	Displayed    int      `json:"displayed"`
	Omitted      int      `json:"omitted"`
	OmittedIDs   []string `json:"omitted_ids,omitempty"`
}

func main() {
	flags := registerCLIFlags(flag.CommandLine)
	if err := flag.CommandLine.Parse(os.Args[1:]); err != nil {
		exitf(errorMessageFormat, err)
		return
	}

	config, err := flags.config()
	if err != nil {
		exitf(errorMessageFormat, err)
		return
	}

	outcome, err := runPolicyEvaluation(config)
	if err != nil {
		exitf(errorMessageFormat, err)
		return
	}

	printResult(config.scanMode, outcome.result)
	if err = writeScanReportIfConfigured(config, outcome); err != nil {
		exitf(errorMessageFormat, err)
		return
	}

	if hasBlockingFindings(outcome.result) {
		exitProcess(1)
		return
	}
}

type cliConfig struct {
	inputPath        string
	overridesPath    string
	scanMode         string
	excludeInput     string
	nvdAPIBaseURL    string
	nvdAPIKeyFile    string
	ghsaAPIBaseURL   string
	ghsaTokenFile    string
	severitySnapshot string
	offlineMode      bool
	nvdTimeout       time.Duration
	reportFile       string
}

type policyEvaluationOutcome struct {
	result       evaluationResult
	runTime      time.Time
	apiKeySet    bool
	ghsaTokenSet bool
}

type cliFlags struct {
	inputPath        *string
	overridesPath    *string
	scanMode         *string
	excludeInput     *string
	nvdAPIBaseURL    *string
	nvdAPIKeyFile    *string
	ghsaAPIBaseURL   *string
	ghsaTokenFile    *string
	severitySnapshot *string
	offlineMode      *bool
	nvdTimeout       *time.Duration
	reportFile       *string
}

func registerCLIFlags(flagSet *flag.FlagSet) cliFlags {
	return cliFlags{
		inputPath:        flagSet.String("input", "", "path to govulncheck JSON output"),
		overridesPath:    flagSet.String("overrides", "", "path to vulnerability override config"),
		scanMode:         flagSet.String("scan-mode", scanModeSource, "govulncheck scan mode used by the input: source or binary"),
		excludeInput:     flagSet.String("exclude-input", "", "optional path to govulncheck JSON output whose vulnerabilities should be excluded"),
		nvdAPIBaseURL:    flagSet.String("nvd-api-base-url", defaultNVDAPIBaseURL, "NVD CVE API base URL"),
		nvdAPIKeyFile:    flagSet.String("nvd-api-key-file", "", "path to file containing NVD API key"),
		ghsaAPIBaseURL:   flagSet.String("ghsa-api-base-url", defaultGHSAAPIBaseURL, "GHSA advisory API base URL"),
		ghsaTokenFile:    flagSet.String("ghsa-token-file", "", "path to file containing optional GHSA API token"),
		severitySnapshot: flagSet.String("severity-snapshot", "", "path to pinned NVD severity snapshot JSON"),
		offlineMode:      flagSet.Bool("offline", false, "disable live GHSA and NVD lookups and use pinned snapshot data only"),
		nvdTimeout:       flagSet.Duration("nvd-timeout", 15*time.Second, "timeout per severity API request"),
		reportFile:       flagSet.String("report-file", "", "optional path to write full vulnerability scan report JSON"),
	}
}

func (flags cliFlags) config() (cliConfig, error) {
	trimmedInputPath := strings.TrimSpace(*flags.inputPath)
	if trimmedInputPath == "" {
		return cliConfig{}, errors.New("-input is required")
	}
	trimmedOverridesPath := strings.TrimSpace(*flags.overridesPath)
	if trimmedOverridesPath == "" {
		return cliConfig{}, errors.New("-overrides is required")
	}
	normalizedScanMode, err := normalizeScanMode(*flags.scanMode)
	if err != nil {
		return cliConfig{}, err
	}

	return cliConfig{
		inputPath:        trimmedInputPath,
		overridesPath:    trimmedOverridesPath,
		scanMode:         normalizedScanMode,
		excludeInput:     strings.TrimSpace(*flags.excludeInput),
		nvdAPIBaseURL:    strings.TrimSpace(*flags.nvdAPIBaseURL),
		nvdAPIKeyFile:    strings.TrimSpace(*flags.nvdAPIKeyFile),
		ghsaAPIBaseURL:   strings.TrimSpace(*flags.ghsaAPIBaseURL),
		ghsaTokenFile:    strings.TrimSpace(*flags.ghsaTokenFile),
		severitySnapshot: strings.TrimSpace(*flags.severitySnapshot),
		offlineMode:      *flags.offlineMode,
		nvdTimeout:       *flags.nvdTimeout,
		reportFile:       strings.TrimSpace(*flags.reportFile),
	}, nil
}

func runPolicyEvaluation(config cliConfig) (policyEvaluationOutcome, error) {
	vulns, err := loadInputVulnerabilities(config)
	if err != nil {
		return policyEvaluationOutcome{}, err
	}

	overrides, err := loadOverrides(config.overridesPath)
	if err != nil {
		return policyEvaluationOutcome{}, fmt.Errorf("load overrides: %w", err)
	}

	resolver, apiKey, ghsaToken, err := buildSeverityResolver(config)
	if err != nil {
		return policyEvaluationOutcome{}, err
	}

	runTime := time.Now().UTC()
	result := evaluateVulnerabilities(context.Background(), vulns, overrides, resolver, runTime)
	return policyEvaluationOutcome{
		result:       result,
		runTime:      runTime,
		apiKeySet:    apiKey != "",
		ghsaTokenSet: ghsaToken != "",
	}, nil
}

func loadInputVulnerabilities(config cliConfig) ([]vulnAssessment, error) {
	vulns, err := parseVulnerabilityInput(config.inputPath, config.scanMode)
	if err != nil {
		return nil, err
	}
	if config.excludeInput == "" {
		return vulns, nil
	}

	excludedIDs, excludeErr := collectExcludedIDs(config.excludeInput)
	if excludeErr != nil {
		return nil, fmt.Errorf("load exclude-input: %w", excludeErr)
	}
	return filterExcludedVulnerabilities(vulns, excludedIDs), nil
}

func parseVulnerabilityInput(inputPath, scanMode string) ([]vulnAssessment, error) {
	inputFile, err := os.Open(inputPath)
	if err != nil {
		return nil, fmt.Errorf("open govulncheck output: %w", err)
	}

	vulns, parseErr := parseGovulncheckOutputWithMode(inputFile, scanMode)
	closeErr := inputFile.Close()
	if parseErr != nil {
		return nil, fmt.Errorf("parse govulncheck output: %w", parseErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close govulncheck output: %w", closeErr)
	}
	return vulns, nil
}

func buildSeverityResolver(config cliConfig) (resolver *nvdSeverityResolver, apiKey string, ghsaToken string, err error) {
	apiKey, err = resolveNVDAPIKey(config.nvdAPIKeyFile)
	if err != nil {
		return nil, "", "", fmt.Errorf("resolve NVD API key: %w", err)
	}

	ghsaToken, err = resolveGHSAToken(config.ghsaTokenFile)
	if err != nil {
		return nil, "", "", fmt.Errorf("resolve GHSA token: %w", err)
	}

	snapshot, err := loadSeveritySnapshot(config.severitySnapshot)
	if err != nil {
		return nil, "", "", fmt.Errorf("load severity snapshot: %w", err)
	}
	if config.offlineMode && config.severitySnapshot == "" {
		return nil, "", "", errors.New("-offline requires -severity-snapshot")
	}

	resolver = &nvdSeverityResolver{
		client: &http.Client{
			Timeout: config.nvdTimeout,
		},
		baseURL:     config.nvdAPIBaseURL,
		apiKey:      apiKey,
		ghsaBaseURL: config.ghsaAPIBaseURL,
		ghsaToken:   ghsaToken,
		offline:     config.offlineMode,
		snapshot:    snapshot,
		cache:       make(map[string]severityAssessment),
		errorMap:    make(map[string]error),
	}
	return resolver, apiKey, ghsaToken, nil
}

func writeScanReportIfConfigured(config cliConfig, outcome policyEvaluationOutcome) error {
	if config.reportFile == "" {
		return nil
	}

	report := buildScanReport(config.scanMode, outcome.runTime, outcome.result, reportConfiguration{
		InputPath:            config.inputPath,
		OverridesPath:        config.overridesPath,
		ExcludeInputPath:     config.excludeInput,
		SeveritySnapshotPath: config.severitySnapshot,
		NVDAPIBaseURL:        config.nvdAPIBaseURL,
		GHSAAPIBaseURL:       config.ghsaAPIBaseURL,
		NVDTimeout:           config.nvdTimeout.String(),
		Offline:              config.offlineMode,
		NVDAPIKeyConfigured:  outcome.apiKeySet,
		GHSATokenConfigured:  outcome.ghsaTokenSet,
	})
	if err := writeScanReport(config.reportFile, report); err != nil {
		return fmt.Errorf("write report file: %w", err)
	}
	return nil
}

func hasBlockingFindings(result evaluationResult) bool {
	return len(result.Fail) > 0 || len(result.Expired) > 0
}

func exitf(format string, args ...any) {
	_, _ = fmt.Fprintf(stderrWriter, format+"\n", args...)
	exitProcess(1)
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

		processGovulnEvent(event, scanMode, vulnByID)
	}

	return sortedVulnAssessments(vulnByID), nil
}

func processGovulnEvent(event govulnEvent, scanMode string, vulnByID map[string]*vulnAssessment) {
	if event.OSV != nil {
		updateVulnFromOSV(event.OSV, vulnByID)
	}
	if event.Finding != nil {
		updateVulnFromFinding(event.Finding, scanMode, vulnByID)
	}
}

func updateVulnFromOSV(osv *govulnOSV, vulnByID map[string]*vulnAssessment) {
	entry := ensureVuln(vulnByID, osv.ID)
	entry.Aliases = uniqueStrings(append(entry.Aliases, osv.Aliases...))
	if summary := strings.TrimSpace(osv.Summary); summary != "" {
		entry.Summary = summary
	}
	if databaseURL := strings.TrimSpace(osv.DatabaseSpecific.URL); databaseURL != "" {
		entry.URL = databaseURL
	}
	if severityValue, ok := resolveOSVSeverity(*osv); ok && betterSeverity(severityValue, entry.OSVSeverity) {
		entry.OSVSeverity = severityValue
	}
}

func updateVulnFromFinding(finding *govulnFinding, scanMode string, vulnByID map[string]*vulnAssessment) {
	entry := ensureVuln(vulnByID, finding.OSV)
	if fixed := strings.TrimSpace(finding.FixedVersion); fixed != "" {
		entry.FixedVersions = uniqueStrings(append(entry.FixedVersions, fixed))
	}
	if scanMode == scanModeBinary || findingIsReachable(finding) {
		entry.Reachable = true
	}
}

func sortedVulnAssessments(vulnByID map[string]*vulnAssessment) []vulnAssessment {
	result := make([]vulnAssessment, 0, len(vulnByID))
	for _, vuln := range vulnByID {
		sort.Strings(vuln.Aliases)
		sort.Strings(vuln.FixedVersions)
		result = append(result, *vuln)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})

	return result
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

func matchesExcludedIDs(vuln vulnAssessment, excludedIDs excludedVulnerabilityIDs) (matchedAll bool, matchedReachable bool) {
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
		override, parseErr := parseOverrideInput(item)
		if parseErr != nil {
			return nil, parseErr
		}
		if _, exists := overrides[override.ID]; exists {
			return nil, fmt.Errorf("duplicate override id: %s", override.ID)
		}
		overrides[override.ID] = override
	}

	return overrides, nil
}

func parseOverrideInput(item overrideInput) (riskOverride, error) {
	id := normalizeID(item.ID)
	if id == "" {
		return riskOverride{}, errors.New("override id is required")
	}

	reason, err := requiredOverrideField(id, "reason", item.Reason)
	if err != nil {
		return riskOverride{}, err
	}
	owner, err := requiredOverrideField(id, "owner", item.Owner)
	if err != nil {
		return riskOverride{}, err
	}
	trackingTicket, err := requiredOverrideField(id, "tracking_ticket", item.TrackingTicket)
	if err != nil {
		return riskOverride{}, err
	}
	scope, err := requiredOverrideField(id, "scope", item.Scope)
	if err != nil {
		return riskOverride{}, err
	}
	expiresOn, err := requiredOverrideField(id, "expires_on", item.ExpiresOn)
	if err != nil {
		return riskOverride{}, err
	}

	expiryDate, err := parseOverrideDate(id, "expires_on", expiresOn)
	if err != nil {
		return riskOverride{}, err
	}
	approvedDate, hasApprovedDate, err := parseOptionalOverrideDate(id, "approved_date", item.ApprovedDate)
	if err != nil {
		return riskOverride{}, err
	}
	overrideSeverity, err := parseOptionalOverrideSeverity(id, item.Severity)
	if err != nil {
		return riskOverride{}, err
	}
	var approvedDatePtr *time.Time
	if hasApprovedDate {
		approvedDatePtr = &approvedDate
	}

	return riskOverride{
		ID:             id,
		Reason:         reason,
		ExpiresOn:      expiryDate.UTC(),
		Owner:          owner,
		TrackingTicket: trackingTicket,
		Scope:          scope,
		ApprovedBy:     strings.TrimSpace(item.ApprovedBy),
		ApprovedDate:   approvedDatePtr,
		Severity:       overrideSeverity,
	}, nil
}

func requiredOverrideField(id, name, rawValue string) (string, error) {
	value := strings.TrimSpace(rawValue)
	if value == "" {
		return "", fmt.Errorf("override %s must include %s", id, name)
	}
	return value, nil
}

func parseOverrideDate(id, name, rawValue string) (time.Time, error) {
	date, err := time.Parse(dateLayoutISO, rawValue)
	if err != nil {
		return time.Time{}, fmt.Errorf("override %s has invalid %s %q: %w", id, name, rawValue, err)
	}
	return date, nil
}

func parseOptionalOverrideDate(id, name, rawValue string) (time.Time, bool, error) {
	trimmed := strings.TrimSpace(rawValue)
	if trimmed == "" {
		return time.Time{}, false, nil
	}
	date, err := parseOverrideDate(id, name, trimmed)
	if err != nil {
		return time.Time{}, false, err
	}
	return date.UTC(), true, nil
}

func parseOptionalOverrideSeverity(id, rawValue string) (severity, error) {
	trimmed := strings.TrimSpace(rawValue)
	if trimmed == "" {
		return "", nil
	}
	parsedSeverity, err := parseOverrideSeverity(trimmed)
	if err != nil {
		return "", fmt.Errorf("override %s has invalid severity %q: %w", id, trimmed, err)
	}
	return parsedSeverity, nil
}

func parseOverrideSeverity(raw string) (severity, error) {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case string(severityUnknown):
		return severityUnknown, nil
	case string(severityLow):
		return severityLow, nil
	case string(severityMedium):
		return severityMedium, nil
	case string(severityHigh):
		return severityHigh, nil
	case string(severityCritical):
		return severityCritical, nil
	default:
		return "", errors.New("must be one of LOW, MEDIUM, HIGH, CRITICAL, UNKNOWN")
	}
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
				Severity:    overrideBypassSeverity(vuln, matchedByID),
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
			result.Info = append(result.Info, evaluatedVuln{
				Vuln:     vuln,
				Severity: unreachableSeverity(vuln),
			})
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

func unreachableSeverity(vuln vulnAssessment) severityAssessment {
	return unknownSeverityAssessmentWithReason(
		normalizeID(vuln.ID),
		unknownUnreachableReason,
		severityMethodUnknown,
	)
}

func overrideBypassSeverity(vuln vulnAssessment, matchedByID string) severityAssessment {
	source := normalizeID(matchedByID)
	if source == "" {
		source = normalizeID(vuln.ID)
	}
	return unknownSeverityAssessmentWithReason(
		source,
		unknownOverrideReason,
		severityMethodUnknown,
	)
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
		return strings.TrimSpace(os.Getenv(envNVDAPIKey)), nil
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
		token := strings.TrimSpace(os.Getenv(envGHSAToken))
		if token != "" {
			return token, nil
		}
		return strings.TrimSpace(os.Getenv(envGitHubToken)), nil
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

// Resolve looks up the best available severity assessment for a vulnerability.
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
	if cached, ok, cachedErr := resolver.readCache(normalizedCVE); ok {
		return cached, cachedErr
	}

	if snapshotSeverity, ok := resolver.snapshot[normalizedCVE]; ok {
		resolver.writeCache(normalizedCVE, snapshotSeverity, nil)
		return snapshotSeverity, nil
	}

	if resolver.offline {
		return resolver.cacheUnknownWithError(normalizedCVE, fmt.Errorf("offline mode enabled and %s is missing from severity snapshot", normalizedCVE))
	}

	requestURL, err := addQueryParam(resolver.baseURL, "cveId", normalizedCVE)
	if err != nil {
		return resolver.cacheUnknownWithError(normalizedCVE, err)
	}

	return resolver.resolveCVEWithRetry(ctx, normalizedCVE, requestURL)
}

func (resolver *nvdSeverityResolver) resolveGHSA(ctx context.Context, ghsaID string) (severityAssessment, error) {
	normalizedGHSA := normalizeID(ghsaID)
	if cached, ok, cachedErr := resolver.readCache(normalizedGHSA); ok {
		return cached, cachedErr
	}

	if resolver.offline {
		return resolver.cacheUnknownWithError(normalizedGHSA, fmt.Errorf("offline mode enabled and %s requires live GHSA lookup", normalizedGHSA))
	}

	requestURL, err := advisoryLookupURL(resolver.ghsaBaseURL, normalizedGHSA)
	if err != nil {
		return resolver.cacheUnknownWithError(normalizedGHSA, err)
	}

	return resolver.resolveGHSAWithRetry(ctx, normalizedGHSA, requestURL)
}

func (resolver *nvdSeverityResolver) cacheUnknownWithError(id string, err error) (severityAssessment, error) {
	assessment := unknownSeverityAssessment(id)
	resolver.writeCache(id, assessment, err)
	return assessment, err
}

func (resolver *nvdSeverityResolver) resolveCVEWithRetry(ctx context.Context, normalizedCVE, requestURL string) (severityAssessment, error) {
	const maxAttempts = 3

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		request, apiKeyConfigured, err := resolver.newNVDRequest(ctx, requestURL)
		if err != nil {
			return resolver.cacheUnknownWithError(normalizedCVE, err)
		}
		response, err := resolver.client.Do(request)
		if err != nil {
			retry, retryAssessment, retryErr := resolver.retryOrCacheUnknown(ctx, attempt, maxAttempts, apiKeyConfigured, normalizedCVE, err)
			if retry {
				continue
			}
			return retryAssessment, retryErr
		}

		assessment, shouldRetry, responseErr := resolver.handleNVDResponse(response, normalizedCVE)
		if shouldRetry {
			retry, retryAssessment, retryErr := resolver.retryOrCacheUnknown(ctx, attempt, maxAttempts, apiKeyConfigured, normalizedCVE, responseErr)
			if retry {
				continue
			}
			return retryAssessment, retryErr
		}

		resolver.writeCache(normalizedCVE, assessment, responseErr)
		return assessment, responseErr
	}

	return resolver.cacheUnknownWithError(normalizedCVE, fmt.Errorf("exhausted NVD resolution attempts for %s", normalizedCVE))
}

func (resolver *nvdSeverityResolver) retryOrCacheUnknown(
	ctx context.Context,
	attempt int,
	maxAttempts int,
	credentialConfigured bool,
	id string,
	err error,
) (bool, severityAssessment, error) {
	if attempt < maxAttempts {
		if sleepErr := sleepWithBackoff(ctx, attempt, credentialConfigured); sleepErr != nil {
			return false, unknownSeverityAssessment(id), sleepErr
		}
		return true, severityAssessment{}, nil
	}
	assessment, cachedErr := resolver.cacheUnknownWithError(id, err)
	return false, assessment, cachedErr
}

func (resolver *nvdSeverityResolver) newNVDRequest(ctx context.Context, requestURL string) (*http.Request, bool, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, false, err
	}
	request.Header.Set(headerAccept, contentTypeJSON)
	request.Header.Set("User-Agent", "plato-govuln-policy/1.0")
	if resolver.apiKey == "" {
		return request, false, nil
	}
	request.Header.Set("apiKey", resolver.apiKey)
	return request, true, nil
}

func (resolver *nvdSeverityResolver) handleNVDResponse(response *http.Response, normalizedCVE string) (severityAssessment, bool, error) {
	defer response.Body.Close()

	unknown := unknownSeverityAssessment(normalizedCVE)
	switch response.StatusCode {
	case http.StatusUnauthorized:
		return unknown, false, errors.New(nvd401ErrorMessage)
	case http.StatusForbidden:
		return unknown, false, errors.New(nvd403ErrorMessage)
	}

	if shouldRetrySeverityStatus(response.StatusCode) {
		return severityAssessment{}, true, retryableNVDStatusError(response.StatusCode, normalizedCVE)
	}
	if response.StatusCode != http.StatusOK {
		return unknown, false, fmt.Errorf("NVD API returned HTTP %d for %s", response.StatusCode, normalizedCVE)
	}

	var payload nvdResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return unknown, false, err
	}

	severityValue, score := bestNVDSeverity(payload)
	assessment := severityAssessment{
		Severity: severityValue,
		Score:    score,
		Source:   normalizedCVE,
		Method:   severityMethodNVD,
	}
	if severityValue == severityUnknown {
		return assessment, false, fmt.Errorf("NVD API returned no severity data for %s", normalizedCVE)
	}
	return assessment, false, nil
}

func (resolver *nvdSeverityResolver) resolveGHSAWithRetry(ctx context.Context, normalizedGHSA, requestURL string) (severityAssessment, error) {
	const maxAttempts = 3

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		request, tokenConfigured, err := resolver.newGHSARequest(ctx, requestURL)
		if err != nil {
			return resolver.cacheUnknownWithError(normalizedGHSA, err)
		}
		response, err := resolver.client.Do(request)
		if err != nil {
			retry, retryAssessment, retryErr := resolver.retryOrCacheUnknown(ctx, attempt, maxAttempts, tokenConfigured, normalizedGHSA, err)
			if retry {
				continue
			}
			return retryAssessment, retryErr
		}

		assessment, shouldRetry, responseErr := resolver.handleGHSAResponse(response, normalizedGHSA)
		if shouldRetry {
			retry, retryAssessment, retryErr := resolver.retryOrCacheUnknown(ctx, attempt, maxAttempts, tokenConfigured, normalizedGHSA, responseErr)
			if retry {
				continue
			}
			return retryAssessment, retryErr
		}

		resolver.writeCache(normalizedGHSA, assessment, responseErr)
		return assessment, responseErr
	}

	return resolver.cacheUnknownWithError(normalizedGHSA, fmt.Errorf("exhausted GHSA resolution attempts for %s", normalizedGHSA))
}

func (resolver *nvdSeverityResolver) newGHSARequest(ctx context.Context, requestURL string) (*http.Request, bool, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, false, err
	}
	request.Header.Set(headerAccept, "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	request.Header.Set("User-Agent", "plato-govuln-policy/1.0")
	if resolver.ghsaToken == "" {
		return request, false, nil
	}
	request.Header.Set(headerAuthorization, "Bearer "+resolver.ghsaToken)
	return request, true, nil
}

func (resolver *nvdSeverityResolver) handleGHSAResponse(response *http.Response, normalizedGHSA string) (severityAssessment, bool, error) {
	defer response.Body.Close()

	unknown := unknownSeverityAssessment(normalizedGHSA)
	switch response.StatusCode {
	case http.StatusUnauthorized:
		return unknown, false, errors.New(ghsa401ErrorMessage)
	case http.StatusForbidden:
		return unknown, false, errors.New(ghsa403ErrorMessage)
	}

	if shouldRetrySeverityStatus(response.StatusCode) {
		return severityAssessment{}, true, retryableGHSAStatusError(response.StatusCode, normalizedGHSA)
	}
	if response.StatusCode != http.StatusOK {
		return unknown, false, fmt.Errorf("GHSA API returned HTTP %d for %s", response.StatusCode, normalizedGHSA)
	}

	var payload ghsaResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return unknown, false, err
	}

	assessment := bestGHSASeverity(payload, normalizedGHSA)
	if assessment.Severity == severityUnknown {
		return assessment, false, fmt.Errorf("GHSA API returned no severity data for %s", normalizedGHSA)
	}
	return assessment, false, nil
}

func shouldRetrySeverityStatus(statusCode int) bool {
	return statusCode == http.StatusTooManyRequests || statusCode >= http.StatusInternalServerError
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
		return "", errors.New("advisory base URL is required")
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

func (resolver *nvdSeverityResolver) readCache(cveID string) (severityAssessment, bool, error) {
	resolver.mu.RLock()
	defer resolver.mu.RUnlock()
	cached, ok := resolver.cache[cveID]
	if !ok {
		return severityAssessment{}, false, nil
	}
	return cached, true, resolver.errorMap[cveID]
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

func buildScanReport(scanMode string, generatedAt time.Time, result evaluationResult, configuration reportConfiguration) scanReport {
	return scanReport{
		ReportVersion: reportFormatVersion,
		Metadata: reportMetadata{
			Mode:        scanMode,
			GeneratedAt: generatedAt,
			Tool: reportTool{
				Name:    reportToolName,
				Version: currentToolVersion(),
			},
			Configuration: configuration,
		},
		Summary: reportSummary{
			Fail:     len(result.Fail),
			Warn:     len(result.Warn),
			Info:     len(result.Info),
			Accepted: len(result.Accepted),
			Expired:  len(result.Expired),
			Blocking: len(result.Fail) + len(result.Expired),
		},
		Findings: reportFindingGroups{
			Fail:     reportFindingsFromEvaluated(result.Fail),
			Warn:     reportFindingsFromEvaluated(result.Warn),
			Info:     reportFindingsFromEvaluated(result.Info),
			Accepted: reportFindingsFromEvaluated(result.Accepted),
			Expired:  reportFindingsFromEvaluated(result.Expired),
		},
		Truncation: buildTruncationReport(scanMode, result),
	}
}

func currentToolVersion() string {
	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		return "devel"
	}
	version := strings.TrimSpace(buildInfo.Main.Version)
	if version == "" {
		return "devel"
	}
	return version
}

func buildTruncationReport(scanMode string, result evaluationResult) reportTruncation {
	displayed := len(result.Info)
	if displayed > consoleInfoDisplayCap {
		displayed = consoleInfoDisplayCap
	}
	omitted := len(result.Info) - displayed
	return reportTruncation{
		Applied: omitted > 0,
		Categories: []reportTruncationBucket{
			{
				Category:     "info",
				ConsoleLabel: infoHeading(scanMode),
				ConsoleLimit: consoleInfoDisplayCap,
				Total:        len(result.Info),
				Displayed:    displayed,
				Omitted:      omitted,
				OmittedIDs:   truncatedInfoIDs(result.Info, displayed),
			},
		},
	}
}

func truncatedInfoIDs(infoFindings []evaluatedVuln, displayed int) []string {
	if displayed >= len(infoFindings) {
		return nil
	}
	omittedIDs := make([]string, 0, len(infoFindings)-displayed)
	for index := displayed; index < len(infoFindings); index++ {
		omittedIDs = append(omittedIDs, infoFindings[index].Vuln.ID)
	}
	return omittedIDs
}

func reportFindingsFromEvaluated(items []evaluatedVuln) []reportFinding {
	findings := make([]reportFinding, 0, len(items))
	for _, item := range items {
		findings = append(findings, reportFindingFromEvaluated(item))
	}
	return findings
}

func reportFindingFromEvaluated(item evaluatedVuln) reportFinding {
	resolvedSeverity := reportSeverityFromEvaluated(item)
	reportItem := reportFinding{
		ID:            item.Vuln.ID,
		Aliases:       append([]string(nil), item.Vuln.Aliases...),
		Summary:       item.Vuln.Summary,
		URL:           item.Vuln.URL,
		FixedVersions: append([]string(nil), item.Vuln.FixedVersions...),
		Reachable:     item.Vuln.Reachable,
		MatchedByID:   item.MatchedByID,
	}
	if item.ResolverError != nil {
		reportItem.ResolverError = item.ResolverError.Error()
	}
	reportItem.Severity = &reportSeverity{
		Level:  resolvedSeverity.Severity,
		Score:  resolvedSeverity.Score,
		Source: resolvedSeverity.Source,
		Method: resolvedSeverity.Method,
		Reason: resolvedSeverity.Reason,
	}
	if item.Override != nil {
		reportItem.Override = &reportOverride{
			ID:             item.Override.ID,
			Reason:         item.Override.Reason,
			ExpiresOn:      item.Override.ExpiresOn,
			Owner:          item.Override.Owner,
			TrackingTicket: item.Override.TrackingTicket,
			Scope:          item.Override.Scope,
			ApprovedBy:     item.Override.ApprovedBy,
			ApprovedDate:   item.Override.ApprovedDate,
			Severity:       item.Override.Severity,
		}
	}
	return reportItem
}

func reportSeverityFromEvaluated(item evaluatedVuln) severityAssessment {
	assessment := item.Severity

	if assessment.Severity == "" {
		assessment.Severity = severityUnknown
	}

	if strings.TrimSpace(assessment.Source) == "" {
		assessment.Source = normalizeID(item.Vuln.ID)
	}

	if assessment.Severity == severityUnknown {
		if assessment.Method == "" {
			assessment.Method = severityMethodUnknown
		}
		if strings.TrimSpace(assessment.Reason) == "" {
			assessment.Reason = unknownSeverityReasonForReport(item)
		}
	}

	return assessment
}

func unknownSeverityReasonForReport(item evaluatedVuln) string {
	if item.Override != nil {
		return unknownOverrideReason
	}
	if !item.Vuln.Reachable {
		return unknownUnreachableReason
	}
	return "Severity resolution produced UNKNOWN without an explicit reason"
}

func hasReportSeverity(assessment severityAssessment) bool {
	if assessment.Severity != "" {
		return true
	}
	if assessment.Score > 0 {
		return true
	}
	if assessment.Source != "" {
		return true
	}
	if assessment.Method != "" {
		return true
	}
	return assessment.Reason != ""
}

func writeScanReport(path string, report scanReport) error {
	reportPath := strings.TrimSpace(path)
	if reportPath == "" {
		return nil
	}
	reportDir := filepath.Dir(reportPath)
	if reportDir != "." {
		if mkdirErr := os.MkdirAll(reportDir, 0o755); mkdirErr != nil {
			return mkdirErr
		}
	}
	reportData, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	reportData = append(reportData, '\n')
	return os.WriteFile(reportPath, reportData, 0o600)
}

func printResult(scanMode string, result evaluationResult) {
	fmt.Printf("govulncheck policy results (%s)\n", scanMode)
	fmt.Printf("  fail: %d\n", len(result.Fail)+len(result.Expired))
	fmt.Printf("  warn: %d\n", len(result.Warn))
	fmt.Printf("  accepted: %d\n", len(result.Accepted))
	fmt.Printf("  info: %d\n", len(result.Info))

	printExpiredOverrides(result.Expired)
	printEvaluatedVulnerabilitySection("Failing vulnerabilities", result.Fail)
	printEvaluatedVulnerabilitySection("Warning vulnerabilities", result.Warn)
	printAcceptedOverrides(result.Accepted)
	printInformationalFindings(scanMode, result.Info)
}

func printExpiredOverrides(items []evaluatedVuln) {
	if len(items) == 0 {
		return
	}

	fmt.Println("")
	fmt.Println("Expired overrides")
	for _, item := range items {
		fmt.Printf("  - %s override %s expired on %s\n", item.Vuln.ID, item.MatchedByID, item.Override.ExpiresOn.Format(dateLayoutISO))
		fmt.Printf("    reason: %s\n", item.Override.Reason)
	}
}

func printEvaluatedVulnerabilitySection(title string, items []evaluatedVuln) {
	if len(items) == 0 {
		return
	}

	fmt.Println("")
	fmt.Println(title)
	for _, item := range items {
		printEvaluated(item)
	}
}

func printAcceptedOverrides(items []evaluatedVuln) {
	if len(items) == 0 {
		return
	}

	fmt.Println("")
	fmt.Println("Accepted risk overrides")
	for _, item := range items {
		fmt.Printf("  - %s accepted by %s until %s\n", item.Vuln.ID, item.MatchedByID, item.Override.ExpiresOn.Format(dateLayoutISO))
		fmt.Printf("    reason: %s\n", item.Override.Reason)
	}
}

func printInformationalFindings(scanMode string, items []evaluatedVuln) {
	if len(items) == 0 {
		return
	}

	fmt.Println("")
	infoLabel := infoHeading(scanMode)
	fmt.Println(infoLabel)

	limit := len(items)
	if limit > consoleInfoDisplayCap {
		limit = consoleInfoDisplayCap
	}
	for index := 0; index < limit; index++ {
		item := items[index]
		fmt.Printf("  - %s %s\n", item.Vuln.ID, item.Vuln.Summary)
		if item.Vuln.URL != "" {
			fmt.Printf("    more info: %s\n", item.Vuln.URL)
		}
	}
	if len(items) > limit {
		fmt.Printf("  ... and %d more %s\n", len(items)-limit, strings.ToLower(infoLabel))
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
