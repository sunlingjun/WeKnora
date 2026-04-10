-- 删除索引
DROP INDEX IF EXISTS idx_kb_members_deleted_at;
DROP INDEX IF EXISTS idx_kb_members_role;
DROP INDEX IF EXISTS idx_kb_members_tenant_id;
DROP INDEX IF EXISTS idx_kb_members_user_id;
DROP INDEX IF EXISTS idx_kb_members_kb_id;

-- 删除表
DROP TABLE IF EXISTS knowledge_base_members;
