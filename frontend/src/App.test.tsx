import { fireEvent, render, screen, waitFor, within } from "@testing-library/react"
import { afterEach, beforeEach, describe, vi } from "vitest"
import App from "./App"
import { buildMockAPI, jsonResponse } from "./test-utils/mocks"

/**
 * Scope: focused component and panel-level integration behavior.
 * These tests validate one behavior or edge case per test.
 * Cross-panel journeys live in App.flows.test.tsx.
 */

beforeEach(() => {
  vi.stubGlobal(
    "fetch",
    vi.fn(async () => jsonResponse([]))
  )
})

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

function sectionByHeading(name: RegExp): ReturnType<typeof within> {
  const heading = screen.getByRole("heading", { name })
  const section = heading.closest("section")
  if (!section) {
    throw new Error(`section for heading ${name.toString()} not found`)
  }
  return within(section as HTMLElement)
}

describe("App focused behaviors", () => {
  it("renders the title", async () => {
    render(<App />)
    expect(screen.getByRole("heading", { name: /plato mvp/i })).toBeInTheDocument()
    await waitFor(() => {
      expect(fetch).toHaveBeenCalled()
    })
  })

  it("renders one holiday and unavailability flow with scope and unit-driven fields", async () => {
    render(<App />)

    const holidaysHeading = screen.getByRole("heading", { name: /holidays and unavailability/i })
    const holidaysSection = holidaysHeading.closest("section")
    expect(holidaysSection).not.toBeNull()

    const holidays = within(holidaysSection as HTMLElement)
    expect(holidays.getByRole("heading", { level: 3, name: /entry scope/i })).toBeInTheDocument()
    expect(holidays.queryByRole("heading", { level: 3, name: /organisation holidays/i })).not.toBeInTheDocument()
    expect(holidays.queryByRole("heading", { level: 3, name: /person unavailability/i })).not.toBeInTheDocument()
    expect(holidays.queryByRole("heading", { level: 3, name: /group unavailability/i })).not.toBeInTheDocument()

    const scopeSelect = holidays.getByLabelText(/^scope$/i)
    const unitsSelect = holidays.getByLabelText(/^units$/i)
    expect(scopeSelect).toBeInTheDocument()
    expect(unitsSelect).toBeInTheDocument()
    expect(holidays.getByLabelText(/^organisation$/i)).toBeInTheDocument()
    expect(holidays.getByLabelText(/^date$/i)).toBeInTheDocument()
    expect(holidays.getByLabelText(/^hours$/i)).toBeInTheDocument()
    expect(holidays.queryByLabelText(/^start date$/i)).not.toBeInTheDocument()
    expect(holidays.queryByLabelText(/^end date$/i)).not.toBeInTheDocument()
    expect(holidays.queryByLabelText(/^days$/i)).not.toBeInTheDocument()
    expect(holidays.queryByLabelText(/^weeks$/i)).not.toBeInTheDocument()
    expect(holidays.queryByLabelText(/^person$/i)).not.toBeInTheDocument()
    expect(holidays.queryByLabelText(/^group$/i)).not.toBeInTheDocument()

    fireEvent.change(unitsSelect, { target: { value: "days" } })
    expect(holidays.getByLabelText(/^start date$/i)).toBeInTheDocument()
    expect(holidays.getByLabelText(/^end date$/i)).toBeInTheDocument()
    expect(holidays.queryByLabelText(/^date$/i)).not.toBeInTheDocument()
    expect(holidays.queryByLabelText(/^hours$/i)).not.toBeInTheDocument()
    expect(holidays.queryByLabelText(/^days$/i)).not.toBeInTheDocument()
    expect(holidays.queryByLabelText(/^weeks$/i)).not.toBeInTheDocument()
    expect(holidays.queryByLabelText(/^start week$/i)).not.toBeInTheDocument()
    expect(holidays.queryByLabelText(/^end week$/i)).not.toBeInTheDocument()

    fireEvent.change(unitsSelect, { target: { value: "weeks" } })
    expect(holidays.getByLabelText(/^start week$/i)).toBeInTheDocument()
    expect(holidays.getByLabelText(/^end week$/i)).toBeInTheDocument()
    expect(holidays.queryByLabelText(/^start date$/i)).not.toBeInTheDocument()
    expect(holidays.queryByLabelText(/^end date$/i)).not.toBeInTheDocument()
    expect(holidays.queryByLabelText(/^days$/i)).not.toBeInTheDocument()
    expect(holidays.queryByLabelText(/^weeks$/i)).not.toBeInTheDocument()

    fireEvent.change(scopeSelect, { target: { value: "person" } })
    expect(holidays.getByLabelText(/^person$/i)).toBeInTheDocument()
    expect(holidays.queryByLabelText(/^organisation$/i)).not.toBeInTheDocument()
    expect(holidays.queryByLabelText(/^group$/i)).not.toBeInTheDocument()

    fireEvent.change(scopeSelect, { target: { value: "group" } })
    expect(holidays.getByLabelText(/^group$/i)).toBeInTheDocument()
    expect(holidays.queryByLabelText(/^organisation$/i)).not.toBeInTheDocument()
    expect(holidays.queryByLabelText(/^person$/i)).not.toBeInTheDocument()

    await waitFor(() => {
      expect(fetch).toHaveBeenCalled()
    })
  })

  it("shows scope-specific report metrics", async () => {
    render(<App />)
    await waitFor(() => {
      expect(fetch).toHaveBeenCalled()
    })

    const reportHeading = screen.getByRole("heading", { name: /^report$/i })
    const reportSection = reportHeading.closest("section")
    expect(reportSection).not.toBeNull()
    const report = within(reportSection as HTMLElement)

    expect(report.getByRole("columnheader", { name: /^availability hours$/i })).toBeInTheDocument()
    expect(report.getByRole("columnheader", { name: /^load hours$/i })).toBeInTheDocument()
    expect(report.getByRole("columnheader", { name: /^free hours$/i })).toBeInTheDocument()
    expect(report.getByRole("columnheader", { name: /^utilization %$/i })).toBeInTheDocument()
    expect(report.queryByRole("columnheader", { name: /^project load hours$/i })).not.toBeInTheDocument()
    expect(report.queryByRole("columnheader", { name: /^project estimation hours$/i })).not.toBeInTheDocument()
    expect(report.queryByRole("columnheader", { name: /^project completion %$/i })).not.toBeInTheDocument()

    fireEvent.change(report.getByLabelText(/^scope$/i), { target: { value: "project" } })

    expect(report.queryByRole("columnheader", { name: /^availability hours$/i })).not.toBeInTheDocument()
    expect(report.queryByRole("columnheader", { name: /^load hours$/i })).not.toBeInTheDocument()
    expect(report.queryByRole("columnheader", { name: /^free hours$/i })).not.toBeInTheDocument()
    expect(report.queryByRole("columnheader", { name: /^utilization %$/i })).not.toBeInTheDocument()
    expect(report.getByRole("columnheader", { name: /^project load hours$/i })).toBeInTheDocument()
    expect(report.getByRole("columnheader", { name: /^project estimation hours$/i })).toBeInTheDocument()
    expect(report.getByRole("columnheader", { name: /^project completion %$/i })).toBeInTheDocument()
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

  it("sends month-scoped employment updates for person edits", async () => {
    const organisations = [
      { id: "org_1", name: "Org One", hours_per_day: 8, hours_per_week: 40, hours_per_year: 2080 }
    ]
    let persons = [
      {
        id: "person_1",
        organisation_id: "org_1",
        name: "Alice",
        employment_pct: 80,
        employment_changes: [{ effective_month: "2026-04", employment_pct: 70 }]
      }
    ]

    const fetchMock = vi.fn(async (input: string | URL | Request, options?: RequestInit) => {
      const url = typeof input === "string" ? input : input.toString()
      const method = options?.method ?? "GET"

      if (url.endsWith("/api/organisations") && method === "GET") {
        return jsonResponse(organisations)
      }
      if (url.endsWith("/api/persons") && method === "GET") {
        return jsonResponse(persons)
      }
      if (url.endsWith("/api/projects") && method === "GET") {
        return jsonResponse([])
      }
      if (url.endsWith("/api/groups") && method === "GET") {
        return jsonResponse([])
      }
      if (url.endsWith("/api/allocations") && method === "GET") {
        return jsonResponse([])
      }
      if (url.endsWith("/api/persons/person_1/unavailability") && method === "GET") {
        return jsonResponse([])
      }
      if (url.endsWith("/api/persons/person_1") && method === "PUT") {
        const payload = JSON.parse((options?.body as string) ?? "{}") as {
          name?: string
          employment_pct?: number
          employment_effective_from_month?: string
        }
        if (payload.employment_effective_from_month) {
          persons = [{
            ...persons[0],
            name: payload.name ?? persons[0].name,
            employment_pct: payload.employment_pct ?? persons[0].employment_pct,
            employment_changes: [
              ...(persons[0].employment_changes ?? []),
              {
                effective_month: payload.employment_effective_from_month,
                employment_pct: payload.employment_pct ?? persons[0].employment_pct
              }
            ]
          }]
        }
        return jsonResponse(persons[0])
      }

      return jsonResponse([])
    })
    vi.stubGlobal("fetch", fetchMock)

    render(<App />)

    const personsHeading = screen.getByRole("heading", { name: /^persons$/i })
    const personsSection = personsHeading.closest("section")
    expect(personsSection).not.toBeNull()
    const personPanel = within(personsSection as HTMLElement)

    await waitFor(() => {
      expect(personPanel.getByRole("cell", { name: "Alice" })).toBeInTheDocument()
    })

    const personRow = personPanel.getByRole("cell", { name: "Alice" }).closest("tr")
    expect(personRow).not.toBeNull()
    fireEvent.click(within(personRow as HTMLElement).getByRole("button", { name: /^edit$/i }))
    fireEvent.change(personPanel.getByLabelText(/^employment percent$/i), { target: { value: "75" } })
    fireEvent.change(personPanel.getByLabelText(/^effective from month$/i), { target: { value: "2026-06" } })
    fireEvent.click(personPanel.getByRole("button", { name: /^save person$/i }))

    await waitFor(() => {
      const putCall = fetchMock.mock.calls.find((call) => {
        const [callURL, callOptions] = call
        return String(callURL).includes("/api/persons/person_1")
          && (callOptions as RequestInit | undefined)?.method === "PUT"
      })
      expect(putCall).toBeDefined()
      const callOptions = putCall?.[1] as RequestInit
      const payload = JSON.parse(String(callOptions.body))
      expect(payload).toMatchObject({
        name: "Alice",
        employment_pct: 75,
        employment_effective_from_month: "2026-06"
      })
    })
  })

  it("converts allocation hours per week input to FTE percent payload", async () => {
    const organisations = [
      { id: "org_1", name: "Org One", hours_per_day: 8, hours_per_week: 40, hours_per_year: 2080 }
    ]
    const persons = [
      { id: "person_1", organisation_id: "org_1", name: "Alice", employment_pct: 80 }
    ]
    const projects = [
      {
        id: "project_1",
        organisation_id: "org_1",
        name: "Project One",
        start_date: "2026-01-01",
        end_date: "2026-12-31",
        estimated_effort_hours: 1000
      }
    ]

    const fetchMock = vi.fn(async (input: string | URL | Request, options?: RequestInit) => {
      const url = typeof input === "string" ? input : input.toString()
      const method = options?.method ?? "GET"

      if (url.endsWith("/api/organisations") && method === "GET") {
        return jsonResponse(organisations)
      }
      if (url.endsWith("/api/persons") && method === "GET") {
        return jsonResponse(persons)
      }
      if (url.endsWith("/api/projects") && method === "GET") {
        return jsonResponse(projects)
      }
      if (url.endsWith("/api/groups") && method === "GET") {
        return jsonResponse([])
      }
      if (url.endsWith("/api/allocations") && method === "GET") {
        return jsonResponse([])
      }
      if (url.endsWith("/api/persons/person_1/unavailability") && method === "GET") {
        return jsonResponse([])
      }
      if (url.endsWith("/api/allocations") && method === "POST") {
        return jsonResponse({
          id: "allocation_1",
          organisation_id: "org_1",
          target_type: "person",
          target_id: "person_1",
          project_id: "project_1",
          start_date: "2026-01-01",
          end_date: "2026-12-31",
          percent: 50
        })
      }

      return jsonResponse([])
    })
    vi.stubGlobal("fetch", fetchMock)

    render(<App />)

    const allocationsHeading = screen.getByRole("heading", { name: /^allocations$/i })
    const allocationsSection = allocationsHeading.closest("section")
    expect(allocationsSection).not.toBeNull()
    const allocationPanel = within(allocationsSection as HTMLElement)

    await waitFor(() => {
      expect(allocationPanel.getByRole("option", { name: "Alice" })).toBeInTheDocument()
      expect(allocationPanel.getByRole("option", { name: "Project One" })).toBeInTheDocument()
    })

    fireEvent.change(allocationPanel.getByLabelText(/^target$/i), { target: { value: "person_1" } })
    fireEvent.change(allocationPanel.getByLabelText(/^project$/i), { target: { value: "project_1" } })
    fireEvent.change(allocationPanel.getByLabelText(/^load value type$/i), { target: { value: "hours" } })
    fireEvent.change(allocationPanel.getByLabelText(/^load unit$/i), { target: { value: "week" } })
    fireEvent.change(allocationPanel.getByLabelText(/^hours per week$/i), { target: { value: "20" } })
    fireEvent.click(allocationPanel.getByRole("button", { name: /^save allocation$/i }))

    await waitFor(() => {
      const postCall = fetchMock.mock.calls.find((call) => {
        const [callURL, callOptions] = call
        return String(callURL).endsWith("/api/allocations")
          && (callOptions as RequestInit | undefined)?.method === "POST"
      })
      expect(postCall).toBeDefined()
      const callOptions = postCall?.[1] as RequestInit
      const payload = JSON.parse(String(callOptions.body))
      expect(payload).toMatchObject({
        target_type: "person",
        target_id: "person_1",
        project_id: "project_1",
        percent: 50
      })
    })
  })

  it("requires selecting at least one user before saving an allocation", async () => {
    buildMockAPI()

    render(<App />)

    const allocationsHeading = screen.getByRole("heading", { name: /^allocations$/i })
    const allocationsSection = allocationsHeading.closest("section")
    expect(allocationsSection).not.toBeNull()
    const allocationPanel = within(allocationsSection as HTMLElement)

    await waitFor(() => {
      expect(allocationPanel.getByRole("option", { name: "Apollo" })).toBeInTheDocument()
    })

    fireEvent.click(allocationPanel.getByRole("button", { name: /^save allocation$/i }))

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent("select at least one user to allocate")
    })
  })

  it("requires an active organisation before saving an allocation", async () => {
    buildMockAPI()

    render(<App />)

    await waitFor(() => {
      expect(screen.getAllByRole("option", { name: "Alpha Org" }).length).toBeGreaterThan(0)
    })

    const authPanel = sectionByHeading(/^auth and tenant$/i)
    fireEvent.change(authPanel.getByLabelText(/^active organisation$/i), { target: { value: "" } })

    const allocationsHeading = screen.getByRole("heading", { name: /^allocations$/i })
    const allocationsSection = allocationsHeading.closest("section")
    expect(allocationsSection).not.toBeNull()
    const allocationPanel = within(allocationsSection as HTMLElement)

    fireEvent.change(allocationPanel.getByLabelText(/^target$/i), { target: { value: "person_1" } })
    fireEvent.change(allocationPanel.getByLabelText(/^project$/i), { target: { value: "project_1" } })
    fireEvent.click(allocationPanel.getByRole("button", { name: /^save allocation$/i }))

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent("select an organisation first")
    })
  })

  it("shows an over-allocation warning before save and cancels when declined", async () => {
    const organisations = [
      { id: "org_1", name: "Org One", hours_per_day: 8, hours_per_week: 40, hours_per_year: 2080 }
    ]
    const persons = [
      { id: "person_1", organisation_id: "org_1", name: "Alice", employment_pct: 50 }
    ]
    const projects = [
      {
        id: "project_1",
        organisation_id: "org_1",
        name: "Project One",
        start_date: "2026-01-01",
        end_date: "2026-12-31",
        estimated_effort_hours: 1000
      }
    ]

    const fetchMock = vi.fn(async (input: string | URL | Request, options?: RequestInit) => {
      const url = typeof input === "string" ? input : input.toString()
      const method = options?.method ?? "GET"

      if (url.endsWith("/api/organisations") && method === "GET") {
        return jsonResponse(organisations)
      }
      if (url.endsWith("/api/persons") && method === "GET") {
        return jsonResponse(persons)
      }
      if (url.endsWith("/api/projects") && method === "GET") {
        return jsonResponse(projects)
      }
      if (url.endsWith("/api/groups") && method === "GET") {
        return jsonResponse([])
      }
      if (url.endsWith("/api/allocations") && method === "GET") {
        return jsonResponse([])
      }
      if (url.endsWith("/api/persons/person_1/unavailability") && method === "GET") {
        return jsonResponse([])
      }
      if (url.endsWith("/api/allocations") && method === "POST") {
        return jsonResponse({})
      }

      return jsonResponse([])
    })
    vi.stubGlobal("fetch", fetchMock)

    const confirmMock = vi.spyOn(window, "confirm").mockReturnValue(false)

    render(<App />)

    const allocationsHeading = screen.getByRole("heading", { name: /^allocations$/i })
    const allocationsSection = allocationsHeading.closest("section")
    expect(allocationsSection).not.toBeNull()
    const allocationPanel = within(allocationsSection as HTMLElement)

    await waitFor(() => {
      expect(allocationPanel.getByRole("option", { name: "Alice" })).toBeInTheDocument()
      expect(allocationPanel.getByRole("option", { name: "Project One" })).toBeInTheDocument()
    })

    fireEvent.change(allocationPanel.getByLabelText(/^target$/i), { target: { value: "person_1" } })
    fireEvent.change(allocationPanel.getByLabelText(/^project$/i), { target: { value: "project_1" } })
    fireEvent.change(allocationPanel.getByLabelText(/^fte % per day$/i), { target: { value: "60" } })
    fireEvent.click(allocationPanel.getByRole("button", { name: /^save allocation$/i }))

    await waitFor(() => {
      expect(confirmMock).toHaveBeenCalledTimes(1)
      expect(confirmMock).toHaveBeenCalledWith(
        "Warning: allocation exceeds employment percentage for Alice. Continue?"
      )
    })

    const postCalls = fetchMock.mock.calls.filter((call) => {
      const [callURL, callOptions] = call
      return String(callURL).endsWith("/api/allocations")
        && (callOptions as RequestInit | undefined)?.method === "POST"
    })
    expect(postCalls).toHaveLength(0)
  })

  it("highlights over-allocated persons after user confirms save", async () => {
    const organisations = [
      { id: "org_1", name: "Org One", hours_per_day: 8, hours_per_week: 40, hours_per_year: 2080 }
    ]
    const persons = [
      { id: "person_1", organisation_id: "org_1", name: "Alice", employment_pct: 100 }
    ]
    const projects = [
      {
        id: "project_1",
        organisation_id: "org_1",
        name: "Project One",
        start_date: "2026-01-01",
        end_date: "2026-12-31",
        estimated_effort_hours: 1000
      },
      {
        id: "project_2",
        organisation_id: "org_1",
        name: "Project Two",
        start_date: "2026-01-01",
        end_date: "2026-12-31",
        estimated_effort_hours: 1000
      }
    ]
    let allocations = [
      {
        id: "allocation_1",
        organisation_id: "org_1",
        target_type: "person",
        target_id: "person_1",
        project_id: "project_1",
        start_date: "2026-01-01",
        end_date: "2026-12-31",
        percent: 90
      }
    ]

    const fetchMock = vi.fn(async (input: string | URL | Request, options?: RequestInit) => {
      const url = typeof input === "string" ? input : input.toString()
      const method = options?.method ?? "GET"

      if (url.endsWith("/api/organisations") && method === "GET") {
        return jsonResponse(organisations)
      }
      if (url.endsWith("/api/persons") && method === "GET") {
        return jsonResponse(persons)
      }
      if (url.endsWith("/api/projects") && method === "GET") {
        return jsonResponse(projects)
      }
      if (url.endsWith("/api/groups") && method === "GET") {
        return jsonResponse([])
      }
      if (url.endsWith("/api/allocations") && method === "GET") {
        return jsonResponse(allocations)
      }
      if (url.endsWith("/api/persons/person_1/unavailability") && method === "GET") {
        return jsonResponse([])
      }
      if (url.endsWith("/api/allocations") && method === "POST") {
        const payload = JSON.parse(String(options?.body ?? "{}"))
        allocations = [
          ...allocations,
          {
            id: "allocation_2",
            organisation_id: "org_1",
            target_type: payload.target_type,
            target_id: payload.target_id,
            project_id: payload.project_id,
            start_date: payload.start_date,
            end_date: payload.end_date,
            percent: payload.percent
          }
        ]
        return jsonResponse(allocations[allocations.length - 1])
      }
      if (url.endsWith("/api/reports/availability-load") && method === "POST") {
        return jsonResponse({
          buckets: [{
            period_start: "2026-01-01",
            availability_hours: 160,
            load_hours: 80,
            free_hours: 80,
            utilization_pct: 50
          }]
        })
      }

      return jsonResponse([])
    })
    vi.stubGlobal("fetch", fetchMock)

    const confirmMock = vi.spyOn(window, "confirm").mockReturnValue(true)

    render(<App />)

    const allocationsHeading = screen.getByRole("heading", { name: /^allocations$/i })
    const allocationsSection = allocationsHeading.closest("section")
    expect(allocationsSection).not.toBeNull()
    const allocationPanel = within(allocationsSection as HTMLElement)

    await waitFor(() => {
      expect(allocationPanel.getByRole("option", { name: "Alice" })).toBeInTheDocument()
      expect(allocationPanel.getByRole("option", { name: "Project Two" })).toBeInTheDocument()
    })

    fireEvent.change(allocationPanel.getByLabelText(/^target$/i), { target: { value: "person_1" } })
    fireEvent.change(allocationPanel.getByLabelText(/^project$/i), { target: { value: "project_2" } })
    fireEvent.change(allocationPanel.getByLabelText(/^fte % per day$/i), { target: { value: "20" } })
    fireEvent.click(allocationPanel.getByRole("button", { name: /^save allocation$/i }))

    const personsHeading = screen.getByRole("heading", { name: /^persons$/i })
    const personsSection = personsHeading.closest("section")
    expect(personsSection).not.toBeNull()
    const personsPanel = within(personsSection as HTMLElement)

    await waitFor(() => {
      expect(confirmMock).toHaveBeenCalledTimes(1)
      const personRow = personsPanel.getByRole("cell", { name: "Alice" }).closest("tr")
      expect(personRow).not.toBeNull()
      expect(personRow).toHaveClass("person-overallocated")
    })

    const reportHeading = screen.getByRole("heading", { name: /^report$/i })
    const reportSection = reportHeading.closest("section")
    expect(reportSection).not.toBeNull()
    const reportPanel = within(reportSection as HTMLElement)

    fireEvent.change(reportPanel.getByLabelText(/^scope$/i), { target: { value: "person" } })
    fireEvent.click(reportPanel.getByRole("button", { name: /^run report$/i }))

    await waitFor(() => {
      const reportPersonCheckbox = reportPanel.getByRole("checkbox", { name: "Alice (100%)" })
      const reportPersonLabel = reportPersonCheckbox.closest("label")
      expect(reportPersonLabel).not.toBeNull()
      expect(reportPersonLabel).toHaveClass("person-overallocated")

      const reportPersonCell = reportPanel.getByRole("cell", { name: "Alice (100%)" })
      const reportPersonRow = reportPersonCell.closest("tr")
      expect(reportPersonRow).not.toBeNull()
      expect(reportPersonRow).toHaveClass("person-overallocated")
      expect(reportPanel.getByRole("cell", { name: "50.00" })).toBeInTheDocument()
    })
  })

  it("shows merge strategy only on allocation conflict and can keep existing allocations", async () => {
    const organisations = [
      { id: "org_1", name: "Org One", hours_per_day: 8, hours_per_week: 40, hours_per_year: 2080 }
    ]
    const persons = [
      { id: "person_1", organisation_id: "org_1", name: "Alice", employment_pct: 100 }
    ]
    const groups = [
      { id: "group_1", organisation_id: "org_1", name: "Team", member_ids: ["person_1"] }
    ]
    const projects = [
      {
        id: "project_1",
        organisation_id: "org_1",
        name: "Project One",
        start_date: "2026-01-01",
        end_date: "2026-12-31",
        estimated_effort_hours: 1000
      }
    ]
    const allocations = [
      {
        id: "allocation_group_1",
        organisation_id: "org_1",
        target_type: "group",
        target_id: "group_1",
        project_id: "project_1",
        start_date: "2026-01-01",
        end_date: "2026-12-31",
        percent: 20
      }
    ]

    const fetchMock = vi.fn(async (input: string | URL | Request, options?: RequestInit) => {
      const url = typeof input === "string" ? input : input.toString()
      const method = options?.method ?? "GET"

      if (url.endsWith("/api/organisations") && method === "GET") {
        return jsonResponse(organisations)
      }
      if (url.endsWith("/api/persons") && method === "GET") {
        return jsonResponse(persons)
      }
      if (url.endsWith("/api/projects") && method === "GET") {
        return jsonResponse(projects)
      }
      if (url.endsWith("/api/groups") && method === "GET") {
        return jsonResponse(groups)
      }
      if (url.endsWith("/api/allocations") && method === "GET") {
        return jsonResponse(allocations)
      }
      if (url.endsWith("/api/persons/person_1/unavailability") && method === "GET") {
        return jsonResponse([])
      }
      if (url.endsWith("/api/allocations") && method === "POST") {
        return jsonResponse({})
      }

      return jsonResponse([])
    })
    vi.stubGlobal("fetch", fetchMock)

    render(<App />)

    const allocationsHeading = screen.getByRole("heading", { name: /^allocations$/i })
    const allocationsSection = allocationsHeading.closest("section")
    expect(allocationsSection).not.toBeNull()
    const allocationPanel = within(allocationsSection as HTMLElement)

    await waitFor(() => {
      expect(allocationPanel.getByRole("option", { name: "Alice" })).toBeInTheDocument()
      expect(allocationPanel.getByRole("option", { name: "Project One" })).toBeInTheDocument()
      expect(allocationPanel.getByText(/^creation context: creating a new allocation\.$/i)).toBeInTheDocument()
    })

    expect(allocationPanel.queryByLabelText(/^merge strategy$/i)).not.toBeInTheDocument()

    fireEvent.change(allocationPanel.getByLabelText(/^target$/i), { target: { value: "person_1" } })
    fireEvent.change(allocationPanel.getByLabelText(/^project$/i), { target: { value: "project_1" } })

    await waitFor(() => {
      expect(allocationPanel.getByLabelText(/^merge strategy$/i)).toBeInTheDocument()
      expect(allocationPanel.getByText(/^affected persons: alice$/i)).toBeInTheDocument()
    })

    fireEvent.change(allocationPanel.getByLabelText(/^merge strategy$/i), { target: { value: "keep" } })
    fireEvent.click(allocationPanel.getByRole("button", { name: /^save allocation$/i }))

    await waitFor(() => {
      const postCalls = fetchMock.mock.calls.filter((call) => {
        const [callURL, callOptions] = call
        return String(callURL).endsWith("/api/allocations")
          && (callOptions as RequestInit | undefined)?.method === "POST"
      })
      expect(postCalls).toHaveLength(0)
    })
  })

  it("keep strategy excludes only conflicting users from a group selection", async () => {
    const organisations = [
      { id: "org_1", name: "Org One", hours_per_day: 8, hours_per_week: 40, hours_per_year: 2080 }
    ]
    const persons = [
      { id: "person_1", organisation_id: "org_1", name: "Alice", employment_pct: 100 },
      { id: "person_2", organisation_id: "org_1", name: "Bob", employment_pct: 100 }
    ]
    const groups = [
      { id: "group_1", organisation_id: "org_1", name: "Team", member_ids: ["person_1", "person_2"] }
    ]
    const projects = [
      {
        id: "project_1",
        organisation_id: "org_1",
        name: "Project One",
        start_date: "2026-01-01",
        end_date: "2026-12-31",
        estimated_effort_hours: 1000
      }
    ]
    const allocations = [
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

    const fetchMock = vi.fn(async (input: string | URL | Request, options?: RequestInit) => {
      const url = typeof input === "string" ? input : input.toString()
      const method = options?.method ?? "GET"

      if (url.endsWith("/api/organisations") && method === "GET") {
        return jsonResponse(organisations)
      }
      if (url.endsWith("/api/persons") && method === "GET") {
        return jsonResponse(persons)
      }
      if (url.endsWith("/api/projects") && method === "GET") {
        return jsonResponse(projects)
      }
      if (url.endsWith("/api/groups") && method === "GET") {
        return jsonResponse(groups)
      }
      if (url.endsWith("/api/allocations") && method === "GET") {
        return jsonResponse(allocations)
      }
      if (url.endsWith("/api/persons/person_1/unavailability") && method === "GET") {
        return jsonResponse([])
      }
      if (url.endsWith("/api/persons/person_2/unavailability") && method === "GET") {
        return jsonResponse([])
      }
      if (url.endsWith("/api/allocations") && method === "POST") {
        return jsonResponse({})
      }

      return jsonResponse([])
    })
    vi.stubGlobal("fetch", fetchMock)

    render(<App />)

    const allocationsHeading = screen.getByRole("heading", { name: /^allocations$/i })
    const allocationsSection = allocationsHeading.closest("section")
    expect(allocationsSection).not.toBeNull()
    const allocationPanel = within(allocationsSection as HTMLElement)

    await waitFor(() => {
      expect(allocationPanel.getByRole("option", { name: "Project One" })).toBeInTheDocument()
    })

    fireEvent.change(allocationPanel.getByLabelText(/^target type$/i), { target: { value: "group" } })
    await waitFor(() => {
      expect(allocationPanel.getByRole("option", { name: "Team" })).toBeInTheDocument()
    })
    fireEvent.change(allocationPanel.getByLabelText(/^target$/i), { target: { value: "group_1" } })
    fireEvent.change(allocationPanel.getByLabelText(/^project$/i), { target: { value: "project_1" } })

    await waitFor(() => {
      expect(allocationPanel.getByLabelText(/^merge strategy$/i)).toBeInTheDocument()
    })

    fireEvent.change(allocationPanel.getByLabelText(/^merge strategy$/i), { target: { value: "keep" } })
    await waitFor(() => {
      expect(allocationPanel.getByText(/1 selected user\(s\) will be excluded/i)).toBeInTheDocument()
    })
    fireEvent.click(allocationPanel.getByRole("button", { name: /^save allocation$/i }))

    await waitFor(() => {
      const postCalls = fetchMock.mock.calls.filter((call) => {
        const [callURL, callOptions] = call
        return String(callURL).endsWith("/api/allocations")
          && (callOptions as RequestInit | undefined)?.method === "POST"
      })
      expect(postCalls).toHaveLength(1)
      const payload = JSON.parse(String((postCalls[0]?.[1] as RequestInit).body))
      expect(payload).toMatchObject({
        target_type: "person",
        target_id: "person_2",
        project_id: "project_1"
      })
    })
  })

  it("warns with sorted person names when multiple users become over-allocated", async () => {
    const organisations = [
      { id: "org_1", name: "Org One", hours_per_day: 8, hours_per_week: 40, hours_per_year: 2080 }
    ]
    const persons = [
      { id: "person_1", organisation_id: "org_1", name: "Alice", employment_pct: 50 },
      { id: "person_2", organisation_id: "org_1", name: "Bob", employment_pct: 40 }
    ]
    const groups = [
      { id: "group_1", organisation_id: "org_1", name: "Team", member_ids: ["person_1", "person_2"] }
    ]
    const projects = [
      {
        id: "project_1",
        organisation_id: "org_1",
        name: "Project One",
        start_date: "2026-01-01",
        end_date: "2026-12-31",
        estimated_effort_hours: 1000
      }
    ]

    const fetchMock = vi.fn(async (input: string | URL | Request, options?: RequestInit) => {
      const url = typeof input === "string" ? input : input.toString()
      const method = options?.method ?? "GET"

      if (url.endsWith("/api/organisations") && method === "GET") {
        return jsonResponse(organisations)
      }
      if (url.endsWith("/api/persons") && method === "GET") {
        return jsonResponse(persons)
      }
      if (url.endsWith("/api/projects") && method === "GET") {
        return jsonResponse(projects)
      }
      if (url.endsWith("/api/groups") && method === "GET") {
        return jsonResponse(groups)
      }
      if (url.endsWith("/api/allocations") && method === "GET") {
        return jsonResponse([])
      }
      if (url.endsWith("/api/persons/person_1/unavailability") && method === "GET") {
        return jsonResponse([])
      }
      if (url.endsWith("/api/persons/person_2/unavailability") && method === "GET") {
        return jsonResponse([])
      }
      if (url.endsWith("/api/allocations") && method === "POST") {
        return jsonResponse({})
      }

      return jsonResponse([])
    })
    vi.stubGlobal("fetch", fetchMock)
    const confirmMock = vi.spyOn(window, "confirm").mockReturnValue(false)

    render(<App />)

    const allocationsHeading = screen.getByRole("heading", { name: /^allocations$/i })
    const allocationsSection = allocationsHeading.closest("section")
    expect(allocationsSection).not.toBeNull()
    const allocationPanel = within(allocationsSection as HTMLElement)

    await waitFor(() => {
      expect(allocationPanel.getByRole("option", { name: "Project One" })).toBeInTheDocument()
    })

    fireEvent.change(allocationPanel.getByLabelText(/^target type$/i), { target: { value: "group" } })
    await waitFor(() => {
      expect(allocationPanel.getByRole("option", { name: "Team" })).toBeInTheDocument()
    })
    fireEvent.change(allocationPanel.getByLabelText(/^target$/i), { target: { value: "group_1" } })
    fireEvent.change(allocationPanel.getByLabelText(/^project$/i), { target: { value: "project_1" } })
    fireEvent.change(allocationPanel.getByLabelText(/^fte % per day$/i), { target: { value: "60" } })
    fireEvent.click(allocationPanel.getByRole("button", { name: /^save allocation$/i }))

    await waitFor(() => {
      expect(confirmMock).toHaveBeenCalledWith(
        "Warning: allocation exceeds employment percentage for Alice, Bob. Continue?"
      )
    })

    const postCalls = fetchMock.mock.calls.filter((call) => {
      const [callURL, callOptions] = call
      return String(callURL).endsWith("/api/allocations")
        && (callOptions as RequestInit | undefined)?.method === "POST"
    })
    expect(postCalls).toHaveLength(0)
  })

  it("restores deleted allocations when replace flow partially fails", async () => {
    const { fetchMock } = buildMockAPI({
      groups: [
        { id: "group_1", organisation_id: "org_1", name: "Team", member_ids: ["person_1", "person_2"] }
      ],
      allocations: [
        {
          id: "allocation_person_1",
          organisation_id: "org_1",
          target_type: "person",
          target_id: "person_1",
          project_id: "project_1",
          start_date: "2026-01-01",
          end_date: "2026-12-31",
          percent: 20
        }
      ]
    })
    const baseImpl = fetchMock.getMockImplementation() as
      | ((input: string | URL | Request, options?: RequestInit) => unknown)
      | undefined
    fetchMock.mockImplementation(async (input: string | URL | Request, options?: RequestInit) => {
      const url = typeof input === "string" ? input : input.toString()
      const path = new URL(url, "http://localhost").pathname
      const method = options?.method ?? "GET"
      if (path === "/api/allocations" && method === "POST") {
        const payload = JSON.parse(String(options?.body ?? "{}")) as { target_id?: string }
        if (payload.target_id === "person_2") {
          return jsonResponse({ error: "create failed for person_2" }, 500)
        }
      }
      if (!baseImpl) {
        return jsonResponse([])
      }
      return baseImpl(input, options)
    })

    render(<App />)

    const allocationsHeading = screen.getByRole("heading", { name: /^allocations$/i })
    const allocationsSection = allocationsHeading.closest("section")
    expect(allocationsSection).not.toBeNull()
    const allocationPanel = within(allocationsSection as HTMLElement)

    await waitFor(() => {
      expect(allocationPanel.getByRole("option", { name: "Alice" })).toBeInTheDocument()
      expect(allocationPanel.getByRole("option", { name: "Apollo" })).toBeInTheDocument()
    })

    fireEvent.change(allocationPanel.getByLabelText(/^target type$/i), { target: { value: "group" } })
    await waitFor(() => {
      expect(allocationPanel.getByRole("option", { name: "Team" })).toBeInTheDocument()
    })
    fireEvent.change(allocationPanel.getByLabelText(/^target$/i), { target: { value: "group_1" } })
    fireEvent.change(allocationPanel.getByLabelText(/^project$/i), { target: { value: "project_1" } })

    await waitFor(() => {
      expect(allocationPanel.getByLabelText(/^merge strategy$/i)).toBeInTheDocument()
    })

    fireEvent.change(allocationPanel.getByLabelText(/^merge strategy$/i), { target: { value: "replace" } })
    fireEvent.change(allocationPanel.getByLabelText(/^fte % per day$/i), { target: { value: "25" } })
    fireEvent.click(allocationPanel.getByRole("button", { name: /^save allocation$/i }))

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent("created 1 of 2 allocations")
    })

    const deleteCalls = fetchMock.mock.calls.filter(([requestURL, requestInit]) => {
      return String(requestURL).includes("/api/allocations/allocation_person_1")
        && (requestInit as RequestInit | undefined)?.method === "DELETE"
    })
    expect(deleteCalls).toHaveLength(1)

    // Expected POST sequence: create person_1 succeeds, create person_2 fails, rollback restore posts original conflict.
    const createCalls = fetchMock.mock.calls.filter(([requestURL, requestInit]) => {
      return String(requestURL).endsWith("/api/allocations")
        && (requestInit as RequestInit | undefined)?.method === "POST"
    })
    expect(createCalls.length).toBeGreaterThanOrEqual(3)
  })

  it("reports rollback restore failures when replace flow cannot restore conflicts", async () => {
    const { fetchMock } = buildMockAPI({
      groups: [
        { id: "group_1", organisation_id: "org_1", name: "Team", member_ids: ["person_1", "person_2"] }
      ],
      allocations: [
        {
          id: "allocation_person_1",
          organisation_id: "org_1",
          target_type: "person",
          target_id: "person_1",
          project_id: "project_1",
          start_date: "2026-01-01",
          end_date: "2026-12-31",
          percent: 20
        }
      ]
    })
    const baseImpl = fetchMock.getMockImplementation() as
      | ((input: string | URL | Request, options?: RequestInit) => unknown)
      | undefined
    fetchMock.mockImplementation(async (input: string | URL | Request, options?: RequestInit) => {
      const url = typeof input === "string" ? input : input.toString()
      const path = new URL(url, "http://localhost").pathname
      const method = options?.method ?? "GET"
      if (path === "/api/allocations" && method === "POST") {
        const payload = JSON.parse(String(options?.body ?? "{}")) as { target_id?: string, percent?: number }
        if (payload.target_id === "person_2") {
          return jsonResponse({ error: "create failed for person_2" }, 500)
        }
        if (payload.target_id === "person_1" && payload.percent === 20) {
          return jsonResponse({ error: "restore failed for person_1" }, 500)
        }
      }
      if (!baseImpl) {
        return jsonResponse([])
      }
      return baseImpl(input, options)
    })

    render(<App />)

    const allocationsHeading = screen.getByRole("heading", { name: /^allocations$/i })
    const allocationsSection = allocationsHeading.closest("section")
    expect(allocationsSection).not.toBeNull()
    const allocationPanel = within(allocationsSection as HTMLElement)

    await waitFor(() => {
      expect(allocationPanel.getByRole("option", { name: "Alice" })).toBeInTheDocument()
      expect(allocationPanel.getByRole("option", { name: "Apollo" })).toBeInTheDocument()
    })

    fireEvent.change(allocationPanel.getByLabelText(/^target type$/i), { target: { value: "group" } })
    await waitFor(() => {
      expect(allocationPanel.getByRole("option", { name: "Team" })).toBeInTheDocument()
    })
    fireEvent.change(allocationPanel.getByLabelText(/^target$/i), { target: { value: "group_1" } })
    fireEvent.change(allocationPanel.getByLabelText(/^project$/i), { target: { value: "project_1" } })

    await waitFor(() => {
      expect(allocationPanel.getByLabelText(/^merge strategy$/i)).toBeInTheDocument()
    })

    fireEvent.change(allocationPanel.getByLabelText(/^merge strategy$/i), { target: { value: "replace" } })
    fireEvent.change(allocationPanel.getByLabelText(/^fte % per day$/i), { target: { value: "25" } })
    fireEvent.click(allocationPanel.getByRole("button", { name: /^save allocation$/i }))

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent("rollback restore failed for 1 allocation(s)")
    })
  })

  it("reports refresh failures after rollback processing", async () => {
    const { fetchMock } = buildMockAPI({
      groups: [
        { id: "group_1", organisation_id: "org_1", name: "Team", member_ids: ["person_1", "person_2"] }
      ],
      allocations: [
        {
          id: "allocation_person_1",
          organisation_id: "org_1",
          target_type: "person",
          target_id: "person_1",
          project_id: "project_1",
          start_date: "2026-01-01",
          end_date: "2026-12-31",
          percent: 20
        }
      ]
    })
    const baseImpl = fetchMock.getMockImplementation() as
      | ((input: string | URL | Request, options?: RequestInit) => unknown)
      | undefined
    let failReload = false
    fetchMock.mockImplementation(async (input: string | URL | Request, options?: RequestInit) => {
      const url = typeof input === "string" ? input : input.toString()
      const path = new URL(url, "http://localhost").pathname
      const method = options?.method ?? "GET"

      if (path === "/api/allocations" && method === "POST") {
        const payload = JSON.parse(String(options?.body ?? "{}")) as { target_id?: string }
        if (payload.target_id === "person_2") {
          failReload = true
          return jsonResponse({ error: "create failed for person_2" }, 500)
        }
      }

      if (failReload && method === "GET" && path === "/api/projects") {
        return jsonResponse({ error: "reload failed" }, 500)
      }

      if (!baseImpl) {
        return jsonResponse([])
      }
      return baseImpl(input, options)
    })

    render(<App />)

    const allocationsHeading = screen.getByRole("heading", { name: /^allocations$/i })
    const allocationsSection = allocationsHeading.closest("section")
    expect(allocationsSection).not.toBeNull()
    const allocationPanel = within(allocationsSection as HTMLElement)

    await waitFor(() => {
      expect(allocationPanel.getByRole("option", { name: "Alice" })).toBeInTheDocument()
      expect(allocationPanel.getByRole("option", { name: "Apollo" })).toBeInTheDocument()
    })

    fireEvent.change(allocationPanel.getByLabelText(/^target type$/i), { target: { value: "group" } })
    await waitFor(() => {
      expect(allocationPanel.getByRole("option", { name: "Team" })).toBeInTheDocument()
    })
    fireEvent.change(allocationPanel.getByLabelText(/^target$/i), { target: { value: "group_1" } })
    fireEvent.change(allocationPanel.getByLabelText(/^project$/i), { target: { value: "project_1" } })

    await waitFor(() => {
      expect(allocationPanel.getByLabelText(/^merge strategy$/i)).toBeInTheDocument()
    })

    fireEvent.change(allocationPanel.getByLabelText(/^merge strategy$/i), { target: { value: "replace" } })
    fireEvent.change(allocationPanel.getByLabelText(/^fte % per day$/i), { target: { value: "25" } })
    fireEvent.click(allocationPanel.getByRole("button", { name: /^save allocation$/i }))

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent("refresh failed")
    })
  })

  it("restores the edited group allocation when group edit replace flow fails", async () => {
    const { fetchMock } = buildMockAPI({
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
        }
      ]
    })
    const baseImpl = fetchMock.getMockImplementation() as
      | ((input: string | URL | Request, options?: RequestInit) => unknown)
      | undefined
    fetchMock.mockImplementation(async (input: string | URL | Request, options?: RequestInit) => {
      const url = typeof input === "string" ? input : input.toString()
      const path = new URL(url, "http://localhost").pathname
      const method = options?.method ?? "GET"
      if (path === "/api/allocations" && method === "POST") {
        const payload = JSON.parse(String(options?.body ?? "{}")) as { target_type?: string, target_id?: string }
        if (payload.target_type === "person" && payload.target_id === "person_2") {
          return jsonResponse({ error: "create failed for person_2" }, 500)
        }
      }
      if (!baseImpl) {
        return jsonResponse([])
      }
      return baseImpl(input, options)
    })

    render(<App />)

    const allocationsHeading = screen.getByRole("heading", { name: /^allocations$/i })
    const allocationsSection = allocationsHeading.closest("section")
    expect(allocationsSection).not.toBeNull()
    const allocationPanel = within(allocationsSection as HTMLElement)

    await waitFor(() => {
      expect(allocationPanel.getByText(/group:\s*Team/i)).toBeInTheDocument()
    })

    const groupRow = allocationPanel.getByText(/group:\s*Team/i).closest("tr")
    expect(groupRow).not.toBeNull()
    fireEvent.click(within(groupRow as HTMLElement).getByRole("button", { name: /^edit$/i }))
    fireEvent.change(allocationPanel.getByLabelText(/^fte % per day$/i), { target: { value: "25" } })
    fireEvent.click(allocationPanel.getByRole("button", { name: /^save allocation$/i }))

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent("created 1 of 2 allocations")
    })

    const deleteGroupCall = fetchMock.mock.calls.find(([requestURL, requestInit]) => {
      return String(requestURL).includes("/api/allocations/allocation_group_1")
        && (requestInit as RequestInit | undefined)?.method === "DELETE"
    })
    expect(deleteGroupCall).toBeDefined()

    const createCalls = fetchMock.mock.calls
      .filter(([requestURL, requestInit]) => {
        return String(requestURL).endsWith("/api/allocations")
          && (requestInit as RequestInit | undefined)?.method === "POST"
      })
      .map(([, requestInit]) => JSON.parse(String((requestInit as RequestInit).body)) as { target_type?: string })
    expect(createCalls.some((payload) => payload.target_type === "group")).toBe(true)
  })

  it("switches allocation form between creation and edit context via row actions", async () => {
    const organisations = [
      { id: "org_1", name: "Org One", hours_per_day: 8, hours_per_week: 40, hours_per_year: 2080 }
    ]
    const persons = [
      { id: "person_1", organisation_id: "org_1", name: "Alice", employment_pct: 100 }
    ]
    const projects = [
      {
        id: "project_1",
        organisation_id: "org_1",
        name: "Project One",
        start_date: "2026-01-01",
        end_date: "2026-12-31",
        estimated_effort_hours: 1000
      }
    ]
    const allocations = [
      {
        id: "allocation_person_1",
        organisation_id: "org_1",
        target_type: "person",
        target_id: "person_1",
        project_id: "project_1",
        start_date: "2026-01-01",
        end_date: "2026-12-31",
        percent: 40
      }
    ]

    const fetchMock = vi.fn(async (input: string | URL | Request, options?: RequestInit) => {
      const url = typeof input === "string" ? input : input.toString()
      const method = options?.method ?? "GET"

      if (url.endsWith("/api/organisations") && method === "GET") {
        return jsonResponse(organisations)
      }
      if (url.endsWith("/api/persons") && method === "GET") {
        return jsonResponse(persons)
      }
      if (url.endsWith("/api/projects") && method === "GET") {
        return jsonResponse(projects)
      }
      if (url.endsWith("/api/groups") && method === "GET") {
        return jsonResponse([])
      }
      if (url.endsWith("/api/allocations") && method === "GET") {
        return jsonResponse(allocations)
      }
      if (url.endsWith("/api/persons/person_1/unavailability") && method === "GET") {
        return jsonResponse([])
      }

      return jsonResponse([])
    })
    vi.stubGlobal("fetch", fetchMock)

    render(<App />)

    const allocationsHeading = screen.getByRole("heading", { name: /^allocations$/i })
    const allocationsSection = allocationsHeading.closest("section")
    expect(allocationsSection).not.toBeNull()
    const allocationPanel = within(allocationsSection as HTMLElement)

    await waitFor(() => {
      expect(allocationPanel.getByText("person: Alice")).toBeInTheDocument()
      expect(allocationPanel.getByText(/^creation context: creating a new allocation\.$/i)).toBeInTheDocument()
    })

    const allocationRow = allocationPanel.getByText("person: Alice").closest("tr")
    expect(allocationRow).not.toBeNull()
    fireEvent.click(within(allocationRow as HTMLElement).getByRole("button", { name: /^edit$/i }))

    await waitFor(() => {
      expect(allocationPanel.getByText(/^edit context: person: alice -> project one$/i)).toBeInTheDocument()
      expect((allocationPanel.getByLabelText(/^target$/i) as HTMLSelectElement).value).toBe("person_1")
      expect((allocationPanel.getByLabelText(/^project$/i) as HTMLSelectElement).value).toBe("project_1")
      expect((allocationPanel.getByLabelText(/^fte % per day$/i) as HTMLInputElement).value).toBe("40")
      expect(allocationPanel.getByRole("button", { name: /^switch to creation context$/i })).toBeInTheDocument()
    })

    fireEvent.click(allocationPanel.getByRole("button", { name: /^switch to creation context$/i }))

    await waitFor(() => {
      expect(allocationPanel.getByText(/^creation context: creating a new allocation\.$/i)).toBeInTheDocument()
      expect((allocationPanel.getByLabelText(/^target$/i) as HTMLSelectElement).value).toBe("")
    })
  })

  it("updates an edited person allocation with PUT when the edited target remains selected", async () => {
    const { fetchMock } = buildMockAPI({
      allocations: [
        {
          id: "allocation_person_1",
          organisation_id: "org_1",
          target_type: "person",
          target_id: "person_1",
          project_id: "project_1",
          start_date: "2026-01-01",
          end_date: "2026-12-31",
          percent: 40
        }
      ]
    })

    render(<App />)

    await waitFor(() => {
      expect(screen.getAllByRole("option", { name: "Alpha Org" }).length).toBeGreaterThan(0)
    })

    const allocationPanel = sectionByHeading(/^allocations$/i)
    const allocationRow = allocationPanel.getByText("person: Alice").closest("tr")
    expect(allocationRow).not.toBeNull()
    fireEvent.click(within(allocationRow as HTMLElement).getByRole("button", { name: /^edit$/i }))

    await waitFor(() => {
      expect(allocationPanel.getByText(/^edit context:/i)).toBeInTheDocument()
    })

    fireEvent.change(allocationPanel.getByLabelText(/^start date$/i), { target: { value: "2026-02-01" } })
    fireEvent.change(allocationPanel.getByLabelText(/^end date$/i), { target: { value: "2026-11-30" } })
    fireEvent.change(allocationPanel.getByLabelText(/^fte % per day$/i), { target: { value: "35" } })
    fireEvent.click(allocationPanel.getByRole("button", { name: /^save allocation$/i }))

    await waitFor(() => {
      const putCall = fetchMock.mock.calls.find(([requestURL, requestInit]) => {
        return String(requestURL).includes("/api/allocations/allocation_person_1")
          && (requestInit as RequestInit | undefined)?.method === "PUT"
      })
      expect(putCall).toBeDefined()
      const payload = JSON.parse(String((putCall?.[1] as RequestInit).body))
      expect(payload).toMatchObject({
        target_type: "person",
        target_id: "person_1",
        project_id: "project_1",
        start_date: "2026-02-01",
        end_date: "2026-11-30",
        percent: 35
      })
    })

    const postCalls = fetchMock.mock.calls.filter(([requestURL, requestInit]) => {
      return String(requestURL).endsWith("/api/allocations")
        && (requestInit as RequestInit | undefined)?.method === "POST"
    })
    expect(postCalls).toHaveLength(0)
  })

  it("deletes allocation immediately without showing a delete strategy prompt", async () => {
    const organisations = [
      { id: "org_1", name: "Org One", hours_per_day: 8, hours_per_week: 40, hours_per_year: 2080 }
    ]
    const persons = [
      { id: "person_1", organisation_id: "org_1", name: "Alice", employment_pct: 100 }
    ]
    const projects = [
      {
        id: "project_1",
        organisation_id: "org_1",
        name: "Project One",
        start_date: "2026-01-01",
        end_date: "2026-12-31",
        estimated_effort_hours: 1000
      }
    ]
    const allocations = [
      {
        id: "allocation_person_1",
        organisation_id: "org_1",
        target_type: "person",
        target_id: "person_1",
        project_id: "project_1",
        start_date: "2026-01-01",
        end_date: "2026-12-31",
        percent: 40
      }
    ]

    const fetchMock = vi.fn(async (input: string | URL | Request, options?: RequestInit) => {
      const url = typeof input === "string" ? input : input.toString()
      const method = options?.method ?? "GET"

      if (url.endsWith("/api/organisations") && method === "GET") {
        return jsonResponse(organisations)
      }
      if (url.endsWith("/api/persons") && method === "GET") {
        return jsonResponse(persons)
      }
      if (url.endsWith("/api/projects") && method === "GET") {
        return jsonResponse(projects)
      }
      if (url.endsWith("/api/groups") && method === "GET") {
        return jsonResponse([])
      }
      if (url.endsWith("/api/allocations") && method === "GET") {
        return jsonResponse(allocations)
      }
      if (url.endsWith("/api/persons/person_1/unavailability") && method === "GET") {
        return jsonResponse([])
      }
      if (url.includes("/api/allocations/") && method === "DELETE") {
        return jsonResponse({})
      }

      return jsonResponse([])
    })
    vi.stubGlobal("fetch", fetchMock)

    render(<App />)

    const allocationsHeading = screen.getByRole("heading", { name: /^allocations$/i })
    const allocationsSection = allocationsHeading.closest("section")
    expect(allocationsSection).not.toBeNull()
    const allocationPanel = within(allocationsSection as HTMLElement)

    await waitFor(() => {
      expect(allocationPanel.getByText("person: Alice")).toBeInTheDocument()
    })

    const personRow = allocationPanel.getByText("person: Alice").closest("tr")
    expect(personRow).not.toBeNull()
    fireEvent.click(within(personRow as HTMLElement).getByRole("button", { name: /^edit$/i }))
    await waitFor(() => {
      expect(allocationPanel.getByText(/^edit context:/i)).toBeInTheDocument()
    })
    fireEvent.click(within(personRow as HTMLElement).getByRole("button", { name: /^delete$/i }))

    await waitFor(() => {
      const deleteCalls = fetchMock.mock.calls.filter((call) => {
        const [callURL, callOptions] = call
        return String(callURL).includes("/api/allocations/")
          && (callOptions as RequestInit | undefined)?.method === "DELETE"
      })
      expect(deleteCalls).toHaveLength(1)
      expect(String(deleteCalls[0][0])).toContain("/api/allocations/allocation_person_1")
      expect(allocationPanel.getByText(/^creation context:/i)).toBeInTheDocument()
    })

    expect(allocationPanel.queryByLabelText(/^delete merge strategy$/i)).not.toBeInTheDocument()
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

  it("shows allocation batch-delete action only when at least two rows are selected", async () => {
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
    const confirmMock = vi.spyOn(window, "confirm").mockReturnValue(true)

    render(<App />)

    await waitFor(() => {
      expect(screen.getAllByRole("option", { name: "Alpha Org" }).length).toBeGreaterThan(0)
    })

    const allocationsPanel = sectionByHeading(/^allocations$/i)
    fireEvent.click(allocationsPanel.getByLabelText(/select allocation allocation_1/i))

    expect(allocationsPanel.queryByRole("button", { name: /^delete selected items$/i })).not.toBeInTheDocument()

    fireEvent.click(allocationsPanel.getByLabelText(/select allocation allocation_2/i))
    expect(allocationsPanel.getByRole("button", { name: /^delete selected items$/i })).toBeInTheDocument()
    expect(confirmMock).not.toHaveBeenCalled()
  })

  it("updates allocation date inputs and toggles row and select-all checkboxes", async () => {
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

    const allocationsPanel = sectionByHeading(/^allocations$/i)
    const startDateInput = allocationsPanel.getByLabelText(/^start date$/i) as HTMLInputElement
    const endDateInput = allocationsPanel.getByLabelText(/^end date$/i) as HTMLInputElement
    fireEvent.change(startDateInput, { target: { value: "2026-02-01" } })
    fireEvent.change(endDateInput, { target: { value: "2026-11-30" } })
    expect(startDateInput.value).toBe("2026-02-01")
    expect(endDateInput.value).toBe("2026-11-30")

    const firstRowCheckbox = allocationsPanel.getByLabelText(/select allocation allocation_1/i) as HTMLInputElement
    fireEvent.click(firstRowCheckbox)
    expect(firstRowCheckbox.checked).toBe(true)
    fireEvent.click(firstRowCheckbox)
    expect(firstRowCheckbox.checked).toBe(false)

    const secondRowCheckbox = allocationsPanel.getByLabelText(/select allocation allocation_2/i) as HTMLInputElement
    const selectAll = allocationsPanel.getByLabelText(/select all allocations/i) as HTMLInputElement
    fireEvent.click(selectAll)
    expect(firstRowCheckbox.checked).toBe(true)
    expect(secondRowCheckbox.checked).toBe(true)
    fireEvent.click(selectAll)
    expect(firstRowCheckbox.checked).toBe(false)
    expect(secondRowCheckbox.checked).toBe(false)
  })

  it("batch-deletes selected allocations after confirmation and resets edit context", async () => {
    const { fetchMock } = buildMockAPI({
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

    const allocationsPanel = sectionByHeading(/^allocations$/i)
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
      expect(allocationsPanel.queryByRole("cell", { name: /person:\s*Alice/i })).not.toBeInTheDocument()
      expect(allocationsPanel.queryByRole("cell", { name: /person:\s*Bob/i })).not.toBeInTheDocument()

      const deleteCalls = fetchMock.mock.calls
        .filter(([requestURL, requestInit]) => {
          return String(requestURL).includes("/api/allocations/")
            && (requestInit as RequestInit | undefined)?.method === "DELETE"
        })
        .map(([requestURL]) => String(requestURL))
      expect(deleteCalls.some((url) => url.includes("/api/allocations/allocation_1"))).toBe(true)
      expect(deleteCalls.some((url) => url.includes("/api/allocations/allocation_2"))).toBe(true)
    })
  })

  it("clears group member group selection when selected groups are batch-deleted", async () => {
    buildMockAPI({
      groups: [
        { id: "group_1", organisation_id: "org_1", name: "Team", member_ids: ["person_1", "person_2"] },
        { id: "group_2", organisation_id: "org_1", name: "Ops", member_ids: ["person_1"] }
      ]
    })
    vi.spyOn(window, "confirm").mockReturnValue(true)

    render(<App />)

    await waitFor(() => {
      expect(screen.getAllByRole("option", { name: "Alpha Org" }).length).toBeGreaterThan(0)
    })

    const groupsPanel = sectionByHeading(/^groups$/i)
    fireEvent.change(groupsPanel.getByLabelText(/^group$/i), { target: { value: "group_1" } })
    fireEvent.change(groupsPanel.getByLabelText(/^person$/i), { target: { value: "person_1" } })

    fireEvent.click(groupsPanel.getByLabelText(/select group team/i))
    fireEvent.click(groupsPanel.getByLabelText(/select group ops/i))
    fireEvent.click(groupsPanel.getByRole("button", { name: /^delete selected items$/i }))

    await waitFor(() => {
      expect(screen.getByText("selected groups deleted")).toBeInTheDocument()
      expect((groupsPanel.getByLabelText(/^group$/i) as HTMLSelectElement).value).toBe("")
    })
  })

  it("toggles report object selection off when the same checkbox is clicked twice", async () => {
    buildMockAPI()

    render(<App />)

    await waitFor(() => {
      expect(screen.getAllByRole("option", { name: "Alpha Org" }).length).toBeGreaterThan(0)
    })

    const reportPanel = sectionByHeading(/^report$/i)
    fireEvent.change(reportPanel.getByLabelText(/^scope$/i), { target: { value: "person" } })

    const personOption = reportPanel.getByRole("checkbox", { name: "Alice (100%)" }) as HTMLInputElement
    expect(personOption.checked).toBe(false)
    fireEvent.click(personOption)
    expect(personOption.checked).toBe(true)
    fireEvent.click(personOption)
    expect(personOption.checked).toBe(false)
  })

  it("runs a project report with no selectable projects and falls back to scope defaults", async () => {
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
      if (path === "/api/reports/availability-load" && method === "POST") {
        return jsonResponse({})
      }
      return jsonResponse([])
    })
    vi.stubGlobal("fetch", fetchMock)

    render(<App />)

    await waitFor(() => {
      expect(screen.getAllByRole("option", { name: "Org" }).length).toBeGreaterThan(0)
    })

    const reportPanel = sectionByHeading(/^report$/i)
    fireEvent.change(reportPanel.getByLabelText(/^scope$/i), { target: { value: "project" } })
    fireEvent.click(reportPanel.getByRole("button", { name: /^run report$/i }))

    await waitFor(() => {
      expect(screen.getByText("report calculated")).toBeInTheDocument()
    })

    const postCalls = fetchMock.mock.calls.filter(([requestURL, requestInit]) => {
      return String(requestURL).endsWith("/api/reports/availability-load")
        && (requestInit as RequestInit | undefined)?.method === "POST"
    })
    expect(postCalls).toHaveLength(1)
    const payload = JSON.parse(String((postCalls[0][1] as RequestInit).body))
    expect(payload).toMatchObject({ scope: "project", ids: [] })
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
        if (Array.isArray(payload.ids) && payload.ids.includes("person_2")) {
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
