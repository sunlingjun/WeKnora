package service

import (
	"context"
	"errors"
	"testing"
	"time"

	apprepo "github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeCASTenantSvc struct {
	byID       map[uint64]*types.Tenant
	getErr     map[uint64]error
	created    []*types.Tenant
	deletedIDs []uint64
	nextID     uint64
}

func (f *fakeCASTenantSvc) CreateTenant(_ context.Context, tenant *types.Tenant) (*types.Tenant, error) {
	if f.nextID == 0 {
		f.nextID = 9000
	}
	f.nextID++
	cp := *tenant
	cp.ID = f.nextID
	cp.CreatedAt = time.Now()
	if f.byID == nil {
		f.byID = map[uint64]*types.Tenant{}
	}
	f.byID[cp.ID] = &cp
	f.created = append(f.created, &cp)
	return &cp, nil
}

func (f *fakeCASTenantSvc) GetTenantByID(_ context.Context, id uint64) (*types.Tenant, error) {
	if err, ok := f.getErr[id]; ok {
		return nil, err
	}
	t, ok := f.byID[id]
	if !ok || t == nil {
		return nil, apprepo.ErrTenantNotFound
	}
	return t, nil
}

func (f *fakeCASTenantSvc) GetTenantsByIDs(_ context.Context, ids []uint64) (map[uint64]*types.Tenant, error) {
	out := map[uint64]*types.Tenant{}
	for _, id := range ids {
		if t, ok := f.byID[id]; ok && t != nil {
			out[id] = t
		}
	}
	return out, nil
}

func (f *fakeCASTenantSvc) DeleteTenant(_ context.Context, id uint64) error {
	f.deletedIDs = append(f.deletedIDs, id)
	delete(f.byID, id)
	return nil
}

func (f *fakeCASTenantSvc) ListTenants(context.Context) ([]*types.Tenant, error) {
	return nil, nil
}
func (f *fakeCASTenantSvc) UpdateTenant(context.Context, *types.Tenant) (*types.Tenant, error) {
	return nil, nil
}
func (f *fakeCASTenantSvc) UpdateAPIKey(context.Context, uint64) (string, error) { return "", nil }
func (f *fakeCASTenantSvc) ExtractTenantIDFromAPIKey(string) (uint64, error)     { return 0, nil }
func (f *fakeCASTenantSvc) ListAllTenants(context.Context) ([]*types.Tenant, error) {
	return nil, nil
}
func (f *fakeCASTenantSvc) BulkSetStorageQuota(context.Context, int64) (int64, error) {
	return 0, nil
}
func (f *fakeCASTenantSvc) SearchTenants(context.Context, string, uint64, int, int) ([]*types.Tenant, int64, error) {
	return nil, 0, nil
}
func (f *fakeCASTenantSvc) GetTenantByIDForUser(context.Context, uint64, string) (*types.Tenant, error) {
	return nil, nil
}
func (f *fakeCASTenantSvc) GetWeKnoraCloudCredentials(context.Context) *types.WeKnoraCloudCredentials {
	return nil
}

var _ interfaces.TenantService = (*fakeCASTenantSvc)(nil)

type fakeCASUserRepo struct {
	updated []*types.User
}

