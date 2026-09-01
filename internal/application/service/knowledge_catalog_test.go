package service

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

type stubCatalogKBService struct {
	interfaces.KnowledgeBaseService
	kbs []*types.KnowledgeBase
	err error
}

func (s *stubCatalogKBService) ListKnowledgeBasesByTenantID(context.Context, uint64) ([]*types.KnowledgeBase, error) {
	return s.kbs, s.err
}

type stubCatalogShareService struct {
	interfaces.KBShareService
	items []*types.SharedKnowledgeBaseInfo
	err   error
}

func (s *stubCatalogShareService) ListSharedKnowledgeBases(
	context.Context, uint64, types.TenantRole,
) ([]*types.SharedKnowledgeBaseInfo, error) {
	return s.items, s.err
}

type stubCatalogKGRepo struct {
	interfaces.KnowledgeRepository
	counts     map[string]int64
	items      []*types.Knowledge
	lastTenant uint64
	lastKBID   string
}

func countKey(tenantID uint64, kbID string) string {
	return fmt.Sprintf("%d:%s", tenantID, kbID)
}

func (s *stubCatalogKGRepo) CountKnowledgeByKnowledgeBaseID(_ context.Context, tenantID uint64, kbID string) (int64, error) {
	if s.counts == nil {
		return 0, nil
	}
	return s.counts[countKey(tenantID, kbID)], nil
}

func (s *stubCatalogKGRepo) ListKnowledgeCatalogCursor(
	_ context.Context,
	tenantID uint64,
	kbID string,
	q types.KnowledgeCatalogCursorQuery,
) ([]*types.Knowledge, error) {
	s.lastTenant = tenantID
	s.lastKBID = kbID
	out := make([]*types.Knowledge, 0, len(s.items))
	for _, item := range s.items {
		if item.TenantID != tenantID || item.KnowledgeBaseID != kbID {
			continue
		}
		if q.ParseStatus != "" {
			if item.ParseStatus != q.ParseStatus {
				continue
			}
		} else if item.ParseStatus == types.ParseStatusDeleting {
			continue
		}
		if !q.UpdatedAfter.IsZero() && !item.UpdatedAt.After(q.UpdatedAfter) {
			continue
		}
		if q.HasCursor {
			if item.UpdatedAt.Before(q.CursorUpdatedAt) {
				continue
			}
			if item.UpdatedAt.Equal(q.CursorUpdatedAt) && item.ID <= q.CursorID {
				continue
			}
		}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].UpdatedAt.Equal(out[j].UpdatedAt) {
			return out[i].UpdatedAt.Before(out[j].UpdatedAt)
		}
		return out[i].ID < out[j].ID
	})
	if q.Limit > 0 && len(out) > q.Limit {
		out = out[:q.Limit]
	}
	return out, nil
}

func catalogAPIKeyCtx(tenantID uint64, role types.TenantRole, kbIDs []string) context.Context {
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, tenantID)
	ctx = context.WithValue(ctx, types.TenantRoleContextKey, role)
	return types.WithTenantAPIKeyScope(ctx, types.TenantAPIKeyScope{
		KeyID:            9,
		KnowledgeBaseIDs: kbIDs,
		Capabilities:     types.StringArray{string(types.APIKeyCapabilityRetrieve)},
	})
}

func catalogJWTCtx(tenantID uint64, role types.TenantRole) context.Context {
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, tenantID)
	return context.WithValue(ctx, types.TenantRoleContextKey, role)
}

