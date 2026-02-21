import { describe, expect, it } from "vitest"
import { showAvailabilityMetrics, showProjectMetrics, type ReportScope } from "./reportColumns"

const NON_PROJECT_SCOPES: ReportScope[] = ["organisation", "person", "group"]

describe("reportColumns", () => {
  it("shows availability metrics for non-project scopes", () => {
    for (const scope of NON_PROJECT_SCOPES) {
      expect(showAvailabilityMetrics(scope)).toBe(true)
      expect(showProjectMetrics(scope)).toBe(false)
    }
  })

  it("shows project metrics only for project scope", () => {
    expect(showAvailabilityMetrics("project")).toBe(false)
    expect(showProjectMetrics("project")).toBe(true)
  })
})
