-- 密码哈希值验证和排查脚本
-- 用于检查导入后的用户密码哈希值是否正确

-- ============================================================================
-- 1. 检查密码哈希值长度
-- ============================================================================
SELECT 
    '密码哈希长度检查' as check_type,
    COUNT(*) as total_users,
    COUNT(CASE WHEN LENGTH(password_hash) = 60 THEN 1 END) as valid_length_60,
    COUNT(CASE WHEN LENGTH(password_hash) < 60 THEN 1 END) as too_short,
    COUNT(CASE WHEN LENGTH(password_hash) > 60 THEN 1 END) as too_long,
    COUNT(CASE WHEN password_hash IS NULL THEN 1 END) as is_null
FROM users;

-- ============================================================================
-- 2. 检查密码哈希值格式
-- ============================================================================
SELECT 
    '密码哈希格式检查' as check_type,
    COUNT(*) as total_users,
    COUNT(CASE WHEN password_hash LIKE '$2a$10$%' THEN 1 END) as format_2a_10,
    COUNT(CASE WHEN password_hash LIKE '$2b$10$%' THEN 1 END) as format_2b_10,
    COUNT(CASE WHEN password_hash LIKE '$2y$10$%' THEN 1 END) as format_2y_10,
    COUNT(CASE WHEN password_hash NOT LIKE '$2%$10$%' THEN 1 END) as invalid_format
FROM users
WHERE password_hash IS NOT NULL;

-- ============================================================================
-- 3. 检查是否有空格或换行符
-- ============================================================================
SELECT 
    '空格和换行符检查' as check_type,
    COUNT(*) as total_users,
    COUNT(CASE WHEN password_hash != TRIM(password_hash) THEN 1 END) as has_whitespace,
    COUNT(CASE WHEN password_hash ~ E'[\\n\\r\\t]' THEN 1 END) as has_newline,
    COUNT(CASE WHEN password_hash !~ '^[$./A-Za-z0-9]{60}$' THEN 1 END) as has_invalid_chars
FROM users
WHERE password_hash IS NOT NULL;

-- ============================================================================
-- 4. 列出所有有问题的用户
-- ============================================================================
SELECT 
    id,
    email,
    username,
    LENGTH(password_hash) as hash_length,
    CASE 
        WHEN LENGTH(password_hash) != 60 THEN '❌ 长度不正确'
        WHEN password_hash NOT LIKE '$2%$10$%' THEN '❌ 格式不正确'
        WHEN password_hash != TRIM(password_hash) THEN '❌ 包含空格'
        WHEN password_hash ~ E'[\\n\\r\\t]' THEN '❌ 包含换行符'
        WHEN password_hash !~ '^[$./A-Za-z0-9]{60}$' THEN '❌ 包含无效字符'
        ELSE '✅ 正常'
    END as status,
    SUBSTRING(password_hash, 1, 7) as hash_prefix,
    password_hash
FROM users
WHERE password_hash IS NOT NULL
  AND (
    LENGTH(password_hash) != 60
    OR password_hash NOT LIKE '$2%$10$%'
    OR password_hash != TRIM(password_hash)
    OR password_hash ~ E'[\\n\\r\\t]'
    OR password_hash !~ '^[$./A-Za-z0-9]{60}$'
  )
ORDER BY status, email;

-- ============================================================================
-- 5. 修复脚本（清理空格和换行符）
-- ============================================================================
-- 注意：执行前请先备份数据！
-- 
-- UPDATE users 
-- SET password_hash = REGEXP_REPLACE(
--     TRIM(password_hash),
--     E'[^$./A-Za-z0-9]',
--     '',
--     'g'
-- )
-- WHERE password_hash IS NOT NULL
--   AND (
--     password_hash != TRIM(password_hash)
--     OR password_hash ~ E'[\\n\\r\\t]'
--     OR password_hash !~ '^[$./A-Za-z0-9]{60}$'
--   );

-- ============================================================================
-- 6. 验证修复后的数据
-- ============================================================================
-- SELECT 
--     COUNT(*) as total,
--     COUNT(CASE WHEN LENGTH(password_hash) = 60 THEN 1 END) as valid_length,
--     COUNT(CASE WHEN password_hash LIKE '$2%$10$%' THEN 1 END) as valid_format,
--     COUNT(CASE WHEN password_hash = TRIM(password_hash) THEN 1 END) as no_whitespace
-- FROM users
-- WHERE password_hash IS NOT NULL;
