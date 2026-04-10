-- ============================================================================
-- 快速修复：导入用户数据到测试环境
-- 使用方法：psql -h test_host -U username -d test_db -f scripts/import_user_fix.sql
-- ============================================================================

BEGIN;

-- ============================================================================
-- 步骤 1：检查并导入租户（如果不存在）
-- ============================================================================
INSERT INTO public.tenants (
    id,
    "name",
    description,
    api_key,
    retriever_engines,
    status,
    business,
    storage_quota,
    storage_used,
    agent_config,
    created_at,
    updated_at,
    deleted_at,
    context_config,
    conversation_config,
    web_search_config
) VALUES (
    10000,
    'slj001''s Workspace',
    'Default workspace',
    'sk-02zhrKUk51twe_PzGcaTXP9tktawM5T2EfnN0st4iZkxYZty',
    '{"engines": []}'::jsonb,
    'active',
    '',
    1073741824,
    7988309,
    NULL,
    '2026-01-19 13:47:16.963303+08'::timestamptz,
    '2026-01-22 11:12:43.035256+08'::timestamptz,
    NULL,
    '{"max_tokens": 0, "summarize_threshold": 0, "compression_strategy": "", "recent_message_count": 0}'::jsonb,
    NULL,
    '{"api_key": "", "provider": "duckduckgo", "blacklist": [], "max_results": 5, "include_date": true, "compression_method": "llm_summary"}'::jsonb
)
ON CONFLICT (id) DO NOTHING;

-- ============================================================================
-- 步骤 2：导入或更新用户数据
-- ============================================================================
INSERT INTO public.users (
    id,
    username,
    email,
    password_hash,
    avatar,
    tenant_id,
    is_active,
    created_at,
    updated_at,
    deleted_at,
    can_access_all_tenants,
    cas_user_id,
    cas_login_name,
    cas_real_name,
    cas_mobile_phone
) VALUES (
    '67d7b5fe-2294-4440-87ee-f0fa246364b4',
    'slj001',
    'slj0713@163.com',
    '$2a$10$aYn/vJaQFegjT8YOlkBdcONbqIPKHPenhHfSnJi2sZnB5gNjoAgcS',
    '',
    10000,
    true,
    '2026-01-19 13:47:16.982086+08'::timestamptz,
    '2026-01-19 13:47:16.982086+08'::timestamptz,
    NULL,  -- 确保 deleted_at 为 NULL
    false,
    NULL,
    NULL,
    NULL,
    NULL
)
ON CONFLICT (id) DO UPDATE SET
    email = EXCLUDED.email,
    password_hash = EXCLUDED.password_hash,
    tenant_id = EXCLUDED.tenant_id,
    deleted_at = NULL,  -- 恢复被软删除的用户
    is_active = true,
    updated_at = NOW();

-- ============================================================================
-- 步骤 3：验证导入结果
-- ============================================================================
SELECT 
    '导入验证' as check_type,
    u.id,
    u.email,
    u.username,
    u.tenant_id,
    t."name" as tenant_name,
    u.deleted_at IS NULL as is_active,
    LENGTH(u.password_hash) as hash_length,
    CASE 
        WHEN LENGTH(u.password_hash) = 60 THEN '✅'
        ELSE '❌'
    END as hash_length_ok,
    CASE 
        WHEN u.password_hash LIKE '$2a$10$%' THEN '✅'
        ELSE '❌'
    END as hash_format_ok,
    CASE 
        WHEN u.deleted_at IS NULL AND t.id IS NOT NULL THEN '✅ 可以登录'
        WHEN u.deleted_at IS NOT NULL THEN '⚠️ 用户已删除'
        WHEN t.id IS NULL THEN '❌ 租户不存在'
        ELSE '❌ 未知错误'
    END as status
FROM users u
LEFT JOIN tenants t ON u.tenant_id = t.id
WHERE u.email = 'slj0713@163.com';

COMMIT;

-- ============================================================================
-- 步骤 4：如果验证失败，显示详细信息
-- ============================================================================
-- 如果上面的验证显示有问题，运行以下查询查看详细信息：

-- SELECT * FROM users WHERE email = 'slj0713@163.com';
-- SELECT * FROM tenants WHERE id = 10000;
