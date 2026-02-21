import { useCallback } from "react"
import { PERSON_UNAVAILABILITY_LOAD_CONCURRENCY } from "../app/constants"
import { toErrorMessage } from "../app/helpers"
import type {
  Allocation,
  Group,
  Organisation,
  Person,
  PersonDateHoursEntry,
  PersonUnavailability,
  Project,
  ReportBucket,
  ReportGranularity,
  Role
} from "../app/types"
import type { ReportScope } from "../reportColumns"
import { useApiClient } from "./useApiClient"

type UsePlatoApiOptions = {
  role: Role
  selectedOrganisationID: string
  canUseNetwork: boolean
}

type OrganisationPayload = {
  name: string
  hours_per_day: number
  hours_per_week: number
  hours_per_year: number
}

type PersonPayload = {
  name: string
  employment_pct: number
  employment_effective_from_month?: string
}

type ProjectPayload = {
  name: string
  start_date: string
  end_date: string
  estimated_effort_hours: number
}

type GroupPayload = {
  name: string
  member_ids: string[]
}

type AllocationPayload = {
  target_type: "person" | "group"
  target_id: string
  project_id: string
  start_date: string
  end_date: string
  percent: number
}

type AvailabilityLoadReportPayload = {
  scope: ReportScope
  ids: string[]
  from_date: string
  to_date: string
  granularity: ReportGranularity
}

type OrganisationScopedData = {
  persons: Person[]
  projects: Project[]
  groups: Group[]
  allocations: Allocation[]
  personUnavailability: PersonUnavailability[]
}

