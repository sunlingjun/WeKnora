# WeKnora-slj 升级内容清单（`v0.3.6` → `v0.6.3`）

> **文档创建**：2026-07-09  
> **配套方案**：[WeKnora-slj v0.3.6 → v0.6.3 升级方案](./2026-07-09-WeKnora-slj-v0.3.6-to-v0.6.3升级方案.md)  
> **基线 Tag → 目标 Tag**：`v0.3.6` → `v0.6.3`  
> **用途**：升级执行过程中的勾选跟踪表（保护项 / 上游吸收 / 阶段 / 回归）

---

## 一、升级范围总览

| 维度 | 基线 | 目标 | 状态 |
|------|------|------|------|
| 开源 Tag | `v0.3.6` | `v0.6.3` | ☐ |
| Go 工具链 | 1.24.11 | 1.26 | ☐ |
| 上游提交差量 | 落后 ~85 commit | 0 | ☐ |
| 数据库迁移 | 000000–000032 + feature/ | 000000–000059 + feature/ | ☐ |
| 本地定制提交保留 | 7 个 NXIN 提交 | rebase 叠于 Tag `v0.6.3` 上 | ☐ |

---

## 二、上游能力吸收清单（跟随合并）

> 以下取自首版方案与 CHANGELOG，合并时**直接采纳上游**，不与本地定制冲突的部分。

### 2.1 Tag `v0.4.0`

| # | 能力 | 状态 |
|---|------|------|
| U1 | VectorStore 实体与 CRUD API | ☐ |
| U2 | 附件处理 / 多模态查询增强 | ☐ |
| U3 | Azure OpenAI / 阿里云 OSS | ☐ |
| U4 | Notion 连接器 | ☐ |
| U5 | IM 附件与引用上下文增强 | ☐ |

### 2.2 Tag `v0.5.2`（含 `v0.5.0`、`v0.5.1`）

| # | 能力 | 状态 |
|---|------|------|
| U6 | RBAC / 租户成员管理 | ☐ |
| U7 | 审计日志 / 凭证 AES 加密 | ☐ |
| U8 | 每 KB 所有权（per-KB ownership） | ☐ |
| U9 | 多向量库 fan-out 检索 | ☐ |
| U10 | 工作区模型调整 | ☐ |

### 2.3 Tag `v0.6.3`（含 `v0.6.0`–`v0.6.2`）

| # | 能力 | 状态 |
|---|------|------|
| U11 | Wiki / 文件夹层级导航 | ☐ |
| U12 | `process_config` 按上传配置 / 文档 reparse | ☐ |
| U13 | 解析 Trace Timeline（Langfuse） | ☐ |
| U14 | OpenSearch 向量驱动（若环境启用） | ☐ |
| U15 | 内置模型 YAML（`builtin_models.yaml`） | ☐ |
| U16 | Settings UI 重构 / 系统管理员 | ☐ |
| U17 | HNSW 索引迁移 `000059` | ☐ |
| U18 | chat 引用弹窗 / 流式 Markdown 重构 | ☐ |
| U19 | 网站嵌入组件 / Integrations Center | ☐ |
| U20 | Jaeger 移除 → Langfuse only | ☐ |
| U21 | `weknora` CLI 更新 | ☐ |
| U22 | Go 1.26 升级 | ☐ |

---

## 三、本地定制保护清单

### 3.1 P0 — 完全保护（5 大类）

#### A. CAS 认证

| # | 检查项 | 关键文件 | 合并 | 验证 |
|---|--------|----------|------|------|
| P0-A1 | CAS 会话验证 API | `internal/handler/cas_auth.go` | ☐ | ☐ |
| P0-A2 | AutoBind 用户/租户 + 密码规则 | `internal/application/service/cas_auth.go` | ☐ | ☐ |
| P0-A3 | NXIN Open API 客户端 | `internal/application/service/cas_client.go` | ☐ | ☐ |
| P0-A4 | Cookie 兜底鉴权 | `internal/middleware/auth.go` | ☐ | ☐ |
| P0-A5 | CAS 用户字段迁移 | `feature/0000016_add_cas_fields` | ☐ | ☐ |
| P0-A6 | 前端路由守卫 | `frontend/src/stores/cas.ts`、`router/index.ts` | ☐ | ☐ |
| P0-A7 | CAS 登出全链路 | `UserMenu.vue` → `casStore.logout` | ☐ | ☐ |
| P0-A8 | 配置项 | `cas`、`auth.nxin_cas_auth` | ☐ | ☐ |

