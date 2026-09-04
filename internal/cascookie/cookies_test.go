package cascookie

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestReadTicketAndSID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		CAS: &config.CASConfig{
			Environment: "test",
			Test: &config.CASEnvConfig{
				CookieSID: "_cas_t_sid",
				CookieUID: "_cas_t_uid",
			},
		},
	}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.AddCookie(&http.Cookie{Name: TicketCookieName, Value: " tk-1 "})
	c.Request.AddCookie(&http.Cookie{Name: "_cas_t_sid", Value: "sid-1"})
	c.Request.AddCookie(&http.Cookie{Name: "_cas_t_uid", Value: "uid-1"})

	ticket, sid, uid := Read(c, cfg)
	require.Equal(t, "tk-1", ticket)
	require.Equal(t, "sid-1", sid)
	require.Equal(t, "uid-1", uid)
}

func TestReadNilSafe(t *testing.T) {
	ticket, sid, uid := Read(nil, nil)
	require.Equal(t, "", ticket)
	require.Equal(t, "", sid)
	require.Equal(t, "", uid)
}
