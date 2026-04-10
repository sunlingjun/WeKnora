-- 共享知识库成员表
CREATE TABLE IF NOT EXISTS knowledge_base_members (
    id VARCHAR(36) PRIMARY KEY DEFAULT gen_random_uuid(),
    knowledge_base_id VARCHAR(36) NOT NULL,  -- 关联 knowledge_bases.id
    user_id VARCHAR(36) NOT NULL,             -- 关联 users.id
    tenant_id INTEGER NOT NULL,              -- 关联 tenants.id（成员所属租户）
    role VARCHAR(32) NOT NULL DEFAULT 'viewer',  -- 'owner' | 'editor' | 'viewer'
    joined_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,  -- 加入时间
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,
    
    CONSTRAINT fk_kb_members_kb FOREIGN KEY (knowledge_base_id) REFERENCES knowledge_bases(id) ON DELETE CASCADE,
    CONSTRAINT fk_kb_members_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_kb_members_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    CONSTRAINT uk_kb_members_kb_user UNIQUE (knowledge_base_id, user_id, deleted_at)  -- 唯一约束（支持软删除）
);

-- 添加索引
CREATE INDEX IF NOT EXISTS idx_kb_members_kb_id ON knowledge_base_members(knowledge_base_id);
CREATE INDEX IF NOT EXISTS idx_kb_members_user_id ON knowledge_base_members(user_id);
CREATE INDEX IF NOT EXISTS idx_kb_members_tenant_id ON knowledge_base_members(tenant_id);
CREATE INDEX IF NOT EXISTS idx_kb_members_role ON knowledge_base_members(role);
CREATE INDEX IF NOT EXISTS idx_kb_members_deleted_at ON knowledge_base_members(deleted_at);
