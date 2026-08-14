package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

type nxinCASAuthCacheEntry struct {
	UserID   string `json:"user_id"`
	TenantID uint64 `json:"tenant_id"`
}

func tryNXINCASAuth(
	c *gin.Context,
	tenantService interfaces.TenantService,
	userService interfaces.UserService,
	memberService interfaces.TenantMemberService,
	casAuthService interfaces.CASAuthService,
	redisClient redis.UniversalClient,
	cfg *config.Config,
) bool {
	if casAuthService == nil || redisClient == nil || cfg == nil || cfg.Auth == nil || cfg.Auth.NXINCASAuth == nil || !cfg.Auth.NXINCASAuth.Enabled {
		return false
	}
	if !isNXINCASAuthPathAllowed(c.Request.URL.Path, cfg.Auth.NXINCASAuth.AllowedPathGlobs) {
		return false
	}
	if cfg.Auth.NXINCASAuth.RequireHTTPS && !isRequestHTTPS(c) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: HTTPS required for NXIN CAS auth"})
		c.Abort()
		return true
	}
	if cfg.CAS == nil {
		return false
	}
	casEnv := cfg.CAS.GetCurrentConfig()
	if casEnv == nil {
		return false
	}
	casSid, _ := c.Cookie(casEnv.CookieSID)
	casUid, _ := c.Cookie(casEnv.CookieUID)
	if casSid == "" || casUid == "" {
		return false
	}

	cacheTTL := time.Duration(cfg.Auth.NXINCASAuth.CacheTTLSeconds) * time.Second
	cacheKey := buildNXINCASAuthCacheKey(casEnv.APIHost, casSid, casUid)

	if entry, ok := getNXINCASAuthCache(c.Request.Context(), redisClient, cacheKey); ok {
		user, err := userService.GetUserByID(c.Request.Context(), entry.UserID)
		if err == nil && user != nil && user.IsActive {
			if !authenticateJWTUser(c, tenantService, memberService, cfg, user, entry.TenantID) {
				return true
			}
			c.Next()
			return true
		}
	}

	referer := buildCASReferer(c, casEnv.LoginHost)
	casUserInfo, err := casAuthService.ValidateCASSession(c.Request.Context(), casSid, casUid, referer)
	if err != nil {
		log.Printf("NXIN CAS auth validate failed: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: invalid CAS session"})
		c.Abort()
		return true
	}
	user, err := casAuthService.AutoBindUser(c.Request.Context(), casUserInfo)
	if err != nil {
		log.Printf("NXIN CAS auth bind user failed: %v", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Auth provider unavailable"})
		c.Abort()
		return true
	}
	tenant, err := casAuthService.AutoBindTenant(c.Request.Context(), casUserInfo, user)
	if err != nil {
		log.Printf("NXIN CAS auth bind tenant failed: %v", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Auth provider unavailable"})
		c.Abort()
		return true
	}

	if !authenticateJWTUser(c, tenantService, memberService, cfg, user, tenant.ID) {
		return true
	}
	_ = setNXINCASAuthCache(c.Request.Context(), redisClient, cacheKey, nxinCASAuthCacheEntry{
		UserID:   user.ID,
		TenantID: tenant.ID,
	}, cacheTTL)
	c.Next()
	return true
}

func isNXINCASAuthPathAllowed(path string, patterns []string) bool {
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		if strings.HasSuffix(pattern, "*") {
			prefix := strings.TrimSuffix(pattern, "*")
			if strings.HasPrefix(path, prefix) {
				return true
			}
			continue
		}
		if path == pattern {
			return true
		}
	}
	return false
}

func isRequestHTTPS(c *gin.Context) bool {
	if c.Request.TLS != nil {
		return true
	}
	return strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https")
}

func buildCASReferer(c *gin.Context, loginHost string) string {
	referer := c.GetHeader("Referer")
	if referer != "" {
		return referer
	}
	origin := c.GetHeader("Origin")
	if origin != "" {
		return origin + "/"
	}
	return fmt.Sprintf("https://%s/", loginHost)
}

func buildNXINCASAuthCacheKey(apiHost, casSid, casUid string) string {
	raw := apiHost + "|" + casSid + "|" + casUid
	sum := sha256.Sum256([]byte(raw))
	return "auth:nxin_cas_auth:" + hex.EncodeToString(sum[:])
}

func getNXINCASAuthCache(ctx context.Context, redisClient redis.UniversalClient, key string) (*nxinCASAuthCacheEntry, bool) {
	raw, err := redisClient.Get(ctx, key).Result()
	if err != nil || raw == "" {
		return nil, false
	}
	var entry nxinCASAuthCacheEntry
	if err := json.Unmarshal([]byte(raw), &entry); err != nil {
		return nil, false
	}
	if entry.UserID == "" || entry.TenantID == 0 {
		return nil, false
	}
	return &entry, true
}

func setNXINCASAuthCache(ctx context.Context, redisClient redis.UniversalClient, key string, entry nxinCASAuthCacheEntry, ttl time.Duration) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	return redisClient.Set(ctx, key, data, ttl).Err()
}