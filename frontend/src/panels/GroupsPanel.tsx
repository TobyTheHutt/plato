import type { Dispatch, FormEvent, RefObject, SetStateAction } from "react"
import type { Group, GroupFormState, GroupMemberFormState, Person } from "../app/types"

type GroupsPanelProps = {
  groupForm: GroupFormState
  setGroupForm: Dispatch<SetStateAction<GroupFormState>>
  groupFormContextLabel: string
  editingGroup: Group | undefined
  onSwitchToCreateContext: () => void
  onSaveGroup: (event: FormEvent) => void
  selectedGroupIDs: string[]
  onDeleteSelectedGroups: () => void
  persons: Person[]
  isOverallocatedPersonID: (personID: string) => boolean
  groupMemberForm: GroupMemberFormState
  setGroupMemberForm: Dispatch<SetStateAction<GroupMemberFormState>>
  groups: Group[]
  onAddGroupMember: (event: FormEvent) => void
  selectAllGroupsCheckboxRef: RefObject<HTMLInputElement>
  setSelectedGroupIDs: Dispatch<SetStateAction<string[]>>
  onRemoveGroupMember: (groupID: string, personID: string) => void
  onSwitchToEditContext: (group: Group) => void
  onDeleteGroup: (groupID: string) => void
}

export function GroupsPanel(props: GroupsPanelProps) {
  const {
    groupForm,
    setGroupForm,
    groupFormContextLabel,
    editingGroup,
    onSwitchToCreateContext,
    onSaveGroup,
    selectedGroupIDs,
    onDeleteSelectedGroups,
    persons,
    isOverallocatedPersonID,
    groupMemberForm,
    setGroupMemberForm,
    groups,
    onAddGroupMember,
    selectAllGroupsCheckboxRef,
    setSelectedGroupIDs,
    onRemoveGroupMember,
    onSwitchToEditContext,
    onDeleteGroup
  } = props
  const personByID = new Map(persons.map((person) => [person.id, person]))

  return (
    <section className="panel">
      <h2>Groups</h2>
      <form className="grid-form" onSubmit={onSaveGroup}>
        <p>{groupFormContextLabel}</p>
        {editingGroup && (
          <div className="actions">
            <button type="button" onClick={onSwitchToCreateContext}>Switch to creation context</button>
          </div>
        )}
        <label>
          Name
          <input value={groupForm.name} onChange={(event) => setGroupForm((current) => ({ ...current, name: event.target.value }))} />
        </label>
        <fieldset>
          <legend>Members</legend>
          <div className="chips">
            {persons.map((person) => (
              <label key={person.id} className={isOverallocatedPersonID(person.id) ? "person-overallocated" : undefined}>
                <input
                  type="checkbox"
                  checked={groupForm.memberIDs.includes(person.id)}
                  onChange={() => {
                    setGroupForm((current) => {
                      if (current.memberIDs.includes(person.id)) {
                        return { ...current, memberIDs: current.memberIDs.filter((entry) => entry !== person.id) }
                      }
                      return { ...current, memberIDs: [...current.memberIDs, person.id] }
                    })
                  }}
                />
                {person.name}
              </label>
            ))}
          </div>
        </fieldset>
        <div className="actions">
          <button type="submit">Save group</button>
          {selectedGroupIDs.length > 1 && (
            <button type="button" onClick={onDeleteSelectedGroups}>Delete selected items</button>
          )}
        </div>
      </form>

      <form className="row" onSubmit={onAddGroupMember}>
        <label>
          Group
          <select value={groupMemberForm.groupID} onChange={(event) => setGroupMemberForm((current) => ({ ...current, groupID: event.target.value }))}>
            <option value="">Select group</option>
            {groups.map((group) => (
              <option key={group.id} value={group.id}>{group.name}</option>
            ))}
          </select>
        </label>
        <label>
          Person
          <select value={groupMemberForm.personID} onChange={(event) => setGroupMemberForm((current) => ({ ...current, personID: event.target.value }))}>
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
        <button type="submit">Add member</button>
      </form>

      <table>
        <thead>
          <tr>
            <th>
              <input
                ref={selectAllGroupsCheckboxRef}
                type="checkbox"
                aria-label="Select all groups"
                checked={groups.length > 0 && selectedGroupIDs.length === groups.length}
                onChange={(event) => {
                  if (event.target.checked) {
                    setSelectedGroupIDs(groups.map((group) => group.id))
                    return
                  }
                  setSelectedGroupIDs([])
                }}
              />
            </th>
            <th>Name</th>
            <th>Members</th>
            <th />
          </tr>
        </thead>
        <tbody>
          {groups.map((group) => (
            <tr key={group.id}>
              <td>
                <input
                  type="checkbox"
                  aria-label={`Select group ${group.name}`}
                  checked={selectedGroupIDs.includes(group.id)}
                  onChange={() => {
                    setSelectedGroupIDs((current) => {
                      if (current.includes(group.id)) {
                        return current.filter((id) => id !== group.id)
                      }
                      return [...current, group.id]
                    })
                  }}
                />
              </td>
              <td>{group.name}</td>
              <td>
                <details>
                  <summary>{group.member_ids.length} member(s)</summary>
                  {group.member_ids.length === 0 && <p>No members</p>}
                  {group.member_ids.map((memberID) => {
                    const person = personByID.get(memberID)
                    return (
                      <div key={memberID} className="member-row">
                        <span className={isOverallocatedPersonID(memberID) ? "person-overallocated" : undefined}>
                          {person?.name ?? memberID}
                        </span>
                        <button type="button" onClick={() => onRemoveGroupMember(group.id, memberID)}>Remove</button>
                      </div>
                    )
                  })}
                </details>
              </td>
              <td>
                <div className="actions">
                  <button type="button" onClick={() => onSwitchToEditContext(group)}>Edit</button>
                  <button type="button" onClick={() => onDeleteGroup(group.id)}>Delete</button>
                </div>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </section>
  )
}
