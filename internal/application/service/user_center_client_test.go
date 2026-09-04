package service

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/Tencent/WeKnora/internal/config"
	"github.com/stretchr/testify/require"
)

func TestUserCenterAccessTokenIsMD5HexLower(t *testing.T) {
	got := userCenterAccessToken("secret", "1710000000000")
	sum := md5.Sum([]byte("secret1710000000000"))
	require.Equal(t, hex.EncodeToString(sum[:]), got)
}

func TestUserCenterFindByAuthorizedPhone(t *testing.T) {
	var sawPhone, sawSystemID, sawToken string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/person/findByAuthorizedPhone", r.URL.Path)
		require.Equal(t, http.MethodPost, r.Method)
		body, _ := io.ReadAll(r.Body)
		vals, err := url.ParseQuery(string(body))
		require.NoError(t, err)
		sawPhone = vals.Get("phone")
		sawSystemID = r.Header.Get("systemId")
		ts := r.Header.Get("timestamp")
		sawToken = r.Header.Get("accessToken")
		require.Equal(t, userCenterAccessToken("cert-value", ts), sawToken)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"data":{"id":1787410,"idStr":"1787410","loginName":"zhangsan","realName":"张三","email":"a@nxin.com","mobilePhone":"138****8000"}}`))
	}))
	t.Cleanup(srv.Close)

	c := &userCenterDirectoryClient{
		cfg: config.CASUserCenterConfig{
			URL:      srv.URL + "/",
			SystemID: "sys-1",
			Cert:     "cert-value",
		},
		httpClient: srv.Client(),
	}
	info, err := c.FindByAuthorizedPhone(context.Background(), "13800138000")
	require.NoError(t, err)
	require.NotNil(t, info)
	require.Equal(t, "1787410", info.ID)
	require.Equal(t, "张三", info.RealName)
	require.Equal(t, "zhangsan", info.LoginName)
	require.Equal(t, "13800138000", sawPhone)
	require.Equal(t, "sys-1", sawSystemID)
	require.NotEmpty(t, sawToken)
}

func TestUserCenterFindByAuthorizedPhoneMiss(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":0,"data":null}`))
	}))
	t.Cleanup(srv.Close)
	c := &userCenterDirectoryClient{
		cfg:        config.CASUserCenterConfig{URL: srv.URL, SystemID: "s", Cert: "c"},
		httpClient: srv.Client(),
	}
	info, err := c.FindByAuthorizedPhone(context.Background(), "13800138000")
	require.NoError(t, err)
	require.Nil(t, info)
}

func TestUserCenterSearchByNameOrPhoneList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/person/searchByNameOrPhone", r.URL.Path)
		_, _ = w.Write([]byte(`{"code":"0","data":[{"id":"1","realName":"张三"},{"id":2,"realName":"李四"}]}`))
	}))
	t.Cleanup(srv.Close)
	c := &userCenterDirectoryClient{
		cfg:        config.CASUserCenterConfig{URL: srv.URL, SystemID: "s", Cert: "c"},
		httpClient: srv.Client(),
	}
	list, err := c.SearchByNameOrPhone(context.Background(), "13800138000")
	require.NoError(t, err)
	require.Len(t, list, 2)
	require.Equal(t, "1", list[0].ID)
	require.Equal(t, "2", list[1].ID)
}

func TestUserCenterNotConfigured(t *testing.T) {
	t.Setenv("CAS_UC_URL", "")
	t.Setenv("CAS_UC_SYSTEM_ID", "")
	t.Setenv("CAS_UC_CERT", "")
	t.Setenv("CAS_USER_CENTER_URL", "")
	t.Setenv("CAS_USER_CENTER_SYSTEM_ID", "")
	t.Setenv("CAS_USER_CENTER_CERT", "")
	c := NewUserCenterDirectoryClient(&config.Config{CAS: &config.CASConfig{}})
	require.False(t, c.Configured())
	_, err := c.FindByAuthorizedPhone(context.Background(), "13800138000")
	require.Error(t, err)
}

func TestGetBoIDByZNTToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/login/get-boId-by-znt-token/tok-1", r.URL.Path)
		require.Empty(t, r.Header.Get("systemId"))
		_, _ = w.Write([]byte(`{"code":0,"data":1787161,"error":""}`))
	}))
	t.Cleanup(srv.Close)
	c := &userCenterDirectoryClient{
		cfg:        config.CASUserCenterConfig{URL: srv.URL + "/"},
		httpClient: srv.Client(),
	}
	id, err := c.GetBoIDByZNTToken(context.Background(), "tok-1")
	require.NoError(t, err)
	require.Equal(t, "1787161", id)
}

func TestGetBoIDByUcTicket(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/login/getUserByUcTicket", r.URL.Path)
		body, _ := io.ReadAll(r.Body)
		vals, _ := url.ParseQuery(string(body))
		require.Equal(t, "sid-1", vals.Get("ticket"))
		_, _ = w.Write([]byte(`{"code":0,"data":{"id":1787161}}`))
	}))
	t.Cleanup(srv.Close)
	c := &userCenterDirectoryClient{
		cfg:        config.CASUserCenterConfig{URL: srv.URL + "/"},
		httpClient: srv.Client(),
	}
	id, err := c.GetBoIDByUcTicket(context.Background(), "sid-1")
	require.NoError(t, err)
	require.Equal(t, "1787161", id)
}

func TestGetUserArchiveByBoID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/person/getUserArchive/1787161", r.URL.Path)
		require.Equal(t, "sys-1", r.Header.Get("systemId"))
		_, _ = w.Write([]byte(`{"code":0,"data":{"id":1787161,"idStr":"1787161","loginName":"u1","realName":"刘二","mobilePhone":"182****2222","unionId":"ff1bb4e8-85e9"}}`))
	}))
	t.Cleanup(srv.Close)
	c := &userCenterDirectoryClient{
		cfg: config.CASUserCenterConfig{
			URL: srv.URL + "/", SystemID: "sys-1", Cert: "cert-value",
		},
		httpClient: srv.Client(),
	}
	info, err := c.GetUserArchive(context.Background(), "1787161")
	require.NoError(t, err)
	require.Equal(t, "1787161", info.ID)
	require.Equal(t, "刘二", info.RealName)
	require.Equal(t, "u1", info.LoginName)
}

func TestGetBoIDByZNTTokenNonZeroCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":20003,"data":null,"error":"invalid token"}`))
	}))
	t.Cleanup(srv.Close)
	c := &userCenterDirectoryClient{
		cfg:        config.CASUserCenterConfig{URL: srv.URL + "/"},
		httpClient: srv.Client(),
	}
	_, err := c.GetBoIDByZNTToken(context.Background(), "bad-tok")
	require.Error(t, err)
	require.Contains(t, err.Error(), "20003")
}

func TestGetBoIDByUcTicketNonZeroCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":20010,"data":null}`))
	}))
	t.Cleanup(srv.Close)
	c := &userCenterDirectoryClient{
		cfg:        config.CASUserCenterConfig{URL: srv.URL + "/"},
		httpClient: srv.Client(),
	}
	_, err := c.GetBoIDByUcTicket(context.Background(), "bad-sid")
	require.Error(t, err)
	require.Contains(t, err.Error(), "20010")
}

func TestGetUserArchiveByBoIDNonZeroCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":20003,"data":null}`))
	}))
	t.Cleanup(srv.Close)
	c := &userCenterDirectoryClient{
		cfg: config.CASUserCenterConfig{
			URL: srv.URL + "/", SystemID: "sys-1", Cert: "cert-value",
		},
		httpClient: srv.Client(),
	}
	_, err := c.GetUserArchive(context.Background(), "1787161")
	require.Error(t, err)
	require.Contains(t, err.Error(), "20003")
	require.NotContains(t, err.Error(), "empty")
}
