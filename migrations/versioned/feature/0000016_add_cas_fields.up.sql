-- 添加 CAS 相关字段到 users 表
ALTER TABLE users ADD COLUMN IF NOT EXISTS cas_user_id VARCHAR(64);  -- CAS 用户 ID（对应 data.id）
ALTER TABLE users ADD COLUMN IF NOT EXISTS cas_login_name VARCHAR(100);  -- CAS 登录名（对应 data.loginName）
ALTER TABLE users ADD COLUMN IF NOT EXISTS cas_real_name VARCHAR(100);  -- CAS 真实姓名（对应 data.realName）
ALTER TABLE users ADD COLUMN IF NOT EXISTS cas_mobile_phone VARCHAR(20);  -- CAS 手机号（对应 data.mobilePhone）

-- 添加索引
CREATE INDEX IF NOT EXISTS idx_users_cas_user_id ON users(cas_user_id);
CREATE INDEX IF NOT EXISTS idx_users_cas_login_name ON users(cas_login_name);
