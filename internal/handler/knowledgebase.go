package handler

import (
	"encoding/json"
	stderrors "errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/errors"
	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/Tencent/WeKnora/internal/utils"
	secutils "github.com/Tencent/WeKnora/internal/utils"
	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
)

// KnowledgeBaseHandler defines the HTTP handler for knowledge base operations
type KnowledgeBaseHandler struct {
	service           interfaces.KnowledgeBaseService
	knowledgeService  interfaces.KnowledgeService
	chunkService      interfaces.ChunkService
	kbShareService    interfaces.KBShareService
	agentShareService interfaces.AgentShareService
	asynqClient       interfaces.TaskEnqueuer
	sharedKBService   interfaces.SharedKnowledgeBaseService
}

// NewKnowledgeBaseHandler creates a new knowledge base handler instance
func NewKnowledgeBaseHandler(
	service interfaces.KnowledgeBaseService,
	knowledgeService interfaces.KnowledgeService,
	chunkService interfaces.ChunkService,
	kbShareService interfaces.KBShareService,
	agentShareService interfaces.AgentShareService,
	asynqClient interfaces.TaskEnqueuer,
	sharedKBService interfaces.SharedKnowledgeBaseService,
) *KnowledgeBaseHandler {
	return &KnowledgeBaseHandler{
		service:           service,
		knowledgeService:  knowledgeService,
		chunkService:      chunkService,
		kbShareService:    kbShareService,
		agentShareService: agentShareService,
		asynqClient:       asynqClient,
		sharedKBService:   sharedKBService,
	}
}

// HybridSearch godoc
// @Summary      混合搜索
// @Description  在知识库中执行向量和关键词混合搜索
// @Tags         知识库
// @Accept       json
// @Produce      json
// @Param        id       path      string             true  "知识库ID"
// @Param        request  body      types.SearchParams true  "搜索参数"
// @Success      200      {object}  map[string]interface{}  "搜索结果"
// @Failure      400      {object}  errors.AppError         "请求参数错误"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /knowledge-bases/{id}/hybrid-search [get]
func (h *KnowledgeBaseHandler) HybridSearch(c *gin.Context) {
	ctx := c.Request.Context()

	logger.Info(ctx, "Start hybrid search")

	// Validate and check permission for knowledge base access
	_, id, effectiveTenantID, _, err := h.validateAndGetKnowledgeBase(c)
	if err != nil {
		c.Error(err)
		return
	}

	// Parse request body
	var req types.SearchParams
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error(ctx, "Failed to parse request parameters", err)
		c.Error(apperrors.NewBadRequestError("Invalid request parameters").WithDetails(err.Error()))
		return
	}

	logger.Infof(ctx, "Executing hybrid search, knowledge base ID: %s, query: %s, effectiveTenantID: %d",
		secutils.SanitizeForLog(id), secutils.SanitizeForLog(req.QueryText), effectiveTenantID)

	// Execute hybrid search with default search parameters
	// Note: For shared KBs, the service uses effectiveTenantID internally via context
	results, err := h.service.HybridSearch(ctx, id, req)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(apperrors.NewInternalServerError(err.Error()))
		return
	}

	logger.Infof(ctx, "Hybrid search completed, knowledge base ID: %s, result count: %d",
		secutils.SanitizeForLog(id), len(results))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    results,
	})
}

// CreateKnowledgeBase godoc
// @Summary      创建知识库
// @Description  创建新的知识库
// @Tags         知识库
// @Accept       json
// @Produce      json
// @Param        request  body      types.KnowledgeBase  true  "知识库信息"
// @Success      201      {object}  map[string]interface{}  "创建的知识库"
// @Failure      400      {object}  errors.AppError         "请求参数错误"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /knowledge-bases [post]
func (h *KnowledgeBaseHandler) CreateKnowledgeBase(c *gin.Context) {
	ctx := c.Request.Context()

	logger.Info(ctx, "Start creating knowledge base")

	// Parse request body
	var req types.KnowledgeBase
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error(ctx, "Failed to parse request parameters", err)
		c.Error(apperrors.NewBadRequestError("Invalid request parameters").WithDetails(err.Error()))
		return
	}
	if err := validateExtractConfig(req.ExtractConfig); err != nil {
		logger.Error(ctx, "Invalid extract configuration", err)
		c.Error(err)
		return
	}

	logger.Infof(ctx, "Creating knowledge base, name: %s", secutils.SanitizeForLog(req.Name))
	// Create knowledge base using the service
	kb, err := h.service.CreateKnowledgeBase(ctx, &req)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(apperrors.NewInternalServerError(err.Error()))
		return
	}

	logger.Infof(ctx, "Knowledge base created successfully, ID: %s, name: %s",
		secutils.SanitizeForLog(kb.ID), secutils.SanitizeForLog(kb.Name))
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    kb,
	})
}

