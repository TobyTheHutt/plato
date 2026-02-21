import type { Dispatch, FormEvent, RefObject, SetStateAction } from "react"
import type { Person, PersonFormState } from "../app/types"

type PersonsPanelProps = {
  personForm: PersonFormState
  setPersonForm: Dispatch<SetStateAction<PersonFormState>>
  personFormContextLabel: string
  editingPerson: Person | undefined
  onSwitchToCreateContext: () => void
  onSavePerson: (event: FormEvent) => void
  selectedPersonIDs: string[]
  onDeleteSelectedPersons: () => void
  selectAllPersonsCheckboxRef: RefObject<HTMLInputElement>
  persons: Person[]
  setSelectedPersonIDs: Dispatch<SetStateAction<string[]>>
  isOverallocatedPersonID: (personID: string) => boolean
  onSwitchToEditContext: (person: Person) => void
  onDeletePerson: (personID: string) => void
}

export function PersonsPanel(props: PersonsPanelProps) {
  const {
    personForm,
    setPersonForm,
    personFormContextLabel,
    editingPerson,
    onSwitchToCreateContext,
    onSavePerson,
    selectedPersonIDs,
    onDeleteSelectedPersons,
    selectAllPersonsCheckboxRef,
    persons,
    setSelectedPersonIDs,
    isOverallocatedPersonID,
    onSwitchToEditContext,
    onDeletePerson
  } = props
  const selectedPersonIDSet = new Set(selectedPersonIDs)

  return (
    <section className="panel">
      <h2>Persons</h2>
      <form className="grid-form" onSubmit={onSavePerson}>
        <p>{personFormContextLabel}</p>
        {editingPerson && (
          <div className="actions">
            <button type="button" onClick={onSwitchToCreateContext}>Switch to creation context</button>
          </div>
        )}
        <label>
          Name
          <input value={personForm.name} onChange={(event) => setPersonForm((current) => ({ ...current, name: event.target.value }))} />
        </label>
        <label>
          Employment percent
          <input type="number" value={personForm.employmentPct} onChange={(event) => setPersonForm((current) => ({ ...current, employmentPct: event.target.value }))} />
        </label>
        {personForm.id && (
          <label>
            Effective from month
            <input
              type="month"
              value={personForm.employmentEffectiveFromMonth}
              onChange={(event) => {
                setPersonForm((current) => ({ ...current, employmentEffectiveFromMonth: event.target.value }))
              }}
            />
          </label>
        )}
        <div className="actions">
          <button type="submit">Save person</button>
          {selectedPersonIDs.length > 1 && (
            <button type="button" onClick={onDeleteSelectedPersons}>Delete selected items</button>
          )}
        </div>
      </form>
      <table>
        <thead>
          <tr>
            <th>
              <input
                ref={selectAllPersonsCheckboxRef}
                type="checkbox"
                aria-label="Select all persons"
                checked={persons.length > 0 && selectedPersonIDs.length === persons.length}
                onChange={(event) => {
                  if (event.target.checked) {
                    setSelectedPersonIDs(persons.map((person) => person.id))
                    return
                  }
                  setSelectedPersonIDs([])
                }}
              />
            </th>
            <th>Name</th>
            <th>Employment %</th>
            <th>Employment changes</th>
            <th />
          </tr>
        </thead>
        <tbody>
          {persons.map((person) => (
            <tr key={person.id} className={isOverallocatedPersonID(person.id) ? "person-overallocated" : undefined}>
              <td>
                <input
                  type="checkbox"
                  aria-label={`Select person ${person.name}`}
                  checked={selectedPersonIDSet.has(person.id)}
                  onChange={() => {
                    const nextSelectedPersonIDs = new Set(selectedPersonIDSet)
                    if (nextSelectedPersonIDs.has(person.id)) {
                      nextSelectedPersonIDs.delete(person.id)
                    } else {
                      nextSelectedPersonIDs.add(person.id)
                    }
                    setSelectedPersonIDs(Array.from(nextSelectedPersonIDs))
                  }}
                />
              </td>
              <td>{person.name}</td>
              <td>{person.employment_pct}</td>
              <td>
                {person.employment_changes && person.employment_changes.length > 0
                  ? person.employment_changes
                    .map((change) => `${change.effective_month}: ${change.employment_pct}%`)
                    .join(", ")
                  : "-"}
              </td>
              <td>
                <div className="actions">
                  <button type="button" onClick={() => onSwitchToEditContext(person)}>Edit</button>
                  <button type="button" onClick={() => onDeletePerson(person.id)}>Delete</button>
                </div>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </section>
  )
}
