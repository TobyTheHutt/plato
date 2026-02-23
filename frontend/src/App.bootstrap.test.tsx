import { render, screen, waitFor } from "@testing-library/react"
import { afterEach, describe, it, vi } from "vitest"
import App from "./App"

function makeAbortError(): Error {
  const error = new Error("request aborted")
  error.name = "AbortError"
  return error
}

describe("App bootstrap loading", () => {
  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
    vi.restoreAllMocks()
  })

  it("does not show an error for intentional request cancellation", async () => {
    const fetchMock = vi.fn(async () => {
      throw makeAbortError()
    })
    vi.stubGlobal("fetch", fetchMock)

    render(<App />)

    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalled()
    })
    await waitFor(() => {
      expect(screen.queryByRole("alert")).not.toBeInTheDocument()
    })
  })

})
