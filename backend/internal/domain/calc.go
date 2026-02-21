package domain

import (
	"math"
	"sort"
	"strings"
	"time"
)

type CalculationInput struct {
	Organisation         Organisation
	Persons              []Person
	Groups               []Group
	Projects             []Project
	Allocations          []Allocation
	OrgHolidays          []OrgHoliday
	GroupUnavailability  []GroupUnavailability
	PersonUnavailability []PersonUnavailability
	Request              ReportRequest
}

type personAllocation struct {
	ProjectID string
	Percent   float64
	StartDate time.Time
	EndDate   time.Time
}

type allocationResolution struct {
	personIDs []string
	startDate time.Time
	endDate   time.Time
}

func CalculateAvailabilityLoad(input CalculationInput) ([]ReportBucket, error) {
	if err := ValidateScope(input.Request.Scope); err != nil {
		return nil, err
	}
	if err := ValidateGranularity(input.Request.Granularity); err != nil {
		return nil, err
	}

	fromDate, err := time.Parse(DateLayout, input.Request.FromDate)
	if err != nil {
		return nil, ErrValidation
	}
	toDate, err := time.Parse(DateLayout, input.Request.ToDate)
	if err != nil {
		return nil, ErrValidation
	}
	if toDate.Before(fromDate) {
		return nil, ErrValidation
	}

	personsByID := make(map[string]Person, len(input.Persons))
	allPersonIDs := make([]string, 0, len(input.Persons))
	for _, person := range input.Persons {
		personsByID[person.ID] = person
		allPersonIDs = append(allPersonIDs, person.ID)
	}

	groupsByID := make(map[string]Group, len(input.Groups))
	allGroupIDs := make([]string, 0, len(input.Groups))
	personGroupIDs := make(map[string][]string)
	for _, group := range input.Groups {
		groupsByID[group.ID] = group
		allGroupIDs = append(allGroupIDs, group.ID)
		for _, memberID := range group.MemberIDs {
			personGroupIDs[memberID] = append(personGroupIDs[memberID], group.ID)
		}
	}

	allProjectIDs := make([]string, 0, len(input.Projects))
	for _, project := range input.Projects {
		allProjectIDs = append(allProjectIDs, project.ID)
	}

	allocationsByPerson := make(map[string][]personAllocation)
	for _, allocation := range input.Allocations {
		resolved, ok, err := resolveAllocation(allocation, personsByID, groupsByID)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}

		for _, personID := range resolved.personIDs {
			allocationsByPerson[personID] = append(allocationsByPerson[personID], personAllocation{
				ProjectID: allocation.ProjectID,
				Percent:   allocation.Percent,
				StartDate: resolved.startDate,
				EndDate:   resolved.endDate,
			})
		}
	}

	orgHolidayHoursByDate := make(map[string]float64)
	for _, holiday := range input.OrgHolidays {
		orgHolidayHoursByDate[holiday.Date] += holiday.Hours
	}

	groupUnavailableHours := make(map[string]float64)
	for _, entry := range input.GroupUnavailability {
		groupUnavailableHours[entry.GroupID+"|"+entry.Date] += entry.Hours
	}

	personUnavailableHours := make(map[string]float64)
	for _, entry := range input.PersonUnavailability {
		personUnavailableHours[entry.PersonID+"|"+entry.Date] += entry.Hours
	}

	selectedPersonIDs, targetProjectIDs, err := selectedPeopleForScope(
		input.Request,
		allPersonIDs,
		allGroupIDs,
		allProjectIDs,
		personsByID,
		groupsByID,
		input.Allocations,
	)
	if err != nil {
		return nil, err
	}
	projectEstimationHours := projectEstimationForScope(input.Request.Scope, input.Projects, targetProjectIDs)

	buckets := map[string]ReportBucket{}

	for current := fromDate; !current.After(toDate); current = current.AddDate(0, 0, 1) {
		period := periodStart(current, input.Request.Granularity)
		periodKey := period.Format(DateLayout)
		bucket := buckets[periodKey]
		bucket.PeriodStart = periodKey
		bucket.ProjectEstimation = projectEstimationHours

		dayKey := current.Format(DateLayout)
		for _, personID := range selectedPersonIDs {
			person, ok := personsByID[personID]
			if !ok {
				continue
			}

			employmentPct, err := EmploymentPctOnDate(person, dayKey)
			if err != nil {
				return nil, ErrValidation
			}
			baseCapacity := input.Organisation.HoursPerDay * employmentPct / 100
			if baseCapacity <= 0 {
				continue
			}

			unavailableHours := orgHolidayHoursByDate[dayKey] + personUnavailableHours[personID+"|"+dayKey]
			for _, groupID := range personGroupIDs[personID] {
				unavailableHours += groupUnavailableHours[groupID+"|"+dayKey]
			}
			if unavailableHours < 0 {
				unavailableHours = 0
			}
			if unavailableHours > baseCapacity {
				unavailableHours = baseCapacity
			}

			effectiveAvailability := baseCapacity - unavailableHours
			allocationPct := 0.0
			for _, allocation := range allocationsByPerson[personID] {
				if input.Request.Scope == ScopeProject && !targetProjectIDs[allocation.ProjectID] {
					continue
				}
				if !allocationAppliesToDate(allocation, current) {
					continue
				}
				allocationPct += allocation.Percent
			}

			// Allocation percent is interpreted on full-time capacity.
			// Capacity limits are enforced during allocation writes.
			loadHours := input.Organisation.HoursPerDay * allocationPct / 100
			bucket.AvailabilityHours += effectiveAvailability
			bucket.LoadHours += loadHours
			if input.Request.Scope == ScopeProject {
				bucket.ProjectLoadHours += loadHours
			}
			bucket.FreeHours += effectiveAvailability - loadHours
		}

		buckets[periodKey] = bucket
	}

	sortedKeys := make([]string, 0, len(buckets))
	for key := range buckets {
		sortedKeys = append(sortedKeys, key)
	}
	sort.Strings(sortedKeys)

	result := make([]ReportBucket, 0, len(sortedKeys))
	cumulativeProjectLoad := 0.0
	for _, key := range sortedKeys {
		bucket := buckets[key]
		if bucket.AvailabilityHours > 0 {
			bucket.UtilizationPct = bucket.LoadHours / bucket.AvailabilityHours * 100
		}
		if input.Request.Scope == ScopeProject {
			cumulativeProjectLoad += bucket.ProjectLoadHours
			bucket.ProjectLoadHours = cumulativeProjectLoad
			if bucket.ProjectEstimation > 0 {
				bucket.CompletionPct = bucket.ProjectLoadHours / bucket.ProjectEstimation * 100
			}
		}
		bucket.AvailabilityHours = round2(bucket.AvailabilityHours)
		bucket.LoadHours = round2(bucket.LoadHours)
		bucket.ProjectLoadHours = round2(bucket.ProjectLoadHours)
		bucket.ProjectEstimation = round2(bucket.ProjectEstimation)
		bucket.FreeHours = round2(bucket.FreeHours)
		bucket.UtilizationPct = round2(bucket.UtilizationPct)
		bucket.CompletionPct = round2(bucket.CompletionPct)
		result = append(result, bucket)
	}

	return result, nil
}

