export type ReportScope = "organisation" | "person" | "group" | "project"

export function showAvailabilityMetrics(scope: ReportScope): boolean {
  return scope !== "project"
}

export function showProjectMetrics(scope: ReportScope): boolean {
  return scope === "project"
}
