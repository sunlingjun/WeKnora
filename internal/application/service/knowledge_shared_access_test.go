package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type fakeKBShareService struct {
	allowedKBs map[string]bool
}

func (f *fakeKBShareService) ShareKnowledgeBase(context.Context, string, string, string, uint64, types.OrgMemberRole) (*types.KnowledgeBaseShare, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeKBShareService) UpdateSharePermission(context.Context, string, types.OrgMemberRole, string, uint64) error {
	return errors.New("not implemented")
}
func (f *fakeKBShareService) RemoveShare(context.Context, string, string, uint64) error {
	return errors.New("not implemented")
}
func (f *fakeKBShareService) ListSharesByKnowledgeBase(context.Context, string, uint64) ([]*types.KnowledgeBaseShare, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeKBShareService) ListSharesByOrganization(context.Context, string) ([]*types.KnowledgeBaseShare, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeKBShareService) ListSharedKnowledgeBases(context.Context, uint64, types.TenantRole) ([]*types.SharedKnowledgeBaseInfo, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeKBShareService) ListSharedKnowledgeBasesInOrganization(context.Context, string, uint64, types.TenantRole) ([]*types.OrganizationSharedKnowledgeBaseItem, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeKBShareService) ListSharedKnowledgeBaseIDsByOrganizations(context.Context, []string, uint64) (map[string][]string, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeKBShareService) GetShare(context.Context, string) (*types.KnowledgeBaseShare, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeKBShareService) GetShareByKBAndOrg(context.Context, string, string) (*types.KnowledgeBaseShare, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeKBShareService) CheckTenantKBPermission(context.Context, string, uint64, types.TenantRole) (types.OrgMemberRole, bool, error) {
	return "", false, errors.New("not implemented")
}
func (f *fakeKBShareService) HasTenantKBPermission(ctx context.Context, kbID string, callerTenantID uint64, callerTenantRole types.TenantRole, requiredRole types.OrgMemberRole) (bool, error) {
	return f.allowedKBs[kbID], nil
}
func (f *fakeKBShareService) GetKBSourceTenant(context.Context, string) (uint64, error) {
	return 0, errors.New("not implemented")
}
func (f *fakeKBShareService) CountSharesByKnowledgeBaseIDs(context.Context, []string) (map[string]int64, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeKBShareService) CountByOrganizations(context.Context, []string) (map[string]int64, error) {
	return nil, errors.New("not implemented")
}

func setupKnowledgeSharedAccessDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&types.Knowledge{}))
	return db
}

func newKnowledgeSharedAccessService(t *testing.T, kbShare interfaces.KBShareService) (*knowledgeService, *gorm.DB) {
	t.Helper()

	db := setupKnowledgeSharedAccessDB(t)
	repo := repository.NewKnowledgeRepository(db)
	return &knowledgeService{
		repo:           repo,
		kbShareService: kbShare,
	}, db
}

func newSharedAccessContext() context.Context {
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(1))
	ctx = context.WithValue(ctx, types.UserIDContextKey, "user-1")
	return ctx
}

func seedKnowledge(t *testing.T, db *gorm.DB, knowledge *types.Knowledge) {
	t.Helper()
	require.NoError(t, db.Create(knowledge).Error)
}

func TestGetKnowledgeBatchWithSharedAccess_IncludesSharedKnowledgeWithPermission(t *testing.T) {
	service, db := newKnowledgeSharedAccessService(t, &fakeKBShareService{
		allowedKBs: map[string]bool{"kb-shared": true},
	})

	now := time.Now()
	sharedKnowledge := &types.Knowledge{
		ID:              "k-shared",
		TenantID:        2,
		KnowledgeBaseID: "kb-shared",
		Type:            "file",
		Title:           "shared doc",
		FileName:        "shared.txt",
		FileType:        "txt",
		ParseStatus:     types.ParseStatusCompleted,
		EnableStatus:    "enabled",
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	seedKnowledge(t, db, sharedKnowledge)

	got, err := service.GetKnowledgeBatchWithSharedAccess(newSharedAccessContext(), 1, []string{"k-shared"})

	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "k-shared", got[0].ID)
	require.Equal(t, uint64(2), got[0].TenantID)
	require.Equal(t, "kb-shared", got[0].KnowledgeBaseID)
}

