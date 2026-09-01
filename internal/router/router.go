package router

import (
	"context"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.uber.org/dig"

	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/Tencent/WeKnora/internal/handler/session"
	"github.com/Tencent/WeKnora/internal/knowledge_mcp"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/tracing/langfuse"
	"github.com/Tencent/WeKnora/internal/types/interfaces"

	_ "github.com/Tencent/WeKnora/docs" // swagger docs
)

// RouterParams 路由参数
type RouterParams struct {
	dig.In

	Config                       *config.Config
	FileService                  interfaces.FileService
	UserService                  interfaces.UserService
	KBService                    interfaces.KnowledgeBaseService
	KnowledgeService             interfaces.KnowledgeService
	ChunkService                 interfaces.ChunkService
	SessionService               interfaces.SessionService
	MessageService               interfaces.MessageService
	ModelService                 interfaces.ModelService
	EvaluationService            interfaces.EvaluationService
	KBShareService               interfaces.KBShareService
	SharedKBService              interfaces.SharedKnowledgeBaseService
	AgentShareService            interfaces.AgentShareService
	KBHandler                    *handler.KnowledgeBaseHandler
	KnowledgeCatalogHandler      *handler.KnowledgeCatalogHandler
	KnowledgeHandler             *handler.KnowledgeHandler
	TenantHandler                *handler.TenantHandler
	TenantService                interfaces.TenantService
	TenantAPIKeyService          interfaces.TenantAPIKeyService
	TenantMemberService          interfaces.TenantMemberService
	TenantMemberHandler          *handler.TenantMemberHandler
	TenantInvitationHandler      *handler.TenantInvitationHandler
	AuditLogHandler              *handler.AuditLogHandler
	AuditLogService              interfaces.AuditLogService
	ChunkHandler                 *handler.ChunkHandler
	SessionHandler               *session.Handler
	MessageHandler               *handler.MessageHandler
	MessageSuggestionHandler     *handler.MessageSuggestionHandler
	ModelHandler                 *handler.ModelHandler
	ModelCredentialsHandler      *handler.ModelCredentialsHandler
	EvaluationHandler            *handler.EvaluationHandler
	AuthHandler                  *handler.AuthHandler
	InitializationHandler        *handler.InitializationHandler
	SystemHandler                *handler.SystemHandler
	MCPServiceHandler            *handler.MCPServiceHandler
	MCPCredentialsHandler        *handler.MCPCredentialsHandler
	MCPOAuthHandler              *handler.MCPOAuthHandler
	WebSearchHandler             *handler.WebSearchHandler
	WebSearchProviderHandler     *handler.WebSearchProviderHandler
	WebSearchCredentialsHandler  *handler.WebSearchProviderCredentialsHandler
	VectorStoreHandler           *handler.VectorStoreHandler
	StorageBackendHandler        *handler.StorageBackendHandler
	StorageBackendResolver       interfaces.StorageBackendResolver
	ResourceCatalog              interfaces.ResourceCatalog
	FAQHandler                   *handler.FAQHandler
	TagHandler                   *handler.TagHandler
	CustomAgentHandler           *handler.CustomAgentHandler
	UserFavoriteHandler          *handler.UserResourceFavoriteHandler
	SkillHandler                 *handler.SkillHandler
	OrganizationHandler          *handler.OrganizationHandler
	IMHandler                    *handler.IMHandler
	EmbedChannelHandler          *handler.EmbedChannelHandler
	EmbedChannelService          interfaces.EmbedChannelService
	RedisClient                  redis.UniversalClient
	DataSourceHandler            *handler.DataSourceHandler
	DataSourceCredentialsHandler *handler.DataSourceCredentialsHandler
	CASAuthHandler               *handler.CASAuthHandler
	CASAuthService               interfaces.CASAuthService
	OpenRetrieveHandler          *handler.OpenRetrieveHandler
	WeKnoraCloudHandler          *handler.WeKnoraCloudHandler
	WikiPageHandler              *handler.WikiPageHandler
	WikiPageService              interfaces.WikiPageService
}

// defaultTrustedPrivateProxies 当 behind_proxy 开启但未配置 trusted_proxies 时的保守默认值（私网 + 本机）。
func defaultTrustedPrivateProxies() []string {
	return []string{
		"127.0.0.1",
		"::1",
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
	}
}

func applyGinTrustedProxies(r *gin.Engine, cfg *config.Config) {
	if r == nil || cfg == nil || cfg.Server == nil || !cfg.Server.BehindProxy {
		return
	}
	proxies := cfg.Server.TrustedProxies
	if len(proxies) == 0 {
		proxies = defaultTrustedPrivateProxies()
		logger.Infof(context.Background(), "server.behind_proxy=true with empty trusted_proxies, using default private ranges")
	}
	if err := r.SetTrustedProxies(proxies); err != nil {
		logger.Warnf(context.Background(), "SetTrustedProxies failed: %v", err)
	}
}

