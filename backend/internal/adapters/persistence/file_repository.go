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

type FileRepository struct {
	path           string
	mu             sync.RWMutex
	state          fileState
	persistedState fileState
}

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
			if err := r.persistLocked(); err != nil {
				return err
			}
			return nil
		}
		return err
	}

	if len(content) == 0 {
		return nil
	}

	if err := json.Unmarshal(content, &r.state); err != nil {
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

	if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err != nil {
		r.state = cloneFileState(r.persistedState)
		return err
	}

	tmp := r.path + ".tmp"
	if err := os.WriteFile(tmp, body, 0o600); err != nil {
		_ = os.Remove(tmp)
		r.state = cloneFileState(r.persistedState)
		return err
	}

	if err := os.Rename(tmp, r.path); err != nil {
		_ = os.Remove(tmp)
		r.state = cloneFileState(r.persistedState)
		return err
	}
	r.persistedState = cloneFileState(r.state)

	return nil
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

func normalizedAllocationTarget(allocation domain.Allocation) (string, string) {
	targetType := strings.TrimSpace(allocation.TargetType)
	targetID := strings.TrimSpace(allocation.TargetID)
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

func (r *FileRepository) ListOrganisations(_ context.Context) ([]domain.Organisation, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]domain.Organisation, 0, len(r.state.Organisations))
	for _, organisation := range r.state.Organisations {
		result = append(result, organisation)
	}
	sortedOrganisations(result)
	return result, nil
}

func (r *FileRepository) GetOrganisation(_ context.Context, id string) (domain.Organisation, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	organisation, ok := r.state.Organisations[id]
	if !ok {
		return domain.Organisation{}, domain.ErrNotFound
	}
	return organisation, nil
}

func (r *FileRepository) CreateOrganisation(_ context.Context, organisation domain.Organisation) (domain.Organisation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	organisation.ID = r.nextIDLocked("org")
	organisation.CreatedAt = now
	organisation.UpdatedAt = now
	r.state.Organisations[organisation.ID] = organisation

	if err := r.persistLocked(); err != nil {
		return domain.Organisation{}, err
	}

	return organisation, nil
}

func (r *FileRepository) UpdateOrganisation(_ context.Context, organisation domain.Organisation) (domain.Organisation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	current, ok := r.state.Organisations[organisation.ID]
	if !ok {
		return domain.Organisation{}, domain.ErrNotFound
	}

	organisation.CreatedAt = current.CreatedAt
	organisation.UpdatedAt = time.Now().UTC()
	r.state.Organisations[organisation.ID] = organisation

	if err := r.persistLocked(); err != nil {
		return domain.Organisation{}, err
	}

	return organisation, nil
}

func (r *FileRepository) DeleteOrganisation(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.state.Organisations[id]; !ok {
		return domain.ErrNotFound
	}

	delete(r.state.Organisations, id)
	for personID, person := range r.state.Persons {
		if person.OrganisationID == id {
			delete(r.state.Persons, personID)
		}
	}
	for projectID, project := range r.state.Projects {
		if project.OrganisationID == id {
			delete(r.state.Projects, projectID)
		}
	}
	for groupID, group := range r.state.Groups {
		if group.OrganisationID == id {
			delete(r.state.Groups, groupID)
		}
	}
	for allocationID, allocation := range r.state.Allocations {
		if allocation.OrganisationID == id {
			delete(r.state.Allocations, allocationID)
		}
	}
	for holidayID, entry := range r.state.OrgHolidays {
		if entry.OrganisationID == id {
			delete(r.state.OrgHolidays, holidayID)
		}
	}
	for entryID, entry := range r.state.GroupUnavailability {
		if entry.OrganisationID == id {
			delete(r.state.GroupUnavailability, entryID)
		}
	}
	for entryID, entry := range r.state.PersonUnavailability {
		if entry.OrganisationID == id {
			delete(r.state.PersonUnavailability, entryID)
		}
	}

	return r.persistLocked()
}

