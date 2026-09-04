package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

type fakeUCDir struct {
	hasURL     bool
	configured bool
	znt        map[string]string
	sid        map[string]string
	arch       map[string]*types.CASUserInfo
	zntErr     error
	sidErr     error
	archErr    error
	zntCalls   int
	sidCalls   int
}

func (f *fakeUCDir) Configured() bool { return f.configured }
func (f *fakeUCDir) HasBaseURL() bool { return f.hasURL }
func (f *fakeUCDir) FindByAuthorizedPhone(context.Context, string) (*types.CASUserInfo, error) {
	return nil, nil
}
func (f *fakeUCDir) SearchByNameOrPhone(context.Context, string) ([]*types.CASUserInfo, error) {
	return nil, nil
}
func (f *fakeUCDir) GetBoIDByZNTToken(_ context.Context, token string) (string, error) {
	f.zntCalls++
	if f.zntErr != nil {
		return "", f.zntErr
	}
	if f.znt == nil {
		return "", errors.New("znt not stubbed")
	}
	id, ok := f.znt[token]
	if !ok {
		return "", errors.New("znt unknown token")
	}
	return id, nil
}
func (f *fakeUCDir) GetBoIDByUcTicket(_ context.Context, ticket string) (string, error) {
	f.sidCalls++
	if f.sidErr != nil {
		return "", f.sidErr
	}
	if f.sid == nil {
		return "", errors.New("sid not stubbed")
	}
	id, ok := f.sid[ticket]
	if !ok {
		return "", errors.New("sid unknown ticket")
	}
	return id, nil
}
func (f *fakeUCDir) GetUserArchive(_ context.Context, boID string) (*types.CASUserInfo, error) {
	if f.archErr != nil {
		return nil, f.archErr
	}
	if f.arch == nil {
		return nil, errors.New("archive not stubbed")
	}
	info, ok := f.arch[boID]
	if !ok {
		return nil, errors.New("archive unknown boID")
	}
	return info, nil
}

var _ interfaces.UserCenterDirectory = (*fakeUCDir)(nil)

func newCASAuthWithDir(dir interfaces.UserCenterDirectory) *casAuthService {
	return &casAuthService{userCenter: dir}
}

func TestResolvePrefersTicketCookieOverSid(t *testing.T) {
	dir := &fakeUCDir{
		hasURL:     true,
		configured: true,
		znt:        map[string]string{"tk": "100"},
		sid:        map[string]string{"sid": "200"},
		arch: map[string]*types.CASUserInfo{
			"100": {ID: "100", LoginName: "from-znt"},
			"200": {ID: "200", LoginName: "from-sid"},
		},
	}
	svc := newCASAuthWithDir(dir)
	info, err := svc.ResolveCASUserFromCookies(context.Background(), "tk", "sid")
	require.NoError(t, err)
	require.Equal(t, "100", info.ID)
	require.Equal(t, "from-znt", info.LoginName)
	require.Equal(t, 1, dir.zntCalls)
	require.Equal(t, 0, dir.sidCalls)
}

func TestResolveFallsToSidWhenNoTicketCookie(t *testing.T) {
	dir := &fakeUCDir{
		hasURL:     true,
		configured: true,
		znt:        map[string]string{"tk": "100"},
		sid:        map[string]string{"sid": "200"},
		arch: map[string]*types.CASUserInfo{
			"100": {ID: "100", LoginName: "from-znt"},
			"200": {ID: "200", LoginName: "from-sid"},
		},
	}
	svc := newCASAuthWithDir(dir)
	info, err := svc.ResolveCASUserFromCookies(context.Background(), "", "sid")
	require.NoError(t, err)
	require.Equal(t, "200", info.ID)
	require.Equal(t, "from-sid", info.LoginName)
	require.Equal(t, 0, dir.zntCalls)
	require.Equal(t, 1, dir.sidCalls)
}

func TestResolveMissingBoth(t *testing.T) {
	dir := &fakeUCDir{hasURL: true, configured: true}
	svc := newCASAuthWithDir(dir)
	_, err := svc.ResolveCASUserFromCookies(context.Background(), "", "")
	require.ErrorIs(t, err, types.ErrCASCredentialsMissing)
}

func TestResolveTicketInvalidDoesNotFallToSid(t *testing.T) {
	dir := &fakeUCDir{
		hasURL:     true,
		configured: true,
		zntErr:     errors.New("znt rejected"),
		sid:        map[string]string{"sid": "200"},
		arch: map[string]*types.CASUserInfo{
			"200": {ID: "200", LoginName: "from-sid"},
		},
	}
	svc := newCASAuthWithDir(dir)
	_, err := svc.ResolveCASUserFromCookies(context.Background(), "tk", "sid")
	require.ErrorIs(t, err, types.ErrCASTicketInvalid)
	require.Equal(t, 1, dir.zntCalls)
	require.Equal(t, 0, dir.sidCalls)
}

func TestResolveUserCenterUnavailable(t *testing.T) {
	svc := newCASAuthWithDir(nil)
	_, err := svc.ResolveCASUserFromCookies(context.Background(), "tk", "")
	require.ErrorIs(t, err, types.ErrCASUserCenterUnavailable)

	dir := &fakeUCDir{hasURL: false, configured: false}
	svc = newCASAuthWithDir(dir)
	_, err = svc.ResolveCASUserFromCookies(context.Background(), "tk", "")
	require.ErrorIs(t, err, types.ErrCASUserCenterUnavailable)
}

func TestResolveArchiveRequiresConfigured(t *testing.T) {
	dir := &fakeUCDir{
		hasURL:     true,
		configured: false,
		znt:        map[string]string{"tk": "100"},
	}
	svc := newCASAuthWithDir(dir)
	_, err := svc.ResolveCASUserFromCookies(context.Background(), "tk", "")
	require.ErrorIs(t, err, types.ErrCASUserCenterUnavailable)
}
