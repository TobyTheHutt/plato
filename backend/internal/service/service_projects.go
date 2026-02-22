package service

import (
	"context"
	"strings"

	"plato/backend/internal/domain"
	"plato/backend/internal/ports"
)

func (s *Service) ListProjects(ctx context.Context, auth ports.AuthContext) ([]domain.Project, error) {
	if err := requireAnyRole(auth, domain.RoleOrgAdmin, domain.RoleOrgUser); err != nil {
		return nil, err
	}
	organisationID, err := requiredOrganisationID(auth)
	if err != nil {
		return nil, err
	}
	return s.repo.ListProjects(ctx, organisationID)
}

func (s *Service) GetProject(ctx context.Context, auth ports.AuthContext, projectID string) (domain.Project, error) {
	if err := requireAnyRole(auth, domain.RoleOrgAdmin, domain.RoleOrgUser); err != nil {
		return domain.Project{}, err
	}
	organisationID, err := requiredOrganisationID(auth)
	if err != nil {
		return domain.Project{}, err
	}
	return s.repo.GetProject(ctx, organisationID, projectID)
}

func (s *Service) CreateProject(ctx context.Context, auth ports.AuthContext, input domain.Project) (domain.Project, error) {
	if err := requireAnyRole(auth, domain.RoleOrgAdmin); err != nil {
		return domain.Project{}, err
	}
	organisationID, err := requiredOrganisationID(auth)
	if err != nil {
		return domain.Project{}, err
	}
	if err := validateProject(input); err != nil {
		return domain.Project{}, err
	}

	project := domain.Project{
		OrganisationID:       organisationID,
		Name:                 strings.TrimSpace(input.Name),
		StartDate:            input.StartDate,
		EndDate:              input.EndDate,
		EstimatedEffortHours: input.EstimatedEffortHours,
	}

	created, err := s.repo.CreateProject(ctx, project)
	if err != nil {
		return domain.Project{}, err
	}

	s.telemetry.Record("project.created", map[string]string{"project_id": created.ID})
	return created, nil
}

func (s *Service) UpdateProject(ctx context.Context, auth ports.AuthContext, projectID string, input domain.Project) (domain.Project, error) {
	if err := requireAnyRole(auth, domain.RoleOrgAdmin); err != nil {
		return domain.Project{}, err
	}
	organisationID, err := requiredOrganisationID(auth)
	if err != nil {
		return domain.Project{}, err
	}
	if err := validateProject(input); err != nil {
		return domain.Project{}, err
	}

	project, err := s.repo.GetProject(ctx, organisationID, projectID)
	if err != nil {
		return domain.Project{}, err
	}
	project.Name = strings.TrimSpace(input.Name)
	project.StartDate = input.StartDate
	project.EndDate = input.EndDate
	project.EstimatedEffortHours = input.EstimatedEffortHours

	updated, err := s.repo.UpdateProject(ctx, project)
	if err != nil {
		return domain.Project{}, err
	}

	s.telemetry.Record("project.updated", map[string]string{"project_id": updated.ID})
	return updated, nil
}

func (s *Service) DeleteProject(ctx context.Context, auth ports.AuthContext, projectID string) error {
	if err := requireAnyRole(auth, domain.RoleOrgAdmin); err != nil {
		return err
	}
	organisationID, err := requiredOrganisationID(auth)
	if err != nil {
		return err
	}

	if err := s.repo.DeleteProject(ctx, organisationID, projectID); err != nil {
		return err
	}

	s.telemetry.Record("project.deleted", map[string]string{"project_id": projectID})
	return nil
}