func newCatalogFixture() (*KnowledgeCatalogService, *stubCatalogKGRepo, *stubCatalogShareService) {
	now := time.Date(2026, 8, 30, 8, 0, 0, 0, time.UTC)
	owned := []*types.KnowledgeBase{
		{ID: "kb-own-1", Name: "产品文档", Type: types.KnowledgeBaseTypeDocument, TenantID: 42, UpdatedAt: now},
		{ID: "kb-own-2", Name: "FAQ", Type: types.KnowledgeBaseTypeFAQ, TenantID: 42, UpdatedAt: now.Add(time.Hour)},
		{ID: "kb-temp", Name: "临时", Type: types.KnowledgeBaseTypeDocument, TenantID: 42, IsTemporary: true, UpdatedAt: now},
	}
	share := &stubCatalogShareService{
		items: []*types.SharedKnowledgeBaseInfo{
			{
				KnowledgeBase:  &types.KnowledgeBase{ID: "kb-share-9", Name: "集团制度", Type: types.KnowledgeBaseTypeDocument, TenantID: 7, UpdatedAt: now.Add(-time.Hour)},
				ShareID:        "share-1",
				OrganizationID: "org-1",
				Permission:     types.OrgRoleViewer,
				SourceTenantID: 7,
			},
			{
				KnowledgeBase:  &types.KnowledgeBase{ID: "kb-own-1", Name: "自己分享出去", Type: types.KnowledgeBaseTypeDocument, TenantID: 42, UpdatedAt: now},
				ShareID:        "share-self",
				OrganizationID: "org-1",
				Permission:     types.OrgRoleEditor,
				SourceTenantID: 42,
			},
		},
	}
	repo := &stubCatalogKGRepo{
		counts: map[string]int64{
			countKey(42, "kb-own-1"):   120,
			countKey(42, "kb-own-2"):   3,
			countKey(7, "kb-share-9"):  88,
			countKey(42, "kb-share-9"): 0,
		},
		items: []*types.Knowledge{
			{
				ID: "k-share-a", TenantID: 7, KnowledgeBaseID: "kb-share-9",
				Type: "file", Title: "报销制度.pdf", FileName: "报销制度.pdf", FileType: "pdf",
				FilePath: "must-not-leak", FileSize: 102400, FolderPath: "制度/财务",
				ParseStatus: types.ParseStatusCompleted, EnableStatus: "enabled",
				Channel: types.ChannelWeb, CreatedAt: now, UpdatedAt: now,
			},
		},
	}
	svc := NewKnowledgeCatalogService(&stubCatalogKBService{kbs: owned}, share, repo).(*KnowledgeCatalogService)
	return svc, repo, share
}

func TestListAuthorizedCatalogKBs_OwnedAndOrgShare(t *testing.T) {
	svc, _, _ := newCatalogFixture()
	ctx := catalogAPIKeyCtx(42, types.TenantRoleViewer, nil)

	got, err := svc.ListAuthorizedCatalogKBs(ctx, types.ListCatalogKBsQuery{IncludeOrgShared: true})
	require.NoError(t, err)
	require.Equal(t, uint64(42), got.TenantID)
	require.False(t, got.Truncated)
	require.Len(t, got.KnowledgeBases, 3)

	byID := map[string]types.CatalogKnowledgeBase{}
	for _, kb := range got.KnowledgeBases {
		byID[kb.ID] = kb
	}
	require.Contains(t, byID, "kb-own-1")
	require.Contains(t, byID, "kb-own-2")
	require.Contains(t, byID, "kb-share-9")
	require.NotContains(t, byID, "kb-temp")

	require.Equal(t, types.CatalogAccessOwned, byID["kb-own-1"].AccessSource)
	require.Equal(t, uint64(42), byID["kb-own-1"].OwnerTenantID)
	require.Equal(t, string(types.OrgRoleAdmin), byID["kb-own-1"].Permission)
	require.True(t, byID["kb-own-1"].CanDownload)
	require.Equal(t, int64(120), byID["kb-own-1"].KnowledgeCount)

	require.Equal(t, types.CatalogAccessOrgShare, byID["kb-share-9"].AccessSource)
	require.Equal(t, uint64(7), byID["kb-share-9"].OwnerTenantID)
	require.Equal(t, string(types.OrgRoleViewer), byID["kb-share-9"].Permission)
	require.False(t, byID["kb-share-9"].CanDownload)
	require.Equal(t, int64(88), byID["kb-share-9"].KnowledgeCount)
	require.Equal(t, "org-1", byID["kb-share-9"].OrganizationID)
	require.Equal(t, "share-1", byID["kb-share-9"].ShareID)

	ownedCount := 0
	for _, kb := range got.KnowledgeBases {
		if kb.ID == "kb-own-1" {
			ownedCount++
			require.Equal(t, types.CatalogAccessOwned, kb.AccessSource)
		}
	}
	require.Equal(t, 1, ownedCount)
}

