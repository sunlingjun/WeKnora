package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/google/uuid"
)

// sharedKnowledgeBaseService 实现共享知识库服务
type sharedKnowledgeBaseService struct {
	kbRepo     interfaces.KnowledgeBaseRepository
	memberRepo interfaces.KnowledgeBaseMemberRepository
	tenantRepo interfaces.TenantRepository
}

// NewSharedKnowledgeBaseService 创建共享知识库服务
func NewSharedKnowledgeBaseService(
	kbRepo interfaces.KnowledgeBaseRepository,
	memberRepo interfaces.KnowledgeBaseMemberRepository,
	tenantRepo interfaces.TenantRepository,
) interfaces.SharedKnowledgeBaseService {
	return &sharedKnowledgeBaseService{
		kbRepo:     kbRepo,
		memberRepo: memberRepo,
		tenantRepo: tenantRepo,
	}
}

// CreateSharedKnowledgeBase 创建共享知识库
func (s *sharedKnowledgeBaseService) CreateSharedKnowledgeBase(ctx context.Context, kb *types.KnowledgeBase) (*types.KnowledgeBase, error) {
	userID := ctx.Value(types.UserIDContextKey).(string)
	tenantID := ctx.Value(types.TenantIDContextKey).(uint64)

	// 设置共享知识库属性
	kb.Visibility = types.KnowledgeBaseVisibilityShared
	kb.OwnerID = userID
	now := time.Now()
	kb.SharedAt = &now
	kb.MemberCount = 1 // 创建者自己

	// 创建知识库
	if err := s.kbRepo.CreateKnowledgeBase(ctx, kb); err != nil {
		return nil, fmt.Errorf("failed to create shared knowledge base: %w", err)
	}

	// 创建 owner 成员记录
	member := &types.KnowledgeBaseMember{
		ID:              uuid.New().String(),
		KnowledgeBaseID: kb.ID,
		UserID:          userID,
		TenantID:        tenantID,
		Role:            types.KBMemberRoleOwner,
		JoinedAt:        now,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := s.memberRepo.CreateMember(ctx, member); err != nil {
		// 如果创建成员失败，删除知识库（回滚）
		if delErr := s.kbRepo.DeleteKnowledgeBase(ctx, kb.ID); delErr != nil {
			logger.Errorf(ctx, "Failed to rollback knowledge base after member creation failed: %v", delErr)
		}
		return nil, fmt.Errorf("failed to create owner member: %w", err)
	}

	logger.Infof(ctx, "Created shared knowledge base: %s (owner: %s)", kb.ID, userID)
	return kb, nil
}

// ListSharedKnowledgeBases 列出共享知识库广场（支持搜索、分页）
func (s *sharedKnowledgeBaseService) ListSharedKnowledgeBases(ctx context.Context, keyword string, page, pageSize int) ([]*types.KnowledgeBase, int64, error) {
	// 查询所有知识库（后续可以优化为直接查询共享知识库）
	allKBs, err := s.kbRepo.ListKnowledgeBases(ctx)
	if err != nil {
		return nil, 0, err
	}

	// 过滤共享知识库
	var sharedKBs []*types.KnowledgeBase
	for _, kb := range allKBs {
		if kb.Visibility == types.KnowledgeBaseVisibilityShared {
			// 关键词搜索
			if keyword == "" ||
				containsIgnoreCase(kb.Name, keyword) ||
				containsIgnoreCase(kb.Description, keyword) {
				sharedKBs = append(sharedKBs, kb)
			}
		}
	}

	// 分页处理
	total := int64(len(sharedKBs))
	start := (page - 1) * pageSize
	end := start + pageSize
	if start >= len(sharedKBs) {
		return []*types.KnowledgeBase{}, total, nil
	}
	if end > len(sharedKBs) {
		end = len(sharedKBs)
	}

	return sharedKBs[start:end], total, nil
}

// JoinSharedKnowledgeBase 加入共享知识库
func (s *sharedKnowledgeBaseService) JoinSharedKnowledgeBase(ctx context.Context, kbID string) error {
	userID := ctx.Value(types.UserIDContextKey).(string)
	tenantID := ctx.Value(types.TenantIDContextKey).(uint64)

	// 检查知识库是否存在且为共享知识库
	kb, err := s.kbRepo.GetKnowledgeBaseByID(ctx, kbID)
	if err != nil {
		return fmt.Errorf("knowledge base not found: %w", err)
	}
	if kb.Visibility != types.KnowledgeBaseVisibilityShared {
		return errors.New("knowledge base is not shared")
	}

	// 检查是否已经是成员
	existingMember, err := s.memberRepo.GetMemberByKBAndUser(ctx, kbID, userID)
	if err == nil && existingMember != nil {
		return errors.New("already a member of this knowledge base")
	}

	// 创建成员记录
	member := &types.KnowledgeBaseMember{
		ID:              uuid.New().String(),
		KnowledgeBaseID: kbID,
		UserID:          userID,
		TenantID:        tenantID,
		Role:            types.KBMemberRoleViewer, // 默认角色为 viewer
		JoinedAt:        time.Now(),
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	if err := s.memberRepo.CreateMember(ctx, member); err != nil {
		return fmt.Errorf("failed to join knowledge base: %w", err)
	}

	// 更新成员数量
	count, err := s.memberRepo.CountMembersByKB(ctx, kbID)
	if err == nil {
		kb.MemberCount = int(count)
		s.kbRepo.UpdateKnowledgeBase(ctx, kb)
	}

	logger.Infof(ctx, "User %s joined shared knowledge base %s", userID, kbID)
	return nil
}

// LeaveSharedKnowledgeBase 离开共享知识库
func (s *sharedKnowledgeBaseService) LeaveSharedKnowledgeBase(ctx context.Context, kbID string) error {
	userID := ctx.Value(types.UserIDContextKey).(string)

	// 检查知识库是否存在且为共享知识库
	kb, err := s.kbRepo.GetKnowledgeBaseByID(ctx, kbID)
	if err != nil {
		return fmt.Errorf("knowledge base not found: %w", err)
	}
	if kb.Visibility != types.KnowledgeBaseVisibilityShared {
		return errors.New("knowledge base is not shared")
	}

	// 检查是否是创建者（创建者不能离开）
	if kb.OwnerID == userID {
		return errors.New("owner cannot leave the knowledge base")
	}

	// 检查是否是成员
	member, err := s.memberRepo.GetMemberByKBAndUser(ctx, kbID, userID)
	if err != nil {
		return fmt.Errorf("not a member of this knowledge base: %w", err)
	}

	// 双重检查：如果成员角色是 owner，也不能离开
	if member.Role == types.KBMemberRoleOwner {
		return errors.New("owner cannot leave the knowledge base")
	}

	// 删除成员记录
	if err := s.memberRepo.DeleteMember(ctx, kbID, userID); err != nil {
		return fmt.Errorf("failed to leave knowledge base: %w", err)
	}

	// 更新成员数量
	kb, err = s.kbRepo.GetKnowledgeBaseByID(ctx, kbID)
	if err == nil {
		count, err := s.memberRepo.CountMembersByKB(ctx, kbID)
		if err == nil {
			kb.MemberCount = int(count)
			s.kbRepo.UpdateKnowledgeBase(ctx, kb)
		}
	}

	logger.Infof(ctx, "User %s left shared knowledge base %s", userID, kbID)
	return nil
}

// ListKnowledgeBaseMembers 列出知识库成员
func (s *sharedKnowledgeBaseService) ListKnowledgeBaseMembers(ctx context.Context, kbID string, keyword string, page, pageSize int) ([]*types.KnowledgeBaseMember, int64, error) {
	return s.memberRepo.ListMembersByKB(ctx, kbID, keyword, page, pageSize)
}

// UpdateMemberRole 更新成员权限（仅创建者可操作）
func (s *sharedKnowledgeBaseService) UpdateMemberRole(ctx context.Context, kbID string, userID string, role string) error {
	// 验证角色
	if role != types.KBMemberRoleViewer && role != types.KBMemberRoleEditor {
		return errors.New("invalid role")
	}

	// 检查当前用户是否是创建者
	currentUserID := ctx.Value(types.UserIDContextKey).(string)
	kb, err := s.kbRepo.GetKnowledgeBaseByID(ctx, kbID)
	if err != nil {
		return fmt.Errorf("knowledge base not found: %w", err)
	}
	if kb.OwnerID != currentUserID {
		return errors.New("only owner can update member role")
	}

	// 不能修改创建者的角色
	if userID == kb.OwnerID {
		return errors.New("cannot update owner role")
	}

	// 更新成员角色
	return s.memberRepo.UpdateMemberRole(ctx, kbID, userID, role)
}

// RemoveMember 移除成员（仅创建者可操作）
func (s *sharedKnowledgeBaseService) RemoveMember(ctx context.Context, kbID string, userID string) error {
	// 检查当前用户是否是创建者
	currentUserID := ctx.Value(types.UserIDContextKey).(string)
	kb, err := s.kbRepo.GetKnowledgeBaseByID(ctx, kbID)
	if err != nil {
		return fmt.Errorf("knowledge base not found: %w", err)
	}
	if kb.OwnerID != currentUserID {
		return errors.New("only owner can remove members")
	}

	// 不能移除创建者
	if userID == kb.OwnerID {
		return errors.New("cannot remove owner")
	}

	// 删除成员
	if err := s.memberRepo.DeleteMember(ctx, kbID, userID); err != nil {
		return fmt.Errorf("failed to remove member: %w", err)
	}

	// 更新成员数量
	count, err := s.memberRepo.CountMembersByKB(ctx, kbID)
	if err == nil {
		kb.MemberCount = int(count)
		s.kbRepo.UpdateKnowledgeBase(ctx, kb)
	}

	logger.Infof(ctx, "Removed member %s from knowledge base %s", userID, kbID)
	return nil
}

// CheckMemberPermission 检查成员权限
func (s *sharedKnowledgeBaseService) CheckMemberPermission(ctx context.Context, kbID string, permission string) (bool, error) {
	userID := ctx.Value(types.UserIDContextKey).(string)

	// 获取知识库信息
	kb, err := s.kbRepo.GetKnowledgeBaseByID(ctx, kbID)
	if err != nil {
		return false, fmt.Errorf("knowledge base not found: %w", err)
	}

	// 如果是个人知识库，检查是否是创建者
	if kb.Visibility == types.KnowledgeBaseVisibilityPrivate {
		return kb.OwnerID == userID, nil
	}

	// 如果是共享知识库，检查成员权限
	member, err := s.memberRepo.GetMemberByKBAndUser(ctx, kbID, userID)
	if err != nil {
		return false, nil // 不是成员，无权限
	}

	switch permission {
	case "read":
		return member != nil, nil
	case "write":
		return member != nil && (member.Role == types.KBMemberRoleOwner || member.Role == types.KBMemberRoleEditor), nil
	case "delete":
		return member != nil && member.Role == types.KBMemberRoleOwner, nil
	default:
		return false, errors.New("invalid permission")
	}
}

// ListUserKnowledgeBases 列出用户的知识库（个人 + 创建的共享知识库 + 加入的共享知识库）
func (s *sharedKnowledgeBaseService) ListUserKnowledgeBases(ctx context.Context, includeShared bool) ([]*types.KnowledgeBase, error) {
	userID := ctx.Value(types.UserIDContextKey).(string)
	tenantID := ctx.Value(types.TenantIDContextKey).(uint64)

	var result []*types.KnowledgeBase
	kbIDMap := make(map[string]bool) // 用于去重（知识库ID -> 是否已添加）

	// 1. 查询个人知识库（visibility = 'private' AND owner_id = 当前用户ID）
	personalKBs, err := s.kbRepo.ListKnowledgeBasesByTenantID(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	for _, kb := range personalKBs {
		if kb.Visibility == types.KnowledgeBaseVisibilityPrivate && kb.OwnerID == userID {
			result = append(result, kb)
			kbIDMap[kb.ID] = true
		}
	}

	// 2. 查询共享知识库（包括创建的 + 加入的）
	if includeShared {
		// 2.1 查询用户创建的共享知识库（通过 owner_id）
		allKBs, err := s.kbRepo.ListKnowledgeBasesByTenantID(ctx, tenantID)
		if err != nil {
			return nil, err
		}
		for _, kb := range allKBs {
			if kb.Visibility == types.KnowledgeBaseVisibilityShared && kb.OwnerID == userID {
				// 避免重复添加
				if !kbIDMap[kb.ID] {
					result = append(result, kb)
					kbIDMap[kb.ID] = true
				}
			}
		}

		// 2.2 查询用户加入的共享知识库（通过成员表，不包括自己创建的）
		members, err := s.memberRepo.ListMembersByUser(ctx, userID)
		if err != nil {
			return nil, err
		}

		// 批量查询知识库（避免多次单独查询）
		kbIDs := make([]string, 0)
		for _, member := range members {
			// 只查询不是自己创建的知识库（避免重复）
			if !kbIDMap[member.KnowledgeBaseID] {
				kbIDs = append(kbIDs, member.KnowledgeBaseID)
			}
		}

		if len(kbIDs) > 0 {
			sharedKBs, err := s.kbRepo.GetKnowledgeBaseByIDs(ctx, kbIDs)
			if err == nil {
				for _, kb := range sharedKBs {
					if kb.Visibility == types.KnowledgeBaseVisibilityShared {
						result = append(result, kb)
						kbIDMap[kb.ID] = true
					}
				}
			}
		}
	}

	return result, nil
}

// GetMemberRoleByKBAndUser 获取用户在指定知识库中的角色
func (s *sharedKnowledgeBaseService) GetMemberRoleByKBAndUser(ctx context.Context, kbID string, userID string) (string, error) {
	member, err := s.memberRepo.GetMemberByKBAndUser(ctx, kbID, userID)
	if err != nil {
		// 成员不存在，检查是否为创建者
		kb, err := s.kbRepo.GetKnowledgeBaseByID(ctx, kbID)
		if err != nil {
			return "", err
		}
		if kb.OwnerID == userID {
			return types.KBMemberRoleOwner, nil
		}
		return "", nil // 不是成员也不是创建者
	}
	return member.Role, nil
}

// containsIgnoreCase 检查字符串是否包含子字符串（忽略大小写）
func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) &&
		(strings.Contains(strings.ToLower(s), strings.ToLower(substr)))
}
