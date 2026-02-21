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

export type AllocationTargetType = "person" | "group"

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
  objectID: string
  objectLabel: string
  bucket: ReportBucket
  isTotal: boolean
  isDetail: boolean
  detailCount: number
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

export type OrganisationFormState = {
  id: string
  name: string
  workingTimeValue: string
  workingTimeUnit: WorkingTimeUnit
}

export type PersonFormState = {
  id: string
  name: string
  employmentPct: string
  employmentEffectiveFromMonth: string
}

export type ProjectFormState = {
  id: string
  name: string
  startDate: string
  endDate: string
  estimatedEffortHours: string
}

export type GroupFormState = {
  id: string
  name: string
  memberIDs: string[]
}

export type GroupMemberFormState = {
  groupID: string
  personID: string
}

export type HolidayFormState = {
  date: string
  hours: string
}

export type ScopedPersonUnavailabilityFormState = {
  personID: string
  date: string
  hours: string
}

export type ScopedGroupUnavailabilityFormState = {
  groupID: string
  date: string
  hours: string
}

export type TimespanFormState = {
  startDate: string
  endDate: string
  startWeek: string
  endWeek: string
}