func TestListAuthorizedCatalogKBs_ExcludeOrgShared(t *testing.T) {
	svc, _, _ := newCatalogFixture()
	ctx := catalogAPIKeyCtx(42, types.TenantRoleViewer, nil)

	got, err := svc.ListAuthorizedCatalogKBs(ctx, types.ListCatalogKBsQuery{IncludeOrgShared: false})
	require.NoError(t, err)
	require.Len(t, got.KnowledgeBases, 2)
	for _, kb := range got.KnowledgeBases {
		require.Equal(t, types.CatalogAccessOwned, kb.AccessSource)
		require.NotEqual(t, "kb-share-9", kb.ID)
	}
}

func TestListAuthorizedCatalogKBs_WhitelistDropsOtherKBs(t *testing.T) {
	svc, _, _ := newCatalogFixture()
	ctx := catalogAPIKeyCtx(42, types.TenantRoleViewer, []string{"kb-own-1"})

	got, err := svc.ListAuthorizedCatalogKBs(ctx, types.ListCatalogKBsQuery{IncludeOrgShared: true})
	require.NoError(t, err)
	require.Len(t, got.KnowledgeBases, 1)
	require.Equal(t, "kb-own-1", got.KnowledgeBases[0].ID)
}

func TestListAuthorizedCatalogKBs_RevokedShareDisappears(t *testing.T) {
	svc, _, share := newCatalogFixture()
	ctx := catalogAPIKeyCtx(42, types.TenantRoleViewer, nil)

	share.items = nil
	got, err := svc.ListAuthorizedCatalogKBs(ctx, types.ListCatalogKBsQuery{IncludeOrgShared: true})
	require.NoError(t, err)
	for _, kb := range got.KnowledgeBases {
		require.NotEqual(t, "kb-share-9", kb.ID)
	}

	_, err = svc.ListCatalogKnowledge(ctx, types.ListCatalogKnowledgeQuery{KBID: "kb-share-9", Limit: 100})
	require.ErrorIs(t, err, ErrCatalogKBNotFound)
}

func TestListCatalogKnowledge_SharedKBUsesOwnerTenant(t *testing.T) {
	svc, repo, _ := newCatalogFixture()
	ctx := catalogAPIKeyCtx(42, types.TenantRoleViewer, nil)

	got, err := svc.ListCatalogKnowledge(ctx, types.ListCatalogKnowledgeQuery{KBID: "kb-share-9", Limit: 100})
	require.NoError(t, err)
	require.Equal(t, uint64(7), repo.lastTenant)
	require.Equal(t, "kb-share-9", repo.lastKBID)
	require.Len(t, got.Items, 1)
	require.Equal(t, "k-share-a", got.Items[0].ID)
	require.True(t, got.Items[0].HasFile)
	require.Equal(t, "file", got.Items[0].KnowledgeType)
	require.Empty(t, got.NextCursor)
	require.False(t, got.HasMore)
}

func TestListCatalogKnowledge_UnauthorizedKBIDNotFound(t *testing.T) {
	svc, _, _ := newCatalogFixture()
	ctx := catalogAPIKeyCtx(42, types.TenantRoleViewer, []string{"kb-own-1"})

	_, err := svc.ListCatalogKnowledge(ctx, types.ListCatalogKnowledgeQuery{KBID: "kb-share-9"})
	require.ErrorIs(t, err, ErrCatalogKBNotFound)

	_, err = svc.ListCatalogKnowledge(ctx, types.ListCatalogKnowledgeQuery{KBID: "kb-foreign"})
	require.ErrorIs(t, err, ErrCatalogKBNotFound)
}

