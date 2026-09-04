package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	apprepo "github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// casAuthService 实现 CAS 认证服务
type casAuthService struct {
	casClient     *CASClient
	userCenter    interfaces.UserCenterDirectory
	userRepo      interfaces.UserRepository
	userService   interfaces.UserService
	tenantService interfaces.TenantService
	memberService interfaces.TenantMemberService
	config        *config.CASConfig
}

// NewCASAuthService 创建 CAS 认证服务
func NewCASAuthService(
	casClient *CASClient,
	userCenter interfaces.UserCenterDirectory,
	userRepo interfaces.UserRepository,
	userService interfaces.UserService,
	tenantService interfaces.TenantService,
	memberService interfaces.TenantMemberService,
	cfg *config.Config,
) interfaces.CASAuthService {
	var casConfig *config.CASConfig
	if cfg != nil {
		casConfig = cfg.CAS
	}
	return &casAuthService{
		casClient:     casClient,
		userCenter:    userCenter,
		userRepo:      userRepo,
		userService:   userService,
		tenantService: tenantService,
		memberService: memberService,
		config:        casConfig,
	}
}

// ResolveCASUserFromCookies follows gateway priority:
// ticketCookie → ZNT+Archive; else casSid → UcTicket+Archive.
// If ticketCookie is non-empty and ZNT fails, does not fall through to sid.
func (s *casAuthService) ResolveCASUserFromCookies(ctx context.Context, ticketCookie, casSid string) (*types.CASUserInfo, error) {
	if s.userCenter == nil || !s.userCenter.HasBaseURL() {
		return nil, types.ErrCASUserCenterUnavailable
	}
	ticketCookie = strings.TrimSpace(ticketCookie)
	casSid = strings.TrimSpace(casSid)

	var boID string
	var err error
	switch {
	case ticketCookie != "":
		boID, err = s.userCenter.GetBoIDByZNTToken(ctx, ticketCookie)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", types.ErrCASTicketInvalid, err)
		}
	case casSid != "":
		boID, err = s.userCenter.GetBoIDByUcTicket(ctx, casSid)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", types.ErrCASTicketInvalid, err)
		}
	default:
		return nil, types.ErrCASCredentialsMissing
	}
	if !s.userCenter.Configured() {
		return nil, types.ErrCASUserCenterUnavailable
	}
	info, err := s.userCenter.GetUserArchive(ctx, boID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", types.ErrCASUserCenterUnavailable, err)
	}
	return info, nil
}

// ValidateCASSession 验证 CAS 会话（通过 _cas_sid 和 _cas_uid）。
// Cookie primary path is deprecated: prefer ResolveCASUserFromCookies.
// referer 参数用于设置 Referer 头，CAS API 需要此头进行校验。
func (s *casAuthService) ValidateCASSession(ctx context.Context, casSid, casUid string, referer string) (*types.CASUserInfo, error) {
	if casSid == "" || casUid == "" {
		return nil, fmt.Errorf("CAS session ID and UID are required")
	}

	// 调用 CAS 客户端验证会话
	casUserInfo, err := s.casClient.ValidateSession(ctx, casSid, casUid, referer)
	if err != nil {
		return nil, fmt.Errorf("failed to validate CAS session: %w", err)
	}

	return casUserInfo, nil
}