// validateAndGetKnowledgeBase validates request parameters and retrieves the knowledge base
// Returns the knowledge base, knowledge base ID, effective tenant ID for embedding, permission level, and any errors encountered
// For owned KBs, effectiveTenantID is the caller's tenant ID
// For shared KBs, effectiveTenantID is the source tenant ID (owner's tenant)
func (h *KnowledgeBaseHandler) validateAndGetKnowledgeBase(c *gin.Context) (*types.KnowledgeBase, string, uint64, types.OrgMemberRole, error) {
	ctx := c.Request.Context()

	// Get tenant ID from context
	tenantID, exists := c.Get(types.TenantIDContextKey.String())
	if !exists {
		logger.Error(ctx, "Failed to get tenant ID")
		return nil, "", 0, "", apperrors.NewUnauthorizedError("Unauthorized")
	}

	// Get user ID from context (needed for shared KB permission check)
	userID, userExists := c.Get(types.UserIDContextKey.String())

	// Get knowledge base ID from URL parameter
	id := secutils.SanitizeForLog(c.Param("id"))
	if id == "" {
		logger.Error(ctx, "Knowledge base ID is empty")
		return nil, "", 0, "", apperrors.NewBadRequestError("Knowledge base ID cannot be empty")
	}

	// Verify tenant has permission to access this knowledge base
	kb, err := h.service.GetKnowledgeBaseByID(ctx, id)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		return nil, id, 0, "", apperrors.NewInternalServerError(err.Error())
	}

	// Check 1: Verify tenant ownership (owner has full access)
	if kb.TenantID == tenantID.(uint64) {
		return kb, id, tenantID.(uint64), types.OrgRoleAdmin, nil
	}

	// Check 2: If not owner, check organization shared access
	if userExists && h.kbShareService != nil {
		// Check if user has shared access through organization
		permission, isShared, permErr := h.kbShareService.CheckUserKBPermission(ctx, id, userID.(string))
		if permErr == nil && isShared {
			// User has shared access, get the source tenant ID for embedding queries
			sourceTenantID, srcErr := h.kbShareService.GetKBSourceTenant(ctx, id)
			if srcErr == nil {
				logger.Infof(ctx, "User %s accessing shared KB %s with permission %s, source tenant: %d",
					userID.(string), id, permission, sourceTenantID)
				return kb, id, sourceTenantID, permission, nil
			}
		}
	}

	// Check 2.5: Direct shared KB (user is member)
	if userExists && h.sharedKBService != nil {
		role, err := h.sharedKBService.GetMemberRoleByKBAndUser(ctx, id, userID.(string))
		if err == nil && role != "" {
			logger.Infof(ctx, "User %s accessing direct shared KB %s with role %s", userID.(string), id, role)
			return kb, id, kb.TenantID, types.OrgRoleViewer, nil
		}
	}

	// Check 3: Shared agent — allow if request has agent_id (and agent can access this KB) OR user has any shared agent that can access this KB (e.g. opened from "通过智能体可见" list without agent_id)
	if userExists && h.agentShareService != nil {
		currentTenantID := tenantID.(uint64)
		agentID := c.Query("agent_id")
		if agentID != "" {
			agent, err := h.agentShareService.GetSharedAgentForUser(ctx, userID.(string), currentTenantID, agentID)
			if err == nil && agent != nil {
				if kb.TenantID != agent.TenantID {
					logger.Warnf(ctx, "Shared agent tenant mismatch, KB %s tenant: %d, agent tenant: %d", id, kb.TenantID, agent.TenantID)
				} else {
					mode := agent.Config.KBSelectionMode
					if mode == "none" {
						// no-op, fall through
					} else if mode == "all" {
						logger.Infof(ctx, "User %s accessing KB %s via shared agent %s (mode=all)", userID.(string), id, agentID)
						return kb, id, kb.TenantID, types.OrgRoleViewer, nil
					} else if mode == "selected" {
						for _, allowedID := range agent.Config.KnowledgeBases {
							if allowedID == id {
								logger.Infof(ctx, "User %s accessing KB %s via shared agent %s (mode=selected)", userID.(string), id, agentID)
								return kb, id, kb.TenantID, types.OrgRoleViewer, nil
							}
						}
					}
				}
			}
		} else {
			// No agent_id in query: allow if user has any shared agent that can access this KB (e.g. from space list "通过智能体可见")
			can, err := h.agentShareService.UserCanAccessKBViaSomeSharedAgent(ctx, userID.(string), currentTenantID, kb)
			if err == nil && can {
				logger.Infof(ctx, "User %s accessing KB %s via some shared agent (no agent_id in query)", userID.(string), id)
				return kb, id, kb.TenantID, types.OrgRoleViewer, nil
			}
		}
	}

	// No permission: not owner and no shared access
	logger.Warnf(
		ctx,
		"Tenant has no permission to access this knowledge base, knowledge base ID: %s, "+
			"request tenant ID: %d, knowledge base tenant ID: %d",
		id, tenantID.(uint64), kb.TenantID,
	)
	return nil, id, 0, "", apperrors.NewForbiddenError("No permission to operate")
}

