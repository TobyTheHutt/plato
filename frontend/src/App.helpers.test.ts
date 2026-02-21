import {
  allocationPercentFromInput,
  asNumber,
  buildDayRangeDateHours,
  buildWeekRangeDateHours,
  buildWeekdayEntries,
  buildReportTableRows,
  dateAfterValue,
  dateRangesOverlap,
  formatDateValue,
  formatHours,
  hasPersonIntersection,
  isExpandableReportPeriodRow,
  isPersonOverallocated,
  isReportRowVisible,
  isSubsetOf,
  isValidMonthValue,
  newAllocationFormState,
  normalizeAllocationTargetID,
  normalizeAllocationTargetType,
  parseDateValue,
  parseWeekStartValue,
  personDailyHours,
  personEmploymentPctOnDate,
  reportBucketTotal,
  reportDetailToggleLabel,
  roundHours,
  toErrorMessage,
  toWorkingHours,
  type Allocation,
  type Person,
  type ReportObjectResult,
  workingHoursForAllocationUnit
} from "./App"

describe("App helpers", () => {
  it("builds the default allocation form state", () => {
    expect(newAllocationFormState()).toEqual({
      id: "",
      targetType: "person",
      targetID: "",
      projectID: "",
      startDate: "2026-01-01",
      endDate: "2026-12-31",
      loadInputType: "fte_pct",
      loadUnit: "day",
      loadValue: "0"
    })
  })

  it("parses numeric values safely", () => {
    expect(asNumber("12.5")).toBe(12.5)
    expect(asNumber("not-a-number")).toBe(0)
  })

  it("formats hours values and handles invalid input", () => {
    expect(formatHours(5)).toBe("5.00")
    expect(formatHours(undefined)).toBe("n/a")
    expect(formatHours(Number.NaN)).toBe("n/a")
    expect(formatHours(Number.POSITIVE_INFINITY)).toBe("n/a")
  })

  it("normalizes unknown errors to a message", () => {
    expect(toErrorMessage(new Error("failed"))).toBe("failed")
    expect(toErrorMessage("boom")).toBe("unexpected error")
  })

  it("converts working hours for each unit", () => {
    expect(toWorkingHours(8, "day")).toEqual({ day: 8, week: 40, year: 2080 })
    expect(toWorkingHours(40, "week")).toEqual({ day: 8, week: 40, year: 2080 })
    expect(toWorkingHours(173.33333333333334, "month").year).toBeCloseTo(2080, 6)
    expect(toWorkingHours(2080, "year")).toEqual({ day: 8, week: 40, year: 2080 })
  })

  it("converts allocation load input to percent", () => {
    expect(allocationPercentFromInput(33.3333, "fte_pct", "day", 8)).toBe(33.333)
    expect(allocationPercentFromInput(20, "hours", "week", 8)).toBe(50)
    expect(allocationPercentFromInput(10, "hours", "day", 0)).toBe(0)
    expect(workingHoursForAllocationUnit(8, "day")).toBe(8)
    expect(workingHoursForAllocationUnit(8, "month")).toBeCloseTo(173.333333, 3)
  })

  it("parses and formats date values", () => {
    const parsed = parseDateValue("2026-01-05")
    expect(parsed).not.toBeNull()
    expect(formatDateValue(parsed as Date)).toBe("2026-01-05")
    expect(parseDateValue("")).toBeNull()
    expect(parseDateValue("not-a-date")).toBeNull()
  })

  it("rounds decimal hours", () => {
    expect(roundHours(1.23456)).toBe(1.235)
    expect(personDailyHours(8, 62.5)).toBe(5)
  })

  it("validates month values", () => {
    expect(isValidMonthValue("2026-01")).toBe(true)
    expect(isValidMonthValue("2026-1")).toBe(false)
    expect(isValidMonthValue("invalid")).toBe(false)
  })

  it("resolves employment percentage from monthly history", () => {
    const person: Person = {
      id: "person_1",
      organisation_id: "org_1",
      name: "Alice",
      employment_pct: 100,
      employment_changes: [
        { effective_month: "bad", employment_pct: 10 },
        { effective_month: "2026-02", employment_pct: 80 },
        { effective_month: "2026-04", employment_pct: 60 }
      ]
    }

    expect(personEmploymentPctOnDate(person, "invalid-date")).toBe(100)
    expect(personEmploymentPctOnDate(person, "2026-01-15")).toBe(100)
    expect(personEmploymentPctOnDate(person, "2026-03-01")).toBe(80)
    expect(personEmploymentPctOnDate(person, "2026-05-01")).toBe(60)
  })

  it("builds weekday entries and validates inputs", () => {
    expect(
      buildWeekdayEntries(new Date("2026-01-02T00:00:00Z"), new Date("2026-01-06T00:00:00Z"), 8)
    ).toEqual([
      { date: "2026-01-02", hours: 8 },
      { date: "2026-01-05", hours: 8 },
      { date: "2026-01-06", hours: 8 }
    ])

    expect(() => {
      buildWeekdayEntries(new Date("2026-01-06T00:00:00Z"), new Date("2026-01-02T00:00:00Z"), 8)
    }).toThrow("start date must be before or equal to end date")
    expect(() => {
      buildWeekdayEntries(new Date("2026-01-02T00:00:00Z"), new Date("2026-01-02T00:00:00Z"), 0)
    }).toThrow("organisation working hours must be greater than 0")
    expect(() => {
      buildWeekdayEntries(new Date("2026-01-03T00:00:00Z"), new Date("2026-01-04T00:00:00Z"), 8)
    }).toThrow("timespan must include at least one weekday")
  })

  it("parses ISO week start values", () => {
    expect(formatDateValue(parseWeekStartValue("2026-W01") as Date)).toBe("2025-12-29")
    expect(parseWeekStartValue("2026-W00")).toBeNull()
    expect(parseWeekStartValue("2026-W54")).toBeNull()
    expect(parseWeekStartValue("invalid")).toBeNull()
  })

  it("builds date-hour ranges from dates and weeks", () => {
    expect(buildDayRangeDateHours("2026-01-05", "2026-01-06", 8)).toEqual([
      { date: "2026-01-05", hours: 8 },
      { date: "2026-01-06", hours: 8 }
    ])
    expect(() => {
      buildDayRangeDateHours("", "2026-01-06", 8)
    }).toThrow("valid start and end dates are required")

    expect(buildWeekRangeDateHours("2026-W01", "2026-W01", 8)).toEqual([
      { date: "2025-12-29", hours: 8 },
      { date: "2025-12-30", hours: 8 },
      { date: "2025-12-31", hours: 8 },
      { date: "2026-01-01", hours: 8 },
      { date: "2026-01-02", hours: 8 }
    ])
    expect(() => {
      buildWeekRangeDateHours("bad", "2026-W01", 8)
    }).toThrow("valid start and end weeks are required")
    expect(() => {
      buildWeekRangeDateHours("2026-W02", "2026-W01", 8)
    }).toThrow("start week must be before or equal to end week")
  })

  it("normalizes allocation target metadata", () => {
    const groupAllocation: Allocation = {
      id: "allocation_1",
      organisation_id: "org_1",
      target_type: "group",
      target_id: "group_1",
      project_id: "project_1",
      start_date: "2026-01-01",
      end_date: "2026-01-02",
      percent: 10
    }
    const personFallbackAllocation: Allocation = {
      ...groupAllocation,
      id: "allocation_2",
      target_type: "person",
      target_id: "",
      person_id: "person_1"
    }

    expect(normalizeAllocationTargetType(groupAllocation)).toBe("group")
    expect(normalizeAllocationTargetType(personFallbackAllocation)).toBe("person")
    expect(normalizeAllocationTargetID(groupAllocation)).toBe("group_1")
    expect(normalizeAllocationTargetID(personFallbackAllocation)).toBe("person_1")
    expect(normalizeAllocationTargetID({ ...personFallbackAllocation, person_id: undefined, target_id: "" })).toBe("")
  })

  it("checks date and membership overlap helpers", () => {
    expect(dateRangesOverlap("2026-01-01", "2026-01-10", "2026-01-10", "2026-01-20")).toBe(true)
    expect(dateRangesOverlap("2026-01-01", "2026-01-05", "2026-01-06", "2026-01-10")).toBe(false)
    expect(dateRangesOverlap("bad", "2026-01-05", "2026-01-06", "2026-01-10")).toBe(false)

    expect(hasPersonIntersection(["a", "b"], ["x", "b"])).toBe(true)
    expect(hasPersonIntersection(["a"], ["x"])).toBe(false)
    expect(hasPersonIntersection([], ["x"])).toBe(false)

    expect(isSubsetOf(["a", "b"], ["a", "b", "c"])).toBe(true)
    expect(isSubsetOf(["a", "z"], ["a", "b", "c"])).toBe(false)
  })

  it("computes the next day value", () => {
    expect(dateAfterValue("2026-01-01")).toBe("2026-01-02")
    expect(dateAfterValue("bad")).toBeNull()
  })

  it("detects over-allocation with and without employment changes", () => {
    const person: Person = {
      id: "person_1",
      organisation_id: "org_1",
      name: "Alice",
      employment_pct: 100
    }

    expect(isPersonOverallocated(person, [])).toBe(false)
    expect(
      isPersonOverallocated(person, [
        { startDate: "2026-01-01", endDate: "2026-01-31", percent: 60 },
        { startDate: "2026-01-10", endDate: "2026-01-20", percent: 40 }
      ])
    ).toBe(false)
    expect(
      isPersonOverallocated(person, [
        { startDate: "2026-01-01", endDate: "2026-01-31", percent: 80 },
        { startDate: "2026-01-10", endDate: "2026-01-20", percent: 40 }
      ])
    ).toBe(true)

    const personWithChanges: Person = {
      ...person,
      employment_changes: [{ effective_month: "2026-02", employment_pct: 50 }]
    }
    expect(
      isPersonOverallocated(personWithChanges, [
        { startDate: "2026-02-01", endDate: "2026-02-28", percent: 60 }
      ])
    ).toBe(true)

    expect(
      isPersonOverallocated(person, [
        { startDate: "bad", endDate: "2026-01-31", percent: 100 },
        { startDate: "2026-01-01", endDate: "2026-01-31", percent: 0 }
      ])
    ).toBe(false)
  })

  it("sums report buckets and derives percentages", () => {
    const total = reportBucketTotal([
      {
        period_start: "2026-01-01",
        availability_hours: 8,
        load_hours: 4,
        project_load_hours: 4,
        project_estimation_hours: 16,
        free_hours: 4,
        utilization_pct: 50,
        project_completion_pct: 25
      },
      {
        period_start: "2026-01-01",
        availability_hours: 4,
        load_hours: 3,
        project_load_hours: 3,
        project_estimation_hours: 8,
        free_hours: 1,
        utilization_pct: 75,
        project_completion_pct: 37.5
      }
    ])

    expect(total.availability_hours).toBe(12)
    expect(total.load_hours).toBe(7)
    expect(total.project_load_hours).toBe(7)
    expect(total.project_estimation_hours).toBe(24)
    expect(total.free_hours).toBe(5)
    expect(total.utilization_pct).toBeCloseTo(58.333, 3)
    expect(total.project_completion_pct).toBeCloseTo(29.167, 3)
  })

  it("keeps project totals empty when estimation is missing", () => {
    const total = reportBucketTotal([{
      period_start: "2026-01-01",
      availability_hours: 8,
      load_hours: 4,
      free_hours: 4,
      utilization_pct: 50
    }])

    expect(total.project_load_hours).toBe(0)
    expect(total.project_estimation_hours).toBe(0)
    expect(total.project_completion_pct).toBe(0)
  })

  it("builds report rows with period summaries and expandable details", () => {
    const reportResults: ReportObjectResult[] = [
      {
        objectID: "person_2",
        objectLabel: "Bob",
        buckets: [{
          period_start: "2026-01-01",
          availability_hours: 8,
          load_hours: 4,
          project_load_hours: 0,
          project_estimation_hours: 0,
          free_hours: 4,
          utilization_pct: 50,
          project_completion_pct: 0
        }]
      },
      {
        objectID: "person_1",
        objectLabel: "Alice",
        buckets: [{
          period_start: "2026-01-01",
          availability_hours: 10,
          load_hours: 2,
          project_load_hours: 0,
          project_estimation_hours: 0,
          free_hours: 8,
          utilization_pct: 20,
          project_completion_pct: 0
        }, {
          period_start: "2026-02-01",
          availability_hours: 6,
          load_hours: 3,
          project_load_hours: 0,
          project_estimation_hours: 0,
          free_hours: 3,
          utilization_pct: 50,
          project_completion_pct: 0
        }]
      }
    ]

    const rows = buildReportTableRows(reportResults)
    expect(rows).toHaveLength(4)
    expect(rows[0]?.periodStart).toBe("2026-01-01")
    expect(rows[0]?.objectLabel).toBe("Total")
    expect(rows[0]?.isTotal).toBe(true)
    expect(rows[0]?.isDetail).toBe(false)
    expect(rows[0]?.detailCount).toBe(2)
    expect(rows[0]?.bucket.availability_hours).toBe(18)
    expect(rows[0]?.bucket.load_hours).toBe(6)
    expect(rows[1]?.objectLabel).toBe("Alice")
    expect(rows[1]?.isDetail).toBe(true)
    expect(rows[2]?.objectLabel).toBe("Bob")
    expect(rows[2]?.isDetail).toBe(true)
    expect(rows[3]?.periodStart).toBe("2026-02-01")
    expect(rows[3]?.objectLabel).toBe("Alice")
    expect(rows[3]?.isDetail).toBe(false)
    expect(rows[3]?.detailCount).toBe(0)
  })

  it("handles report row expandability and visibility helpers", () => {
    const rows = buildReportTableRows([
      {
        objectID: "person_1",
        objectLabel: "Alice",
        buckets: [{
          period_start: "2026-01-01",
          availability_hours: 10,
          load_hours: 2,
          free_hours: 8,
          utilization_pct: 20
        }]
      },
      {
        objectID: "person_2",
        objectLabel: "Bob",
        buckets: [{
          period_start: "2026-01-01",
          availability_hours: 8,
          load_hours: 4,
          free_hours: 4,
          utilization_pct: 50
        }]
      }
    ])

    const summaryRow = rows[0]
    const detailRow = rows[1]
    expect(summaryRow).toBeDefined()
    expect(detailRow).toBeDefined()
    expect(isExpandableReportPeriodRow(summaryRow!)).toBe(true)
    expect(reportDetailToggleLabel(summaryRow!, false)).toBe("Show 2 entries")
    expect(reportDetailToggleLabel(summaryRow!, true)).toBe("Hide entries")
    expect(isReportRowVisible(detailRow!, new Set())).toBe(false)
    expect(isReportRowVisible(detailRow!, new Set(["2026-01-01"]))).toBe(true)
  })
})
