package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveUserCenterEnvOverrides(t *testing.T) {
	cfg := &CASConfig{UserCenter: &CASUserCenterConfig{
		URL:      "https://from-yaml/",
		SystemID: "yaml-sys",
		Cert:     "yaml-cert",
	}}
	t.Setenv("CAS_UC_URL", "https://from-env/")
	t.Setenv("CAS_UC_SYSTEM_ID", "env-sys")
	t.Setenv("CAS_UC_CERT", "env-cert")
	got := cfg.ResolveUserCenter()
	require.Equal(t, "https://from-env/", got.URL)
	require.Equal(t, "env-sys", got.SystemID)
	require.Equal(t, "env-cert", got.Cert)
	require.True(t, got.Configured())
}

func TestResolveUserCenterPrefersShortPrefix(t *testing.T) {
	cfg := &CASConfig{}
	t.Setenv("CAS_USER_CENTER_URL", "https://alias/")
	t.Setenv("CAS_USER_CENTER_SYSTEM_ID", "alias-sys")
	t.Setenv("CAS_USER_CENTER_CERT", "alias-cert")
	t.Setenv("CAS_UC_URL", "https://short/")
	t.Setenv("CAS_UC_SYSTEM_ID", "short-sys")
	t.Setenv("CAS_UC_CERT", "short-cert")
	got := cfg.ResolveUserCenter()
	require.Equal(t, "https://short/", got.URL)
	require.Equal(t, "short-sys", got.SystemID)
	require.Equal(t, "short-cert", got.Cert)
}

func TestResolveUserCenterLongAliasFallback(t *testing.T) {
	cfg := &CASConfig{}
	t.Setenv("CAS_UC_URL", "")
	t.Setenv("CAS_UC_SYSTEM_ID", "")
	t.Setenv("CAS_UC_CERT", "")
	t.Setenv("CAS_USER_CENTER_URL", "https://uc/")
	t.Setenv("CAS_USER_CENTER_SYSTEM_ID", "sid")
	t.Setenv("CAS_USER_CENTER_CERT", "alias-cert")
	got := cfg.ResolveUserCenter()
	require.Equal(t, "https://uc/", got.URL)
	require.Equal(t, "sid", got.SystemID)
	require.Equal(t, "alias-cert", got.Cert)
	require.True(t, got.Configured())
}

func TestUserCenterConfiguredRequiresAll(t *testing.T) {
	require.False(t, (CASUserCenterConfig{URL: "u", SystemID: "s"}).Configured())
	require.True(t, (CASUserCenterConfig{URL: "u", SystemID: "s", Cert: "c"}).Configured())
}
