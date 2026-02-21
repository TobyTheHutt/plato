import type { ReportScope } from "../reportColumns"
import type {
  Allocation,
  AllocationFormState,
  AllocationLoadInputType,
  AllocationLoadUnit,
  AllocationTargetType,
  DateHoursEntry,
  Person,
  PersonAllocationLoadSegment,
  ReportBucket,
  ReportObjectResult,
  ReportTableRow,
  WorkingTimeUnit
} from "./types"

const DAYS_PER_WEEK = 5
const WEEKS_PER_YEAR = 52
const MONTHS_PER_YEAR = 12
const DAYS_PER_YEAR = DAYS_PER_WEEK * WEEKS_PER_YEAR
const WEEKS_PER_MONTH = WEEKS_PER_YEAR / MONTHS_PER_YEAR
const DAYS_PER_MONTH = DAYS_PER_YEAR / MONTHS_PER_YEAR

export function newAllocationFormState(): AllocationFormState {
  return {
    id: "",
    targetType: "person",
    targetID: "",
    projectID: "",
    startDate: "2026-01-01",
    endDate: "2026-12-31",
    loadInputType: "fte_pct",
    loadUnit: "day",
    loadValue: "0"
  }
}

export function asNumber(value: string): number {
  const parsed = Number(value)
  if (Number.isNaN(parsed)) {
    return 0
  }
  return parsed
}

export function formatHours(value: number | null | undefined): string {
  if (typeof value !== "number" || Number.isNaN(value) || !Number.isFinite(value)) {
    return "n/a"
  }
  return value.toFixed(2)
}

export function toErrorMessage(error: unknown): string {
  if (error instanceof Error) {
    return error.message
  }
  return "unexpected error"
}

export function toWorkingHours(value: number, unit: WorkingTimeUnit): { day: number; week: number; year: number } {
  if (unit === "day") {
    return {
      day: value,
      week: value * DAYS_PER_WEEK,
      year: value * DAYS_PER_YEAR
    }
  }
  if (unit === "week") {
    return {
      day: value / DAYS_PER_WEEK,
      week: value,
      year: value * WEEKS_PER_YEAR
    }
  }
  if (unit === "month") {
    return {
      day: value / DAYS_PER_MONTH,
      week: value / WEEKS_PER_MONTH,
      year: value * MONTHS_PER_YEAR
    }
  }
  return {
    day: value / DAYS_PER_YEAR,
    week: value / WEEKS_PER_YEAR,
    year: value
  }
}

export function workingHoursForAllocationUnit(hoursPerDay: number, unit: AllocationLoadUnit): number {
  if (unit === "day") {
    return hoursPerDay
  }
  if (unit === "week") {
    return hoursPerDay * DAYS_PER_WEEK
  }
  return hoursPerDay * DAYS_PER_MONTH
}

export function allocationPercentFromInput(
  value: number,
  inputType: AllocationLoadInputType,
  unit: AllocationLoadUnit,
  hoursPerDay: number
): number {
  if (inputType === "fte_pct") {
    return roundHours(value)
  }
  if (hoursPerDay <= 0) {
    return 0
  }

  const unitHours = workingHoursForAllocationUnit(hoursPerDay, unit)
  if (unitHours <= 0) {
    return 0
  }
  return roundHours((value / unitHours) * 100)
}

export function parseDateValue(value: string): Date | null {
  if (!value) {
    return null
  }

  const parsed = new Date(`${value}T00:00:00Z`)
  if (Number.isNaN(parsed.getTime())) {
    return null
  }

  return parsed
}

export function formatDateValue(value: Date): string {
  return value.toISOString().slice(0, 10)
}

export function roundHours(value: number): number {
  return Math.round(value * 1000) / 1000
}

export function personDailyHours(hoursPerDay: number, employmentPct: number): number {
  return roundHours((hoursPerDay * employmentPct) / 100)
}

export function isValidMonthValue(value: string): boolean {
  return /^\d{4}-\d{2}$/.test(value)
}

export function personEmploymentPctOnDate(person: Person, date: string): number {
  const month = date.slice(0, 7)
  if (!isValidMonthValue(month)) {
    return person.employment_pct
  }

  let currentEmploymentPct = person.employment_pct
  let latestEffectiveMonth = ""
  for (const change of person.employment_changes ?? []) {
    if (!isValidMonthValue(change.effective_month)) {
      continue
    }
    if (change.effective_month <= month && change.effective_month >= latestEffectiveMonth) {
      currentEmploymentPct = change.employment_pct
      latestEffectiveMonth = change.effective_month
    }
  }

  return currentEmploymentPct
}

