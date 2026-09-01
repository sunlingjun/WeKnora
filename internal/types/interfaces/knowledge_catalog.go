package interfaces

import (
	"context"

	"github.com/Tencent/WeKnora/internal/types"
)

// KnowledgeCatalogService is the independent read surface for workspace
// authorized knowledge-base directories and per-KB knowledge metadata.
type KnowledgeCatalogService interface {
	ListAuthorizedCatalogKBs(ctx context.Context, q types.ListCatalogKBsQuery) (*types.CatalogKBListResult, error)
	ListCatalogKnowledge(ctx context.Context, q types.ListCatalogKnowledgeQuery) (*types.CatalogKnowledgeListResult, error)
}
