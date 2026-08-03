package router

import (
	"github.com/gin-gonic/gin"
)

// RegisterKnowledgeMCPRoutes mounts the retrieve MCP Streamable HTTP endpoint
// at /api/v1/mcp/retrieve. Requires API key capability retrieve (or JWT session).
func RegisterKnowledgeMCPRoutes(r *gin.RouterGroup, handler gin.HandlerFunc, g *rbacGuards) {
	if handler == nil || g == nil {
		return
	}
	grp := g.apiKeyGroup(r.Group("/mcp/retrieve"), apiKeyRetrieve(apiKeyFullAccess()))
	grp.GET("", handler)
	grp.POST("", handler)
	grp.DELETE("", handler)
}