func selectedPeopleForScope(
	request ReportRequest,
	allPersonIDs []string,
	allGroupIDs []string,
	allProjectIDs []string,
	personsByID map[string]Person,
	groupsByID map[string]Group,
	allocations []Allocation,
) ([]string, map[string]bool, error) {
	switch request.Scope {
	case ScopeOrganisation:
		return append([]string{}, allPersonIDs...), map[string]bool{}, nil
	case ScopePerson:
		ids := request.IDs
		if len(ids) == 0 {
			return append([]string{}, allPersonIDs...), map[string]bool{}, nil
		}
		selected := make([]string, 0, len(ids))
		for _, id := range ids {
			if _, ok := personsByID[id]; !ok {
				return nil, nil, ErrNotFound
			}
			selected = append(selected, id)
		}
		return uniqueStrings(selected), map[string]bool{}, nil
	case ScopeGroup:
		ids := request.IDs
		if len(ids) == 0 {
			ids = allGroupIDs
		}
		selected := make([]string, 0)
		for _, id := range ids {
			group, ok := groupsByID[id]
			if !ok {
				return nil, nil, ErrNotFound
			}
			selected = append(selected, group.MemberIDs...)
		}
		return uniqueStrings(selected), map[string]bool{}, nil
	case ScopeProject:
		ids := request.IDs
		if len(ids) == 0 {
			ids = allProjectIDs
		}
		targetProjectIDs := make(map[string]bool, len(ids))
		allProjects := make(map[string]bool, len(allProjectIDs))
		for _, projectID := range allProjectIDs {
			allProjects[projectID] = true
		}
		for _, id := range ids {
			if !allProjects[id] {
				return nil, nil, ErrNotFound
			}
			targetProjectIDs[id] = true
		}
		selected := make([]string, 0)
		for _, allocation := range allocations {
			if targetProjectIDs[allocation.ProjectID] {
				targetType, targetID := normalizedAllocationTarget(allocation)
				switch targetType {
				case AllocationTargetPerson:
					if _, ok := personsByID[targetID]; ok {
						selected = append(selected, targetID)
					}
				case AllocationTargetGroup:
					group, ok := groupsByID[targetID]
					if !ok {
						continue
					}
					selected = append(selected, group.MemberIDs...)
				}
			}
		}
		return uniqueStrings(selected), targetProjectIDs, nil
	default:
		return nil, nil, ErrValidation
	}
}