export function usePlatoApi(options: UsePlatoApiOptions) {
  const { requestJSON, requestNoContent } = useApiClient(options)

  const fetchOrganisations = useCallback(async (): Promise<Organisation[]> => {
    return requestJSON<Organisation[]>("/api/organisations", { method: "GET" }, "", [])
  }, [requestJSON])

  const fetchOrganisationScopedData = useCallback(async (organisationID: string): Promise<OrganisationScopedData> => {
    if (!organisationID) {
      return {
        persons: [],
        projects: [],
        groups: [],
        allocations: [],
        personUnavailability: []
      }
    }

    const [persons, projects, groups, allocations] = await Promise.all([
      requestJSON<Person[]>("/api/persons", { method: "GET" }, undefined, []),
      requestJSON<Project[]>("/api/projects", { method: "GET" }, undefined, []),
      requestJSON<Group[]>("/api/groups", { method: "GET" }, undefined, []),
      requestJSON<Allocation[]>("/api/allocations", { method: "GET" }, undefined, [])
    ])

    const personUnavailability: PersonUnavailability[] = []
    const personLoadErrors: string[] = []

    for (let index = 0; index < persons.length; index += PERSON_UNAVAILABILITY_LOAD_CONCURRENCY) {
      const personBatch = persons.slice(index, index + PERSON_UNAVAILABILITY_LOAD_CONCURRENCY)
      const settledBatch = await Promise.allSettled(
        personBatch.map((person) =>
          requestJSON<PersonUnavailability[]>(
            `/api/persons/${person.id}/unavailability`,
            { method: "GET" },
            undefined,
            []
          )
        )
      )

      settledBatch.forEach((result, resultIndex) => {
        if (result.status === "fulfilled") {
          personUnavailability.push(...result.value)
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

    return {
      persons,
      projects,
      groups,
      allocations,
      personUnavailability
    }
  }, [requestJSON])

  const createOrganisationRequest = useCallback(async (payload: OrganisationPayload): Promise<Organisation> => {
    return requestJSON<Organisation>(
      "/api/organisations",
      {
        method: "POST",
        body: JSON.stringify(payload)
      },
      ""
    )
  }, [requestJSON])

  const updateOrganisationRequest = useCallback(async (organisationID: string, payload: OrganisationPayload): Promise<Organisation> => {
    return requestJSON<Organisation>(`/api/organisations/${organisationID}`, {
      method: "PUT",
      body: JSON.stringify(payload)
    })
  }, [requestJSON])

  const deleteOrganisationRequest = useCallback(async (organisationID: string): Promise<void> => {
    return requestNoContent(`/api/organisations/${organisationID}`, { method: "DELETE" })
  }, [requestNoContent])

  const createPersonRequest = useCallback(async (payload: PersonPayload): Promise<Person> => {
    return requestJSON<Person>("/api/persons", {
      method: "POST",
      body: JSON.stringify(payload)
    })
  }, [requestJSON])

  const updatePersonRequest = useCallback(async (personID: string, payload: PersonPayload): Promise<Person> => {
    return requestJSON<Person>(`/api/persons/${personID}`, {
      method: "PUT",
      body: JSON.stringify(payload)
    })
  }, [requestJSON])

  const deletePersonRequest = useCallback(async (personID: string): Promise<void> => {
    return requestNoContent(`/api/persons/${personID}`, { method: "DELETE" })
  }, [requestNoContent])

  const createProjectRequest = useCallback(async (payload: ProjectPayload): Promise<Project> => {
    return requestJSON<Project>("/api/projects", {
      method: "POST",
      body: JSON.stringify(payload)
    })
  }, [requestJSON])

  const updateProjectRequest = useCallback(async (projectID: string, payload: ProjectPayload): Promise<Project> => {
    return requestJSON<Project>(`/api/projects/${projectID}`, {
      method: "PUT",
      body: JSON.stringify(payload)
    })
  }, [requestJSON])

  const deleteProjectRequest = useCallback(async (projectID: string): Promise<void> => {
    return requestNoContent(`/api/projects/${projectID}`, { method: "DELETE" })
  }, [requestNoContent])

  const createGroupRequest = useCallback(async (payload: GroupPayload): Promise<Group> => {
    return requestJSON<Group>("/api/groups", {
      method: "POST",
      body: JSON.stringify(payload)
    })
  }, [requestJSON])

  const updateGroupRequest = useCallback(async (groupID: string, payload: GroupPayload): Promise<Group> => {
    return requestJSON<Group>(`/api/groups/${groupID}`, {
      method: "PUT",
      body: JSON.stringify(payload)
    })
  }, [requestJSON])

  const deleteGroupRequest = useCallback(async (groupID: string): Promise<void> => {
    return requestNoContent(`/api/groups/${groupID}`, { method: "DELETE" })
  }, [requestNoContent])

  const addGroupMemberRequest = useCallback(async (groupID: string, personID: string): Promise<Group> => {
    return requestJSON<Group>(`/api/groups/${groupID}/members`, {
      method: "POST",
      body: JSON.stringify({ person_id: personID })
    })
  }, [requestJSON])

  const removeGroupMemberRequest = useCallback(async (groupID: string, personID: string): Promise<Group> => {
    return requestJSON<Group>(`/api/groups/${groupID}/members/${personID}`, { method: "DELETE" })
  }, [requestJSON])

  const createAllocationRequest = useCallback(async (payload: AllocationPayload): Promise<Allocation> => {
    return requestJSON<Allocation>("/api/allocations", {
      method: "POST",
      body: JSON.stringify(payload)
    })
  }, [requestJSON])

  const updateAllocationRequest = useCallback(async (allocationID: string, payload: AllocationPayload): Promise<Allocation> => {
    return requestJSON<Allocation>(`/api/allocations/${allocationID}`, {
      method: "PUT",
      body: JSON.stringify(payload)
    })
  }, [requestJSON])

  const deleteAllocationRequest = useCallback(async (allocationID: string): Promise<void> => {
    return requestNoContent(`/api/allocations/${allocationID}`, { method: "DELETE" })
  }, [requestNoContent])

  const createPersonUnavailabilityRequest = useCallback(async (
    personID: string,
    payload: { date: string, hours: number }
  ): Promise<PersonUnavailability> => {
    return requestJSON<PersonUnavailability>(`/api/persons/${personID}/unavailability`, {
      method: "POST",
      body: JSON.stringify(payload)
    })
  }, [requestJSON])

  const createPersonUnavailabilityEntriesRequest = useCallback(async (entries: PersonDateHoursEntry[]) => {
    const failedCreates: string[] = []
    let attemptedCreates = 0
    for (let index = 0; index < entries.length; index += PERSON_UNAVAILABILITY_LOAD_CONCURRENCY) {
      const batchEntries = entries.slice(index, index + PERSON_UNAVAILABILITY_LOAD_CONCURRENCY)
      attemptedCreates += batchEntries.length
      const batchResults = await Promise.allSettled(
        batchEntries.map((entry) => createPersonUnavailabilityRequest(entry.personID, { date: entry.date, hours: entry.hours }))
      )

      batchResults.forEach((result, resultIndex) => {
        if (result.status === "rejected") {
          const entry = batchEntries[resultIndex]
          failedCreates.push(`${entry.personID} on ${entry.date}: ${toErrorMessage(result.reason)}`)
        }
      })
    }

    if (failedCreates.length > 0) {
      const createdCount = attemptedCreates - failedCreates.length
      throw new Error(
        `created ${createdCount} of ${attemptedCreates} unavailability entries. failed ${failedCreates.length}: ${failedCreates.join(", ")}`
      )
    }
  }, [createPersonUnavailabilityRequest])

  const deletePersonUnavailabilityRequest = useCallback(async (personID: string, entryID: string): Promise<void> => {
    return requestNoContent(`/api/persons/${personID}/unavailability/${entryID}`, { method: "DELETE" })
  }, [requestNoContent])

  const runAvailabilityLoadReportRequest = useCallback(async (
    payload: AvailabilityLoadReportPayload
  ): Promise<{ buckets?: ReportBucket[] }> => {
    return requestJSON<{ buckets?: ReportBucket[] }>("/api/reports/availability-load", {
      method: "POST",
      body: JSON.stringify(payload)
    })
  }, [requestJSON])

  return {
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
  }
}
