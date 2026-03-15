package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"plato/backend/internal/domain"
)

type fileState struct {
	Organisations        map[string]domain.Organisation         `json:"organisations"`
	Persons              map[string]domain.Person               `json:"persons"`
	Projects             map[string]domain.Project              `json:"projects"`
	Groups               map[string]domain.Group                `json:"groups"`
	Allocations          map[string]domain.Allocation           `json:"allocations"`
	OrgHolidays          map[string]domain.OrgHoliday           `json:"org_holidays"`
	GroupUnavailability  map[string]domain.GroupUnavailability  `json:"group_unavailability"`
	PersonUnavailability map[string]domain.PersonUnavailability `json:"person_unavailability"`
	Sequence             int64                                  `json:"sequence"`
}

// FileRepository stores backend state in a JSON file on local disk.
type FileRepository struct {
	path           string
	mu             sync.RWMutex
	state          fileState
	persistedState fileState
}

const (
	organisationIDPrefix         = "org"
	personIDPrefix               = "person"
	projectIDPrefix              = "project"
	groupIDPrefix                = "group"
	allocationIDPrefix           = "allocation"
	orgHolidayIDPrefix           = "org_holiday"
	groupUnavailabilityIDPrefix  = "group_unavailability"
	personUnavailabilityIDPrefix = "person_unavailability"
)

// Close flushes the current in-memory state to disk.
func (r *FileRepository) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.persistLocked()
}

// NewFileRepository returns a file-backed repository for the provided path.
func NewFileRepository(path string) (*FileRepository, error) {
	if path == "" {
		path = "./plato_runtime_data.json"
	}

	repo := &FileRepository{
		path: path,
		state: fileState{
			Organisations:        map[string]domain.Organisation{},
			Persons:              map[string]domain.Person{},
			Projects:             map[string]domain.Project{},
			Groups:               map[string]domain.Group{},
			Allocations:          map[string]domain.Allocation{},
			OrgHolidays:          map[string]domain.OrgHoliday{},
			GroupUnavailability:  map[string]domain.GroupUnavailability{},
			PersonUnavailability: map[string]domain.PersonUnavailability{},
		},
	}
	repo.persistedState = cloneFileState(repo.state)

	if err := repo.load(); err != nil {
		return nil, err
	}

	return repo, nil
}

func (r *FileRepository) load() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	content, err := os.ReadFile(r.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return r.persistLocked()
		}
		return err
	}

	if len(content) == 0 {
		return nil
	}

	err = json.Unmarshal(content, &r.state)
	if err != nil {
		return fmt.Errorf("decode repository data: %w", err)
	}

	r.ensureMapsLocked()
	r.normalizeLegacyAllocationsLocked()
	r.persistedState = cloneFileState(r.state)
	return nil
}

func (r *FileRepository) ensureMapsLocked() {
	if r.state.Organisations == nil {
		r.state.Organisations = map[string]domain.Organisation{}
	}
	if r.state.Persons == nil {
		r.state.Persons = map[string]domain.Person{}
	}
	if r.state.Projects == nil {
		r.state.Projects = map[string]domain.Project{}
	}
	if r.state.Groups == nil {
		r.state.Groups = map[string]domain.Group{}
	}
	if r.state.Allocations == nil {
		r.state.Allocations = map[string]domain.Allocation{}
	}
	if r.state.OrgHolidays == nil {
		r.state.OrgHolidays = map[string]domain.OrgHoliday{}
	}
	if r.state.GroupUnavailability == nil {
		r.state.GroupUnavailability = map[string]domain.GroupUnavailability{}
	}
	if r.state.PersonUnavailability == nil {
		r.state.PersonUnavailability = map[string]domain.PersonUnavailability{}
	}
}

func (r *FileRepository) nextIDLocked(prefix string) string {
	r.state.Sequence++
	return fmt.Sprintf("%s_%d", prefix, r.state.Sequence)
}

func (r *FileRepository) persistLocked() error {
	r.ensureMapsLocked()
	body, err := json.MarshalIndent(r.state, "", "  ")
	if err != nil {
		return err
	}

	err = os.MkdirAll(filepath.Dir(r.path), 0o755)
	if err != nil {
		r.state = cloneFileState(r.persistedState)
		return err
	}

	tmp := r.path + ".tmp"
	err = os.WriteFile(tmp, body, 0o600)
	if err != nil {
		_ = os.Remove(tmp)
		r.state = cloneFileState(r.persistedState)
		return err
	}

	err = os.Rename(tmp, r.path)
	if err != nil {
		_ = os.Remove(tmp)
		r.state = cloneFileState(r.persistedState)
		return err
	}
	r.persistedState = cloneFileState(r.state)

	return nil
}

