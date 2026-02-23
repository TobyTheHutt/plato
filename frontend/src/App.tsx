import { FormEvent, useCallback, useEffect, useMemo, useRef, useState } from "react"
import { showAvailabilityMetrics, showProjectMetrics, type ReportScope } from "./reportColumns"
import {
  allocationPercentFromInput,
  asNumber,
  buildDayRangeDateHours,
  buildReportTableRows,
  buildWeekRangeDateHours,
  dateRangesOverlap,
  formatHours,
  hasPersonIntersection,
  isPersonOverallocated,
  isReportRowVisible,
  isSubsetOf,
  newAllocationFormState,
  normalizeAllocationTargetID,
  normalizeAllocationTargetType,
  personDailyHours,
  personEmploymentPctOnDate,
  toErrorMessage,
  toWorkingHours
} from "./app/helpers"
import { REPORT_MULTI_OBJECT_CONCURRENCY } from "./app/constants"
import { usePlatoApi } from "./hooks/usePlatoApi"
import type {
  Allocation,
  AllocationFormState,
  AllocationMergeStrategy,
  AllocationTargetType,
  AvailabilityScope,
  AvailabilityUnitScope,
  Group,
  GroupFormState,
  GroupMemberFormState,
  HolidayFormState,
  Organisation,
  OrganisationFormState,
  Person,
  PersonAllocationLoadSegment,
  PersonDateHoursEntry,
  PersonFormState,
  PersonUnavailability,
  Project,
  ProjectFormState,
  ReportGranularity,
  ReportObjectResult,
  Role,
  ScopedGroupUnavailabilityFormState,
  ScopedPersonUnavailabilityFormState,
  TimespanFormState,
  WorkingTimeUnit
} from "./app/types"
import { AllocationsPanel } from "./panels/AllocationsPanel"
import { AuthTenantPanel } from "./panels/AuthTenantPanel"
import { GroupsPanel } from "./panels/GroupsPanel"
import { HolidaysUnavailabilityPanel } from "./panels/HolidaysUnavailabilityPanel"
import { OrganisationPanel } from "./panels/OrganisationPanel"
import { PersonsPanel } from "./panels/PersonsPanel"
import { ProjectsPanel } from "./panels/ProjectsPanel"
import { ReportPanel } from "./panels/ReportPanel"

const BOOTSTRAP_TIMEOUT_MS = 10_000

