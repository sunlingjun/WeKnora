package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

var (
	ErrCatalogKBNotFound    = errors.New("knowledge catalog: knowledge base not found")
	ErrCatalogInvalidCursor = errors.New("knowledge catalog: invalid cursor")
	ErrCatalogInvalidLimit  = errors.New("knowledge catalog: invalid limit")
	ErrCatalogMissingKBID   = errors.New("knowledge catalog: kb_id is required")
)

type catalogCursorPayload struct {
	U time.Time `json:"u"`
	I string    `json:"i"`
}

// KnowledgeCatalogService lists the authorized knowledge-base directory and
// per-KB knowledge metadata for a workspace API key (or JWT Viewer+).
type KnowledgeCatalogService struct {
	kbSvc    interfaces.KnowledgeBaseService
	shareSvc interfaces.KBShareService
	kgRepo   interfaces.KnowledgeRepository
}

// NewKnowledgeCatalogService constructs the catalog service.
func NewKnowledgeCatalogService(
	kbSvc interfaces.KnowledgeBaseService,
	shareSvc interfaces.KBShareService,
	kgRepo interfaces.KnowledgeRepository,
) interfaces.KnowledgeCatalogService {
	return &KnowledgeCatalogService{
		kbSvc:    kbSvc,
		shareSvc: shareSvc,
		kgRepo:   kgRepo,
	}
}

func (s *KnowledgeCatalogService) ListAuthorizedCatalogKBs(
	ctx context.Context,
	q types.ListCatalogKBsQuery,
) (*types.CatalogKBListResult, error) {
	tenantID := types.MustTenantIDFromContext(ctx)
	items, err := s.collectAuthorizedKBs(ctx, tenantID, q, true)
	if err != nil {
		return nil, err
	}
	truncated := false
	if len(items) > types.CatalogKBHardLimit {
		items = items[:types.CatalogKBHardLimit]
		truncated = true
	}
	return &types.CatalogKBListResult{
		TenantID:       tenantID,
		GeneratedAt:    time.Now().UTC(),
		KnowledgeBases: items,
		Total:          len(items),
		Truncated:      truncated,
	}, nil
}

