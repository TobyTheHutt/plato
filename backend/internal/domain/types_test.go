package domain

import (
	"errors"
	"testing"
	"time"
)

func TestValidationHelpers(t *testing.T) {
	if _, err := ValidateDate("2026-01-01"); err != nil {
		t.Fatalf("expected valid date: %v", err)
	}
	if _, err := ValidateDate("bad"); err == nil {
		t.Fatal("expected invalid date")
	}
	if _, err := ValidateMonth("2026-03"); err != nil {
		t.Fatalf("expected valid month: %v", err)
	}
	if _, err := ValidateMonth("bad"); err == nil {
		t.Fatal("expected invalid month")
	}
	if err := ValidateName("Alice"); err != nil {
		t.Fatalf("expected valid name: %v", err)
	}
	if err := ValidateName("  "); !errors.Is(err, ErrValidation) {
		t.Fatalf("expected validation error for empty name, got %v", err)
	}
	if err := ValidatePercent(0); err != nil {
		t.Fatalf("expected valid percent 0: %v", err)
	}
	if err := ValidatePercent(100); err != nil {
		t.Fatalf("expected valid percent 100: %v", err)
	}
	if err := ValidatePercent(120); !errors.Is(err, ErrValidation) {
		t.Fatalf("expected validation error for percent, got %v", err)
	}
	if err := ValidateScope(ScopeGroup); err != nil {
		t.Fatalf("expected valid scope group: %v", err)
	}
	if err := ValidateScope("x"); !errors.Is(err, ErrValidation) {
		t.Fatalf("expected scope validation error, got %v", err)
	}
	if err := ValidateGranularity(GranularityWeek); err != nil {
		t.Fatalf("expected valid week granularity: %v", err)
	}
	if err := ValidateGranularity("x"); !errors.Is(err, ErrValidation) {
		t.Fatalf("expected granularity validation error, got %v", err)
	}
	if err := ValidateAllocationTargetType(AllocationTargetPerson); err != nil {
		t.Fatalf("expected valid person allocation target: %v", err)
	}
	if err := ValidateAllocationTargetType(AllocationTargetGroup); err != nil {
		t.Fatalf("expected valid group allocation target: %v", err)
	}
	if err := ValidateAllocationTargetType("x"); !errors.Is(err, ErrValidation) {
		t.Fatalf("expected target type validation error, got %v", err)
	}

	person := Person{
		EmploymentPct: 80,
		EmploymentChanges: []EmploymentChange{
			{EffectiveMonth: "2026-04", EmploymentPct: 60},
			{EffectiveMonth: "2026-07", EmploymentPct: 100},
		},
	}
	pct, err := EmploymentPctOnDate(person, "2026-03-15")
	if err != nil {
		t.Fatalf("expected baseline employment percent for March, got %v", err)
	}
	if pct != 80 {
		t.Fatalf("expected March employment percent 80, got %v", pct)
	}
	pct, err = EmploymentPctOnDate(person, "2026-04-01")
	if err != nil {
		t.Fatalf("expected April employment percent, got %v", err)
	}
	if pct != 60 {
		t.Fatalf("expected April employment percent 60, got %v", pct)
	}
	pct, err = EmploymentPctOnDate(person, "2026-08-10")
	if err != nil {
		t.Fatalf("expected August employment percent, got %v", err)
	}
	if pct != 100 {
		t.Fatalf("expected August employment percent 100, got %v", pct)
	}
	_, err = EmploymentPctOnDate(Person{EmploymentPct: 80, EmploymentChanges: []EmploymentChange{{EffectiveMonth: "bad", EmploymentPct: 70}}}, "2026-01-01")
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("expected invalid employment month to fail, got %v", err)
	}
	_, err = EmploymentPctOnDate(person, "bad-date")
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("expected invalid employment date to fail, got %v", err)
	}
	_, err = EmploymentPctOnDate(Person{EmploymentPct: 101}, "2026-01-01")
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("expected invalid base employment percent to fail, got %v", err)
	}
	_, err = EmploymentPctOnDate(
		Person{
			EmploymentPct: 80,
			EmploymentChanges: []EmploymentChange{
				{EffectiveMonth: "2026-01", EmploymentPct: 120},
			},
		},
		"2026-02-01",
	)
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("expected invalid changed employment percent to fail, got %v", err)
	}

	duplicateMonthPerson := Person{
		EmploymentPct: 80,
		EmploymentChanges: []EmploymentChange{
			{EffectiveMonth: "2026-05", EmploymentPct: 60},
			{EffectiveMonth: "2026-05", EmploymentPct: 70},
		},
	}
	_, err = EmploymentPctOnDate(duplicateMonthPerson, "2026-05-10")
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("expected duplicate effective month to fail validation, got %v", err)
	}
}

