import { afterEach, describe, expect, it, vi } from "vitest"
import { buildMockAPI, textResponse } from "./mocks"

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

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

  it("falls back to text array responses for unknown routes", async () => {
    buildMockAPI()

    const response = await fetch("http://localhost/api/unknown")
    expect(response.status).toBe(200)
    const body = await response.text()
    expect(body).toBe("[]")
  })
})
