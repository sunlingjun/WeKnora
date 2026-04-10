-- 添加共享知识库相关字段到 knowledge_bases 表
ALTER TABLE knowledge_bases ADD COLUMN IF NOT EXISTS visibility VARCHAR(32) DEFAULT 'private';  -- 'private' | 'shared'
ALTER TABLE knowledge_bases ADD COLUMN IF NOT EXISTS owner_id VARCHAR(36);  -- 创建者用户 ID（关联 users.id）
ALTER TABLE knowledge_bases ADD COLUMN IF NOT EXISTS shared_at TIMESTAMP WITH TIME ZONE;  -- 共享时间
ALTER TABLE knowledge_bases ADD COLUMN IF NOT EXISTS member_count INTEGER DEFAULT 0;  -- 成员数量（冗余字段，用于快速查询）

-- 添加索引
CREATE INDEX IF NOT EXISTS idx_knowledge_bases_visibility ON knowledge_bases(visibility);
CREATE INDEX IF NOT EXISTS idx_knowledge_bases_owner_id ON knowledge_bases(owner_id);
CREATE INDEX IF NOT EXISTS idx_knowledge_bases_shared_at ON knowledge_bases(shared_at);
