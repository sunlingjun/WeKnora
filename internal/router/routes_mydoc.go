package router

import (
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/gin-gonic/gin"
)

// RegisterMyDocRoutes wires personal document hub APIs under /my-docs.
// Auth is already applied on the v1 group. No tenant-member RBAC gate: MyDoc
// is account-global and keyed by owner_user_id from the signed-in user.
func RegisterMyDocRoutes(r *gin.RouterGroup, h *handler.MyDocHandler) {
	if h == nil {
		return
	}
	m := r.Group("/my-docs")
	{
		m.GET("/tree", h.GetTree)
		m.POST("/folders", h.CreateFolder)
		m.PATCH("/nodes/:id", h.RenameNode)
		m.POST("/nodes/:id/move", h.MoveNode)
		m.POST("/nodes/batch-move", h.BatchMove)
		m.POST("/nodes/:id/archive", h.ArchiveNode)
		m.POST("/files", h.UploadOrWriteFile)
		m.GET("/files/:id", h.GetFileMeta)
		m.GET("/files/:id/content", h.GetFileContent)
		m.PUT("/files/:id/content", h.PutFileContent)
		m.POST("/files/:id/shares", h.CreateShare)
		m.GET("/files/:id/shares", h.ListShares)
		m.DELETE("/shares/:id", h.DeleteShare)
		m.POST("/files/:id/tickets", h.CreateTicket)
		m.POST("/tickets/:id/renew", h.RenewTicket)
		m.POST("/qa-snapshots", h.CreateQASnapshot)
		m.POST("/files/:id/mounts", h.MountNotImplemented)
	}
}
