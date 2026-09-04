package cascookie

import (
	"strings"

	"github.com/Tencent/WeKnora/internal/config"
	"github.com/gin-gonic/gin"
)

// TicketCookieName is the APP/ZNT cookie (gateway APP_TOKEY_COOKIE_NAME).
const TicketCookieName = "ticketCookie"

// Read returns ticketCookie, environment CAS sid, and CAS uid from the request.
// uid is optional for auth; callers use it only for JWT conflict detection.
func Read(c *gin.Context, cfg *config.Config) (ticketCookie, casSid, casUID string) {
	if c == nil {
		return "", "", ""
	}
	if v, err := c.Cookie(TicketCookieName); err == nil {
		ticketCookie = strings.TrimSpace(v)
	}
	if cfg == nil || cfg.CAS == nil {
		return ticketCookie, "", ""
	}
	env := cfg.CAS.GetCurrentConfig()
	if env == nil {
		return ticketCookie, "", ""
	}
	if env.CookieSID != "" {
		if v, err := c.Cookie(env.CookieSID); err == nil {
			casSid = strings.TrimSpace(v)
		}
	}
	if env.CookieUID != "" {
		if v, err := c.Cookie(env.CookieUID); err == nil {
			casUID = strings.TrimSpace(v)
		}
	}
	return ticketCookie, casSid, casUID
}