#### B. 共享知识库

| # | 检查项 | 关键文件 | 合并 | 验证 |
|---|--------|----------|------|------|
| P0-B1 | 直接成员共享服务 | `shared_kb.go` | ☐ | ☐ |
| P0-B2 | 成员表与仓库 | `kb_member.go`、`feature/0000013` | ☐ | ☐ |
| P0-B3 | KB 扩展字段 | `feature/0000017` | ☐ | ☐ |
| P0-B4 | 跨租户 chunk 查询 | `knowledgebase_search_shared.go` | ☐ | ☐ |
| P0-B5 | 共享 API（create/list/join/leave/members） | `handler/knowledgebase.go` | ☐ | ☐ |
| P0-B6 | 广场页 / 成员管理 UI | `SharedKnowledgeBaseSquare.vue`、`KnowledgeBaseMembers.vue` | ☐ | ☐ |
| P0-B7 | 组织共享集成 | `kbshare.go`、`KBShareSettings.vue` | ☐ | ☐ |
| P0-B8 | 权限顺序 | owner > 组织 > 直接成员 > Agent | ☐ | ☐ |

#### C. 开放检索 open_retrieve

| # | 检查项 | 关键文件 | 合并 | 验证 |
|---|--------|----------|------|------|
| P0-C1 | 开放检索 Handler | `open_retrieve.go` | ☐ | ☐ |
| P0-C2 | API Key + 限流中间件 | `open_retrieve.go`（middleware） | ☐ | ☐ |
| P0-C3 | SearchKnowledgeOpen | `session_knowledge_qa.go` | ☐ | ☐ |
| P0-C4 | buildOpenSearchTargets | `session_knowledge_qa.go` | ☐ | ☐ |
| P0-C5 | 路由与白名单 | `router.go`、`auth.go` noAuthAPI | ☐ | ☐ |
| P0-C6 | 配置项 | `open_retrieve` | ☐ | ☐ |
| P0-C7 | 中间件测试 | `open_retrieve_auth_test.go` | ☐ | ☐ |

#### D. 前端 SVG 组件化

| # | 检查项 | 关键文件 | 合并 | 验证 |
|---|--------|----------|------|------|
| P0-D1 | SvgIcon 组件 | `icons/SvgIcon.vue` | ☐ | ☐ |
| P0-D2 | 图标注册表（13 个） | `icons/registry.ts` | ☐ | ☐ |
| P0-D3 | 业务组件 SvgIcon 引用 | `menu.vue` 等 8 个组件 | ☐ | ☐ |
| P0-D4 | NXIN Logo | `nxin-weknora.svg` | ☐ | ☐ |
| P0-D5 | 禁止回退 `<img src="*.svg">` | 全前端 grep 检查 | ☐ | ☐ |

#### E. Redis 集群

| # | 检查项 | 关键文件 | 合并 | 验证 |
|---|--------|----------|------|------|
| P0-E1 | initRedisClient 双模式 | `container/container.go` | ☐ | ☐ |
| P0-E2 | asynq 集群连接 | `router/task.go` | ☐ | ☐ |
| P0-E3 | UniversalClient 兼容 | `llmcontext/redis_storage.go` | ☐ | ☐ |
| P0-E4 | docker-compose env | `REDIS_MODE`、`REDIS_CLUSTER_ADDRS` | ☐ | ☐ |
| P0-E5 | 集群模式 CAS 缓存 | `auth.go` + Redis | ☐ | ☐ |
| P0-E6 | 集群模式 asynq 任务 | 文档上传异步解析 | ☐ | ☐ |
| P0-E7 | Helm 集群 env（升级后补充） | `helm/templates/app.yaml` | ☐ | ☐ |

