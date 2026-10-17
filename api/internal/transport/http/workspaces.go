package httptransport

import (
	"context"

	"github.com/gofiber/fiber/v3"

	"example.com/rolescout/api/internal/domain"
)

type WorkspaceStore interface {
	Create(ctx context.Context, name, ownerID string) (domain.Workspace, error)
	Membership(ctx context.Context, workspaceID, userID string) (domain.WorkspaceMembership, error)
	ListMemberWorkspaces(ctx context.Context, userID string) ([]domain.MemberWorkspace, error)
	ListMembers(ctx context.Context, workspaceID string) ([]domain.WorkspaceMember, error)
	AddMemberByEmail(ctx context.Context, workspaceID, email string, role domain.MembershipRole) (domain.WorkspaceMember, error)
}

// MembershipAuthorizer allows workspace access to its members only.
type MembershipAuthorizer struct {
	Workspaces interface {
		Membership(ctx context.Context, workspaceID, userID string) (domain.WorkspaceMembership, error)
	}
}

func (m MembershipAuthorizer) AuthorizeWorkspace(c fiber.Ctx, workspaceID string) (domain.MembershipRole, error) {
	user, ok := currentUser(c)
	if !ok {
		return "", errUnauthenticated
	}
	membership, err := m.Workspaces.Membership(c.Context(), workspaceID, user.ID)
	if err != nil {
		return "", err
	}
	return membership.Role, nil
}

func workspaceRole(c fiber.Ctx) domain.MembershipRole {
	role, _ := c.Locals(workspaceRoleLocal).(domain.MembershipRole)
	return role
}

func (h *handlers) listWorkspaces(c fiber.Ctx) error {
	user, _ := currentUser(c)
	ws, err := h.WorkspaceStore.ListMemberWorkspaces(c.Context(), user.ID)
	if err != nil {
		return err
	}
	return c.JSON(dataResponse[[]workspaceDTO]{Data: mapSlice(ws, toWorkspace)})
}

type createWorkspaceRequest struct {
	Name string `json:"name"`
}

func (h *handlers) createWorkspace(c fiber.Ctx) error {
	var req createWorkspaceRequest
	if err := decodeJSON(c, &req); err != nil {
		return err
	}
	var v domain.ValidationError
	name := requiredText(&v, "name", req.Name, maxWorkspaceName)
	if err := v.Err(); err != nil {
		return err
	}
	user, _ := currentUser(c)
	w, err := h.WorkspaceStore.Create(c.Context(), name, user.ID)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(dataResponse[workspaceDTO]{
		Data: toWorkspace(domain.MemberWorkspace{Workspace: w, Role: domain.RoleOwner}),
	})
}

func (h *handlers) listMembers(c fiber.Ctx) error {
	members, err := h.WorkspaceStore.ListMembers(c.Context(), workspaceID(c))
	if err != nil {
		return err
	}
	return c.JSON(dataResponse[[]memberDTO]{Data: mapSlice(members, toMember)})
}

type addMemberRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

// addMember lets owners add an existing user to the workspace.
func (h *handlers) addMember(c fiber.Ctx) error {
	if workspaceRole(c) != domain.RoleOwner {
		return domain.ErrForbidden
	}
	var req addMemberRequest
	if err := decodeJSON(c, &req); err != nil {
		return err
	}
	var v domain.ValidationError
	email := validateEmail(&v, req.Email)
	role := domain.MembershipRole(req.Role)
	if role == "" {
		role = domain.RoleMember
	}
	if role != domain.RoleOwner && role != domain.RoleMember {
		v.Add("role", "must be owner or member")
	}
	if err := v.Err(); err != nil {
		return err
	}
	member, err := h.WorkspaceStore.AddMemberByEmail(c.Context(), workspaceID(c), email, role)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(dataResponse[memberDTO]{Data: toMember(member)})
}
