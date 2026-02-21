import { FormEvent, useCallback, useEffect, useMemo, useRef, useState } from "react"
import { showAvailabilityMetrics, showProjectMetrics, type ReportScope } from "./reportColumns"

export type Role = "org_admin" | "org_user"

export type Organisation = {
  id: string
  name: string
  hours_per_day: number
  hours_per_week: number
  hours_per_year: number
}

export type EmploymentChange = {
  effective_month: string
  employment_pct: number
}

export type Person = {
  id: string
  organisation_id: string
  name: string
  employment_pct: number
  employment_changes?: EmploymentChange[]
}

export type Project = {
  id: string
  organisation_id: string
  name: string
  start_date: string
  end_date: string
  estimated_effort_hours: number
}

export type Group = {
  id: string
  organisation_id: string
  name: string
  member_ids: string[]
}

export type Allocation = {
  id: string
  organisation_id: string
  target_type: AllocationTargetType
  target_id: string
  project_id: string
  start_date: string
  end_date: string
  percent: number
  person_id?: string
}

export type AllocationTargetType = "person" | "group"
export type AllocationLoadInputType = "fte_pct" | "hours"
export type AllocationLoadUnit = "day" | "week" | "month"
export type AllocationMergeStrategy = "stack" | "replace" | "keep"
export type AllocationFormState = {
  id: string
  targetType: AllocationTargetType
  targetID: string
  projectID: string
  startDate: string
  endDate: string
  loadInputType: AllocationLoadInputType
  loadUnit: AllocationLoadUnit
  loadValue: string
}

export type PersonUnavailability = {
  id: string
  organisation_id: string
  person_id: string
  date: string
  hours: number
}

export type ReportBucket = {
  period_start: string
  availability_hours: number
  load_hours: number
  project_load_hours?: number
  project_estimation_hours?: number
  free_hours: number
  utilization_pct: number
  project_completion_pct?: number
}

export type ReportObjectResult = {
  objectID: string
  objectLabel: string
  buckets: ReportBucket[]
}

export type ReportTableRow = {
  id: string
  periodStart: string
  objectLabel: string
  bucket: ReportBucket
  isTotal: boolean
}

export type ReportGranularity = "day" | "week" | "month" | "year"

export type WorkingTimeUnit = "day" | "week" | "month" | "year"
export type AvailabilityScope = "organisation" | "person" | "group"
export type AvailabilityUnitScope = "hours" | "days" | "weeks"
export type DateHoursEntry = {
  date: string
  hours: number
}
export type PersonDateHoursEntry = {
  personID: string
  date: string
  hours: number
}
export type PersonAllocationLoadSegment = {
  startDate: string
  endDate: string
  percent: number
}

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL ?? "http://localhost:8070"

const DAYS_PER_WEEK = 5
const WEEKS_PER_YEAR = 52
const MONTHS_PER_YEAR = 12
const DAYS_PER_YEAR = DAYS_PER_WEEK * WEEKS_PER_YEAR
const WEEKS_PER_MONTH = WEEKS_PER_YEAR / MONTHS_PER_YEAR
const DAYS_PER_MONTH = DAYS_PER_YEAR / MONTHS_PER_YEAR
const PERSON_UNAVAILABILITY_LOAD_CONCURRENCY = 5
const REPORT_MULTI_OBJECT_CONCURRENCY = 5

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
    for (const row of periodRows) {
      rows.push({
        id: `${periodStart}:${row.objectID}`,
        periodStart,
        objectLabel: row.objectLabel,
        bucket: row.bucket,
        isTotal: false
      })
    }

    if (periodRows.length > 1) {
      const totalBucket = reportBucketTotal(periodRows.map((row) => row.bucket))
      totalBucket.period_start = periodStart
      rows.push({
        id: `${periodStart}:total`,
        periodStart,
        objectLabel: "Total",
        bucket: totalBucket,
        isTotal: true
      })
    }
  }

  return rows
}