func contextErr(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}

func (r *FileRepository) persistLockedWithContext(ctx context.Context) error {
	if err := contextErr(ctx); err != nil {
		r.state = cloneFileState(r.persistedState)
		return err
	}
	return r.persistLocked()
}

func copyGroup(group domain.Group) domain.Group {
	group.MemberIDs = append([]string{}, group.MemberIDs...)
	return group
}

func copyPerson(person domain.Person) domain.Person {
	person.EmploymentChanges = append([]domain.EmploymentChange{}, person.EmploymentChanges...)
	return person
}

func cloneFileState(state fileState) fileState {
	clone := fileState{
		Organisations:        make(map[string]domain.Organisation, len(state.Organisations)),
		Persons:              make(map[string]domain.Person, len(state.Persons)),
		Projects:             make(map[string]domain.Project, len(state.Projects)),
		Groups:               make(map[string]domain.Group, len(state.Groups)),
		Allocations:          make(map[string]domain.Allocation, len(state.Allocations)),
		OrgHolidays:          make(map[string]domain.OrgHoliday, len(state.OrgHolidays)),
		GroupUnavailability:  make(map[string]domain.GroupUnavailability, len(state.GroupUnavailability)),
		PersonUnavailability: make(map[string]domain.PersonUnavailability, len(state.PersonUnavailability)),
		Sequence:             state.Sequence,
	}

	for id, organisation := range state.Organisations {
		clone.Organisations[id] = organisation
	}
	for id, person := range state.Persons {
		clone.Persons[id] = copyPerson(person)
	}
	for id, project := range state.Projects {
		clone.Projects[id] = project
	}
	for id, group := range state.Groups {
		clone.Groups[id] = copyGroup(group)
	}
	for id, allocation := range state.Allocations {
		clone.Allocations[id] = allocation
	}
	for id, holiday := range state.OrgHolidays {
		clone.OrgHolidays[id] = holiday
	}
	for id, entry := range state.GroupUnavailability {
		clone.GroupUnavailability[id] = entry
	}
	for id, entry := range state.PersonUnavailability {
		clone.PersonUnavailability[id] = entry
	}

	return clone
}

func sortedOrganisations(items []domain.Organisation) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].Name == items[j].Name {
			return items[i].ID < items[j].ID
		}
		return items[i].Name < items[j].Name
	})
}

func sortedPersons(items []domain.Person) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].Name == items[j].Name {
			return items[i].ID < items[j].ID
		}
		return items[i].Name < items[j].Name
	})
}

func sortedProjects(items []domain.Project) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].Name == items[j].Name {
			return items[i].ID < items[j].ID
		}
		return items[i].Name < items[j].Name
	})
}

func sortedGroups(items []domain.Group) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].Name == items[j].Name {
			return items[i].ID < items[j].ID
		}
		return items[i].Name < items[j].Name
	})
}

func sortedAllocations(items []domain.Allocation) {
	sort.Slice(items, func(i, j int) bool {
		iTargetType, iTargetID := normalizedAllocationTarget(items[i])
		jTargetType, jTargetID := normalizedAllocationTarget(items[j])
		if iTargetType == jTargetType {
			if iTargetID == jTargetID {
				if items[i].ProjectID == items[j].ProjectID {
					return items[i].ID < items[j].ID
				}
				return items[i].ProjectID < items[j].ProjectID
			}
			return iTargetID < jTargetID
		}
		return iTargetType < jTargetType
	})
}

func normalizedAllocationTarget(allocation domain.Allocation) (targetType string, targetID string) {
	targetType = strings.TrimSpace(allocation.TargetType)
	targetID = strings.TrimSpace(allocation.TargetID)
	if targetType == "" && strings.TrimSpace(allocation.PersonID) != "" {
		targetType = domain.AllocationTargetPerson
		targetID = strings.TrimSpace(allocation.PersonID)
	}
	return targetType, targetID
}

