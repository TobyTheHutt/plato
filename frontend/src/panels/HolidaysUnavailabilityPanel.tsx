import type { Dispatch, FormEvent, SetStateAction } from "react"
import type {
  AvailabilityScope,
  AvailabilityUnitScope,
  Group,
  HolidayFormState,
  Organisation,
  Person,
  PersonUnavailability,
  ScopedGroupUnavailabilityFormState,
  ScopedPersonUnavailabilityFormState,
  TimespanFormState
} from "../app/types"

type HolidaysUnavailabilityPanelProps = {
  availabilityScope: AvailabilityScope
  setAvailabilityScope: Dispatch<SetStateAction<AvailabilityScope>>
  availabilityUnitScope: AvailabilityUnitScope
  setAvailabilityUnitScope: Dispatch<SetStateAction<AvailabilityUnitScope>>
  onCreateHoliday: (event: FormEvent) => void
  onCreatePersonUnavailability: (event: FormEvent) => void
  onCreateGroupUnavailability: (event: FormEvent) => void
  selectedOrganisationID: string
  setSelectedOrganisationID: (organisationID: string) => void
  organisations: Organisation[]
  holidayForm: HolidayFormState
  setHolidayForm: Dispatch<SetStateAction<HolidayFormState>>
  timespanForm: TimespanFormState
  setTimespanForm: Dispatch<SetStateAction<TimespanFormState>>
  personUnavailability: PersonUnavailability[]
  persons: Person[]
  isOverallocatedPersonID: (personID: string) => boolean
  onDeletePersonUnavailability: (entry: PersonUnavailability) => void
  personUnavailabilityForm: ScopedPersonUnavailabilityFormState
  setPersonUnavailabilityForm: Dispatch<SetStateAction<ScopedPersonUnavailabilityFormState>>
  groupUnavailabilityForm: ScopedGroupUnavailabilityFormState
  setGroupUnavailabilityForm: Dispatch<SetStateAction<ScopedGroupUnavailabilityFormState>>
  groups: Group[]
  selectedGroupPersonUnavailability: PersonUnavailability[]
}

