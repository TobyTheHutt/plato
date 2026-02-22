package service

import (
	"math"
	"sort"
	"strings"
	"time"

	"plato/backend/internal/domain"
)

func validateOrganisation(organisation domain.Organisation) error {
	if err := domain.ValidateName(organisation.Name); err != nil {
		return domain.ErrValidation
	}
	if organisation.HoursPerDay <= 0 || organisation.HoursPerWeek <= 0 || organisation.HoursPerYear <= 0 {
		return domain.ErrValidation
	}
	return nil
}

func validatePerson(person domain.Person) error {
	if err := domain.ValidateName(person.Name); err != nil {
		return domain.ErrValidation
	}
	if err := domain.ValidatePercent(person.EmploymentPct); err != nil {
		return domain.ErrValidation
	}
	if strings.TrimSpace(person.EmploymentEffectiveFromMonth) != "" {
		if _, err := domain.ValidateMonth(strings.TrimSpace(person.EmploymentEffectiveFromMonth)); err != nil {
			return domain.ErrValidation
		}
	}
	for _, change := range person.EmploymentChanges {
		if _, err := domain.ValidateMonth(change.EffectiveMonth); err != nil {
			return domain.ErrValidation
		}
		if err := domain.ValidatePercent(change.EmploymentPct); err != nil {
			return domain.ErrValidation
		}
	}
	return nil
}

func upsertEmploymentChange(changes []domain.EmploymentChange, month string, employmentPct float64) []domain.EmploymentChange {
	normalized := make([]domain.EmploymentChange, 0, len(changes))
	updated := false
	for _, change := range changes {
		if change.EffectiveMonth == month {
			normalized = append(normalized, domain.EmploymentChange{
				EffectiveMonth: month,
				EmploymentPct:  employmentPct,
			})
			updated = true
			continue
		}
		normalized = append(normalized, change)
	}
	if !updated {
		normalized = append(normalized, domain.EmploymentChange{
			EffectiveMonth: month,
			EmploymentPct:  employmentPct,
		})
	}

	sort.Slice(normalized, func(i, j int) bool {
		if normalized[i].EffectiveMonth == normalized[j].EffectiveMonth {
			return i < j
		}
		return normalized[i].EffectiveMonth < normalized[j].EffectiveMonth
	})

	return normalized
}

func validateProject(project domain.Project) error {
	if err := domain.ValidateName(project.Name); err != nil {
		return domain.ErrValidation
	}
	if project.EstimatedEffortHours <= 0 {
		return domain.ErrValidation
	}
	if strings.TrimSpace(project.StartDate) == "" || strings.TrimSpace(project.EndDate) == "" {
		return domain.ErrValidation
	}
	if _, _, err := parseDateRange(project.StartDate, project.EndDate); err != nil {
		return domain.ErrValidation
	}
	return nil
}

func validateGroup(group domain.Group) error {
	if err := domain.ValidateName(group.Name); err != nil {
		return domain.ErrValidation
	}
	return nil
}

func validateAllocation(allocation domain.Allocation) error {
	if err := domain.ValidateAllocationTargetType(allocation.TargetType); err != nil {
		return domain.ErrValidation
	}
	if strings.TrimSpace(allocation.TargetID) == "" {
		return domain.ErrValidation
	}
	if strings.TrimSpace(allocation.ProjectID) == "" {
		return domain.ErrValidation
	}
	if strings.TrimSpace(allocation.StartDate) == "" || strings.TrimSpace(allocation.EndDate) == "" {
		return domain.ErrValidation
	}
	if _, _, err := parseDateRange(allocation.StartDate, allocation.EndDate); err != nil {
		return domain.ErrValidation
	}
	if math.IsNaN(allocation.Percent) || math.IsInf(allocation.Percent, 0) || allocation.Percent < 0 {
		return domain.ErrValidation
	}
	return nil
}

func validateDateHours(date string, hours float64, maxHours float64) error {
	if math.IsNaN(hours) || math.IsInf(hours, 0) {
		return domain.ErrValidation
	}
	if _, err := domain.ValidateDate(date); err != nil {
		return domain.ErrValidation
	}
	if hours < 0 || hours > maxHours {
		return domain.ErrValidation
	}
	return nil
}

func parseDateRange(startDate, endDate string) (time.Time, time.Time, error) {
	startDate = strings.TrimSpace(startDate)
	endDate = strings.TrimSpace(endDate)

	if startDate == "" {
		startDate = "1970-01-01"
	}
	if endDate == "" {
		endDate = "9999-12-31"
	}

	start, err := domain.ValidateDate(startDate)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	end, err := domain.ValidateDate(endDate)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}

	startParsed, err := time.Parse(domain.DateLayout, start)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	endParsed, err := time.Parse(domain.DateLayout, end)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	if endParsed.Before(startParsed) {
		return time.Time{}, time.Time{}, domain.ErrValidation
	}

	return startParsed, endParsed, nil
}
