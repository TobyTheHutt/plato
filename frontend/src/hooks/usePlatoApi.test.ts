import { renderHook } from "@testing-library/react"
import { afterEach, describe, expect, it, vi } from "vitest"
import { jsonResponse, textResponse } from "../test-utils/mocks"
import { usePlatoApi } from "./usePlatoApi"

function asURL(input: string | URL | Request): string {
  if (typeof input === "string") {
    return input
  }
  if (input instanceof Request) {
    return input.url
  }
  return input.toString()
}

describe("usePlatoApi", () => {
  afterEach(() => {
    vi.unstubAllGlobals()
    vi.restoreAllMocks()
  })

  it("returns empty organisation scoped data when no organisation is selected", async () => {
    const fetchMock = vi.fn()
    vi.stubGlobal("fetch", fetchMock)

    const { result } = renderHook(() => usePlatoApi({
      role: "org_admin",
      selectedOrganisationID: "",
      canUseNetwork: true
    }))

    const data = await result.current.fetchOrganisationScopedData("")
    expect(data).toEqual({
      persons: [],
      projects: [],
      groups: [],
      allocations: [],
      personUnavailability: []
    })
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it("surfaces person specific unavailability load errors", async () => {
    const fetchMock = vi.fn(async (input: string | URL | Request, options?: RequestInit) => {
      const url = asURL(input)
      const method = options?.method ?? "GET"

      if (url.endsWith("/api/persons") && method === "GET") {
        return jsonResponse([
          { id: "person_1", organisation_id: "org_1", name: "Alice", employment_pct: 100 },
          { id: "person_2", organisation_id: "org_1", name: "Bob", employment_pct: 80 }
        ])
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
        return jsonResponse([{ id: "entry_1", organisation_id: "org_1", person_id: "person_1", date: "2026-01-03", hours: 4 }])
      }
      if (url.endsWith("/api/persons/person_2/unavailability") && method === "GET") {
        return textResponse(JSON.stringify({ error: "permission denied" }), 500)
      }

      return jsonResponse([])
    })
    vi.stubGlobal("fetch", fetchMock)

    const { result } = renderHook(() => usePlatoApi({
      role: "org_admin",
      selectedOrganisationID: "org_1",
      canUseNetwork: true
    }))

    await expect(result.current.fetchOrganisationScopedData("org_1")).rejects.toThrow(
      "failed to load unavailability for 1 person(s): person_2: permission denied"
    )
  })

  it("reports partial unavailability create failures with created and failed counts", async () => {
    const fetchMock = vi.fn(async (input: string | URL | Request, options?: RequestInit) => {
      const url = asURL(input)
      const method = options?.method ?? "GET"

      if (url.endsWith("/api/persons/person_1/unavailability") && method === "POST") {
        return jsonResponse({
          id: "entry_1",
          organisation_id: "org_1",
          person_id: "person_1",
          date: "2026-01-02",
          hours: 8
        })
      }
      if (url.endsWith("/api/persons/person_2/unavailability") && method === "POST") {
        return textResponse(JSON.stringify({ error: "blocked" }), 500)
      }

      return jsonResponse([])
    })
    vi.stubGlobal("fetch", fetchMock)

    const { result } = renderHook(() => usePlatoApi({
      role: "org_admin",
      selectedOrganisationID: "org_1",
      canUseNetwork: true
    }))

    await expect(result.current.createPersonUnavailabilityEntriesRequest([
      { personID: "person_1", date: "2026-01-02", hours: 8 },
      { personID: "person_2", date: "2026-01-03", hours: 8 }
    ])).rejects.toThrow(
      "created 1 of 2 unavailability entries. failed 1: person_2 on 2026-01-03: blocked"
    )
  })
})