func (s *KnowledgeCatalogService) ListCatalogKnowledge(
	ctx context.Context,
	q types.ListCatalogKnowledgeQuery,
) (*types.CatalogKnowledgeListResult, error) {
	tenantID := types.MustTenantIDFromContext(ctx)
	kbID := strings.TrimSpace(q.KBID)
	if kbID == "" {
		return nil, ErrCatalogMissingKBID
	}
	limit := q.Limit
	if limit == 0 {
		limit = types.CatalogKnowledgeDefaultLimit
	}
	if limit < 1 || limit > types.CatalogKnowledgeMaxLimit {
		return nil, ErrCatalogInvalidLimit
	}

	authorized, err := s.collectAuthorizedKBs(ctx, tenantID, types.ListCatalogKBsQuery{IncludeOrgShared: true}, false)
	if err != nil {
		return nil, err
	}
	var target *types.CatalogKnowledgeBase
	for i := range authorized {
		if authorized[i].ID == kbID {
			target = &authorized[i]
			break
		}
	}
	if target == nil {
		return nil, ErrCatalogKBNotFound
	}

	cursorQ := types.KnowledgeCatalogCursorQuery{
		Limit:        limit + 1,
		ParseStatus:  q.ParseStatus,
		UpdatedAfter: q.UpdatedAfter,
	}
	if strings.TrimSpace(q.Cursor) != "" {
		updatedAt, id, err := decodeCatalogCursor(q.Cursor)
		if err != nil {
			return nil, ErrCatalogInvalidCursor
		}
		cursorQ.HasCursor = true
		cursorQ.CursorUpdatedAt = updatedAt
		cursorQ.CursorID = id
	}

	rows, err := s.kgRepo.ListKnowledgeCatalogCursor(ctx, target.OwnerTenantID, kbID, cursorQ)
	if err != nil {
		logger.Errorf(ctx, "catalog list knowledge failed: tenant=%d owner=%d kb=%s err=%v",
			tenantID, target.OwnerTenantID, kbID, err)
		return nil, err
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	items := make([]types.CatalogKnowledgeItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, toCatalogKnowledgeItem(row))
	}
	nextCursor := ""
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		nextCursor = encodeCatalogCursor(last.UpdatedAt, last.ID)
	}
	return &types.CatalogKnowledgeListResult{
		TenantID:   tenantID,
		KBID:       kbID,
		Items:      items,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

func (s *KnowledgeCatalogService) collectAuthorizedKBs(
	ctx context.Context,
	tenantID uint64,
	q types.ListCatalogKBsQuery,
	withCount bool,
) ([]types.CatalogKnowledgeBase, error) {
	owned, err := s.kbSvc.ListKnowledgeBasesByTenantID(ctx, tenantID)
	if err != nil {
		logger.Errorf(ctx, "catalog list owned knowledge bases failed: tenant=%d err=%v", tenantID, err)
		return nil, err
	}

	seen := make(map[string]struct{}, len(owned))
	out := make([]types.CatalogKnowledgeBase, 0, len(owned))
	for _, kb := range owned {
		if kb == nil || kb.IsTemporary {
			continue
		}
		if q.Type != "" && kb.Type != q.Type {
			continue
		}
		if !catalogAllowsKnowledgeBase(ctx, kb.ID) {
			continue
		}
		item := types.CatalogKnowledgeBase{
			ID:            kb.ID,
			Name:          kb.Name,
			Type:          kb.Type,
			Description:   kb.Description,
			AccessSource:  types.CatalogAccessOwned,
			OwnerTenantID: kb.TenantID,
			Permission:    string(types.OrgRoleAdmin),
			CanDownload:   catalogCanDownloadOwned(ctx),
			UpdatedAt:     kb.UpdatedAt,
		}
		if item.OwnerTenantID == 0 {
			item.OwnerTenantID = tenantID
		}
		if withCount {
			item.KnowledgeCount = s.countKnowledge(ctx, item.OwnerTenantID, kb.ID)
		}
		seen[kb.ID] = struct{}{}
		out = append(out, item)
	}

	if q.IncludeOrgShared {
		shared, err := s.shareSvc.ListSharedKnowledgeBases(ctx, tenantID, types.TenantRoleFromContext(ctx))
		if err != nil {
			logger.Errorf(ctx, "catalog list org-shared knowledge bases failed: tenant=%d err=%v", tenantID, err)
			return nil, err
		}
		for _, info := range shared {
			if info == nil || info.KnowledgeBase == nil {
				continue
			}
			kb := info.KnowledgeBase
			if kb.IsTemporary || info.SourceTenantID == tenantID {
				continue
			}
			if _, exists := seen[kb.ID]; exists {
				continue
			}
			if q.Type != "" && kb.Type != q.Type {
				continue
			}
			if !catalogAllowsKnowledgeBase(ctx, kb.ID) {
				continue
			}
			perm := info.Permission
			item := types.CatalogKnowledgeBase{
				ID:             kb.ID,
				Name:           kb.Name,
				Type:           kb.Type,
				Description:    kb.Description,
				AccessSource:   types.CatalogAccessOrgShare,
				OwnerTenantID:  info.SourceTenantID,
				OrganizationID: info.OrganizationID,
				ShareID:        info.ShareID,
				Permission:     string(perm),
				CanDownload:    perm.HasPermission(types.OrgRoleEditor),
				UpdatedAt:      kb.UpdatedAt,
			}
			if item.OwnerTenantID == 0 {
				item.OwnerTenantID = kb.TenantID
			}
			if withCount {
				item.KnowledgeCount = s.countKnowledge(ctx, item.OwnerTenantID, kb.ID)
			}
			seen[kb.ID] = struct{}{}
			out = append(out, item)
		}
	}
	return out, nil
}

func (s *KnowledgeCatalogService) countKnowledge(ctx context.Context, ownerTenantID uint64, kbID string) int64 {
	count, err := s.kgRepo.CountKnowledgeByKnowledgeBaseID(ctx, ownerTenantID, kbID)
	if err != nil {
		logger.Warnf(ctx, "catalog count knowledge failed: owner=%d kb=%s err=%v", ownerTenantID, kbID, err)
		return 0
	}
	return count
}

func catalogAllowsKnowledgeBase(ctx context.Context, kbID string) bool {
	scope, ok := types.TenantAPIKeyScopeFromContext(ctx)
	if !ok {
		return true
	}
	return scope.AllowsKnowledgeBase(kbID)
}

func catalogCanDownloadOwned(ctx context.Context) bool {
	if _, ok := types.TenantAPIKeyScopeFromContext(ctx); ok {
		return true
	}
	return types.TenantRoleFromContext(ctx).HasPermission(types.TenantRoleContributor)
}

func toCatalogKnowledgeItem(k *types.Knowledge) types.CatalogKnowledgeItem {
	if k == nil {
		return types.CatalogKnowledgeItem{}
	}
	source := k.Channel
	if source == "" {
		source = types.ChannelWeb
	}
	return types.CatalogKnowledgeItem{
		ID:              k.ID,
		KnowledgeBaseID: k.KnowledgeBaseID,
		KnowledgeType:   k.Type,
		Title:           k.Title,
		FileName:        k.FileName,
		FileType:        k.FileType,
		FileSize:        k.FileSize,
		FolderPath:      k.FolderPath,
		ParseStatus:     k.ParseStatus,
		EnableStatus:    k.EnableStatus,
		Source:          source,
		HasFile:         k.Type == types.CatalogKnowledgeTypeFile && strings.TrimSpace(k.FilePath) != "",
		CreatedAt:       k.CreatedAt,
		UpdatedAt:       k.UpdatedAt,
	}
}

func encodeCatalogCursor(updatedAt time.Time, id string) string {
	payload, err := json.Marshal(catalogCursorPayload{U: updatedAt.UTC(), I: id})
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(payload)
}

func decodeCatalogCursor(raw string) (time.Time, string, error) {
	data, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(raw))
	if err != nil {
		return time.Time{}, "", err
	}
	var payload catalogCursorPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return time.Time{}, "", err
	}
	if payload.I == "" || payload.U.IsZero() {
		return time.Time{}, "", ErrCatalogInvalidCursor
	}
	return payload.U, payload.I, nil
}
