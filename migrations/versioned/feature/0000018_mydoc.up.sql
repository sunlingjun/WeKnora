-- 我的文档（MyDoc）账号级个人资产中枢
-- 放置于 migrations/versioned/feature：不进官方 goose/versioned 序号，需环境手工执行。
-- 是否向上游贡献未定；合入正式 versioned 前勿改用占用官方号段的文件名。

CREATE TABLE IF NOT EXISTS mydoc_nodes (
  id            VARCHAR(36) PRIMARY KEY,
  owner_user_id VARCHAR(64) NOT NULL,
  parent_id     VARCHAR(36) NULL REFERENCES mydoc_nodes(id),
  name          VARCHAR(512) NOT NULL,
  type          VARCHAR(16) NOT NULL, -- folder | file
  archived      BOOLEAN NOT NULL DEFAULT FALSE,
  archived_at   TIMESTAMPTZ NULL,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_mydoc_nodes_owner_parent
  ON mydoc_nodes(owner_user_id, parent_id) WHERE archived = FALSE;

CREATE INDEX IF NOT EXISTS idx_mydoc_nodes_owner_archived
  ON mydoc_nodes(owner_user_id, archived);

CREATE TABLE IF NOT EXISTS mydoc_files (
  node_id         VARCHAR(36) PRIMARY KEY REFERENCES mydoc_nodes(id) ON DELETE CASCADE,
  kind            VARCHAR(32) NOT NULL, -- upload | ai_artifact | qa_snapshot
  storage_key     TEXT NOT NULL,
  mime            VARCHAR(255) NOT NULL DEFAULT '',
  size            BIGINT NOT NULL DEFAULT 0,
  content_hash    VARCHAR(128) NOT NULL DEFAULT '',
  version         INT NOT NULL DEFAULT 1,
  source_agent    VARCHAR(128) NOT NULL DEFAULT '',
  source_session  VARCHAR(128) NOT NULL DEFAULT '',
  extra           JSONB NOT NULL DEFAULT '{}',
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS mydoc_shares (
  id              VARCHAR(36) PRIMARY KEY,
  doc_id          VARCHAR(36) NOT NULL REFERENCES mydoc_nodes(id) ON DELETE CASCADE,
  grantee_user_id VARCHAR(64) NOT NULL,
  role            VARCHAR(16) NOT NULL DEFAULT 'viewer',
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(doc_id, grantee_user_id)
);

CREATE INDEX IF NOT EXISTS idx_mydoc_shares_grantee ON mydoc_shares(grantee_user_id);

CREATE TABLE IF NOT EXISTS mydoc_tickets (
  id          VARCHAR(36) PRIMARY KEY,
  doc_id      VARCHAR(36) NOT NULL REFERENCES mydoc_nodes(id) ON DELETE CASCADE,
  purpose     VARCHAR(16) NOT NULL, -- preview | download
  token_hash  VARCHAR(128) NOT NULL,
  expires_at  TIMESTAMPTZ NOT NULL,
  revoked     BOOLEAN NOT NULL DEFAULT FALSE,
  created_by  VARCHAR(64) NOT NULL,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_mydoc_tickets_doc ON mydoc_tickets(doc_id);

-- P1 挂库派生索引占位；P0 可不使用
CREATE TABLE IF NOT EXISTS mydoc_mounts (
  id                    VARCHAR(36) PRIMARY KEY,
  doc_id                VARCHAR(36) NOT NULL REFERENCES mydoc_nodes(id) ON DELETE CASCADE,
  tenant_id             BIGINT NOT NULL,
  knowledge_base_id     VARCHAR(36) NOT NULL,
  derived_knowledge_id  VARCHAR(36) NOT NULL DEFAULT '',
  index_status          VARCHAR(16) NOT NULL DEFAULT 'pending',
  created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(doc_id, tenant_id, knowledge_base_id)
);