func (r *FileRepository) ListPersons(_ context.Context, organisationID string) ([]domain.Person, error) {
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

func (r *FileRepository) GetPerson(_ context.Context, organisationID, id string) (domain.Person, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	person, ok := r.state.Persons[id]
	if !ok || person.OrganisationID != organisationID {
		return domain.Person{}, domain.ErrNotFound
	}
	return person, nil
}

func (r *FileRepository) CreatePerson(_ context.Context, person domain.Person) (domain.Person, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	person.ID = r.nextIDLocked("person")
	person.CreatedAt = now
	person.UpdatedAt = now
	r.state.Persons[person.ID] = person

	if err := r.persistLocked(); err != nil {
		return domain.Person{}, err
	}

	return person, nil
}

func (r *FileRepository) UpdatePerson(_ context.Context, person domain.Person) (domain.Person, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	current, ok := r.state.Persons[person.ID]
	if !ok || current.OrganisationID != person.OrganisationID {
		return domain.Person{}, domain.ErrNotFound
	}

	person.CreatedAt = current.CreatedAt
	person.UpdatedAt = time.Now().UTC()
	r.state.Persons[person.ID] = person

	if err := r.persistLocked(); err != nil {
		return domain.Person{}, err
	}

	return person, nil
}

func (r *FileRepository) DeletePerson(_ context.Context, organisationID, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	person, ok := r.state.Persons[id]
	if !ok || person.OrganisationID != organisationID {
		return domain.ErrNotFound
	}
	delete(r.state.Persons, id)

	for groupID, group := range r.state.Groups {
		if group.OrganisationID != organisationID {
			continue
		}
		members := make([]string, 0, len(group.MemberIDs))
		for _, memberID := range group.MemberIDs {
			if memberID != id {
				members = append(members, memberID)
			}
		}
		group.MemberIDs = members
		group.UpdatedAt = time.Now().UTC()
		r.state.Groups[groupID] = group
	}

	for allocationID, allocation := range r.state.Allocations {
		targetType, targetID := normalizedAllocationTarget(allocation)
		if allocation.OrganisationID == organisationID && targetType == domain.AllocationTargetPerson && targetID == id {
			delete(r.state.Allocations, allocationID)
		}
	}
	for entryID, entry := range r.state.PersonUnavailability {
		if entry.OrganisationID == organisationID && entry.PersonID == id {
			delete(r.state.PersonUnavailability, entryID)
		}
	}

	return r.persistLocked()
}

func (r *FileRepository) ListProjects(_ context.Context, organisationID string) ([]domain.Project, error) {
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

func (r *FileRepository) GetProject(_ context.Context, organisationID, id string) (domain.Project, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	project, ok := r.state.Projects[id]
	if !ok || project.OrganisationID != organisationID {
		return domain.Project{}, domain.ErrNotFound
	}
	return project, nil
}

func (r *FileRepository) CreateProject(_ context.Context, project domain.Project) (domain.Project, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	project.ID = r.nextIDLocked("project")
	project.CreatedAt = now
	project.UpdatedAt = now
	r.state.Projects[project.ID] = project

	if err := r.persistLocked(); err != nil {
		return domain.Project{}, err
	}

	return project, nil
}

func (r *FileRepository) UpdateProject(_ context.Context, project domain.Project) (domain.Project, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	current, ok := r.state.Projects[project.ID]
	if !ok || current.OrganisationID != project.OrganisationID {
		return domain.Project{}, domain.ErrNotFound
	}

	project.CreatedAt = current.CreatedAt
	project.UpdatedAt = time.Now().UTC()
	r.state.Projects[project.ID] = project

	if err := r.persistLocked(); err != nil {
		return domain.Project{}, err
	}

	return project, nil
}

func (r *FileRepository) DeleteProject(_ context.Context, organisationID, id string) error {
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

	return r.persistLocked()
}

func (r *FileRepository) ListGroups(_ context.Context, organisationID string) ([]domain.Group, error) {
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

func (r *FileRepository) GetGroup(_ context.Context, organisationID, id string) (domain.Group, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	group, ok := r.state.Groups[id]
	if !ok || group.OrganisationID != organisationID {
		return domain.Group{}, domain.ErrNotFound
	}
	return copyGroup(group), nil
}

func (r *FileRepository) CreateGroup(_ context.Context, group domain.Group) (domain.Group, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	group.ID = r.nextIDLocked("group")
	group.MemberIDs = uniqueStrings(group.MemberIDs)
	group.CreatedAt = now
	group.UpdatedAt = now
	r.state.Groups[group.ID] = copyGroup(group)

	if err := r.persistLocked(); err != nil {
		return domain.Group{}, err
	}

	return group, nil
}

func (r *FileRepository) UpdateGroup(_ context.Context, group domain.Group) (domain.Group, error) {
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

	if err := r.persistLocked(); err != nil {
		return domain.Group{}, err
	}

	return group, nil
}

func (r *FileRepository) DeleteGroup(_ context.Context, organisationID, id string) error {
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

	return r.persistLocked()
}

func (r *FileRepository) ListAllocations(_ context.Context, organisationID string) ([]domain.Allocation, error) {
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

func (r *FileRepository) GetAllocation(_ context.Context, organisationID, id string) (domain.Allocation, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	allocation, ok := r.state.Allocations[id]
	if !ok || allocation.OrganisationID != organisationID {
		return domain.Allocation{}, domain.ErrNotFound
	}
	return allocation, nil
}

func (r *FileRepository) CreateAllocation(_ context.Context, allocation domain.Allocation) (domain.Allocation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	allocation.TargetType, allocation.TargetID = normalizedAllocationTarget(allocation)
	if allocation.TargetType == domain.AllocationTargetPerson {
		allocation.PersonID = allocation.TargetID
	} else {
		allocation.PersonID = ""
	}
	allocation.ID = r.nextIDLocked("allocation")
	allocation.CreatedAt = now
	allocation.UpdatedAt = now
	r.state.Allocations[allocation.ID] = allocation

	if err := r.persistLocked(); err != nil {
		return domain.Allocation{}, err
	}

	return allocation, nil
}

func (r *FileRepository) UpdateAllocation(_ context.Context, allocation domain.Allocation) (domain.Allocation, error) {
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

	if err := r.persistLocked(); err != nil {
		return domain.Allocation{}, err
	}

	return allocation, nil
}

func (r *FileRepository) DeleteAllocation(_ context.Context, organisationID, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	allocation, ok := r.state.Allocations[id]
	if !ok || allocation.OrganisationID != organisationID {
		return domain.ErrNotFound
	}
	delete(r.state.Allocations, id)
	return r.persistLocked()
}

func (r *FileRepository) ListOrgHolidays(_ context.Context, organisationID string) ([]domain.OrgHoliday, error) {
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

func (r *FileRepository) CreateOrgHoliday(_ context.Context, entry domain.OrgHoliday) (domain.OrgHoliday, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	entry.ID = r.nextIDLocked("org_holiday")
	entry.CreatedAt = now
	entry.UpdatedAt = now
	r.state.OrgHolidays[entry.ID] = entry

	if err := r.persistLocked(); err != nil {
		return domain.OrgHoliday{}, err
	}

	return entry, nil
}

func (r *FileRepository) DeleteOrgHoliday(_ context.Context, organisationID, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, ok := r.state.OrgHolidays[id]
	if !ok || entry.OrganisationID != organisationID {
		return domain.ErrNotFound
	}
	delete(r.state.OrgHolidays, id)
	return r.persistLocked()
}

func (r *FileRepository) ListGroupUnavailability(_ context.Context, organisationID string) ([]domain.GroupUnavailability, error) {
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

func (r *FileRepository) CreateGroupUnavailability(_ context.Context, entry domain.GroupUnavailability) (domain.GroupUnavailability, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	entry.ID = r.nextIDLocked("group_unavailability")
	entry.CreatedAt = now
	entry.UpdatedAt = now
	r.state.GroupUnavailability[entry.ID] = entry

	if err := r.persistLocked(); err != nil {
		return domain.GroupUnavailability{}, err
	}

	return entry, nil
}

func (r *FileRepository) DeleteGroupUnavailability(_ context.Context, organisationID, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, ok := r.state.GroupUnavailability[id]
	if !ok || entry.OrganisationID != organisationID {
		return domain.ErrNotFound
	}
	delete(r.state.GroupUnavailability, id)
	return r.persistLocked()
}

func (r *FileRepository) ListPersonUnavailability(_ context.Context, organisationID string) ([]domain.PersonUnavailability, error) {
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

func (r *FileRepository) ListPersonUnavailabilityByPerson(_ context.Context, organisationID, personID string) ([]domain.PersonUnavailability, error) {
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

func (r *FileRepository) ListPersonUnavailabilityByPersonAndDate(_ context.Context, organisationID, personID, date string) ([]domain.PersonUnavailability, error) {
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

func (r *FileRepository) CreatePersonUnavailability(_ context.Context, entry domain.PersonUnavailability) (domain.PersonUnavailability, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	entry.ID = r.nextIDLocked("person_unavailability")
	entry.CreatedAt = now
	entry.UpdatedAt = now
	r.state.PersonUnavailability[entry.ID] = entry

	if err := r.persistLocked(); err != nil {
		return domain.PersonUnavailability{}, err
	}

	return entry, nil
}

func (r *FileRepository) CreatePersonUnavailabilityWithDailyLimit(_ context.Context, entry domain.PersonUnavailability, maxHours float64) (domain.PersonUnavailability, error) {
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
	entry.ID = r.nextIDLocked("person_unavailability")
	entry.CreatedAt = now
	entry.UpdatedAt = now
	r.state.PersonUnavailability[entry.ID] = entry

	if err := r.persistLocked(); err != nil {
		return domain.PersonUnavailability{}, err
	}

	return entry, nil
}

func (r *FileRepository) DeletePersonUnavailability(_ context.Context, organisationID, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, ok := r.state.PersonUnavailability[id]
	if !ok || entry.OrganisationID != organisationID {
		return domain.ErrNotFound
	}
	delete(r.state.PersonUnavailability, id)
	return r.persistLocked()
}

func (r *FileRepository) DeletePersonUnavailabilityByPerson(_ context.Context, organisationID, personID, id string) error {
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
	return r.persistLocked()
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