func (f *fakeCASUserRepo) CreateUser(context.Context, *types.User) error { return nil }
func (f *fakeCASUserRepo) GetUserByID(context.Context, string) (*types.User, error) {
	return nil, nil
}
func (f *fakeCASUserRepo) GetUsersByIDs(context.Context, []string) (map[string]*types.User, error) {
	return nil, nil
}
func (f *fakeCASUserRepo) GetUserByEmail(context.Context, string) (*types.User, error) {
	return nil, nil
}
func (f *fakeCASUserRepo) GetUserByUsername(context.Context, string) (*types.User, error) {
	return nil, nil
}
func (f *fakeCASUserRepo) GetUserByTenantID(context.Context, uint64) (*types.User, error) {
	return nil, nil
}
func (f *fakeCASUserRepo) GetUserByCASUserID(context.Context, string) (*types.User, error) {
	return nil, nil
}
func (f *fakeCASUserRepo) UpdateUser(_ context.Context, user *types.User) error {
	cp := *user
	f.updated = append(f.updated, &cp)
	return nil
}
func (f *fakeCASUserRepo) DeleteUser(context.Context, string) error { return nil }
func (f *fakeCASUserRepo) ListUsers(context.Context, int, int) ([]*types.User, error) {
	return nil, nil
}
func (f *fakeCASUserRepo) ListSystemAdmins(context.Context, int, int) ([]*types.User, int64, error) {
	return nil, 0, nil
}
func (f *fakeCASUserRepo) RevokeSystemAdmin(context.Context, string, string) (*types.User, error) {
	return nil, nil
}
func (f *fakeCASUserRepo) SearchUsers(context.Context, string, int) ([]*types.User, error) {
	return nil, nil
}

var _ interfaces.UserRepository = (*fakeCASUserRepo)(nil)

type fakeCASMemberSvc struct {
	members      []*types.TenantMember
	ensureCalls  []uint64
	ensureUserID string
}

func (f *fakeCASMemberSvc) AddMember(context.Context, string, uint64, types.TenantRole, *string) (*types.TenantMember, error) {
	return nil, nil
}
func (f *fakeCASMemberSvc) EnsureOwner(_ context.Context, userID string, tenantID uint64) (*types.TenantMember, error) {
	f.ensureUserID = userID
	f.ensureCalls = append(f.ensureCalls, tenantID)
	return &types.TenantMember{UserID: userID, TenantID: tenantID, Role: types.TenantRoleOwner}, nil
}
func (f *fakeCASMemberSvc) GetMembership(context.Context, string, uint64) (*types.TenantMember, error) {
	return nil, nil
}
func (f *fakeCASMemberSvc) ListByUser(context.Context, string) ([]*types.TenantMember, error) {
	return f.members, nil
}
func (f *fakeCASMemberSvc) ListByTenant(context.Context, uint64) ([]*types.TenantMember, error) {
	return nil, nil
}
func (f *fakeCASMemberSvc) ListMembersPage(context.Context, uint64, string, int, int) ([]*types.TenantMember, int64, error) {
	return nil, 0, nil
}
func (f *fakeCASMemberSvc) HasAnyMembers(context.Context, uint64) (bool, error) { return false, nil }
func (f *fakeCASMemberSvc) UpdateRole(context.Context, string, uint64, types.TenantRole) error {
	return nil
}
func (f *fakeCASMemberSvc) RemoveMember(context.Context, string, uint64) error { return nil }

var _ interfaces.TenantMemberService = (*fakeCASMemberSvc)(nil)

func newCASAuthForTest(ts *fakeCASTenantSvc, ur *fakeCASUserRepo, ms *fakeCASMemberSvc) *casAuthService {
	return &casAuthService{
		userRepo:      ur,
		tenantService: ts,
		memberService: ms,
	}
}

func TestAutoBindTenant_HomeValid_ReusesAndEnsureOwner(t *testing.T) {
	ts := &fakeCASTenantSvc{byID: map[uint64]*types.Tenant{
		10035: {ID: 10035, Name: "刘二的工作空间", StorageUsed: 100, CreatedAt: time.Date(2026, 1, 23, 0, 0, 0, 0, time.UTC)},
	}}
	ur := &fakeCASUserRepo{}
	ms := &fakeCASMemberSvc{}
	svc := newCASAuthForTest(ts, ur, ms)

	user := &types.User{ID: "u1", Username: "u1", TenantID: 10035}
	got, err := svc.AutoBindTenant(context.Background(), &types.CASUserInfo{RealName: "刘二"}, user)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, uint64(10035), got.ID)
	assert.Empty(t, ts.created)
	assert.Equal(t, []uint64{10035}, ms.ensureCalls)
}

