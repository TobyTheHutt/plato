import type { Dispatch, FormEvent, RefObject, SetStateAction } from "react"
import type { Project, ProjectFormState } from "../app/types"

type ProjectsPanelProps = {
  projectForm: ProjectFormState
  setProjectForm: Dispatch<SetStateAction<ProjectFormState>>
  projectFormContextLabel: string
  editingProject: Project | undefined
  onSwitchToCreateContext: () => void
  onSaveProject: (event: FormEvent) => void
  selectedProjectIDs: string[]
  onDeleteSelectedProjects: () => void
  selectAllProjectsCheckboxRef: RefObject<HTMLInputElement>
  projects: Project[]
  setSelectedProjectIDs: Dispatch<SetStateAction<string[]>>
  onSwitchToEditContext: (project: Project) => void
  onDeleteProject: (projectID: string) => void
}

export function ProjectsPanel(props: ProjectsPanelProps) {
  const {
    projectForm,
    setProjectForm,
    projectFormContextLabel,
    editingProject,
    onSwitchToCreateContext,
    onSaveProject,
    selectedProjectIDs,
    onDeleteSelectedProjects,
    selectAllProjectsCheckboxRef,
    projects,
    setSelectedProjectIDs,
    onSwitchToEditContext,
    onDeleteProject
  } = props
  const selectedProjectIDSet = new Set(selectedProjectIDs)

  return (
    <section className="panel">
      <h2>Projects</h2>
      <form className="grid-form" onSubmit={onSaveProject}>
        <p>{projectFormContextLabel}</p>
        {editingProject && (
          <div className="actions">
            <button type="button" onClick={onSwitchToCreateContext}>Switch to creation context</button>
          </div>
        )}
        <label>
          Name
          <input value={projectForm.name} onChange={(event) => setProjectForm((current) => ({ ...current, name: event.target.value }))} />
        </label>
        <label>
          Start date
          <input type="date" value={projectForm.startDate} onChange={(event) => setProjectForm((current) => ({ ...current, startDate: event.target.value }))} />
        </label>
        <label>
          End date
          <input type="date" value={projectForm.endDate} onChange={(event) => setProjectForm((current) => ({ ...current, endDate: event.target.value }))} />
        </label>
        <label>
          Estimated effort hours
          <input
            type="number"
            value={projectForm.estimatedEffortHours}
            onChange={(event) => setProjectForm((current) => ({ ...current, estimatedEffortHours: event.target.value }))}
          />
        </label>
        <div className="actions">
          <button type="submit">Save project</button>
          {selectedProjectIDs.length > 1 && (
            <button type="button" onClick={onDeleteSelectedProjects}>Delete selected items</button>
          )}
        </div>
      </form>
      <table>
        <thead>
          <tr>
            <th>
              <input
                ref={selectAllProjectsCheckboxRef}
                type="checkbox"
                aria-label="Select all projects"
                checked={projects.length > 0 && selectedProjectIDs.length === projects.length}
                onChange={(event) => {
                  if (event.target.checked) {
                    setSelectedProjectIDs(projects.map((project) => project.id))
                    return
                  }
                  setSelectedProjectIDs([])
                }}
              />
            </th>
            <th>Name</th>
            <th>Start</th>
            <th>End</th>
            <th>Effort hours</th>
            <th />
          </tr>
        </thead>
        <tbody>
          {projects.map((project) => (
            <tr key={project.id}>
              <td>
                <input
                  type="checkbox"
                  aria-label={`Select project ${project.name}`}
                  checked={selectedProjectIDSet.has(project.id)}
                  onChange={() => {
                    const nextSelectedProjectIDs = new Set(selectedProjectIDSet)
                    if (nextSelectedProjectIDs.has(project.id)) {
                      nextSelectedProjectIDs.delete(project.id)
                    } else {
                      nextSelectedProjectIDs.add(project.id)
                    }
                    setSelectedProjectIDs(Array.from(nextSelectedProjectIDs))
                  }}
                />
              </td>
              <td>{project.name}</td>
              <td>{project.start_date}</td>
              <td>{project.end_date}</td>
              <td>{project.estimated_effort_hours}</td>
              <td>
                <div className="actions">
                  <button type="button" onClick={() => onSwitchToEditContext(project)}>Edit</button>
                  <button type="button" onClick={() => onDeleteProject(project.id)}>Delete</button>
                </div>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </section>
  )
}
