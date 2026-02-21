package ports

import (
	"context"
	"net/http"

	"plato/backend/internal/domain"
)

type AuthContext struct {
	UserID         string   `json:"user_id"`
	OrganisationID string   `json:"organisation_id"`
	Roles          []string `json:"roles"`
}

func (a AuthContext) HasRole(role string) bool {
	for _, entry := range a.Roles {
		if entry == role {
			return true
		}
	}

	return false
}

type AuthProvider interface {
	FromRequest(r *http.Request) (AuthContext, error)
}

type Telemetry interface {
	Record(name string, attributes map[string]string)
}

type ImportExport interface {
	Import(ctx context.Context, raw []byte) error
	Export(ctx context.Context) ([]byte, error)
}

type Repository interface {
	ListOrganisations(ctx context.Context) ([]domain.Organisation, error)
	GetOrganisation(ctx context.Context, id string) (domain.Organisation, error)
	CreateOrganisation(ctx context.Context, organisation domain.Organisation) (domain.Organisation, error)
	UpdateOrganisation(ctx context.Context, organisation domain.Organisation) (domain.Organisation, error)
	DeleteOrganisation(ctx context.Context, id string) error

	ListPersons(ctx context.Context, organisationID string) ([]domain.Person, error)
	GetPerson(ctx context.Context, organisationID, id string) (domain.Person, error)
	CreatePerson(ctx context.Context, person domain.Person) (domain.Person, error)
	UpdatePerson(ctx context.Context, person domain.Person) (domain.Person, error)
	DeletePerson(ctx context.Context, organisationID, id string) error

	ListProjects(ctx context.Context, organisationID string) ([]domain.Project, error)
	GetProject(ctx context.Context, organisationID, id string) (domain.Project, error)
	CreateProject(ctx context.Context, project domain.Project) (domain.Project, error)
	UpdateProject(ctx context.Context, project domain.Project) (domain.Project, error)
	DeleteProject(ctx context.Context, organisationID, id string) error

	ListGroups(ctx context.Context, organisationID string) ([]domain.Group, error)
	GetGroup(ctx context.Context, organisationID, id string) (domain.Group, error)
	CreateGroup(ctx context.Context, group domain.Group) (domain.Group, error)
	UpdateGroup(ctx context.Context, group domain.Group) (domain.Group, error)
	DeleteGroup(ctx context.Context, organisationID, id string) error

	ListAllocations(ctx context.Context, organisationID string) ([]domain.Allocation, error)
	GetAllocation(ctx context.Context, organisationID, id string) (domain.Allocation, error)
	CreateAllocation(ctx context.Context, allocation domain.Allocation) (domain.Allocation, error)
	UpdateAllocation(ctx context.Context, allocation domain.Allocation) (domain.Allocation, error)
	DeleteAllocation(ctx context.Context, organisationID, id string) error

	ListOrgHolidays(ctx context.Context, organisationID string) ([]domain.OrgHoliday, error)
	CreateOrgHoliday(ctx context.Context, entry domain.OrgHoliday) (domain.OrgHoliday, error)
	DeleteOrgHoliday(ctx context.Context, organisationID, id string) error

	ListGroupUnavailability(ctx context.Context, organisationID string) ([]domain.GroupUnavailability, error)
	CreateGroupUnavailability(ctx context.Context, entry domain.GroupUnavailability) (domain.GroupUnavailability, error)
	DeleteGroupUnavailability(ctx context.Context, organisationID, id string) error

	ListPersonUnavailability(ctx context.Context, organisationID string) ([]domain.PersonUnavailability, error)
	CreatePersonUnavailability(ctx context.Context, entry domain.PersonUnavailability) (domain.PersonUnavailability, error)
	DeletePersonUnavailability(ctx context.Context, organisationID, id string) error
}
