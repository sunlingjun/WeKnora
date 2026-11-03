package mydoc

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	secutils "github.com/Tencent/WeKnora/internal/utils"
)

var (
	ErrForbidden     = errors.New("mydoc: forbidden")
	ErrInvalidInput  = errors.New("mydoc: invalid input")
	ErrNotFound      = repository.ErrMyDocNotFound
	ErrDuplicateName = repository.ErrMyDocDuplicateName
	ErrCycle         = errors.New("mydoc: move would create a cycle")
	ErrNotOwner      = errors.New("mydoc: owner only")
	ErrNotText       = errors.New("mydoc: only text/markdown content is editable")
	ErrMachineCaller = errors.New("mydoc: machine principals cannot own personal docs")
)

// CanRead reports whether userID may read the node (owner, share, or valid ticket).
func (s *Service) CanRead(ctx context.Context, userID string, node *types.MyDocNode, ticket string) (bool, error) {
	if node == nil || node.Archived {
		return false, nil
	}
	if userID != "" && node.OwnerUserID == userID {
		return true, nil
	}
	if userID != "" {
		share, err := s.repo.GetShareByDocGrantee(ctx, node.ID, userID)
		if err != nil {
			return false, err
		}
		if share != nil && share.Role == types.MyDocRoleViewer {
			return true, nil
		}
	}
	if strings.TrimSpace(ticket) == "" {
		return false, nil
	}
	claims, err := secutils.ParseMyDocTicket(ticket, time.Now(), false)
	if err != nil {
		return false, nil
	}
	if claims.DocID != node.ID {
		return false, nil
	}
	hash := secutils.HashMyDocTicketToken(ticket)
	row, err := s.repo.GetTicketByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, repository.ErrMyDocNotFound) {
			return false, nil
		}
		return false, err
	}
	if row.Revoked || time.Now().After(row.ExpiresAt.Add(secutils.MyDocTicketSkew)) {
		return false, nil
	}
	return true, nil
}

// CanWrite is owner-only (Agents act as the owner identity).
func (s *Service) CanWrite(userID string, node *types.MyDocNode) bool {
	return userID != "" && node != nil && !node.Archived && node.OwnerUserID == userID
}

// CanShare is owner-only.
func (s *Service) CanShare(userID string, node *types.MyDocNode) bool {
	return s.CanWrite(userID, node)
}

// RequireHumanUser rejects API-key / MCP-endpoint machine principals.
func RequireHumanUser(ctx context.Context) (string, error) {
	if p, ok := types.PrincipalFromContext(ctx); ok {
		switch p.Type {
		case types.PrincipalAPITenant, types.PrincipalAPIPlatform, types.PrincipalMCPEndpoint:
			return "", ErrMachineCaller
		}
	}
	uid, ok := types.UserIDFromContext(ctx)
	uid = strings.TrimSpace(uid)
	if !ok || uid == "" {
		return "", ErrForbidden
	}
	if strings.HasPrefix(uid, "mcp-") || types.IsAPISessionOwnerID(uid) {
		return "", ErrMachineCaller
	}
	return uid, nil
}