// GetKnowledgeBase godoc
// @Summary      获取知识库详情
// @Description  根据ID获取知识库详情。当使用共享智能体时，可传 agent_id 以校验该智能体是否有权访问该知识库。支持组织共享与直接共享。
// @Tags         知识库
// @Accept       json
// @Produce      json
// @Param        id         path      string  true   "知识库ID"
// @Param        agent_id   query     string  false  "共享智能体 ID（用于校验智能体是否有权访问该知识库）"
// @Success      200  {object}  map[string]interface{}  "知识库详情"
// @Failure      400  {object}  errors.AppError         "请求参数错误"
// @Failure      404  {object}  errors.AppError         "知识库不存在"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /knowledge-bases/{id} [get]
func (h *KnowledgeBaseHandler) GetKnowledgeBase(c *gin.Context) {
	// Validate and get the knowledge base
	kb, _, _, permission, err := h.validateAndGetKnowledgeBase(c)
	if err != nil {
		c.Error(err)
		return
	}
	// Fill counts (knowledge_count, chunk_count, is_processing) so hover/detail shows correct numbers
	if fillErr := h.service.FillKnowledgeBaseCounts(c.Request.Context(), kb); fillErr != nil {
		logger.Warnf(c.Request.Context(), "Failed to fill KB counts for %s: %v", kb.ID, fillErr)
	}
	tenantID := c.GetUint64(types.TenantIDContextKey.String())
	data := interface{}(kb)
	if kb.TenantID != tenantID && permission != "" {
		// Include my_permission in data so frontend can show role (e.g. "只读") instead of "--" for agent-visible KBs
		var dataMap map[string]interface{}
		b, _ := json.Marshal(kb)
		_ = json.Unmarshal(b, &dataMap)
		if dataMap != nil {
			dataMap["my_permission"] = permission
			data = dataMap
		}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

// ListKnowledgeBases godoc
// @Summary      获取知识库列表
// @Description  获取当前租户的所有知识库；或当传入 agent_id（共享智能体）时，校验权限后返回该智能体配置的知识库范围（用于 @ 提及）
// @Tags         知识库
// @Accept       json
// @Produce      json
// @Param        agent_id  query     string  false  "共享智能体 ID（传入时返回该智能体可用的知识库）"
// @Success      200  {object}  map[string]interface{}  "知识库列表"
// @Failure      500  {object}  errors.AppError         "服务器错误"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /knowledge-bases [get]
func (h *KnowledgeBaseHandler) ListKnowledgeBases(c *gin.Context) {
	ctx := c.Request.Context()

	agentID := c.Query("agent_id")
	if agentID != "" {
		userIDVal, ok := c.Get(types.UserIDContextKey.String())
		if !ok {
			c.Error(apperrors.NewUnauthorizedError("user ID not found"))
			return
		}
		userID, _ := userIDVal.(string)
		currentTenantID := c.GetUint64(types.TenantIDContextKey.String())
		if currentTenantID == 0 {
			c.Error(apperrors.NewUnauthorizedError("tenant ID not found"))
			return
		}
		agent, err := h.agentShareService.GetSharedAgentForUser(ctx, userID, currentTenantID, agentID)
		if err != nil {
			if stderrors.Is(err, service.ErrAgentShareNotFound) || stderrors.Is(err, service.ErrAgentSharePermission) || stderrors.Is(err, service.ErrAgentNotFoundForShare) {
				c.Error(apperrors.NewForbiddenError("no permission for this shared agent"))
				return
			}
			logger.ErrorWithFields(ctx, err, nil)
			c.Error(apperrors.NewInternalServerError(err.Error()))
			return
		}
		mode := agent.Config.KBSelectionMode
		if mode == "none" {
			c.JSON(http.StatusOK, gin.H{"success": true, "data": []interface{}{}})
			return
		}
		sourceTenantID := agent.TenantID
		kbs, err := h.service.ListKnowledgeBasesByTenantID(ctx, sourceTenantID)
		if err != nil {
			logger.ErrorWithFields(ctx, err, nil)
			c.Error(apperrors.NewInternalServerError(err.Error()))
			return
		}
		if mode == "selected" && len(agent.Config.KnowledgeBases) > 0 {
			allowed := make(map[string]bool)
			for _, id := range agent.Config.KnowledgeBases {
				allowed[id] = true
			}
			filtered := make([]*types.KnowledgeBase, 0, len(kbs))
			for _, kb := range kbs {
				if allowed[kb.ID] {
					filtered = append(filtered, kb)
				}
			}
			kbs = filtered
		}
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    kbs,
		})
		return
	}

	// Get all knowledge bases for this tenant (including direct shared via ListUserKnowledgeBases)
	kbs, err := h.sharedKBService.ListUserKnowledgeBases(ctx, true)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(apperrors.NewInternalServerError(err.Error()))
		return
	}

	// knowledge_count / chunk_count 为派生字段，ListUserKnowledgeBases 不填充；前端 @ 列表等依赖列表 JSON
	for _, kb := range kbs {
		if kb == nil {
			continue
		}
		if fillErr := h.service.FillKnowledgeBaseCounts(ctx, kb); fillErr != nil {
			logger.Warnf(ctx, "FillKnowledgeBaseCounts failed for KB %s: %v", kb.ID, fillErr)
		}
	}

	// Get share counts for all knowledge bases (org share)
	if len(kbs) > 0 && h.kbShareService != nil {
		kbIDs := make([]string, len(kbs))
		for i, kb := range kbs {
			kbIDs[i] = kb.ID
		}

		shareCounts, err := h.kbShareService.CountSharesByKnowledgeBaseIDs(ctx, kbIDs)
		if err != nil {
			logger.Warnf(ctx, "Failed to get share counts: %v", err)
		} else {
			for _, kb := range kbs {
				if count, ok := shareCounts[kb.ID]; ok {
					kb.ShareCount = count
				}
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    kbs,
	})
}