func TestListCatalogKnowledge_MissingKBIDAndInvalidLimit(t *testing.T) {
	svc, _, _ := newCatalogFixture()
	ctx := catalogAPIKeyCtx(42, types.TenantRoleViewer, nil)

	_, err := svc.ListCatalogKnowledge(ctx, types.ListCatalogKnowledgeQuery{})
	require.ErrorIs(t, err, ErrCatalogMissingKBID)

	_, err = svc.ListCatalogKnowledge(ctx, types.ListCatalogKnowledgeQuery{KBID: "kb-own-1", Limit: 0})
	require.NoError(t, err)

	_, err = svc.ListCatalogKnowledge(ctx, types.ListCatalogKnowledgeQuery{KBID: "kb-own-1", Limit: 501})
	require.ErrorIs(t, err, ErrCatalogInvalidLimit)
}

func TestListCatalogKnowledge_CursorSecondPageStable(t *testing.T) {
	svc, repo, _ := newCatalogFixture()
	now := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	repo.items = []*types.Knowledge{
		{ID: "k-1", TenantID: 42, KnowledgeBaseID: "kb-own-1", Type: "file", UpdatedAt: now, ParseStatus: types.ParseStatusCompleted},
		{ID: "k-2", TenantID: 42, KnowledgeBaseID: "kb-own-1", Type: "file", UpdatedAt: now.Add(time.Minute), ParseStatus: types.ParseStatusCompleted},
		{ID: "k-3", TenantID: 42, KnowledgeBaseID: "kb-own-1", Type: "manual", UpdatedAt: now.Add(2 * time.Minute), ParseStatus: types.ParseStatusFailed},
	}
	ctx := catalogAPIKeyCtx(42, types.TenantRoleViewer, nil)

	page1, err := svc.ListCatalogKnowledge(ctx, types.ListCatalogKnowledgeQuery{KBID: "kb-own-1", Limit: 2})
	require.NoError(t, err)
	require.True(t, page1.HasMore)
	require.NotEmpty(t, page1.NextCursor)
	require.Equal(t, []string{"k-1", "k-2"}, catalogItemIDs(page1.Items))

	page2, err := svc.ListCatalogKnowledge(ctx, types.ListCatalogKnowledgeQuery{
		KBID:   "kb-own-1",
		Limit:  2,
		Cursor: page1.NextCursor,
	})
	require.NoError(t, err)
	require.False(t, page2.HasMore)
	require.Empty(t, page2.NextCursor)
	require.Equal(t, []string{"k-3"}, catalogItemIDs(page2.Items))
}

func TestListCatalogKnowledge_JSONSafeFields(t *testing.T) {
	svc, _, _ := newCatalogFixture()
	ctx := catalogAPIKeyCtx(42, types.TenantRoleViewer, nil)

	got, err := svc.ListCatalogKnowledge(ctx, types.ListCatalogKnowledgeQuery{KBID: "kb-share-9"})
	require.NoError(t, err)
	require.Len(t, got.Items, 1)
	require.NotContains(t, fmt.Sprintf("%#v", got.Items[0]), "must-not-leak")
}

func TestCatalogCanDownload_JWTViewerOwnedIsFalse(t *testing.T) {
	svc, _, _ := newCatalogFixture()
	ctx := catalogJWTCtx(42, types.TenantRoleViewer)

	got, err := svc.ListAuthorizedCatalogKBs(ctx, types.ListCatalogKBsQuery{IncludeOrgShared: false})
	require.NoError(t, err)
	require.NotEmpty(t, got.KnowledgeBases)
	for _, kb := range got.KnowledgeBases {
		require.False(t, kb.CanDownload)
	}
}

func catalogItemIDs(items []types.CatalogKnowledgeItem) []string {
	ids := make([]string, len(items))
	for i, item := range items {
		ids[i] = item.ID
	}
	return ids
}
