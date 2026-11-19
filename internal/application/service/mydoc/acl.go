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
	// Stable English sentinels for errors.Is; localized copy is keyed by Code()
	// in frontend i18n (mydoc.errors.*) and nxin-home messageLocale.
	ErrForbidden     = errors.New("mydoc: forbidden")
	ErrInvalidInput  = errors.New("mydoc: invalid input")
	ErrNotFound      = repository.ErrMyDocNotFound
	ErrDuplicateName = repository.ErrMyDocDuplicateName
	ErrCycle         = errors.New("mydoc: move would create a cycle")
	ErrNotOwner      = errors.New("mydoc: owner only")
	ErrNotText       = errors.New("mydoc: only text/markdown content is editable")
	ErrMachineCaller = errors.New("mydoc: machine principals cannot own personal docs")
)

// Stable wire codes for MyDoc business errors (looked up by i18n).
const (
	ErrCodeDuplicateFolder = "MYDOC_DUPLICATE_FOLDER"
	ErrCodeDuplicateFile   = "MYDOC_DUPLICATE_FILE"
	ErrCodeDuplicateName   = "MYDOC_DUPLICATE_NAME"
	ErrCodeNotFound        = "MYDOC_NOT_FOUND"
	ErrCodeForbidden       = "MYDOC_FORBIDDEN"
	ErrCodeNotOwner        = "MYDOC_NOT_OWNER"
	ErrCodeCycle           = "MYDOC_MOVE_CYCLE"
	ErrCodeNotText         = "MYDOC_NOT_TEXT"
	ErrCodeInvalidInput    = "MYDOC_INVALID_INPUT"
	ErrCodeMachineCaller   = "MYDOC_MACHINE_CALLER"
)

// DuplicateNameError is returned when a sibling already uses the same name.
// Unwraps to ErrDuplicateName so callers can use errors.Is.
type DuplicateNameError struct {
	Name string
	Kind string // folder | file | ""
}

func (e *DuplicateNameError) Code() string {
	switch e.Kind {
	case types.MyDocNodeFolder:
		return ErrCodeDuplicateFolder
	case types.MyDocNodeFile:
		return ErrCodeDuplicateFile
	default:
		return ErrCodeDuplicateName
	}
}

func (e *DuplicateNameError) Error() string {
	name := strings.TrimSpace(e.Name)
	switch e.Kind {
	case types.MyDocNodeFolder:
		if name != "" {
			return "a folder named \"" + name + "\" already exists in this directory"
		}
		return "a folder with this name already exists in this directory"
	case types.MyDocNodeFile:
		if name != "" {
			return "a file named \"" + name + "\" already exists in this directory"
		}
		return "a file with this name already exists in this directory"
	default:
		if name != "" {
			return "an item named \"" + name + "\" already exists in this directory"
		}
		return ErrDuplicateName.Error()
	}
}

func (e *DuplicateNameError) Unwrap() error { return ErrDuplicateName }

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
