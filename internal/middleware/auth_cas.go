package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/cascookie"
	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/types"
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

	ticketCookie, casSid, _ := cascookie.Read(c, cfg)
	if ticketCookie == "" && casSid == "" {
		return false
	}

	cacheTTL := time.Duration(cfg.Auth.NXINCASAuth.CacheTTLSeconds) * time.Second
	channel, token := casAuthCacheChannelToken(ticketCookie, casSid)
	cacheKey := buildNXINCASAuthCacheKey(casEnv.APIHost, channel, token)

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

	casUserInfo, err := casAuthService.ResolveCASUserFromCookies(c.Request.Context(), ticketCookie, casSid)
	if errors.Is(err, types.ErrCASCredentialsMissing) {
		return false
	}
	if errors.Is(err, types.ErrCASTicketInvalid) {
		log.Printf("NXIN CAS auth ticket invalid: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: invalid CAS session"})
		c.Abort()
		return true
	}
	if err != nil {
		log.Printf("NXIN CAS auth resolve failed: %v", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Auth provider unavailable"})
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

func casAuthCacheChannelToken(ticketCookie, casSid string) (channel, token string) {
	if strings.TrimSpace(ticketCookie) != "" {
		return "znt", strings.TrimSpace(ticketCookie)
	}
	return "sid", strings.TrimSpace(casSid)
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

// buildNXINCASAuthCacheKey hashes apiHost|channel|token so tickets are not stored in Redis keys.
func buildNXINCASAuthCacheKey(apiHost, channel, token string) string {
	raw := apiHost + "|" + channel + "|" + token
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