func TestGetKnowledgeBatchWithSharedAccess_ExcludesSharedKnowledgeWithoutPermission(t *testing.T) {
	service, db := newKnowledgeSharedAccessService(t, &fakeKBShareService{
		allowedKBs: map[string]bool{},
	})

	now := time.Now()
	sharedKnowledge := &types.Knowledge{
		ID:              "k-shared",
		TenantID:        2,
		KnowledgeBaseID: "kb-shared",
		Type:            "file",
		Title:           "shared doc",
		FileName:        "shared.txt",
		FileType:        "txt",
		ParseStatus:     types.ParseStatusCompleted,
		EnableStatus:    "enabled",
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	seedKnowledge(t, db, sharedKnowledge)

	got, err := service.GetKnowledgeBatchWithSharedAccess(newSharedAccessContext(), 1, []string{"k-shared"})

	require.NoError(t, err)
	require.Empty(t, got)
}

// fakePlazaSharedKBService only implements GetMemberRoleByKBAndUser for list-tenant resolution tests.
type fakePlazaSharedKBService struct {
	interfaces.SharedKnowledgeBaseService
	roles map[string]string // "kbID:userID" -> role
}

func (f *fakePlazaSharedKBService) GetMemberRoleByKBAndUser(_ context.Context, kbID, userID string) (string, error) {
	if f.roles == nil {
		return "", nil
	}
	return f.roles[kbID+":"+userID], nil
}

type fakeKBServiceForListTenant struct {
	interfaces.KnowledgeBaseService
	kbs map[string]*types.KnowledgeBase
}

func (f *fakeKBServiceForListTenant) GetKnowledgeBaseByID(_ context.Context, id string) (*types.KnowledgeBase, error) {
	kb, ok := f.kbs[id]
	if !ok || kb == nil {
		return nil, repository.ErrKnowledgeBaseNotFound
	}
	return kb, nil
}

func TestListPagedKnowledgeByKnowledgeBaseID_UsesOwnerTenantForPlazaMember(t *testing.T) {
	db := setupKnowledgeSharedAccessDB(t)
	repo := repository.NewKnowledgeRepository(db)
	svc := &knowledgeService{
		repo:      repo,
		kbService: &fakeKBServiceForListTenant{kbs: map[string]*types.KnowledgeBase{
			"kb-plaza": {ID: "kb-plaza", TenantID: 10038, Visibility: types.KnowledgeBaseVisibilityShared},
		}},
		sharedKBService: &fakePlazaSharedKBService{roles: map[string]string{"kb-plaza:user-1": types.KBMemberRoleViewer}},
	}

	now := time.Now()
	seedKnowledge(t, db, &types.Knowledge{
		ID: "k-plaza-1", TenantID: 10038, KnowledgeBaseID: "kb-plaza",
		Type: "file", Title: "课程内容", FileName: "课程内容.docx", FileType: "docx",
		ParseStatus: types.ParseStatusCompleted, EnableStatus: "enabled",
		CreatedAt: now, UpdatedAt: now,
	})

	// Caller is workspace 10035 (same as the reported 学联网 bug).
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(10035))
	ctx = context.WithValue(ctx, types.UserIDContextKey, "user-1")

	result, err := svc.ListPagedKnowledgeByKnowledgeBaseID(ctx, "kb-plaza", &types.Pagination{Page: 1, PageSize: 10}, types.KnowledgeListFilter{
		ParseStatus: types.ParseStatusCompleted,
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, int64(1), result.Total)
	docs, ok := result.Data.([]*types.Knowledge)
	require.True(t, ok)
	require.Len(t, docs, 1)
	require.Equal(t, "k-plaza-1", docs[0].ID)
}

func TestListPagedKnowledgeByKnowledgeBaseID_KeepsCallerTenantWithoutPlazaAccess(t *testing.T) {
	db := setupKnowledgeSharedAccessDB(t)
	repo := repository.NewKnowledgeRepository(db)
	svc := &knowledgeService{
		repo:      repo,
		kbService: &fakeKBServiceForListTenant{kbs: map[string]*types.KnowledgeBase{
			"kb-plaza": {ID: "kb-plaza", TenantID: 10038, Visibility: types.KnowledgeBaseVisibilityShared},
		}},
		sharedKBService: &fakePlazaSharedKBService{roles: map[string]string{}},
	}

	now := time.Now()
	seedKnowledge(t, db, &types.Knowledge{
		ID: "k-plaza-1", TenantID: 10038, KnowledgeBaseID: "kb-plaza",
		Type: "file", Title: "课程内容", FileName: "课程内容.docx", FileType: "docx",
		ParseStatus: types.ParseStatusCompleted, EnableStatus: "enabled",
		CreatedAt: now, UpdatedAt: now,
	})

	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(10035))
	ctx = context.WithValue(ctx, types.UserIDContextKey, "user-1")

	result, err := svc.ListPagedKnowledgeByKnowledgeBaseID(ctx, "kb-plaza", &types.Pagination{Page: 1, PageSize: 10}, types.KnowledgeListFilter{})
	require.NoError(t, err)
	require.Equal(t, int64(0), result.Total)
}

var _ interfaces.KBShareService = (*fakeKBShareService)(nil)
