package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/Tencent/WeKnora/internal/config"
	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

const headerOpenRetrieveAPIKey = "X-Open-Retrieve-Api-Key"

// OpenRetrieveApiKey enforces open_retrieve.enabled and X-Open-Retrieve-Api-Key before the handler runs.
// The route must be registered under noAuthAPI so global user Auth does not run.
func OpenRetrieveApiKey(cfg *config.Config) gin.HandlerFunc {
	var lim *rate.Limiter
	if cfg != nil && cfg.OpenRetrieve != nil && cfg.OpenRetrieve.RateLimitQPS > 0 {
		burst := int(cfg.OpenRetrieve.RateLimitQPS)
		if burst < 1 {
			burst = 1
		}
		lim = rate.NewLimiter(rate.Limit(cfg.OpenRetrieve.RateLimitQPS), burst)
	}

	return func(c *gin.Context) {
		or := cfg.OpenRetrieve
		if or == nil || !or.Enabled {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "OPEN_RETRIEVE_DISABLED",
					"message": "open retrieve is disabled",
				},
			})
			return
		}
		keys := or.EffectiveAPIKeys()
		if len(keys) == 0 {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "OPEN_RETRIEVE_DISABLED",
					"message": "open retrieve has no api_key configured",
				},
			})
			return
		}
		provided := strings.TrimSpace(c.GetHeader(headerOpenRetrieveAPIKey))
		if !openRetrieveConstantTimeKeyMatch(provided, keys) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "OPEN_RETRIEVE_UNAUTHORIZED",
					"message": "invalid or missing X-Open-Retrieve-Api-Key",
				},
			})
			return
		}
		if lim != nil && !lim.Allow() {
			c.Header("Retry-After", "1")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "OPEN_RETRIEVE_RATE_LIMITED",
					"message": "rate limit exceeded",
				},
			})
			return
		}
		c.Next()
	}
}

func openRetrieveConstantTimeKeyMatch(provided string, allowed []string) bool {
	if provided == "" {
		return false
	}
	p := []byte(provided)
	for _, a := range allowed {
		ak := []byte(a)
		if len(p) != len(ak) {
			continue
		}
		if subtle.ConstantTimeCompare(p, ak) == 1 {
			return true
		}
	}
	return false
}