// AutoBindUser 自动绑定或创建用户（CAS 用户信息 → WeKnora 用户）
func (s *casAuthService) AutoBindUser(ctx context.Context, casUserInfo *types.CASUserInfo) (*types.User, error) {
	// 提前声明，避免 goto 跳过变量声明
	var user *types.User
	var err error
	var hashedPassword []byte
	var password string
	var username string
	var userEmail string
	var existingUser *types.User
	var createdTenant *types.Tenant
	var tenantName string
	var tenant *types.Tenant

	// 步骤1：用户存在性检查（按优先级查找）
	// 1.1 优先通过 cas_user_id 查找（最可靠，CAS 用户 ID 唯一）
	if casUserInfo.ID != "" {
		user, err = s.userRepo.GetUserByCASUserID(ctx, casUserInfo.ID)
		if err == nil && user != nil {
			logger.Infof(ctx, "Found user by cas_user_id: %s", user.ID)
			goto updateUser
		}
	}

	// 1.2 如果未找到，通过 email 查找（CAS 邮箱与 WeKnora 邮箱匹配）
	if casUserInfo.Email != "" {
		user, err = s.userRepo.GetUserByEmail(ctx, casUserInfo.Email)
		if err == nil && user != nil {
			logger.Infof(ctx, "Found user by email: %s", user.ID)
			goto updateUser
		}
	}

	// 步骤2：用户不存在，创建新用户
	logger.Infof(ctx, "User not found, creating new user for CAS user: %s", casUserInfo.ID)

	// 2.1 生成用户名（检查唯一性）
	username = casUserInfo.LoginName
	if username == "" {
		// 如果 loginName 为空，使用 email 前缀
		if casUserInfo.Email != "" {
			username = strings.Split(casUserInfo.Email, "@")[0]
		} else {
			// 如果 email 也为空，使用 CAS ID
			username = fmt.Sprintf("cas_%s", casUserInfo.ID)
		}
	}

	// 2.2 检查用户名是否已存在（处理冲突）
	existingUser, _ = s.userRepo.GetUserByUsername(ctx, username)
	if existingUser != nil {
		// 用户名冲突，添加后缀
		if len(casUserInfo.ID) >= 8 {
			username = fmt.Sprintf("%s_%s", username, casUserInfo.ID[:8])
		} else {
			username = fmt.Sprintf("%s_%s", username, casUserInfo.ID)
		}
		logger.Warnf(ctx, "Username conflict, using: %s", username)
	}

	// 2.3 检查邮箱是否已存在（理论上不应该发生，因为已经通过 email 查找过）
	if casUserInfo.Email != "" {
		existingUser, _ = s.userRepo.GetUserByEmail(ctx, casUserInfo.Email)
		if existingUser != nil {
			// 邮箱已存在，说明可能是并发请求，返回现有用户
			logger.Warnf(ctx, "Email already exists, returning existing user: %s", existingUser.ID)
			return existingUser, nil
		}
	}

	// 2.4 生成密码并哈希（CAS 用户不需要密码，但数据库字段要求非空）
	// 密码默认取手机号后四位，如果手机号为空或格式不对，使用默认密码
	password = "1234" // 默认密码
	if casUserInfo.MobilePhone != "" {
		// 提取手机号后四位（处理脱敏格式，如 "166****8186"）
		mobilePhone := casUserInfo.MobilePhone
		// 移除所有非数字字符
		digits := ""
		for _, r := range mobilePhone {
			if r >= '0' && r <= '9' {
				digits += string(r)
			}
		}
		// 取最后四位
		if len(digits) >= 4 {
			password = digits[len(digits)-4:]
		}
	}
	hashedPassword, err = bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// 2.5 先创建租户（因为用户需要租户 ID，外键约束要求）
	// 注意：这里先创建租户，然后再创建用户并绑定租户
	tenantName = fmt.Sprintf("%s的工作空间", casUserInfo.RealName)
	if tenantName == "的工作空间" {
		// 如果真实姓名为空，使用登录名
		tenantName = fmt.Sprintf("%s的工作空间", casUserInfo.LoginName)
	}
	if tenantName == "的工作空间" {
		// 如果登录名也为空，使用用户名
		tenantName = fmt.Sprintf("%s的工作空间", username)
	}

	tenant = &types.Tenant{
		Name:        tenantName,
		Description: "默认工作空间",
		Status:      "active",
		Business:    "default",
	}

	createdTenant, err = s.tenantService.CreateTenant(ctx, tenant)
	if err != nil {
		return nil, fmt.Errorf("failed to create tenant: %w", err)
	}
	logger.Infof(ctx, "Created tenant for new user: %d", createdTenant.ID)

	// 2.6 创建用户对象（绑定到刚创建的租户）
	// 处理 Email：如果 CAS Email 为空，使用 username 生成邮箱地址
	userEmail = casUserInfo.Email
	if userEmail == "" {
		// Email 为空时，使用 username 生成邮箱地址
		userEmail = fmt.Sprintf("%s@nxin.local", username)
		logger.Infof(ctx, "CAS Email is empty, using generated email: %s", userEmail)
	}

	user = &types.User{
		ID:             uuid.New().String(),
		Username:       username,
		Email:          userEmail,
		PasswordHash:   string(hashedPassword),
		Avatar:         "",               // 使用系统默认头像（空字符串）
		TenantID:       createdTenant.ID, // 设置租户 ID，避免外键约束错误
		IsActive:       true,
		CASUserID:      casUserInfo.ID,
		CASLoginName:   casUserInfo.LoginName,
		CASRealName:    casUserInfo.RealName,
		CASMobilePhone: casUserInfo.MobilePhone,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	// 2.7 保存用户到数据库
	if err := s.userRepo.CreateUser(ctx, user); err != nil {
		// 处理唯一约束冲突（并发创建）
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "UNIQUE constraint") {
			// 重新查找用户（按优先级：cas_user_id → email）
			if casUserInfo.ID != "" {
				user, err = s.userRepo.GetUserByCASUserID(ctx, casUserInfo.ID)
			}
			if user == nil && userEmail != "" {
				user, err = s.userRepo.GetUserByEmail(ctx, userEmail)
			}
			if user != nil {
				logger.Warnf(ctx, "Concurrent user creation detected, returning existing user: %s", user.ID)
				// 本次新建的租户尚未绑定到任何人，必须清理，否则留下无主空壳。
				if createdTenant != nil && createdTenant.ID > 0 {
					if deleteErr := s.tenantService.DeleteTenant(ctx, createdTenant.ID); deleteErr != nil {
						logger.Errorf(ctx, "Failed to delete orphan tenant %d after concurrent user create: %v",
							createdTenant.ID, deleteErr)
					} else {
						logger.Infof(ctx, "Deleted orphan tenant %d after concurrent user create", createdTenant.ID)
					}
				}
				return user, nil
			}
		}

		// 创建用户失败，删除之前创建的租户（避免遗留未使用的租户）
		// 注意：删除租户失败不影响错误返回，但会记录日志
		if createdTenant != nil && createdTenant.ID > 0 {
			if deleteErr := s.tenantService.DeleteTenant(ctx, createdTenant.ID); deleteErr != nil {
				logger.Errorf(ctx, "Failed to delete tenant %d after user creation failed: %v", createdTenant.ID, deleteErr)
				// 不返回错误，因为主要错误是用户创建失败
			} else {
				logger.Infof(ctx, "Deleted tenant %d after user creation failed", createdTenant.ID)
			}
		}

		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	logger.Infof(ctx, "Created new user: %s (CAS ID: %s)", user.ID, casUserInfo.ID)
	return user, nil

updateUser:
	// 步骤3：用户已存在，更新 CAS 相关字段
	logger.Infof(ctx, "Updating existing user: %s", user.ID)

	// 3.1 更新 CAS 相关字段（如果为空或需要更新）
	needUpdate := false
	if user.CASUserID == "" && casUserInfo.ID != "" {
		user.CASUserID = casUserInfo.ID
		needUpdate = true
	}
	if user.CASLoginName == "" && casUserInfo.LoginName != "" {
		user.CASLoginName = casUserInfo.LoginName
		needUpdate = true
	}
	if user.CASRealName != casUserInfo.RealName {
		user.CASRealName = casUserInfo.RealName
		needUpdate = true
	}
	if user.CASMobilePhone != casUserInfo.MobilePhone {
		user.CASMobilePhone = casUserInfo.MobilePhone
		needUpdate = true
	}

	// 3.2 确保用户是激活状态
	if !user.IsActive {
		user.IsActive = true
		needUpdate = true
	}

	// 3.3 如果有更新，保存到数据库
	if needUpdate {
		user.UpdatedAt = time.Now()
		if err := s.userRepo.UpdateUser(ctx, user); err != nil {
			logger.Errorf(ctx, "Failed to update user: %v", err)
			// 不影响主流程，继续返回用户
		} else {
			logger.Infof(ctx, "Updated user CAS fields: %s", user.ID)
		}
	}

	return user, nil
}

// AutoBindTenant 自动绑定或创建租户（CAS → WeKnora）。
//
// 决策顺序：
//  1. home (users.tenant_id) 可用 → 幂等 EnsureOwner 后返回
//  2. home 不存在（软删/缺失）→ 从 owner/admin membership 恢复，禁止误建
//  3. home 查询出现非 NotFound 错误（解密失败、DB 错）→ 直接失败，禁止新建
//  4. 无任何可恢复空间 → 创建默认工作空间（真正的首次用户）
func (s *casAuthService) AutoBindTenant(ctx context.Context, casUserInfo *types.CASUserInfo, user *types.User) (*types.Tenant, error) {
	if user == nil {
		return nil, fmt.Errorf("user is required")
	}

	if user.TenantID > 0 {
		tenant, err := s.tenantService.GetTenantByID(ctx, user.TenantID)
		if err == nil && tenant != nil {
			logger.Infof(ctx, "User already has tenant: %d", user.TenantID)
			s.ensureOwnerBestEffort(ctx, user.ID, tenant.ID)
			return tenant, nil
		}
		if err != nil && !errors.Is(err, apprepo.ErrTenantNotFound) {
			logger.Errorf(ctx, "Failed to load home tenant %d (non-not-found): %v", user.TenantID, err)
			return nil, fmt.Errorf("load home tenant %d: %w", user.TenantID, err)
		}
		logger.Warnf(ctx, "Home tenant %d unavailable for user %s, attempting membership recovery",
			user.TenantID, user.ID)
	}

	if recovered, err := s.recoverTenantFromMemberships(ctx, user); err != nil {
		return nil, err
	} else if recovered != nil {
		return recovered, nil
	}

	return s.createDefaultTenantForCASUser(ctx, casUserInfo, user)
}

// recoverTenantFromMemberships picks a surviving owner/admin workspace and
// rewrites users.tenant_id. Viewer/contributor-only memberships are ignored
// so an invited collaborator is not treated as owning that tenant's home.
func (s *casAuthService) recoverTenantFromMemberships(ctx context.Context, user *types.User) (*types.Tenant, error) {
	if s.memberService == nil {
		return nil, nil
	}
	members, err := s.memberService.ListByUser(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("list memberships for recovery: %w", err)
	}

	candidateIDs := make([]uint64, 0, len(members))
	for _, m := range members {
		if m == nil || m.Status != types.TenantMemberStatusActive {
			continue
		}
		if m.Role != types.TenantRoleOwner && m.Role != types.TenantRoleAdmin {
			continue
		}
		candidateIDs = append(candidateIDs, m.TenantID)
	}
	if len(candidateIDs) == 0 {
		return nil, nil
	}

	tenants, err := s.tenantService.GetTenantsByIDs(ctx, candidateIDs)
	if err != nil {
		return nil, fmt.Errorf("load candidate tenants for recovery: %w", err)
	}

	var best *types.Tenant
	for _, id := range candidateIDs {
		t := tenants[id]
		if t == nil {
			continue
		}
		if best == nil {
			best = t
			continue
		}
		bestHasData := best.StorageUsed > 0
		candHasData := t.StorageUsed > 0
		switch {
		case candHasData && !bestHasData:
			best = t
		case candHasData == bestHasData &&
			(t.CreatedAt.Before(best.CreatedAt) ||
				(t.CreatedAt.Equal(best.CreatedAt) && t.ID < best.ID)):
			best = t
		}
	}
	if best == nil {
		return nil, nil
	}

	if err := s.bindUserHome(ctx, user, best.ID); err != nil {
		return nil, err
	}
	s.ensureOwnerBestEffort(ctx, user.ID, best.ID)
	logger.Infof(ctx, "Recovered home tenant %d for user %s from memberships", best.ID, user.ID)
	return best, nil
}

func (s *casAuthService) createDefaultTenantForCASUser(
	ctx context.Context,
	casUserInfo *types.CASUserInfo,
	user *types.User,
) (*types.Tenant, error) {
	logger.Infof(ctx, "Creating new tenant for user: %s", user.ID)

	tenantName := "的工作空间"
	if casUserInfo != nil {
		tenantName = fmt.Sprintf("%s的工作空间", casUserInfo.RealName)
		if tenantName == "的工作空间" {
			tenantName = fmt.Sprintf("%s的工作空间", casUserInfo.LoginName)
		}
	}
	if tenantName == "的工作空间" {
		tenantName = fmt.Sprintf("%s的工作空间", user.Username)
	}

	createdTenant, err := s.tenantService.CreateTenant(ctx, &types.Tenant{
		Name:        tenantName,
		Description: "默认工作空间",
		Status:      "active",
		Business:    "default",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create tenant: %w", err)
	}

	if err := s.bindUserHome(ctx, user, createdTenant.ID); err != nil {
		if deleteErr := s.tenantService.DeleteTenant(ctx, createdTenant.ID); deleteErr != nil {
			logger.Errorf(ctx, "Failed to delete tenant %d after user binding failed: %v",
				createdTenant.ID, deleteErr)
		} else {
			logger.Infof(ctx, "Deleted tenant %d after user binding failed", createdTenant.ID)
		}
		return nil, fmt.Errorf("failed to bind user to tenant: %w", err)
	}

	s.ensureOwnerBestEffort(ctx, user.ID, createdTenant.ID)
	logger.Infof(ctx, "Bound user %s to tenant %d", user.ID, createdTenant.ID)
	return createdTenant, nil
}

func (s *casAuthService) bindUserHome(ctx context.Context, user *types.User, tenantID uint64) error {
	user.TenantID = tenantID
	user.UpdatedAt = time.Now()
	if err := s.userRepo.UpdateUser(ctx, user); err != nil {
		logger.Errorf(ctx, "Failed to bind user to tenant: %v", err)
		return err
	}
	return nil
}

func (s *casAuthService) ensureOwnerBestEffort(ctx context.Context, userID string, tenantID uint64) {
	if s.memberService == nil {
		return
	}
	if _, err := s.memberService.EnsureOwner(ctx, userID, tenantID); err != nil {
		logger.Warnf(ctx, "EnsureOwner failed for user=%s tenant=%d: %v", userID, tenantID, err)
	}
}
