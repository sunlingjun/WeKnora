package repository

import (
	"context"
	"errors"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/gorm"
)

var ErrKnowledgeBaseMemberNotFound = errors.New("knowledge base member not found")

// knowledgeBaseMemberRepository 实现知识库成员 Repository
type knowledgeBaseMemberRepository struct {
	db *gorm.DB
}

// NewKnowledgeBaseMemberRepository 创建知识库成员 Repository
func NewKnowledgeBaseMemberRepository(db *gorm.DB) interfaces.KnowledgeBaseMemberRepository {
	return &knowledgeBaseMemberRepository{db: db}
}

// CreateMember 创建成员记录
func (r *knowledgeBaseMemberRepository) CreateMember(ctx context.Context, member *types.KnowledgeBaseMember) error {
	return r.db.WithContext(ctx).Create(member).Error
}

// GetMemberByKBAndUser 根据知识库和用户查询成员
func (r *knowledgeBaseMemberRepository) GetMemberByKBAndUser(ctx context.Context, kbID string, userID string) (*types.KnowledgeBaseMember, error) {
	var member types.KnowledgeBaseMember
	if err := r.db.WithContext(ctx).
		Where("knowledge_base_id = ? AND user_id = ?", kbID, userID).
		First(&member).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrKnowledgeBaseMemberNotFound
		}
		return nil, err
	}
	return &member, nil
}

// GetSoftDeletedMemberByKBAndUser returns the most recently soft-deleted
// membership for (kb, user), if any. Used by Join to revive instead of insert.
func (r *knowledgeBaseMemberRepository) GetSoftDeletedMemberByKBAndUser(ctx context.Context, kbID string, userID string) (*types.KnowledgeBaseMember, error) {
	var member types.KnowledgeBaseMember
	if err := r.db.WithContext(ctx).Unscoped().
		Where("knowledge_base_id = ? AND user_id = ? AND deleted_at IS NOT NULL", kbID, userID).
		Order("deleted_at DESC").
		First(&member).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrKnowledgeBaseMemberNotFound
		}
		return nil, err
	}
	return &member, nil
}

// RestoreMember clears deleted_at and refreshes join metadata on a soft-deleted row.
func (r *knowledgeBaseMemberRepository) RestoreMember(ctx context.Context, member *types.KnowledgeBaseMember) error {
	now := time.Now()
	member.DeletedAt = gorm.DeletedAt{}
	member.JoinedAt = now
	member.UpdatedAt = now
	return r.db.WithContext(ctx).Unscoped().Model(member).Updates(map[string]interface{}{
		"deleted_at": nil,
		"role":       member.Role,
		"tenant_id":  member.TenantID,
		"joined_at":  member.JoinedAt,
		"updated_at": member.UpdatedAt,
	}).Error
}

// ListMembersByKB 列出知识库所有成员（支持分页，支持 keyword 按 email/username/cas_real_name 搜索）
func (r *knowledgeBaseMemberRepository) ListMembersByKB(ctx context.Context, kbID string, keyword string, page, pageSize int) ([]*types.KnowledgeBaseMember, int64, error) {
	var members []*types.KnowledgeBaseMember
	var total int64

	baseQuery := r.db.WithContext(ctx).Model(&types.KnowledgeBaseMember{}).
		Where("knowledge_base_members.knowledge_base_id = ?", kbID)

	// keyword 非空时：JOIN users 并按 email/username/cas_real_name 过滤
	if keyword != "" {
		likePattern := "%" + keyword + "%"
		baseQuery = baseQuery.
			Joins("JOIN users ON users.id = knowledge_base_members.user_id").
			Where("users.email LIKE ? OR users.username LIKE ? OR users.cas_real_name LIKE ?", likePattern, likePattern, likePattern)
	}

	// 统计总数
	if err := baseQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	if err := baseQuery.
		Preload("User").
		Offset(offset).
		Limit(pageSize).
		Order("knowledge_base_members.joined_at DESC").
		Find(&members).Error; err != nil {
		return nil, 0, err
	}

	return members, total, nil
}

// ListMembersByUser 列出用户加入的所有知识库
func (r *knowledgeBaseMemberRepository) ListMembersByUser(ctx context.Context, userID string) ([]*types.KnowledgeBaseMember, error) {
	var members []*types.KnowledgeBaseMember
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Preload("KnowledgeBase").
		Find(&members).Error; err != nil {
		return nil, err
	}
	return members, nil
}

// UpdateMemberRole 更新成员角色
func (r *knowledgeBaseMemberRepository) UpdateMemberRole(ctx context.Context, kbID string, userID string, role string) error {
	return r.db.WithContext(ctx).
		Model(&types.KnowledgeBaseMember{}).
		Where("knowledge_base_id = ? AND user_id = ?", kbID, userID).
		Update("role", role).Error
}

// DeleteMember soft-deletes the active membership. Historical soft-deleted rows
// for the same (kb, user) are hard-deleted first so the legacy unique constraint
// UNIQUE (knowledge_base_id, user_id, deleted_at) cannot reject the new deleted_at
// (leave → rejoin → leave). Safe with the partial unique index as well.
func (r *knowledgeBaseMemberRepository) DeleteMember(ctx context.Context, kbID string, userID string) error {
	if err := r.db.WithContext(ctx).Unscoped().
		Where("knowledge_base_id = ? AND user_id = ? AND deleted_at IS NOT NULL", kbID, userID).
		Delete(&types.KnowledgeBaseMember{}).Error; err != nil {
		return err
	}
	return r.db.WithContext(ctx).
		Where("knowledge_base_id = ? AND user_id = ?", kbID, userID).
		Delete(&types.KnowledgeBaseMember{}).Error
}

// CountMembersByKB 统计知识库成员数量（不包括已删除的）
func (r *knowledgeBaseMemberRepository) CountMembersByKB(ctx context.Context, kbID string) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&types.KnowledgeBaseMember{}).
		Where("knowledge_base_id = ?", kbID).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}
