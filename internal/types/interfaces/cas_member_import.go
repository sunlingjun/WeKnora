package interfaces

import (
	"context"
	"io"

	"github.com/Tencent/WeKnora/internal/types"
)

// UserCenterDirectory looks up 农信 users (directory + session ticket resolve).
type UserCenterDirectory interface {
	Configured() bool
	// HasBaseURL reports whether CAS_UC_URL (or alias) is set. ZNT/UcTicket need URL only.
	HasBaseURL() bool
	FindByAuthorizedPhone(ctx context.Context, phone string) (*types.CASUserInfo, error)
	SearchByNameOrPhone(ctx context.Context, keyword string) ([]*types.CASUserInfo, error)
	// GetBoIDByZNTToken GET login/get-boId-by-znt-token/{token} → archive id string.
	GetBoIDByZNTToken(ctx context.Context, token string) (string, error)
	// GetBoIDByUcTicket POST login/getUserByUcTicket ticket= → archive id string.
	GetBoIDByUcTicket(ctx context.Context, ticket string) (string, error)
	// GetUserArchive POST person/getUserArchive/{boID} with service credentials.
	GetUserArchive(ctx context.Context, boID string) (*types.CASUserInfo, error)
}

// CASMemberImportService previews and imports 农信 users into a workspace.
type CASMemberImportService interface {
	Configured() bool
	ParseFile(filename string, r io.Reader) ([]types.CASImportRow, error)
	ParsePhonesText(text string) []types.CASImportRow
	Preview(ctx context.Context, tenantID uint64, rows []types.CASImportRow) (*types.CASImportPreview, error)
	Import(ctx context.Context, tenantID uint64, role types.TenantRole, invitedBy *string, rows []types.CASImportRow) (*types.CASImportResult, error)
}