func (r *FileRepository) normalizeLegacyAllocationsLocked() {
	for id, allocation := range r.state.Allocations {
		targetType, targetID := normalizedAllocationTarget(allocation)
		if allocation.TargetType == targetType && allocation.TargetID == targetID {
			continue
		}
		allocation.TargetType = targetType
		allocation.TargetID = targetID
		r.state.Allocations[id] = allocation
	}
}

func sortedOrgHolidays(items []domain.OrgHoliday) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].Date == items[j].Date {
			return items[i].ID < items[j].ID
		}
		return items[i].Date < items[j].Date
	})
}

func sortedGroupUnavailability(items []domain.GroupUnavailability) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].Date == items[j].Date {
			return items[i].ID < items[j].ID
		}
		return items[i].Date < items[j].Date
	})
}

func sortedPersonUnavailability(items []domain.PersonUnavailability) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].Date == items[j].Date {
			return items[i].ID < items[j].ID
		}
		return items[i].Date < items[j].Date
	})
}

// ListOrganisations returns all stored organisations in sorted order.
func (r *FileRepository) ListOrganisations(ctx context.Context) ([]domain.Organisation, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]domain.Organisation, 0, len(r.state.Organisations))
	for _, organisation := range r.state.Organisations {
		result = append(result, organisation)
	}
	sortedOrganisations(result)
	return result, nil
}

