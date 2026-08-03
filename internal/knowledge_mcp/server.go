package knowledge_mcp

import (
	"net/http"

	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

const (
	serverName    = "weknora-knowledge-retrieve"
	serverVersion = "1.0.0"
)

// Dependencies for the retrieve MCP endpoint.
type Dependencies struct {
	KBService   interfaces.KnowledgeBaseService
	WikiService interfaces.WikiPageService
}

// NewHTTPHandler builds a Streamable HTTP MCP handler (stateless) with the
// curated retrieve tool surface.
func NewHTTPHandler(deps Dependencies) http.Handler {
	mcpSrv := mcpserver.NewMCPServer(serverName, serverVersion)
	registerTools(mcpSrv, toolServices{
		scope: newScopeResolver(deps.KBService),
		kb:    deps.KBService,
		wiki:  deps.WikiService,
	})
	return mcpserver.NewStreamableHTTPServer(
		mcpSrv,
		mcpserver.WithStateLess(true),
	)
}

// GinHandler adapts the MCP HTTP handler for Gin while preserving the
// request context populated by Auth / API-key middleware.
func GinHandler(deps Dependencies) gin.HandlerFunc {
	h := NewHTTPHandler(deps)
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request.WithContext(c.Request.Context()))
	}
}
