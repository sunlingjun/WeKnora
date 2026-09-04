package service

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	apprepo "github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

type stubUCDir struct {
	find   map[string]*types.CASUserInfo
	search map[string][]*types.CASUserInfo
}

func (s *stubUCDir) Configured() bool { return true }
func (s *stubUCDir) HasBaseURL() bool { return true }
func (s *stubUCDir) FindByAuthorizedPhone(_ context.Context, phone string) (*types.CASUserInfo, error) {
	if s.find == nil {
		return nil, nil
	}
	return s.find[phone], nil
}
func (s *stubUCDir) SearchByNameOrPhone(_ context.Context, keyword string) ([]*types.CASUserInfo, error) {
	if s.search == nil {
		return nil, nil
	}
	return s.search[keyword], nil
}
func (s *stubUCDir) GetBoIDByZNTToken(context.Context, string) (string, error) {
	return "", errors.New("GetBoIDByZNTToken not stubbed")
}
func (s *stubUCDir) GetBoIDByUcTicket(context.Context, string) (string, error) {
	return "", errors.New("GetBoIDByUcTicket not stubbed")
}
func (s *stubUCDir) GetUserArchive(context.Context, string) (*types.CASUserInfo, error) {
	return nil, errors.New("GetUserArchive not stubbed")
}

type stubImportUserRepo struct {
	byCAS   map[string]*types.User
	byEmail map[string]*types.User
}

func (s *stubImportUserRepo) CreateUser(context.Context, *types.User) error {
	return errors.New("CreateUser must not be called from import service")
}
func (s *stubImportUserRepo) GetUserByID(context.Context, string) (*types.User, error) {
	return nil, apprepo.ErrUserNotFound
}
func (s *stubImportUserRepo) GetUsersByIDs(context.Context, []string) (map[string]*types.User, error) {
	return map[string]*types.User{}, nil
}
func (s *stubImportUserRepo) GetUserByEmail(_ context.Context, email string) (*types.User, error) {
	if u, ok := s.byEmail[email]; ok {
		return u, nil
	}
	return nil, apprepo.ErrUserNotFound
}
func (s *stubImportUserRepo) GetUserByUsername(context.Context, string) (*types.User, error) {
	return nil, apprepo.ErrUserNotFound
}
func (s *stubImportUserRepo) GetUserByTenantID(context.Context, uint64) (*types.User, error) {
	return nil, apprepo.ErrUserNotFound
}
func (s *stubImportUserRepo) GetUserByCASUserID(_ context.Context, casUserID string) (*types.User, error) {
	if u, ok := s.byCAS[casUserID]; ok {
		return u, nil
	}
	return nil, apprepo.ErrUserNotFound
}
func (s *stubImportUserRepo) UpdateUser(context.Context, *types.User) error { return nil }
func (s *stubImportUserRepo) DeleteUser(context.Context, string) error      { return nil }
func (s *stubImportUserRepo) ListUsers(context.Context, int, int) ([]*types.User, error) {
	return nil, nil
}
func (s *stubImportUserRepo) ListSystemAdmins(context.Context, int, int) ([]*types.User, int64, error) {
	return nil, 0, nil
}
func (s *stubImportUserRepo) RevokeSystemAdmin(context.Context, string, string) (*types.User, error) {
	return nil, nil
}
func (s *stubImportUserRepo) SearchUsers(context.Context, string, int) ([]*types.User, error) {
	return nil, nil
}

var _ interfaces.UserRepository = (*stubImportUserRepo)(nil)

type spyCASAuth struct {
	bindUserN   int
	bindTenantN int
	lastCAS     *types.CASUserInfo
	users       map[string]*types.User
}

func (s *spyCASAuth) ResolveCASUserFromCookies(context.Context, string, string) (*types.CASUserInfo, error) {
	return nil, errors.New("not used")
}
func (s *spyCASAuth) ValidateCASSession(context.Context, string, string, string) (*types.CASUserInfo, error) {
	return nil, errors.New("not used")
}
func (s *spyCASAuth) AutoBindUser(_ context.Context, cas *types.CASUserInfo) (*types.User, error) {
	s.bindUserN++
	s.lastCAS = cas
	if cas == nil || strings.TrimSpace(cas.ID) == "" {
		return nil, errors.New("AutoBindUser called without cas_user_id")
	}
	if s.users != nil {
		if u, ok := s.users[cas.ID]; ok {
			return u, nil
		}
	}
	return &types.User{ID: "u-" + cas.ID, CASUserID: cas.ID, Email: cas.Email, Username: cas.LoginName}, nil
}
func (s *spyCASAuth) AutoBindTenant(_ context.Context, _ *types.CASUserInfo, user *types.User) (*types.Tenant, error) {
	s.bindTenantN++
	return &types.Tenant{ID: user.TenantID}, nil
}