// TogglePinKnowledgeBase godoc
// @Summary      置顶/取消置顶知识库
// @Description  切换知识库的置顶状态
// @Tags         知识库
// @Accept       json
// @Produce      json
// @Param        id  path      string  true  "知识库ID"
// @Success      200  {object}  map[string]interface{}  "更新后的知识库"
// @Failure      404  {object}  errors.AppError         "知识库不存在"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /knowledge-bases/{id}/pin [put]
func (h *KnowledgeBaseHandler) TogglePinKnowledgeBase(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Param("id")
	if id == "" {
		c.Error(apperrors.NewBadRequestError("knowledge base ID is required"))
		return
	}

	kb, err := h.service.TogglePinKnowledgeBase(ctx, id)
	if err != nil {
		if stderrors.Is(err, repository.ErrKnowledgeBaseNotFound) {
			c.Error(apperrors.NewNotFoundError("knowledge base not found"))
			return
		}
		if stderrors.Is(err, service.ErrSharedKnowledgeBasePinNotAllowed) {
			c.Error(apperrors.NewBadRequestError("shared knowledge base does not support pin operation"))
			return
		}
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(apperrors.NewInternalServerError(err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    kb,
	})
}

// UpdateKnowledgeBaseRequest defines the request body structure for updating a knowledge base
type UpdateKnowledgeBaseRequest struct {
	Name        string                     `json:"name"        binding:"required"`
	Description string                     `json:"description"`
	Config      *types.KnowledgeBaseConfig `json:"config"`
}

// UpdateKnowledgeBase godoc
// @Summary      更新知识库
// @Description  更新知识库的名称、描述和配置
// @Tags         知识库
// @Accept       json
// @Produce      json
// @Param        id       path      string                     true  "知识库ID"
// @Param        request  body      UpdateKnowledgeBaseRequest true  "更新请求"
// @Success      200      {object}  map[string]interface{}     "更新后的知识库"
// @Failure      400      {object}  errors.AppError            "请求参数错误"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /knowledge-bases/{id} [put]
func (h *KnowledgeBaseHandler) UpdateKnowledgeBase(c *gin.Context) {
	ctx := c.Request.Context()
	logger.Info(ctx, "Start updating knowledge base")

	// Validate and get the knowledge base
	_, id, _, permission, err := h.validateAndGetKnowledgeBase(c)
	if err != nil {
		c.Error(err)
		return
	}

	// Only admin/editor can update knowledge base
	if permission != types.OrgRoleAdmin && permission != types.OrgRoleEditor {
		c.Error(apperrors.NewForbiddenError("No permission to update knowledge base"))
		return
	}

	// Parse request body
	var req UpdateKnowledgeBaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error(ctx, "Failed to parse request parameters", err)
		c.Error(apperrors.NewBadRequestError("Invalid request parameters").WithDetails(err.Error()))
		return
	}

	logger.Infof(ctx, "Updating knowledge base, ID: %s, name: %s",
		secutils.SanitizeForLog(id), secutils.SanitizeForLog(req.Name))

	// Update the knowledge base
	kb, err := h.service.UpdateKnowledgeBase(ctx, id, req.Name, req.Description, req.Config)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(apperrors.NewInternalServerError(err.Error()))
		return
	}

	logger.Infof(ctx, "Knowledge base updated successfully, ID: %s",
		secutils.SanitizeForLog(id))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    kb,
	})
}

// DeleteKnowledgeBase godoc
// @Summary      删除知识库
// @Description  删除指定的知识库及其所有内容
// @Tags         知识库
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "知识库ID"
// @Success      200  {object}  map[string]interface{}  "删除成功"
// @Failure      400  {object}  errors.AppError         "请求参数错误"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /knowledge-bases/{id} [delete]
func (h *KnowledgeBaseHandler) DeleteKnowledgeBase(c *gin.Context) {
	ctx := c.Request.Context()
	logger.Info(ctx, "Start deleting knowledge base")

	// Validate and get the knowledge base
	kb, id, _, permission, err := h.validateAndGetKnowledgeBase(c)
	if err != nil {
		c.Error(err)
		return
	}

	// Only owner (admin with matching tenant) can delete knowledge base
	tenantID, _ := c.Get(types.TenantIDContextKey.String())
	if kb.TenantID != tenantID.(uint64) || permission != types.OrgRoleAdmin {
		c.Error(apperrors.NewForbiddenError("Only knowledge base owner can delete"))
		return
	}

	logger.Infof(ctx, "Deleting knowledge base, ID: %s, name: %s",
		secutils.SanitizeForLog(id), secutils.SanitizeForLog(kb.Name))

	// Delete the knowledge base
	if err := h.service.DeleteKnowledgeBase(ctx, id); err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(apperrors.NewInternalServerError(err.Error()))
		return
	}

	logger.Infof(ctx, "Knowledge base deleted successfully, ID: %s",
		secutils.SanitizeForLog(id))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Knowledge base deleted successfully",
	})
}

type CopyKnowledgeBaseRequest struct {
	TaskID   string `json:"task_id"`
	SourceID string `json:"source_id" binding:"required"`
	TargetID string `json:"target_id"`
}

// CopyKnowledgeBaseResponse defines the response for copy knowledge base
type CopyKnowledgeBaseResponse struct {
	TaskID   string `json:"task_id"`
	SourceID string `json:"source_id"`
	TargetID string `json:"target_id"`
	Message  string `json:"message"`
}