---

### 3.2 P0+ — 强耦合保护（4 项）

| # | 检查项 | 关键文件 | 合并 | 验证 |
|---|--------|----------|------|------|
| P0+-1 | 跨租户 KB 权限解析链 | `knowledge.go`、`chunk.go`、`faq.go`、`tag.go` | ☐ | ☐ |
| P0+-2 | 共享 KB 不可 pin | `knowledgebase.go`（service + handler） | ☐ | ☐ |
| P0+-3 | buildSearchTargets 成员判定 | `session_knowledge_qa.go`（`5fa1767e`） | ☐ | ☐ |
| P0+-4 | Agent 跨租户读共享 KB | `agent/tools/list_knowledge_chunks.go` | ☐ | ☐ |
| P0+-5 | CORS 白名单 NXIN 域名 | `router.go` | ☐ | ☐ |
| P0+-6 | 前端 allowedHosts / API 默认址 | `vite.config.ts`、`request.ts` | ☐ | ☐ |
| P0+-7 | MinIO 桶名 | `docker-compose.yml` → `nxinweknora` | ☐ | ☐ |
| P0+-8 | HTTPS 进程内 TLS | `config.yaml`、`config.go` | ☐ | ☐ |
| P0+-9 | kb_access 收口（建议） | `extensions/nxin/kb_access/resolver.go` | ☐ | ☐ |

---

### 3.3 P1 — 合并保护（7 项）

| # | 检查项 | 关键文件 | 合并 | 验证 |
|---|--------|----------|------|------|
| P1-1 | 颜色统一化 CSS 变量 | `theme/theme.css` + 51 组件 | ☐ | ☐ |
| P1-2 | kb-permission 工具 | `utils/kb-permission.ts` | ☐ | ☐ |
| P1-3 | 成员邮箱/姓名/CAS 姓名搜索 | `kb_member.go` | ☐ | ☐ |
| P1-4 | 知识库广场菜单/路由/i18n | `menu.ts`、`router`、`locales/*` | ☐ | ☐ |
| P1-5 | KB 编辑/三源列表 | `KnowledgeBaseEditorModal.vue`、`KnowledgeBaseList.vue` | ☐ | ☐ |
| P1-6 | Settings API 信息门控 | `Settings.vue`（tenantId ≤ 10001） | ☐ | ☐ |
| P1-7 | UserMenu 品牌化（隐藏外链） | `UserMenu.vue` | ☐ | ☐ |
| P1-8 | 存储配额默认 1GB | `feature/0000014` | ☐ | ☐ |
| P1-9 | NXIN 配置结构完整 | `config.yaml` + `config.go` | ☐ | ☐ |

---

### 3.4 P2 — 关注项（不阻断合并）

| # | 检查项 | 关键文件 | 关注时机 | 状态 |
|---|--------|----------|----------|------|
| P2-1 | 测试环境编排 | `docker-compose.test.yml` | Phase 5 | ☐ |
| P2-2 | Windows 沙箱编译 | `sandbox/local_windows.go` | 本地构建 | ☐ |
| P2-3 | ParadeDB / BM25 修复 | `migrations/paradedb/*`、`fix_bm25_*.sql` | 用 ParadeDB 时 | ☐ |
| P2-4 | 合并辅助脚本 | `merge-execute.ps1` 等 | 合并期间 | ☐ |
| P2-5 | SSL 证书脚本 | `generate-ssl-cert.*` | 部署时 | ☐ |
| P2-6 | 用户导入修复脚本 | `import_user_fix.sql` 等 | 数据迁移时 | ☐ |

---

## 四、关键文件合并清单（高风险）

