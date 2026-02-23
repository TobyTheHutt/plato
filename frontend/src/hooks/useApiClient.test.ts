import { renderHook } from "@testing-library/react"
import { afterEach, describe, expect, it, vi } from "vitest"
import { useApiClient } from "./useApiClient"

type MockResponse = Pick<Response, "ok" | "status" | "text" | "json">

function makeResponse(status: number, text: string): MockResponse {
  return {
    ok: status >= 200 && status < 300,
    status,
    text: async () => text,
    json: async () => {
      return JSON.parse(text)
    }
  }
}

describe("useApiClient", () => {
  afterEach(() => {
    vi.unstubAllEnvs()
    vi.unstubAllGlobals()
    vi.restoreAllMocks()
  })

  it("applies auth headers and allows explicit header overrides", async () => {
    vi.stubEnv("VITE_DEV_USER_ID", "dev-user-42")
    const fetchMock = vi.fn(async () => makeResponse(200, JSON.stringify({ ok: true })))
    vi.stubGlobal("fetch", fetchMock)

    const { result } = renderHook(() => useApiClient({
      role: "org_admin",
      selectedOrganisationID: "org_1",
      canUseNetwork: true
    }))

    const payload = await result.current.requestJSON<{ ok: boolean }>(
      "/api/check",
      { method: "GET", headers: { "X-Role": "override-role" } },
      "org_2"
    )

    expect(payload).toEqual({ ok: true })
    expect(fetchMock).toHaveBeenCalledTimes(1)

    const firstCall = fetchMock.mock.calls[0] as unknown[] | undefined
    expect(firstCall).toBeDefined()

    const requestURL = firstCall?.[0] as string | URL | Request | undefined
    const requestInit = firstCall?.[1] as RequestInit | undefined
    expect(String(requestURL)).toBe("http://localhost:8070/api/check")
    expect(requestInit?.method).toBe("GET")

    const headers = (requestInit?.headers ?? {}) as Record<string, string>
    expect(headers["Content-Type"]).toBe("application/json")
    expect(headers["X-Org-ID"]).toBe("org_2")
    expect(headers["X-User-ID"]).toBe("dev-user-42")
    expect(headers["X-Role"]).toBe("override-role")
  })

  it("omits user and organisation headers when values are empty", async () => {
    vi.stubEnv("VITE_DEV_USER_ID", "   ")
    const fetchMock = vi.fn(async () => makeResponse(200, JSON.stringify({ ok: true })))
    vi.stubGlobal("fetch", fetchMock)

    const { result } = renderHook(() => useApiClient({
      role: "org_user",
      selectedOrganisationID: "",
      canUseNetwork: true
    }))

    await result.current.requestJSON<{ ok: boolean }>("/api/check", { method: "GET" }, "")
    const firstCall = fetchMock.mock.calls[0] as unknown[] | undefined
    expect(firstCall).toBeDefined()

    const requestInit = firstCall?.[1] as RequestInit | undefined
    const headers = (requestInit?.headers ?? {}) as Record<string, string>

    expect(headers["X-Role"]).toBe("org_user")
    expect(headers["X-User-ID"]).toBeUndefined()
    expect(headers["X-Org-ID"]).toBeUndefined()
  })

  it("fails requests when fetch is unavailable", async () => {
    const fetchMock = vi.fn()
    vi.stubGlobal("fetch", fetchMock)

    const { result } = renderHook(() => useApiClient({
      role: "org_admin",
      selectedOrganisationID: "org_1",
      canUseNetwork: false
    }))

    await expect(result.current.requestJSON("/api/check", { method: "GET" })).rejects.toThrow("fetch is not available")
    await expect(result.current.requestNoContent("/api/check", { method: "DELETE" })).rejects.toThrow("fetch is not available")
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it("handles no-content, invalid json, and failed status branches for requestJSON", async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(makeResponse(204, ""))
      .mockResolvedValueOnce(makeResponse(200, "not-json"))
      .mockResolvedValueOnce(makeResponse(500, JSON.stringify({ detail: "missing" })))
      .mockResolvedValueOnce(makeResponse(500, JSON.stringify({ error: "denied" })))
      .mockResolvedValueOnce(makeResponse(200, ""))
      .mockResolvedValueOnce(makeResponse(200, ""))
    vi.stubGlobal("fetch", fetchMock)

    const { result } = renderHook(() => useApiClient({
      role: "org_admin",
      selectedOrganisationID: "org_1",
      canUseNetwork: true
    }))

    await expect(result.current.requestJSON("/api/no-content", { method: "GET" })).rejects.toThrow(
      "request returned no content for /api/no-content"
    )
    await expect(result.current.requestJSON("/api/invalid", { method: "GET" })).rejects.toThrow(
      "request returned invalid json for /api/invalid"
    )
    await expect(result.current.requestJSON("/api/failure", { method: "GET" })).rejects.toThrow(
      "request failed with status 500"
    )
    await expect(result.current.requestJSON("/api/failure-with-error", { method: "GET" })).rejects.toThrow("denied")
    await expect(result.current.requestJSON("/api/empty", { method: "GET" })).rejects.toThrow(
      "request returned empty body for /api/empty"
    )

    const defaultPayload = await result.current.requestJSON<string[]>("/api/empty-default", { method: "GET" }, undefined, [])
    expect(defaultPayload).toEqual([])
  })

  it("handles response body branches for requestNoContent", async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(makeResponse(204, ""))
      .mockResolvedValueOnce(makeResponse(500, ""))
      .mockResolvedValueOnce(makeResponse(500, JSON.stringify({ error: "locked" })))
      .mockResolvedValueOnce(makeResponse(500, JSON.stringify({ reason: "missing" })))
      .mockResolvedValueOnce(makeResponse(500, "service unavailable"))
      .mockResolvedValueOnce(makeResponse(500, "   "))
    vi.stubGlobal("fetch", fetchMock)

    const { result } = renderHook(() => useApiClient({
      role: "org_admin",
      selectedOrganisationID: "org_1",
      canUseNetwork: true
    }))

    await expect(result.current.requestNoContent("/api/ok", { method: "DELETE" })).resolves.toBeUndefined()
    await expect(result.current.requestNoContent("/api/failure-empty", { method: "DELETE" })).rejects.toThrow(
      "request failed with status 500"
    )
    await expect(result.current.requestNoContent("/api/failure-json", { method: "DELETE" })).rejects.toThrow("locked")
    await expect(result.current.requestNoContent("/api/failure-json-no-error", { method: "DELETE" })).rejects.toThrow(
      "request failed with status 500"
    )
    await expect(result.current.requestNoContent("/api/failure-text", { method: "DELETE" })).rejects.toThrow(
      "request failed with status 500: service unavailable"
    )
    await expect(result.current.requestNoContent("/api/failure-whitespace", { method: "DELETE" })).rejects.toThrow(
      "request failed with status 500"
    )
  })
})