// NewRouter 创建新的路由
func NewRouter(params RouterParams) *gin.Engine {
	r := gin.New()
	r.ContextWithFallback = true
	if params.Config != nil && params.Config.Server != nil && params.Config.Server.BehindProxy {
		applyGinTrustedProxies(r, params.Config)
	} else if err := r.SetTrustedProxies(trustedProxies()); err != nil {
		logger.Errorf(context.Background(), "[Router] failed to set trusted proxies: %v", err)
	}

	// CORS：AllowOriginFunc 回显 Origin；Hostname() 放行 *.nxin.com（含端口）
	allowedOrigins := []string{
		"https://zsk.t.nxin.com",
		"https://zsk.t.nxin.com:443",
		"https://zsk.t.nxin.com:80",
		"https://zsk.nxin.com",
		"https://zsk.nxin.com:443",
		"https://zsk.nxin.com:80",
		"https://localhost",
		"https://localhost:443",
		"https://localhost:80",
		"https://localhost:8081",
		"http://localhost",
		"http://localhost:8081",
	}
	if params.Config != nil && params.Config.Server != nil && len(params.Config.Server.CORSAllowedOrigins) > 0 {
		allowedOrigins = params.Config.Server.CORSAllowedOrigins
	}
	r.Use(cors.New(cors.Config{
		AllowOriginFunc: func(origin string) bool {
			for _, allowed := range allowedOrigins {
				if origin == allowed {
					return true
				}
			}
			if strings.HasPrefix(origin, "http://localhost:") || strings.HasPrefix(origin, "https://localhost:") {
				return true
			}
			if parsed, err := url.Parse(origin); err == nil {
				host := parsed.Hostname()
				scheme := strings.ToLower(parsed.Scheme)
				if (scheme == "http" || scheme == "https") &&
					(host == "nxin.com" || strings.HasSuffix(host, ".nxin.com")) {
					return true
				}
			}
			return false
		},
		AllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders: []string{
			"Origin", "Content-Type", "Accept", "Authorization",
			"X-API-Key", "X-Open-Retrieve-Api-Key", "X-Request-ID", "X-Tenant-ID", "X-Embed-Session",
			"X-External-User-ID", "X-External-User-Token",
			"Cache-Control", "Pragma", "X-Requested-With", "DNT", "If-Modified-Since",
			"Keep-Alive", "User-Agent", "pd", "systemid",
		},
		ExposeHeaders:    []string{"Content-Length", "Access-Control-Allow-Origin"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// 基础中间件（不需要认证）
	r.Use(middleware.RequestID())
	r.Use(middleware.Language())
	r.Use(middleware.Logger())
	r.Use(middleware.Recovery())
	r.Use(middleware.ErrorHandler())

	// 健康检查（不需要认证）
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	if gin.Mode() != gin.ReleaseMode {
		r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler,
			ginSwagger.DefaultModelsExpandDepth(-1),
			ginSwagger.DocExpansion("list"),
			ginSwagger.DeepLinking(true),
			ginSwagger.PersistAuthorization(true),
		))
	}

	if params.EmbedChannelService != nil {
		r.Use(embedFrameAncestorsMiddleware(params.EmbedChannelService))
	}

	if handler.Edition == "lite" {
		serveFrontendStatic(r)
	}

	RegisterIMRoutes(r, params.IMHandler)

	if params.CASAuthHandler != nil {
		RegisterCASRoutes(r, params.CASAuthHandler)
	}

	RegisterEmbedPublicRoutes(
		r,
		params.EmbedChannelHandler,
		params.EmbedChannelService,
		params.TenantService,
		singleRedisClient(params.RedisClient),
		params.FileService,
		params.StorageBackendResolver,
		params.ResourceCatalog,
	)

	serveResourceGrants(r, params.ResourceCatalog, params.TenantService, params.FileService, params.StorageBackendResolver)

	r.Use(middleware.Auth(
		params.TenantService,
		params.UserService,
		params.TenantMemberService,
		params.CASAuthService,
		params.TenantAPIKeyService,
		params.RedisClient,
		params.Config,
	))

	serveFilesWithResources(r, params.FileService, params.StorageBackendResolver, params.ResourceCatalog)
	servePresignedFiles(r, params.TenantService, params.StorageBackendResolver)
	servePresignedPreview(r, params.Config, params.StorageBackendResolver)

	r.Use(langfuse.GinMiddleware())
	r.Use(middleware.AuditServiceProvider(params.AuditLogService))

	v1 := r.Group("/api/v1")
	{
		rbacGuards := newRBACGuards(
			params.Config,
			params.KBHandler,
			params.CustomAgentHandler,
			params.KnowledgeHandler,
			params.ChunkHandler,
			params.WikiPageHandler,
			params.KBService,
			params.KnowledgeService,
			params.ChunkService,
			params.KBShareService,
			params.SharedKBService,
			params.AgentShareService,
		)

		v1.Use(rbacGuards.apiKeyAuthorizer.Middleware())

		RegisterAuthRoutes(v1, params.AuthHandler, rbacGuards)
		RegisterTenantRoutes(v1, params.TenantHandler, params.TenantMemberHandler, params.TenantInvitationHandler, params.AuditLogHandler, rbacGuards)
		RegisterMyInvitationRoutes(v1, params.TenantInvitationHandler)
		RegisterKnowledgeBaseRoutes(v1, params.KBHandler, rbacGuards)
		RegisterKnowledgeCatalogRoutes(v1, params.KnowledgeCatalogHandler, rbacGuards)
		RegisterKnowledgeBaseActivityRoutes(v1, params.AuditLogHandler, rbacGuards)
		serveKBScopedFiles(
			v1,
			rbacGuards,
			params.TenantService,
			params.FileService,
			params.StorageBackendResolver,
			params.ResourceCatalog,
		)
		RegisterKnowledgeTagRoutes(v1, params.TagHandler, rbacGuards)
		RegisterKnowledgeRoutes(v1, params.KnowledgeHandler, rbacGuards)
		RegisterFAQRoutes(v1, params.FAQHandler, rbacGuards)
		RegisterChunkRoutes(v1, params.ChunkHandler, rbacGuards)
		RegisterSessionRoutes(v1, params.SessionHandler, params.MessageSuggestionHandler, rbacGuards)
		RegisterChatRoutes(v1, params.SessionHandler, rbacGuards)
		RegisterMessageRoutes(v1, params.MessageHandler, rbacGuards)
		RegisterModelRoutes(v1, params.ModelHandler, params.ModelCredentialsHandler, rbacGuards)
		RegisterEvaluationRoutes(v1, params.EvaluationHandler, rbacGuards)
		RegisterInitializationRoutes(v1, params.InitializationHandler, rbacGuards)
		RegisterSystemRoutes(v1, params.SystemHandler, rbacGuards)
		RegisterSystemAdminRoutes(v1, params.SystemHandler, params.AuditLogHandler, rbacGuards)
		RegisterMCPServiceRoutes(v1, params.MCPServiceHandler, params.MCPCredentialsHandler, params.MCPOAuthHandler, rbacGuards)
		RegisterWebSearchRoutes(v1, params.WebSearchHandler, rbacGuards)
		RegisterWebSearchProviderRoutes(v1, params.WebSearchProviderHandler, params.WebSearchCredentialsHandler, rbacGuards)
		RegisterVectorStoreRoutes(v1, params.VectorStoreHandler, rbacGuards)
		RegisterStorageBackendRoutes(v1, params.StorageBackendHandler, rbacGuards)
		RegisterCustomAgentRoutes(v1, params.CustomAgentHandler, rbacGuards)
		RegisterUserFavoriteRoutes(v1, params.UserFavoriteHandler, rbacGuards)
		RegisterSkillRoutes(v1, params.SkillHandler, rbacGuards)
		RegisterOrganizationRoutes(v1, params.OrganizationHandler, rbacGuards)
		RegisterIMChannelRoutes(v1, params.IMHandler, rbacGuards)
		RegisterEmbedChannelRoutes(v1, params.EmbedChannelHandler, rbacGuards)
		RegisterDataSourceRoutes(v1, params.DataSourceHandler, params.DataSourceCredentialsHandler, rbacGuards)
		RegisterWeKnoraCloudRoutes(v1, params.WeKnoraCloudHandler, rbacGuards)
		RegisterWikiPageRoutes(v1, params.WikiPageHandler, rbacGuards)
		if params.KBService != nil && params.WikiPageService != nil {
			RegisterKnowledgeMCPRoutes(v1, knowledge_mcp.GinHandler(knowledge_mcp.Dependencies{
				KBService:   params.KBService,
				WikiService: params.WikiPageService,
			}), rbacGuards)
		}
		RegisterChunkerDebugRoutes(v1, rbacGuards)

		if params.OpenRetrieveHandler != nil {
			openG := v1.Group("/open")
			openG.Use(middleware.OpenRetrieveApiKey(params.Config))
			openG.POST("/knowledge/retrieve", params.OpenRetrieveHandler.Retrieve)
		}

		rbacGuards.assertAPIKeyPoliciesMatchRoutes(r)
	}

	return r
}

// trustedProxies returns the proxy CIDRs/IPs whose X-Forwarded-For headers
// gin should trust when resolving the client IP. Defaults to loopback and
// private ranges (covers the bundled nginx in a container network); override
// with WEKNORA_TRUSTED_PROXIES (comma-separated). An explicit empty value
// disables proxy trust entirely so ClientIP() returns the direct peer.
func trustedProxies() []string {
	raw, ok := os.LookupEnv("WEKNORA_TRUSTED_PROXIES")
	if !ok {
		return []string{
			"127.0.0.0/8",
			"::1/128",
			"10.0.0.0/8",
			"172.16.0.0/12",
			"192.168.0.0/16",
			"fc00::/7",
		}
	}
	proxies := make([]string, 0)
	for _, p := range strings.Split(raw, ",") {
		if p = strings.TrimSpace(p); p != "" {
			proxies = append(proxies, p)
		}
	}
	return proxies
}

// singleRedisClient unwraps a standalone *redis.Client from UniversalClient for
// components (embed rate limiter) that require the concrete type.
func singleRedisClient(rdb redis.UniversalClient) *redis.Client {
	if c, ok := rdb.(*redis.Client); ok {
		return c
	}
	return nil
}
