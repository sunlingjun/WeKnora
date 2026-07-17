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

	// AutoBindTenant 自动绑定或创建租户（CAS → WeKnora）。
	// 参数 user 是 AutoBindUser 返回的用户对象。
	// 行为：home 可用则复用；home 软删/缺失则从 owner/admin membership 恢复；
	// 非 NotFound 的查询错误直接失败（禁止误建）；真正无空间时才创建默认工作空间。
	AutoBindTenant(ctx context.Context, casUserInfo *types.CASUserInfo, user *types.User) (*types.Tenant, error)

	// 注意：不需要 CAS 会话管理方法
	// CAS 会话信息存储在浏览器 Cookie 中，由 CAS 服务器管理
	// WeKnora 使用 JWT Token 管理用户认证状态，存储在 auth_tokens 表中
}
