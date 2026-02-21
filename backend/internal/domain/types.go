package domain

import (
	"errors"
	"strings"
	"time"
)

const DateLayout = "2006-01-02"
const MonthLayout = "2006-01"

const (
	RoleOrgAdmin = "org_admin"
	RoleOrgUser  = "org_user"
)

const (
	ScopeOrganisation = "organisation"
	ScopePerson       = "person"
	ScopeGroup        = "group"
	ScopeProject      = "project"
)

const (
	AllocationTargetPerson = "person"
	AllocationTargetGroup  = "group"
)

const (
	GranularityDay   = "day"
	GranularityWeek  = "week"
	GranularityMonth = "month"
	GranularityYear  = "year"
)

var (
	ErrValidation = errors.New("validation failed")
	ErrForbidden  = errors.New("forbidden")
	ErrNotFound   = errors.New("not found")
)

type Organisation struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	HoursPerDay  float64   `json:"hours_per_day"`
	HoursPerWeek float64   `json:"hours_per_week"`
	HoursPerYear float64   `json:"hours_per_year"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

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

type EmploymentChange struct {
	EffectiveMonth string  `json:"effective_month"`
	EmploymentPct  float64 `json:"employment_pct"`
}

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

type Group struct {
	ID             string    `json:"id"`
	OrganisationID string    `json:"organisation_id"`
	Name           string    `json:"name"`
	MemberIDs      []string  `json:"member_ids"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

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

type OrgHoliday struct {
	ID             string    `json:"id"`
	OrganisationID string    `json:"organisation_id"`
	Date           string    `json:"date"`
	Hours          float64   `json:"hours"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type GroupUnavailability struct {
	ID             string    `json:"id"`
	OrganisationID string    `json:"organisation_id"`
	GroupID        string    `json:"group_id"`
	Date           string    `json:"date"`
	Hours          float64   `json:"hours"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type PersonUnavailability struct {
	ID             string    `json:"id"`
	OrganisationID string    `json:"organisation_id"`
	PersonID       string    `json:"person_id"`
	Date           string    `json:"date"`
	Hours          float64   `json:"hours"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type ReportRequest struct {
	Scope       string   `json:"scope"`
	IDs         []string `json:"ids"`
	FromDate    string   `json:"from_date"`
	ToDate      string   `json:"to_date"`
	Granularity string   `json:"granularity"`
}

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

func ValidateDate(value string) (string, error) {
	parsed, err := time.Parse(DateLayout, value)
	if err != nil {
		return "", err
	}

	return parsed.Format(DateLayout), nil
}

func ValidateMonth(value string) (string, error) {
	parsed, err := time.Parse(MonthLayout, value)
	if err != nil {
		return "", err
	}

	return parsed.Format(MonthLayout), nil
}

func ValidateName(name string) error {
	if strings.TrimSpace(name) == "" {
		return ErrValidation
	}

	return nil
}

func ValidatePercent(value float64) error {
	if value < 0 || value > 100 {
		return ErrValidation
	}

	return nil
}

func ValidateGranularity(value string) error {
	switch value {
	case GranularityDay, GranularityWeek, GranularityMonth, GranularityYear:
		return nil
	default:
		return ErrValidation
	}
}

func ValidateScope(value string) error {
	switch value {
	case ScopeOrganisation, ScopePerson, ScopeGroup, ScopeProject:
		return nil
	default:
		return ErrValidation
	}
}

func ValidateAllocationTargetType(value string) error {
	switch value {
	case AllocationTargetPerson, AllocationTargetGroup:
		return nil
	default:
		return ErrValidation
	}
}

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

	if err := ValidatePercent(person.EmploymentPct); err != nil {
		return 0, ErrValidation
	}

	result := person.EmploymentPct
	latestMonth := ""
	seenMonths := map[string]bool{}
	for _, change := range person.EmploymentChanges {
		effectiveMonth, err := ValidateMonth(change.EffectiveMonth)
		if err != nil {
			return 0, ErrValidation
		}
		if seenMonths[effectiveMonth] {
			return 0, ErrValidation
		}
		seenMonths[effectiveMonth] = true
		if err := ValidatePercent(change.EmploymentPct); err != nil {
			return 0, ErrValidation
		}
		if effectiveMonth <= normalizedMonth && effectiveMonth > latestMonth {
			result = change.EmploymentPct
			latestMonth = effectiveMonth
		}
	}

	return result, nil
}