| 优先级 | 文件 | 保护级别 | 合并完成 | 冲突已解 |
|--------|------|----------|----------|----------|
| ⭐⭐⭐⭐⭐ | `internal/container/container.go` | P0-E | ☐ | ☐ |
| ⭐⭐⭐⭐⭐ | `internal/router/task.go` | P0-E | ☐ | ☐ |
| ⭐⭐⭐⭐⭐ | `internal/middleware/auth.go` | P0-A/C/E | ☐ | ☐ |
| ⭐⭐⭐⭐⭐ | `internal/router/router.go` | P0-A/C + P0+-5 | ☐ | ☐ |
| ⭐⭐⭐⭐⭐ | `internal/application/service/session_knowledge_qa.go` | P0-C + P0+-3 | ☐ | ☐ |
| ⭐⭐⭐⭐⭐ | `internal/handler/knowledgebase.go` | P0-B + P0+-2 | ☐ | ☐ |
| ⭐⭐⭐⭐ | `internal/handler/knowledge.go` | P0+-1 | ☐ | ☐ |
| ⭐⭐⭐⭐ | `internal/config/config.go` | P1-9 + P0+-8 | ☐ | ☐ |
| ⭐⭐⭐⭐ | `config/config.yaml` | 全部 NXIN 配置 | ☐ | ☐ |
| ⭐⭐⭐⭐ | `frontend/src/router/index.ts` | P0-A + P1-4 | ☐ | ☐ |
| ⭐⭐⭐⭐ | `frontend/src/views/knowledge/KnowledgeBaseList.vue` | P0-B + P1-5 + P0-D | ☐ | ☐ |
| ⭐⭐⭐⭐ | `frontend/src/components/menu.vue` | P0-D + P1-5 | ☐ | ☐ |
| ⭐⭐⭐⭐ | `frontend/src/components/icons/*` | P0-D | ☐ | ☐ |
| ⭐⭐⭐⭐ | `frontend/vite.config.ts` | P0+-6 | ☐ | ☐ |
| ⭐⭐⭐⭐ | `frontend/src/utils/request.ts` | P0+-6 | ☐ | ☐ |
| ⭐⭐⭐ | `frontend/src/assets/theme/theme.css` | P1-1 | ☐ | ☐ |
| ⭐⭐⭐ | `frontend/src/views/settings/Settings.vue` | P1-6 | ☐ | ☐ |
| ⭐⭐⭐ | `migrations/versioned/*` | 上游 000059 + feature/ | ☐ | ☐ |
| ⭐⭐⭐ | `go.mod` | U22 | ☐ | ☐ |
| ⭐⭐⭐ | `docker-compose.yml` | P0-E + P0+-7 | ☐ | ☐ |

---

## 五、数据库迁移清单

| 顺序 | 迁移范围 | 说明 | 测试库 | 生产库 |
|------|----------|------|--------|--------|
| 1 | 上游 `000033`–`000059` | 按序号执行 | ☐ | ☐ |
| 2 | 本地 `feature/0000013` | KB 成员表 | ☐ | ☐ |
| 3 | 本地 `feature/0000014` | 存储配额 1GB | ☐ | ☐ |
| 4 | 本地 `feature/0000016` | CAS 用户字段 | ☐ | ☐ |
| 5 | 本地 `feature/0000017` | 共享 KB 字段 | ☐ | ☐ |
| 6 | HNSW `000059` 低峰执行 | 1024 维向量索引 | ☐ | ☐ |

**迁移后数据校验**

| 检查项 | 状态 |
|--------|------|
| `users.cas_user_id` / `cas_real_name` 等字段存在 | ☐ |
| `knowledge_base_members` 表存在且有数据 | ☐ |
| `knowledge_bases.visibility` / `owner_id` 字段存在 | ☐ |
| 租户 `storage_quota` 默认 1GB | ☐ |

---

## 六、分阶段执行清单

### Phase 0：准备（1–2 天）

