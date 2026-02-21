import type { Dispatch, FormEvent, RefObject, SetStateAction } from "react"
import { normalizeAllocationTargetID, normalizeAllocationTargetType } from "../app/helpers"
import type {
  Allocation,
  AllocationFormState,
  AllocationLoadInputType,
  AllocationLoadUnit,
  AllocationMergeStrategy,
  AllocationTargetType,
  Group,
  Person,
  Project
} from "../app/types"

type AllocationsPanelProps = {
  allocationForm: AllocationFormState
  setAllocationForm: Dispatch<SetStateAction<AllocationFormState>>
  allocationFormContextLabel: string
  editingAllocation: Allocation | undefined
  onSwitchToCreateContext: () => void
  onSaveAllocation: (event: FormEvent) => void
  allocationTargetOptions: Array<{ id: string; label: string }>
  isOverallocatedPersonID: (personID: string) => boolean
  projects: Project[]
  allocationFormConflicts: Allocation[]
  allocationFormConflictingSelectedPersonNames: string[]
  allocationFormConflictingSelectedPersonIDs: string[]
  allocationMergeStrategy: AllocationMergeStrategy
  setAllocationMergeStrategy: Dispatch<SetStateAction<AllocationMergeStrategy>>
  allocationFormCanReplaceConflicts: boolean
  selectedAllocationIDs: string[]
  onDeleteSelectedAllocations: () => void
  selectAllAllocationsCheckboxRef: RefObject<HTMLInputElement>
  allocations: Allocation[]
  setSelectedAllocationIDs: Dispatch<SetStateAction<string[]>>
  groups: Group[]
  persons: Person[]
  resolveAllocationPersonIDs: (allocation: Allocation) => string[]
  onSwitchToEditContext: (allocation: Allocation) => void
  onDeleteAllocation: (allocation: Allocation) => void
}

