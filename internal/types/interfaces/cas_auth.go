package interfaces

import (
	"context"
	"github.com/Tencent/WeKnora/internal/types"
)

// CASAuthService 定义 CAS 认证服务接口
type CASAuthService interface {
	// ValidateCASSession 验证 CAS 会话（通过 _cas_sid 和 _cas_uid）
	// referer 参数用于设置 Referer 头，CAS API 需要此头进行校验
	ValidateCASSession(ctx context.Context, casSid, casUid string, referer string) (*types.CASUserInfo, error)

	// AutoBindUser 自动绑定或创建用户（CAS 用户信息 → WeKnora 用户）
	AutoBindUser(ctx context.Context, casUserInfo *types.CASUserInfo) (*types.User, error)

	// AutoBindTenant 自动绑定或创建租户（CAS 平台租户 → WeKnora 租户）
	// 参数 user 是 AutoBindUser 返回的用户对象，用于检查用户是否已有租户
	AutoBindTenant(ctx context.Context, casUserInfo *types.CASUserInfo, user *types.User) (*types.Tenant, error)

	// 注意：不需要 CAS 会话管理方法
	// CAS 会话信息存储在浏览器 Cookie 中，由 CAS 服务器管理
	// WeKnora 使用 JWT Token 管理用户认证状态，存储在 auth_tokens 表中
}
