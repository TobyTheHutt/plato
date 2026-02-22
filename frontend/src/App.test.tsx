import { fireEvent, render, screen, waitFor, within } from "@testing-library/react"
import { afterEach, beforeEach, describe, vi } from "vitest"
import App from "./App"
import { jsonResponse } from "./test-utils/mocks"

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

describe("App", () => {
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
    fireEvent.click(within(personRow as HTMLElement).getByRole("button", { name: /^delete$/i }))

    await waitFor(() => {
      const deleteCalls = fetchMock.mock.calls.filter((call) => {
        const [callURL, callOptions] = call
        return String(callURL).includes("/api/allocations/")
          && (callOptions as RequestInit | undefined)?.method === "DELETE"
      })
      expect(deleteCalls).toHaveLength(1)
      expect(String(deleteCalls[0][0])).toContain("/api/allocations/allocation_person_1")
    })

    expect(allocationPanel.queryByLabelText(/^delete merge strategy$/i)).not.toBeInTheDocument()
  })
})