// CopyKnowledgeBase godoc
// @Summary      复制知识库
// @Description  将一个知识库的内容复制到另一个知识库（异步任务）
// @Tags         知识库
// @Accept       json
// @Produce      json
// @Param        request  body      CopyKnowledgeBaseRequest   true  "复制请求"
// @Success      200      {object}  map[string]interface{}     "任务ID"
// @Failure      400      {object}  errors.AppError            "请求参数错误"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /knowledge-bases/copy [post]
func (h *KnowledgeBaseHandler) CopyKnowledgeBase(c *gin.Context) {
	ctx := c.Request.Context()
	var req CopyKnowledgeBaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error(ctx, "Failed to parse request parameters", err)
		c.Error(apperrors.NewBadRequestError("Invalid request parameters").WithDetails(err.Error()))
		return
	}

	// Get tenant ID from context
	tenantID, exists := c.Get(types.TenantIDContextKey.String())
	if !exists {
		logger.Error(ctx, "Failed to get tenant ID")
		c.Error(apperrors.NewUnauthorizedError("Unauthorized"))
		return
	}

	// Validate source knowledge base exists and belongs to caller's tenant (prevent cross-tenant clone)
	sourceKB, err := h.service.GetKnowledgeBaseByID(ctx, req.SourceID)
	if err != nil {
		if stderrors.Is(err, repository.ErrKnowledgeBaseNotFound) {
			c.Error(errors.NewNotFoundError("Source knowledge base not found"))
			return
		}
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}
	if sourceKB.TenantID != tenantID.(uint64) {
		logger.Warnf(ctx,
			"Copy rejected: source knowledge base belongs to another tenant, source_id: %s, caller_tenant: %d, kb_tenant: %d",
			secutils.SanitizeForLog(req.SourceID), tenantID.(uint64), sourceKB.TenantID)
		c.Error(errors.NewForbiddenError("No permission to copy this knowledge base"))
		return
	}

	// If target_id provided, validate target belongs to caller's tenant
	if req.TargetID != "" {
		targetKB, err := h.service.GetKnowledgeBaseByID(ctx, req.TargetID)
		if err != nil {
			if stderrors.Is(err, repository.ErrKnowledgeBaseNotFound) {
				c.Error(errors.NewNotFoundError("Target knowledge base not found"))
				return
			}
			logger.ErrorWithFields(ctx, err, nil)
			c.Error(errors.NewInternalServerError(err.Error()))
			return
		}
		if targetKB.TenantID != tenantID.(uint64) {
			logger.Warnf(ctx, "Copy rejected: target knowledge base belongs to another tenant, target_id: %s",
				secutils.SanitizeForLog(req.TargetID))
			c.Error(errors.NewForbiddenError("No permission to copy to this knowledge base"))
			return
		}
	}

	// Generate task ID if not provided
	taskID := req.TaskID
	if taskID == "" {
		taskID = utils.GenerateTaskID("kb_clone", tenantID.(uint64), req.SourceID)
	}

	// Create KB clone payload
	payload := types.KBClonePayload{
		TenantID: tenantID.(uint64),
		TaskID:   taskID,
		SourceID: req.SourceID,
		TargetID: req.TargetID,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		logger.Errorf(ctx, "Failed to marshal KB clone payload: %v", err)
		c.Error(apperrors.NewInternalServerError("Failed to create task"))
		return
	}

	// Enqueue KB clone task to Asynq
	task := asynq.NewTask(types.TypeKBClone, payloadBytes,
		asynq.TaskID(taskID), asynq.Queue("default"), asynq.MaxRetry(3))
	info, err := h.asynqClient.Enqueue(task)
	if err != nil {
		logger.Errorf(ctx, "Failed to enqueue KB clone task: %v", err)
		c.Error(apperrors.NewInternalServerError("Failed to enqueue task"))
		return
	}

	logger.Infof(ctx, "KB clone task enqueued: %s, asynq task ID: %s, source: %s, target: %s",
		taskID, info.ID, secutils.SanitizeForLog(req.SourceID), secutils.SanitizeForLog(req.TargetID))

	// Save initial progress to Redis so frontend can query immediately
	initialProgress := &types.KBCloneProgress{
		TaskID:    taskID,
		SourceID:  req.SourceID,
		TargetID:  req.TargetID,
		Status:    types.KBCloneStatusPending,
		Progress:  0,
		Message:   "Task queued, waiting to start...",
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}
	if err := h.knowledgeService.SaveKBCloneProgress(ctx, initialProgress); err != nil {
		logger.Warnf(ctx, "Failed to save initial KB clone progress: %v", err)
		// Don't fail the request, task is already enqueued
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": CopyKnowledgeBaseResponse{
			TaskID:   taskID,
			SourceID: req.SourceID,
			TargetID: req.TargetID,
			Message:  "Knowledge base copy task started",
		},
	})
}

// GetKBCloneProgress godoc
// @Summary      获取知识库复制进度
// @Description  获取知识库复制任务的进度
// @Tags         知识库
// @Accept       json
// @Produce      json
// @Param        task_id  path      string  true  "任务ID"
// @Success      200      {object}  map[string]interface{}  "进度信息"
// @Failure      404      {object}  errors.AppError         "任务不存在"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /knowledge-bases/copy/progress/{task_id} [get]
func (h *KnowledgeBaseHandler) GetKBCloneProgress(c *gin.Context) {
	ctx := c.Request.Context()

	taskID := c.Param("task_id")
	if taskID == "" {
		logger.Error(ctx, "Task ID is empty")
		c.Error(apperrors.NewBadRequestError("Task ID cannot be empty"))
		return
	}

	progress, err := h.knowledgeService.GetKBCloneProgress(ctx, taskID)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    progress,
	})
}

// validateExtractConfig validates the graph configuration parameters
func validateExtractConfig(config *types.ExtractConfig) error {
	if config == nil {
		return nil
	}
	if !config.Enabled {
		*config = types.ExtractConfig{Enabled: false}
		return nil
	}
	// Validate text field
	if config.Text == "" {
		return apperrors.NewBadRequestError("text cannot be empty")
	}

	// Validate tags field
	if len(config.Tags) == 0 {
		return apperrors.NewBadRequestError("tags cannot be empty")
	}
	for i, tag := range config.Tags {
		if tag == "" {
			return apperrors.NewBadRequestError("tag cannot be empty at index " + strconv.Itoa(i))
		}
	}

	// Validate nodes
	if len(config.Nodes) == 0 {
		return apperrors.NewBadRequestError("nodes cannot be empty")
	}
	nodeNames := make(map[string]bool)
	for i, node := range config.Nodes {
		if node.Name == "" {
			return apperrors.NewBadRequestError("node name cannot be empty at index " + strconv.Itoa(i))
		}
		// Check for duplicate node names
		if nodeNames[node.Name] {
			return apperrors.NewBadRequestError("duplicate node name: " + node.Name)
		}
		nodeNames[node.Name] = true
	}

	if len(config.Relations) == 0 {
		return apperrors.NewBadRequestError("relations cannot be empty")
	}
	// Validate relations
	for i, relation := range config.Relations {
		if relation.Node1 == "" {
			return apperrors.NewBadRequestError("relation node1 cannot be empty at index " + strconv.Itoa(i))
		}
		if relation.Node2 == "" {
			return apperrors.NewBadRequestError("relation node2 cannot be empty at index " + strconv.Itoa(i))
		}
		if relation.Type == "" {
			return apperrors.NewBadRequestError("relation type cannot be empty at index " + strconv.Itoa(i))
		}
		// Check if referenced nodes exist
		if !nodeNames[relation.Node1] {
			return apperrors.NewBadRequestError("relation references non-existent node1: " + relation.Node1)
		}
		if !nodeNames[relation.Node2] {
			return apperrors.NewBadRequestError("relation references non-existent node2: " + relation.Node2)
		}
	}

	return nil
}

