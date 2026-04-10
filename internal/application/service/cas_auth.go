package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// casAuthService 实现 CAS 认证服务
type casAuthService struct {
	casClient     *CASClient
	userRepo      interfaces.UserRepository
	userService   interfaces.UserService
	tenantService interfaces.TenantService
	config        *config.CASConfig
}

// NewCASAuthService 创建 CAS 认证服务
func NewCASAuthService(
	casClient *CASClient,
	userRepo interfaces.UserRepository,
	userService interfaces.UserService,
	tenantService interfaces.TenantService,
	cfg *config.Config,
) interfaces.CASAuthService {
	var casConfig *config.CASConfig
	if cfg != nil {
		casConfig = cfg.CAS
	}
	return &casAuthService{
		casClient:     casClient,
		userRepo:      userRepo,
		userService:   userService,
		tenantService: tenantService,
		config:        casConfig,
	}
}

// ValidateCASSession 验证 CAS 会话（通过 _cas_sid 和 _cas_uid）
// referer 参数用于设置 Referer 头，CAS API 需要此头进行校验
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
				// 并发创建时，用户已存在，不需要删除租户（可能其他请求已绑定）
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

// AutoBindTenant 自动绑定或创建租户（CAS 平台租户 → WeKnora 租户）
func (s *casAuthService) AutoBindTenant(ctx context.Context, casUserInfo *types.CASUserInfo, user *types.User) (*types.Tenant, error) {
	// 步骤1：检查用户是否已有租户
	if user.TenantID > 0 {
		// 用户已有租户，查询并返回
		tenant, err := s.tenantService.GetTenantByID(ctx, user.TenantID)
		if err != nil {
			logger.Errorf(ctx, "Failed to get tenant by ID: %d, error: %v", user.TenantID, err)
			// 租户不存在或查询失败，需要创建新租户
		} else if tenant != nil {
			logger.Infof(ctx, "User already has tenant: %d", user.TenantID)
			return tenant, nil
		}
		// 如果租户不存在，继续创建新租户
	}

	// 步骤2：用户没有租户或租户不存在，创建新租户
	logger.Infof(ctx, "Creating new tenant for user: %s", user.ID)

	// 2.1 生成租户名称
	tenantName := fmt.Sprintf("%s的工作空间", casUserInfo.RealName)
	if tenantName == "的工作空间" {
		// 如果真实姓名为空，使用登录名
		tenantName = fmt.Sprintf("%s的工作空间", casUserInfo.LoginName)
	}
	if tenantName == "的工作空间" {
		// 如果登录名也为空，使用用户名
		tenantName = fmt.Sprintf("%s的工作空间", user.Username)
	}

	// 2.2 创建租户对象
	tenant := &types.Tenant{
		Name:        tenantName,
		Description: "默认工作空间",
		Status:      "active",
		Business:    "default",
	}

	// 2.3 创建租户（会自动生成 API Key 和设置默认配置）
	createdTenant, err := s.tenantService.CreateTenant(ctx, tenant)
	if err != nil {
		return nil, fmt.Errorf("failed to create tenant: %w", err)
	}

	// 步骤3：将用户绑定到租户
	user.TenantID = createdTenant.ID
	user.UpdatedAt = time.Now()
	if err := s.userRepo.UpdateUser(ctx, user); err != nil {
		logger.Errorf(ctx, "Failed to bind user to tenant: %v", err)
		// 绑定失败，删除刚创建的租户（避免遗留未使用的租户）
		if deleteErr := s.tenantService.DeleteTenant(ctx, createdTenant.ID); deleteErr != nil {
			logger.Errorf(ctx, "Failed to delete tenant %d after user binding failed: %v", createdTenant.ID, deleteErr)
			// 不返回错误，因为主要错误是用户绑定失败
		} else {
			logger.Infof(ctx, "Deleted tenant %d after user binding failed", createdTenant.ID)
		}
		return nil, fmt.Errorf("failed to bind user to tenant: %w", err)
	}

	logger.Infof(ctx, "Bound user %s to tenant %d", user.ID, createdTenant.ID)
	return createdTenant, nil
}