// GetOrganisation returns the organisation with the provided id.
func (r *FileRepository) GetOrganisation(ctx context.Context, id string) (domain.Organisation, error) {
	if err := contextErr(ctx); err != nil {
		return domain.Organisation{}, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	organisation, ok := r.state.Organisations[id]
	if !ok {
		return domain.Organisation{}, domain.ErrNotFound
	}
	return organisation, nil
}

// CreateOrganisation stores a new organisation.
func (r *FileRepository) CreateOrganisation(ctx context.Context, organisation domain.Organisation) (domain.Organisation, error) {
	if err := contextErr(ctx); err != nil {
		return domain.Organisation{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	organisation.ID = r.nextIDLocked(organisationIDPrefix)
	organisation.CreatedAt = now
	organisation.UpdatedAt = now
	r.state.Organisations[organisation.ID] = organisation

	if err := r.persistLockedWithContext(ctx); err != nil {
		return domain.Organisation{}, err
	}

	return organisation, nil
}

// UpdateOrganisation stores changes to an existing organisation.
func (r *FileRepository) UpdateOrganisation(ctx context.Context, organisation domain.Organisation) (domain.Organisation, error) {
	if err := contextErr(ctx); err != nil {
		return domain.Organisation{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	current, ok := r.state.Organisations[organisation.ID]
	if !ok {
		return domain.Organisation{}, domain.ErrNotFound
	}

	organisation.CreatedAt = current.CreatedAt
	organisation.UpdatedAt = time.Now().UTC()
	r.state.Organisations[organisation.ID] = organisation

	if err := r.persistLockedWithContext(ctx); err != nil {
		return domain.Organisation{}, err
	}

	return organisation, nil
}

// DeleteOrganisation removes an organisation and its dependent records.
func (r *FileRepository) DeleteOrganisation(ctx context.Context, id string) error {
	if err := contextErr(ctx); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.state.Organisations[id]; !ok {
		return domain.ErrNotFound
	}

	delete(r.state.Organisations, id)
	r.deleteOrganisationResourcesLocked(id)

	return r.persistLockedWithContext(ctx)
}

func (r *FileRepository) deleteOrganisationResourcesLocked(organisationID string) {
	r.deletePersonsByOrganisationLocked(organisationID)
	r.deleteProjectsByOrganisationLocked(organisationID)
	r.deleteGroupsByOrganisationLocked(organisationID)
	r.deleteAllocationsByOrganisationLocked(organisationID)
	r.deleteOrgHolidaysByOrganisationLocked(organisationID)
	r.deleteGroupUnavailabilityByOrganisationLocked(organisationID)
	r.deletePersonUnavailabilityByOrganisationLocked(organisationID)
}

func (r *FileRepository) deletePersonsByOrganisationLocked(organisationID string) {
	for personID, person := range r.state.Persons {
		if person.OrganisationID == organisationID {
			delete(r.state.Persons, personID)
		}
	}
}

func (r *FileRepository) deleteProjectsByOrganisationLocked(organisationID string) {
	for projectID, project := range r.state.Projects {
		if project.OrganisationID == organisationID {
			delete(r.state.Projects, projectID)
		}
	}
}

func (r *FileRepository) deleteGroupsByOrganisationLocked(organisationID string) {
	for groupID, group := range r.state.Groups {
		if group.OrganisationID == organisationID {
			delete(r.state.Groups, groupID)
		}
	}
}

func (r *FileRepository) deleteAllocationsByOrganisationLocked(organisationID string) {
	for allocationID, allocation := range r.state.Allocations {
		if allocation.OrganisationID == organisationID {
			delete(r.state.Allocations, allocationID)
		}
	}
}

func (r *FileRepository) deleteOrgHolidaysByOrganisationLocked(organisationID string) {
	for holidayID, holiday := range r.state.OrgHolidays {
		if holiday.OrganisationID == organisationID {
			delete(r.state.OrgHolidays, holidayID)
		}
	}
}

func (r *FileRepository) deleteGroupUnavailabilityByOrganisationLocked(organisationID string) {
	for entryID, entry := range r.state.GroupUnavailability {
		if entry.OrganisationID == organisationID {
			delete(r.state.GroupUnavailability, entryID)
		}
	}
}

func (r *FileRepository) deletePersonUnavailabilityByOrganisationLocked(organisationID string) {
	for entryID, entry := range r.state.PersonUnavailability {
		if entry.OrganisationID == organisationID {
			delete(r.state.PersonUnavailability, entryID)
		}
	}
}

// ListPersons returns all people for one organisation in sorted order.
func (r *FileRepository) ListPersons(ctx context.Context, organisationID string) ([]domain.Person, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]domain.Person, 0)
	for _, person := range r.state.Persons {
		if person.OrganisationID == organisationID {
			result = append(result, person)
		}
	}
	sortedPersons(result)
	return result, nil
}

// GetPerson returns the person with the provided id from one organisation.
func (r *FileRepository) GetPerson(ctx context.Context, organisationID, id string) (domain.Person, error) {
	if err := contextErr(ctx); err != nil {
		return domain.Person{}, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	person, ok := r.state.Persons[id]
	if !ok || person.OrganisationID != organisationID {
		return domain.Person{}, domain.ErrNotFound
	}
	return person, nil
}

// CreatePerson stores a new person.
func (r *FileRepository) CreatePerson(ctx context.Context, person domain.Person) (domain.Person, error) {
	if err := contextErr(ctx); err != nil {
		return domain.Person{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	person.ID = r.nextIDLocked(personIDPrefix)
	person.CreatedAt = now
	person.UpdatedAt = now
	r.state.Persons[person.ID] = person

	if err := r.persistLockedWithContext(ctx); err != nil {
		return domain.Person{}, err
	}

	return person, nil
}

// UpdatePerson stores changes to an existing person.
func (r *FileRepository) UpdatePerson(ctx context.Context, person domain.Person) (domain.Person, error) {
	if err := contextErr(ctx); err != nil {
		return domain.Person{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	current, ok := r.state.Persons[person.ID]
	if !ok || current.OrganisationID != person.OrganisationID {
		return domain.Person{}, domain.ErrNotFound
	}

	person.CreatedAt = current.CreatedAt
	person.UpdatedAt = time.Now().UTC()
	r.state.Persons[person.ID] = person

	if err := r.persistLockedWithContext(ctx); err != nil {
		return domain.Person{}, err
	}

	return person, nil
}

// DeletePerson removes a person and dependent records from one organisation.
func (r *FileRepository) DeletePerson(ctx context.Context, organisationID, id string) error {
	if err := contextErr(ctx); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	person, ok := r.state.Persons[id]
	if !ok || person.OrganisationID != organisationID {
		return domain.ErrNotFound
	}
	delete(r.state.Persons, id)

	r.removePersonFromOrganisationGroupsLocked(organisationID, id)
	r.deletePersonAllocationsLocked(organisationID, id)
	r.deletePersonUnavailabilityLocked(organisationID, id)

	return r.persistLockedWithContext(ctx)
}

func (r *FileRepository) removePersonFromOrganisationGroupsLocked(organisationID, personID string) {
	for groupID, group := range r.state.Groups {
		if group.OrganisationID != organisationID {
			continue
		}
		group.MemberIDs = removePersonFromMemberList(group.MemberIDs, personID)
		group.UpdatedAt = time.Now().UTC()
		r.state.Groups[groupID] = group
	}
}

func removePersonFromMemberList(memberIDs []string, personID string) []string {
	members := make([]string, 0, len(memberIDs))
	for _, memberID := range memberIDs {
		if memberID != personID {
			members = append(members, memberID)
		}
	}
	return members
}

func (r *FileRepository) deletePersonAllocationsLocked(organisationID, personID string) {
	for allocationID, allocation := range r.state.Allocations {
		targetType, targetID := normalizedAllocationTarget(allocation)
		if allocation.OrganisationID == organisationID && targetType == domain.AllocationTargetPerson && targetID == personID {
			delete(r.state.Allocations, allocationID)
		}
	}
}

func (r *FileRepository) deletePersonUnavailabilityLocked(organisationID, personID string) {
	for entryID, entry := range r.state.PersonUnavailability {
		if entry.OrganisationID == organisationID && entry.PersonID == personID {
			delete(r.state.PersonUnavailability, entryID)
		}
	}
}

// ListProjects returns all projects for one organisation in sorted order.
func (r *FileRepository) ListProjects(ctx context.Context, organisationID string) ([]domain.Project, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]domain.Project, 0)
	for _, project := range r.state.Projects {
		if project.OrganisationID == organisationID {
			result = append(result, project)
		}
	}
	sortedProjects(result)
	return result, nil
}

// GetProject returns the project with the provided id from one organisation.
func (r *FileRepository) GetProject(ctx context.Context, organisationID, id string) (domain.Project, error) {
	if err := contextErr(ctx); err != nil {
		return domain.Project{}, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	project, ok := r.state.Projects[id]
	if !ok || project.OrganisationID != organisationID {
		return domain.Project{}, domain.ErrNotFound
	}
	return project, nil
}

// CreateProject stores a new project.
func (r *FileRepository) CreateProject(ctx context.Context, project domain.Project) (domain.Project, error) {
	if err := contextErr(ctx); err != nil {
		return domain.Project{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	project.ID = r.nextIDLocked(projectIDPrefix)
	project.CreatedAt = now
	project.UpdatedAt = now
	r.state.Projects[project.ID] = project

	if err := r.persistLockedWithContext(ctx); err != nil {
		return domain.Project{}, err
	}

	return project, nil
}

// UpdateProject stores changes to an existing project.
func (r *FileRepository) UpdateProject(ctx context.Context, project domain.Project) (domain.Project, error) {
	if err := contextErr(ctx); err != nil {
		return domain.Project{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	current, ok := r.state.Projects[project.ID]
	if !ok || current.OrganisationID != project.OrganisationID {
		return domain.Project{}, domain.ErrNotFound
	}

	project.CreatedAt = current.CreatedAt
	project.UpdatedAt = time.Now().UTC()
	r.state.Projects[project.ID] = project

	if err := r.persistLockedWithContext(ctx); err != nil {
		return domain.Project{}, err
	}

	return project, nil
}

// DeleteProject removes a project and dependent records from one organisation.
func (r *FileRepository) DeleteProject(ctx context.Context, organisationID, id string) error {
	if err := contextErr(ctx); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	project, ok := r.state.Projects[id]
	if !ok || project.OrganisationID != organisationID {
		return domain.ErrNotFound
	}
	delete(r.state.Projects, id)

	for allocationID, allocation := range r.state.Allocations {
		if allocation.OrganisationID == organisationID && allocation.ProjectID == id {
			delete(r.state.Allocations, allocationID)
		}
	}

	return r.persistLockedWithContext(ctx)
}

// ListGroups returns all groups for one organisation in sorted order.
func (r *FileRepository) ListGroups(ctx context.Context, organisationID string) ([]domain.Group, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]domain.Group, 0)
	for _, group := range r.state.Groups {
		if group.OrganisationID == organisationID {
			result = append(result, copyGroup(group))
		}
	}
	sortedGroups(result)
	return result, nil
}

// GetGroup returns the group with the provided id from one organisation.
func (r *FileRepository) GetGroup(ctx context.Context, organisationID, id string) (domain.Group, error) {
	if err := contextErr(ctx); err != nil {
		return domain.Group{}, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	group, ok := r.state.Groups[id]
	if !ok || group.OrganisationID != organisationID {
		return domain.Group{}, domain.ErrNotFound
	}
	return copyGroup(group), nil
}

// CreateGroup stores a new group.
func (r *FileRepository) CreateGroup(ctx context.Context, group domain.Group) (domain.Group, error) {
	if err := contextErr(ctx); err != nil {
		return domain.Group{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	group.ID = r.nextIDLocked(groupIDPrefix)
	group.MemberIDs = uniqueStrings(group.MemberIDs)
	group.CreatedAt = now
	group.UpdatedAt = now
	r.state.Groups[group.ID] = copyGroup(group)

	if err := r.persistLockedWithContext(ctx); err != nil {
		return domain.Group{}, err
	}

	return group, nil
}

// UpdateGroup stores changes to an existing group.
func (r *FileRepository) UpdateGroup(ctx context.Context, group domain.Group) (domain.Group, error) {
	if err := contextErr(ctx); err != nil {
		return domain.Group{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	current, ok := r.state.Groups[group.ID]
	if !ok || current.OrganisationID != group.OrganisationID {
		return domain.Group{}, domain.ErrNotFound
	}

	group.MemberIDs = uniqueStrings(group.MemberIDs)
	group.CreatedAt = current.CreatedAt
	group.UpdatedAt = time.Now().UTC()
	r.state.Groups[group.ID] = copyGroup(group)

	if err := r.persistLockedWithContext(ctx); err != nil {
		return domain.Group{}, err
	}

	return group, nil
}

// DeleteGroup removes a group from one organisation.
func (r *FileRepository) DeleteGroup(ctx context.Context, organisationID, id string) error {
	if err := contextErr(ctx); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	group, ok := r.state.Groups[id]
	if !ok || group.OrganisationID != organisationID {
		return domain.ErrNotFound
	}
	delete(r.state.Groups, id)

	for entryID, entry := range r.state.GroupUnavailability {
		if entry.OrganisationID == organisationID && entry.GroupID == id {
			delete(r.state.GroupUnavailability, entryID)
		}
	}
	for allocationID, allocation := range r.state.Allocations {
		targetType, targetID := normalizedAllocationTarget(allocation)
		if allocation.OrganisationID == organisationID && targetType == domain.AllocationTargetGroup && targetID == id {
			delete(r.state.Allocations, allocationID)
		}
	}

	return r.persistLockedWithContext(ctx)
}

// ListAllocations returns all allocations for one organisation in sorted order.
func (r *FileRepository) ListAllocations(ctx context.Context, organisationID string) ([]domain.Allocation, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]domain.Allocation, 0)
	for _, allocation := range r.state.Allocations {
		if allocation.OrganisationID == organisationID {
			result = append(result, allocation)
		}
	}
	sortedAllocations(result)
	return result, nil
}

// GetAllocation returns the allocation with the provided id from one organisation.
func (r *FileRepository) GetAllocation(ctx context.Context, organisationID, id string) (domain.Allocation, error) {
	if err := contextErr(ctx); err != nil {
		return domain.Allocation{}, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	allocation, ok := r.state.Allocations[id]
	if !ok || allocation.OrganisationID != organisationID {
		return domain.Allocation{}, domain.ErrNotFound
	}
	return allocation, nil
}

// CreateAllocation stores a new allocation.
func (r *FileRepository) CreateAllocation(ctx context.Context, allocation domain.Allocation) (domain.Allocation, error) {
	if err := contextErr(ctx); err != nil {
		return domain.Allocation{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	allocation.TargetType, allocation.TargetID = normalizedAllocationTarget(allocation)
	if allocation.TargetType == domain.AllocationTargetPerson {
		allocation.PersonID = allocation.TargetID
	} else {
		allocation.PersonID = ""
	}
	allocation.ID = r.nextIDLocked(allocationIDPrefix)
	allocation.CreatedAt = now
	allocation.UpdatedAt = now
	r.state.Allocations[allocation.ID] = allocation

	if err := r.persistLockedWithContext(ctx); err != nil {
		return domain.Allocation{}, err
	}

	return allocation, nil
}

// UpdateAllocation stores changes to an existing allocation.
func (r *FileRepository) UpdateAllocation(ctx context.Context, allocation domain.Allocation) (domain.Allocation, error) {
	if err := contextErr(ctx); err != nil {
		return domain.Allocation{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	current, ok := r.state.Allocations[allocation.ID]
	if !ok || current.OrganisationID != allocation.OrganisationID {
		return domain.Allocation{}, domain.ErrNotFound
	}

	allocation.TargetType, allocation.TargetID = normalizedAllocationTarget(allocation)
	if allocation.TargetType == domain.AllocationTargetPerson {
		allocation.PersonID = allocation.TargetID
	} else {
		allocation.PersonID = ""
	}
	allocation.CreatedAt = current.CreatedAt
	allocation.UpdatedAt = time.Now().UTC()
	r.state.Allocations[allocation.ID] = allocation

	if err := r.persistLockedWithContext(ctx); err != nil {
		return domain.Allocation{}, err
	}

	return allocation, nil
}

// DeleteAllocation removes an allocation from one organisation.
func (r *FileRepository) DeleteAllocation(ctx context.Context, organisationID, id string) error {
	if err := contextErr(ctx); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	allocation, ok := r.state.Allocations[id]
	if !ok || allocation.OrganisationID != organisationID {
		return domain.ErrNotFound
	}
	delete(r.state.Allocations, id)
	return r.persistLockedWithContext(ctx)
}

// ListOrgHolidays returns organisation holiday entries in sorted order.
func (r *FileRepository) ListOrgHolidays(ctx context.Context, organisationID string) ([]domain.OrgHoliday, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]domain.OrgHoliday, 0)
	for _, entry := range r.state.OrgHolidays {
		if entry.OrganisationID == organisationID {
			result = append(result, entry)
		}
	}
	sortedOrgHolidays(result)
	return result, nil
}

// CreateOrgHoliday stores a new organisation holiday entry.
func (r *FileRepository) CreateOrgHoliday(ctx context.Context, entry domain.OrgHoliday) (domain.OrgHoliday, error) {
	if err := contextErr(ctx); err != nil {
		return domain.OrgHoliday{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	entry.ID = r.nextIDLocked(orgHolidayIDPrefix)
	entry.CreatedAt = now
	entry.UpdatedAt = now
	r.state.OrgHolidays[entry.ID] = entry

	if err := r.persistLockedWithContext(ctx); err != nil {
		return domain.OrgHoliday{}, err
	}

	return entry, nil
}

// DeleteOrgHoliday removes an organisation holiday entry.
func (r *FileRepository) DeleteOrgHoliday(ctx context.Context, organisationID, id string) error {
	if err := contextErr(ctx); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	entry, ok := r.state.OrgHolidays[id]
	if !ok || entry.OrganisationID != organisationID {
		return domain.ErrNotFound
	}
	delete(r.state.OrgHolidays, id)
	return r.persistLockedWithContext(ctx)
}

// ListGroupUnavailability returns group unavailability entries in sorted order.
func (r *FileRepository) ListGroupUnavailability(ctx context.Context, organisationID string) ([]domain.GroupUnavailability, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]domain.GroupUnavailability, 0)
	for _, entry := range r.state.GroupUnavailability {
		if entry.OrganisationID == organisationID {
			result = append(result, entry)
		}
	}
	sortedGroupUnavailability(result)
	return result, nil
}

// CreateGroupUnavailability stores a new group unavailability entry.
func (r *FileRepository) CreateGroupUnavailability(ctx context.Context, entry domain.GroupUnavailability) (domain.GroupUnavailability, error) {
	if err := contextErr(ctx); err != nil {
		return domain.GroupUnavailability{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	entry.ID = r.nextIDLocked(groupUnavailabilityIDPrefix)
	entry.CreatedAt = now
	entry.UpdatedAt = now
	r.state.GroupUnavailability[entry.ID] = entry

	if err := r.persistLockedWithContext(ctx); err != nil {
		return domain.GroupUnavailability{}, err
	}

	return entry, nil
}

// DeleteGroupUnavailability removes a group unavailability entry.
func (r *FileRepository) DeleteGroupUnavailability(ctx context.Context, organisationID, id string) error {
	if err := contextErr(ctx); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	entry, ok := r.state.GroupUnavailability[id]
	if !ok || entry.OrganisationID != organisationID {
		return domain.ErrNotFound
	}
	delete(r.state.GroupUnavailability, id)
	return r.persistLockedWithContext(ctx)
}

// ListPersonUnavailability returns person unavailability entries in sorted order.
func (r *FileRepository) ListPersonUnavailability(ctx context.Context, organisationID string) ([]domain.PersonUnavailability, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]domain.PersonUnavailability, 0)
	for _, entry := range r.state.PersonUnavailability {
		if entry.OrganisationID == organisationID {
			result = append(result, entry)
		}
	}
	sortedPersonUnavailability(result)
	return result, nil
}

// ListPersonUnavailabilityByPerson returns person unavailability entries for one person.
func (r *FileRepository) ListPersonUnavailabilityByPerson(ctx context.Context, organisationID, personID string) ([]domain.PersonUnavailability, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]domain.PersonUnavailability, 0)
	for _, entry := range r.state.PersonUnavailability {
		if entry.OrganisationID == organisationID && entry.PersonID == personID {
			result = append(result, entry)
		}
	}
	sortedPersonUnavailability(result)
	return result, nil
}

// ListPersonUnavailabilityByPersonAndDate returns person unavailability entries for one day.
func (r *FileRepository) ListPersonUnavailabilityByPersonAndDate(ctx context.Context, organisationID, personID, date string) ([]domain.PersonUnavailability, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]domain.PersonUnavailability, 0)
	for _, entry := range r.state.PersonUnavailability {
		if entry.OrganisationID == organisationID && entry.PersonID == personID && entry.Date == date {
			result = append(result, entry)
		}
	}
	sortedPersonUnavailability(result)
	return result, nil
}

// CreatePersonUnavailability stores a new person unavailability entry.
func (r *FileRepository) CreatePersonUnavailability(ctx context.Context, entry domain.PersonUnavailability) (domain.PersonUnavailability, error) {
	if err := contextErr(ctx); err != nil {
		return domain.PersonUnavailability{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	entry.ID = r.nextIDLocked(personUnavailabilityIDPrefix)
	entry.CreatedAt = now
	entry.UpdatedAt = now
	r.state.PersonUnavailability[entry.ID] = entry

	if err := r.persistLockedWithContext(ctx); err != nil {
		return domain.PersonUnavailability{}, err
	}

	return entry, nil
}

// CreatePersonUnavailabilityWithDailyLimit stores a person unavailability entry within the provided daily limit.
func (r *FileRepository) CreatePersonUnavailabilityWithDailyLimit(ctx context.Context, entry domain.PersonUnavailability, maxHours float64) (domain.PersonUnavailability, error) {
	if err := contextErr(ctx); err != nil {
		return domain.PersonUnavailability{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	existingTotal := 0.0
	for _, existing := range r.state.PersonUnavailability {
		if existing.OrganisationID == entry.OrganisationID && existing.PersonID == entry.PersonID && existing.Date == entry.Date {
			existingTotal += existing.Hours
		}
	}
	if existingTotal+entry.Hours > maxHours+1e-9 {
		return domain.PersonUnavailability{}, domain.ErrValidation
	}

	now := time.Now().UTC()
	entry.ID = r.nextIDLocked(personUnavailabilityIDPrefix)
	entry.CreatedAt = now
	entry.UpdatedAt = now
	r.state.PersonUnavailability[entry.ID] = entry

	if err := r.persistLockedWithContext(ctx); err != nil {
		return domain.PersonUnavailability{}, err
	}

	return entry, nil
}

// DeletePersonUnavailability removes a person unavailability entry.
func (r *FileRepository) DeletePersonUnavailability(ctx context.Context, organisationID, id string) error {
	if err := contextErr(ctx); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	entry, ok := r.state.PersonUnavailability[id]
	if !ok || entry.OrganisationID != organisationID {
		return domain.ErrNotFound
	}
	delete(r.state.PersonUnavailability, id)
	return r.persistLockedWithContext(ctx)
}

// DeletePersonUnavailabilityByPerson removes a person's unavailability entry.
func (r *FileRepository) DeletePersonUnavailabilityByPerson(ctx context.Context, organisationID, personID, id string) error {
	if err := contextErr(ctx); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	entry, ok := r.state.PersonUnavailability[id]
	if !ok || entry.OrganisationID != organisationID {
		return domain.ErrNotFound
	}
	if entry.PersonID != personID {
		return domain.ErrForbidden
	}
	delete(r.state.PersonUnavailability, id)
	return r.persistLockedWithContext(ctx)
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}