// ListMoveTargets returns knowledge bases eligible as move targets for the given source KB.
// Filters: same Type, same EmbeddingModelID, different ID, not temporary.
func (h *KnowledgeBaseHandler) ListMoveTargets(c *gin.Context) {
	ctx := c.Request.Context()

	sourceKBID := c.Param("id")
	if sourceKBID == "" {
		c.Error(apperrors.NewBadRequestError("Knowledge base ID is required"))
		return
	}

	tenantID, exists := c.Get(types.TenantIDContextKey.String())
	if !exists {
		c.Error(apperrors.NewUnauthorizedError("Unauthorized"))
		return
	}

	// Get source knowledge base
	sourceKB, err := h.service.GetKnowledgeBaseByID(ctx, sourceKBID)
	if err != nil {
		if stderrors.Is(err, repository.ErrKnowledgeBaseNotFound) {
			c.Error(errors.NewNotFoundError("Source knowledge base not found"))
			return
		}
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}
	if sourceKB.TenantID != tenantID.(uint64) {
		c.Error(errors.NewForbiddenError("No permission to access this knowledge base"))
		return
	}

	// Get all knowledge bases
	allKBs, err := h.service.ListKnowledgeBases(ctx)
	if err != nil {
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}

	// Filter eligible targets
	targets := make([]*types.KnowledgeBase, 0)
	for _, kb := range allKBs {
		if kb.ID == sourceKBID {
			continue
		}
		if kb.IsTemporary {
			continue
		}
		if kb.Type != sourceKB.Type {
			continue
		}
		if kb.EmbeddingModelID != sourceKB.EmbeddingModelID {
			continue
		}
		targets = append(targets, kb)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    targets,
	})
}

// CreateSharedKnowledgeBase 创建共享知识库
// @Summary      创建共享知识库
// @Description  创建新的共享知识库
// @Tags         知识库
// @Accept       json
// @Produce      json
// @Param        request  body      types.KnowledgeBase  true  "知识库信息（visibility 必须为 'shared'）"
// @Success      201      {object}  map[string]interface{}  "创建的共享知识库"
// @Failure      400      {object}  errors.AppError         "请求参数错误"
// @Security     Bearer
// @Router       /knowledge-bases/shared [post]
func (h *KnowledgeBaseHandler) CreateSharedKnowledgeBase(c *gin.Context) {
	ctx := c.Request.Context()

	logger.Info(ctx, "Start creating shared knowledge base")

	var req types.KnowledgeBase
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error(ctx, "Failed to parse request parameters", err)
		c.Error(errors.NewBadRequestError("Invalid request parameters").WithDetails(err.Error()))
		return
	}

	// 确保 visibility 为 shared
	req.Visibility = types.KnowledgeBaseVisibilityShared

	kb, err := h.sharedKBService.CreateSharedKnowledgeBase(ctx, &req)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}

	logger.Infof(ctx, "Shared knowledge base created successfully, ID: %s", secutils.SanitizeForLog(kb.ID))
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    kb,
	})
}

