package domain

import (
	"errors"
	"strings"
	"time"
)

// DateLayout is the canonical layout for full dates in API payloads.
const DateLayout = "2006-01-02"

// MonthLayout is the canonical layout for year-month values in API payloads.
const MonthLayout = "2006-01"

const (
	// RoleOrgAdmin grants organisation administrator permissions.
	RoleOrgAdmin = "org_admin"
	// RoleOrgUser grants standard organisation user permissions.
	RoleOrgUser = "org_user"
)

const (
	// ScopeOrganisation scopes a report to the whole organisation.
	ScopeOrganisation = "organisation"
	// ScopePerson scopes a report to one or more people.
	ScopePerson = "person"
	// ScopeGroup scopes a report to one or more groups.
	ScopeGroup = "group"
	// ScopeProject scopes a report to one or more projects.
	ScopeProject = "project"
)

const (
	// AllocationTargetPerson identifies an allocation targeted at a person.
	AllocationTargetPerson = "person"
	// AllocationTargetGroup identifies an allocation targeted at a group.
	AllocationTargetGroup = "group"
)

const (
	// GranularityDay groups report output by day.
	GranularityDay = "day"
	// GranularityWeek groups report output by week.
	GranularityWeek = "week"
	// GranularityMonth groups report output by month.
	GranularityMonth = "month"
	// GranularityYear groups report output by year.
	GranularityYear = "year"
)

var (
	// ErrValidation reports invalid input data.
	ErrValidation = errors.New("validation failed")
	// ErrForbidden reports an authorization failure.
	ErrForbidden = errors.New("forbidden")
	// ErrNotFound reports a missing resource.
	ErrNotFound = errors.New("not found")
)

// Organisation describes an organisation and its working-time baselines.
type Organisation struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	HoursPerDay  float64   `json:"hours_per_day"`
	HoursPerWeek float64   `json:"hours_per_week"`
	HoursPerYear float64   `json:"hours_per_year"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Person describes a person and their employment settings.
type Person struct {
	ID                           string             `json:"id"`
	OrganisationID               string             `json:"organisation_id"`
	Name                         string             `json:"name"`
	EmploymentPct                float64            `json:"employment_pct"`
	EmploymentChanges            []EmploymentChange `json:"employment_changes,omitempty"`
	EmploymentEffectiveFromMonth string             `json:"employment_effective_from_month,omitempty"`
	CreatedAt                    time.Time          `json:"created_at"`
	UpdatedAt                    time.Time          `json:"updated_at"`
}

// EmploymentChange records a person's employment percentage from a month onward.
type EmploymentChange struct {
	EffectiveMonth string  `json:"effective_month"`
	EmploymentPct  float64 `json:"employment_pct"`
}

// Project describes a project tracked within an organisation.
type Project struct {
	ID                   string    `json:"id"`
	OrganisationID       string    `json:"organisation_id"`
	Name                 string    `json:"name"`
	StartDate            string    `json:"start_date"`
	EndDate              string    `json:"end_date"`
	EstimatedEffortHours float64   `json:"estimated_effort_hours"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// Group describes a named group of people within an organisation.
type Group struct {
	ID             string    `json:"id"`
	OrganisationID string    `json:"organisation_id"`
	Name           string    `json:"name"`
	MemberIDs      []string  `json:"member_ids"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// Allocation assigns project effort to a person or a group.
type Allocation struct {
	ID             string    `json:"id"`
	OrganisationID string    `json:"organisation_id"`
	TargetType     string    `json:"target_type"`
	TargetID       string    `json:"target_id"`
	ProjectID      string    `json:"project_id"`
	StartDate      string    `json:"start_date"`
	EndDate        string    `json:"end_date"`
	Percent        float64   `json:"percent"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	// PersonID is kept for compatibility with older local JSON records.
	PersonID string `json:"person_id,omitempty"`
}