export function AllocationsPanel(props: AllocationsPanelProps) {
  const {
    allocationForm,
    setAllocationForm,
    allocationFormContextLabel,
    editingAllocation,
    onSwitchToCreateContext,
    onSaveAllocation,
    allocationTargetOptions,
    isOverallocatedPersonID,
    projects,
    allocationFormConflicts,
    allocationFormConflictingSelectedPersonNames,
    allocationFormConflictingSelectedPersonIDs,
    allocationMergeStrategy,
    setAllocationMergeStrategy,
    allocationFormCanReplaceConflicts,
    selectedAllocationIDs,
    onDeleteSelectedAllocations,
    selectAllAllocationsCheckboxRef,
    allocations,
    setSelectedAllocationIDs,
    groups,
    persons,
    resolveAllocationPersonIDs,
    onSwitchToEditContext,
    onDeleteAllocation
  } = props
  const groupsByID = new Map(groups.map((group) => [group.id, group]))
  const personsByID = new Map(persons.map((person) => [person.id, person]))
  const projectsByID = new Map(projects.map((project) => [project.id, project]))

  return (
    <section className="panel">
      <h2>Allocations</h2>
      <form className="grid-form" onSubmit={onSaveAllocation}>
        <p>{allocationFormContextLabel}</p>
        {editingAllocation && (
          <div className="actions">
            <button type="button" onClick={onSwitchToCreateContext}>Switch to creation context</button>
          </div>
        )}
        <label>
          Target type
          <select
            value={allocationForm.targetType}
            onChange={(event) => {
              setAllocationForm((current) => ({
                ...current,
                targetType: event.target.value as AllocationTargetType,
                targetID: ""
              }))
            }}
          >
            <option value="person">person</option>
            <option value="group">group</option>
          </select>
        </label>
        <label>
          Target
          <select value={allocationForm.targetID} onChange={(event) => setAllocationForm((current) => ({ ...current, targetID: event.target.value }))}>
            <option value="">Select target</option>
            {allocationTargetOptions.map((target) => (
              <option
                key={target.id}
                value={target.id}
                className={
                  allocationForm.targetType === "person" && isOverallocatedPersonID(target.id)
                    ? "person-overallocated"
                    : undefined
                }
              >
                {target.label}
              </option>
            ))}
          </select>
        </label>
        <label>
          Project
          <select value={allocationForm.projectID} onChange={(event) => setAllocationForm((current) => ({ ...current, projectID: event.target.value }))}>
            <option value="">Select project</option>
            {projects.map((project) => (
              <option key={project.id} value={project.id}>{project.name}</option>
            ))}
          </select>
        </label>
        <label>
          Start date
          <input type="date" value={allocationForm.startDate} onChange={(event) => setAllocationForm((current) => ({ ...current, startDate: event.target.value }))} />
        </label>
        <label>
          End date
          <input type="date" value={allocationForm.endDate} onChange={(event) => setAllocationForm((current) => ({ ...current, endDate: event.target.value }))} />
        </label>
        <label>
          Load value type
          <select
            value={allocationForm.loadInputType}
            onChange={(event) => setAllocationForm((current) => ({ ...current, loadInputType: event.target.value as AllocationLoadInputType }))}
          >
            <option value="fte_pct">FTE % (full-time basis)</option>
            <option value="hours">Hours</option>
          </select>
        </label>
        <label>
          Load unit
          <select
            value={allocationForm.loadUnit}
            onChange={(event) => setAllocationForm((current) => ({ ...current, loadUnit: event.target.value as AllocationLoadUnit }))}
          >
            <option value="day">per day</option>
            <option value="week">per week</option>
            <option value="month">per month</option>
          </select>
        </label>
        <label>
          {allocationForm.loadInputType === "fte_pct"
            ? `FTE % per ${allocationForm.loadUnit}`
            : `Hours per ${allocationForm.loadUnit}`}
          <input
            type="number"
            value={allocationForm.loadValue}
            onChange={(event) => setAllocationForm((current) => ({ ...current, loadValue: event.target.value }))}
          />
        </label>
        {allocationFormConflicts.length > 0 && (
          <>
            <p>{allocationFormConflicts.length} allocation conflict(s) detected for the selected users and timespan.</p>
            {allocationFormConflictingSelectedPersonNames.length > 0 && (
              <p>Affected persons: {allocationFormConflictingSelectedPersonNames.join(", ")}</p>
            )}
            {allocationMergeStrategy === "keep" && allocationFormConflictingSelectedPersonIDs.length > 0 && (
              <p>{allocationFormConflictingSelectedPersonIDs.length} selected user(s) will be excluded from this allocation.</p>
            )}
            <label>
              Merge strategy
              <select
                value={allocationMergeStrategy}
                onChange={(event) => setAllocationMergeStrategy(event.target.value as AllocationMergeStrategy)}
              >
                <option value="stack">stack with existing allocations</option>
                <option value="replace" disabled={!allocationFormCanReplaceConflicts}>replace conflicting allocations</option>
                <option value="keep">keep existing allocations as-is (exclude affected users)</option>
              </select>
            </label>
            {!allocationFormCanReplaceConflicts && (
              <p>Replace is unavailable because some conflicts include users outside the current selection.</p>
            )}
          </>
        )}
        <div className="actions">
          <button type="submit">Save allocation</button>
          {selectedAllocationIDs.length > 1 && (
            <button type="button" onClick={onDeleteSelectedAllocations}>Delete selected items</button>
          )}
        </div>
      </form>

      <table>
        <thead>
          <tr>
            <th>
              <input
                ref={selectAllAllocationsCheckboxRef}
                type="checkbox"
                aria-label="Select all allocations"
                checked={allocations.length > 0 && selectedAllocationIDs.length === allocations.length}
                onChange={(event) => {
                  if (event.target.checked) {
                    setSelectedAllocationIDs(allocations.map((allocation) => allocation.id))
                    return
                  }
                  setSelectedAllocationIDs([])
                }}
              />
            </th>
            <th>Target</th>
            <th>Project</th>
            <th>Start</th>
            <th>End</th>
            <th>FTE % (full-time basis)</th>
            <th />
          </tr>
        </thead>
        <tbody>
          {allocations.map((allocation) => {
            const targetType = normalizeAllocationTargetType(allocation)
            const targetID = normalizeAllocationTargetID(allocation)
            const targetLabel = targetType === "group"
              ? groupsByID.get(targetID)?.name ?? targetID
              : personsByID.get(targetID)?.name ?? targetID
            const hasOverallocatedPerson = resolveAllocationPersonIDs(allocation)
              .some((personID) => isOverallocatedPersonID(personID))
            const project = projectsByID.get(allocation.project_id)
            return (
              <tr key={allocation.id} className={hasOverallocatedPerson ? "person-overallocated" : undefined}>
                <td>
                  <input
                    type="checkbox"
                    aria-label={`Select allocation ${allocation.id}`}
                    checked={selectedAllocationIDs.includes(allocation.id)}
                    onChange={() => {
                      setSelectedAllocationIDs((current) => {
                        if (current.includes(allocation.id)) {
                          return current.filter((id) => id !== allocation.id)
                        }
                        return [...current, allocation.id]
                      })
                    }}
                  />
                </td>
                <td>{targetType}: {targetLabel}</td>
                <td>{project?.name ?? allocation.project_id}</td>
                <td>{allocation.start_date || "-"}</td>
                <td>{allocation.end_date || "-"}</td>
                <td>{allocation.percent}</td>
                <td>
                  <div className="actions">
                    <button type="button" onClick={() => onSwitchToEditContext(allocation)}>Edit</button>
                    <button type="button" onClick={() => onDeleteAllocation(allocation)}>Delete</button>
                  </div>
                </td>
              </tr>
            )
          })}
        </tbody>
      </table>
    </section>
  )
}