// ListSharedKnowledgeBases 列出共享知识库广场
// @Summary      列出共享知识库广场
// @Description  列出所有共享知识库，支持搜索和分页
// @Tags         知识库
// @Accept       json
// @Produce      json
// @Param        keyword   query     string  false  "搜索关键词"
// @Param        page      query     int     false  "页码（默认1）"
// @Param        page_size query     int     false  "每页数量（默认10）"
// @Success      200       {object}  map[string]interface{}  "共享知识库列表"
// @Security     Bearer
// @Router       /knowledge-bases/shared [get]
func (h *KnowledgeBaseHandler) ListSharedKnowledgeBases(c *gin.Context) {
	ctx := c.Request.Context()

	keyword := c.DefaultQuery("keyword", "")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}

	kbs, total, err := h.sharedKBService.ListSharedKnowledgeBases(ctx, keyword, page, pageSize)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}

	// 为每个知识库计算知识数量
	knowledgeRepo := h.knowledgeService.GetRepository()
	chunkRepo := h.chunkService.GetRepository()

	result := make([]map[string]interface{}, 0, len(kbs))
	for _, kb := range kbs {
		// 计算知识库统计信息（文档数量/分块数量）
		// 注意：使用知识库自己的 tenant_id，而不是当前用户的 tenant_id
		var knowledgeCount int64
		var chunkCount int64

		switch kb.Type {
		case types.KnowledgeBaseTypeDocument:
			// 文档类型：统计知识数量（使用知识库的 tenant_id）
			count, err := knowledgeRepo.CountKnowledgeByKnowledgeBaseID(ctx, kb.TenantID, kb.ID)
			if err == nil {
				knowledgeCount = count
			}
		case types.KnowledgeBaseTypeFAQ:
			// FAQ类型：统计分块数量（使用知识库的 tenant_id）
			count, err := chunkRepo.CountChunksByKnowledgeBaseID(ctx, kb.TenantID, kb.ID)
			if err == nil {
				chunkCount = count
			}
		}

		kbMap := map[string]interface{}{
			"id":              kb.ID,
			"name":            kb.Name,
			"description":     kb.Description,
			"type":            kb.Type,
			"visibility":      kb.Visibility,
			"owner_id":        kb.OwnerID,
			"shared_at":       kb.SharedAt,
			"member_count":    kb.MemberCount,
			"knowledge_count": knowledgeCount,
			"chunk_count":     chunkCount,
			"created_at":      kb.CreatedAt,
			"updated_at":      kb.UpdatedAt,
		}
		result = append(result, kbMap)
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"data":      result,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// ListUserKnowledgeBases 列出用户的知识库（个人 + 加入的共享知识库）
// @Summary      获取用户的知识库列表
// @Description  返回用户创建的个人知识库和加入的共享知识库
// @Tags         知识库
// @Accept       json
// @Produce      json
// @Param        include_shared  query     bool  false  "是否包含共享知识库（默认true）"
// @Success      200             {object}  map[string]interface{}  "知识库列表"
// @Failure      500             {object}  errors.AppError         "服务器错误"
// @Security     Bearer
// @Router       /knowledge-bases/user [get]
func (h *KnowledgeBaseHandler) ListUserKnowledgeBases(c *gin.Context) {
	ctx := c.Request.Context()

	includeShared := c.DefaultQuery("include_shared", "true") == "true"

	kbs, err := h.sharedKBService.ListUserKnowledgeBases(ctx, includeShared)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}

	// 为每个知识库添加 is_owner 和 member_role 字段
	userID := ctx.Value(types.UserIDContextKey).(string)

	// 批量查询所有共享知识库的成员信息（优化性能）
	sharedKBIDs := make([]string, 0)
	for _, kb := range kbs {
		if kb.Visibility == types.KnowledgeBaseVisibilityShared {
			sharedKBIDs = append(sharedKBIDs, kb.ID)
		}
	}

	// 构建成员角色映射表（知识库ID -> 成员角色）
	memberRoleMap := make(map[string]string)
	if len(sharedKBIDs) > 0 {
		// 通过 Service 层批量查询当前用户在所有共享知识库中的成员角色（更高效）
		for _, kbID := range sharedKBIDs {
			role, err := h.sharedKBService.GetMemberRoleByKBAndUser(ctx, kbID, userID)
			if err == nil && role != "" {
				memberRoleMap[kbID] = role
			}
		}
	}

	result := make([]map[string]interface{}, 0, len(kbs))

	// 获取 repository 用于统计
	knowledgeRepo := h.knowledgeService.GetRepository()
	chunkRepo := h.chunkService.GetRepository()

	for _, kb := range kbs {
		// 计算知识库统计信息（文档数量/分块数量）
		// 注意：使用知识库自己的 tenant_id，而不是当前用户的 tenant_id
		var knowledgeCount int64
		var chunkCount int64

		switch kb.Type {
		case types.KnowledgeBaseTypeDocument:
			// 文档类型：统计知识数量（使用知识库的 tenant_id）
			count, err := knowledgeRepo.CountKnowledgeByKnowledgeBaseID(ctx, kb.TenantID, kb.ID)
			if err == nil {
				knowledgeCount = count
			}
		case types.KnowledgeBaseTypeFAQ:
			// FAQ类型：统计分块数量（使用知识库的 tenant_id）
			count, err := chunkRepo.CountChunksByKnowledgeBaseID(ctx, kb.TenantID, kb.ID)
			if err == nil {
				chunkCount = count
			}
		}

		kbMap := map[string]interface{}{
			"id":                         kb.ID,
			"name":                       kb.Name,
			"description":                kb.Description,
			"type":                       kb.Type,
			"visibility":                 kb.Visibility,
			"owner_id":                   kb.OwnerID,
			"is_pinned":                  kb.IsPinned,
			"pinned_at":                  kb.PinnedAt,
			"shared_at":                  kb.SharedAt,
			"member_count":               kb.MemberCount,
			"tenant_id":                  kb.TenantID,
			"embedding_model_id":         kb.EmbeddingModelID,
			"summary_model_id":           kb.SummaryModelID,
			"chunking_config":            kb.ChunkingConfig,
			"image_processing_config":    kb.ImageProcessingConfig,
			"vlm_config":                 kb.VLMConfig,
			"cos_config":                 kb.StorageConfig,
			"extract_config":             kb.ExtractConfig,
			"faq_config":                 kb.FAQConfig,
			"question_generation_config": kb.QuestionGenerationConfig,
			"created_at":                 kb.CreatedAt,
			"updated_at":                 kb.UpdatedAt,
			"knowledge_count":            knowledgeCount,
			"chunk_count":                chunkCount,
		}

		// 判断是否为创建者
		isOwner := kb.OwnerID == userID
		kbMap["is_owner"] = isOwner

		// 设置成员角色
		if kb.Visibility == types.KnowledgeBaseVisibilityShared {
			// 共享知识库：从映射表获取或设置为 owner（如果是创建者）
			if role, ok := memberRoleMap[kb.ID]; ok {
				kbMap["member_role"] = role
			} else if isOwner {
				kbMap["member_role"] = types.KBMemberRoleOwner
			}
		} else {
			// 个人知识库，角色为 owner
			kbMap["member_role"] = types.KBMemberRoleOwner
		}

		result = append(result, kbMap)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

// JoinSharedKnowledgeBase 加入共享知识库
// @Summary      加入共享知识库
// @Description  当前用户加入指定的共享知识库
// @Tags         知识库
// @Accept       json
// @Produce      json
// @Param        id       path      string  true  "知识库ID"
// @Success      200      {object}  map[string]interface{}  "成功"
// @Failure      400      {object}  errors.AppError         "请求参数错误"
// @Security     Bearer
// @Router       /knowledge-bases/{id}/join [post]
func (h *KnowledgeBaseHandler) JoinSharedKnowledgeBase(c *gin.Context) {
	ctx := c.Request.Context()

	kbID := secutils.SanitizeForLog(c.Param("id"))
	if kbID == "" {
		c.Error(errors.NewBadRequestError("Knowledge base ID cannot be empty"))
		return
	}

	if err := h.sharedKBService.JoinSharedKnowledgeBase(ctx, kbID); err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Successfully joined knowledge base",
	})
}