type stubImportMembers struct {
	members map[string]*types.TenantMember
	added   []string
}

func (s *stubImportMembers) AddMember(_ context.Context, userID string, tenantID uint64, role types.TenantRole, _ *string) (*types.TenantMember, error) {
	key := userID
	if s.members != nil {
		if _, ok := s.members[key]; ok {
			return nil, ErrMembershipAlreadyExists
		}
	}
	s.added = append(s.added, userID)
	m := &types.TenantMember{UserID: userID, TenantID: tenantID, Role: role, Status: types.TenantMemberStatusActive}
	if s.members == nil {
		s.members = map[string]*types.TenantMember{}
	}
	s.members[key] = m
	return m, nil
}
func (s *stubImportMembers) EnsureOwner(context.Context, string, uint64) (*types.TenantMember, error) {
	return nil, nil
}
func (s *stubImportMembers) GetMembership(_ context.Context, userID string, _ uint64) (*types.TenantMember, error) {
	if s.members == nil {
		return nil, nil
	}
	return s.members[userID], nil
}
func (s *stubImportMembers) ListByUser(context.Context, string) ([]*types.TenantMember, error) {
	return nil, nil
}
func (s *stubImportMembers) ListByTenant(context.Context, uint64) ([]*types.TenantMember, error) {
	return nil, nil
}
func (s *stubImportMembers) ListMembersPage(context.Context, uint64, string, int, int) ([]*types.TenantMember, int64, error) {
	return nil, 0, nil
}
func (s *stubImportMembers) HasAnyMembers(context.Context, uint64) (bool, error) { return false, nil }
func (s *stubImportMembers) UpdateRole(context.Context, string, uint64, types.TenantRole) error {
	return nil
}
func (s *stubImportMembers) RemoveMember(context.Context, string, uint64) error { return nil }

var _ interfaces.TenantMemberService = (*stubImportMembers)(nil)

type stubPrefUserSvc struct {
	last map[string]uint64
}

func (s *stubPrefUserSvc) UpdateUserPreferences(_ context.Context, userID string, patch types.UserPreferences) (types.UserPreferences, error) {
	if patch.LastActiveTenantID != nil {
		if s.last == nil {
			s.last = map[string]uint64{}
		}
		s.last[userID] = *patch.LastActiveTenantID
	}
	return patch, nil
}

func newImportSvc(dir *stubUCDir, repo *stubImportUserRepo, auth *spyCASAuth, members *stubImportMembers, prefs *stubPrefUserSvc) *casMemberImportService {
	if dir == nil {
		dir = &stubUCDir{}
	}
	if repo == nil {
		repo = &stubImportUserRepo{}
	}
	if auth == nil {
		auth = &spyCASAuth{}
	}
	if members == nil {
		members = &stubImportMembers{}
	}
	return &casMemberImportService{
		dir:           dir,
		userRepo:      repo,
		casAuth:       auth,
		memberService: members,
		userService:   prefs,
	}
}

func TestParseXLSXMarketCenterHeaders(t *testing.T) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	sheet := f.GetSheetName(0)
	require.NoError(t, f.SetCellValue(sheet, "A1", "序号"))
	require.NoError(t, f.SetCellValue(sheet, "B1", "单位"))
	require.NoError(t, f.SetCellValue(sheet, "C1", "姓名"))
	require.NoError(t, f.SetCellValue(sheet, "D1", "部门"))
	require.NoError(t, f.SetCellValue(sheet, "E1", "手机号"))
	require.NoError(t, f.SetCellValue(sheet, "A2", "1"))
	require.NoError(t, f.SetCellValue(sheet, "C2", "张三"))
	require.NoError(t, f.SetCellValue(sheet, "E2", "13800138000"))
	var buf bytes.Buffer
	require.NoError(t, f.Write(&buf))

	svc := &casMemberImportService{}
	rows, err := svc.ParseFile("市场中心.xlsx", &buf)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "13800138000", rows[0].Phone)
	require.Equal(t, "张三", rows[0].Name)
}

func TestParseCSVAndPhonesText(t *testing.T) {
	svc := &casMemberImportService{}
	rows, err := svc.ParseFile("a.csv", strings.NewReader("手机号,姓名\n13900139000,李四\n"))
	require.NoError(t, err)
	require.Equal(t, "13900139000", rows[0].Phone)
	require.Equal(t, "李四", rows[0].Name)

	pasted := svc.ParsePhonesText("13700137000\n13600136000,王五")
	require.Len(t, pasted, 2)
	require.Equal(t, "王五", pasted[1].Name)
}