func TestPeriodStartAndRoundHelpers(t *testing.T) {
	day := time.Date(2026, time.February, 18, 0, 0, 0, 0, time.UTC)
	if got := periodStart(day, GranularityDay).Format(DateLayout); got != "2026-02-18" {
		t.Fatalf("unexpected day period: %s", got)
	}
	if got := periodStart(day, GranularityWeek).Format(DateLayout); got != "2026-02-16" {
		t.Fatalf("unexpected week period: %s", got)
	}
	if got := periodStart(day, GranularityMonth).Format(DateLayout); got != "2026-02-01" {
		t.Fatalf("unexpected month period: %s", got)
	}
	if got := periodStart(day, GranularityYear).Format(DateLayout); got != "2026-01-01" {
		t.Fatalf("unexpected year period: %s", got)
	}
	if got := periodStart(day, "invalid").Format(DateLayout); got != "2026-02-18" {
		t.Fatalf("unexpected default period for invalid granularity: %s", got)
	}

	if got := round2(-3.995); got != -4 {
		t.Fatalf("unexpected round2 negative value: %v", got)
	}
}

func TestSelectedPeopleForScope(t *testing.T) {
	persons := map[string]Person{"p1": {ID: "p1"}, "p2": {ID: "p2"}}
	groups := map[string]Group{"g1": {ID: "g1", MemberIDs: []string{"p1", "p2"}}}
	allocations := []Allocation{
		{PersonID: "p1", TargetType: AllocationTargetPerson, TargetID: "p1", ProjectID: "pr1"},
		{TargetType: AllocationTargetGroup, TargetID: "g1", ProjectID: "pr2"},
	}

	selected, _, err := selectedPeopleForScope(ReportRequest{Scope: ScopePerson, IDs: []string{"p2"}}, []string{"p1", "p2"}, []string{"g1"}, []string{"pr1"}, persons, groups, allocations)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(selected) != 1 || selected[0] != "p2" {
		t.Fatalf("unexpected selected people: %v", selected)
	}

	selected, _, err = selectedPeopleForScope(ReportRequest{Scope: ScopeGroup, IDs: []string{"g1"}}, []string{"p1", "p2"}, []string{"g1"}, []string{"pr1"}, persons, groups, allocations)
	if err != nil {
		t.Fatalf("unexpected group scope error: %v", err)
	}
	if len(selected) != 2 {
		t.Fatalf("expected 2 selected from group, got %v", selected)
	}

	selected, _, err = selectedPeopleForScope(ReportRequest{Scope: ScopeProject, IDs: []string{"pr1"}}, []string{"p1", "p2"}, []string{"g1"}, []string{"pr1", "pr2"}, persons, groups, allocations)
	if err != nil {
		t.Fatalf("unexpected project scope error: %v", err)
	}
	if len(selected) != 1 || selected[0] != "p1" {
		t.Fatalf("unexpected project selection: %v", selected)
	}

	selected, _, err = selectedPeopleForScope(ReportRequest{Scope: ScopeProject, IDs: []string{"pr2"}}, []string{"p1", "p2"}, []string{"g1"}, []string{"pr1", "pr2"}, persons, groups, allocations)
	if err != nil {
		t.Fatalf("unexpected group-backed project scope error: %v", err)
	}
	if len(selected) != 2 {
		t.Fatalf("expected 2 selected from group-backed project allocation, got %v", selected)
	}

	_, _, err = selectedPeopleForScope(ReportRequest{Scope: ScopeGroup, IDs: []string{"missing"}}, []string{"p1", "p2"}, []string{"g1"}, []string{"pr1"}, persons, groups, allocations)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected not found for missing group, got %v", err)
	}

	_, _, err = selectedPeopleForScope(ReportRequest{Scope: ScopeProject, IDs: []string{"missing"}}, []string{"p1", "p2"}, []string{"g1"}, []string{"pr1"}, persons, groups, allocations)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected not found for missing project, got %v", err)
	}
}
