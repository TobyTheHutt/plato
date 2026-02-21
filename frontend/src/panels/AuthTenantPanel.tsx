import type { Organisation, Role } from "../app/types"

type AuthTenantPanelProps = {
  role: Role
  setRole: (role: Role) => void
  selectedOrganisationID: string
  setSelectedOrganisationID: (organisationID: string) => void
  organisations: Organisation[]
  onRefresh: () => void
}

export function AuthTenantPanel(props: AuthTenantPanelProps) {
  const {
    role,
    setRole,
    selectedOrganisationID,
    setSelectedOrganisationID,
    organisations,
    onRefresh
  } = props

  return (
    <section className="panel">
      <h2>Auth and Tenant</h2>
      <div className="row">
        <label>
          Role
          <select value={role} onChange={(event) => setRole(event.target.value as Role)}>
            <option value="org_admin">org_admin</option>
            <option value="org_user">org_user</option>
          </select>
        </label>
        <label>
          Active organisation
          <select value={selectedOrganisationID} onChange={(event) => setSelectedOrganisationID(event.target.value)}>
            <option value="">None</option>
            {organisations.map((organisation) => (
              <option key={organisation.id} value={organisation.id}>
                {organisation.name}
              </option>
            ))}
          </select>
        </label>
        <button type="button" onClick={onRefresh}>
          Refresh
        </button>
      </div>
    </section>
  )
}