export function buildWeekdayEntries(
  startDate: Date,
  endDate: Date,
  hoursPerDay: number
): DateHoursEntry[] {
  if (startDate.getTime() > endDate.getTime()) {
    throw new Error("start date must be before or equal to end date")
  }
  if (hoursPerDay <= 0) {
    throw new Error("organisation working hours must be greater than 0")
  }

  const entries: DateHoursEntry[] = []
  const cursor = new Date(startDate)
  while (cursor.getTime() <= endDate.getTime()) {
    const dayOfWeek = cursor.getUTCDay()
    if (dayOfWeek >= 1 && dayOfWeek <= 5) {
      entries.push({ date: formatDateValue(cursor), hours: roundHours(hoursPerDay) })
    }
    cursor.setUTCDate(cursor.getUTCDate() + 1)
  }

  if (entries.length === 0) {
    throw new Error("timespan must include at least one weekday")
  }

  return entries
}

export function parseWeekStartValue(value: string): Date | null {
  const match = value.match(/^(\d{4})-W(\d{2})$/)
  if (!match) {
    return null
  }

  const year = Number(match[1])
  const week = Number(match[2])
  if (Number.isNaN(year) || Number.isNaN(week) || week < 1 || week > 53) {
    return null
  }

  const januaryFourth = new Date(Date.UTC(year, 0, 4))
  const dayOfWeek = januaryFourth.getUTCDay() || 7
  const firstWeekMonday = new Date(januaryFourth)
  firstWeekMonday.setUTCDate(januaryFourth.getUTCDate() - dayOfWeek + 1)

  const start = new Date(firstWeekMonday)
  start.setUTCDate(firstWeekMonday.getUTCDate() + ((week - 1) * 7))
  return start
}

export function buildDayRangeDateHours(
  startDate: string,
  endDate: string,
  hoursPerDay: number
): DateHoursEntry[] {
  const start = parseDateValue(startDate)
  const end = parseDateValue(endDate)
  if (!start || !end) {
    throw new Error("valid start and end dates are required")
  }
  return buildWeekdayEntries(start, end, hoursPerDay)
}

export function buildWeekRangeDateHours(
  startWeek: string,
  endWeek: string,
  hoursPerDay: number
): DateHoursEntry[] {
  const start = parseWeekStartValue(startWeek)
  const end = parseWeekStartValue(endWeek)
  if (!start || !end) {
    throw new Error("valid start and end weeks are required")
  }

  if (start.getTime() > end.getTime()) {
    throw new Error("start week must be before or equal to end week")
  }

  const endWeekFriday = new Date(end)
  endWeekFriday.setUTCDate(endWeekFriday.getUTCDate() + 4)
  return buildWeekdayEntries(start, endWeekFriday, hoursPerDay)
}

export function normalizeAllocationTargetType(allocation: Allocation): AllocationTargetType {
  if (allocation.target_type === "group") {
    return "group"
  }
  return "person"
}

export function normalizeAllocationTargetID(allocation: Allocation): string {
  if (allocation.target_id) {
    return allocation.target_id
  }
  return allocation.person_id ?? ""
}

export function dateRangesOverlap(
  leftStartDate: string,
  leftEndDate: string,
  rightStartDate: string,
  rightEndDate: string
): boolean {
  const leftStart = parseDateValue(leftStartDate)
  const leftEnd = parseDateValue(leftEndDate)
  const rightStart = parseDateValue(rightStartDate)
  const rightEnd = parseDateValue(rightEndDate)

  if (!leftStart || !leftEnd || !rightStart || !rightEnd) {
    return false
  }

  return leftStart.getTime() <= rightEnd.getTime() && rightStart.getTime() <= leftEnd.getTime()
}

export function hasPersonIntersection(left: string[], right: string[]): boolean {
  if (left.length === 0 || right.length === 0) {
    return false
  }

  const rightSet = new Set(right)
  for (const personID of left) {
    if (rightSet.has(personID)) {
      return true
    }
  }

  return false
}

export function isSubsetOf(candidateValues: string[], referenceValues: string[]): boolean {
  const referenceSet = new Set(referenceValues)
  for (const value of candidateValues) {
    if (!referenceSet.has(value)) {
      return false
    }
  }
  return true
}

export function dateAfterValue(value: string): string | null {
  const date = parseDateValue(value)
  if (!date) {
    return null
  }
  const nextDate = new Date(date)
  nextDate.setUTCDate(nextDate.getUTCDate() + 1)
  return formatDateValue(nextDate)
}

export function isPersonOverallocated(person: Person, segments: PersonAllocationLoadSegment[]): boolean {
  if (segments.length === 0) {
    return false
  }

  const deltaByDate = new Map<string, number>()
  const checkpoints = new Set<string>()
  const addDelta = (date: string, delta: number) => {
    deltaByDate.set(date, (deltaByDate.get(date) ?? 0) + delta)
  }

  for (const segment of segments) {
    const start = parseDateValue(segment.startDate)
    const end = parseDateValue(segment.endDate)
    if (!start || !end || start.getTime() > end.getTime() || segment.percent <= 0) {
      continue
    }

    const startDate = formatDateValue(start)
    const endDate = formatDateValue(end)
    const dateAfterEnd = dateAfterValue(endDate)

    addDelta(startDate, segment.percent)
    checkpoints.add(startDate)
    if (dateAfterEnd) {
      addDelta(dateAfterEnd, -segment.percent)
      checkpoints.add(dateAfterEnd)
    }
  }

  for (const change of person.employment_changes ?? []) {
    if (!isValidMonthValue(change.effective_month)) {
      continue
    }
    checkpoints.add(`${change.effective_month}-01`)
  }

  if (checkpoints.size === 0) {
    return false
  }

  let activePercent = 0
  const sortedCheckpoints = Array.from(checkpoints).sort((left, right) => left.localeCompare(right))
  for (const checkpoint of sortedCheckpoints) {
    activePercent += deltaByDate.get(checkpoint) ?? 0
    const employmentPct = personEmploymentPctOnDate(person, checkpoint)
    if (activePercent > employmentPct + 1e-9) {
      return true
    }
  }

  return false
}