function isAbortError(error: unknown): boolean {
  if (typeof error !== "object" || error === null || !("name" in error)) {
    return false
  }
  return (error as { name?: unknown }).name === "AbortError"
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

  const [organisationForm, setOrganisationForm] = useState<OrganisationFormState>({
    id: "",
    name: "",
    workingTimeValue: "8",
    workingTimeUnit: "day" as WorkingTimeUnit
  })

  const [personForm, setPersonForm] = useState<PersonFormState>({
    id: "",
    name: "",
    employmentPct: "100",
    employmentEffectiveFromMonth: ""
  })
  const [projectForm, setProjectForm] = useState<ProjectFormState>({
    id: "",
    name: "",
    startDate: "2026-01-01",
    endDate: "2026-12-31",
    estimatedEffortHours: "1000"
  })
  const [groupForm, setGroupForm] = useState<GroupFormState>({ id: "", name: "", memberIDs: [] as string[] })
  const [groupMemberForm, setGroupMemberForm] = useState<GroupMemberFormState>({ groupID: "", personID: "" })

  const [allocationForm, setAllocationForm] = useState<AllocationFormState>(() => newAllocationFormState())
  const [allocationMergeStrategy, setAllocationMergeStrategy] = useState<AllocationMergeStrategy>("stack")

  const [holidayForm, setHolidayForm] = useState<HolidayFormState>({ date: "", hours: "8" })
  const [personUnavailabilityForm, setPersonUnavailabilityForm] = useState<ScopedPersonUnavailabilityFormState>({
    personID: "",
    date: "",
    hours: "8"
  })
  const [groupUnavailabilityForm, setGroupUnavailabilityForm] = useState<ScopedGroupUnavailabilityFormState>({
    groupID: "",
    date: "",
    hours: "8"
  })
  const [availabilityScope, setAvailabilityScope] = useState<AvailabilityScope>("organisation")
  const [availabilityUnitScope, setAvailabilityUnitScope] = useState<AvailabilityUnitScope>("hours")
  const [timespanForm, setTimespanForm] = useState<TimespanFormState>({
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
  const [expandedReportPeriods, setExpandedReportPeriods] = useState<string[]>([])
  const selectAllPersonsCheckboxRef = useRef<HTMLInputElement>(null)
  const selectAllProjectsCheckboxRef = useRef<HTMLInputElement>(null)
  const selectAllGroupsCheckboxRef = useRef<HTMLInputElement>(null)
  const selectAllAllocationsCheckboxRef = useRef<HTMLInputElement>(null)
  const feedbackRequestIDRef = useRef(0)

  const {
    fetchOrganisations,
    fetchOrganisationScopedData,
    createOrganisationRequest,
    updateOrganisationRequest,
    deleteOrganisationRequest,
    createPersonRequest,
    updatePersonRequest,
    deletePersonRequest,
    createProjectRequest,
    updateProjectRequest,
    deleteProjectRequest,
    createGroupRequest,
    updateGroupRequest,
    deleteGroupRequest,
    addGroupMemberRequest,
    removeGroupMemberRequest,
    createAllocationRequest,
    updateAllocationRequest,
    deleteAllocationRequest,
    createPersonUnavailabilityEntriesRequest,
    deletePersonUnavailabilityRequest,
    runAvailabilityLoadReportRequest
  } = usePlatoApi({ role, selectedOrganisationID, canUseNetwork })

  const withFeedback = useCallback(async (operation: () => Promise<void>, success: string) => {
    const tracksFeedback = success !== ""
    const requestID = tracksFeedback ? feedbackRequestIDRef.current + 1 : feedbackRequestIDRef.current
    if (tracksFeedback) {
      feedbackRequestIDRef.current = requestID
    }
    setErrorMessage("")
    if (tracksFeedback) {
      setSuccessMessage("")
    }

    try {
      await operation()
      if (tracksFeedback && feedbackRequestIDRef.current === requestID) {
        setSuccessMessage(success)
      }
    } catch (error) {
      if (isAbortError(error)) {
        return
      }
      if (!tracksFeedback || feedbackRequestIDRef.current === requestID) {
        setErrorMessage(toErrorMessage(error))
      }
    }
  }, [])

  const loadOrganisations = useCallback(async (signal?: AbortSignal) => {
    const loaded = await fetchOrganisations({ signal })
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
  }, [fetchOrganisations])

  const loadOrganisationScopedData = useCallback(async (signal?: AbortSignal) => {
    const scopedData = await fetchOrganisationScopedData(selectedOrganisationID, { signal })
    setPersons(scopedData.persons)
    setProjects(scopedData.projects)
    setGroups(scopedData.groups)
    setAllocations(scopedData.allocations)
    setPersonUnavailability(scopedData.personUnavailability)
  }, [fetchOrganisationScopedData, selectedOrganisationID])

  const runBootstrapLoad = useCallback(async (
    signal: AbortSignal,
    timedOut: () => boolean,
    operation: (abortSignal: AbortSignal) => Promise<void>
  ) => {
    try {
      await operation(signal)
    } catch (error) {
      if (timedOut() && isAbortError(error)) {
        throw new Error(`bootstrap request timed out after ${BOOTSTRAP_TIMEOUT_MS / 1000} seconds`)
      }
      throw error
    }
  }, [])

  useEffect(() => {
    if (!canUseNetwork) {
      return
    }

    const controller = new AbortController()
    let timedOut = false
    const timeoutHandle = window.setTimeout(() => {
      timedOut = true
      controller.abort()
    }, BOOTSTRAP_TIMEOUT_MS)

    void withFeedback(async () => {
      try {
        await runBootstrapLoad(controller.signal, () => timedOut, loadOrganisations)
      } finally {
        window.clearTimeout(timeoutHandle)
      }
    }, "")

    return () => {
      window.clearTimeout(timeoutHandle)
      controller.abort()
    }
  }, [canUseNetwork, loadOrganisations, runBootstrapLoad, withFeedback])

  useEffect(() => {
    if (!canUseNetwork) {
      return
    }

    const controller = new AbortController()
    let timedOut = false
    const timeoutHandle = window.setTimeout(() => {
      timedOut = true
      controller.abort()
    }, BOOTSTRAP_TIMEOUT_MS)

    void withFeedback(async () => {
      try {
        await runBootstrapLoad(controller.signal, () => timedOut, loadOrganisationScopedData)
      } finally {
        window.clearTimeout(timeoutHandle)
      }
    }, "")

    return () => {
      window.clearTimeout(timeoutHandle)
      controller.abort()
    }
  }, [canUseNetwork, loadOrganisationScopedData, runBootstrapLoad, withFeedback])

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

  const personByID = useMemo(
    () => new Map(persons.map((person) => [person.id, person])),
    [persons]
  )

  const editingPerson = useMemo(
    () => persons.find((person) => person.id === personForm.id),
    [personForm.id, persons]
  )

  const personFormContextLabel = useMemo(() => {
    if (!editingPerson) {
      return "Creation context: creating a new person."
    }
    return `Edit context: person: ${editingPerson.name}`
  }, [editingPerson])

  const editingProject = useMemo(
    () => projects.find((project) => project.id === projectForm.id),
    [projectForm.id, projects]
  )

  const projectFormContextLabel = useMemo(() => {
    if (!editingProject) {
      return "Creation context: creating a new project."
    }
    return `Edit context: project: ${editingProject.name}`
  }, [editingProject])

  const editingGroup = useMemo(
    () => groups.find((group) => group.id === groupForm.id),
    [groupForm.id, groups]
  )

  const groupFormContextLabel = useMemo(() => {
    if (!editingGroup) {
      return "Creation context: creating a new group."
    }
    return `Edit context: group: ${editingGroup.name}`
  }, [editingGroup])

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

  const handleOrganisationCreate = (event: FormEvent) => {
    event.preventDefault()

    void withFeedback(async () => {
      const workingHours = toWorkingHours(asNumber(organisationForm.workingTimeValue), organisationForm.workingTimeUnit)

      await createOrganisationRequest({
        name: organisationForm.name,
        hours_per_day: workingHours.day,
        hours_per_week: workingHours.week,
        hours_per_year: workingHours.year
      })
      await loadOrganisations()
    }, "organisation created")
  }

  const handleOrganisationUpdate = () => {
    if (!selectedOrganisationID) {
      return
    }

    void withFeedback(async () => {
      const workingHours = toWorkingHours(asNumber(organisationForm.workingTimeValue), organisationForm.workingTimeUnit)

      await updateOrganisationRequest(selectedOrganisationID, {
        name: organisationForm.name,
        hours_per_day: workingHours.day,
        hours_per_week: workingHours.week,
        hours_per_year: workingHours.year
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
      await deleteOrganisationRequest(selectedOrganisationID)
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
        const payload: {
          name: string
          employment_pct: number
          employment_effective_from_month?: string
        } = {
          name: personForm.name,
          employment_pct: asNumber(personForm.employmentPct)
        }
        if (personForm.employmentEffectiveFromMonth) {
          payload.employment_effective_from_month = personForm.employmentEffectiveFromMonth
        }

        await updatePersonRequest(personForm.id, payload)
      } else {
        await createPersonRequest({ name: personForm.name, employment_pct: asNumber(personForm.employmentPct) })
      }
      switchPersonToCreateContext()
      await loadOrganisationScopedData()
    }, personForm.id ? "person updated" : "person created")
  }

  const deletePerson = (personID: string) => {
    void withFeedback(async () => {
      await deletePersonRequest(personID)
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
        (personID) => deletePersonRequest(personID),
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
        await updateProjectRequest(projectForm.id, {
          name: projectForm.name,
          start_date: projectForm.startDate,
          end_date: projectForm.endDate,
          estimated_effort_hours: asNumber(projectForm.estimatedEffortHours)
        })
      } else {
        await createProjectRequest({
          name: projectForm.name,
          start_date: projectForm.startDate,
          end_date: projectForm.endDate,
          estimated_effort_hours: asNumber(projectForm.estimatedEffortHours)
        })
      }
      switchProjectToCreateContext()
      await loadOrganisationScopedData()
    }, projectForm.id ? "project updated" : "project created")
  }

  const deleteProject = (projectID: string) => {
    void withFeedback(async () => {
      await deleteProjectRequest(projectID)
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
        (projectID) => deleteProjectRequest(projectID),
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
        await updateGroupRequest(groupForm.id, { name: groupForm.name, member_ids: groupForm.memberIDs })
      } else {
        await createGroupRequest({ name: groupForm.name, member_ids: groupForm.memberIDs })
      }
      switchGroupToCreateContext()
      await loadOrganisationScopedData()
    }, groupForm.id ? "group updated" : "group created")
  }

  const deleteGroup = (groupID: string) => {
    void withFeedback(async () => {
      await deleteGroupRequest(groupID)
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
        (groupID) => deleteGroupRequest(groupID),
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
      await addGroupMemberRequest(groupMemberForm.groupID, groupMemberForm.personID)
      await loadOrganisationScopedData()
    }, "group member added")
  }

  const removeGroupMember = (groupID: string, personID: string) => {
    void withFeedback(async () => {
      await removeGroupMemberRequest(groupID, personID)
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
            conflictingIDs.map((allocationID) => deleteAllocationRequest(allocationID))
          )
        }

        if (editingAllocation && normalizeAllocationTargetType(editingAllocation) === "group") {
          deletedEditingGroupAllocationForRollback = editingAllocation
          serverStateMutated = true
          await deleteAllocationRequest(editingAllocation.id)
        }

        if (editingAllocation && normalizeAllocationTargetType(editingAllocation) === "person") {
          const editingPersonID = normalizeAllocationTargetID(editingAllocation)
          if (usersToAllocate.includes(editingPersonID)) {
            serverStateMutated = true
            await updateAllocationRequest(editingAllocation.id, {
              target_type: "person",
              target_id: editingPersonID,
              project_id: allocationForm.projectID,
              start_date: allocationForm.startDate,
              end_date: allocationForm.endDate,
              percent: allocationPercent
            })
            usersToAllocate = usersToAllocate.filter((personID) => personID !== editingPersonID)
          }
        }

        const createResults: PromiseSettledResult<Allocation>[] = []
        for (let index = 0; index < usersToAllocate.length; index += REPORT_MULTI_OBJECT_CONCURRENCY) {
          const batchPersonIDs = usersToAllocate.slice(index, index + REPORT_MULTI_OBJECT_CONCURRENCY)
          const batchResults = await Promise.allSettled(
            batchPersonIDs.map((personID) => createAllocationRequest({
              target_type: "person",
              target_id: personID,
              project_id: allocationForm.projectID,
              start_date: allocationForm.startDate,
              end_date: allocationForm.endDate,
              percent: allocationPercent
            }))
          )
          createResults.push(...batchResults)
        }

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
              rollbackAllocations.map((allocation) => createAllocationRequest({
                target_type: allocation.target_type,
                target_id: allocation.target_id,
                project_id: allocation.project_id,
                start_date: allocation.start_date,
                end_date: allocation.end_date,
                percent: allocation.percent
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
      await deleteAllocationRequest(allocation.id)
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
        (allocationID) => deleteAllocationRequest(allocationID),
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
      await createPersonUnavailabilityEntriesRequest(entries)

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
      await createPersonUnavailabilityEntriesRequest(entries)

      if (availabilityUnitScope !== "hours") {
        setTimespanForm({ startDate: "", endDate: "", startWeek: "", endWeek: "" })
      }
      setPersonUnavailabilityForm({ personID: "", date: "", hours: "8" })
      await loadOrganisationScopedData()
    }, availabilityUnitScope === "hours" ? "person unavailability added" : "person unavailability entries added")
  }

  const deletePersonUnavailability = (entry: PersonUnavailability) => {
    void withFeedback(async () => {
      await deletePersonUnavailabilityRequest(entry.person_id, entry.id)
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
      await createPersonUnavailabilityEntriesRequest(entries)

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
              const payload = await runAvailabilityLoadReportRequest({
                scope: reportScope,
                ids: [id],
                from_date: reportFromDate,
                to_date: reportToDate,
                granularity: reportGranularity
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

      const payload = await runAvailabilityLoadReportRequest({
        scope: reportScope,
        ids: reportScope === "organisation" ? [] : targetIDs,
        from_date: reportFromDate,
        to_date: reportToDate,
        granularity: reportGranularity
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

  useEffect(() => {
    setExpandedReportPeriods([])
  }, [reportTableRows])

  const toggleReportPeriodDetails = (periodStart: string) => {
    setExpandedReportPeriods((current) => {
      if (current.includes(periodStart)) {
        return current.filter((entry) => entry !== periodStart)
      }
      return [...current, periodStart]
    })
  }

  const expandedReportPeriodSet = useMemo(
    () => new Set(expandedReportPeriods),
    [expandedReportPeriods]
  )

  const visibleReportRows = useMemo(
    () => reportTableRows.filter((row) => isReportRowVisible(row, expandedReportPeriodSet)),
    [expandedReportPeriodSet, reportTableRows]
  )

  const showReportObjectColumn = reportScope !== "organisation"

  const showAvailabilityColumns = showAvailabilityMetrics(reportScope)
  const showProjectColumns = showProjectMetrics(reportScope)

  return (
    <main>
      <header>
        <h1>Plato MVP</h1>
        <p>Organisations, people, projects, groups, allocations, calendars, and availability load reports.</p>
      </header>

      <AuthTenantPanel
        role={role}
        setRole={setRole}
        selectedOrganisationID={selectedOrganisationID}
        setSelectedOrganisationID={setSelectedOrganisationID}
        organisations={organisations}
        onRefresh={() => void withFeedback(loadOrganisations, "organisations refreshed")}
      />

      <OrganisationPanel
        organisationForm={organisationForm}
        setOrganisationForm={setOrganisationForm}
        selectedOrganisationID={selectedOrganisationID}
        onCreate={handleOrganisationCreate}
        onUpdate={handleOrganisationUpdate}
        onDelete={handleOrganisationDelete}
      />

      <PersonsPanel
        personForm={personForm}
        setPersonForm={setPersonForm}
        personFormContextLabel={personFormContextLabel}
        editingPerson={editingPerson}
        onSwitchToCreateContext={switchPersonToCreateContext}
        onSavePerson={savePerson}
        selectedPersonIDs={selectedPersonIDs}
        onDeleteSelectedPersons={deleteSelectedPersons}
        selectAllPersonsCheckboxRef={selectAllPersonsCheckboxRef}
        persons={persons}
        setSelectedPersonIDs={setSelectedPersonIDs}
        isOverallocatedPersonID={isOverallocatedPersonID}
        onSwitchToEditContext={switchPersonToEditContext}
        onDeletePerson={deletePerson}
      />

      <ProjectsPanel
        projectForm={projectForm}
        setProjectForm={setProjectForm}
        projectFormContextLabel={projectFormContextLabel}
        editingProject={editingProject}
        onSwitchToCreateContext={switchProjectToCreateContext}
        onSaveProject={saveProject}
        selectedProjectIDs={selectedProjectIDs}
        onDeleteSelectedProjects={deleteSelectedProjects}
        selectAllProjectsCheckboxRef={selectAllProjectsCheckboxRef}
        projects={projects}
        setSelectedProjectIDs={setSelectedProjectIDs}
        onSwitchToEditContext={switchProjectToEditContext}
        onDeleteProject={deleteProject}
      />

      <GroupsPanel
        groupForm={groupForm}
        setGroupForm={setGroupForm}
        groupFormContextLabel={groupFormContextLabel}
        editingGroup={editingGroup}
        onSwitchToCreateContext={switchGroupToCreateContext}
        onSaveGroup={saveGroup}
        selectedGroupIDs={selectedGroupIDs}
        onDeleteSelectedGroups={deleteSelectedGroups}
        persons={persons}
        isOverallocatedPersonID={isOverallocatedPersonID}
        groupMemberForm={groupMemberForm}
        setGroupMemberForm={setGroupMemberForm}
        groups={groups}
        onAddGroupMember={addGroupMember}
        selectAllGroupsCheckboxRef={selectAllGroupsCheckboxRef}
        setSelectedGroupIDs={setSelectedGroupIDs}
        onRemoveGroupMember={removeGroupMember}
        onSwitchToEditContext={switchGroupToEditContext}
        onDeleteGroup={deleteGroup}
      />

      <AllocationsPanel
        allocationForm={allocationForm}
        setAllocationForm={setAllocationForm}
        allocationFormContextLabel={allocationFormContextLabel}
        editingAllocation={editingAllocation}
        onSwitchToCreateContext={switchAllocationToCreateContext}
        onSaveAllocation={saveAllocation}
        allocationTargetOptions={allocationTargetOptions}
        isOverallocatedPersonID={isOverallocatedPersonID}
        projects={projects}
        allocationFormConflicts={allocationFormConflicts}
        allocationFormConflictingSelectedPersonNames={allocationFormConflictingSelectedPersonNames}
        allocationFormConflictingSelectedPersonIDs={allocationFormConflictingSelectedPersonIDs}
        allocationMergeStrategy={allocationMergeStrategy}
        setAllocationMergeStrategy={setAllocationMergeStrategy}
        allocationFormCanReplaceConflicts={allocationFormCanReplaceConflicts}
        selectedAllocationIDs={selectedAllocationIDs}
        onDeleteSelectedAllocations={deleteSelectedAllocations}
        selectAllAllocationsCheckboxRef={selectAllAllocationsCheckboxRef}
        allocations={allocations}
        setSelectedAllocationIDs={setSelectedAllocationIDs}
        groups={groups}
        persons={persons}
        resolveAllocationPersonIDs={resolveAllocationPersonIDs}
        onSwitchToEditContext={switchAllocationToEditContext}
        onDeleteAllocation={deleteAllocation}
      />

      <HolidaysUnavailabilityPanel
        availabilityScope={availabilityScope}
        setAvailabilityScope={setAvailabilityScope}
        availabilityUnitScope={availabilityUnitScope}
        setAvailabilityUnitScope={setAvailabilityUnitScope}
        onCreateHoliday={createHoliday}
        onCreatePersonUnavailability={createPersonUnavailability}
        onCreateGroupUnavailability={createGroupUnavailability}
        selectedOrganisationID={selectedOrganisationID}
        setSelectedOrganisationID={setSelectedOrganisationID}
        organisations={organisations}
        holidayForm={holidayForm}
        setHolidayForm={setHolidayForm}
        timespanForm={timespanForm}
        setTimespanForm={setTimespanForm}
        personUnavailability={personUnavailability}
        persons={persons}
        isOverallocatedPersonID={isOverallocatedPersonID}
        onDeletePersonUnavailability={deletePersonUnavailability}
        personUnavailabilityForm={personUnavailabilityForm}
        setPersonUnavailabilityForm={setPersonUnavailabilityForm}
        groupUnavailabilityForm={groupUnavailabilityForm}
        setGroupUnavailabilityForm={setGroupUnavailabilityForm}
        groups={groups}
        selectedGroupPersonUnavailability={selectedGroupPersonUnavailability}
      />

      <ReportPanel
        reportScope={reportScope}
        setReportScope={setReportScope}
        setReportIDs={setReportIDs}
        setReportResults={setReportResults}
        onRunReport={runReport}
        reportFromDate={reportFromDate}
        setReportFromDate={setReportFromDate}
        reportToDate={reportToDate}
        setReportToDate={setReportToDate}
        reportGranularity={reportGranularity}
        setReportGranularity={setReportGranularity}
        selectableReportItems={selectableReportItems}
        isOverallocatedPersonID={isOverallocatedPersonID}
        reportIDs={reportIDs}
        onToggleReportID={toggleReportID}
        showReportObjectColumn={showReportObjectColumn}
        showAvailabilityColumns={showAvailabilityColumns}
        showProjectColumns={showProjectColumns}
        visibleReportRows={visibleReportRows}
        expandedReportPeriodSet={expandedReportPeriodSet}
        personByID={personByID}
        onToggleReportPeriodDetails={toggleReportPeriodDetails}
      />

      {successMessage && <p className="success" aria-live="polite" role="status">{successMessage}</p>}
      {errorMessage && <p className="error" aria-live="assertive" role="alert">{errorMessage}</p>}
    </main>
  )
}