| # | 任务 | 状态 |
|---|------|------|
| 0.1 | 创建备份分支 `backup/2026-07-09-v0.3.6-pre-upgrade` | ☐ |
| 0.2 | 创建升级分支 `upgrade/v0.3.6-to-v0.6.3` | ☐ |
| 0.3 | `git fetch upstream --tags` | ☐ |
| 0.4 | ~~导出 NXIN 补丁包 `patches/2026-07-09/`~~（已取消；2026-07-17 删除乱码 patch，差异以升级分支为准） | ☑ 取消 |
| 0.5 | 扩展目录收口（`extensions/nxin/`） | ☐ |
| 0.6 | 记录基线（CAS / 共享KB / open_retrieve / SVG / Redis / CORS） | ☐ |
| 0.7 | 测试库 + 生产库备份 | ☐ |

### Phase 1：合并 Tag `v0.4.0`（2–3 天）

| # | 任务 | 状态 |
|---|------|------|
| 1.1 | `git merge v0.4.0` | ☐ |
| 1.2 | 解决冲突（见第四节高风险文件） | ☐ |
| 1.3 | `go build` + 前端 build 通过 | ☐ |
| 1.4 | Checkpoint-1 回归（§七） | ☐ |

### Phase 2：合并 Tag `v0.5.2`（3–4 天）

| # | 任务 | 状态 |
|---|------|------|
| 2.1 | `git merge v0.5.2`（或逐步 `v0.5.0` → `v0.5.1` → `v0.5.2`） | ☐ |
| 2.2 | RBAC 与 CAS AutoBind 角色对齐 | ☐ |
| 2.3 | effectiveTenantID 链接入 RBAC | ☐ |
| 2.4 | Checkpoint-2 回归（§七） | ☐ |

### Phase 3：合并 Tag `v0.6.3`（4–5 天）

| # | 任务 | 状态 |
|---|------|------|
| 3.1 | `git merge v0.6.3`（或逐步 `v0.6.0` → `v0.6.1` → `v0.6.2` → `v0.6.3`） | ☐ |
| 3.2 | session_knowledge_qa 适配新 pipeline | ☐ |
| 3.3 | Settings / menu 大改后 P1 项保留 | ☐ |
| 3.4 | Helm 补充 Redis 集群 env | ☐ |
| 3.5 | 数据库迁移 000033–000059 | ☐ |
| 3.6 | Checkpoint-3 全量回归（§七） | ☐ |

### Phase 4：质量收敛（2–3 天）

| # | 任务 | 状态 |
|---|------|------|
| 4.1 | 补充 CAS / 共享KB 单测 | ☐ |
| 4.2 | 性能基线对比（open_retrieve / CAS） | ☐ |
| 4.3 | 文档与 config 示例同步 | ☐ |

### Phase 5：部署灰度（2–3 天）

| # | 任务 | 状态 |
|---|------|------|
| 5.1 | 测试环境部署 + 冒烟 | ☐ |
| 5.2 | 生产灰度（CAS → 只读 → open_retrieve → 全量） | ☐ |
| 5.3 | 回滚预案验证 | ☐ |

---

## 七、回归验证清单

### 7.1 CAS（C1–C6）

| # | 场景 | 状态 |
|---|------|------|
| C1 | 前端 SSO 登录 | ☐ |
| C2 | Cookie 兜底 API | ☐ |
| C3 | HTTPS 要求 | ☐ |
| C4 | Redis 缓存命中 | ☐ |
| C5 | 登出全链路 | ☐ |
| C6 | 新用户 AutoBind | ☐ |

### 7.2 共享知识库（S1–S8）

| # | 场景 | 状态 |
|---|------|------|
| S1 | 创建共享 KB | ☐ |
| S2 | 广场列表 | ☐ |
| S3 | 加入/离开 | ☐ |
| S4 | 成员管理 | ☐ |
| S5 | 组织共享 | ☐ |
| S6 | 跨租户会话检索 | ☐ |
| S7 | 列表去重 | ☐ |
| S8 | IM 通道（若启用） | ☐ |

### 7.3 open_retrieve（O1–O8）