// LeaveSharedKnowledgeBase 离开共享知识库
// @Summary      离开共享知识库
// @Description  当前用户离开指定的共享知识库
// @Tags         知识库
// @Accept       json
// @Produce      json
// @Param        id       path      string  true  "知识库ID"
// @Success      200      {object}  map[string]interface{}  "成功"
// @Failure      400      {object}  errors.AppError         "请求参数错误"
// @Security     Bearer
// @Router       /knowledge-bases/{id}/leave [post]
func (h *KnowledgeBaseHandler) LeaveSharedKnowledgeBase(c *gin.Context) {
	ctx := c.Request.Context()

	kbID := secutils.SanitizeForLog(c.Param("id"))
	if kbID == "" {
		c.Error(errors.NewBadRequestError("Knowledge base ID cannot be empty"))
		return
	}

	if err := h.sharedKBService.LeaveSharedKnowledgeBase(ctx, kbID); err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Successfully left knowledge base",
	})
}

// ListKnowledgeBaseMembers 列出知识库成员
// @Summary      列出知识库成员
// @Description  列出指定知识库的所有成员
// @Tags         知识库
// @Accept       json
// @Produce      json
// @Param        id         path      string  true  "知识库ID"
// @Param        keyword    query     string  false "搜索关键词（按 email/username/姓名）"
// @Param        page       query     int     false  "页码（默认1）"
// @Param        page_size  query     int     false  "每页数量（默认10）"
// @Success      200        {object}  map[string]interface{}  "成员列表"
// @Security     Bearer
// @Router       /knowledge-bases/{id}/members [get]
func (h *KnowledgeBaseHandler) ListKnowledgeBaseMembers(c *gin.Context) {
	ctx := c.Request.Context()

	kbID := secutils.SanitizeForLog(c.Param("id"))
	if kbID == "" {
		c.Error(errors.NewBadRequestError("Knowledge base ID cannot be empty"))
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}

	keyword := strings.TrimSpace(c.Query("keyword"))
	members, total, err := h.sharedKBService.ListKnowledgeBaseMembers(ctx, kbID, keyword, page, pageSize)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"data":      members,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// UpdateMemberRole 更新成员权限
// @Summary      更新成员权限
// @Description  更新知识库成员的权限（仅创建者可操作）
// @Tags         知识库
// @Accept       json
// @Produce      json
// @Param        id        path      string  true  "知识库ID"
// @Param        user_id   path      string  true  "用户ID"
// @Param        request   body      map[string]string  true  "权限信息（role: 'viewer' | 'editor'）"
// @Success      200       {object}  map[string]interface{}  "成功"
// @Failure      400       {object}  errors.AppError         "请求参数错误"
// @Security     Bearer
// @Router       /knowledge-bases/{id}/members/{user_id} [put]
func (h *KnowledgeBaseHandler) UpdateMemberRole(c *gin.Context) {
	ctx := c.Request.Context()

	kbID := secutils.SanitizeForLog(c.Param("id"))
	userID := secutils.SanitizeForLog(c.Param("user_id"))

	if kbID == "" || userID == "" {
		c.Error(errors.NewBadRequestError("Knowledge base ID and user ID cannot be empty"))
		return
	}

	var req struct {
		Role string `json:"role" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errors.NewBadRequestError("Invalid request parameters").WithDetails(err.Error()))
		return
	}

	if err := h.sharedKBService.UpdateMemberRole(ctx, kbID, userID, req.Role); err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Member role updated successfully",
	})
}

// RemoveMember 移除成员
// @Summary      移除成员
// @Description  从知识库中移除成员（仅创建者可操作）
// @Tags         知识库
// @Accept       json
// @Produce      json
// @Param        id        path      string  true  "知识库ID"
// @Param        user_id   path      string  true  "用户ID"
// @Success      200       {object}  map[string]interface{}  "成功"
// @Failure      400       {object}  errors.AppError         "请求参数错误"
// @Security     Bearer
// @Router       /knowledge-bases/{id}/members/{user_id} [delete]
func (h *KnowledgeBaseHandler) RemoveMember(c *gin.Context) {
	ctx := c.Request.Context()

	kbID := secutils.SanitizeForLog(c.Param("id"))
	userID := secutils.SanitizeForLog(c.Param("user_id"))

	if kbID == "" || userID == "" {
		c.Error(errors.NewBadRequestError("Knowledge base ID and user ID cannot be empty"))
		return
	}

	if err := h.sharedKBService.RemoveMember(ctx, kbID, userID); err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Member removed successfully",
	})
}