func TestAutoBindTenant_HomeSoftDeleted_RecoversFromOwnerMembership(t *testing.T) {
	old := time.Date(2026, 1, 23, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC)
	ts := &fakeCASTenantSvc{
		byID: map[uint64]*types.Tenant{
			10035: {ID: 10035, Name: "刘二的工作空间", StorageUsed: 112963048, CreatedAt: old},
			10043: {ID: 10043, Name: "刘二的工作空间", StorageUsed: 0, CreatedAt: newer},
		},
		getErr: map[uint64]error{10042: apprepo.ErrTenantNotFound},
	}
	ur := &fakeCASUserRepo{}
	ms := &fakeCASMemberSvc{members: []*types.TenantMember{
		{UserID: "u1", TenantID: 10035, Role: types.TenantRoleOwner, Status: types.TenantMemberStatusActive},
		{UserID: "u1", TenantID: 10043, Role: types.TenantRoleOwner, Status: types.TenantMemberStatusActive},
	}}
	svc := newCASAuthForTest(ts, ur, ms)

	user := &types.User{ID: "u1", Username: "u1", TenantID: 10042}
	got, err := svc.AutoBindTenant(context.Background(), &types.CASUserInfo{RealName: "刘二"}, user)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, uint64(10035), got.ID, "should prefer tenant with storage data")
	assert.Empty(t, ts.created)
	assert.Equal(t, uint64(10035), user.TenantID)
	require.Len(t, ur.updated, 1)
	assert.Equal(t, uint64(10035), ur.updated[0].TenantID)
	assert.Equal(t, []uint64{10035}, ms.ensureCalls)
}

func TestAutoBindTenant_HomeDecryptError_DoesNotCreate(t *testing.T) {
	decryptErr := errors.New("decrypt tenants.api_key (id=10035): key missing")
	ts := &fakeCASTenantSvc{
		getErr: map[uint64]error{10035: decryptErr},
	}
	ur := &fakeCASUserRepo{}
	ms := &fakeCASMemberSvc{}
	svc := newCASAuthForTest(ts, ur, ms)

	user := &types.User{ID: "u1", Username: "u1", TenantID: 10035}
	got, err := svc.AutoBindTenant(context.Background(), &types.CASUserInfo{RealName: "刘二"}, user)
	require.Error(t, err)
	assert.Nil(t, got)
	assert.Empty(t, ts.created)
	assert.Contains(t, err.Error(), "load home tenant")
}

func TestAutoBindTenant_NoHomeNoMembership_CreatesDefault(t *testing.T) {
	ts := &fakeCASTenantSvc{byID: map[uint64]*types.Tenant{}}
	ur := &fakeCASUserRepo{}
	ms := &fakeCASMemberSvc{}
	svc := newCASAuthForTest(ts, ur, ms)

	user := &types.User{ID: "u1", Username: "YYN", TenantID: 0}
	got, err := svc.AutoBindTenant(context.Background(), &types.CASUserInfo{RealName: "刘二"}, user)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "刘二的工作空间", got.Name)
	assert.Equal(t, "默认工作空间", got.Description)
	assert.Equal(t, got.ID, user.TenantID)
	assert.Equal(t, []uint64{got.ID}, ms.ensureCalls)
}

func TestAutoBindTenant_ViewerOnlyMembership_CreatesPersonalHome(t *testing.T) {
	ts := &fakeCASTenantSvc{byID: map[uint64]*types.Tenant{
		200: {ID: 200, Name: "别人的空间", StorageUsed: 99, CreatedAt: time.Now()},
	}}
	ur := &fakeCASUserRepo{}
	ms := &fakeCASMemberSvc{members: []*types.TenantMember{
		{UserID: "u1", TenantID: 200, Role: types.TenantRoleViewer, Status: types.TenantMemberStatusActive},
	}}
	svc := newCASAuthForTest(ts, ur, ms)

	user := &types.User{ID: "u1", Username: "u1", TenantID: 0}
	got, err := svc.AutoBindTenant(context.Background(), &types.CASUserInfo{RealName: "刘二"}, user)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.NotEqual(t, uint64(200), got.ID, "must not promote viewer-only collab tenant to home")
	assert.Equal(t, "刘二的工作空间", got.Name)
}
