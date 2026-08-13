package interfaces

import (
	"context"
	"io"

	"github.com/Tencent/WeKnora/internal/types"
)

// UserCenterDirectory looks up 农信 users by phone using service credentials.
type UserCenterDirectory interface {
	Configured() bool
	FindByAuthorizedPhone(ctx context.Context, phone string) (*types.CASUserInfo, error)
	SearchByNameOrPhone(ctx context.Context, keyword string) ([]*types.CASUserInfo, error)
}

// CASMemberImportService previews and imports 农信 users into a workspace.
type CASMemberImportService interface {
	Configured() bool
	ParseFile(filename string, r io.Reader) ([]types.CASImportRow, error)
	ParsePhonesText(text string) []types.CASImportRow
	Preview(ctx context.Context, tenantID uint64, rows []types.CASImportRow) (*types.CASImportPreview, error)
	Import(ctx context.Context, tenantID uint64, role types.TenantRole, invitedBy *string, rows []types.CASImportRow) (*types.CASImportResult, error)
}
