package handler

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/Tencent/WeKnora/internal/application/service/mydoc"
	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
)

const myDocTicketHeader = "X-WeKnora-MyDoc-Ticket"

// MyDocHandler serves /api/v1/my-docs.
type MyDocHandler struct {
	svc *mydoc.Service
}

// NewMyDocHandler constructs the handler.
func NewMyDocHandler(svc *mydoc.Service) *MyDocHandler {
	return &MyDocHandler{svc: svc}
}

func (h *MyDocHandler) requireUser(c *gin.Context) (string, bool) {
	uid, err := mydoc.RequireHumanUser(c.Request.Context())
	if err != nil {
		if errors.Is(err, mydoc.ErrMachineCaller) {
			c.Error(apperrors.NewForbiddenError("MyDoc requires a signed-in user, not an API key or MCP endpoint"))
		} else {
			c.Error(apperrors.NewUnauthorizedError("unauthorized"))
		}
		return "", false
	}
	return uid, true
}

func (h *MyDocHandler) respondErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, mydoc.ErrNotFound):
		c.Error(apperrors.NewNotFoundError("not found"))
	case errors.Is(err, mydoc.ErrForbidden), errors.Is(err, mydoc.ErrNotOwner), errors.Is(err, mydoc.ErrMachineCaller):
		c.Error(apperrors.NewForbiddenError(err.Error()))
	case errors.Is(err, mydoc.ErrDuplicateName), errors.Is(err, mydoc.ErrCycle),
		errors.Is(err, mydoc.ErrInvalidInput), errors.Is(err, mydoc.ErrNotText):
		c.Error(apperrors.NewValidationError(err.Error()))
	default:
		c.Error(err)
	}
}

func ticketFromRequest(c *gin.Context) string {
	t := strings.TrimSpace(c.GetHeader(myDocTicketHeader))
	if t == "" {
		t = strings.TrimSpace(c.Query("ticket"))
	}
	return t
}

