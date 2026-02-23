import { afterEach, describe, expect, it, vi } from "vitest"
import { buildMockAPI, textResponse } from "./mocks"

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

async function requestJSON(path: string, init?: RequestInit) {
  const response = await fetch(`http://localhost${path}`, init)
  return {
    response,
    body: await response.json()
  }
}

describe("test utils mocks", () => {
  it("surfaces json parsing errors for invalid text responses", async () => {
    const response = textResponse("not-json", 500)
    expect(response.ok).toBe(false)
    expect(response.status).toBe(500)
    await expect(response.json()).rejects.toBeInstanceOf(SyntaxError)
  })

  it("returns not found errors for missing group member routes", async () => {
    buildMockAPI()

    const addResponse = await fetch("http://localhost/api/groups/missing-group/members", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ person_id: "person_1" })
    })
    expect(addResponse.status).toBe(404)
    expect(await addResponse.json()).toEqual({ error: "group not found" })

    const removeResponse = await fetch("http://localhost/api/groups/missing-group/members/person_1", {
      method: "DELETE"
    })
    expect(removeResponse.status).toBe(404)
    expect(await removeResponse.json()).toEqual({ error: "group not found" })
  })

  it("returns not found for allocation update on unknown id", async () => {
    buildMockAPI()

    const response = await fetch("http://localhost/api/allocations/missing-allocation", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ percent: 55 })
    })
    expect(response.status).toBe(404)
    expect(await response.json()).toEqual({ error: "allocation not found" })
  })

  it("covers create defaults when payload values are omitted", async () => {
    const { store } = buildMockAPI({
      organisations: [],
      persons: [],
      projects: [],
      groups: [],
      allocations: [],
      personUnavailability: []
    })

    const personCreate = await requestJSON("/api/persons", {
      method: "POST",
      body: JSON.stringify({})
    })
    expect(personCreate.body).toMatchObject({
      organisation_id: "org_1",
      name: "",
      employment_pct: 0
    })

    const personUnavailabilityCreate = await requestJSON(`/api/persons/${personCreate.body.id}/unavailability`, {
      method: "POST",
      body: JSON.stringify({})
    })
    expect(personUnavailabilityCreate.body).toMatchObject({
      organisation_id: "org_1",
      person_id: personCreate.body.id,
      date: "",
      hours: 0
    })

    const projectCreate = await requestJSON("/api/projects", {
      method: "POST",
      body: JSON.stringify({})
    })
    expect(projectCreate.body).toMatchObject({
      organisation_id: "org_1",
      name: "",
      start_date: "2026-01-01",
      end_date: "2026-12-31",
      estimated_effort_hours: 0
    })

    const groupCreate = await requestJSON("/api/groups", {
      method: "POST",
      body: JSON.stringify({})
    })
    expect(groupCreate.body).toMatchObject({
      organisation_id: "org_1",
      name: "",
      member_ids: []
    })

    const allocationCreate = await requestJSON("/api/allocations", {
      method: "POST",
      body: JSON.stringify({})
    })
    expect(allocationCreate.body).toMatchObject({
      organisation_id: "org_1",
      target_type: "person",
      target_id: "",
      project_id: "",
      start_date: "",
      end_date: "",
      percent: 0
    })

    const organisationsFetch = await fetch(new URL("http://localhost/api/organisations"))
    expect(await organisationsFetch.json()).toEqual([])

    const organisationCreate = await fetch(new URL("http://localhost/api/organisations"), {
      method: "POST",
      body: JSON.stringify({})
    })
    expect(await organisationCreate.json()).toMatchObject({
      id: "org_2",
      name: "",
      hours_per_day: 0,
      hours_per_week: 0,
      hours_per_year: 0
    })

    const report = await requestJSON("/api/reports/availability-load", {
      method: "POST",
      body: JSON.stringify({})
    })
    expect(report.body).toEqual({ buckets: store.reportBuckets })
  })

  it("covers update fallbacks and missing-resource responses", async () => {
    buildMockAPI()

    const updateOrganisation = await requestJSON("/api/organisations/org_1", {
      method: "PUT",
      body: JSON.stringify({})
    })
    expect(updateOrganisation.body).toMatchObject({
      id: "org_1",
      name: "Alpha Org",
      hours_per_day: 8,
      hours_per_week: 40,
      hours_per_year: 2080
    })

    const updatePersonNoChange = await requestJSON("/api/persons/person_1", {
      method: "PUT",
      body: JSON.stringify({})
    })
    expect(updatePersonNoChange.body).toMatchObject({
      id: "person_1",
      name: "Alice",
      employment_pct: 100
    })

    const updatePersonWithChange = await requestJSON("/api/persons/person_1", {
      method: "PUT",
      body: JSON.stringify({ employment_effective_from_month: "2026-02" })
    })
    expect(updatePersonWithChange.body.employment_changes).toContainEqual({
      effective_month: "2026-02",
      employment_pct: 100
    })

    const updateProject = await requestJSON("/api/projects/project_1", {
      method: "PUT",
      body: JSON.stringify({})
    })
    expect(updateProject.body).toMatchObject({
      id: "project_1",
      name: "Apollo",
      start_date: "2026-01-01",
      end_date: "2026-12-31",
      estimated_effort_hours: 1000
    })

    const updateGroup = await requestJSON("/api/groups/group_1", {
      method: "PUT",
      body: JSON.stringify({ member_ids: "not-an-array" })
    })
    expect(updateGroup.body).toMatchObject({
      id: "group_1",
      name: "Team",
      member_ids: ["person_1"]
    })

    const updateAllocation = await requestJSON("/api/allocations/allocation_1", {
      method: "PUT",
      body: JSON.stringify({})
    })
    expect(updateAllocation.body).toMatchObject({
      id: "allocation_1",
      target_type: "person",
      target_id: "person_1",
      project_id: "project_1",
      start_date: "2026-01-01",
      end_date: "2026-12-31",
      percent: 20
    })

    const missingOrganisation = await requestJSON("/api/organisations/missing", {
      method: "PUT",
      body: JSON.stringify({})
    })
    expect(missingOrganisation.response.status).toBe(404)
    expect(missingOrganisation.body).toEqual({ error: "organisation not found" })

    const missingPerson = await requestJSON("/api/persons/missing", {
      method: "PUT",
      body: JSON.stringify({})
    })
    expect(missingPerson.response.status).toBe(404)
    expect(missingPerson.body).toEqual({ error: "person not found" })

    const missingProject = await requestJSON("/api/projects/missing", {
      method: "PUT",
      body: JSON.stringify({})
    })
    expect(missingProject.response.status).toBe(404)
    expect(missingProject.body).toEqual({ error: "project not found" })

    const missingGroup = await requestJSON("/api/groups/missing", {
      method: "PUT",
      body: JSON.stringify({})
    })
    expect(missingGroup.response.status).toBe(404)
    expect(missingGroup.body).toEqual({ error: "group not found" })

    const missingAllocation = await requestJSON("/api/allocations/missing", {
      method: "PUT",
      body: JSON.stringify({})
    })
    expect(missingAllocation.response.status).toBe(404)
    expect(missingAllocation.body).toEqual({ error: "allocation not found" })
  })

  it("covers delete cascades and group member branch behavior", async () => {
    const { store } = buildMockAPI({
      groups: [
        { id: "group_1", organisation_id: "org_1", name: "Team", member_ids: ["person_1", "person_2"] }
      ],
      allocations: [
        {
          id: "allocation_1",
          organisation_id: "org_1",
          target_type: "person",
          target_id: "person_2",
          project_id: "project_1",
          start_date: "2026-01-01",
          end_date: "2026-12-31",
          percent: 20
        },
        {
          id: "allocation_2",
          organisation_id: "org_1",
          target_type: "group",
          target_id: "group_1",
          project_id: "project_1",
          start_date: "2026-01-01",
          end_date: "2026-12-31",
          percent: 30
        }
      ],
      personUnavailability: [
        {
          id: "entry_1",
          organisation_id: "org_1",
          person_id: "person_2",
          date: "2026-01-10",
          hours: 4
        }
      ]
    })

    const addMember = await requestJSON("/api/groups/group_1/members", {
      method: "POST",
      body: JSON.stringify({ person_id: "person_3" })
    })
    expect(addMember.body.member_ids).toContain("person_3")

    const duplicateMember = await requestJSON("/api/groups/group_1/members", {
      method: "POST",
      body: JSON.stringify({ person_id: "person_3" })
    })
    expect(duplicateMember.body.member_ids.filter((memberID: string) => memberID === "person_3")).toHaveLength(1)

    const emptyMember = await requestJSON("/api/groups/group_1/members", {
      method: "POST",
      body: JSON.stringify({})
    })
    // buildMockAPI currently defaults missing person_id to an empty string.
    expect(emptyMember.body.member_ids).toContain("")

    const removeMember = await requestJSON("/api/groups/group_1/members/person_3", {
      method: "DELETE"
    })
    expect(removeMember.body.member_ids).not.toContain("person_3")

    await requestJSON("/api/persons/person_2/unavailability/entry_1", { method: "DELETE" })
    expect(store.personUnavailability).toEqual([])

    await requestJSON("/api/persons/person_2", { method: "DELETE" })
    expect(store.persons.find((entry) => entry.id === "person_2")).toBeUndefined()
    expect(store.groups[0].member_ids).not.toContain("person_2")
    expect(store.allocations.find((entry) => entry.target_id === "person_2")).toBeUndefined()

    await requestJSON("/api/groups/group_1", { method: "DELETE" })
    expect(store.groups).toHaveLength(0)
    expect(store.allocations.find((entry) => entry.target_id === "group_1")).toBeUndefined()

    await requestJSON("/api/projects/project_1", { method: "DELETE" })
    expect(store.projects).toHaveLength(0)
    expect(store.allocations).toHaveLength(0)

    await requestJSON("/api/organisations/org_1", { method: "DELETE" })
    expect(store.organisations).toEqual([])
  })

  it("falls back to text array responses for unknown routes", async () => {
    buildMockAPI()

    const response = await fetch("http://localhost/api/unknown")
    expect(response.status).toBe(200)
    const body = await response.text()
    expect(body).toBe("[]")
  })
})
