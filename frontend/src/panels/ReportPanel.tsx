import type { Dispatch, FormEvent, SetStateAction } from "react"
import {
  formatHours,
  isExpandableReportPeriodRow,
  reportDetailToggleLabel,
  reportUtilizationForDisplay
} from "../app/helpers"
import type { Person, ReportGranularity, ReportObjectResult, ReportTableRow } from "../app/types"
import type { ReportScope } from "../reportColumns"

type ReportPanelProps = {
  reportScope: ReportScope
  setReportScope: Dispatch<SetStateAction<ReportScope>>
  setReportIDs: Dispatch<SetStateAction<string[]>>
  setReportResults: Dispatch<SetStateAction<ReportObjectResult[]>>
  onRunReport: (event: FormEvent) => void
  reportFromDate: string
  setReportFromDate: (value: string) => void
  reportToDate: string
  setReportToDate: (value: string) => void
  reportGranularity: ReportGranularity
  setReportGranularity: Dispatch<SetStateAction<ReportGranularity>>
  selectableReportItems: Array<{ id: string; label: string }>
  isOverallocatedPersonID: (personID: string) => boolean
  reportIDs: string[]
  onToggleReportID: (id: string) => void
  showReportObjectColumn: boolean
  showAvailabilityColumns: boolean
  showProjectColumns: boolean
  visibleReportRows: ReportTableRow[]
  expandedReportPeriodSet: ReadonlySet<string>
  personByID: Map<string, Person>
  onToggleReportPeriodDetails: (periodStart: string) => void
}

export function ReportPanel(props: ReportPanelProps) {
  const {
    reportScope,
    setReportScope,
    setReportIDs,
    setReportResults,
    onRunReport,
    reportFromDate,
    setReportFromDate,
    reportToDate,
    setReportToDate,
    reportGranularity,
    setReportGranularity,
    selectableReportItems,
    isOverallocatedPersonID,
    reportIDs,
    onToggleReportID,
    showReportObjectColumn,
    showAvailabilityColumns,
    showProjectColumns,
    visibleReportRows,
    expandedReportPeriodSet,
    personByID,
    onToggleReportPeriodDetails
  } = props
  const reportIDSet = new Set(reportIDs)

  return (
    <section className="panel">
      <h2>Report</h2>
      <form className="grid-form" onSubmit={onRunReport}>
        <label>
          Scope
          <select
            value={reportScope}
            onChange={(event) => {
              setReportScope(event.target.value as ReportScope)
              setReportIDs([])
              setReportResults([])
            }}
          >
            <option value="organisation">organisation</option>
            <option value="person">person</option>
            <option value="group">group</option>
            <option value="project">project</option>
          </select>
        </label>

        <label>
          From
          <input type="date" value={reportFromDate} onChange={(event) => setReportFromDate(event.target.value)} />
        </label>

        <label>
          To
          <input type="date" value={reportToDate} onChange={(event) => setReportToDate(event.target.value)} />
        </label>

        <label>
          Granularity
          <select value={reportGranularity} onChange={(event) => setReportGranularity(event.target.value as ReportGranularity)}>
            <option value="day">day</option>
            <option value="week">week</option>
            <option value="month">month</option>
            <option value="year">year</option>
          </select>
        </label>

        {reportScope !== "organisation" && (
          <fieldset>
            <legend>Scope IDs</legend>
            <div className="chips">
              {selectableReportItems.map((entry) => (
                <label
                  key={entry.id}
                  className={reportScope === "person" && isOverallocatedPersonID(entry.id) ? "person-overallocated" : undefined}
                >
                  <input type="checkbox" checked={reportIDSet.has(entry.id)} onChange={() => onToggleReportID(entry.id)} />
                  {entry.label}
                </label>
              ))}
            </div>
          </fieldset>
        )}

        <div className="actions">
          <button type="submit">Run report</button>
        </div>
      </form>

      <table>
        <thead>
          <tr>
            <th>Period start</th>
            {showReportObjectColumn && <th>Object</th>}
            {showAvailabilityColumns && <th>Availability hours</th>}
            {showAvailabilityColumns && <th>Load hours</th>}
            {showProjectColumns && <th>Project load hours</th>}
            {showProjectColumns && <th>Project estimation hours</th>}
            {showAvailabilityColumns && <th>Free hours</th>}
            {showAvailabilityColumns && <th>Utilization %</th>}
            {showProjectColumns && <th>Project completion %</th>}
          </tr>
        </thead>
        <tbody>
          {visibleReportRows.map((row) => {
            const isExpandableRow = isExpandableReportPeriodRow(row)
            const isExpanded = expandedReportPeriodSet.has(row.periodStart)
            const isOverallocatedReportPerson = reportScope === "person" && isOverallocatedPersonID(row.objectID)
            const utilizationForDisplay = reportUtilizationForDisplay(row, reportScope, personByID)
            const rowClassNames = [
              row.isDetail ? "report-period-detail-row" : "report-period-summary-row",
              row.isTotal ? "report-total-row" : "",
              isOverallocatedReportPerson ? "person-overallocated" : ""
            ]
              .filter((className) => className !== "")
              .join(" ")

            return (
              <tr key={row.id} className={rowClassNames}>
                <td className={row.isDetail ? "report-period-empty-cell" : "report-period-start-cell"}>
                  {!row.isDetail && (
                    <div className="report-period-start-content">
                      <span>{row.periodStart}</span>
                      {!showReportObjectColumn && isExpandableRow && (
                        <button
                          type="button"
                          className="report-period-toggle-button"
                          onClick={() => onToggleReportPeriodDetails(row.periodStart)}
                        >
                          {reportDetailToggleLabel(row, isExpanded)}
                        </button>
                      )}
                    </div>
                  )}
                </td>
                {showReportObjectColumn && (
                  <td>
                    {isExpandableRow ? (
                      <div className="report-period-object-cell">
                        <span>{row.objectLabel}</span>
                        <button
                          type="button"
                          className="report-period-toggle-button"
                          onClick={() => onToggleReportPeriodDetails(row.periodStart)}
                        >
                          {reportDetailToggleLabel(row, isExpanded)}
                        </button>
                      </div>
                    ) : (
                      row.objectLabel
                    )}
                  </td>
                )}
                {showAvailabilityColumns && <td>{formatHours(row.bucket.availability_hours)}</td>}
                {showAvailabilityColumns && <td>{formatHours(row.bucket.load_hours)}</td>}
                {showProjectColumns && <td>{formatHours(row.bucket.project_load_hours)}</td>}
                {showProjectColumns && <td>{formatHours(row.bucket.project_estimation_hours)}</td>}
                {showAvailabilityColumns && <td>{formatHours(row.bucket.free_hours)}</td>}
                {showAvailabilityColumns && <td>{formatHours(utilizationForDisplay)}</td>}
                {showProjectColumns && <td>{formatHours(row.bucket.project_completion_pct)}</td>}
              </tr>
            )
          })}
        </tbody>
      </table>
    </section>
  )
}