func TestPreviewCreateUserAddMemberSkipConflict(t *testing.T) {
	dir := &stubUCDir{find: map[string]*types.CASUserInfo{
		"13800138000": {ID: "100", RealName: "新用户", Email: "new@nxin.com", LoginName: "new"},
		"13800138001": {ID: "101", RealName: "已有账号", Email: "old@nxin.com", LoginName: "old"},
		"13800138002": {ID: "102", RealName: "已是成员", Email: "mem@nxin.com", LoginName: "mem"},
		"13800138003": {ID: "103", RealName: "冲突", Email: "conflict@nxin.com", LoginName: "c"},
	}}
	repo := &stubImportUserRepo{
		byCAS: map[string]*types.User{
			"101": {ID: "u-101", CASUserID: "101", Email: "old@nxin.com"},
			"102": {ID: "u-102", CASUserID: "102", Email: "mem@nxin.com"},
		},
		byEmail: map[string]*types.User{
			"conflict@nxin.com": {ID: "u-other", CASUserID: "999", Email: "conflict@nxin.com"},
		},
	}
	members := &stubImportMembers{members: map[string]*types.TenantMember{
		"u-102": {UserID: "u-102", TenantID: 10384, Role: types.TenantRoleContributor, Status: types.TenantMemberStatusActive},
	}}
	svc := newImportSvc(dir, repo, nil, members, nil)
	preview, err := svc.Preview(context.Background(), 10384, []types.CASImportRow{
		{Row: 1, Phone: "13800138000", Name: "新用户"},
		{Row: 2, Phone: "13800138001", Name: "已有账号"},
		{Row: 3, Phone: "13800138002", Name: "已是成员"},
		{Row: 4, Phone: "13800138003", Name: "冲突"},
		{Row: 5, Phone: "not-a-phone", Name: "坏号"},
	})
	require.NoError(t, err)
	require.Equal(t, 1, preview.WillCreate)
	require.Equal(t, 1, preview.WillAdd)
	require.Equal(t, 1, preview.AlreadyMember)
	require.Equal(t, 1, preview.LocalConflict)
	require.Equal(t, 1, preview.InvalidPhone)
	require.Equal(t, types.CASImportActionCreateUser, preview.Rows[0].Action)
	require.Equal(t, types.CASImportActionAddMember, preview.Rows[1].Action)
	require.Equal(t, "138****8000", preview.Rows[0].PhoneMasked)
}

func TestPreviewNotFoundAndNameMismatch(t *testing.T) {
	dir := &stubUCDir{find: map[string]*types.CASUserInfo{
		"13800138009": {ID: "109", RealName: "真名", Email: "a@x.com"},
	}}
	svc := newImportSvc(dir, nil, nil, nil, nil)
	preview, err := svc.Preview(context.Background(), 1, []types.CASImportRow{
		{Row: 1, Phone: "13800138008", Name: "没有"},
		{Row: 2, Phone: "13800138009", Name: "Excel名"},
	})
	require.NoError(t, err)
	require.Equal(t, types.CASImportStatusNotFound, preview.Rows[0].Status)
	require.Equal(t, types.CASImportStatusNameMismatch, preview.Rows[1].Status)
}

func TestImportCreateUserAndSkipExistingMember(t *testing.T) {
	dir := &stubUCDir{find: map[string]*types.CASUserInfo{
		"13800138000": {ID: "200", RealName: "新建", Email: "n@nxin.com", LoginName: "n200"},
		"13800138001": {ID: "201", RealName: "成员", Email: "m@nxin.com", LoginName: "m201"},
		"13800138999": {ID: "404", RealName: "未找到", Email: "x@nxin.com"},
	}}
	// 13800138999 not in find → not_found; put only two numbers
	dir.find["13800138999"] = nil
	repo := &stubImportUserRepo{
		byCAS: map[string]*types.User{
			"201": {ID: "u-201", CASUserID: "201", Email: "m@nxin.com"},
		},
	}
	auth := &spyCASAuth{}
	members := &stubImportMembers{members: map[string]*types.TenantMember{
		"u-201": {UserID: "u-201", TenantID: 7, Status: types.TenantMemberStatusActive, Role: types.TenantRoleAdmin},
	}}
	prefs := &stubPrefUserSvc{}
	svc := newImportSvc(dir, repo, auth, members, prefs)
	inviter := "owner-1"
	result, err := svc.Import(context.Background(), 7, types.TenantRoleContributor, &inviter, []types.CASImportRow{
		{Row: 1, Phone: "13800138000", Name: "新建"},
		{Row: 2, Phone: "13800138001", Name: "成员"},
		{Row: 3, Phone: "13800138999", Name: "未找到"},
	})
	require.NoError(t, err)
	require.Equal(t, 1, result.Imported)
	require.Equal(t, 1, result.Skipped)
	require.Equal(t, 1, result.Failed)
	require.Equal(t, 1, auth.bindUserN, "only create_user row binds")
	require.Equal(t, 1, auth.bindTenantN)
	require.Equal(t, []string{"u-200"}, members.added)
	require.Equal(t, uint64(7), prefs.last["u-200"])
	require.Equal(t, types.TenantRoleAdmin, members.members["u-201"].Role, "existing role must not change")
}