export function HolidaysUnavailabilityPanel(props: HolidaysUnavailabilityPanelProps) {
  const {
    availabilityScope,
    setAvailabilityScope,
    availabilityUnitScope,
    setAvailabilityUnitScope,
    onCreateHoliday,
    onCreatePersonUnavailability,
    onCreateGroupUnavailability,
    selectedOrganisationID,
    setSelectedOrganisationID,
    organisations,
    holidayForm,
    setHolidayForm,
    timespanForm,
    setTimespanForm,
    personUnavailability,
    persons,
    isOverallocatedPersonID,
    onDeletePersonUnavailability,
    personUnavailabilityForm,
    setPersonUnavailabilityForm,
    groupUnavailabilityForm,
    setGroupUnavailabilityForm,
    groups,
    selectedGroupPersonUnavailability
  } = props
  const personByID = new Map(persons.map((person) => [person.id, person]))

  return (
    <section className="panel">
      <h2>Holidays and Unavailability</h2>

      <h3>Entry Scope</h3>
      <form className="row">
        <label>
          Scope
          <select value={availabilityScope} onChange={(event) => setAvailabilityScope(event.target.value as AvailabilityScope)}>
            <option value="organisation">organisation</option>
            <option value="person">person</option>
            <option value="group">group</option>
          </select>
        </label>
        <label>
          Units
          <select value={availabilityUnitScope} onChange={(event) => setAvailabilityUnitScope(event.target.value as AvailabilityUnitScope)}>
            <option value="hours">hours (single day)</option>
            <option value="days">days (timespan)</option>
            <option value="weeks">weeks (timespan)</option>
          </select>
        </label>
      </form>

      {availabilityScope === "organisation" && (
        <>
          <form className="row" onSubmit={onCreateHoliday}>
            <label>
              Organisation
              <select value={selectedOrganisationID} onChange={(event) => setSelectedOrganisationID(event.target.value)}>
                <option value="">Select organisation</option>
                {organisations.map((organisation) => (
                  <option key={organisation.id} value={organisation.id}>{organisation.name}</option>
                ))}
              </select>
            </label>
            {availabilityUnitScope === "hours" ? (
              <>
                <label>
                  Date
                  <input type="date" value={holidayForm.date} onChange={(event) => setHolidayForm((current) => ({ ...current, date: event.target.value }))} />
                </label>
                <label>
                  Hours
                  <input type="number" value={holidayForm.hours} onChange={(event) => setHolidayForm((current) => ({ ...current, hours: event.target.value }))} />
                </label>
              </>
            ) : availabilityUnitScope === "days" ? (
              <>
                <label>
                  Start date
                  <input type="date" value={timespanForm.startDate} onChange={(event) => setTimespanForm((current) => ({ ...current, startDate: event.target.value }))} />
                </label>
                <label>
                  End date
                  <input type="date" value={timespanForm.endDate} onChange={(event) => setTimespanForm((current) => ({ ...current, endDate: event.target.value }))} />
                </label>
              </>
            ) : (
              <>
                <label>
                  Start week
                  <input type="week" value={timespanForm.startWeek} onChange={(event) => setTimespanForm((current) => ({ ...current, startWeek: event.target.value }))} />
                </label>
                <label>
                  End week
                  <input type="week" value={timespanForm.endWeek} onChange={(event) => setTimespanForm((current) => ({ ...current, endWeek: event.target.value }))} />
                </label>
              </>
            )}
            <button type="submit">{availabilityUnitScope === "hours" ? "Add org unavailability" : "Add org member unavailability"}</button>
          </form>

          <table>
            <thead>
              <tr>
                <th>Person</th>
                <th>Date</th>
                <th>Hours</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {personUnavailability.map((entry) => (
                <tr key={entry.id}>
                  <td className={isOverallocatedPersonID(entry.person_id) ? "person-overallocated" : undefined}>
                    {personByID.get(entry.person_id)?.name ?? entry.person_id}
                  </td>
                  <td>{entry.date}</td>
                  <td>{entry.hours}</td>
                  <td>
                    <button type="button" onClick={() => onDeletePersonUnavailability(entry)}>Delete</button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </>
      )}

      {availabilityScope === "person" && (
        <>
          <form className="row" onSubmit={onCreatePersonUnavailability}>
            <label>
              Person
              <select value={personUnavailabilityForm.personID} onChange={(event) => setPersonUnavailabilityForm((current) => ({ ...current, personID: event.target.value }))}>
                <option value="">Select person</option>
                {persons.map((person) => (
                  <option
                    key={person.id}
                    value={person.id}
                    className={isOverallocatedPersonID(person.id) ? "person-overallocated" : undefined}
                  >
                    {person.name}
                  </option>
                ))}
              </select>
            </label>
            {availabilityUnitScope === "hours" ? (
              <>
                <label>
                  Date
                  <input type="date" value={personUnavailabilityForm.date} onChange={(event) => setPersonUnavailabilityForm((current) => ({ ...current, date: event.target.value }))} />
                </label>
                <label>
                  Hours
                  <input type="number" value={personUnavailabilityForm.hours} onChange={(event) => setPersonUnavailabilityForm((current) => ({ ...current, hours: event.target.value }))} />
                </label>
              </>
            ) : availabilityUnitScope === "days" ? (
              <>
                <label>
                  Start date
                  <input type="date" value={timespanForm.startDate} onChange={(event) => setTimespanForm((current) => ({ ...current, startDate: event.target.value }))} />
                </label>
                <label>
                  End date
                  <input type="date" value={timespanForm.endDate} onChange={(event) => setTimespanForm((current) => ({ ...current, endDate: event.target.value }))} />
                </label>
              </>
            ) : (
              <>
                <label>
                  Start week
                  <input type="week" value={timespanForm.startWeek} onChange={(event) => setTimespanForm((current) => ({ ...current, startWeek: event.target.value }))} />
                </label>
                <label>
                  End week
                  <input type="week" value={timespanForm.endWeek} onChange={(event) => setTimespanForm((current) => ({ ...current, endWeek: event.target.value }))} />
                </label>
              </>
            )}
            <button type="submit">{availabilityUnitScope === "hours" ? "Add person unavailability" : "Add person unavailability entries"}</button>
          </form>

          <table>
            <thead>
              <tr>
                <th>Person</th>
                <th>Date</th>
                <th>Hours</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {personUnavailability.map((entry) => {
                const person = personByID.get(entry.person_id)
                return (
                  <tr key={entry.id}>
                    <td className={isOverallocatedPersonID(entry.person_id) ? "person-overallocated" : undefined}>
                      {person?.name ?? entry.person_id}
                    </td>
                    <td>{entry.date}</td>
                    <td>{entry.hours}</td>
                    <td>
                      <button type="button" onClick={() => onDeletePersonUnavailability(entry)}>Delete</button>
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </>
      )}

      {availabilityScope === "group" && (
        <>
          <form className="row" onSubmit={onCreateGroupUnavailability}>
            <label>
              Group
              <select value={groupUnavailabilityForm.groupID} onChange={(event) => setGroupUnavailabilityForm((current) => ({ ...current, groupID: event.target.value }))}>
                <option value="">Select group</option>
                {groups.map((group) => (
                  <option key={group.id} value={group.id}>{group.name}</option>
                ))}
              </select>
            </label>
            {availabilityUnitScope === "hours" ? (
              <>
                <label>
                  Date
                  <input type="date" value={groupUnavailabilityForm.date} onChange={(event) => setGroupUnavailabilityForm((current) => ({ ...current, date: event.target.value }))} />
                </label>
                <label>
                  Hours
                  <input type="number" value={groupUnavailabilityForm.hours} onChange={(event) => setGroupUnavailabilityForm((current) => ({ ...current, hours: event.target.value }))} />
                </label>
              </>
            ) : availabilityUnitScope === "days" ? (
              <>
                <label>
                  Start date
                  <input type="date" value={timespanForm.startDate} onChange={(event) => setTimespanForm((current) => ({ ...current, startDate: event.target.value }))} />
                </label>
                <label>
                  End date
                  <input type="date" value={timespanForm.endDate} onChange={(event) => setTimespanForm((current) => ({ ...current, endDate: event.target.value }))} />
                </label>
              </>
            ) : (
              <>
                <label>
                  Start week
                  <input type="week" value={timespanForm.startWeek} onChange={(event) => setTimespanForm((current) => ({ ...current, startWeek: event.target.value }))} />
                </label>
                <label>
                  End week
                  <input type="week" value={timespanForm.endWeek} onChange={(event) => setTimespanForm((current) => ({ ...current, endWeek: event.target.value }))} />
                </label>
              </>
            )}
            <button type="submit">{availabilityUnitScope === "hours" ? "Add group unavailability" : "Add group unavailability entries"}</button>
          </form>

          <table>
            <thead>
              <tr>
                <th>Person</th>
                <th>Date</th>
                <th>Hours</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {selectedGroupPersonUnavailability.map((entry) => {
                const person = personByID.get(entry.person_id)
                return (
                  <tr key={entry.id}>
                    <td className={isOverallocatedPersonID(entry.person_id) ? "person-overallocated" : undefined}>
                      {person?.name ?? entry.person_id}
                    </td>
                    <td>{entry.date}</td>
                    <td>{entry.hours}</td>
                    <td>
                      <button type="button" onClick={() => onDeletePersonUnavailability(entry)}>Delete</button>
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </>
      )}
    </section>
  )
}
