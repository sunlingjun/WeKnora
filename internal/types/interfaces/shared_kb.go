package interfaces

import (
	"context"
	"github.com/Tencent/WeKnora/internal/types"
)

// SharedKnowledgeBaseService 定义共享知识库服务接口
type SharedKnowledgeBaseService interface {
	// CreateSharedKnowledgeBase 创建共享知识库
	CreateSharedKnowledgeBase(ctx context.Context, kb *types.KnowledgeBase) (*types.KnowledgeBase, error)

	// ListSharedKnowledgeBases 列出共享知识库广场（支持搜索、分页）
	ListSharedKnowledgeBases(ctx context.Context, keyword string, page, pageSize int) ([]*types.KnowledgeBase, int64, error)

	// JoinSharedKnowledgeBase 加入共享知识库
	JoinSharedKnowledgeBase(ctx context.Context, kbID string) error

	// LeaveSharedKnowledgeBase 离开共享知识库
	LeaveSharedKnowledgeBase(ctx context.Context, kbID string) error

	// ListKnowledgeBaseMembers 列出知识库成员（支持 keyword 按 email/username/cas_real_name 搜索）
	ListKnowledgeBaseMembers(ctx context.Context, kbID string, keyword string, page, pageSize int) ([]*types.KnowledgeBaseMember, int64, error)

	// UpdateMemberRole 更新成员权限（仅创建者可操作）
	UpdateMemberRole(ctx context.Context, kbID string, userID string, role string) error

	// RemoveMember 移除成员（仅创建者可操作）
	RemoveMember(ctx context.Context, kbID string, userID string) error

	// CheckMemberPermission 检查成员权限
	CheckMemberPermission(ctx context.Context, kbID string, permission string) (bool, error)

	// ListUserKnowledgeBases 列出用户的知识库（个人 + 加入的共享知识库）
	ListUserKnowledgeBases(ctx context.Context, includeShared bool) ([]*types.KnowledgeBase, error)

	// GetMemberRoleByKBAndUser 获取用户在指定知识库中的角色
	GetMemberRoleByKBAndUser(ctx context.Context, kbID string, userID string) (string, error)
}

// KnowledgeBaseMemberRepository 定义知识库成员 Repository 接口
type KnowledgeBaseMemberRepository interface {
	// CreateMember 创建成员记录
	CreateMember(ctx context.Context, member *types.KnowledgeBaseMember) error

	// GetMemberByKBAndUser 根据知识库和用户查询成员
	GetMemberByKBAndUser(ctx context.Context, kbID string, userID string) (*types.KnowledgeBaseMember, error)

	// ListMembersByKB 列出知识库所有成员（支持分页，支持 keyword 按 email/username/cas_real_name 搜索）
	ListMembersByKB(ctx context.Context, kbID string, keyword string, page, pageSize int) ([]*types.KnowledgeBaseMember, int64, error)

	// ListMembersByUser 列出用户加入的所有知识库
	ListMembersByUser(ctx context.Context, userID string) ([]*types.KnowledgeBaseMember, error)

	// UpdateMemberRole 更新成员角色
	UpdateMemberRole(ctx context.Context, kbID string, userID string, role string) error

	// DeleteMember 软删除成员
	DeleteMember(ctx context.Context, kbID string, userID string) error

	// CountMembersByKB 统计知识库成员数量（不包括已删除的）
	CountMembersByKB(ctx context.Context, kbID string) (int64, error)
}
