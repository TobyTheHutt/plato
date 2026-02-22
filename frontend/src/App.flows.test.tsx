import { fireEvent, render, screen, waitFor, within } from "@testing-library/react"
import { afterEach, describe, vi } from "vitest"
import App from "./App"
import { buildMockAPI } from "./test-utils/mocks"

/**
 * Scope: multi-step integration workflows.
 * These tests cover cross-panel journeys and verify milestone outcomes.
 * Focused behavior and error-path assertions live in App.test.tsx.
 */

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

describe("App multi-step flows", () => {
  it("covers management and report actions across sections", async () => {
    const { fetchMock, restore } = buildMockAPI()

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
    restore()
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

    fireEvent.click(reportPanel.getByRole("button", { name: /^hide entries$/i }))
    await waitFor(() => {
      expect(reportPanel.getByRole("button", { name: /^show 2 entries$/i })).toBeInTheDocument()
      expect(reportPanel.queryByRole("cell", { name: "Alice (100%)" })).not.toBeInTheDocument()
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

  it("runs report workflow across organisation and project scopes", async () => {
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
      expect(screen.getByText("report calculated")).toBeInTheDocument()
      expect(reportPanel.getByRole("cell", { name: "Apollo" })).toBeInTheDocument()
    })

    const reportCalls = fetchMock.mock.calls.filter(([requestURL, requestInit]) => {
      return String(requestURL).endsWith("/api/reports/availability-load")
        && (requestInit as RequestInit | undefined)?.method === "POST"
    })
    expect(reportCalls).toHaveLength(2)
  })
})