| # | 场景 | 状态 |
|---|------|------|
| O1 | 正常召回 | ☐ |
| O2 | 无 Key → 401 | ☐ |
| O3 | 错误 Key → 403 | ☐ |
| O4 | 限流 429 | ☐ |
| O5 | 禁用开关 | ☐ |
| O6 | 无用户权限校验（设计行为） | ☐ |
| O7 | 多 Key 轮换 | ☐ |
| O8 | 管道升级后 top-5 重合率 ≥ 80% | ☐ |

### 7.4 SVG（I1–I10）

| # | 场景 | 状态 |
|---|------|------|
| I1–I10 | 见主方案 §6.4 | ☐ |

### 7.5 Redis（R1–R10）

| # | 场景 | 状态 |
|---|------|------|
| R1–R10 | 见主方案 §6.5 | ☐ |

### 7.6 P0+ / P1 补充（X1–X8）

| # | 场景 | 状态 |
|---|------|------|
| X1 | CORS 跨域 `zsk.t.nxin.com` | ☐ |
| X2 | 共享 KB 会话检索 buildSearchTargets | ☐ |
| X3 | 共享 KB pin 拒绝 | ☐ |
| X4 | 成员 CAS 姓名搜索 | ☐ |
| X5 | tenantId > 10001 隐藏 API 信息 | ☐ |
| X6 | HTTPS + CAS Cookie 兜底 | ☐ |
| X7 | light/dark 主题切换 | ☐ |
| X8 | 新租户存储配额 1GB | ☐ |

---

## 八、配置与环境清单

### 8.1 后端配置（`config/config.yaml`）

| 配置段 | 用途 | 已合并 | 已验证 |
|--------|------|--------|--------|
| `cas` | NXIN CAS 环境 | ☐ | ☐ |
| `auth.nxin_cas_auth` | Cookie 兜底鉴权 | ☐ | ☐ |
| `open_retrieve` | 开放检索 | ☐ | ☐ |
| `server.https` | 进程内 TLS | ☐ | ☐ |

### 8.2 环境变量

| 变量 | 环境 | 已配置 | 已验证 |
|------|------|--------|--------|
| `REDIS_MODE=cluster` | 生产 | ☐ | ☐ |
| `REDIS_CLUSTER_ADDRS` | 生产 | ☐ | ☐ |
| `REDIS_ADDR` | 测试/开发 | ☐ | ☐ |
| `VITE_APP_CAS` | 前端 | ☐ | ☐ |
| `VITE_APP_APP` | 前端 | ☐ | ☐ |
| `VITE_CAS_ENV` | 前端 | ☐ | ☐ |
| `MINIO_BUCKET_NAME=nxinweknora` | 部署 | ☐ | ☐ |

### 8.3 域名与白名单

| 项 | 值 | 已保留 |
|----|-----|--------|
| 生产前端 | `zsk.nxin.com` | ☐ |
| 测试前端 | `zsk.t.nxin.com` | ☐ |
| CORS（router.go） | 上述 + localhost | ☐ |
| Vite allowedHosts | `.nxin.com` | ☐ |
| API 默认址 | `zsk.t.nxin.com:8080` | ☐ |

---

## 九、统计摘要

| 类别 | 数量 |
|------|------|
| 上游能力吸收项 | 22 |
| P0 检查项 | 38 |
| P0+ 检查项 | 9 |
| P1 检查项 | 9 |
| P2 关注项 | 6 |
| 高风险合并文件 | 20 |
| 回归用例 | 50+ |
| 预估总工时 | 14–20 人天 |

---

## 十、相关文档

- [v0.3.6 → v0.6.3 升级方案](./2026-07-09-WeKnora-slj-v0.3.6-to-v0.6.3升级方案.md)（详细设计与决策）
- [项目升级与合并指南](./2026-02-11-项目升级与合并指南.md)
- [代码合并执行方案](./2026-04-01-代码合并执行方案.md)
- [开放检索 API](./api/open-knowledge-retrieve.md)

---

*执行时在「状态」列勾选 ☐ → ☑。每完成一个 Phase 的 Checkpoint 后更新本节并归档基线快照。*
