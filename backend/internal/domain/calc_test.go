package domain

import (
	"errors"
	"math"
	"testing"
	"time"
)

func TestCalculateAvailabilityLoadPersonScopeWithHolidaysAndOverrides(t *testing.T) {
	input := CalculationInput{
		Organisation: Organisation{
			ID:           "org-1",
			HoursPerDay:  8,
			HoursPerWeek: 40,
			HoursPerYear: 2080,
		},
		Persons: []Person{{ID: "p1", OrganisationID: "org-1", EmploymentPct: 100}},
		Groups: []Group{{
			ID:             "g1",
			OrganisationID: "org-1",
			MemberIDs:      []string{"p1"},
		}},
		Projects: []Project{testProject("pr1")},
		Allocations: []Allocation{
			personAllocationEntry("a1", "p1", "pr1", 50, "2026-01-01", "2026-01-31"),
		},
		OrgHolidays: []OrgHoliday{{ID: "h1", OrganisationID: "org-1", Date: "2026-01-02", Hours: 8}},
		GroupUnavailability: []GroupUnavailability{{
			ID:             "gu1",
			OrganisationID: "org-1",
			GroupID:        "g1",
			Date:           "2026-01-03",
			Hours:          4,
		}},
		PersonUnavailability: []PersonUnavailability{{
			ID:             "pu1",
			OrganisationID: "org-1",
			PersonID:       "p1",
			Date:           "2026-01-03",
			Hours:          2,
		}},
		Request: ReportRequest{
			Scope:       ScopePerson,
			IDs:         []string{"p1"},
			FromDate:    "2026-01-01",
			ToDate:      "2026-01-03",
			Granularity: GranularityDay,
		},
	}

	result, err := CalculateAvailabilityLoad(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 3 {
		t.Fatalf("expected 3 buckets, got %d", len(result))
	}

	assertBucket(t, result[0], "2026-01-01", 8, 4, 4)
	assertBucket(t, result[1], "2026-01-02", 0, 4, -4)
	assertBucket(t, result[2], "2026-01-03", 2, 4, -2)
}

func TestCalculateAvailabilityLoadGroupScopeMonthAggregation(t *testing.T) {
	input := CalculationInput{
		Organisation: Organisation{
			ID:           "org-1",
			HoursPerDay:  8,
			HoursPerWeek: 40,
			HoursPerYear: 2080,
		},
		Persons: []Person{
			{ID: "p1", OrganisationID: "org-1", EmploymentPct: 100},
			{ID: "p2", OrganisationID: "org-1", EmploymentPct: 50},
		},
		Groups:   []Group{{ID: "g1", OrganisationID: "org-1", MemberIDs: []string{"p1", "p2"}}},
		Projects: []Project{testProject("pr1"), testProject("pr2")},
		Allocations: []Allocation{
			personAllocationEntry("a1", "p1", "pr1", 25, "2026-01-01", "2026-01-31"),
			groupAllocation("a2", "g1", "pr2", 20, "2026-01-01", "2026-01-31"),
		},
		Request: ReportRequest{
			Scope:       ScopeGroup,
			IDs:         []string{"g1"},
			FromDate:    "2026-01-01",
			ToDate:      "2026-01-02",
			Granularity: GranularityMonth,
		},
	}

	result, err := CalculateAvailabilityLoad(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 bucket, got %d", len(result))
	}

	assertBucket(t, result[0], "2026-01-01", 24, 10.4, 13.6)
	if !approxEqual(43.33, result[0].UtilizationPct, 0.01) {
		t.Fatalf("expected utilization 43.33 +/- 0.01, got %v", result[0].UtilizationPct)
	}
}

func TestCalculateAvailabilityLoadProjectScopeFiltersAllocationAndRange(t *testing.T) {
	input := CalculationInput{
		Organisation: Organisation{ID: "org-1", HoursPerDay: 8, HoursPerWeek: 40, HoursPerYear: 2080},
		Persons:      []Person{{ID: "p1", OrganisationID: "org-1", EmploymentPct: 100}},
		Projects:     []Project{testProject("pr1"), testProject("pr2")},
		Allocations: []Allocation{
			personAllocationEntry("a1", "p1", "pr1", 60, "2026-01-01", "2026-01-31"),
			personAllocationEntry("a2", "p1", "pr1", 30, "2026-02-01", "2026-02-28"),
			personAllocationEntry("a3", "p1", "pr2", 20, "2026-01-01", "2026-01-31"),
		},
		Request: ReportRequest{Scope: ScopeProject, IDs: []string{"pr1"}, FromDate: "2026-01-10", ToDate: "2026-01-10", Granularity: GranularityDay},
	}

	result, err := CalculateAvailabilityLoad(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 bucket, got %d", len(result))
	}

	assertBucket(t, result[0], "2026-01-10", 8, 4.8, 3.2)
	if result[0].ProjectEstimation != 1000 {
		t.Fatalf("expected project estimation 1000, got %v", result[0].ProjectEstimation)
	}
	if result[0].ProjectLoadHours != 4.8 {
		t.Fatalf("expected project load 4.8, got %v", result[0].ProjectLoadHours)
	}
	if result[0].CompletionPct != 0.48 {
		t.Fatalf("expected project completion 0.48, got %v", result[0].CompletionPct)
	}
}

func TestCalculateAvailabilityLoadProjectScopeIncludesGroupAllocations(t *testing.T) {
	input := CalculationInput{
		Organisation: Organisation{ID: "org-1", HoursPerDay: 8, HoursPerWeek: 40, HoursPerYear: 2080},
		Persons: []Person{
			{ID: "p1", OrganisationID: "org-1", EmploymentPct: 100},
			{ID: "p2", OrganisationID: "org-1", EmploymentPct: 100},
		},
		Groups: []Group{
			{ID: "g1", OrganisationID: "org-1", MemberIDs: []string{"p1", "p2"}},
		},
		Projects: []Project{testProject("pr1")},
		Allocations: []Allocation{
			groupAllocation("a1", "g1", "pr1", 50, "2026-01-01", "2026-01-31"),
		},
		Request: ReportRequest{Scope: ScopeProject, IDs: []string{"pr1"}, FromDate: "2026-01-10", ToDate: "2026-01-10", Granularity: GranularityDay},
	}

	result, err := CalculateAvailabilityLoad(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 bucket, got %d", len(result))
	}

	assertBucket(t, result[0], "2026-01-10", 16, 8, 8)
	if result[0].ProjectEstimation != 1000 {
		t.Fatalf("expected project estimation 1000, got %v", result[0].ProjectEstimation)
	}
	if result[0].ProjectLoadHours != 8 {
		t.Fatalf("expected project load 8, got %v", result[0].ProjectLoadHours)
	}
	if result[0].CompletionPct != 0.8 {
		t.Fatalf("expected project completion 0.8, got %v", result[0].CompletionPct)
	}
}

func TestCalculateAvailabilityLoadProjectScopeUsesCumulativeProjectLoadForCompletion(t *testing.T) {
	input := CalculationInput{
		Organisation: Organisation{ID: "org-1", HoursPerDay: 8, HoursPerWeek: 40, HoursPerYear: 2080},
		Persons:      []Person{{ID: "p1", OrganisationID: "org-1", EmploymentPct: 100}},
		Projects: []Project{{
			ID:                   "pr1",
			OrganisationID:       "org-1",
			Name:                 "pr1",
			StartDate:            "2026-01-01",
			EndDate:              "2026-12-31",
			EstimatedEffortHours: 16,
		}},
		Allocations: []Allocation{
			personAllocationEntry("a1", "p1", "pr1", 50, "2026-01-01", "2026-01-31"),
		},
		Request: ReportRequest{
			Scope:       ScopeProject,
			IDs:         []string{"pr1"},
			FromDate:    "2026-01-01",
			ToDate:      "2026-01-02",
			Granularity: GranularityDay,
		},
	}

	result, err := CalculateAvailabilityLoad(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 buckets, got %d", len(result))
	}

	assertBucket(t, result[0], "2026-01-01", 8, 4, 4)
	if result[0].ProjectLoadHours != 4 {
		t.Fatalf("expected day 1 cumulative project load 4, got %v", result[0].ProjectLoadHours)
	}
	if result[0].CompletionPct != 25 {
		t.Fatalf("expected day 1 completion 25, got %v", result[0].CompletionPct)
	}

	assertBucket(t, result[1], "2026-01-02", 8, 4, 4)
	if result[1].ProjectLoadHours != 8 {
		t.Fatalf("expected day 2 cumulative project load 8, got %v", result[1].ProjectLoadHours)
	}
	if result[1].CompletionPct != 50 {
		t.Fatalf("expected day 2 completion 50, got %v", result[1].CompletionPct)
	}
}

func TestCalculateAvailabilityLoadAllocationsUseFullTimeScale(t *testing.T) {
	input := CalculationInput{
		Organisation: Organisation{
			ID:           "org-1",
			HoursPerDay:  8,
			HoursPerWeek: 40,
			HoursPerYear: 2080,
		},
		Persons:  []Person{{ID: "p1", OrganisationID: "org-1", EmploymentPct: 80}},
		Projects: []Project{testProject("pr1"), testProject("pr2")},
		Allocations: []Allocation{
			personAllocationEntry("a1", "p1", "pr1", 60, "2026-01-01", "2026-01-31"),
			personAllocationEntry("a2", "p1", "pr2", 20, "2026-01-01", "2026-01-31"),
		},
		Request: ReportRequest{
			Scope:       ScopePerson,
			IDs:         []string{"p1"},
			FromDate:    "2026-01-05",
			ToDate:      "2026-01-05",
			Granularity: GranularityDay,
		},
	}

	result, err := CalculateAvailabilityLoad(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 bucket, got %d", len(result))
	}

	assertBucket(t, result[0], "2026-01-05", 6.4, 6.4, 0)
	if !approxEqual(100, result[0].UtilizationPct, 0.01) {
		t.Fatalf("expected utilization 100 +/- 0.01, got %v", result[0].UtilizationPct)
	}
}

func TestCalculateAvailabilityLoadUsesEmploymentTimelineByDate(t *testing.T) {
	input := CalculationInput{
		Organisation: Organisation{
			ID:           "org-1",
			HoursPerDay:  8,
			HoursPerWeek: 40,
			HoursPerYear: 2080,
		},
		Persons: []Person{{
			ID:             "p1",
			OrganisationID: "org-1",
			EmploymentPct:  80,
			EmploymentChanges: []EmploymentChange{
				{EffectiveMonth: "2026-06", EmploymentPct: 50},
			},
		}},
		Request: ReportRequest{
			Scope:       ScopePerson,
			IDs:         []string{"p1"},
			FromDate:    "2026-05-31",
			ToDate:      "2026-06-01",
			Granularity: GranularityDay,
		},
	}

	result, err := CalculateAvailabilityLoad(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 buckets, got %d", len(result))
	}

	assertBucket(t, result[0], "2026-05-31", 6.4, 0, 6.4)
	assertBucket(t, result[1], "2026-06-01", 4, 0, 4)
}

func TestCalculateAvailabilityLoadValidation(t *testing.T) {
	_, err := CalculateAvailabilityLoad(CalculationInput{
		Organisation: Organisation{HoursPerDay: 8, HoursPerYear: 2080},
		Request:      ReportRequest{Scope: "bad", Granularity: GranularityDay, FromDate: "2026-01-01", ToDate: "2026-01-01"},
	})
	if err == nil {
		t.Fatal("expected validation error")
	}

	_, err = CalculateAvailabilityLoad(CalculationInput{
		Organisation: Organisation{HoursPerDay: 8, HoursPerYear: 2080},
		Request:      ReportRequest{Scope: ScopeOrganisation, Granularity: GranularityDay, FromDate: "2026-01-02", ToDate: "2026-01-01"},
	})
	if err == nil {
		t.Fatal("expected date range validation error")
	}

	_, err = CalculateAvailabilityLoad(CalculationInput{
		Organisation: Organisation{HoursPerDay: 8, HoursPerYear: 2080},
		Persons:      []Person{{ID: "p1"}},
		Request:      ReportRequest{Scope: ScopePerson, IDs: []string{"missing"}, Granularity: GranularityDay, FromDate: "2026-01-01", ToDate: "2026-01-01"},
	})
	if err == nil {
		t.Fatal("expected not found error")
	}

	_, err = CalculateAvailabilityLoad(CalculationInput{
		Organisation: Organisation{HoursPerDay: 8, HoursPerYear: 2080},
		Projects:     []Project{testProject("pr1")},
		Request:      ReportRequest{Scope: ScopeProject, IDs: []string{"missing"}, Granularity: GranularityDay, FromDate: "2026-01-01", ToDate: "2026-01-01"},
	})
	if err == nil {
		t.Fatal("expected project not found error")
	}
}

func TestAllocationHelperValidationBranches(t *testing.T) {
	if err := ValidateAllocationTargetType(AllocationTargetPerson); err != nil {
		t.Fatalf("expected person target type to be valid, got %v", err)
	}
	if err := ValidateAllocationTargetType(AllocationTargetGroup); err != nil {
		t.Fatalf("expected group target type to be valid, got %v", err)
	}
	if err := ValidateAllocationTargetType("invalid"); err == nil {
		t.Fatal("expected invalid target type to fail")
	}

	targetType, targetID := normalizedAllocationTarget(Allocation{PersonID: "legacy_person"})
	if targetType != AllocationTargetPerson || targetID != "legacy_person" {
		t.Fatalf("expected legacy target normalization, got %s/%s", targetType, targetID)
	}

	if _, _, err := parseAllocationDateRange("bad-date", "2026-01-01"); err == nil {
		t.Fatal("expected invalid allocation start date")
	}
	if _, _, err := parseAllocationDateRange("2026-01-02", "2026-01-01"); err == nil {
		t.Fatal("expected reversed allocation range")
	}

	personsByID := map[string]Person{
		"p1": {ID: "p1"},
	}
	groupsByID := map[string]Group{
		"g1": {ID: "g1", MemberIDs: []string{"p1"}},
	}

	personResolved, ok, err := resolveAllocation(
		personAllocationEntry("a1", "p1", "pr1", 10, "2026-01-01", "2026-01-31"),
		personsByID,
		groupsByID,
	)
	if err != nil || !ok || len(personResolved.personIDs) != 1 {
		t.Fatalf("expected person allocation resolution success, ok=%v err=%v result=%+v", ok, err, personResolved)
	}

	groupResolved, ok, err := resolveAllocation(
		groupAllocation("a2", "g1", "pr1", 10, "2026-01-01", "2026-01-31"),
		personsByID,
		groupsByID,
	)
	if err != nil || !ok || len(groupResolved.personIDs) != 1 {
		t.Fatalf("expected group allocation resolution success, ok=%v err=%v result=%+v", ok, err, groupResolved)
	}

	_, ok, err = resolveAllocation(
		groupAllocation("a3", "missing_group", "pr1", 10, "2026-01-01", "2026-01-31"),
		personsByID,
		groupsByID,
	)
	if err != nil || ok {
		t.Fatalf("expected missing group allocation to be ignored, ok=%v err=%v", ok, err)
	}
}

func TestAllocationHelperEdgeBranches(t *testing.T) {
	startDate, endDate, err := parseAllocationDateRange("", "")
	if err != nil {
		t.Fatalf("expected open date range to parse, got %v", err)
	}
	if got := startDate.Format(DateLayout); got != "1970-01-01" {
		t.Fatalf("unexpected open range start date: %s", got)
	}
	if got := endDate.Format(DateLayout); got != "9999-12-31" {
		t.Fatalf("unexpected open range end date: %s", got)
	}

	windowStart := time.Date(2026, time.January, 10, 0, 0, 0, 0, time.UTC)
	windowEnd := time.Date(2026, time.January, 20, 0, 0, 0, 0, time.UTC)
	window := personAllocation{StartDate: windowStart, EndDate: windowEnd}
	if allocationAppliesToDate(window, windowStart.AddDate(0, 0, -1)) {
		t.Fatal("expected date before allocation window to be excluded")
	}
	if !allocationAppliesToDate(window, windowStart) {
		t.Fatal("expected window start date to be included")
	}
	if !allocationAppliesToDate(window, windowEnd) {
		t.Fatal("expected window end date to be included")
	}
	if allocationAppliesToDate(window, windowEnd.AddDate(0, 0, 1)) {
		t.Fatal("expected date after allocation window to be excluded")
	}

	personsByID := map[string]Person{
		"p1": {ID: "p1"},
	}
	groupsByID := map[string]Group{
		"g-empty": {ID: "g-empty", MemberIDs: []string{"missing-person"}},
	}

	_, ok, err := resolveAllocation(
		personAllocationEntry("a1", "missing-person", "pr1", 10, "2026-01-01", "2026-01-31"),
		personsByID,
		groupsByID,
	)
	if err != nil || ok {
		t.Fatalf("expected missing person allocation to be ignored, ok=%v err=%v", ok, err)
	}

	_, ok, err = resolveAllocation(
		groupAllocation("a2", "g-empty", "pr1", 10, "2026-01-01", "2026-01-31"),
		personsByID,
		groupsByID,
	)
	if err != nil || ok {
		t.Fatalf("expected group without known members to be ignored, ok=%v err=%v", ok, err)
	}

	_, ok, err = resolveAllocation(
		Allocation{
			TargetType: "unknown",
			TargetID:   "x",
			StartDate:  "2026-01-01",
			EndDate:    "2026-01-31",
		},
		personsByID,
		groupsByID,
	)
	if err != nil || ok {
		t.Fatalf("expected unknown target type allocation to be ignored, ok=%v err=%v", ok, err)
	}

	_, ok, err = resolveAllocation(
		Allocation{
			TargetType: AllocationTargetPerson,
			TargetID:   "p1",
			StartDate:  "bad-date",
			EndDate:    "2026-01-31",
		},
		personsByID,
		groupsByID,
	)
	if !errors.Is(err, ErrValidation) || ok {
		t.Fatalf("expected invalid date range to fail validation, ok=%v err=%v", ok, err)
	}
}

func testProject(id string) Project {
	return Project{
		ID:                   id,
		OrganisationID:       "org-1",
		Name:                 id,
		StartDate:            "2026-01-01",
		EndDate:              "2026-12-31",
		EstimatedEffortHours: 1000,
	}
}

func personAllocationEntry(id, personID, projectID string, percent float64, startDate, endDate string) Allocation {
	return Allocation{
		ID:             id,
		OrganisationID: "org-1",
		TargetType:     AllocationTargetPerson,
		TargetID:       personID,
		ProjectID:      projectID,
		StartDate:      startDate,
		EndDate:        endDate,
		Percent:        percent,
		PersonID:       personID,
	}
}

func groupAllocation(id, groupID, projectID string, percent float64, startDate, endDate string) Allocation {
	return Allocation{
		ID:             id,
		OrganisationID: "org-1",
		TargetType:     AllocationTargetGroup,
		TargetID:       groupID,
		ProjectID:      projectID,
		StartDate:      startDate,
		EndDate:        endDate,
		Percent:        percent,
	}
}

func assertBucket(t *testing.T, bucket ReportBucket, period string, availability float64, load float64, free float64) {
	t.Helper()
	if bucket.PeriodStart != period {
		t.Fatalf("expected period %s got %s", period, bucket.PeriodStart)
	}
	if !approxEqual(availability, bucket.AvailabilityHours, 0.01) {
		t.Fatalf("expected availability %v +/- 0.01 got %v", availability, bucket.AvailabilityHours)
	}
	if !approxEqual(load, bucket.LoadHours, 0.01) {
		t.Fatalf("expected load %v +/- 0.01 got %v", load, bucket.LoadHours)
	}
	if !approxEqual(free, bucket.FreeHours, 0.01) {
		t.Fatalf("expected free %v +/- 0.01 got %v", free, bucket.FreeHours)
	}
}

func approxEqual(expected float64, actual float64, epsilon float64) bool {
	return math.Abs(expected-actual) <= epsilon
}
