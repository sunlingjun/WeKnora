-- 删除索引
DROP INDEX IF EXISTS idx_users_cas_login_name;
DROP INDEX IF EXISTS idx_users_cas_user_id;

-- 删除字段
ALTER TABLE users DROP COLUMN IF EXISTS cas_mobile_phone;
ALTER TABLE users DROP COLUMN IF EXISTS cas_real_name;
ALTER TABLE users DROP COLUMN IF EXISTS cas_login_name;
ALTER TABLE users DROP COLUMN IF EXISTS cas_user_id;
