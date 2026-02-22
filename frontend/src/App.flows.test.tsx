import { fireEvent, render, screen, waitFor, within } from "@testing-library/react"
import { afterEach, describe, vi } from "vitest"
import App, {
  type Allocation,
  type Group,
  type Organisation,
  type Person,
  type PersonUnavailability,
  type Project,
  type ReportBucket
} from "./App"

type MockStore = {
  organisations: Organisation[]
  persons: Person[]
  projects: Project[]
  groups: Group[]
  allocations: Allocation[]
  personUnavailability: PersonUnavailability[]
  reportBuckets: ReportBucket[]
}

type MockSetup = {
  fetchMock: ReturnType<typeof vi.fn>
  store: MockStore
}

function jsonResponse(payload: unknown, status = 200) {
  return {
    ok: status >= 200 && status < 300,
    status,
    text: async () => JSON.stringify(payload),
    json: async () => payload
  }
}

function textResponse(text: string, status: number) {
  return {
    ok: status >= 200 && status < 300,
    status,
    text: async () => text,
    json: async () => JSON.parse(text)
  }
}

function cloneStore(store: MockStore): MockStore {
  return JSON.parse(JSON.stringify(store)) as MockStore
}

function buildMockStore(): MockStore {
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

function buildMockAPI(overrides?: Partial<MockStore>): MockSetup {
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
  return { fetchMock, store }
}

function sectionByHeading(name: RegExp): ReturnType<typeof within> {
  const heading = screen.getByRole("heading", { name })
  const section = heading.closest("section")
  expect(section).not.toBeNull()
  return within(section as HTMLElement)
}

afterEach(() => {
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

describe("App broad flows", () => {
  it("covers management and report actions across sections", async () => {
    const { fetchMock } = buildMockAPI()

    render(<App />)

    await waitFor(() => {
      expect(screen.getAllByRole("option", { name: "Alpha Org" }).length).toBeGreaterThan(0)
    })

    const authPanel = sectionByHeading(/^auth and tenant$/i)
    const organisationPanel = sectionByHeading(/^organisation$/i)
    const personsPanel = sectionByHeading(/^persons$/i)
    const projectsPanel = sectionByHeading(/^projects$/i)
    const groupsPanel = sectionByHeading(/^groups$/i)
    const reportPanel = sectionByHeading(/^report$/i)

    fireEvent.change(authPanel.getByLabelText(/^role$/i), { target: { value: "org_user" } })
    fireEvent.click(authPanel.getByRole("button", { name: /^refresh$/i }))

    fireEvent.change(organisationPanel.getByLabelText(/^name$/i), { target: { value: "Beta Org" } })
    fireEvent.change(organisationPanel.getByLabelText(/^working hours$/i), { target: { value: "40" } })
    fireEvent.change(organisationPanel.getByLabelText(/^working hours unit$/i), { target: { value: "week" } })
    fireEvent.click(organisationPanel.getByRole("button", { name: /^create organisation$/i }))

    await waitFor(() => {
      expect(screen.getByText("organisation created")).toBeInTheDocument()
      expect(authPanel.getByRole("option", { name: "Beta Org" })).toBeInTheDocument()
    })

    fireEvent.change(authPanel.getByLabelText(/^active organisation$/i), { target: { value: "org_2" } })
    fireEvent.change(organisationPanel.getByLabelText(/^name$/i), { target: { value: "Beta Org Updated" } })
    fireEvent.click(organisationPanel.getByRole("button", { name: /^update selected organisation$/i }))

    await waitFor(() => {
      expect(screen.getByText("organisation updated")).toBeInTheDocument()
    })

    fireEvent.click(organisationPanel.getByRole("button", { name: /^delete selected organisation$/i }))
    await waitFor(() => {
      expect(screen.getByText("organisation deleted")).toBeInTheDocument()
      expect(authPanel.queryByRole("option", { name: "Beta Org Updated" })).not.toBeInTheDocument()
    })

    fireEvent.change(personsPanel.getByLabelText(/^name$/i), { target: { value: "Cara" } })
    fireEvent.change(personsPanel.getByLabelText(/^employment percent$/i), { target: { value: "75" } })
    fireEvent.click(personsPanel.getByRole("button", { name: /^save person$/i }))

    await waitFor(() => {
      expect(screen.getByText("person created")).toBeInTheDocument()
      expect(personsPanel.getByRole("cell", { name: "Cara" })).toBeInTheDocument()
    })

    const aliceRow = personsPanel.getByRole("cell", { name: "Alice" }).closest("tr")
    expect(aliceRow).not.toBeNull()
    fireEvent.click(within(aliceRow as HTMLElement).getByRole("button", { name: /^edit$/i }))
    fireEvent.change(personsPanel.getByLabelText(/^name$/i), { target: { value: "Alice Prime" } })
    fireEvent.change(personsPanel.getByLabelText(/^employment percent$/i), { target: { value: "95" } })
    fireEvent.change(personsPanel.getByLabelText(/^effective from month$/i), { target: { value: "2026-07" } })
    fireEvent.click(personsPanel.getByRole("button", { name: /^save person$/i }))

    await waitFor(() => {
      expect(screen.getByText("person updated")).toBeInTheDocument()
      expect(personsPanel.getByRole("cell", { name: "Alice Prime" })).toBeInTheDocument()
    })

    const bobRow = personsPanel.getByRole("cell", { name: "Bob" }).closest("tr")
    expect(bobRow).not.toBeNull()
    fireEvent.click(within(bobRow as HTMLElement).getByRole("button", { name: /^delete$/i }))
    await waitFor(() => {
      expect(screen.getByText("person deleted")).toBeInTheDocument()
      expect(personsPanel.queryByRole("cell", { name: "Bob" })).not.toBeInTheDocument()
    })

    fireEvent.change(projectsPanel.getByLabelText(/^name$/i), { target: { value: "Borealis" } })
    fireEvent.change(projectsPanel.getByLabelText(/^start date$/i), { target: { value: "2026-02-01" } })
    fireEvent.change(projectsPanel.getByLabelText(/^end date$/i), { target: { value: "2026-04-30" } })
    fireEvent.change(projectsPanel.getByLabelText(/^estimated effort hours$/i), { target: { value: "120" } })
    fireEvent.click(projectsPanel.getByRole("button", { name: /^save project$/i }))

    await waitFor(() => {
      expect(screen.getByText("project created")).toBeInTheDocument()
      expect(projectsPanel.getByRole("cell", { name: "Borealis" })).toBeInTheDocument()
    })

    const apolloRow = projectsPanel.getByRole("cell", { name: "Apollo" }).closest("tr")
    expect(apolloRow).not.toBeNull()
    fireEvent.click(within(apolloRow as HTMLElement).getByRole("button", { name: /^edit$/i }))
    fireEvent.change(projectsPanel.getByLabelText(/^name$/i), { target: { value: "Apollo Prime" } })
    fireEvent.click(projectsPanel.getByRole("button", { name: /^save project$/i }))

    await waitFor(() => {
      expect(screen.getByText("project updated")).toBeInTheDocument()
      expect(projectsPanel.getByRole("cell", { name: "Apollo Prime" })).toBeInTheDocument()
    })

    const borealisRow = projectsPanel.getByRole("cell", { name: "Borealis" }).closest("tr")
    expect(borealisRow).not.toBeNull()
    fireEvent.click(within(borealisRow as HTMLElement).getByRole("button", { name: /^delete$/i }))
    await waitFor(() => {
      expect(screen.getByText("project deleted")).toBeInTheDocument()
    })

    fireEvent.click(groupsPanel.getByRole("button", { name: /^add member$/i }))
    fireEvent.change(groupsPanel.getByLabelText(/^name$/i), { target: { value: "Ops" } })
    fireEvent.click(groupsPanel.getByRole("checkbox", { name: /alice prime/i }))
    fireEvent.click(groupsPanel.getByRole("button", { name: /^save group$/i }))

    await waitFor(() => {
      expect(screen.getByText("group created")).toBeInTheDocument()
      expect(groupsPanel.getByRole("cell", { name: "Ops" })).toBeInTheDocument()
    })

    const teamRow = groupsPanel.getByRole("cell", { name: "Team" }).closest("tr")
    expect(teamRow).not.toBeNull()
    fireEvent.click(within(teamRow as HTMLElement).getByRole("button", { name: /^edit$/i }))
    fireEvent.change(groupsPanel.getByLabelText(/^name$/i), { target: { value: "Team Updated" } })
    fireEvent.click(groupsPanel.getByRole("button", { name: /^save group$/i }))

    await waitFor(() => {
      expect(screen.getByText("group updated")).toBeInTheDocument()
      expect(groupsPanel.getByRole("cell", { name: "Team Updated" })).toBeInTheDocument()
    })

    fireEvent.change(groupsPanel.getByLabelText(/^group$/i), { target: { value: "group_1" } })
    fireEvent.change(groupsPanel.getByLabelText(/^person$/i), { target: { value: "person_3" } })
    fireEvent.click(groupsPanel.getByRole("button", { name: /^add member$/i }))

    await waitFor(() => {
      expect(screen.getByText("group member added")).toBeInTheDocument()
    })

    fireEvent.click(groupsPanel.getAllByRole("button", { name: /^remove$/i })[0])
    await waitFor(() => {
      expect(screen.getByText("group member removed")).toBeInTheDocument()
    })

    const opsRow = groupsPanel.getByRole("cell", { name: "Ops" }).closest("tr")
    expect(opsRow).not.toBeNull()
    fireEvent.click(within(opsRow as HTMLElement).getByRole("button", { name: /^delete$/i }))
    await waitFor(() => {
      expect(screen.getByText("group deleted")).toBeInTheDocument()
    })

    fireEvent.change(reportPanel.getByLabelText(/^scope$/i), { target: { value: "group" } })
    fireEvent.change(reportPanel.getByLabelText(/^scope$/i), { target: { value: "project" } })
    fireEvent.change(reportPanel.getByLabelText(/^scope$/i), { target: { value: "person" } })

    fireEvent.click(reportPanel.getByRole("checkbox", { name: /alice prime/i }))
    fireEvent.click(reportPanel.getByRole("checkbox", { name: /alice prime/i }))
    fireEvent.click(reportPanel.getByRole("checkbox", { name: /alice prime/i }))
    fireEvent.change(reportPanel.getByLabelText(/^from$/i), { target: { value: "2026-01-02" } })
    fireEvent.change(reportPanel.getByLabelText(/^to$/i), { target: { value: "2026-02-01" } })
    fireEvent.change(reportPanel.getByLabelText(/^granularity$/i), { target: { value: "week" } })
    fireEvent.click(reportPanel.getByRole("button", { name: /^run report$/i }))

    await waitFor(() => {
      expect(screen.getByText("report calculated")).toBeInTheDocument()
      expect(reportPanel.getByRole("cell", { name: "2026-01-01" })).toBeInTheDocument()
    })

    const putPersonCall = fetchMock.mock.calls.find(([requestURL, requestInit]) => {
      return String(requestURL).includes("/api/persons/person_1")
        && (requestInit as RequestInit | undefined)?.method === "PUT"
    })
    expect(putPersonCall).toBeDefined()
  })

  it("covers unavailability and allocation actions with replace and edit paths", async () => {
    const { store } = buildMockAPI({
      groups: [
        { id: "group_1", organisation_id: "org_1", name: "Team", member_ids: ["person_1", "person_2"] }
      ],
      allocations: [
        {
          id: "allocation_group_1",
          organisation_id: "org_1",
          target_type: "group",
          target_id: "group_1",
          project_id: "project_1",
          start_date: "2026-01-01",
          end_date: "2026-12-31",
          percent: 20
        },
        {
          id: "allocation_person_1",
          organisation_id: "org_1",
          target_type: "person",
          target_id: "person_1",
          project_id: "project_1",
          start_date: "2026-01-01",
          end_date: "2026-12-31",
          percent: 30
        }
      ]
    })
    vi.spyOn(window, "confirm").mockReturnValue(true)

    render(<App />)

    await waitFor(() => {
      expect(screen.getAllByRole("option", { name: "Alpha Org" }).length).toBeGreaterThan(0)
    })

    const allocationPanel = sectionByHeading(/^allocations$/i)
    const holidaysPanel = sectionByHeading(/^holidays and unavailability$/i)

    const initialGroupRow = allocationPanel.getByText(/group:\s*Team/i).closest("tr")
    expect(initialGroupRow).not.toBeNull()
    fireEvent.click(within(initialGroupRow as HTMLElement).getByRole("button", { name: /^edit$/i }))
    fireEvent.click(allocationPanel.getByRole("button", { name: /^save allocation$/i }))

    await waitFor(() => {
      expect(screen.getByText(/allocation updated|allocation created/i)).toBeInTheDocument()
    })

    fireEvent.change(allocationPanel.getByLabelText(/^target type$/i), { target: { value: "group" } })
    fireEvent.change(allocationPanel.getByLabelText(/^target$/i), { target: { value: "group_1" } })
    fireEvent.change(allocationPanel.getByLabelText(/^project$/i), { target: { value: "project_1" } })
    fireEvent.change(allocationPanel.getByLabelText(/^start date$/i), { target: { value: "2026-02-01" } })
    fireEvent.change(allocationPanel.getByLabelText(/^end date$/i), { target: { value: "2026-11-30" } })
    fireEvent.change(allocationPanel.getByLabelText(/^merge strategy$/i), { target: { value: "replace" } })
    fireEvent.change(allocationPanel.getByLabelText(/^fte % per day$/i), { target: { value: "25" } })
    fireEvent.click(allocationPanel.getByRole("button", { name: /^save allocation$/i }))

    await waitFor(() => {
      expect(screen.getByText(/allocation created/i)).toBeInTheDocument()
    })

    fireEvent.click(allocationPanel.getAllByRole("button", { name: /^edit$/i })[0])
    await waitFor(() => {
      expect(allocationPanel.getByRole("button", { name: /^switch to creation context$/i })).toBeInTheDocument()
    })
    fireEvent.click(allocationPanel.getByRole("button", { name: /^save allocation$/i }))
    await waitFor(() => {
      expect(screen.getByText(/allocation updated|allocation created/i)).toBeInTheDocument()
    })

    const firstAllocationRow = allocationPanel.getAllByRole("row").find((row: HTMLElement) => {
      return within(row).queryByRole("button", { name: /^delete$/i })
    })
    expect(firstAllocationRow).not.toBeUndefined()
    fireEvent.click(within(firstAllocationRow as HTMLElement).getByRole("button", { name: /^delete$/i }))
    await waitFor(() => {
      expect(screen.getByText("allocation deleted")).toBeInTheDocument()
    })

    fireEvent.change(holidaysPanel.getByLabelText(/^units$/i), { target: { value: "hours" } })
    fireEvent.change(holidaysPanel.getByLabelText(/^organisation$/i), { target: { value: "org_1" } })
    fireEvent.change(holidaysPanel.getByLabelText(/^date$/i), { target: { value: "2026-01-09" } })
    fireEvent.change(holidaysPanel.getByLabelText(/^hours$/i), { target: { value: "4" } })
    fireEvent.click(holidaysPanel.getByRole("button", { name: /^add org unavailability$/i }))

    await waitFor(() => {
      expect(screen.getByText(/organisation unavailability added/i)).toBeInTheDocument()
    })
    fireEvent.click(holidaysPanel.getAllByRole("button", { name: /^delete$/i })[0])
    await waitFor(() => {
      expect(screen.getByText("person unavailability deleted")).toBeInTheDocument()
    })

    fireEvent.change(holidaysPanel.getByLabelText(/^units$/i), { target: { value: "weeks" } })
    fireEvent.change(holidaysPanel.getByLabelText(/^start week$/i), { target: { value: "2026-W04" } })
    fireEvent.change(holidaysPanel.getByLabelText(/^end week$/i), { target: { value: "2026-W04" } })

    fireEvent.change(holidaysPanel.getByLabelText(/^units$/i), { target: { value: "days" } })
    fireEvent.change(holidaysPanel.getByLabelText(/^start date$/i), { target: { value: "2026-01-12" } })
    fireEvent.change(holidaysPanel.getByLabelText(/^end date$/i), { target: { value: "2026-01-13" } })
    fireEvent.click(holidaysPanel.getByRole("button", { name: /^add org member unavailability$/i }))

    await waitFor(() => {
      expect(screen.getByText(/organisation unavailability entries added/i)).toBeInTheDocument()
    })

    fireEvent.change(holidaysPanel.getByLabelText(/^scope$/i), { target: { value: "person" } })
    fireEvent.change(holidaysPanel.getByLabelText(/^units$/i), { target: { value: "hours" } })
    fireEvent.click(holidaysPanel.getByRole("button", { name: /^add person unavailability$/i }))
    fireEvent.change(holidaysPanel.getByLabelText(/^person$/i), { target: { value: "person_1" } })
    fireEvent.change(holidaysPanel.getByLabelText(/^date$/i), { target: { value: "2026-01-14" } })
    fireEvent.change(holidaysPanel.getByLabelText(/^hours$/i), { target: { value: "3" } })
    fireEvent.click(holidaysPanel.getByRole("button", { name: /^add person unavailability$/i }))

    await waitFor(() => {
      expect(screen.getByText(/person unavailability added/i)).toBeInTheDocument()
    })

    fireEvent.change(holidaysPanel.getByLabelText(/^units$/i), { target: { value: "days" } })
    fireEvent.change(holidaysPanel.getByLabelText(/^person$/i), { target: { value: "person_1" } })
    fireEvent.change(holidaysPanel.getByLabelText(/^start date$/i), { target: { value: "2026-01-15" } })
    fireEvent.change(holidaysPanel.getByLabelText(/^end date$/i), { target: { value: "2026-01-16" } })
    fireEvent.click(holidaysPanel.getByRole("button", { name: /^add person unavailability entries$/i }))
    await waitFor(() => {
      expect(screen.getByText(/person unavailability entries added/i)).toBeInTheDocument()
    })
    fireEvent.change(holidaysPanel.getByLabelText(/^units$/i), { target: { value: "weeks" } })
    fireEvent.change(holidaysPanel.getByLabelText(/^start week$/i), { target: { value: "2026-W06" } })
    fireEvent.change(holidaysPanel.getByLabelText(/^end week$/i), { target: { value: "2026-W06" } })

    fireEvent.click(holidaysPanel.getAllByRole("button", { name: /^delete$/i })[0])
    await waitFor(() => {
      expect(screen.getByText("person unavailability deleted")).toBeInTheDocument()
    })

    fireEvent.change(holidaysPanel.getByLabelText(/^scope$/i), { target: { value: "group" } })
    fireEvent.change(holidaysPanel.getByLabelText(/^units$/i), { target: { value: "hours" } })
    fireEvent.click(holidaysPanel.getByRole("button", { name: /^add group unavailability$/i }))
    fireEvent.change(holidaysPanel.getByLabelText(/^group$/i), { target: { value: "group_1" } })
    fireEvent.change(holidaysPanel.getByLabelText(/^date$/i), { target: { value: "2026-01-20" } })
    fireEvent.change(holidaysPanel.getByLabelText(/^hours$/i), { target: { value: "1" } })
    fireEvent.click(holidaysPanel.getByRole("button", { name: /^add group unavailability$/i }))

    await waitFor(() => {
      expect(screen.getByText(/group unavailability added/i)).toBeInTheDocument()
    })

    fireEvent.change(holidaysPanel.getByLabelText(/^group$/i), { target: { value: "group_1" } })
    fireEvent.click(holidaysPanel.getAllByRole("button", { name: /^delete$/i })[0])
    await waitFor(() => {
      expect(screen.getByText("person unavailability deleted")).toBeInTheDocument()
    })

    fireEvent.change(holidaysPanel.getByLabelText(/^units$/i), { target: { value: "days" } })
    fireEvent.change(holidaysPanel.getByLabelText(/^start date$/i), { target: { value: "2026-01-21" } })
    fireEvent.change(holidaysPanel.getByLabelText(/^end date$/i), { target: { value: "2026-01-22" } })

    fireEvent.change(holidaysPanel.getByLabelText(/^units$/i), { target: { value: "weeks" } })
    fireEvent.change(holidaysPanel.getByLabelText(/^group$/i), { target: { value: "group_1" } })
    fireEvent.change(holidaysPanel.getByLabelText(/^start week$/i), { target: { value: "2026-W05" } })
    fireEvent.change(holidaysPanel.getByLabelText(/^end week$/i), { target: { value: "2026-W05" } })
    fireEvent.click(holidaysPanel.getByRole("button", { name: /^add group unavailability entries$/i }))

    await waitFor(() => {
      expect(screen.getByText(/group member unavailability entries added/i)).toBeInTheDocument()
    })

    expect(store.personUnavailability.length).toBeGreaterThan(0)
  })

  it("supports row edit actions, expandable groups, and batch delete across list sections", async () => {
    buildMockAPI({
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
        },
        {
          id: "allocation_2",
          organisation_id: "org_1",
          target_type: "person",
          target_id: "person_2",
          project_id: "project_1",
          start_date: "2026-01-01",
          end_date: "2026-12-31",
          percent: 15
        }
      ]
    })
    vi.spyOn(window, "confirm").mockReturnValue(true)

    render(<App />)

    await waitFor(() => {
      expect(screen.getAllByRole("option", { name: "Alpha Org" }).length).toBeGreaterThan(0)
    })

    const personsPanel = sectionByHeading(/^persons$/i)
    const projectsPanel = sectionByHeading(/^projects$/i)
    const groupsPanel = sectionByHeading(/^groups$/i)
    const allocationsPanel = sectionByHeading(/^allocations$/i)

    const bobRow = personsPanel.getByRole("cell", { name: "Bob" }).closest("tr")
    expect(bobRow).not.toBeNull()
    fireEvent.click(within(bobRow as HTMLElement).getByRole("button", { name: /^edit$/i }))
    expect(personsPanel.getByText(/^edit context: person: bob$/i)).toBeInTheDocument()

    fireEvent.change(projectsPanel.getByLabelText(/^name$/i), { target: { value: "Borealis" } })
    fireEvent.change(projectsPanel.getByLabelText(/^start date$/i), { target: { value: "2026-02-01" } })
    fireEvent.change(projectsPanel.getByLabelText(/^end date$/i), { target: { value: "2026-03-31" } })
    fireEvent.change(projectsPanel.getByLabelText(/^estimated effort hours$/i), { target: { value: "80" } })
    fireEvent.click(projectsPanel.getByRole("button", { name: /^save project$/i }))
    await waitFor(() => {
      expect(screen.getByText("project created")).toBeInTheDocument()
    })

    fireEvent.change(groupsPanel.getByLabelText(/^name$/i), { target: { value: "Ops" } })
    fireEvent.click(groupsPanel.getByRole("checkbox", { name: /alice/i }))
    fireEvent.click(groupsPanel.getByRole("button", { name: /^save group$/i }))
    await waitFor(() => {
      expect(screen.getByText("group created")).toBeInTheDocument()
    })

    const groupsHeading = screen.getByRole("heading", { name: /^groups$/i })
    const groupsSection = groupsHeading.closest("section")
    expect(groupsSection).not.toBeNull()
    const groupDetails = (groupsSection as HTMLElement).querySelector("details")
    expect(groupDetails).not.toBeNull()
    expect((groupDetails as HTMLDetailsElement).open).toBe(false)

    fireEvent.click(allocationsPanel.getAllByRole("button", { name: /^edit$/i })[0])
    await waitFor(() => {
      expect(allocationsPanel.getByText(/^edit context:/i)).toBeInTheDocument()
    })

    fireEvent.click(allocationsPanel.getByLabelText(/select allocation allocation_1/i))
    fireEvent.click(allocationsPanel.getByLabelText(/select allocation allocation_2/i))
    fireEvent.click(allocationsPanel.getByRole("button", { name: /^delete selected items$/i }))
    await waitFor(() => {
      expect(screen.getByText("selected allocations deleted")).toBeInTheDocument()
      expect(allocationsPanel.getByText(/^creation context: creating a new allocation\.$/i)).toBeInTheDocument()
    })

    fireEvent.click(projectsPanel.getByLabelText(/select project apollo/i))
    fireEvent.click(projectsPanel.getByLabelText(/select project borealis/i))
    fireEvent.click(projectsPanel.getByRole("button", { name: /^delete selected items$/i }))
    await waitFor(() => {
      expect(screen.getByText("selected projects deleted")).toBeInTheDocument()
    })

    fireEvent.click(groupsPanel.getByLabelText(/select group team/i))
    fireEvent.click(groupsPanel.getByLabelText(/select group ops/i))
    fireEvent.click(groupsPanel.getByRole("button", { name: /^delete selected items$/i }))
    await waitFor(() => {
      expect(screen.getByText("selected groups deleted")).toBeInTheDocument()
    })

    fireEvent.click(personsPanel.getByLabelText(/select person alice/i))
    fireEvent.click(personsPanel.getByLabelText(/select person bob/i))
    fireEvent.click(personsPanel.getByRole("button", { name: /^delete selected items$/i }))
    await waitFor(() => {
      expect(screen.getByText("selected persons deleted")).toBeInTheDocument()
    })
  })

  it("does not batch-delete allocations when confirmation is declined", async () => {
    buildMockAPI({
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
        },
        {
          id: "allocation_2",
          organisation_id: "org_1",
          target_type: "person",
          target_id: "person_2",
          project_id: "project_1",
          start_date: "2026-01-01",
          end_date: "2026-12-31",
          percent: 15
        }
      ]
    })
    vi.spyOn(window, "confirm").mockReturnValue(false)

    render(<App />)

    await waitFor(() => {
      expect(screen.getAllByRole("option", { name: "Alpha Org" }).length).toBeGreaterThan(0)
    })

    const allocationsPanel = sectionByHeading(/^allocations$/i)
    fireEvent.click(allocationsPanel.getByLabelText(/select allocation allocation_1/i))
    fireEvent.click(allocationsPanel.getByLabelText(/select allocation allocation_2/i))
    fireEvent.click(allocationsPanel.getByRole("button", { name: /^delete selected items$/i }))

    await waitFor(() => {
      expect(screen.queryByText("selected allocations deleted")).not.toBeInTheDocument()
      expect(allocationsPanel.getByRole("cell", { name: /person:\s*Alice/i })).toBeInTheDocument()
    })
  })

  it("does not create organisation unavailability when no organisation is selected", async () => {
    const { fetchMock } = buildMockAPI()

    render(<App />)

    await waitFor(() => {
      expect(screen.getAllByRole("option", { name: "Alpha Org" }).length).toBeGreaterThan(0)
    })

    const authPanel = sectionByHeading(/^auth and tenant$/i)
    const holidaysPanel = sectionByHeading(/^holidays and unavailability$/i)
    fireEvent.change(authPanel.getByLabelText(/^active organisation$/i), { target: { value: "" } })
    fireEvent.click(holidaysPanel.getByRole("button", { name: /^add org unavailability$/i }))

    await waitFor(() => {
      const postUnavailabilityCalls = fetchMock.mock.calls.filter(([requestURL, requestInit]) => {
        return /\/api\/persons\/.+\/unavailability$/.test(String(requestURL))
          && (requestInit as RequestInit | undefined)?.method === "POST"
      })
      expect(postUnavailabilityCalls).toHaveLength(0)
    })
  })

  it("covers select-all toggles and edit actions for each list", async () => {
    buildMockAPI({
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
        },
        {
          id: "allocation_2",
          organisation_id: "org_1",
          target_type: "person",
          target_id: "person_2",
          project_id: "project_1",
          start_date: "2026-01-01",
          end_date: "2026-12-31",
          percent: 15
        }
      ]
    })

    render(<App />)

    await waitFor(() => {
      expect(screen.getAllByRole("option", { name: "Alpha Org" }).length).toBeGreaterThan(0)
    })

    const personsPanel = sectionByHeading(/^persons$/i)
    const projectsPanel = sectionByHeading(/^projects$/i)
    const groupsPanel = sectionByHeading(/^groups$/i)
    const allocationsPanel = sectionByHeading(/^allocations$/i)

    fireEvent.click(personsPanel.getByLabelText(/select all persons/i))
    fireEvent.click(personsPanel.getByLabelText(/select all persons/i))
    fireEvent.click(personsPanel.getByLabelText(/select all persons/i))
    fireEvent.click(personsPanel.getByLabelText(/select person alice/i))
    fireEvent.click(personsPanel.getByLabelText(/select person alice/i))

    fireEvent.click(projectsPanel.getByLabelText(/select all projects/i))
    fireEvent.click(projectsPanel.getByLabelText(/select all projects/i))
    fireEvent.click(projectsPanel.getByLabelText(/select all projects/i))
    fireEvent.click(projectsPanel.getByLabelText(/select project apollo/i))
    fireEvent.click(projectsPanel.getByRole("button", { name: /^edit$/i }))
    expect(projectsPanel.getByText(/^edit context: project: apollo$/i)).toBeInTheDocument()

    fireEvent.click(groupsPanel.getByLabelText(/select all groups/i))
    fireEvent.click(groupsPanel.getByLabelText(/select all groups/i))
    fireEvent.click(groupsPanel.getByLabelText(/select all groups/i))
    fireEvent.click(groupsPanel.getByLabelText(/select group team/i))
    fireEvent.click(groupsPanel.getByRole("button", { name: /^edit$/i }))
    expect(groupsPanel.getByText(/^edit context: group: team$/i)).toBeInTheDocument()

    fireEvent.click(groupsPanel.getByRole("checkbox", { name: /alice/i }))
    fireEvent.click(groupsPanel.getByRole("checkbox", { name: /alice/i }))
    fireEvent.click(groupsPanel.getByRole("checkbox", { name: /alice/i }))

    fireEvent.change(groupsPanel.getByLabelText(/^name$/i), { target: { value: "Empty Team" } })
    fireEvent.click(groupsPanel.getByRole("button", { name: /^save group$/i }))
    await waitFor(() => {
      expect(screen.getByText("group updated")).toBeInTheDocument()
    })
    fireEvent.click(groupsPanel.getByText("0 member(s)"))
    await waitFor(() => {
      expect(groupsPanel.getByText("No members")).toBeInTheDocument()
    })

    fireEvent.click(allocationsPanel.getByLabelText(/select all allocations/i))
    fireEvent.click(allocationsPanel.getByLabelText(/select all allocations/i))
    fireEvent.click(allocationsPanel.getByLabelText(/select all allocations/i))
    fireEvent.click(allocationsPanel.getByLabelText(/select allocation allocation_1/i))
    fireEvent.click(allocationsPanel.getByLabelText(/select allocation allocation_1/i))
  })

  it("shows per-period report summaries with expandable multi-object details", async () => {
    const { fetchMock } = buildMockAPI()

    render(<App />)

    await waitFor(() => {
      expect(screen.getAllByRole("option", { name: "Alpha Org" }).length).toBeGreaterThan(0)
    })

    const reportPanel = sectionByHeading(/^report$/i)
    fireEvent.change(reportPanel.getByLabelText(/^scope$/i), { target: { value: "person" } })
    fireEvent.click(reportPanel.getByRole("button", { name: /^run report$/i }))

    await waitFor(() => {
      expect(screen.getByText("report calculated")).toBeInTheDocument()
      expect(reportPanel.getByRole("columnheader", { name: /^object$/i })).toBeInTheDocument()
      expect(reportPanel.getByText("Total")).toBeInTheDocument()
      expect(reportPanel.getAllByRole("cell", { name: "2026-01-01" })).toHaveLength(1)
      expect(reportPanel.queryByRole("cell", { name: "Alice (100%)" })).not.toBeInTheDocument()
    })

    fireEvent.click(reportPanel.getByRole("button", { name: /^show 2 entries$/i }))
    await waitFor(() => {
      expect(reportPanel.getByRole("button", { name: /^hide entries$/i })).toBeInTheDocument()
      expect(reportPanel.getByRole("cell", { name: "Alice (100%)" })).toBeInTheDocument()
      expect(reportPanel.getByRole("cell", { name: "Bob (80%)" })).toBeInTheDocument()
      expect(reportPanel.getAllByRole("cell", { name: "2026-01-01" })).toHaveLength(1)
    })

    const reportCalls = fetchMock.mock.calls.filter(([requestURL, requestInit]) => {
      return String(requestURL).endsWith("/api/reports/availability-load")
        && (requestInit as RequestInit | undefined)?.method === "POST"
    })
    expect(reportCalls).toHaveLength(2)

    const sortedReportIDs = reportCalls
      .map((call) => JSON.parse(String((call[1] as RequestInit).body)).ids?.[0] as string)
      .sort()
    expect(sortedReportIDs).toEqual(["person_1", "person_2"])
  })

  it("shows a scoped report error when one object report request fails", async () => {
    const { fetchMock } = buildMockAPI()
    const baseImpl = fetchMock.getMockImplementation() as
      | ((input: string | URL | Request, options?: RequestInit) => unknown)
      | undefined
    fetchMock.mockImplementation(async (input: string | URL | Request, options?: RequestInit) => {
      const url = typeof input === "string" ? input : input.toString()
      const path = new URL(url, "http://localhost").pathname
      const method = options?.method ?? "GET"
      if (path === "/api/reports/availability-load" && method === "POST") {
        const payload = JSON.parse(String(options?.body ?? "{}")) as { ids?: string[] }
        if (Array.isArray(payload.ids) && payload.ids[0] === "person_2") {
          return jsonResponse({ error: "report failed" }, 500)
        }
      }
      if (!baseImpl) {
        return jsonResponse([])
      }
      return baseImpl(input, options)
    })

    render(<App />)

    await waitFor(() => {
      expect(screen.getAllByRole("option", { name: "Alpha Org" }).length).toBeGreaterThan(0)
    })

    const reportPanel = sectionByHeading(/^report$/i)
    fireEvent.change(reportPanel.getByLabelText(/^scope$/i), { target: { value: "person" } })
    fireEvent.click(reportPanel.getByRole("button", { name: /^run report$/i }))

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent("report failed for 1 object(s)")
    })
  })

  it("runs organisation and project reports and renders project columns", async () => {
    const { fetchMock } = buildMockAPI()

    render(<App />)

    await waitFor(() => {
      expect(screen.getAllByRole("option", { name: "Alpha Org" }).length).toBeGreaterThan(0)
    })

    const reportPanel = sectionByHeading(/^report$/i)
    fireEvent.click(reportPanel.getByRole("button", { name: /^run report$/i }))
    await waitFor(() => {
      expect(screen.getByText("report calculated")).toBeInTheDocument()
    })

    fireEvent.change(reportPanel.getByLabelText(/^scope$/i), { target: { value: "project" } })
    fireEvent.click(reportPanel.getByRole("checkbox", { name: /apollo/i }))
    fireEvent.click(reportPanel.getByRole("button", { name: /^run report$/i }))

    await waitFor(() => {
      expect(reportPanel.getByRole("columnheader", { name: /^project load hours$/i })).toBeInTheDocument()
      expect(reportPanel.getByRole("columnheader", { name: /^project estimation hours$/i })).toBeInTheDocument()
      expect(reportPanel.getByRole("columnheader", { name: /^project completion %$/i })).toBeInTheDocument()
      expect(reportPanel.getByText("100.00")).toBeInTheDocument()
      expect(reportPanel.getAllByText("60.00").length).toBeGreaterThanOrEqual(1)
    })

    const reportCalls = fetchMock.mock.calls.filter(([requestURL, requestInit]) => {
      return String(requestURL).endsWith("/api/reports/availability-load")
        && (requestInit as RequestInit | undefined)?.method === "POST"
    })
    expect(reportCalls.length).toBeGreaterThanOrEqual(2)
  })

  it("covers request parsing failures and displays errors", async () => {
    const fetchMock = vi.fn(async (input: string | URL | Request, options?: RequestInit) => {
      const method = options?.method ?? "GET"
      const url = typeof input === "string" ? input : input.toString()
      const path = new URL(url, "http://localhost").pathname

      if (path === "/api/organisations" && method === "GET") {
        return {
          ok: true,
          status: 204,
          text: async () => "",
          json: async () => ({})
        }
      }
      if (path === "/api/persons" && method === "GET") {
        return jsonResponse([])
      }
      if (path === "/api/projects" && method === "GET") {
        return jsonResponse([])
      }
      if (path === "/api/groups" && method === "GET") {
        return jsonResponse([])
      }
      if (path === "/api/allocations" && method === "GET") {
        return jsonResponse([])
      }
      if (path === "/api/organisations" && method === "POST") {
        return {
          ok: true,
          status: 200,
          text: async () => "",
          json: async () => ({})
        }
      }
      return jsonResponse([])
    })
    vi.stubGlobal("fetch", fetchMock)

    render(<App />)

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent("request returned no content for /api/organisations")
    })

    const organisationPanel = sectionByHeading(/^organisation$/i)
    fireEvent.change(organisationPanel.getByLabelText(/^name$/i), { target: { value: "Broken Org" } })
    fireEvent.click(organisationPanel.getByRole("button", { name: /^create organisation$/i }))

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent("request returned empty body for /api/organisations")
    })
  })

  it("handles non-json no-content error responses", async () => {
    const fetchMock = vi.fn(async (input: string | URL | Request, options?: RequestInit) => {
      const method = options?.method ?? "GET"
      const url = typeof input === "string" ? input : input.toString()
      const path = new URL(url, "http://localhost").pathname

      if (path === "/api/organisations" && method === "GET") {
        return jsonResponse([
          { id: "org_1", name: "Org", hours_per_day: 8, hours_per_week: 40, hours_per_year: 2080 }
        ])
      }
      if (path === "/api/persons" && method === "GET") {
        return jsonResponse([])
      }
      if (path === "/api/projects" && method === "GET") {
        return jsonResponse([])
      }
      if (path === "/api/groups" && method === "GET") {
        return jsonResponse([])
      }
      if (path === "/api/allocations" && method === "GET") {
        return jsonResponse([])
      }
      if (path === "/api/organisations/org_1" && method === "DELETE") {
        return {
          ok: false,
          status: 502,
          text: async () => "upstream unavailable",
          json: async () => ({})
        }
      }
      return jsonResponse([])
    })
    vi.stubGlobal("fetch", fetchMock)

    render(<App />)

    await waitFor(() => {
      expect(screen.getAllByRole("option", { name: "Org" }).length).toBeGreaterThan(0)
    })

    const organisationPanel = sectionByHeading(/^organisation$/i)
    fireEvent.click(organisationPanel.getByRole("button", { name: /^delete selected organisation$/i }))

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent("request failed with status 502: upstream unavailable")
    })
  })
})