// OrgHoliday records organisation-wide unavailable hours for a date.
type OrgHoliday struct {
	ID             string    `json:"id"`
	OrganisationID string    `json:"organisation_id"`
	Date           string    `json:"date"`
	Hours          float64   `json:"hours"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// GroupUnavailability records unavailable hours for a group on a date.
type GroupUnavailability struct {
	ID             string    `json:"id"`
	OrganisationID string    `json:"organisation_id"`
	GroupID        string    `json:"group_id"`
	Date           string    `json:"date"`
	Hours          float64   `json:"hours"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// PersonUnavailability records unavailable hours for a person on a date.
type PersonUnavailability struct {
	ID             string    `json:"id"`
	OrganisationID string    `json:"organisation_id"`
	PersonID       string    `json:"person_id"`
	Date           string    `json:"date"`
	Hours          float64   `json:"hours"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// ReportRequest defines an availability and load report query.
type ReportRequest struct {
	Scope       string   `json:"scope"`
	IDs         []string `json:"ids"`
	FromDate    string   `json:"from_date"`
	ToDate      string   `json:"to_date"`
	Granularity string   `json:"granularity"`
}

// ReportBucket contains aggregated report values for one period.
type ReportBucket struct {
	PeriodStart       string  `json:"period_start"`
	AvailabilityHours float64 `json:"availability_hours"`
	LoadHours         float64 `json:"load_hours"`
	ProjectLoadHours  float64 `json:"project_load_hours"`
	ProjectEstimation float64 `json:"project_estimation_hours"`
	FreeHours         float64 `json:"free_hours"`
	UtilizationPct    float64 `json:"utilization_pct"`
	CompletionPct     float64 `json:"project_completion_pct"`
}

// ValidateDate normalizes and validates a full date string.
func ValidateDate(value string) (string, error) {
	parsed, err := time.Parse(DateLayout, value)
	if err != nil {
		return "", err
	}

	return parsed.Format(DateLayout), nil
}

// ValidateMonth normalizes and validates a year-month string.
func ValidateMonth(value string) (string, error) {
	parsed, err := time.Parse(MonthLayout, value)
	if err != nil {
		return "", err
	}

	return parsed.Format(MonthLayout), nil
}

// ValidateName validates that a name is not blank after trimming.
func ValidateName(name string) error {
	if strings.TrimSpace(name) == "" {
		return ErrValidation
	}

	return nil
}

// ValidatePercent validates that a percentage is within the supported range.
func ValidatePercent(value float64) error {
	if value < 0 || value > 100 {
		return ErrValidation
	}

	return nil
}

// ValidateGranularity validates a report granularity value.
func ValidateGranularity(value string) error {
	switch value {
	case GranularityDay, GranularityWeek, GranularityMonth, GranularityYear:
		return nil
	default:
		return ErrValidation
	}
}

// ValidateScope validates a report scope value.
func ValidateScope(value string) error {
	switch value {
	case ScopeOrganisation, ScopePerson, ScopeGroup, ScopeProject:
		return nil
	default:
		return ErrValidation
	}
}

// ValidateAllocationTargetType validates an allocation target type value.
func ValidateAllocationTargetType(value string) error {
	switch value {
	case AllocationTargetPerson, AllocationTargetGroup:
		return nil
	default:
		return ErrValidation
	}
}

// EmploymentPctOnDate returns the effective employment percentage for a date.
func EmploymentPctOnDate(person Person, date string) (float64, error) {
	normalizedDate, err := ValidateDate(date)
	if err != nil {
		return 0, ErrValidation
	}

	return employmentPctOnMonth(person, normalizedDate[:7])
}

func employmentPctOnMonth(person Person, month string) (float64, error) {
	normalizedMonth, err := ValidateMonth(month)
	if err != nil {
		return 0, ErrValidation
	}

	err = ValidatePercent(person.EmploymentPct)
	if err != nil {
		return 0, ErrValidation
	}

	result := person.EmploymentPct
	latestMonth := ""
	seenMonths := map[string]bool{}
	for _, change := range person.EmploymentChanges {
		effectiveMonth, monthErr := ValidateMonth(change.EffectiveMonth)
		if monthErr != nil {
			return 0, ErrValidation
		}
		if seenMonths[effectiveMonth] {
			return 0, ErrValidation
		}
		seenMonths[effectiveMonth] = true
		if percentErr := ValidatePercent(change.EmploymentPct); percentErr != nil {
			return 0, ErrValidation
		}
		if effectiveMonth <= normalizedMonth && effectiveMonth > latestMonth {
			result = change.EmploymentPct
			latestMonth = effectiveMonth
		}
	}

	return result, nil
}
