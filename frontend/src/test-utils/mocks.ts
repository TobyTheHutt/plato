import { vi } from "vitest"
import type {
  Allocation,
  Group,
  Organisation,
  Person,
  PersonUnavailability,
  Project,
  ReportBucket
} from "../App"

export type MockStore = {
  organisations: Organisation[]
  persons: Person[]
  projects: Project[]
  groups: Group[]
  allocations: Allocation[]
  personUnavailability: PersonUnavailability[]
  reportBuckets: ReportBucket[]
}

export type MockSetup = {
  fetchMock: ReturnType<typeof vi.fn>
  store: MockStore
  restore: () => void
}

export function jsonResponse(payload: unknown, status = 200) {
  return {
    ok: status >= 200 && status < 300,
    status,
    text: async () => JSON.stringify(payload),
    json: async () => payload
  }
}

export function textResponse(text: string, status: number) {
  return {
    ok: status >= 200 && status < 300,
    status,
    text: async () => text,
    json: async () => {
      try {
        return JSON.parse(text)
      } catch (error) {
        return Promise.reject(error)
      }
    }
  }
}

function cloneStore(store: MockStore): MockStore {
  return JSON.parse(JSON.stringify(store)) as MockStore
}

export function buildMockStore(): MockStore {
  return {
    organisations: [
      { id: "org_1", name: "Alpha Org", hours_per_day: 8, hours_per_week: 40, hours_per_year: 2080 }
    ],
    persons: [
      { id: "person_1", organisation_id: "org_1", name: "Alice", employment_pct: 100 },
      { id: "person_2", organisation_id: "org_1", name: "Bob", employment_pct: 80 }
    ],
    projects: [
      {
        id: "project_1",
        organisation_id: "org_1",
        name: "Apollo",
        start_date: "2026-01-01",
        end_date: "2026-12-31",
        estimated_effort_hours: 1000
      }
    ],
    groups: [
      { id: "group_1", organisation_id: "org_1", name: "Team", member_ids: ["person_1"] }
    ],
    allocations: [
      {
        id: "allocation_1",
        organisation_id: "org_1",
        target_type: "person",
        target_id: "person_1",
        project_id: "project_1",
        start_date: "2026-01-01",
        end_date: "2026-12-31",
        percent: 20
      }
    ],
    personUnavailability: [
      {
        id: "unavailability_1",
        organisation_id: "org_1",
        person_id: "person_1",
        date: "2026-01-08",
        hours: 2
      }
    ],
    reportBuckets: [
      {
        period_start: "2026-01-01",
        availability_hours: 160,
        load_hours: 60,
        project_load_hours: 60,
        project_estimation_hours: 100,
        free_hours: 100,
        utilization_pct: 37.5,
        project_completion_pct: 60
      }
    ]
  }
}

/**
 * Overrides replace top-level store collections.
 * Example: passing { persons: [...] } replaces the default persons fixture.
 */
