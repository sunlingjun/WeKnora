-- 修复后的 SQL 导入语句
-- 修复了 tenants 表中的单引号转义问题

-- ============================================================================
-- 1. 修复 tenants 表 INSERT 语句
-- ============================================================================
-- 原 SQL 问题：'slj001''s Workspace' 中的单引号转义不正确
-- 修复方法：使用两个单引号 '' 转义，或者使用 E'...' 语法

-- 方法1：使用两个单引号转义（推荐）
INSERT INTO public.tenants (
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
    'slj001''s Workspace',  -- 注意：使用两个单引号 '' 转义单引号
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
);

-- ============================================================================
-- 2. users 表 INSERT 语句（已验证，格式正确）
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
    '$2a$10$aYn/vJaQFegjT8YOlkBdcONbqIPKHPenhHfSnJi2sZnB5gNjoAgcS',  -- ✅ 密码哈希值正确（60字符）
    '',
    10000,
    true,
    '2026-01-19 13:47:16.982086+08'::timestamptz,
    '2026-01-19 13:47:16.982086+08'::timestamptz,
    NULL,
    false,
    NULL,
    NULL,
    NULL,
    NULL
);

-- ============================================================================
-- 3. 验证导入的数据
-- ============================================================================
-- 检查密码哈希值
SELECT 
    id,
    email,
    username,
    LENGTH(password_hash) as hash_length,
    SUBSTRING(password_hash, 1, 7) as hash_prefix,
    CASE 
        WHEN LENGTH(password_hash) = 60 THEN '✅ 长度正确'
        ELSE '❌ 长度错误'
    END as length_status,
    CASE 
        WHEN password_hash LIKE '$2a$10$%' THEN '✅ 格式正确'
        ELSE '❌ 格式错误'
    END as format_status
FROM public.users
WHERE email = 'slj0713@163.com';

-- 检查 tenants 数据
SELECT 
    id,
    "name",
    api_key,
    status
FROM public.tenants
WHERE "name" LIKE '%slj001%';