export default function App() {
  const canUseNetwork = typeof fetch === "function"

  const [role, setRole] = useState<Role>("org_admin")
  const [organisations, setOrganisations] = useState<Organisation[]>([])
  const [selectedOrganisationID, setSelectedOrganisationID] = useState("")

  const [persons, setPersons] = useState<Person[]>([])
  const [projects, setProjects] = useState<Project[]>([])
  const [groups, setGroups] = useState<Group[]>([])
  const [allocations, setAllocations] = useState<Allocation[]>([])
  const [personUnavailability, setPersonUnavailability] = useState<PersonUnavailability[]>([])
  const [selectedPersonIDs, setSelectedPersonIDs] = useState<string[]>([])
  const [selectedProjectIDs, setSelectedProjectIDs] = useState<string[]>([])
  const [selectedGroupIDs, setSelectedGroupIDs] = useState<string[]>([])
  const [selectedAllocationIDs, setSelectedAllocationIDs] = useState<string[]>([])

  const [errorMessage, setErrorMessage] = useState("")
  const [successMessage, setSuccessMessage] = useState("")

  const [organisationForm, setOrganisationForm] = useState({
    id: "",
    name: "",
    workingTimeValue: "8",
    workingTimeUnit: "day" as WorkingTimeUnit
  })

  const [personForm, setPersonForm] = useState({
    id: "",
    name: "",
    employmentPct: "100",
    employmentEffectiveFromMonth: ""
  })
  const [projectForm, setProjectForm] = useState({
    id: "",
    name: "",
    startDate: "2026-01-01",
    endDate: "2026-12-31",
    estimatedEffortHours: "1000"
  })
  const [groupForm, setGroupForm] = useState({ id: "", name: "", memberIDs: [] as string[] })
  const [groupMemberForm, setGroupMemberForm] = useState({ groupID: "", personID: "" })

  const [allocationForm, setAllocationForm] = useState<AllocationFormState>(() => newAllocationFormState())
  const [allocationMergeStrategy, setAllocationMergeStrategy] = useState<AllocationMergeStrategy>("stack")

  const [holidayForm, setHolidayForm] = useState({ date: "", hours: "8" })
  const [personUnavailabilityForm, setPersonUnavailabilityForm] = useState({
    personID: "",
    date: "",
    hours: "8"
  })
  const [groupUnavailabilityForm, setGroupUnavailabilityForm] = useState({
    groupID: "",
    date: "",
    hours: "8"
  })
  const [availabilityScope, setAvailabilityScope] = useState<AvailabilityScope>("organisation")
  const [availabilityUnitScope, setAvailabilityUnitScope] = useState<AvailabilityUnitScope>("hours")
  const [timespanForm, setTimespanForm] = useState({
    startDate: "",
    endDate: "",
    startWeek: "",
    endWeek: ""
  })

  const [reportScope, setReportScope] = useState<ReportScope>("organisation")
  const [reportIDs, setReportIDs] = useState<string[]>([])
  const [reportFromDate, setReportFromDate] = useState("2026-01-01")
  const [reportToDate, setReportToDate] = useState("2026-01-31")
  const [reportGranularity, setReportGranularity] = useState<ReportGranularity>("month")
  const [reportResults, setReportResults] = useState<ReportObjectResult[]>([])
  const selectAllPersonsCheckboxRef = useRef<HTMLInputElement | null>(null)
  const selectAllProjectsCheckboxRef = useRef<HTMLInputElement | null>(null)
  const selectAllGroupsCheckboxRef = useRef<HTMLInputElement | null>(null)
  const selectAllAllocationsCheckboxRef = useRef<HTMLInputElement | null>(null)

  const authHeaders = useCallback(
    (organisationID?: string): HeadersInit => {
      const headers: Record<string, string> = {
        "Content-Type": "application/json",
        "X-Role": role,
        "X-User-ID": "dev-user"
      }

      const scopedOrganisationID = organisationID ?? selectedOrganisationID
      if (scopedOrganisationID) {
        headers["X-Org-ID"] = scopedOrganisationID
      }

      return headers
    },
    [role, selectedOrganisationID]
  )

  const sendRequest = useCallback(
    async (path: string, options?: RequestInit, organisationID?: string): Promise<Response> => {
      if (!canUseNetwork) {
        throw new Error("fetch is not available")
      }

      return fetch(`${API_BASE_URL}${path}`, {
        ...options,
        headers: {
          ...authHeaders(organisationID),
          ...(options?.headers ?? {})
        }
      })
    },
    [authHeaders, canUseNetwork]
  )

  const requestJSON = useCallback(
    async <T,>(path: string, options?: RequestInit, organisationID?: string, defaultResponse?: T): Promise<T> => {
      const response = await sendRequest(path, options, organisationID)
      if (response.status === 204) {
        throw new Error(`request returned no content for ${path}`)
      }

      const text = await response.text()
      const hasBody = text.trim() !== ""
      const payload = hasBody ? JSON.parse(text) : defaultResponse

      if (!response.ok) {
        const message = (
          payload
          && typeof payload === "object"
          && "error" in payload
          && typeof payload.error === "string"
        )
          ? payload.error
          : `request failed with status ${response.status}`
        throw new Error(message)
      }

      if (!hasBody) {
        if (defaultResponse !== undefined) {
          return defaultResponse
        }
        throw new Error(`request returned empty body for ${path}`)
      }

      return payload as T
    },
    [sendRequest]
  )

  const requestNoContent = useCallback(
    async (path: string, options?: RequestInit, organisationID?: string): Promise<void> => {
      const response = await sendRequest(path, options, organisationID)
      if (response.ok) {
        return
      }

      const text = await response.text()
      if (!text) {
        throw new Error(`request failed with status ${response.status}`)
      }

      let message = `request failed with status ${response.status}`
      try {
        const payload = JSON.parse(text) as Record<string, unknown>
        if (typeof payload.error === "string") {
          message = payload.error
        }
      } catch {
        if (text.trim() !== "") {
          message = `request failed with status ${response.status}: ${text}`
        }
      }
      throw new Error(message)
    },
    [sendRequest]
  )

  const withFeedback = useCallback(async (operation: () => Promise<void>, success: string) => {
    setErrorMessage("")
    setSuccessMessage("")

    try {
      await operation()
      setSuccessMessage(success)
    } catch (error) {
      setErrorMessage(toErrorMessage(error))
    }
  }, [])

  const loadOrganisations = useCallback(async () => {
    const loaded = await requestJSON<Organisation[]>("/api/organisations", { method: "GET" }, "", [])
    setOrganisations(loaded)
    setSelectedOrganisationID((current) => {
      if (loaded.length === 0) {
        return ""
      }
      if (current && loaded.some((organisation) => organisation.id === current)) {
        return current
      }
      return loaded[0].id
    })
  }, [requestJSON])

  const loadOrganisationScopedData = useCallback(async () => {
    if (!selectedOrganisationID) {
      setPersons([])
      setProjects([])
      setGroups([])
      setAllocations([])
      setPersonUnavailability([])
      return
    }

    const [loadedPersons, loadedProjects, loadedGroups, loadedAllocations] = await Promise.all([
      requestJSON<Person[]>("/api/persons", { method: "GET" }, undefined, []),
      requestJSON<Project[]>("/api/projects", { method: "GET" }, undefined, []),
      requestJSON<Group[]>("/api/groups", { method: "GET" }, undefined, []),
      requestJSON<Allocation[]>("/api/allocations", { method: "GET" }, undefined, [])
    ])

    const personEntries: PersonUnavailability[] = []
    const personLoadErrors: string[] = []

    for (let index = 0; index < loadedPersons.length; index += PERSON_UNAVAILABILITY_LOAD_CONCURRENCY) {
      const personBatch = loadedPersons.slice(index, index + PERSON_UNAVAILABILITY_LOAD_CONCURRENCY)
      const settledBatch = await Promise.allSettled(
        personBatch.map((person) => requestJSON<PersonUnavailability[]>(`/api/persons/${person.id}/unavailability`, { method: "GET" }, undefined, []))
      )

      settledBatch.forEach((result, resultIndex) => {
        if (result.status === "fulfilled") {
          personEntries.push(...result.value)
          return
        }
        const failedPersonID = personBatch[resultIndex]?.id ?? "unknown"
        personLoadErrors.push(`${failedPersonID}: ${toErrorMessage(result.reason)}`)
      })
    }

    if (personLoadErrors.length > 0) {
      throw new Error(
        `failed to load unavailability for ${personLoadErrors.length} person(s): ${personLoadErrors.join(", ")}`
      )
    }

    setPersons(loadedPersons)
    setProjects(loadedProjects)
    setGroups(loadedGroups)
    setAllocations(loadedAllocations)
    setPersonUnavailability(personEntries)
  }, [requestJSON, selectedOrganisationID])

  useEffect(() => {
    if (!canUseNetwork) {
      return
    }

    void withFeedback(async () => {
      await loadOrganisations()
    }, "")
  }, [canUseNetwork, loadOrganisations, withFeedback])

  useEffect(() => {
    if (!canUseNetwork) {
      return
    }

    void withFeedback(async () => {
      await loadOrganisationScopedData()
    }, "")
  }, [canUseNetwork, loadOrganisationScopedData, withFeedback])

  const activeOrganisation = useMemo(
    () => organisations.find((organisation) => organisation.id === selectedOrganisationID),
    [organisations, selectedOrganisationID]
  )

  useEffect(() => {
    if (!activeOrganisation) {
      setOrganisationForm({
        id: "",
        name: "",
        workingTimeValue: "8",
        workingTimeUnit: "day"
      })
      return
    }

    setOrganisationForm({
      id: activeOrganisation.id,
      name: activeOrganisation.name,
      workingTimeValue: String(activeOrganisation.hours_per_day),
      workingTimeUnit: "day"
    })
  }, [activeOrganisation])

  const selectableReportItems = useMemo(() => {
    if (reportScope === "person") {
      return persons.map((person) => ({ id: person.id, label: `${person.name} (${person.employment_pct}%)` }))
    }
    if (reportScope === "group") {
      return groups.map((group) => ({ id: group.id, label: `${group.name} (${group.member_ids.length} members)` }))
    }
    if (reportScope === "project") {
      return projects.map((project) => ({ id: project.id, label: project.name }))
    }
    return []
  }, [groups, persons, projects, reportScope])

  const reportItemLabelByID = useMemo(
    () => new Map(selectableReportItems.map((entry) => [entry.id, entry.label])),
    [selectableReportItems]
  )

  useEffect(() => {
    setSelectedPersonIDs([])
  }, [persons])

  useEffect(() => {
    setSelectedProjectIDs([])
  }, [projects])

  useEffect(() => {
    setSelectedGroupIDs([])
  }, [groups])

  useEffect(() => {
    setSelectedAllocationIDs([])
  }, [allocations])

  useEffect(() => {
    if (!selectAllPersonsCheckboxRef.current) {
      return
    }
    selectAllPersonsCheckboxRef.current.indeterminate =
      selectedPersonIDs.length > 0 && selectedPersonIDs.length < persons.length
  }, [persons.length, selectedPersonIDs.length])

  useEffect(() => {
    if (!selectAllProjectsCheckboxRef.current) {
      return
    }
    selectAllProjectsCheckboxRef.current.indeterminate =
      selectedProjectIDs.length > 0 && selectedProjectIDs.length < projects.length
  }, [projects.length, selectedProjectIDs.length])

  useEffect(() => {
    if (!selectAllGroupsCheckboxRef.current) {
      return
    }
    selectAllGroupsCheckboxRef.current.indeterminate =
      selectedGroupIDs.length > 0 && selectedGroupIDs.length < groups.length
  }, [groups.length, selectedGroupIDs.length])

  useEffect(() => {
    if (!selectAllAllocationsCheckboxRef.current) {
      return
    }
    selectAllAllocationsCheckboxRef.current.indeterminate =
      selectedAllocationIDs.length > 0 && selectedAllocationIDs.length < allocations.length
  }, [allocations.length, selectedAllocationIDs.length])

  const allocationTargetOptions = useMemo(() => {
    if (allocationForm.targetType === "person") {
      return persons.map((person) => ({ id: person.id, label: person.name }))
    }
    return groups.map((group) => ({ id: group.id, label: group.name }))
  }, [allocationForm.targetType, groups, persons])

  const personNameByID = useMemo(
    () => new Map(persons.map((person) => [person.id, person.name])),
    [persons]
  )

  const resolveTargetPersonIDs = useCallback((targetType: AllocationTargetType, targetID: string): string[] => {
    if (targetType === "person") {
      return persons.some((person) => person.id === targetID) ? [targetID] : []
    }

    const group = groups.find((entry) => entry.id === targetID)
    if (!group) {
      return []
    }

    return Array.from(new Set(group.member_ids))
  }, [groups, persons])

  const resolveAllocationPersonIDs = useCallback((allocation: Allocation): string[] => {
    return resolveTargetPersonIDs(
      normalizeAllocationTargetType(allocation),
      normalizeAllocationTargetID(allocation)
    )
  }, [resolveTargetPersonIDs])

  const overallocatedPersonIDsForAllocations = useCallback((allocationEntries: Allocation[]): string[] => {
    const segmentsByPersonID = new Map<string, PersonAllocationLoadSegment[]>()
    for (const allocation of allocationEntries) {
      const personIDs = resolveAllocationPersonIDs(allocation)
      for (const personID of personIDs) {
        const existing = segmentsByPersonID.get(personID) ?? []
        existing.push({
          startDate: allocation.start_date,
          endDate: allocation.end_date,
          percent: allocation.percent
        })
        segmentsByPersonID.set(personID, existing)
      }
    }

    return persons
      .filter((person) => isPersonOverallocated(person, segmentsByPersonID.get(person.id) ?? []))
      .map((person) => person.id)
  }, [persons, resolveAllocationPersonIDs])

  const overallocatedPersonIDs = useMemo(
    () => overallocatedPersonIDsForAllocations(allocations),
    [allocations, overallocatedPersonIDsForAllocations]
  )

  const overallocatedPersonIDSet = useMemo(
    () => new Set(overallocatedPersonIDs),
    [overallocatedPersonIDs]
  )

  const isOverallocatedPersonID = useCallback(
    (personID: string) => overallocatedPersonIDSet.has(personID),
    [overallocatedPersonIDSet]
  )

  const findConflictingAllocations = useCallback((candidate: {
    id?: string
    targetType: AllocationTargetType
    targetID: string
    projectID: string
    startDate: string
    endDate: string
  }): Allocation[] => {
    const targetID = candidate.targetID.trim()
    const projectID = candidate.projectID.trim()
    if (!targetID || !projectID) {
      return []
    }

    const candidatePersons = resolveTargetPersonIDs(candidate.targetType, targetID)
    if (candidatePersons.length === 0) {
      return []
    }

    return allocations.filter((allocation) => {
      if (candidate.id && allocation.id === candidate.id) {
        return false
      }
      if (allocation.project_id !== projectID) {
        return false
      }
      if (!dateRangesOverlap(
        candidate.startDate,
        candidate.endDate,
        allocation.start_date,
        allocation.end_date
      )) {
        return false
      }

      const allocationPersons = resolveAllocationPersonIDs(allocation)
      return hasPersonIntersection(candidatePersons, allocationPersons)
    })
  }, [allocations, resolveAllocationPersonIDs, resolveTargetPersonIDs])

  const allocationFormConflicts = useMemo(() => {
    return findConflictingAllocations({
      id: allocationForm.id || undefined,
      targetType: allocationForm.targetType,
      targetID: allocationForm.targetID,
      projectID: allocationForm.projectID,
      startDate: allocationForm.startDate,
      endDate: allocationForm.endDate
    })
  }, [
    allocationForm.endDate,
    allocationForm.id,
    allocationForm.projectID,
    allocationForm.startDate,
    allocationForm.targetID,
    allocationForm.targetType,
    findConflictingAllocations
  ])

  const allocationFormSelectedPersonIDs = useMemo(
    () => resolveTargetPersonIDs(allocationForm.targetType, allocationForm.targetID),
    [allocationForm.targetID, allocationForm.targetType, resolveTargetPersonIDs]
  )

  const allocationFormConflictingSelectedPersonIDs = useMemo(() => {
    if (allocationFormConflicts.length === 0 || allocationFormSelectedPersonIDs.length === 0) {
      return []
    }

    const selectedSet = new Set(allocationFormSelectedPersonIDs)
    const conflicting = new Set<string>()
    for (const allocation of allocationFormConflicts) {
      for (const personID of resolveAllocationPersonIDs(allocation)) {
        if (selectedSet.has(personID)) {
          conflicting.add(personID)
        }
      }
    }

    return Array.from(conflicting)
  }, [allocationFormConflicts, allocationFormSelectedPersonIDs, resolveAllocationPersonIDs])

  const allocationFormConflictingSelectedPersonNames = useMemo(() => {
    if (allocationFormConflictingSelectedPersonIDs.length === 0) {
      return []
    }

    return allocationFormConflictingSelectedPersonIDs
      .map((personID) => personNameByID.get(personID) ?? personID)
      .sort((left, right) => left.localeCompare(right))
  }, [allocationFormConflictingSelectedPersonIDs, personNameByID])

  const editingAllocation = useMemo(
    () => allocations.find((allocation) => allocation.id === allocationForm.id),
    [allocationForm.id, allocations]
  )

  const allocationFormContextLabel = useMemo(() => {
    if (!editingAllocation) {
      return "Creation context: creating a new allocation."
    }

    const targetType = normalizeAllocationTargetType(editingAllocation)
    const targetID = normalizeAllocationTargetID(editingAllocation)
    const targetLabel = targetType === "group"
      ? groups.find((entry) => entry.id === targetID)?.name ?? targetID
      : persons.find((entry) => entry.id === targetID)?.name ?? targetID
    const projectLabel = projects.find((entry) => entry.id === editingAllocation.project_id)?.name ?? editingAllocation.project_id
    return `Edit context: ${targetType}: ${targetLabel} -> ${projectLabel}`
  }, [editingAllocation, groups, persons, projects])

  const allocationFormCanReplaceConflicts = useMemo(() => {
    if (allocationFormConflicts.length === 0) {
      return true
    }

    const selectedPersons = resolveTargetPersonIDs(allocationForm.targetType, allocationForm.targetID)
    if (selectedPersons.length === 0) {
      return false
    }

    return allocationFormConflicts.every((allocation) =>
      isSubsetOf(resolveAllocationPersonIDs(allocation), selectedPersons)
    )
  }, [
    allocationForm.targetID,
    allocationForm.targetType,
    allocationFormConflicts,
    resolveAllocationPersonIDs,
    resolveTargetPersonIDs
  ])

  useEffect(() => {
    if (allocationFormConflicts.length === 0) {
      setAllocationMergeStrategy("stack")
    }
  }, [allocationFormConflicts.length])

  useEffect(() => {
    if (!allocationFormCanReplaceConflicts && allocationMergeStrategy === "replace") {
      setAllocationMergeStrategy("stack")
    }
  }, [allocationFormCanReplaceConflicts, allocationMergeStrategy])

  const selectedGroupPersonUnavailability = useMemo(() => {
    const group = groups.find((entry) => entry.id === groupUnavailabilityForm.groupID)
    if (!group) {
      return []
    }

    const memberIDs = new Set(group.member_ids)
    return personUnavailability.filter((entry) => memberIDs.has(entry.person_id))
  }, [groupUnavailabilityForm.groupID, groups, personUnavailability])

  const buildScopedPersonUnavailabilityEntries = (scope: AvailabilityScope): PersonDateHoursEntry[] => {
    if (!activeOrganisation) {
      throw new Error("select an organisation first")
    }

    let scopedPersons: Person[] = []
    if (scope === "organisation") {
      scopedPersons = persons
    } else if (scope === "person") {
      const person = persons.find((entry) => entry.id === personUnavailabilityForm.personID)
      if (person) {
        scopedPersons = [person]
      }
    } else {
      const group = groups.find((entry) => entry.id === groupUnavailabilityForm.groupID)
      if (group) {
        const personsByID = new Map(persons.map((person) => [person.id, person]))
        const uniqueMembers = new Set(group.member_ids)
        scopedPersons = Array.from(uniqueMembers)
          .map((memberID) => personsByID.get(memberID))
          .filter((person): person is Person => Boolean(person))
      }
    }

    if (scopedPersons.length === 0) {
      throw new Error("select at least one person for unavailability")
    }

    if (availabilityUnitScope === "hours") {
      const date = scope === "organisation"
        ? holidayForm.date
        : scope === "person"
          ? personUnavailabilityForm.date
          : groupUnavailabilityForm.date
      const hours = scope === "organisation"
        ? asNumber(holidayForm.hours)
        : scope === "person"
          ? asNumber(personUnavailabilityForm.hours)
          : asNumber(groupUnavailabilityForm.hours)

      if (!date) {
        throw new Error("date is required")
      }

      return scopedPersons.map((person) => {
        const maxHours = personDailyHours(
          activeOrganisation.hours_per_day,
          personEmploymentPctOnDate(person, date)
        )
        if (hours > maxHours + 1e-9) {
          throw new Error(`${person.name} max daily unavailability is ${formatHours(maxHours)} hours`)
        }
        const existingHours = personUnavailability
          .filter((entry) => entry.person_id === person.id && entry.date === date)
          .reduce((total, entry) => total + entry.hours, 0)
        if (existingHours + hours > maxHours + 1e-9) {
          throw new Error(`${person.name} already has unavailability on ${date}`)
        }
        return {
          personID: person.id,
          date,
          hours
        }
      })
    }

    const dates = availabilityUnitScope === "days"
      ? buildDayRangeDateHours(timespanForm.startDate, timespanForm.endDate, activeOrganisation.hours_per_day).map((entry) => entry.date)
      : buildWeekRangeDateHours(timespanForm.startWeek, timespanForm.endWeek, activeOrganisation.hours_per_day).map((entry) => entry.date)

    const entries = scopedPersons.flatMap((person) => {
      const personEntries: PersonDateHoursEntry[] = []
      for (const date of dates) {
        const maxHours = personDailyHours(
          activeOrganisation.hours_per_day,
          personEmploymentPctOnDate(person, date)
        )
        if (maxHours <= 0) {
          continue
        }
        const existingHours = personUnavailability
          .filter((entry) => entry.person_id === person.id && entry.date === date)
          .reduce((total, entry) => total + entry.hours, 0)
        const remainingHours = maxHours - existingHours
        if (remainingHours <= 1e-9) {
          throw new Error(`${person.name} already has unavailability on ${date}`)
        }
        personEntries.push({
          personID: person.id,
          date,
          hours: remainingHours
        })
      }
      return personEntries
    })

    if (entries.length === 0) {
      throw new Error("no eligible person capacity for selected scope")
    }

    return entries
  }

  const createPersonUnavailabilityEntries = async (entries: PersonDateHoursEntry[]) => {
    const createResults = await Promise.allSettled(
      entries.map((entry) => requestJSON<PersonUnavailability>(`/api/persons/${entry.personID}/unavailability`, {
        method: "POST",
        body: JSON.stringify({ date: entry.date, hours: entry.hours })
      }))
    )

    const failedCreates: string[] = []
    createResults.forEach((result, index) => {
      if (result.status === "rejected") {
        const entry = entries[index]
        failedCreates.push(`${entry.personID} on ${entry.date}: ${toErrorMessage(result.reason)}`)
      }
    })
    if (failedCreates.length > 0) {
      const createdCount = createResults.length - failedCreates.length
      throw new Error(
        `created ${createdCount} of ${createResults.length} unavailability entries. failed ${failedCreates.length}: ${failedCreates.join(", ")}`
      )
    }
  }

  const handleOrganisationCreate = (event: FormEvent) => {
    event.preventDefault()

    void withFeedback(async () => {
      const workingHours = toWorkingHours(asNumber(organisationForm.workingTimeValue), organisationForm.workingTimeUnit)

      await requestJSON<Organisation>(
        "/api/organisations",
        {
          method: "POST",
          body: JSON.stringify({
            name: organisationForm.name,
            hours_per_day: workingHours.day,
            hours_per_week: workingHours.week,
            hours_per_year: workingHours.year
          })
        },
        ""
      )
      await loadOrganisations()
    }, "organisation created")
  }

  const handleOrganisationUpdate = () => {
    if (!selectedOrganisationID) {
      return
    }

    void withFeedback(async () => {
      const workingHours = toWorkingHours(asNumber(organisationForm.workingTimeValue), organisationForm.workingTimeUnit)

      await requestJSON<Organisation>(`/api/organisations/${selectedOrganisationID}`, {
        method: "PUT",
        body: JSON.stringify({
          name: organisationForm.name,
          hours_per_day: workingHours.day,
          hours_per_week: workingHours.week,
          hours_per_year: workingHours.year
        })
      })
      await loadOrganisations()
      await loadOrganisationScopedData()
    }, "organisation updated")
  }

  const handleOrganisationDelete = () => {
    if (!selectedOrganisationID) {
      return
    }

    void withFeedback(async () => {
      await requestNoContent(`/api/organisations/${selectedOrganisationID}`, { method: "DELETE" })
      await loadOrganisations()
    }, "organisation deleted")
  }

  const switchPersonToCreateContext = () => {
    setPersonForm({ id: "", name: "", employmentPct: "100", employmentEffectiveFromMonth: "" })
  }

  const switchPersonToEditContext = (person: Person) => {
    setPersonForm({
      id: person.id,
      name: person.name,
      employmentPct: String(person.employment_pct),
      employmentEffectiveFromMonth: ""
    })
  }

  const switchProjectToCreateContext = () => {
    setProjectForm({
      id: "",
      name: "",
      startDate: "2026-01-01",
      endDate: "2026-12-31",
      estimatedEffortHours: "1000"
    })
  }

  const switchProjectToEditContext = (project: Project) => {
    setProjectForm({
      id: project.id,
      name: project.name,
      startDate: project.start_date,
      endDate: project.end_date,
      estimatedEffortHours: String(project.estimated_effort_hours)
    })
  }

  const switchGroupToCreateContext = () => {
    setGroupForm({ id: "", name: "", memberIDs: [] })
  }

  const switchGroupToEditContext = (group: Group) => {
    setGroupForm({ id: group.id, name: group.name, memberIDs: group.member_ids })
  }

  const confirmMultiDelete = (count: number, label: string): boolean => {
    if (!window.confirm(`Delete ${count} ${label}?`)) {
      return false
    }
    return window.confirm(`Please confirm again to delete ${count} ${label}.`)
  }

  const deleteSelectedEntries = async (
    ids: string[],
    deleteByID: (id: string) => Promise<void>,
    label: string
  ) => {
    const deleteResults: PromiseSettledResult<void>[] = []
    for (let index = 0; index < ids.length; index += REPORT_MULTI_OBJECT_CONCURRENCY) {
      const batchIDs = ids.slice(index, index + REPORT_MULTI_OBJECT_CONCURRENCY)
      const batchResults = await Promise.allSettled(batchIDs.map((id) => deleteByID(id)))
      deleteResults.push(...batchResults)
    }
    const failedDeletes: string[] = []
    deleteResults.forEach((result, index) => {
      if (result.status === "rejected") {
        failedDeletes.push(`${ids[index]}: ${toErrorMessage(result.reason)}`)
      }
    })
    if (failedDeletes.length > 0) {
      const deletedCount = deleteResults.length - failedDeletes.length
      throw new Error(
        `deleted ${deletedCount} of ${deleteResults.length} ${label}. failed ${failedDeletes.length}: ${failedDeletes.join(", ")}`
      )
    }
  }

  const savePerson = (event: FormEvent) => {
    event.preventDefault()
    if (!selectedOrganisationID) {
      return
    }

    void withFeedback(async () => {
      if (personForm.id) {
        const payload: Record<string, unknown> = {
          name: personForm.name,
          employment_pct: asNumber(personForm.employmentPct)
        }
        if (personForm.employmentEffectiveFromMonth) {
          payload.employment_effective_from_month = personForm.employmentEffectiveFromMonth
        }

        await requestJSON<Person>(`/api/persons/${personForm.id}`, {
          method: "PUT",
          body: JSON.stringify(payload)
        })
      } else {
        await requestJSON<Person>("/api/persons", {
          method: "POST",
          body: JSON.stringify({ name: personForm.name, employment_pct: asNumber(personForm.employmentPct) })
        })
      }
      switchPersonToCreateContext()
      await loadOrganisationScopedData()
    }, personForm.id ? "person updated" : "person created")
  }

  const deletePerson = (personID: string) => {
    void withFeedback(async () => {
      await requestNoContent(`/api/persons/${personID}`, { method: "DELETE" })
      setSelectedPersonIDs((current) => current.filter((id) => id !== personID))
      if (personForm.id === personID) {
        switchPersonToCreateContext()
      }
      await loadOrganisationScopedData()
    }, "person deleted")
  }

  const deleteSelectedPersons = () => {
    if (selectedPersonIDs.length < 2) {
      return
    }
    const personIDs = [...selectedPersonIDs]
    if (!confirmMultiDelete(personIDs.length, "persons")) {
      return
    }

    void withFeedback(async () => {
      await deleteSelectedEntries(
        personIDs,
        (personID) => requestNoContent(`/api/persons/${personID}`, { method: "DELETE" }),
        "persons"
      )
      if (personForm.id && personIDs.includes(personForm.id)) {
        switchPersonToCreateContext()
      }
      setSelectedPersonIDs([])
      await loadOrganisationScopedData()
    }, "selected persons deleted")
  }

  const saveProject = (event: FormEvent) => {
    event.preventDefault()
    if (!selectedOrganisationID) {
      return
    }

    void withFeedback(async () => {
      if (projectForm.id) {
        await requestJSON<Project>(`/api/projects/${projectForm.id}`, {
          method: "PUT",
          body: JSON.stringify({
            name: projectForm.name,
            start_date: projectForm.startDate,
            end_date: projectForm.endDate,
            estimated_effort_hours: asNumber(projectForm.estimatedEffortHours)
          })
        })
      } else {
        await requestJSON<Project>("/api/projects", {
          method: "POST",
          body: JSON.stringify({
            name: projectForm.name,
            start_date: projectForm.startDate,
            end_date: projectForm.endDate,
            estimated_effort_hours: asNumber(projectForm.estimatedEffortHours)
          })
        })
      }
      switchProjectToCreateContext()
      await loadOrganisationScopedData()
    }, projectForm.id ? "project updated" : "project created")
  }

  const deleteProject = (projectID: string) => {
    void withFeedback(async () => {
      await requestNoContent(`/api/projects/${projectID}`, { method: "DELETE" })
      setSelectedProjectIDs((current) => current.filter((id) => id !== projectID))
      if (projectForm.id === projectID) {
        switchProjectToCreateContext()
      }
      await loadOrganisationScopedData()
    }, "project deleted")
  }

  const deleteSelectedProjects = () => {
    if (selectedProjectIDs.length < 2) {
      return
    }
    const projectIDs = [...selectedProjectIDs]
    if (!confirmMultiDelete(projectIDs.length, "projects")) {
      return
    }

    void withFeedback(async () => {
      await deleteSelectedEntries(
        projectIDs,
        (projectID) => requestNoContent(`/api/projects/${projectID}`, { method: "DELETE" }),
        "projects"
      )
      if (projectForm.id && projectIDs.includes(projectForm.id)) {
        switchProjectToCreateContext()
      }
      setSelectedProjectIDs([])
      await loadOrganisationScopedData()
    }, "selected projects deleted")
  }

  const saveGroup = (event: FormEvent) => {
    event.preventDefault()

    void withFeedback(async () => {
      if (groupForm.id) {
        await requestJSON<Group>(`/api/groups/${groupForm.id}`, {
          method: "PUT",
          body: JSON.stringify({ name: groupForm.name, member_ids: groupForm.memberIDs })
        })
      } else {
        await requestJSON<Group>("/api/groups", {
          method: "POST",
          body: JSON.stringify({ name: groupForm.name, member_ids: groupForm.memberIDs })
        })
      }
      switchGroupToCreateContext()
      await loadOrganisationScopedData()
    }, groupForm.id ? "group updated" : "group created")
  }

  const deleteGroup = (groupID: string) => {
    void withFeedback(async () => {
      await requestNoContent(`/api/groups/${groupID}`, { method: "DELETE" })
      setSelectedGroupIDs((current) => current.filter((id) => id !== groupID))
      if (groupForm.id === groupID) {
        switchGroupToCreateContext()
      }
      if (groupMemberForm.groupID === groupID) {
        setGroupMemberForm((current) => ({ ...current, groupID: "" }))
      }
      await loadOrganisationScopedData()
    }, "group deleted")
  }

  const deleteSelectedGroups = () => {
    if (selectedGroupIDs.length < 2) {
      return
    }
    const groupIDs = [...selectedGroupIDs]
    if (!confirmMultiDelete(groupIDs.length, "groups")) {
      return
    }

    void withFeedback(async () => {
      await deleteSelectedEntries(
        groupIDs,
        (groupID) => requestNoContent(`/api/groups/${groupID}`, { method: "DELETE" }),
        "groups"
      )
      if (groupForm.id && groupIDs.includes(groupForm.id)) {
        switchGroupToCreateContext()
      }
      if (groupMemberForm.groupID && groupIDs.includes(groupMemberForm.groupID)) {
        setGroupMemberForm((current) => ({ ...current, groupID: "" }))
      }
      setSelectedGroupIDs([])
      await loadOrganisationScopedData()
    }, "selected groups deleted")
  }

  const addGroupMember = (event: FormEvent) => {
    event.preventDefault()
    if (!groupMemberForm.groupID || !groupMemberForm.personID) {
      return
    }

    void withFeedback(async () => {
      await requestJSON<Group>(`/api/groups/${groupMemberForm.groupID}/members`, {
        method: "POST",
        body: JSON.stringify({ person_id: groupMemberForm.personID })
      })
      await loadOrganisationScopedData()
    }, "group member added")
  }

  const removeGroupMember = (groupID: string, personID: string) => {
    void withFeedback(async () => {
      await requestJSON<Group>(`/api/groups/${groupID}/members/${personID}`, { method: "DELETE" })
      await loadOrganisationScopedData()
    }, "group member removed")
  }

  const switchAllocationToCreateContext = () => {
    setAllocationForm(newAllocationFormState())
    setAllocationMergeStrategy("stack")
  }

  const switchAllocationToEditContext = (allocation: Allocation) => {
    setAllocationForm({
      id: allocation.id,
      targetType: normalizeAllocationTargetType(allocation),
      targetID: normalizeAllocationTargetID(allocation),
      projectID: allocation.project_id,
      startDate: allocation.start_date || "2026-01-01",
      endDate: allocation.end_date || "2026-12-31",
      loadInputType: "fte_pct",
      loadUnit: "day",
      loadValue: String(allocation.percent)
    })
    setAllocationMergeStrategy("stack")
  }

  const saveAllocation = (event: FormEvent) => {
    event.preventDefault()

    void withFeedback(async () => {
      let serverStateMutated = false
      let deletedAllocationsForRollback: Allocation[] = []
      let deletedEditingGroupAllocationForRollback: Allocation | null = null
      try {
        if (!activeOrganisation) {
          throw new Error("select an organisation first")
        }
        if (allocationFormSelectedPersonIDs.length === 0) {
          throw new Error("select at least one user to allocate")
        }

        if (allocationFormConflicts.length > 0 && allocationMergeStrategy === "replace" && !allocationFormCanReplaceConflicts) {
          throw new Error("replace strategy is only available when conflicts fully match the selected users")
        }

        const allocationPercent = allocationPercentFromInput(
          asNumber(allocationForm.loadValue),
          allocationForm.loadInputType,
          allocationForm.loadUnit,
          activeOrganisation.hours_per_day
        )

        const excludedPersonIDs = allocationMergeStrategy === "keep"
          ? new Set(allocationFormConflictingSelectedPersonIDs)
          : new Set<string>()
        let usersToAllocate = allocationFormSelectedPersonIDs.filter((personID) => !excludedPersonIDs.has(personID))

        if (allocationFormConflicts.length > 0 && allocationMergeStrategy === "keep" && usersToAllocate.length === 0) {
          setErrorMessage("")
          setSuccessMessage("allocation kept as-is")
          return
        }

        const usersToAllocateForProjection = [...usersToAllocate]
        const projectedModifiedPersonIDs = new Set(usersToAllocateForProjection)
        let projectedAllocations = [...allocations]

        if (allocationFormConflicts.length > 0 && allocationMergeStrategy === "replace") {
          const conflictingIDSet = new Set(allocationFormConflicts.map((allocation) => allocation.id))
          projectedAllocations = projectedAllocations.filter((allocation) => !conflictingIDSet.has(allocation.id))
        }

        if (editingAllocation && normalizeAllocationTargetType(editingAllocation) === "group") {
          projectedAllocations = projectedAllocations.filter((allocation) => allocation.id !== editingAllocation.id)
        }

        if (editingAllocation && normalizeAllocationTargetType(editingAllocation) === "person") {
          const editingPersonID = normalizeAllocationTargetID(editingAllocation)
          if (usersToAllocateForProjection.includes(editingPersonID)) {
            projectedAllocations = projectedAllocations.filter((allocation) => allocation.id !== editingAllocation.id)
            projectedAllocations.push({
              ...editingAllocation,
              target_type: "person",
              target_id: editingPersonID,
              project_id: allocationForm.projectID,
              start_date: allocationForm.startDate,
              end_date: allocationForm.endDate,
              percent: allocationPercent
            })
            projectedModifiedPersonIDs.add(editingPersonID)
            const index = usersToAllocateForProjection.indexOf(editingPersonID)
            usersToAllocateForProjection.splice(index, 1)
          }
        }

        projectedAllocations.push(
          ...usersToAllocateForProjection.map((personID, index): Allocation => ({
            id: `pending_${personID}_${index}`,
            organisation_id: activeOrganisation.id,
            target_type: "person",
            target_id: personID,
            project_id: allocationForm.projectID,
            start_date: allocationForm.startDate,
            end_date: allocationForm.endDate,
            percent: allocationPercent
          }))
        )

        const projectedOverallocatedPersonIDSet = new Set(overallocatedPersonIDsForAllocations(projectedAllocations))
        const overallocatedAffectedPersonNames = Array.from(projectedModifiedPersonIDs)
          .filter((personID) => projectedOverallocatedPersonIDSet.has(personID))
          .map((personID) => personNameByID.get(personID) ?? personID)
          .sort((left, right) => left.localeCompare(right))
        if (overallocatedAffectedPersonNames.length > 0) {
          const shouldContinue = window.confirm(
            `Warning: allocation exceeds employment percentage for ${overallocatedAffectedPersonNames.join(", ")}. Continue?`
          )
          if (!shouldContinue) {
            return
          }
        }

        if (allocationFormConflicts.length > 0 && allocationMergeStrategy === "replace") {
          deletedAllocationsForRollback = Array.from(
            new Map(allocationFormConflicts.map((allocation) => [allocation.id, allocation])).values()
          )
          const conflictingIDs = deletedAllocationsForRollback.map((allocation) => allocation.id)
          if (conflictingIDs.length > 0) {
            serverStateMutated = true
          }
          await Promise.all(
            conflictingIDs.map((allocationID) => requestNoContent(`/api/allocations/${allocationID}`, { method: "DELETE" }))
          )
        }

        if (editingAllocation && normalizeAllocationTargetType(editingAllocation) === "group") {
          deletedEditingGroupAllocationForRollback = editingAllocation
          serverStateMutated = true
          await requestNoContent(`/api/allocations/${editingAllocation.id}`, { method: "DELETE" })
        }

        if (editingAllocation && normalizeAllocationTargetType(editingAllocation) === "person") {
          const editingPersonID = normalizeAllocationTargetID(editingAllocation)
          if (usersToAllocate.includes(editingPersonID)) {
            serverStateMutated = true
            await requestJSON<Allocation>(`/api/allocations/${editingAllocation.id}`, {
              method: "PUT",
              body: JSON.stringify({
                target_type: "person",
                target_id: editingPersonID,
                project_id: allocationForm.projectID,
                start_date: allocationForm.startDate,
                end_date: allocationForm.endDate,
                percent: allocationPercent
              })
            })
            usersToAllocate = usersToAllocate.filter((personID) => personID !== editingPersonID)
          }
        }

        const createResults = await Promise.allSettled(
          usersToAllocate.map((personID) => requestJSON<Allocation>("/api/allocations", {
            method: "POST",
            body: JSON.stringify({
              target_type: "person",
              target_id: personID,
              project_id: allocationForm.projectID,
              start_date: allocationForm.startDate,
              end_date: allocationForm.endDate,
              percent: allocationPercent
            })
          }))
        )

        const failedCreates: string[] = []
        createResults.forEach((result, index) => {
          if (result.status === "rejected") {
            failedCreates.push(`${usersToAllocate[index]}: ${toErrorMessage(result.reason)}`)
          }
        })
        const createdCount = createResults.length - failedCreates.length
        if (createdCount > 0) {
          serverStateMutated = true
        }
        if (failedCreates.length > 0) {
          throw new Error(
            `created ${createdCount} of ${createResults.length} allocations. failed ${failedCreates.length}: ${failedCreates.join(", ")}`
          )
        }

        switchAllocationToCreateContext()
        await loadOrganisationScopedData()
      } catch (error) {
        if (serverStateMutated) {
          const rollbackAllocationsByID = new Map<string, Allocation>(
            deletedAllocationsForRollback.map((allocation) => [allocation.id, allocation])
          )
          if (deletedEditingGroupAllocationForRollback) {
            rollbackAllocationsByID.set(deletedEditingGroupAllocationForRollback.id, deletedEditingGroupAllocationForRollback)
          }
          const rollbackAllocations = Array.from(rollbackAllocationsByID.values())

          let rollbackRestoreError = ""
          if (rollbackAllocations.length > 0) {
            const restoreResults = await Promise.allSettled(
              rollbackAllocations.map((allocation) => requestJSON<Allocation>("/api/allocations", {
                method: "POST",
                body: JSON.stringify({
                  target_type: allocation.target_type,
                  target_id: allocation.target_id,
                  project_id: allocation.project_id,
                  start_date: allocation.start_date,
                  end_date: allocation.end_date,
                  percent: allocation.percent
                })
              }))
            )
            const failedRestores = restoreResults.filter((result) => result.status === "rejected").length
            if (failedRestores > 0) {
              rollbackRestoreError = `rollback restore failed for ${failedRestores} allocation(s)`
            }
          }

          try {
            await loadOrganisationScopedData()
          } catch (reloadError) {
            throw new Error(
              `${toErrorMessage(error)}${rollbackRestoreError ? `. ${rollbackRestoreError}` : ""}. refresh failed: ${toErrorMessage(reloadError)}`
            )
          }
          if (rollbackRestoreError) {
            throw new Error(`${toErrorMessage(error)}. ${rollbackRestoreError}`)
          }
        }
        throw error
      }
    }, allocationForm.id ? "allocation updated" : "allocation created")
  }

  const deleteAllocation = (allocation: Allocation) => {
    void withFeedback(async () => {
      await requestNoContent(`/api/allocations/${allocation.id}`, { method: "DELETE" })
      setSelectedAllocationIDs((current) => current.filter((id) => id !== allocation.id))
      if (allocationForm.id === allocation.id) {
        switchAllocationToCreateContext()
      }
      await loadOrganisationScopedData()
    }, "allocation deleted")
  }

  const deleteSelectedAllocations = () => {
    if (selectedAllocationIDs.length < 2) {
      return
    }
    const allocationIDs = [...selectedAllocationIDs]
    if (!confirmMultiDelete(allocationIDs.length, "allocations")) {
      return
    }

    void withFeedback(async () => {
      await deleteSelectedEntries(
        allocationIDs,
        (allocationID) => requestNoContent(`/api/allocations/${allocationID}`, { method: "DELETE" }),
        "allocations"
      )
      if (allocationForm.id && allocationIDs.includes(allocationForm.id)) {
        switchAllocationToCreateContext()
      }
      setSelectedAllocationIDs([])
      await loadOrganisationScopedData()
    }, "selected allocations deleted")
  }

  const createHoliday = (event: FormEvent) => {
    event.preventDefault()
    if (!selectedOrganisationID) {
      return
    }

    void withFeedback(async () => {
      const entries = buildScopedPersonUnavailabilityEntries("organisation")
      await createPersonUnavailabilityEntries(entries)

      if (availabilityUnitScope === "hours") {
        setHolidayForm({ date: "", hours: "8" })
      } else {
        setTimespanForm({ startDate: "", endDate: "", startWeek: "", endWeek: "" })
      }

      await loadOrganisationScopedData()
    }, availabilityUnitScope === "hours" ? "organisation unavailability added" : "organisation unavailability entries added")
  }

  const createPersonUnavailability = (event: FormEvent) => {
    event.preventDefault()
    if (!personUnavailabilityForm.personID) {
      return
    }

    void withFeedback(async () => {
      const entries = buildScopedPersonUnavailabilityEntries("person")
      await createPersonUnavailabilityEntries(entries)

      if (availabilityUnitScope !== "hours") {
        setTimespanForm({ startDate: "", endDate: "", startWeek: "", endWeek: "" })
      }
      setPersonUnavailabilityForm({ personID: "", date: "", hours: "8" })
      await loadOrganisationScopedData()
    }, availabilityUnitScope === "hours" ? "person unavailability added" : "person unavailability entries added")
  }

  const deletePersonUnavailability = (entry: PersonUnavailability) => {
    void withFeedback(async () => {
      await requestNoContent(`/api/persons/${entry.person_id}/unavailability/${entry.id}`, { method: "DELETE" })
      await loadOrganisationScopedData()
    }, "person unavailability deleted")
  }

  const createGroupUnavailability = (event: FormEvent) => {
    event.preventDefault()
    if (!groupUnavailabilityForm.groupID) {
      return
    }

    void withFeedback(async () => {
      const entries = buildScopedPersonUnavailabilityEntries("group")
      await createPersonUnavailabilityEntries(entries)

      if (availabilityUnitScope !== "hours") {
        setTimespanForm({ startDate: "", endDate: "", startWeek: "", endWeek: "" })
      }
      setGroupUnavailabilityForm({ groupID: "", date: "", hours: "8" })
      await loadOrganisationScopedData()
    }, availabilityUnitScope === "hours" ? "group unavailability added" : "group member unavailability entries added")
  }

  const toggleReportID = (id: string) => {
    setReportIDs((current) => {
      if (current.includes(id)) {
        return current.filter((entry) => entry !== id)
      }
      return [...current, id]
    })
  }

  const runReport = (event: FormEvent) => {
    event.preventDefault()

    void withFeedback(async () => {
      const selectableIDSet = new Set(selectableReportItems.map((entry) => entry.id))
      const selectedReportIDs = reportIDs.filter((id) => selectableIDSet.has(id))
      const targetIDs = reportScope === "organisation"
        ? []
        : selectedReportIDs.length > 0
          ? selectedReportIDs
          : selectableReportItems.map((entry) => entry.id)

      if (reportScope !== "organisation" && targetIDs.length > 1) {
        const scopedResults: PromiseSettledResult<ReportObjectResult>[] = []
        for (let index = 0; index < targetIDs.length; index += REPORT_MULTI_OBJECT_CONCURRENCY) {
          const batchTargetIDs = targetIDs.slice(index, index + REPORT_MULTI_OBJECT_CONCURRENCY)
          const batchResults = await Promise.allSettled(
            batchTargetIDs.map(async (id) => {
              const payload = await requestJSON<{ buckets?: ReportBucket[] }>("/api/reports/availability-load", {
                method: "POST",
                body: JSON.stringify({
                  scope: reportScope,
                  ids: [id],
                  from_date: reportFromDate,
                  to_date: reportToDate,
                  granularity: reportGranularity
                })
              })
              return {
                objectID: id,
                objectLabel: reportItemLabelByID.get(id) ?? id,
                buckets: Array.isArray(payload.buckets) ? payload.buckets : []
              }
            })
          )
          scopedResults.push(...batchResults)
        }

        const failedResults: string[] = []
        const resolvedResults: ReportObjectResult[] = []
        scopedResults.forEach((result, index) => {
          if (result.status === "rejected") {
            const failedID = targetIDs[index] ?? "unknown"
            failedResults.push(`${failedID}: ${toErrorMessage(result.reason)}`)
            return
          }
          resolvedResults.push(result.value)
        })
        if (failedResults.length > 0) {
          throw new Error(`report failed for ${failedResults.length} object(s): ${failedResults.join(", ")}`)
        }
        setReportResults(resolvedResults)
        return
      }

      const payload = await requestJSON<{ buckets?: ReportBucket[] }>("/api/reports/availability-load", {
        method: "POST",
        body: JSON.stringify({
          scope: reportScope,
          ids: reportScope === "organisation" ? [] : targetIDs,
          from_date: reportFromDate,
          to_date: reportToDate,
          granularity: reportGranularity
        })
      })
      const objectID = reportScope === "organisation" ? "organisation" : targetIDs[0] ?? "scope"
      const objectLabel = reportScope === "organisation"
        ? "Organisation"
        : reportItemLabelByID.get(objectID) ?? objectID
      setReportResults([{
        objectID,
        objectLabel,
        buckets: Array.isArray(payload.buckets) ? payload.buckets : []
      }])
    }, "report calculated")
  }

  const reportTableRows = useMemo(
    () => buildReportTableRows(reportResults),
    [reportResults]
  )

  const showReportObjectColumn = reportScope !== "organisation" && reportResults.length > 1

  const showAvailabilityColumns = showAvailabilityMetrics(reportScope)
  const showProjectColumns = showProjectMetrics(reportScope)

  return (
    <main>
      <header>
        <h1>Plato MVP</h1>
        <p>Organisations, people, projects, groups, allocations, calendars, and availability load reports.</p>
      </header>

      <section className="panel">
        <h2>Auth and Tenant</h2>
        <div className="row">
          <label>
            Role
            <select value={role} onChange={(event) => setRole(event.target.value as Role)}>
              <option value="org_admin">org_admin</option>
              <option value="org_user">org_user</option>
            </select>
          </label>
          <label>
            Active organisation
            <select value={selectedOrganisationID} onChange={(event) => setSelectedOrganisationID(event.target.value)}>
              <option value="">None</option>
              {organisations.map((organisation) => (
                <option key={organisation.id} value={organisation.id}>
                  {organisation.name}
                </option>
              ))}
            </select>
          </label>
          <button type="button" onClick={() => void withFeedback(loadOrganisations, "organisations refreshed")}>
            Refresh
          </button>
        </div>
      </section>

      <section className="panel">
        <h2>Organisation</h2>
        <form className="grid-form" onSubmit={handleOrganisationCreate}>
          <label>
            Name
            <input value={organisationForm.name} onChange={(event) => setOrganisationForm((current) => ({ ...current, name: event.target.value }))} />
          </label>
          <label>
            Working hours
            <input
              type="number"
              value={organisationForm.workingTimeValue}
              onChange={(event) => setOrganisationForm((current) => ({ ...current, workingTimeValue: event.target.value }))}
            />
          </label>
          <label>
            Working hours unit
            <select
              value={organisationForm.workingTimeUnit}
              onChange={(event) => setOrganisationForm((current) => ({ ...current, workingTimeUnit: event.target.value as WorkingTimeUnit }))}
            >
              <option value="day">daily</option>
              <option value="week">weekly</option>
              <option value="month">monthly</option>
              <option value="year">yearly</option>
            </select>
          </label>
          <div className="actions">
            <button type="submit">Create organisation</button>
            <button type="button" disabled={!selectedOrganisationID} onClick={() => handleOrganisationUpdate()}>
              Update selected organisation
            </button>
            <button type="button" disabled={!selectedOrganisationID} onClick={() => handleOrganisationDelete()}>
              Delete selected organisation
            </button>
          </div>
        </form>
      </section>

      <section className="panel">
        <h2>Persons</h2>
        <form className="grid-form" onSubmit={savePerson}>
          <label>
            Editing person
            <select value={personForm.id} onChange={(event) => {
              const person = persons.find((entry) => entry.id === event.target.value)
              if (!person) {
                switchPersonToCreateContext()
                return
              }
              switchPersonToEditContext(person)
            }}>
              <option value="">New person</option>
              {persons.map((person) => (
                <option
                  key={person.id}
                  value={person.id}
                  className={isOverallocatedPersonID(person.id) ? "person-overallocated" : undefined}
                >
                  {person.name}
                </option>
              ))}
            </select>
          </label>
          <label>
            Name
            <input value={personForm.name} onChange={(event) => setPersonForm((current) => ({ ...current, name: event.target.value }))} />
          </label>
          <label>
            Employment percent
            <input type="number" value={personForm.employmentPct} onChange={(event) => setPersonForm((current) => ({ ...current, employmentPct: event.target.value }))} />
          </label>
          {personForm.id && (
            <label>
              Effective from month
              <input
                type="month"
                value={personForm.employmentEffectiveFromMonth}
                onChange={(event) => {
                  setPersonForm((current) => ({ ...current, employmentEffectiveFromMonth: event.target.value }))
                }}
              />
            </label>
          )}
          <div className="actions">
            <button type="submit">Save person</button>
            {selectedPersonIDs.length > 1 && (
              <button type="button" onClick={deleteSelectedPersons}>Delete selected items</button>
            )}
          </div>
        </form>
        <table>
          <thead>
            <tr>
              <th>
                <input
                  ref={selectAllPersonsCheckboxRef}
                  type="checkbox"
                  aria-label="Select all persons"
                  checked={persons.length > 0 && selectedPersonIDs.length === persons.length}
                  onChange={(event) => {
                    if (event.target.checked) {
                      setSelectedPersonIDs(persons.map((person) => person.id))
                      return
                    }
                    setSelectedPersonIDs([])
                  }}
                />
              </th>
              <th>Name</th>
              <th>Employment %</th>
              <th>Employment changes</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {persons.map((person) => (
              <tr key={person.id} className={isOverallocatedPersonID(person.id) ? "person-overallocated" : undefined}>
                <td>
                  <input
                    type="checkbox"
                    aria-label={`Select person ${person.name}`}
                    checked={selectedPersonIDs.includes(person.id)}
                    onChange={() => {
                      setSelectedPersonIDs((current) => {
                        if (current.includes(person.id)) {
                          return current.filter((id) => id !== person.id)
                        }
                        return [...current, person.id]
                      })
                    }}
                  />
                </td>
                <td>{person.name}</td>
                <td>{person.employment_pct}</td>
                <td>
                  {person.employment_changes && person.employment_changes.length > 0
                    ? person.employment_changes
                      .map((change) => `${change.effective_month}: ${change.employment_pct}%`)
                      .join(", ")
                    : "-"}
                </td>
                <td>
                  <div className="actions">
                    <button type="button" onClick={() => switchPersonToEditContext(person)}>Edit</button>
                    <button type="button" onClick={() => deletePerson(person.id)}>Delete</button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </section>

      <section className="panel">
        <h2>Projects</h2>
        <form className="grid-form" onSubmit={saveProject}>
          <label>
            Editing project
            <select value={projectForm.id} onChange={(event) => {
              const project = projects.find((entry) => entry.id === event.target.value)
              if (!project) {
                switchProjectToCreateContext()
                return
              }
              switchProjectToEditContext(project)
            }}>
              <option value="">New project</option>
              {projects.map((project) => (
                <option key={project.id} value={project.id}>{project.name}</option>
              ))}
            </select>
          </label>
          <label>
            Name
            <input value={projectForm.name} onChange={(event) => setProjectForm((current) => ({ ...current, name: event.target.value }))} />
          </label>
          <label>
            Start date
            <input type="date" value={projectForm.startDate} onChange={(event) => setProjectForm((current) => ({ ...current, startDate: event.target.value }))} />
          </label>
          <label>
            End date
            <input type="date" value={projectForm.endDate} onChange={(event) => setProjectForm((current) => ({ ...current, endDate: event.target.value }))} />
          </label>
          <label>
            Estimated effort hours
            <input
              type="number"
              value={projectForm.estimatedEffortHours}
              onChange={(event) => setProjectForm((current) => ({ ...current, estimatedEffortHours: event.target.value }))}
            />
          </label>
          <div className="actions">
            <button type="submit">Save project</button>
            {selectedProjectIDs.length > 1 && (
              <button type="button" onClick={deleteSelectedProjects}>Delete selected items</button>
            )}
          </div>
        </form>
        <table>
          <thead>
            <tr>
              <th>
                <input
                  ref={selectAllProjectsCheckboxRef}
                  type="checkbox"
                  aria-label="Select all projects"
                  checked={projects.length > 0 && selectedProjectIDs.length === projects.length}
                  onChange={(event) => {
                    if (event.target.checked) {
                      setSelectedProjectIDs(projects.map((project) => project.id))
                      return
                    }
                    setSelectedProjectIDs([])
                  }}
                />
              </th>
              <th>Name</th>
              <th>Start</th>
              <th>End</th>
              <th>Effort hours</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {projects.map((project) => (
              <tr key={project.id}>
                <td>
                  <input
                    type="checkbox"
                    aria-label={`Select project ${project.name}`}
                    checked={selectedProjectIDs.includes(project.id)}
                    onChange={() => {
                      setSelectedProjectIDs((current) => {
                        if (current.includes(project.id)) {
                          return current.filter((id) => id !== project.id)
                        }
                        return [...current, project.id]
                      })
                    }}
                  />
                </td>
                <td>{project.name}</td>
                <td>{project.start_date}</td>
                <td>{project.end_date}</td>
                <td>{project.estimated_effort_hours}</td>
                <td>
                  <div className="actions">
                    <button type="button" onClick={() => switchProjectToEditContext(project)}>Edit</button>
                    <button type="button" onClick={() => deleteProject(project.id)}>Delete</button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </section>

      <section className="panel">
        <h2>Groups</h2>
        <form className="grid-form" onSubmit={saveGroup}>
          <label>
            Editing group
            <select value={groupForm.id} onChange={(event) => {
              const group = groups.find((entry) => entry.id === event.target.value)
              if (!group) {
                switchGroupToCreateContext()
                return
              }
              switchGroupToEditContext(group)
            }}>
              <option value="">New group</option>
              {groups.map((group) => (
                <option key={group.id} value={group.id}>{group.name}</option>
              ))}
            </select>
          </label>
          <label>
            Name
            <input value={groupForm.name} onChange={(event) => setGroupForm((current) => ({ ...current, name: event.target.value }))} />
          </label>
          <fieldset>
            <legend>Members</legend>
            <div className="chips">
              {persons.map((person) => (
                <label key={person.id} className={isOverallocatedPersonID(person.id) ? "person-overallocated" : undefined}>
                  <input
                    type="checkbox"
                    checked={groupForm.memberIDs.includes(person.id)}
                    onChange={() => {
                      setGroupForm((current) => {
                        if (current.memberIDs.includes(person.id)) {
                          return { ...current, memberIDs: current.memberIDs.filter((entry) => entry !== person.id) }
                        }
                        return { ...current, memberIDs: [...current.memberIDs, person.id] }
                      })
                    }}
                  />
                  {person.name}
                </label>
              ))}
            </div>
          </fieldset>
          <div className="actions">
            <button type="submit">Save group</button>
            {selectedGroupIDs.length > 1 && (
              <button type="button" onClick={deleteSelectedGroups}>Delete selected items</button>
            )}
          </div>
        </form>

        <form className="row" onSubmit={addGroupMember}>
          <label>
            Group
            <select value={groupMemberForm.groupID} onChange={(event) => setGroupMemberForm((current) => ({ ...current, groupID: event.target.value }))}>
              <option value="">Select group</option>
              {groups.map((group) => (
                <option key={group.id} value={group.id}>{group.name}</option>
              ))}
            </select>
          </label>
          <label>
            Person
            <select value={groupMemberForm.personID} onChange={(event) => setGroupMemberForm((current) => ({ ...current, personID: event.target.value }))}>
              <option value="">Select person</option>
              {persons.map((person) => (
                <option
                  key={person.id}
                  value={person.id}
                  className={isOverallocatedPersonID(person.id) ? "person-overallocated" : undefined}
                >
                  {person.name}
                </option>
              ))}
            </select>
          </label>
          <button type="submit">Add member</button>
        </form>

        <table>
          <thead>
            <tr>
              <th>
                <input
                  ref={selectAllGroupsCheckboxRef}
                  type="checkbox"
                  aria-label="Select all groups"
                  checked={groups.length > 0 && selectedGroupIDs.length === groups.length}
                  onChange={(event) => {
                    if (event.target.checked) {
                      setSelectedGroupIDs(groups.map((group) => group.id))
                      return
                    }
                    setSelectedGroupIDs([])
                  }}
                />
              </th>
              <th>Name</th>
              <th>Members</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {groups.map((group) => (
              <tr key={group.id}>
                <td>
                  <input
                    type="checkbox"
                    aria-label={`Select group ${group.name}`}
                    checked={selectedGroupIDs.includes(group.id)}
                    onChange={() => {
                      setSelectedGroupIDs((current) => {
                        if (current.includes(group.id)) {
                          return current.filter((id) => id !== group.id)
                        }
                        return [...current, group.id]
                      })
                    }}
                  />
                </td>
                <td>{group.name}</td>
                <td>
                  <details>
                    <summary>{group.member_ids.length} member(s)</summary>
                    {group.member_ids.length === 0 && <p>No members</p>}
                    {group.member_ids.map((memberID) => {
                      const person = persons.find((entry) => entry.id === memberID)
                      return (
                        <div key={memberID} className="member-row">
                          <span className={isOverallocatedPersonID(memberID) ? "person-overallocated" : undefined}>
                            {person?.name ?? memberID}
                          </span>
                          <button type="button" onClick={() => removeGroupMember(group.id, memberID)}>Remove</button>
                        </div>
                      )
                    })}
                  </details>
                </td>
                <td>
                  <div className="actions">
                    <button type="button" onClick={() => switchGroupToEditContext(group)}>Edit</button>
                    <button type="button" onClick={() => deleteGroup(group.id)}>Delete</button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </section>

      <section className="panel">
        <h2>Allocations</h2>
        <form className="grid-form" onSubmit={saveAllocation}>
          <p>{allocationFormContextLabel}</p>
          {editingAllocation && (
            <div className="actions">
              <button type="button" onClick={switchAllocationToCreateContext}>Switch to creation context</button>
            </div>
          )}
          <label>
            Target type
            <select
              value={allocationForm.targetType}
              onChange={(event) => {
                setAllocationForm((current) => ({
                  ...current,
                  targetType: event.target.value as AllocationTargetType,
                  targetID: ""
                }))
              }}
            >
              <option value="person">person</option>
              <option value="group">group</option>
            </select>
          </label>
          <label>
            Target
            <select value={allocationForm.targetID} onChange={(event) => setAllocationForm((current) => ({ ...current, targetID: event.target.value }))}>
              <option value="">Select target</option>
              {allocationTargetOptions.map((target) => (
                <option
                  key={target.id}
                  value={target.id}
                  className={
                    allocationForm.targetType === "person" && isOverallocatedPersonID(target.id)
                      ? "person-overallocated"
                      : undefined
                  }
                >
                  {target.label}
                </option>
              ))}
            </select>
          </label>
          <label>
            Project
            <select value={allocationForm.projectID} onChange={(event) => setAllocationForm((current) => ({ ...current, projectID: event.target.value }))}>
              <option value="">Select project</option>
              {projects.map((project) => (
                <option key={project.id} value={project.id}>{project.name}</option>
              ))}
            </select>
          </label>
          <label>
            Start date
            <input type="date" value={allocationForm.startDate} onChange={(event) => setAllocationForm((current) => ({ ...current, startDate: event.target.value }))} />
          </label>
          <label>
            End date
            <input type="date" value={allocationForm.endDate} onChange={(event) => setAllocationForm((current) => ({ ...current, endDate: event.target.value }))} />
          </label>
          <label>
            Load value type
            <select
              value={allocationForm.loadInputType}
              onChange={(event) => setAllocationForm((current) => ({ ...current, loadInputType: event.target.value as AllocationLoadInputType }))}
            >
              <option value="fte_pct">FTE % (full-time basis)</option>
              <option value="hours">Hours</option>
            </select>
          </label>
          <label>
            Load unit
            <select
              value={allocationForm.loadUnit}
              onChange={(event) => setAllocationForm((current) => ({ ...current, loadUnit: event.target.value as AllocationLoadUnit }))}
            >
              <option value="day">per day</option>
              <option value="week">per week</option>
              <option value="month">per month</option>
            </select>
          </label>
          <label>
            {allocationForm.loadInputType === "fte_pct"
              ? `FTE % per ${allocationForm.loadUnit}`
              : `Hours per ${allocationForm.loadUnit}`}
            <input
              type="number"
              value={allocationForm.loadValue}
              onChange={(event) => setAllocationForm((current) => ({ ...current, loadValue: event.target.value }))}
            />
          </label>
          {allocationFormConflicts.length > 0 && (
            <>
              <p>{allocationFormConflicts.length} allocation conflict(s) detected for the selected users and timespan.</p>
              {allocationFormConflictingSelectedPersonNames.length > 0 && (
                <p>Affected persons: {allocationFormConflictingSelectedPersonNames.join(", ")}</p>
              )}
              {allocationMergeStrategy === "keep" && allocationFormConflictingSelectedPersonIDs.length > 0 && (
                <p>{allocationFormConflictingSelectedPersonIDs.length} selected user(s) will be excluded from this allocation.</p>
              )}
              <label>
                Merge strategy
                <select
                  value={allocationMergeStrategy}
                  onChange={(event) => setAllocationMergeStrategy(event.target.value as AllocationMergeStrategy)}
                >
                  <option value="stack">stack with existing allocations</option>
                  <option value="replace" disabled={!allocationFormCanReplaceConflicts}>replace conflicting allocations</option>
                  <option value="keep">keep existing allocations as-is (exclude affected users)</option>
                </select>
              </label>
              {!allocationFormCanReplaceConflicts && (
                <p>Replace is unavailable because some conflicts include users outside the current selection.</p>
              )}
            </>
          )}
          <div className="actions">
            <button type="submit">Save allocation</button>
            {selectedAllocationIDs.length > 1 && (
              <button type="button" onClick={deleteSelectedAllocations}>Delete selected items</button>
            )}
          </div>
        </form>

        <table>
          <thead>
            <tr>
              <th>
                <input
                  ref={selectAllAllocationsCheckboxRef}
                  type="checkbox"
                  aria-label="Select all allocations"
                  checked={allocations.length > 0 && selectedAllocationIDs.length === allocations.length}
                  onChange={(event) => {
                    if (event.target.checked) {
                      setSelectedAllocationIDs(allocations.map((allocation) => allocation.id))
                      return
                    }
                    setSelectedAllocationIDs([])
                  }}
                />
              </th>
              <th>Target</th>
              <th>Project</th>
              <th>Start</th>
              <th>End</th>
              <th>FTE % (full-time basis)</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {allocations.map((allocation) => {
              const targetType = normalizeAllocationTargetType(allocation)
              const targetID = normalizeAllocationTargetID(allocation)
              const targetLabel = targetType === "group"
                ? groups.find((entry) => entry.id === targetID)?.name ?? targetID
                : persons.find((entry) => entry.id === targetID)?.name ?? targetID
              const hasOverallocatedPerson = resolveAllocationPersonIDs(allocation)
                .some((personID) => isOverallocatedPersonID(personID))
              const project = projects.find((entry) => entry.id === allocation.project_id)
              return (
                <tr key={allocation.id} className={hasOverallocatedPerson ? "person-overallocated" : undefined}>
                  <td>
                    <input
                      type="checkbox"
                      aria-label={`Select allocation ${allocation.id}`}
                      checked={selectedAllocationIDs.includes(allocation.id)}
                      onChange={() => {
                        setSelectedAllocationIDs((current) => {
                          if (current.includes(allocation.id)) {
                            return current.filter((id) => id !== allocation.id)
                          }
                          return [...current, allocation.id]
                        })
                      }}
                    />
                  </td>
                  <td>{targetType}: {targetLabel}</td>
                  <td>{project?.name ?? allocation.project_id}</td>
                  <td>{allocation.start_date || "-"}</td>
                  <td>{allocation.end_date || "-"}</td>
                  <td>{allocation.percent}</td>
                  <td>
                    <div className="actions">
                      <button type="button" onClick={() => switchAllocationToEditContext(allocation)}>Edit</button>
                      <button type="button" onClick={() => deleteAllocation(allocation)}>Delete</button>
                    </div>
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>
      </section>

      <section className="panel">
        <h2>Holidays and Unavailability</h2>

        <h3>Entry Scope</h3>
        <form className="row">
          <label>
            Scope
            <select value={availabilityScope} onChange={(event) => setAvailabilityScope(event.target.value as AvailabilityScope)}>
              <option value="organisation">organisation</option>
              <option value="person">person</option>
              <option value="group">group</option>
            </select>
          </label>
          <label>
            Units
            <select value={availabilityUnitScope} onChange={(event) => setAvailabilityUnitScope(event.target.value as AvailabilityUnitScope)}>
              <option value="hours">hours (single day)</option>
              <option value="days">days (timespan)</option>
              <option value="weeks">weeks (timespan)</option>
            </select>
          </label>
        </form>

        {availabilityScope === "organisation" && (
          <>
            <form className="row" onSubmit={createHoliday}>
              <label>
                Organisation
                <select value={selectedOrganisationID} onChange={(event) => setSelectedOrganisationID(event.target.value)}>
                  <option value="">Select organisation</option>
                  {organisations.map((organisation) => (
                    <option key={organisation.id} value={organisation.id}>{organisation.name}</option>
                  ))}
                </select>
              </label>
              {availabilityUnitScope === "hours" ? (
                <>
                  <label>
                    Date
                    <input type="date" value={holidayForm.date} onChange={(event) => setHolidayForm((current) => ({ ...current, date: event.target.value }))} />
                  </label>
                  <label>
                    Hours
                    <input type="number" value={holidayForm.hours} onChange={(event) => setHolidayForm((current) => ({ ...current, hours: event.target.value }))} />
                  </label>
                </>
              ) : availabilityUnitScope === "days" ? (
                <>
                  <label>
                    Start date
                    <input type="date" value={timespanForm.startDate} onChange={(event) => setTimespanForm((current) => ({ ...current, startDate: event.target.value }))} />
                  </label>
                  <label>
                    End date
                    <input type="date" value={timespanForm.endDate} onChange={(event) => setTimespanForm((current) => ({ ...current, endDate: event.target.value }))} />
                  </label>
                </>
              ) : (
                <>
                  <label>
                    Start week
                    <input type="week" value={timespanForm.startWeek} onChange={(event) => setTimespanForm((current) => ({ ...current, startWeek: event.target.value }))} />
                  </label>
                  <label>
                    End week
                    <input type="week" value={timespanForm.endWeek} onChange={(event) => setTimespanForm((current) => ({ ...current, endWeek: event.target.value }))} />
                  </label>
                </>
              )}
              <button type="submit">{availabilityUnitScope === "hours" ? "Add org unavailability" : "Add org member unavailability"}</button>
            </form>

            <table>
              <thead>
                <tr>
                  <th>Person</th>
                  <th>Date</th>
                  <th>Hours</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {personUnavailability.map((entry) => (
                  <tr key={entry.id}>
                    <td className={isOverallocatedPersonID(entry.person_id) ? "person-overallocated" : undefined}>
                      {persons.find((person) => person.id === entry.person_id)?.name ?? entry.person_id}
                    </td>
                    <td>{entry.date}</td>
                    <td>{entry.hours}</td>
                    <td>
                      <button type="button" onClick={() => deletePersonUnavailability(entry)}>Delete</button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </>
        )}

        {availabilityScope === "person" && (
          <>
            <form className="row" onSubmit={createPersonUnavailability}>
              <label>
                Person
                <select value={personUnavailabilityForm.personID} onChange={(event) => setPersonUnavailabilityForm((current) => ({ ...current, personID: event.target.value }))}>
                  <option value="">Select person</option>
                  {persons.map((person) => (
                    <option
                      key={person.id}
                      value={person.id}
                      className={isOverallocatedPersonID(person.id) ? "person-overallocated" : undefined}
                    >
                      {person.name}
                    </option>
                  ))}
                </select>
              </label>
              {availabilityUnitScope === "hours" ? (
                <>
                  <label>
                    Date
                    <input type="date" value={personUnavailabilityForm.date} onChange={(event) => setPersonUnavailabilityForm((current) => ({ ...current, date: event.target.value }))} />
                  </label>
                  <label>
                    Hours
                    <input type="number" value={personUnavailabilityForm.hours} onChange={(event) => setPersonUnavailabilityForm((current) => ({ ...current, hours: event.target.value }))} />
                  </label>
                </>
              ) : availabilityUnitScope === "days" ? (
                <>
                  <label>
                    Start date
                    <input type="date" value={timespanForm.startDate} onChange={(event) => setTimespanForm((current) => ({ ...current, startDate: event.target.value }))} />
                  </label>
                  <label>
                    End date
                    <input type="date" value={timespanForm.endDate} onChange={(event) => setTimespanForm((current) => ({ ...current, endDate: event.target.value }))} />
                  </label>
                </>
              ) : (
                <>
                  <label>
                    Start week
                    <input type="week" value={timespanForm.startWeek} onChange={(event) => setTimespanForm((current) => ({ ...current, startWeek: event.target.value }))} />
                  </label>
                  <label>
                    End week
                    <input type="week" value={timespanForm.endWeek} onChange={(event) => setTimespanForm((current) => ({ ...current, endWeek: event.target.value }))} />
                  </label>
                </>
              )}
              <button type="submit">{availabilityUnitScope === "hours" ? "Add person unavailability" : "Add person unavailability entries"}</button>
            </form>

            <table>
              <thead>
                <tr>
                  <th>Person</th>
                  <th>Date</th>
                  <th>Hours</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {personUnavailability.map((entry) => {
                  const person = persons.find((value) => value.id === entry.person_id)
                  return (
                    <tr key={entry.id}>
                      <td className={isOverallocatedPersonID(entry.person_id) ? "person-overallocated" : undefined}>
                        {person?.name ?? entry.person_id}
                      </td>
                      <td>{entry.date}</td>
                      <td>{entry.hours}</td>
                      <td>
                        <button type="button" onClick={() => deletePersonUnavailability(entry)}>Delete</button>
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </>
        )}

        {availabilityScope === "group" && (
          <>
            <form className="row" onSubmit={createGroupUnavailability}>
              <label>
                Group
                <select value={groupUnavailabilityForm.groupID} onChange={(event) => setGroupUnavailabilityForm((current) => ({ ...current, groupID: event.target.value }))}>
                  <option value="">Select group</option>
                  {groups.map((group) => (
                    <option key={group.id} value={group.id}>{group.name}</option>
                  ))}
                </select>
              </label>
              {availabilityUnitScope === "hours" ? (
                <>
                  <label>
                    Date
                    <input type="date" value={groupUnavailabilityForm.date} onChange={(event) => setGroupUnavailabilityForm((current) => ({ ...current, date: event.target.value }))} />
                  </label>
                  <label>
                    Hours
                    <input type="number" value={groupUnavailabilityForm.hours} onChange={(event) => setGroupUnavailabilityForm((current) => ({ ...current, hours: event.target.value }))} />
                  </label>
                </>
              ) : availabilityUnitScope === "days" ? (
                <>
                  <label>
                    Start date
                    <input type="date" value={timespanForm.startDate} onChange={(event) => setTimespanForm((current) => ({ ...current, startDate: event.target.value }))} />
                  </label>
                  <label>
                    End date
                    <input type="date" value={timespanForm.endDate} onChange={(event) => setTimespanForm((current) => ({ ...current, endDate: event.target.value }))} />
                  </label>
                </>
              ) : (
                <>
                  <label>
                    Start week
                    <input type="week" value={timespanForm.startWeek} onChange={(event) => setTimespanForm((current) => ({ ...current, startWeek: event.target.value }))} />
                  </label>
                  <label>
                    End week
                    <input type="week" value={timespanForm.endWeek} onChange={(event) => setTimespanForm((current) => ({ ...current, endWeek: event.target.value }))} />
                  </label>
                </>
              )}
              <button type="submit">{availabilityUnitScope === "hours" ? "Add group unavailability" : "Add group unavailability entries"}</button>
            </form>

            <table>
              <thead>
                <tr>
                  <th>Person</th>
                  <th>Date</th>
                  <th>Hours</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {selectedGroupPersonUnavailability.map((entry) => {
                  const person = persons.find((value) => value.id === entry.person_id)
                  return (
                    <tr key={entry.id}>
                      <td className={isOverallocatedPersonID(entry.person_id) ? "person-overallocated" : undefined}>
                        {person?.name ?? entry.person_id}
                      </td>
                      <td>{entry.date}</td>
                      <td>{entry.hours}</td>
                      <td>
                        <button type="button" onClick={() => deletePersonUnavailability(entry)}>Delete</button>
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </>
        )}
      </section>

      <section className="panel">
        <h2>Report</h2>
        <form className="grid-form" onSubmit={runReport}>
          <label>
            Scope
            <select
              value={reportScope}
              onChange={(event) => {
                setReportScope(event.target.value as ReportScope)
                setReportIDs([])
                setReportResults([])
              }}
            >
              <option value="organisation">organisation</option>
              <option value="person">person</option>
              <option value="group">group</option>
              <option value="project">project</option>
            </select>
          </label>

          <label>
            From
            <input type="date" value={reportFromDate} onChange={(event) => setReportFromDate(event.target.value)} />
          </label>

          <label>
            To
            <input type="date" value={reportToDate} onChange={(event) => setReportToDate(event.target.value)} />
          </label>

          <label>
            Granularity
            <select value={reportGranularity} onChange={(event) => setReportGranularity(event.target.value as ReportGranularity)}>
              <option value="day">day</option>
              <option value="week">week</option>
              <option value="month">month</option>
              <option value="year">year</option>
            </select>
          </label>

          {reportScope !== "organisation" && (
            <fieldset>
              <legend>Scope IDs</legend>
              <div className="chips">
                {selectableReportItems.map((entry) => (
                  <label
                    key={entry.id}
                    className={reportScope === "person" && isOverallocatedPersonID(entry.id) ? "person-overallocated" : undefined}
                  >
                    <input type="checkbox" checked={reportIDs.includes(entry.id)} onChange={() => toggleReportID(entry.id)} />
                    {entry.label}
                  </label>
                ))}
              </div>
            </fieldset>
          )}

          <div className="actions">
            <button type="submit">Run report</button>
          </div>
        </form>

        <table>
          <thead>
            <tr>
              <th>Period start</th>
              {showReportObjectColumn && <th>Object</th>}
              {showAvailabilityColumns && <th>Availability hours</th>}
              {showAvailabilityColumns && <th>Load hours</th>}
              {showProjectColumns && <th>Project load hours</th>}
              {showProjectColumns && <th>Project estimation hours</th>}
              {showAvailabilityColumns && <th>Free hours</th>}
              {showAvailabilityColumns && <th>Utilization %</th>}
              {showProjectColumns && <th>Project completion %</th>}
            </tr>
          </thead>
          <tbody>
            {reportTableRows.map((row) => (
              <tr key={row.id} className={row.isTotal ? "report-total-row" : undefined}>
                <td>{row.periodStart}</td>
                {showReportObjectColumn && <td>{row.objectLabel}</td>}
                {showAvailabilityColumns && <td>{formatHours(row.bucket.availability_hours)}</td>}
                {showAvailabilityColumns && <td>{formatHours(row.bucket.load_hours)}</td>}
                {showProjectColumns && <td>{formatHours(row.bucket.project_load_hours)}</td>}
                {showProjectColumns && <td>{formatHours(row.bucket.project_estimation_hours)}</td>}
                {showAvailabilityColumns && <td>{formatHours(row.bucket.free_hours)}</td>}
                {showAvailabilityColumns && <td>{formatHours(row.bucket.utilization_pct)}</td>}
                {showProjectColumns && <td>{formatHours(row.bucket.project_completion_pct)}</td>}
              </tr>
            ))}
          </tbody>
        </table>
      </section>

      {successMessage && <p className="success" aria-live="polite" role="status">{successMessage}</p>}
      {errorMessage && <p className="error" aria-live="assertive" role="alert">{errorMessage}</p>}
    </main>
  )
}
