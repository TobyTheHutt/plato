import type { Dispatch, FormEvent, SetStateAction } from "react"
import type { OrganisationFormState, WorkingTimeUnit } from "../app/types"

type OrganisationPanelProps = {
  organisationForm: OrganisationFormState
  setOrganisationForm: Dispatch<SetStateAction<OrganisationFormState>>
  selectedOrganisationID: string
  onCreate: (event: FormEvent) => void
  onUpdate: () => void
  onDelete: () => void
}

export function OrganisationPanel(props: OrganisationPanelProps) {
  const {
    organisationForm,
    setOrganisationForm,
    selectedOrganisationID,
    onCreate,
    onUpdate,
    onDelete
  } = props

  return (
    <section className="panel">
      <h2>Organisation</h2>
      <form className="grid-form" onSubmit={onCreate}>
        <label>
          Name
          <input value={organisationForm.name} onChange={(event) => setOrganisationForm((current) => ({ ...current, name: event.target.value }))} />
        </label>
        <label>
          Working hours
          <input
            type="number"
            value={organisationForm.workingTimeValue}
            onChange={(event) => setOrganisationForm((current) => ({ ...current, workingTimeValue: event.target.value }))}
          />
        </label>
        <label>
          Working hours unit
          <select
            value={organisationForm.workingTimeUnit}
            onChange={(event) => setOrganisationForm((current) => ({ ...current, workingTimeUnit: event.target.value as WorkingTimeUnit }))}
          >
            <option value="day">daily</option>
            <option value="week">weekly</option>
            <option value="month">monthly</option>
            <option value="year">yearly</option>
          </select>
        </label>
        <div className="actions">
          <button type="submit">Create organisation</button>
          <button type="button" disabled={!selectedOrganisationID} onClick={onUpdate}>
            Update selected organisation
          </button>
          <button type="button" disabled={!selectedOrganisationID} onClick={onDelete}>
            Delete selected organisation
          </button>
        </div>
      </form>
    </section>
  )
}
