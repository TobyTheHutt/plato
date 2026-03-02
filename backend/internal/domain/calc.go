package domain

import (
	"math"
	"sort"
	"strconv"
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

type calculationLookups struct {
	personsByID            map[string]Person
	groupsByID             map[string]Group
	personGroupIDs         map[string][]string
	allocationsByPerson    map[string][]personAllocation
	orgHolidayHoursByDate  map[string]float64
	groupUnavailableHours  map[string]float64
	personUnavailableHours map[string]float64
	allPersonIDs           []string
	allGroupIDs            []string
	allProjectIDs          []string
}

type personDayTotals struct {
	availabilityHours float64
	loadHours         float64
	projectLoadHours  float64
	freeHours         float64
}

func CalculateAvailabilityLoad(input CalculationInput) ([]ReportBucket, error) {
	if err := ValidateScope(input.Request.Scope); err != nil {
		return nil, err
	}
	if err := ValidateGranularity(input.Request.Granularity); err != nil {
		return nil, err
	}

	fromDate, toDate, err := parseReportDateRange(input.Request.FromDate, input.Request.ToDate)
	if err != nil {
		return nil, err
	}

	lookups, err := buildCalculationLookups(input)
	if err != nil {
		return nil, err
	}

	selectedPersonIDs, targetProjectIDs, err := selectedPeopleForScope(
		input.Request,
		lookups.allPersonIDs,
		lookups.allGroupIDs,
		lookups.allProjectIDs,
		lookups.personsByID,
		lookups.groupsByID,
		input.Allocations,
	)
	if err != nil {
		return nil, err
	}
	projectEstimationHours := projectEstimationForScope(input.Request.Scope, input.Projects, targetProjectIDs)

	buckets, err := calculateBuckets(
		fromDate,
		toDate,
		input.Request,
		input.Organisation.HoursPerDay,
		projectEstimationHours,
		selectedPersonIDs,
		targetProjectIDs,
		lookups,
	)
	if err != nil {
		return nil, err
	}

	return summarizeBuckets(buckets, input.Request.Scope), nil
}

func parseReportDateRange(fromDate, toDate string) (time.Time, time.Time, error) {
	start, err := time.Parse(DateLayout, fromDate)
	if err != nil {
		return time.Time{}, time.Time{}, ErrValidation
	}

	end, err := time.Parse(DateLayout, toDate)
	if err != nil {
		return time.Time{}, time.Time{}, ErrValidation
	}
	if end.Before(start) {
		return time.Time{}, time.Time{}, ErrValidation
	}

	return start, end, nil
}

func buildCalculationLookups(input CalculationInput) (calculationLookups, error) {
	personsByID, allPersonIDs := indexPersons(input.Persons)
	groupsByID, allGroupIDs, personGroupIDs := indexGroups(input.Groups)
	allProjectIDs := collectProjectIDs(input.Projects)

	allocationsByPerson, err := aggregateAllocations(input.Allocations, personsByID, groupsByID)
	if err != nil {
		return calculationLookups{}, err
	}

	return calculationLookups{
		personsByID:            personsByID,
		groupsByID:             groupsByID,
		personGroupIDs:         personGroupIDs,
		allocationsByPerson:    allocationsByPerson,
		orgHolidayHoursByDate:  aggregateOrgHolidayHours(input.OrgHolidays),
		groupUnavailableHours:  aggregateGroupUnavailableHours(input.GroupUnavailability),
		personUnavailableHours: aggregatePersonUnavailableHours(input.PersonUnavailability),
		allPersonIDs:           allPersonIDs,
		allGroupIDs:            allGroupIDs,
		allProjectIDs:          allProjectIDs,
	}, nil
}

func indexPersons(persons []Person) (map[string]Person, []string) {
	personsByID := make(map[string]Person, len(persons))
	allPersonIDs := make([]string, 0, len(persons))
	for _, person := range persons {
		personsByID[person.ID] = person
		allPersonIDs = append(allPersonIDs, person.ID)
	}

	return personsByID, allPersonIDs
}

func indexGroups(groups []Group) (map[string]Group, []string, map[string][]string) {
	groupsByID := make(map[string]Group, len(groups))
	allGroupIDs := make([]string, 0, len(groups))
	personGroupIDs := make(map[string][]string)
	for _, group := range groups {
		groupsByID[group.ID] = group
		allGroupIDs = append(allGroupIDs, group.ID)
		for _, memberID := range group.MemberIDs {
			personGroupIDs[memberID] = append(personGroupIDs[memberID], group.ID)
		}
	}

	return groupsByID, allGroupIDs, personGroupIDs
}

func collectProjectIDs(projects []Project) []string {
	allProjectIDs := make([]string, 0, len(projects))
	for _, project := range projects {
		allProjectIDs = append(allProjectIDs, project.ID)
	}

	return allProjectIDs
}

func aggregateAllocations(
	allocations []Allocation,
	personsByID map[string]Person,
	groupsByID map[string]Group,
) (map[string][]personAllocation, error) {
	allocationsByPerson := make(map[string][]personAllocation)
	for _, allocation := range allocations {
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

	return allocationsByPerson, nil
}

func aggregateOrgHolidayHours(holidays []OrgHoliday) map[string]float64 {
	orgHolidayHoursByDate := make(map[string]float64)
	for _, holiday := range holidays {
		orgHolidayHoursByDate[holiday.Date] += holiday.Hours
	}

	return orgHolidayHoursByDate
}

func aggregateGroupUnavailableHours(entries []GroupUnavailability) map[string]float64 {
	groupUnavailableHours := make(map[string]float64)
	for _, entry := range entries {
		groupUnavailableHours[compoundDateKey(entry.GroupID, entry.Date)] += entry.Hours
	}

	return groupUnavailableHours
}

func aggregatePersonUnavailableHours(entries []PersonUnavailability) map[string]float64 {
	personUnavailableHours := make(map[string]float64)
	for _, entry := range entries {
		personUnavailableHours[compoundDateKey(entry.PersonID, entry.Date)] += entry.Hours
	}

	return personUnavailableHours
}

func calculateBuckets(
	fromDate time.Time,
	toDate time.Time,
	request ReportRequest,
	hoursPerDay float64,
	projectEstimationHours float64,
	selectedPersonIDs []string,
	targetProjectIDs map[string]bool,
	lookups calculationLookups,
) (map[string]ReportBucket, error) {
	buckets := map[string]ReportBucket{}
	err := iterateDateRange(fromDate, toDate, func(current time.Time) error {
		periodKey := periodStart(current, request.Granularity).Format(DateLayout)
		bucket := buckets[periodKey]
		bucket.PeriodStart = periodKey
		bucket.ProjectEstimation = projectEstimationHours

		dayKey := current.Format(DateLayout)
		for _, personID := range selectedPersonIDs {
			person, ok := lookups.personsByID[personID]
			if !ok {
				continue
			}

			totals, calcErr := calculatePersonAvailability(
				personID,
				person,
				current,
				dayKey,
				request.Scope,
				hoursPerDay,
				lookups,
				targetProjectIDs,
			)
			if calcErr != nil {
				return calcErr
			}

			bucket.AvailabilityHours += totals.availabilityHours
			bucket.LoadHours += totals.loadHours
			bucket.ProjectLoadHours += totals.projectLoadHours
			bucket.FreeHours += totals.freeHours
		}

		buckets[periodKey] = bucket
		return nil
	})
	if err != nil {
		return nil, err
	}

	return buckets, nil
}

func iterateDateRange(fromDate, toDate time.Time, visit func(time.Time) error) error {
	for current := fromDate; !current.After(toDate); current = current.AddDate(0, 0, 1) {
		if err := visit(current); err != nil {
			return err
		}
	}

	return nil
}

func calculatePersonAvailability(
	personID string,
	person Person,
	currentDate time.Time,
	dayKey string,
	scope string,
	hoursPerDay float64,
	lookups calculationLookups,
	targetProjectIDs map[string]bool,
) (personDayTotals, error) {
	employmentPct, err := EmploymentPctOnDate(person, dayKey)
	if err != nil {
		return personDayTotals{}, ErrValidation
	}

	baseCapacity := hoursPerDay * employmentPct / 100
	if baseCapacity <= 0 {
		return personDayTotals{}, nil
	}

	unavailableHours := unavailableHoursForPersonOnDate(personID, dayKey, baseCapacity, lookups)
	effectiveAvailability := baseCapacity - unavailableHours
	allocationPct := allocationPercentForPersonOnDate(
		lookups.allocationsByPerson[personID],
		currentDate,
		scope,
		targetProjectIDs,
	)

	// Allocation percent is interpreted on full-time capacity.
	// Capacity limits are enforced during allocation writes.
	loadHours := hoursPerDay * allocationPct / 100
	totals := personDayTotals{
		availabilityHours: effectiveAvailability,
		loadHours:         loadHours,
		freeHours:         effectiveAvailability - loadHours,
	}
	if scope == ScopeProject {
		totals.projectLoadHours = loadHours
	}

	return totals, nil
}

func unavailableHoursForPersonOnDate(
	personID string,
	dayKey string,
	baseCapacity float64,
	lookups calculationLookups,
) float64 {
	unavailableHours := lookups.orgHolidayHoursByDate[dayKey]
	unavailableHours += lookups.personUnavailableHours[compoundDateKey(personID, dayKey)]
	for _, groupID := range lookups.personGroupIDs[personID] {
		unavailableHours += lookups.groupUnavailableHours[compoundDateKey(groupID, dayKey)]
	}

	if unavailableHours < 0 {
		return 0
	}
	if unavailableHours > baseCapacity {
		return baseCapacity
	}

	return unavailableHours
}

func allocationPercentForPersonOnDate(
	allocations []personAllocation,
	date time.Time,
	scope string,
	targetProjectIDs map[string]bool,
) float64 {
	isProjectScope := scope == ScopeProject
	total := 0.0
	for _, allocation := range allocations {
		if isProjectScope && !targetProjectIDs[allocation.ProjectID] {
			continue
		}
		if !allocationAppliesToDate(allocation, date) {
			continue
		}
		total += allocation.Percent
	}

	return total
}

func summarizeBuckets(buckets map[string]ReportBucket, scope string) []ReportBucket {
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
		if scope == ScopeProject {
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

	return result
}

func compoundDateKey(id, date string) string {
	return strconv.Itoa(len(id)) + ":" + id + ":" + strconv.Itoa(len(date)) + ":" + date
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
		return selectPeopleForOrganisationScope(allPersonIDs)
	case ScopePerson:
		return selectPeopleForPersonScope(request.IDs, allPersonIDs, personsByID)
	case ScopeGroup:
		return selectPeopleForGroupScope(request.IDs, allGroupIDs, groupsByID)
	case ScopeProject:
		return selectPeopleForProjectScope(request.IDs, allProjectIDs, personsByID, groupsByID, allocations)
	default:
		return nil, nil, ErrValidation
	}
}

func selectPeopleForOrganisationScope(allPersonIDs []string) ([]string, map[string]bool, error) {
	return append([]string{}, allPersonIDs...), map[string]bool{}, nil
}

func selectPeopleForPersonScope(
	ids []string,
	allPersonIDs []string,
	personsByID map[string]Person,
) ([]string, map[string]bool, error) {
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
}

func selectPeopleForGroupScope(
	ids []string,
	allGroupIDs []string,
	groupsByID map[string]Group,
) ([]string, map[string]bool, error) {
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
}

func selectPeopleForProjectScope(
	ids []string,
	allProjectIDs []string,
	personsByID map[string]Person,
	groupsByID map[string]Group,
	allocations []Allocation,
) ([]string, map[string]bool, error) {
	if len(ids) == 0 {
		ids = allProjectIDs
	}

	targetProjectIDs, err := targetProjectSet(ids, allProjectIDs)
	if err != nil {
		return nil, nil, err
	}

	selected := make([]string, 0)
	for _, allocation := range allocations {
		if !targetProjectIDs[allocation.ProjectID] {
			continue
		}
		selected = append(selected, peopleForProjectAllocation(allocation, personsByID, groupsByID)...)
	}

	return uniqueStrings(selected), targetProjectIDs, nil
}

func targetProjectSet(ids []string, allProjectIDs []string) (map[string]bool, error) {
	allProjects := make(map[string]bool, len(allProjectIDs))
	for _, projectID := range allProjectIDs {
		allProjects[projectID] = true
	}

	targetProjectIDs := make(map[string]bool, len(ids))
	for _, id := range ids {
		if !allProjects[id] {
			return nil, ErrNotFound
		}
		targetProjectIDs[id] = true
	}

	return targetProjectIDs, nil
}

func peopleForProjectAllocation(
	allocation Allocation,
	personsByID map[string]Person,
	groupsByID map[string]Group,
) []string {
	targetType, targetID := normalizedAllocationTarget(allocation)
	switch targetType {
	case AllocationTargetPerson:
		if _, ok := personsByID[targetID]; !ok {
			return nil
		}
		return []string{targetID}
	case AllocationTargetGroup:
		group, ok := groupsByID[targetID]
		if !ok {
			return nil
		}
		return group.MemberIDs
	default:
		return nil
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