export function buildMockAPI(overrides?: Partial<MockStore>): MockSetup {
  const store = cloneStore({ ...buildMockStore(), ...overrides })
  const nextIDs: Record<string, number> = {
    org: 2,
    person: 3,
    project: 2,
    group: 2,
    allocation: 2,
    unavailability: 2
  }
  const nextID = (prefix: keyof typeof nextIDs): string => {
    const value = `${prefix}_${nextIDs[prefix]}`
    nextIDs[prefix] += 1
    return value
  }

  const fetchMock = vi.fn(async (input: string | URL | Request, options?: RequestInit) => {
    const method = options?.method ?? "GET"
    const url = typeof input === "string" ? input : input.toString()
    const path = new URL(url, "http://localhost").pathname
    const bodyText = typeof options?.body === "string" ? options.body : ""
    const body = bodyText ? JSON.parse(bodyText) as Record<string, unknown> : {}

    if (path === "/api/organisations" && method === "GET") {
      return jsonResponse(store.organisations)
    }
    if (path === "/api/organisations" && method === "POST") {
      const organisation: Organisation = {
        id: nextID("org"),
        name: String(body.name ?? ""),
        hours_per_day: Number(body.hours_per_day ?? 0),
        hours_per_week: Number(body.hours_per_week ?? 0),
        hours_per_year: Number(body.hours_per_year ?? 0)
      }
      store.organisations.push(organisation)
      return jsonResponse(organisation)
    }
    const organisationMatch = path.match(/^\/api\/organisations\/([^/]+)$/)
    if (organisationMatch && method === "PUT") {
      const organisationID = organisationMatch[1]
      const current = store.organisations.find((entry) => entry.id === organisationID)
      if (!current) {
        return jsonResponse({ error: "organisation not found" }, 404)
      }
      current.name = String(body.name ?? current.name)
      current.hours_per_day = Number(body.hours_per_day ?? current.hours_per_day)
      current.hours_per_week = Number(body.hours_per_week ?? current.hours_per_week)
      current.hours_per_year = Number(body.hours_per_year ?? current.hours_per_year)
      return jsonResponse(current)
    }
    if (organisationMatch && method === "DELETE") {
      const organisationID = organisationMatch[1]
      store.organisations = store.organisations.filter((entry) => entry.id !== organisationID)
      return jsonResponse({})
    }

    if (path === "/api/persons" && method === "GET") {
      return jsonResponse(store.persons)
    }
    if (path === "/api/persons" && method === "POST") {
      const person: Person = {
        id: nextID("person"),
        organisation_id: String(store.organisations[0]?.id ?? "org_1"),
        name: String(body.name ?? ""),
        employment_pct: Number(body.employment_pct ?? 0)
      }
      store.persons.push(person)
      return jsonResponse(person)
    }
    const personMatch = path.match(/^\/api\/persons\/([^/]+)$/)
    if (personMatch && method === "PUT") {
      const personID = personMatch[1]
      const current = store.persons.find((entry) => entry.id === personID)
      if (!current) {
        return jsonResponse({ error: "person not found" }, 404)
      }
      current.name = String(body.name ?? current.name)
      current.employment_pct = Number(body.employment_pct ?? current.employment_pct)
      if (typeof body.employment_effective_from_month === "string") {
        const nextChange = {
          effective_month: body.employment_effective_from_month,
          employment_pct: Number(body.employment_pct ?? current.employment_pct)
        }
        current.employment_changes = [...(current.employment_changes ?? []), nextChange]
      }
      return jsonResponse(current)
    }
    if (personMatch && method === "DELETE") {
      const personID = personMatch[1]
      store.persons = store.persons.filter((entry) => entry.id !== personID)
      store.groups = store.groups.map((group) => ({
        ...group,
        member_ids: group.member_ids.filter((memberID) => memberID !== personID)
      }))
      store.allocations = store.allocations.filter((allocation) => allocation.target_id !== personID)
      store.personUnavailability = store.personUnavailability.filter((entry) => entry.person_id !== personID)
      return jsonResponse({})
    }

    const personUnavailabilityMatch = path.match(/^\/api\/persons\/([^/]+)\/unavailability$/)
    if (personUnavailabilityMatch && method === "GET") {
      const personID = personUnavailabilityMatch[1]
      return jsonResponse(store.personUnavailability.filter((entry) => entry.person_id === personID))
    }
    if (personUnavailabilityMatch && method === "POST") {
      const personID = personUnavailabilityMatch[1]
      const entry: PersonUnavailability = {
        id: nextID("unavailability"),
        organisation_id: String(store.organisations[0]?.id ?? "org_1"),
        person_id: personID,
        date: String(body.date ?? ""),
        hours: Number(body.hours ?? 0)
      }
      store.personUnavailability.push(entry)
      return jsonResponse(entry)
    }
    const personUnavailabilityDeleteMatch = path.match(/^\/api\/persons\/([^/]+)\/unavailability\/([^/]+)$/)
    if (personUnavailabilityDeleteMatch && method === "DELETE") {
      const personID = personUnavailabilityDeleteMatch[1]
      const entryID = personUnavailabilityDeleteMatch[2]
      store.personUnavailability = store.personUnavailability
        .filter((entry) => !(entry.person_id === personID && entry.id === entryID))
      return jsonResponse({})
    }

    if (path === "/api/projects" && method === "GET") {
      return jsonResponse(store.projects)
    }
    if (path === "/api/projects" && method === "POST") {
      const project: Project = {
        id: nextID("project"),
        organisation_id: String(store.organisations[0]?.id ?? "org_1"),
        name: String(body.name ?? ""),
        start_date: String(body.start_date ?? "2026-01-01"),
        end_date: String(body.end_date ?? "2026-12-31"),
        estimated_effort_hours: Number(body.estimated_effort_hours ?? 0)
      }
      store.projects.push(project)
      return jsonResponse(project)
    }
    const projectMatch = path.match(/^\/api\/projects\/([^/]+)$/)
    if (projectMatch && method === "PUT") {
      const projectID = projectMatch[1]
      const current = store.projects.find((entry) => entry.id === projectID)
      if (!current) {
        return jsonResponse({ error: "project not found" }, 404)
      }
      current.name = String(body.name ?? current.name)
      current.start_date = String(body.start_date ?? current.start_date)
      current.end_date = String(body.end_date ?? current.end_date)
      current.estimated_effort_hours = Number(body.estimated_effort_hours ?? current.estimated_effort_hours)
      return jsonResponse(current)
    }
    if (projectMatch && method === "DELETE") {
      const projectID = projectMatch[1]
      store.projects = store.projects.filter((entry) => entry.id !== projectID)
      store.allocations = store.allocations.filter((entry) => entry.project_id !== projectID)
      return jsonResponse({})
    }

    if (path === "/api/groups" && method === "GET") {
      return jsonResponse(store.groups)
    }
    if (path === "/api/groups" && method === "POST") {
      const group: Group = {
        id: nextID("group"),
        organisation_id: String(store.organisations[0]?.id ?? "org_1"),
        name: String(body.name ?? ""),
        member_ids: Array.isArray(body.member_ids) ? body.member_ids.map((value) => String(value)) : []
      }
      store.groups.push(group)
      return jsonResponse(group)
    }
    const groupMatch = path.match(/^\/api\/groups\/([^/]+)$/)
    if (groupMatch && method === "PUT") {
      const groupID = groupMatch[1]
      const current = store.groups.find((entry) => entry.id === groupID)
      if (!current) {
        return jsonResponse({ error: "group not found" }, 404)
      }
      current.name = String(body.name ?? current.name)
      current.member_ids = Array.isArray(body.member_ids) ? body.member_ids.map((value) => String(value)) : current.member_ids
      return jsonResponse(current)
    }
    if (groupMatch && method === "DELETE") {
      const groupID = groupMatch[1]
      store.groups = store.groups.filter((entry) => entry.id !== groupID)
      store.allocations = store.allocations.filter((entry) => entry.target_id !== groupID)
      return jsonResponse({})
    }
    const addGroupMemberMatch = path.match(/^\/api\/groups\/([^/]+)\/members$/)
    if (addGroupMemberMatch && method === "POST") {
      const groupID = addGroupMemberMatch[1]
      const personID = String(body.person_id ?? "")
      const current = store.groups.find((entry) => entry.id === groupID)
      if (!current) {
        return jsonResponse({ error: "group not found" }, 404)
      }
      if (!current.member_ids.includes(personID)) {
        current.member_ids.push(personID)
      }
      return jsonResponse(current)
    }
    const removeGroupMemberMatch = path.match(/^\/api\/groups\/([^/]+)\/members\/([^/]+)$/)
    if (removeGroupMemberMatch && method === "DELETE") {
      const groupID = removeGroupMemberMatch[1]
      const personID = removeGroupMemberMatch[2]
      const current = store.groups.find((entry) => entry.id === groupID)
      if (!current) {
        return jsonResponse({ error: "group not found" }, 404)
      }
      current.member_ids = current.member_ids.filter((memberID) => memberID !== personID)
      return jsonResponse(current)
    }

    if (path === "/api/allocations" && method === "GET") {
      return jsonResponse(store.allocations)
    }
    if (path === "/api/allocations" && method === "POST") {
      const allocation: Allocation = {
        id: nextID("allocation"),
        organisation_id: String(store.organisations[0]?.id ?? "org_1"),
        target_type: String(body.target_type ?? "person") as Allocation["target_type"],
        target_id: String(body.target_id ?? ""),
        project_id: String(body.project_id ?? ""),
        start_date: String(body.start_date ?? ""),
        end_date: String(body.end_date ?? ""),
        percent: Number(body.percent ?? 0)
      }
      store.allocations.push(allocation)
      return jsonResponse(allocation)
    }
    const allocationMatch = path.match(/^\/api\/allocations\/([^/]+)$/)
    if (allocationMatch && method === "PUT") {
      const allocationID = allocationMatch[1]
      const current = store.allocations.find((entry) => entry.id === allocationID)
      if (!current) {
        return jsonResponse({ error: "allocation not found" }, 404)
      }
      current.target_type = String(body.target_type ?? current.target_type) as Allocation["target_type"]
      current.target_id = String(body.target_id ?? current.target_id)
      current.project_id = String(body.project_id ?? current.project_id)
      current.start_date = String(body.start_date ?? current.start_date)
      current.end_date = String(body.end_date ?? current.end_date)
      current.percent = Number(body.percent ?? current.percent)
      return jsonResponse(current)
    }
    if (allocationMatch && method === "DELETE") {
      const allocationID = allocationMatch[1]
      store.allocations = store.allocations.filter((entry) => entry.id !== allocationID)
      return jsonResponse({})
    }

    if (path === "/api/reports/availability-load" && method === "POST") {
      return jsonResponse({ buckets: store.reportBuckets })
    }

    return textResponse("[]", 200)
  })

  vi.stubGlobal("fetch", fetchMock)
  return {
    fetchMock,
    store,
    restore: () => {
      vi.unstubAllGlobals()
    }
  }
}