func uniqueStrings(input []string) []string {
	seen := make(map[string]bool, len(input))
	result := make([]string, 0, len(input))
	for _, value := range input {
		if seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func projectEstimationForScope(scope string, projects []Project, targetProjectIDs map[string]bool) float64 {
	if scope != ScopeProject {
		return 0
	}

	total := 0.0
	for _, project := range projects {
		if targetProjectIDs[project.ID] {
			total += project.EstimatedEffortHours
		}
	}

	return total
}

func periodStart(date time.Time, granularity string) time.Time {
	switch granularity {
	case GranularityDay:
		return date
	case GranularityWeek:
		weekday := int(date.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		return date.AddDate(0, 0, -(weekday - 1))
	case GranularityMonth:
		return time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, time.UTC)
	case GranularityYear:
		return time.Date(date.Year(), time.January, 1, 0, 0, 0, 0, time.UTC)
	default:
		return date
	}
}

func round2(value float64) float64 {
	return math.Round(value*100) / 100
}

func allocationAppliesToDate(allocation personAllocation, date time.Time) bool {
	if date.Before(allocation.StartDate) {
		return false
	}
	if date.After(allocation.EndDate) {
		return false
	}
	return true
}

func resolveAllocation(
	allocation Allocation,
	personsByID map[string]Person,
	groupsByID map[string]Group,
) (allocationResolution, bool, error) {
	startDate, endDate, err := parseAllocationDateRange(allocation.StartDate, allocation.EndDate)
	if err != nil {
		return allocationResolution{}, false, ErrValidation
	}

	targetType, targetID := normalizedAllocationTarget(allocation)
	switch targetType {
	case AllocationTargetPerson:
		if _, ok := personsByID[targetID]; !ok {
			return allocationResolution{}, false, nil
		}

		return allocationResolution{
			personIDs: []string{targetID},
			startDate: startDate,
			endDate:   endDate,
		}, true, nil
	case AllocationTargetGroup:
		group, ok := groupsByID[targetID]
		if !ok {
			return allocationResolution{}, false, nil
		}

		personIDs := make([]string, 0, len(group.MemberIDs))
		for _, memberID := range group.MemberIDs {
			if _, exists := personsByID[memberID]; exists {
				personIDs = append(personIDs, memberID)
			}
		}

		if len(personIDs) == 0 {
			return allocationResolution{}, false, nil
		}

		return allocationResolution{
			personIDs: uniqueStrings(personIDs),
			startDate: startDate,
			endDate:   endDate,
		}, true, nil
	default:
		return allocationResolution{}, false, nil
	}
}

func normalizedAllocationTarget(allocation Allocation) (string, string) {
	targetType := strings.TrimSpace(allocation.TargetType)
	targetID := strings.TrimSpace(allocation.TargetID)
	if targetType == "" && strings.TrimSpace(allocation.PersonID) != "" {
		targetType = AllocationTargetPerson
		targetID = strings.TrimSpace(allocation.PersonID)
	}
	return targetType, targetID
}

func parseAllocationDateRange(startDate, endDate string) (time.Time, time.Time, error) {
	startDate = strings.TrimSpace(startDate)
	endDate = strings.TrimSpace(endDate)

	var start time.Time
	var end time.Time
	var err error

	if startDate == "" {
		start = time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC)
	} else {
		start, err = time.Parse(DateLayout, startDate)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	}

	if endDate == "" {
		end = time.Date(9999, time.December, 31, 0, 0, 0, 0, time.UTC)
	} else {
		end, err = time.Parse(DateLayout, endDate)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	}

	if end.Before(start) {
		return time.Time{}, time.Time{}, ErrValidation
	}

	return start, end, nil
}
