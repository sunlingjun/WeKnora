-- ============================================================================
-- 用户数据导入检查脚本
-- 用于排查测试环境用户登录失败问题
-- ============================================================================

-- 1. 检查用户是否存在
SELECT 
    '用户检查' as check_type,
    CASE 
        WHEN COUNT(*) = 0 THEN '❌ 用户不存在'
        WHEN COUNT(*) > 0 AND COUNT(CASE WHEN deleted_at IS NULL THEN 1 END) = 0 THEN '⚠️ 用户存在但已被删除'
        ELSE '✅ 用户存在且活跃'
    END as status,
    COUNT(*) as total_count,
    COUNT(CASE WHEN deleted_at IS NULL THEN 1 END) as active_count,
    COUNT(CASE WHEN deleted_at IS NOT NULL THEN 1 END) as deleted_count
FROM users
WHERE email = 'slj0713@163.com';

-- 2. 显示用户详细信息
SELECT 
    id,
    email,
    username,
    tenant_id,
    is_active,
    deleted_at IS NOT NULL as is_deleted,
    LENGTH(password_hash) as hash_length,
    SUBSTRING(password_hash, 1, 7) as hash_prefix,
    CASE 
        WHEN LENGTH(password_hash) = 60 THEN '✅'
        ELSE '❌'
    END as hash_length_ok,
    CASE 
        WHEN password_hash LIKE '$2a$10$%' THEN '✅'
        ELSE '❌'
    END as hash_format_ok,
    created_at,
    updated_at
FROM users
WHERE email = 'slj0713@163.com';

-- 3. 检查租户是否存在
SELECT 
    '租户检查' as check_type,
    CASE 
        WHEN COUNT(*) = 0 THEN '❌ 租户不存在'
        ELSE '✅ 租户存在'
    END as status,
    id,
    "name",
    status,
    created_at
FROM tenants
WHERE id = 10000;

-- 4. 检查用户-租户关联
SELECT 
    u.id as user_id,
    u.email,
    u.username,
    u.tenant_id,
    t.id as tenant_exists,
    t."name" as tenant_name,
    CASE 
        WHEN t.id IS NULL THEN '❌ 租户不存在'
        WHEN u.deleted_at IS NOT NULL THEN '⚠️ 用户已删除'
        WHEN u.tenant_id != 10000 THEN '⚠️ 租户ID不匹配'
        ELSE '✅ 正常'
    END as status
FROM users u
LEFT JOIN tenants t ON u.tenant_id = t.id
WHERE u.email = 'slj0713@163.com';

-- 5. 列出所有用户（用于对比）
SELECT 
    id,
    email,
    username,
    tenant_id,
    deleted_at IS NOT NULL as is_deleted,
    created_at
FROM users
ORDER BY created_at DESC
LIMIT 10;