func TestImportAddMemberDoesNotCreateUserRow(t *testing.T) {
	dir := &stubUCDir{find: map[string]*types.CASUserInfo{
		"13800138010": {ID: "310", RealName: "老用户", Email: "old@nxin.com", LoginName: "old"},
	}}
	existing := &types.User{ID: "u-310", CASUserID: "310", Email: "old@nxin.com"}
	repo := &stubImportUserRepo{byCAS: map[string]*types.User{"310": existing}}
	auth := &spyCASAuth{users: map[string]*types.User{"310": existing}}
	members := &stubImportMembers{}
	svc := newImportSvc(dir, repo, auth, members, &stubPrefUserSvc{})
	result, err := svc.Import(context.Background(), 9, types.TenantRoleViewer, nil, []types.CASImportRow{
		{Row: 1, Phone: "13800138010", Name: "老用户"},
	})
	require.NoError(t, err)
	require.Equal(t, 1, result.Imported)
	require.Equal(t, types.CASImportActionAddMember, result.Rows[0].Action)
	require.Equal(t, []string{"u-310"}, members.added)
	require.Equal(t, "310", auth.lastCAS.ID)
}

func TestImportNeverCreateUserWithoutCASID(t *testing.T) {
	dir := &stubUCDir{find: map[string]*types.CASUserInfo{
		"13800138011": {ID: "", RealName: "无ID", Email: "x@nxin.com"},
	}}
	auth := &spyCASAuth{}
	svc := newImportSvc(dir, nil, auth, nil, nil)
	result, err := svc.Import(context.Background(), 1, types.TenantRoleContributor, nil, []types.CASImportRow{
		{Row: 1, Phone: "13800138011", Name: "无ID"},
	})
	require.NoError(t, err)
	require.Equal(t, 0, result.Imported)
	require.Equal(t, 0, auth.bindUserN)
}

func TestImportRejectsOwnerRole(t *testing.T) {
	svc := newImportSvc(&stubUCDir{find: map[string]*types.CASUserInfo{
		"13800138000": {ID: "1", RealName: "a"},
	}}, nil, nil, nil, nil)
	_, err := svc.Import(context.Background(), 1, types.TenantRoleOwner, nil, []types.CASImportRow{
		{Row: 1, Phone: "13800138000"},
	})
	require.ErrorIs(t, err, ErrCASImportOwnerRole)
}

func TestImportIdempotentSecondPass(t *testing.T) {
	dir := &stubUCDir{find: map[string]*types.CASUserInfo{
		"13800138020": {ID: "420", RealName: "再导", Email: "z@nxin.com"},
	}}
	auth := &spyCASAuth{}
	members := &stubImportMembers{}
	svc := newImportSvc(dir, &stubImportUserRepo{}, auth, members, &stubPrefUserSvc{})
	rows := []types.CASImportRow{{Row: 1, Phone: "13800138020", Name: "再导"}}
	first, err := svc.Import(context.Background(), 3, types.TenantRoleContributor, nil, rows)
	require.NoError(t, err)
	require.Equal(t, 1, first.Imported)
	second, err := svc.Import(context.Background(), 3, types.TenantRoleContributor, nil, rows)
	require.NoError(t, err)
	require.Equal(t, 1, second.Skipped)
	require.Equal(t, 0, second.Imported)
}

func TestNormalizeAndMaskPhone(t *testing.T) {
	got, ok := normalizeCNMobile("+86 138-0013-8000")
	require.True(t, ok)
	require.Equal(t, "13800138000", got)
	require.Equal(t, "138****8000", maskCNMobile(got))
	_, ok = normalizeCNMobile("12345")
	require.False(t, ok)
}
