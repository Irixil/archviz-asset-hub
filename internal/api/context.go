package api

import (
	"errors"

	"damask/server/internal/auth"
	"damask/server/internal/service"

	"github.com/gofiber/fiber/v3"
)

const (
	localWorkspace = "ws"
	localMember    = "member"
)

func getLocalWorkspace(c fiber.Ctx, svc service.WorkspaceService) (*service.WorkspaceDTO, error) {
	if v := c.Locals(localWorkspace); v != nil {
		ws, ok := v.(*service.WorkspaceDTO)
		if !ok {
			return nil, errors.New("cached workspace has unexpected type")
		}
		return ws, nil
	}
	claims := auth.GetClaims(c)
	ws, err := svc.Get(c.Context(), claims.WorkspaceID)
	if err != nil {
		return nil, err
	}
	c.Locals(localWorkspace, ws)
	return ws, nil
}

func getLocalMember(c fiber.Ctx, svc service.WorkspaceService) (*service.MemberDTO, error) {
	if v := c.Locals(localMember); v != nil {
		member, ok := v.(*service.MemberDTO)
		if !ok {
			return nil, errors.New("cached member has unexpected type")
		}
		return member, nil
	}
	claims := auth.GetClaims(c)
	m, err := svc.GetMember(c.Context(), claims.WorkspaceID, claims.UserID)
	if err != nil {
		return nil, err
	}
	c.Locals(localMember, m)
	return m, nil
}
