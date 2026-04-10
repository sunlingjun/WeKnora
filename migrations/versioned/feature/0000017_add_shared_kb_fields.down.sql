-- 删除索引
DROP INDEX IF EXISTS idx_knowledge_bases_shared_at;
DROP INDEX IF EXISTS idx_knowledge_bases_owner_id;
DROP INDEX IF EXISTS idx_knowledge_bases_visibility;

-- 删除字段
ALTER TABLE knowledge_bases DROP COLUMN IF EXISTS member_count;
ALTER TABLE knowledge_bases DROP COLUMN IF EXISTS shared_at;
ALTER TABLE knowledge_bases DROP COLUMN IF EXISTS owner_id;
ALTER TABLE knowledge_bases DROP COLUMN IF EXISTS visibility;