// GetTree godoc
// @Summary List my documents tree
// @Tags MyDoc
// @Produce json
// @Param view query string false "owned|shared"
// @Success 200 {object} map[string]interface{}
// @Router /my-docs/tree [get]
func (h *MyDocHandler) GetTree(c *gin.Context) {
	uid, ok := h.requireUser(c)
	if !ok {
		return
	}
	nodes, err := h.svc.GetTree(c.Request.Context(), uid, c.Query("view"))
	if err != nil {
		h.respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": nodes})
}

type createFolderReq struct {
	ParentID string `json:"parent_id"`
	Name     string `json:"name" binding:"required"`
}

// CreateFolder creates a directory.
func (h *MyDocHandler) CreateFolder(c *gin.Context) {
	uid, ok := h.requireUser(c)
	if !ok {
		return
	}
	var req createFolderReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(apperrors.NewValidationError(err.Error()))
		return
	}
	node, err := h.svc.Mkdir(c.Request.Context(), uid, req.ParentID, req.Name)
	if err != nil {
		h.respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": node})
}

type renameReq struct {
	Name string `json:"name" binding:"required"`
}

// RenameNode renames a node.
func (h *MyDocHandler) RenameNode(c *gin.Context) {
	uid, ok := h.requireUser(c)
	if !ok {
		return
	}
	var req renameReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(apperrors.NewValidationError(err.Error()))
		return
	}
	node, err := h.svc.Rename(c.Request.Context(), uid, c.Param("id"), req.Name)
	if err != nil {
		h.respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": node})
}

type moveReq struct {
	ParentID string `json:"parent_id"`
}

// MoveNode moves one node.
func (h *MyDocHandler) MoveNode(c *gin.Context) {
	uid, ok := h.requireUser(c)
	if !ok {
		return
	}
	var req moveReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(apperrors.NewValidationError(err.Error()))
		return
	}
	node, err := h.svc.Move(c.Request.Context(), uid, c.Param("id"), req.ParentID)
	if err != nil {
		h.respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": node})
}

type batchMoveReq struct {
	NodeIDs  []string `json:"node_ids" binding:"required"`
	ParentID string   `json:"parent_id"`
}

// BatchMove moves multiple nodes.
func (h *MyDocHandler) BatchMove(c *gin.Context) {
	uid, ok := h.requireUser(c)
	if !ok {
		return
	}
	var req batchMoveReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(apperrors.NewValidationError(err.Error()))
		return
	}
	if err := h.svc.BatchMove(c.Request.Context(), uid, req.NodeIDs, req.ParentID); err != nil {
		h.respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ArchiveNode archives a node subtree.
func (h *MyDocHandler) ArchiveNode(c *gin.Context) {
	uid, ok := h.requireUser(c)
	if !ok {
		return
	}
	if err := h.svc.Archive(c.Request.Context(), uid, c.Param("id")); err != nil {
		h.respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// UploadOrWriteFile handles multipart upload or JSON text create.
func (h *MyDocHandler) UploadOrWriteFile(c *gin.Context) {
	uid, ok := h.requireUser(c)
	if !ok {
		return
	}
	ct := c.ContentType()
	if strings.HasPrefix(ct, "multipart/") {
		fileHeader, err := c.FormFile("file")
		if err != nil {
			c.Error(apperrors.NewValidationError("file is required"))
			return
		}
		f, err := fileHeader.Open()
		if err != nil {
			c.Error(err)
			return
		}
		defer f.Close()
		data, err := io.ReadAll(f)
		if err != nil {
			c.Error(err)
			return
		}
		item, err := h.svc.UploadFile(
			c.Request.Context(),
			uid,
			c.PostForm("parent_id"),
			c.PostForm("kind"),
			fileHeader.Filename,
			fileHeader.Header.Get("Content-Type"),
			data,
			c.PostForm("source_agent"),
			c.PostForm("source_session"),
		)
		if err != nil {
			h.respondErr(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "data": item})
		return
	}
	var req struct {
		ParentID      string `json:"parent_id"`
		Kind          string `json:"kind"`
		Filename      string `json:"filename" binding:"required"`
		Mime          string `json:"mime"`
		Content       string `json:"content"`
		SourceAgent   string `json:"source_agent"`
		SourceSession string `json:"source_session"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(apperrors.NewValidationError(err.Error()))
		return
	}
	item, err := h.svc.UploadFile(
		c.Request.Context(), uid, req.ParentID, req.Kind, req.Filename, req.Mime,
		[]byte(req.Content), req.SourceAgent, req.SourceSession,
	)
	if err != nil {
		h.respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": item})
}

// GetFileMeta returns file metadata.
func (h *MyDocHandler) GetFileMeta(c *gin.Context) {
	uid, _ := types.UserIDFromContext(c.Request.Context())
	item, err := h.svc.GetFileMeta(c.Request.Context(), uid, c.Param("id"), ticketFromRequest(c))
	if err != nil {
		h.respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": item})
}

// GetFileContent streams content.
func (h *MyDocHandler) GetFileContent(c *gin.Context) {
	uid, _ := types.UserIDFromContext(c.Request.Context())
	rc, node, file, err := h.svc.ReadContent(c.Request.Context(), uid, c.Param("id"), ticketFromRequest(c))
	if err != nil {
		h.respondErr(c, err)
		return
	}
	defer rc.Close()
	if file.Mime != "" {
		c.Header("Content-Type", file.Mime)
	}
	c.Header("Content-Disposition", `inline; filename="`+strings.ReplaceAll(node.Name, `"`, "")+`"`)
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, rc)
}

// PutFileContent updates text content.
func (h *MyDocHandler) PutFileContent(c *gin.Context) {
	uid, ok := h.requireUser(c)
	if !ok {
		return
	}
	var req struct {
		Content     string `json:"content"`
		SourceAgent string `json:"source_agent"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(apperrors.NewValidationError(err.Error()))
		return
	}
	item, err := h.svc.WriteText(c.Request.Context(), uid, c.Param("id"), req.Content, req.SourceAgent)
	if err != nil {
		h.respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": item})
}

type shareReq struct {
	GranteeUserID string `json:"grantee_user_id" binding:"required"`
}

// CreateShare shares a file with another user.
func (h *MyDocHandler) CreateShare(c *gin.Context) {
	uid, ok := h.requireUser(c)
	if !ok {
		return
	}
	var req shareReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(apperrors.NewValidationError(err.Error()))
		return
	}
	share, err := h.svc.CreateShare(c.Request.Context(), uid, c.Param("id"), req.GranteeUserID)
	if err != nil {
		h.respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": share})
}

// DeleteShare removes a share.
func (h *MyDocHandler) DeleteShare(c *gin.Context) {
	uid, ok := h.requireUser(c)
	if !ok {
		return
	}
	if err := h.svc.DeleteShare(c.Request.Context(), uid, c.Param("id")); err != nil {
		h.respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ListShares lists shares for a file.
func (h *MyDocHandler) ListShares(c *gin.Context) {
	uid, ok := h.requireUser(c)
	if !ok {
		return
	}
	shares, err := h.svc.ListShares(c.Request.Context(), uid, c.Param("id"))
	if err != nil {
		h.respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": shares})
}

type ticketReq struct {
	Purpose string `json:"purpose"`
}

// CreateTicket mints a short ticket.
func (h *MyDocHandler) CreateTicket(c *gin.Context) {
	uid, ok := h.requireUser(c)
	if !ok {
		return
	}
	var req ticketReq
	_ = c.ShouldBindJSON(&req)
	token, row, err := h.svc.CreateTicket(c.Request.Context(), uid, c.Param("id"), req.Purpose)
	if err != nil {
		h.respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"id":         row.ID,
			"doc_id":     row.DocID,
			"purpose":    row.Purpose,
			"expires_at": row.ExpiresAt,
			"ticket":     token,
		},
	})
}

type renewTicketReq struct {
	Ticket string `json:"ticket" binding:"required"`
}

// RenewTicket renews a ticket within grace.
func (h *MyDocHandler) RenewTicket(c *gin.Context) {
	var req renewTicketReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(apperrors.NewValidationError(err.Error()))
		return
	}
	token, row, err := h.svc.RenewTicket(c.Request.Context(), c.Param("id"), req.Ticket)
	if err != nil {
		h.respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"id":         row.ID,
			"doc_id":     row.DocID,
			"purpose":    row.Purpose,
			"expires_at": row.ExpiresAt,
			"ticket":     token,
		},
	})
}

type qaSnapshotReq struct {
	SessionID string `json:"session_id" binding:"required"`
	ParentID  string `json:"parent_id"`
	Title     string `json:"title"`
}

// CreateQASnapshot exports a session as a markdown snapshot.
func (h *MyDocHandler) CreateQASnapshot(c *gin.Context) {
	uid, ok := h.requireUser(c)
	if !ok {
		return
	}
	var req qaSnapshotReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(apperrors.NewValidationError(err.Error()))
		return
	}
	item, err := h.svc.CreateQASnapshot(c.Request.Context(), uid, req.ParentID, req.SessionID, req.Title)
	if err != nil {
		h.respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": item})
}

// MountNotImplemented returns 501 for P1 mounts.
func (h *MyDocHandler) MountNotImplemented(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"success": false, "error": "mount is P1"})
}
