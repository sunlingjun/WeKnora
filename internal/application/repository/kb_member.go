package repository

import (
	"context"
	"errors"

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

// DeleteMember 软删除成员
func (r *knowledgeBaseMemberRepository) DeleteMember(ctx context.Context, kbID string, userID string) error {
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