export function reportBucketTotal(buckets: ReportBucket[]): ReportBucket {
  const total: ReportBucket = {
    period_start: "",
    availability_hours: 0,
    load_hours: 0,
    project_load_hours: 0,
    project_estimation_hours: 0,
    free_hours: 0,
    utilization_pct: 0,
    project_completion_pct: 0
  }

  for (const bucket of buckets) {
    total.availability_hours += bucket.availability_hours
    total.load_hours += bucket.load_hours
    total.project_load_hours = (total.project_load_hours ?? 0) + (bucket.project_load_hours ?? 0)
    total.project_estimation_hours = (total.project_estimation_hours ?? 0) + (bucket.project_estimation_hours ?? 0)
    total.free_hours += bucket.free_hours
  }

  if (total.availability_hours > 0) {
    total.utilization_pct = (total.load_hours / total.availability_hours) * 100
  }

  if ((total.project_estimation_hours ?? 0) > 0) {
    total.project_completion_pct = ((total.project_load_hours ?? 0) / (total.project_estimation_hours ?? 0)) * 100
  }

  return total
}

export function buildReportTableRows(reportResults: ReportObjectResult[]): ReportTableRow[] {
  if (reportResults.length === 0) {
    return []
  }

  const rowsByPeriod = new Map<string, Array<{ objectID: string; objectLabel: string; bucket: ReportBucket }>>()
  for (const result of reportResults) {
    for (const bucket of result.buckets) {
      const existingRows = rowsByPeriod.get(bucket.period_start) ?? []
      existingRows.push({
        objectID: result.objectID,
        objectLabel: result.objectLabel,
        bucket
      })
      rowsByPeriod.set(bucket.period_start, existingRows)
    }
  }

  const sortedPeriods = Array.from(rowsByPeriod.keys()).sort((left, right) => left.localeCompare(right))
  const rows: ReportTableRow[] = []

  for (const periodStart of sortedPeriods) {
    const periodRows = rowsByPeriod.get(periodStart) ?? []
    periodRows.sort((left, right) => left.objectLabel.localeCompare(right.objectLabel))
    if (periodRows.length > 1) {
      const totalBucket = reportBucketTotal(periodRows.map((row) => row.bucket))
      totalBucket.period_start = periodStart
      rows.push({
        id: `${periodStart}:summary`,
        periodStart,
        objectID: "total",
        objectLabel: "Total",
        bucket: totalBucket,
        isTotal: true,
        isDetail: false,
        detailCount: periodRows.length
      })
      for (const row of periodRows) {
        rows.push({
          id: `${periodStart}:${row.objectID}`,
          periodStart,
          objectID: row.objectID,
          objectLabel: row.objectLabel,
          bucket: row.bucket,
          isTotal: false,
          isDetail: true,
          detailCount: 0
        })
      }
      continue
    }

    const singleRow = periodRows[0]
    if (!singleRow) {
      continue
    }

    rows.push({
      id: `${periodStart}:summary`,
      periodStart,
      objectID: singleRow.objectID,
      objectLabel: singleRow.objectLabel,
      bucket: singleRow.bucket,
      isTotal: false,
      isDetail: false,
      detailCount: 0
    })
  }

  return rows
}

export function isExpandableReportPeriodRow(row: ReportTableRow): boolean {
  return !row.isDetail && row.detailCount > 1
}

export function isReportRowVisible(row: ReportTableRow, expandedPeriods: ReadonlySet<string>): boolean {
  if (!row.isDetail) {
    return true
  }
  return expandedPeriods.has(row.periodStart)
}

export function reportDetailToggleLabel(row: ReportTableRow, isExpanded: boolean): string {
  if (!isExpandableReportPeriodRow(row)) {
    return ""
  }
  if (isExpanded) {
    return "Hide entries"
  }
  return `Show ${row.detailCount} entries`
}

export function reportUtilizationForDisplay(
  row: ReportTableRow,
  scope: ReportScope,
  personsByID: Map<string, Person>
): number {
  if (scope !== "person" || row.objectID === "total") {
    return row.bucket.utilization_pct
  }

  const person = personsByID.get(row.objectID)
  if (!person) {
    return row.bucket.utilization_pct
  }

  const employmentPct = personEmploymentPctOnDate(person, row.periodStart)
  if (employmentPct <= 0) {
    return 0
  }

  return row.bucket.utilization_pct * (employmentPct / 100)
}
