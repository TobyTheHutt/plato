package service

import (
	"context"
	"strings"

	"plato/backend/internal/domain"
	"plato/backend/internal/ports"
)

func (s *Service) ListGroups(ctx context.Context, auth ports.AuthContext) ([]domain.Group, error) {
	if err := requireAnyRole(auth, domain.RoleOrgAdmin, domain.RoleOrgUser); err != nil {
		return nil, err
	}
	organisationID, err := requiredOrganisationID(auth)
	if err != nil {
		return nil, err
	}
	return s.repo.ListGroups(ctx, organisationID)
}

func (s *Service) GetGroup(ctx context.Context, auth ports.AuthContext, groupID string) (domain.Group, error) {
	if err := requireAnyRole(auth, domain.RoleOrgAdmin, domain.RoleOrgUser); err != nil {
		return domain.Group{}, err
	}
	organisationID, err := requiredOrganisationID(auth)
	if err != nil {
		return domain.Group{}, err
	}
	return s.repo.GetGroup(ctx, organisationID, groupID)
}

func (s *Service) CreateGroup(ctx context.Context, auth ports.AuthContext, input domain.Group) (domain.Group, error) {
	if err := requireAnyRole(auth, domain.RoleOrgAdmin); err != nil {
		return domain.Group{}, err
	}
	organisationID, err := requiredOrganisationID(auth)
	if err != nil {
		return domain.Group{}, err
	}
	if err := validateGroup(input); err != nil {
		return domain.Group{}, err
	}
	if err := s.ensureMembersBelongToOrg(ctx, organisationID, input.MemberIDs); err != nil {
		return domain.Group{}, err
	}

	group := domain.Group{
		OrganisationID: organisationID,
		Name:           strings.TrimSpace(input.Name),
		MemberIDs:      input.MemberIDs,
	}

	created, err := s.repo.CreateGroup(ctx, group)
	if err != nil {
		return domain.Group{}, err
	}

	s.telemetry.Record("group.created", map[string]string{"group_id": created.ID})
	return created, nil
}

func (s *Service) UpdateGroup(ctx context.Context, auth ports.AuthContext, groupID string, input domain.Group) (domain.Group, error) {
	if err := requireAnyRole(auth, domain.RoleOrgAdmin); err != nil {
		return domain.Group{}, err
	}
	organisationID, err := requiredOrganisationID(auth)
	if err != nil {
		return domain.Group{}, err
	}
	if err := validateGroup(input); err != nil {
		return domain.Group{}, err
	}
	if err := s.ensureMembersBelongToOrg(ctx, organisationID, input.MemberIDs); err != nil {
		return domain.Group{}, err
	}

	group, err := s.repo.GetGroup(ctx, organisationID, groupID)
	if err != nil {
		return domain.Group{}, err
	}
	group.Name = strings.TrimSpace(input.Name)
	group.MemberIDs = input.MemberIDs

	updated, err := s.repo.UpdateGroup(ctx, group)
	if err != nil {
		return domain.Group{}, err
	}

	s.telemetry.Record("group.updated", map[string]string{"group_id": updated.ID})
	return updated, nil
}

func (s *Service) DeleteGroup(ctx context.Context, auth ports.AuthContext, groupID string) error {
	if err := requireAnyRole(auth, domain.RoleOrgAdmin); err != nil {
		return err
	}
	organisationID, err := requiredOrganisationID(auth)
	if err != nil {
		return err
	}

	if err := s.repo.DeleteGroup(ctx, organisationID, groupID); err != nil {
		return err
	}

	s.telemetry.Record("group.deleted", map[string]string{"group_id": groupID})
	return nil
}

func (s *Service) AddGroupMember(ctx context.Context, auth ports.AuthContext, groupID, personID string) (domain.Group, error) {
	if err := requireAnyRole(auth, domain.RoleOrgAdmin); err != nil {
		return domain.Group{}, err
	}
	organisationID, err := requiredOrganisationID(auth)
	if err != nil {
		return domain.Group{}, err
	}
	if _, err := s.repo.GetPerson(ctx, organisationID, personID); err != nil {
		return domain.Group{}, err
	}

	group, err := s.repo.GetGroup(ctx, organisationID, groupID)
	if err != nil {
		return domain.Group{}, err
	}
	for _, memberID := range group.MemberIDs {
		if memberID == personID {
			return group, nil
		}
	}
	group.MemberIDs = append(group.MemberIDs, personID)
	return s.repo.UpdateGroup(ctx, group)
}

func (s *Service) RemoveGroupMember(ctx context.Context, auth ports.AuthContext, groupID, personID string) (domain.Group, error) {
	if err := requireAnyRole(auth, domain.RoleOrgAdmin); err != nil {
		return domain.Group{}, err
	}
	organisationID, err := requiredOrganisationID(auth)
	if err != nil {
		return domain.Group{}, err
	}

	group, err := s.repo.GetGroup(ctx, organisationID, groupID)
	if err != nil {
		return domain.Group{}, err
	}
	members := make([]string, 0, len(group.MemberIDs))
	for _, memberID := range group.MemberIDs {
		if memberID != personID {
			members = append(members, memberID)
		}
	}
	group.MemberIDs = members
	return s.repo.UpdateGroup(ctx, group)
}

func (s *Service) ensureMembersBelongToOrg(ctx context.Context, organisationID string, memberIDs []string) error {
	for _, memberID := range memberIDs {
		if _, err := s.repo.GetPerson(ctx, organisationID, memberID); err != nil {
			return err
		}
	}
	return nil
}
