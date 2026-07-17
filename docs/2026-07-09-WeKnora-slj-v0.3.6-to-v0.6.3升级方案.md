# WeKnora-slj 升级方案（`v0.3.6` → `v0.6.3`）

> **文档创建**：2026-07-09  
> **上游仓库**：`Tencent/WeKnora`（Git tag 命名：`vMAJOR.MINOR.PATCH`）  
> **基线 Tag**：`v0.3.6`（WeKnora-slj 当前 `VERSION` + 7 个 NXIN 定制提交）  
> **目标 Tag**：`v0.6.3`（上游最新 Release，2026-06-26）  
> **执行清单**：[升级内容清单](./2026-07-09-WeKnora-slj-v0.3.6-to-v0.6.3升级内容清单.md)（勾选跟踪表）  
> **核心约束**：升级后 §三 全部 **P0 / P0+ / P1** 保护项必须保持可用；**P2** 按部署环境关注验证

---

## 一、概述

### 1.1 升级目标

将 `WeKnora-slj` 从上游 Tag **`v0.3.6`** 升级到 **`v0.6.3`**，阶梯合并其间官方 Tag（`v0.4.0` → `v0.5.2` → `v0.6.3`），完整保留 NXIN 企业定制能力。

### 1.1.1 官方 Tag 阶梯（`Tencent/WeKnora`）

```
v0.3.6  ← 基线（当前）
  → v0.4.0
  → v0.5.0 → v0.5.1 → v0.5.2   （合并检查点选用 v0.5.2）
  → v0.6.0 → v0.6.1 → v0.6.2 → v0.6.3   （目标）
```

> 合并命令统一使用：`git fetch upstream --tags` 后 `git merge v0.x.y`（Tag 名带 `v` 前缀）。

### 1.2 当前状态快照

| 维度 | 当前值 | 目标值 |
|------|--------|--------|
| 版本 Tag | `v0.3.6`（`5fa1767e`） | `v0.6.3` |
| 上游仓库 | `upstream` → `Tencent/WeKnora` | 同左，对齐 Tag |
| 落后上游提交数 | ~85 个（`HEAD..upstream/main`） | 0 |
| 领先上游定制提交 | 7 个（`upstream/main..HEAD`） | rebase 叠于 Tag `v0.6.3` 之上 |
| 本地定制文件变更量 | 198 文件，+29148 / -2948 行 | 扩展模块边界清晰、可测试 |
| 数据库迁移 | `000000`–`000032` + `feature/0000013–0000017` | 上游 `000000`–`000059` + 保留 feature 迁移 |
| Go 版本 | `1.24.11` | 上游 `1.26`（v0.6.x） |
| 前端 | Vue 3.5 + Vite 7 + TDesign 1.17 | 上游大改（chatResources、Settings 重构、Wiki 等） |

### 1.3 本地定制提交清单（必须保留）

```
5fa1767e bugfix: 还原共享知识库流程及优化向量搜索逻辑
367e6e66 feat(knowledge): Expand the knowledge base retrieval independent API key solution 1
fd063ab6 feat(knowledge): Expand the knowledge base retrieval independent API key solution
d98ecc8f bugfix: knowledgeBase back
b31fa1d1 feat: add NXIN CAS auth
```

（更早的 UI/主题/共享 KB 基础提交已合入 main 历史，上述为相对 upstream/main 的净增量。）

### 1.4 推荐策略：分阶段合并 + 扩展层隔离

**不推荐**一次性 `git merge v0.6.3`（或 `upstream/main`）跨越全部 Tag（冲突面过大）。

**推荐**按官方 Tag 阶梯合并（Strangler）：

```
v0.3.6（基线）
    ↓ git merge v0.4.0  → Checkpoint-1
v0.4.0
    ↓ git merge v0.5.2  → Checkpoint-2
v0.5.2
    ↓ git merge v0.6.3  → Checkpoint-3
v0.6.3 + NXIN 扩展层
```

每阶段只消化一个版本的 BREAKING 变更，冲突可定位、可回滚。

---

## 二、版本跨度与关键变更矩阵

### 2.1 版本路线图

| 版本 | 发布日期 | 对 NXIN 扩展的影响等级 | 核心变更摘要 |
|------|----------|------------------------|--------------|
| **v0.4.0** | 2026-04-14 | 🟡 中 | VectorStore 实体、附件处理、Azure/OSS、Notion 连接器、IM 增强 |
| **v0.5.0** | 2026-04-27 | 🟠 较高 | RBAC 初版、工作区/租户模型调整、检索参数重构 |
| **v0.5.2** | 2026-05-13 | 🟠 较高 | 租户成员管理、审计日志、凭证 AES 加密、多向量库 fan-out |
| **v0.6.0** | 2026-05-21 | 🔴 高 | Wiki、NDJSON CLI、MCP 人工审批、自适应分块、全局 ⌘K |
| **v0.6.1** | 2026-06-05 | 🔴 高 | OpenSearch 驱动、内置模型 YAML、系统管理员、解析 Trace Timeline、RBAC 完善 |
| **v0.6.2** | 2026-06-10 | 🟠 较高 | 按上传 process_config、文档 reparse、HNSW 索引、Jaeger 移除 |
| **v0.6.3** | 2026-06-26 | 🟡 中 | 网站嵌入组件、Integrations Center、引用弹窗、Wiki 文件夹、RSS 数据源 |

### 2.2 与保护清单相关的上游变更

#### CAS 认证

| 上游变更 | 影响 | 应对 |
|----------|------|------|
| v0.3.6 引入 OIDC | 与 CAS 并存，不冲突 | 保留双通道：OIDC（上游）+ NXIN CAS（本地） |
| v0.6.1 RBAC / 系统管理员 | 用户角色模型扩展 | CAS AutoBind 需写入默认角色；检查 `users` 表新字段 |
| v0.6.3 `auth: clear session resource caches on logout` | 登出流程变更 | CAS logout 需同步清理新缓存 |
| 中间件 `auth.go` 大幅重构 | **最高风险** | 将 `tryNXINCASAuth` 作为独立函数注入新中间件链末尾 |

#### 共享知识库

| 上游变更 | 影响 | 应对 |
|----------|------|------|
| v0.3.6 组织/共享空间（`kb_shares`） | 已合并 | 保持双模式共存 |
| v0.5.x 每 KB 所有权（per-KB ownership） | 权限模型变更 | `owner_id` 与上游 ownership 字段对齐 |
| v0.6.1 IM synthetic identity for shared KBs | IM 通道访问共享 KB | 验证 IM 场景下直接共享 + 组织共享 |
| v0.6.2 KB list deduplication 修复 | 列表逻辑变更 | 合并本地去重逻辑与上游修复 |
| `session_knowledge_qa.go` 多次重构 | 检索目标构建变更 | `buildOpenSearchTargets` / 共享 KB 合并逻辑需重新接入 |

#### 开放检索（open_retrieve）

| 上游变更 | 影响 | 应对 |
|----------|------|------|
| 上游无此功能 | 纯本地新增 | 作为独立路由组保留，不并入租户 API Key |
| v0.6.x 检索管道（rerank threshold、progress events） | 返回结构可能变化 | 更新 `SearchKnowledgeOpen` 适配新 pipeline 接口 |
| v0.6.1 SSRF / 向量库校验增强 | 开放接口需合规 | 确认 open_retrieve 不走用户权限但仍受 SSRF 策略约束 |
| v0.6.2 `process_config` | 检索参数来源变化 | 明确 open_retrieve 使用 KB 级默认配置，不支持 per-request override |

#### 前端 SVG 组件化

| 上游变更 | 影响 | 应对 |
|----------|------|------|
| v0.6.3 侧边栏/菜单大改（`menu` 组件增强） | **高风险**：上游可能回退为 `<img src="*.svg">` | 合并时保留 `<SvgIcon>` 引用，禁止覆盖 `icons/` 目录 |
| v0.6.1 Settings UI 重构 | 设置页图标引用方式变化 | 新页面继续使用 `SvgIcon`，不引入硬编码色 SVG 文件 |
| v0.6.3 chat/agent 流式 UI 重构 | `AgentStreamDisplay` 等组件冲突 | 上游布局 + 本地 `SvgIcon` 图标替换 |
| 上游品牌 Logo 组件 | 与 NXIN Logo 冲突 | 保留 `nxin-weknora.svg` + `menu.vue` 品牌区 |
| `theme.css` 变量体系变更 | `SvgIcon` 的 `theme` 预设依赖 CSS 变量 | 合并变量表，保留 `--td-*` 映射与 `--wk-*` 自定义变量 |

#### Redis 集群配置

| 上游变更 | 影响 | 应对 |
|----------|------|------|
| 上游默认单机 `redis.NewClient` | **高风险**：合并可能覆盖 `initRedisClient` 集群分支 | 保留 `REDIS_MODE=cluster` 双模式初始化逻辑 |
| v0.3.5+ IM 分布式协调（Redis） | IM 限流/队列/去重依赖 Redis | 确认 `redis.UniversalClient` 注入不变，集群模式可用 |
| v0.6.1 asynq 并发调优 | 异步任务队列配置变更 | 保留 `getAsynqRedisConnOpt` 的 `RedisClusterClientOpt` 分支 |
| CAS Cookie 兜底缓存（`auth:nxin_cas_auth:*`） | 依赖 Redis 读写 | 集群模式下验证 CAS 缓存 TTL 正常 |
| Helm 模板仅含单机 Redis | K8s 部署缺少集群 env | 合并后补充 `REDIS_MODE` / `REDIS_CLUSTER_ADDRS` 到 `helm/templates/app.yaml` |

---

## 三、扩展功能保护清单

### 3.0 保护分级总览

| 级别 | 章节 | 项数 | 合并要求 |
|------|------|------|----------|
| **P0** | §3.1–§3.5 | 5 大类 | 完全保护，禁止覆盖 |
| **P0+** | §3.6–§3.9 | 4 项 | 完全保护，与 P0 强耦合 |
| **P1** | §3.10–§3.16 | 7 项 | 合并保护，冲突时优先保留本地 |
| **P2 关注** | §3.17 | 6 类 | 按部署环境验证，不阻断合并 |
| **跟随上游** | §3.18 | — | 无需单独保护 |

---

### 3.1 CAS 认证（P0）

**后端（不可删除/覆盖）**

| 文件 | 职责 |
|------|------|
| `internal/handler/cas_auth.go` | `GET /api/v1/cas/validate` |
| `internal/application/service/cas_auth.go` | 会话验证、AutoBindUser/Tenant、密码规则（手机后四位/默认 `1234`） |
| `internal/application/service/cas_client.go` | NXIN Open API 客户端 |
| `internal/types/cas.go` | `CASUserInfo` |
| `internal/types/interfaces/cas_auth.go` | 服务接口 |
| `internal/middleware/auth.go` | `tryNXINCASAuth` 兜底逻辑 |
| `migrations/versioned/feature/0000016_add_cas_fields.up.sql` | `users.cas_*` 四字段 |

**前端（不可删除/覆盖）**

| 文件 | 职责 |
|------|------|
| `frontend/src/stores/cas.ts` | 路由守卫 `validateSession`、CAS 登出跳转 |
| `frontend/src/api/cas/index.ts` | CAS API 封装 |
| `frontend/src/router/index.ts` | `beforeEach` CAS 跳转逻辑 |
| `frontend/src/components/UserMenu.vue` | 登出链：`logoutApi` → `authStore.logout` → `casStore.logout` |

**配置项**

```yaml
auth.nxin_cas_auth:      # Cookie 直调 API 兜底
cas:                     # NXIN CAS 环境（test/production）
```

```env
VITE_APP_CAS / VITE_APP_APP / VITE_CAS_ENV
```

### 3.2 共享知识库（P0）

**直接成员共享（本地独有）**

| 文件 | 职责 |
|------|------|
| `internal/application/service/shared_kb.go` | 广场/加入/离开/成员管理 |
| `internal/application/repository/kb_member.go` | `knowledge_base_members` CRUD |
| `internal/types/interfaces/shared_kb.go` | 接口定义 |
| `internal/application/service/knowledgebase_search_shared.go` | 跨租户 chunk 查询 |
| `internal/handler/knowledgebase.go` | 共享 KB API（+join/leave/members） |
| `frontend/src/views/knowledge/SharedKnowledgeBaseSquare.vue` | 广场页 |
| `frontend/src/views/knowledge/settings/KnowledgeBaseMembers.vue` | 成员管理 |
| `migrations/versioned/feature/0000013_create_kb_members.up.sql` | 成员表 |
| `migrations/versioned/feature/0000017_add_shared_kb_fields.up.sql` | visibility/owner_id 等 |

**组织共享（上游 + 本地集成）**

| 文件 | 职责 |
|------|------|
| `internal/application/service/kbshare.go` | 组织级共享 |
| `frontend/src/views/knowledge/settings/KBShareSettings.vue` | 组织共享 UI |

**权限解析顺序（必须保持）**

```
owner > 组织共享(kb_shares) > 直接成员共享(knowledge_base_members) > Agent共享
```

### 3.3 开放检索 open_retrieve（P0）

| 文件 | 职责 |
|------|------|
| `internal/handler/open_retrieve.go` | `POST /api/v1/open/knowledge/retrieve` |
| `internal/middleware/open_retrieve.go` | API Key + QPS 限流 |
| `internal/middleware/open_retrieve_auth_test.go` | 中间件测试 |
| `internal/application/service/session_knowledge_qa.go` | `SearchKnowledgeOpen`、`buildOpenSearchTargets` |
| `internal/types/interfaces/session.go` | 接口定义 |
| `internal/middleware/auth.go` | `noAuthAPI` 白名单 |
| `internal/router/router.go` | `/api/v1/open` 路由组 |
| `docs/api/open-knowledge-retrieve.md` | API 文档 |

**配置项**

```yaml
open_retrieve:
  enabled: true
  api_key: "..."
  api_keys: ["..."]
  rate_limit_qps: 100
```

### 3.4 前端 SVG 组件化（P0）

> 设计文档：[自定义 SVG 组件化方案](./2026-02-12-自定义SVG组件化方案.md)

**核心组件（不可删除/覆盖）**

| 文件 | 职责 |
|------|------|
| `frontend/src/components/icons/SvgIcon.vue` | 通用图标容器（`name` / `size` / `color` / `theme` / `variant`） |
| `frontend/src/components/icons/registry.ts` | 图标注册表（13 个内联 SVG，均 `currentColor`） |
| `frontend/src/components/icons/index.ts` | 统一导出 `SvgIcon`、`IconName`、`IconVariant` |

**已注册图标（`registry.ts`）**

`zhishiku` / `zhishikuThin` / `organization` / `ziliao` / `agent` / `agentGreen` / `agentActive` / `user` / `thinking` / `websearch` / `fileAdd` / `setting` / `prefixIcon`

**已接入 SvgIcon 的业务组件（合并时须保留引用）**

| 文件 | 用途 |
|------|------|
| `frontend/src/components/menu.vue` | 侧边栏导航图标 + NXIN 品牌 Logo |
| `frontend/src/components/AgentSelector.vue` | Agent 选择器组织/用户图标 |
| `frontend/src/components/MentionSelector.vue` | @提及选择器图标 |
| `frontend/src/views/knowledge/KnowledgeBaseList.vue` | KB 列表组织来源图标 |
| `frontend/src/views/agent/AgentList.vue` | Agent 列表组织图标 |
| `frontend/src/views/organization/OrganizationList.vue` | 组织管理页图标 |
| `frontend/src/views/chat/components/AgentStreamDisplay.vue` | 流式对话工具/状态图标 |
| `frontend/src/views/chat/components/docInfo.vue` | 文档资料图标 |
| `frontend/src/assets/img/nxin-weknora.svg` | NXIN 品牌 Logo |

**合并原则**

1. **禁止**将 `<SvgIcon name="..." />` 回退为 `<img src="xxx.svg">` 或多份 `-green`/`-grey` 变体文件
2. 上游组件功能优先，本地图标引用方式优先（`SvgIcon` + `currentColor`）
3. 新增页面/组件继续使用 `import { SvgIcon } from '@/components/icons'`，不新增散落 SVG 文件
4. 冲突解决顺序：`icons/` 目录完整保留 → `theme.css` 合并变量 → 各业务 Vue 组件保留 `SvgIcon` 标签

### 3.5 Redis 集群配置（P0）

> 引入时间：2026-04-10（基于 Tag `v0.3.6` 的 NXIN 提交），生产环境使用 Redis Cluster，开发/测试环境使用单机。

**核心实现（不可删除/覆盖）**

| 文件 | 职责 |
|------|------|
| `internal/container/container.go` | `initRedisClient()`：`REDIS_MODE=cluster` 时创建 `*redis.ClusterClient`，否则单机 `*redis.Client`；统一返回 `redis.UniversalClient` |
| `internal/router/task.go` | `getAsynqRedisConnOpt()`：集群模式返回 `asynq.RedisClusterClientOpt`，单机返回 `asynq.RedisClientOpt` |
| `internal/application/service/llmcontext/redis_storage.go` | 上下文存储基于 `redis.UniversalClient`，兼容单机/集群 |

**依赖 Redis 的业务模块（须验证集群模式）**

| 文件 | 用途 |
|------|------|
| `internal/middleware/auth.go` | NXIN CAS 会话缓存（`auth:nxin_cas_auth:*`） |
| `internal/im/ratelimit.go` | IM 分布式限流 |
| `internal/im/qaqueue.go` | IM QA 队列 |
| `internal/im/service.go` | IM 多实例协调 |
| `internal/application/service/knowledge.go` | 知识处理任务状态 |
| `internal/application/service/web_search_state.go` | 网页搜索状态 |
| `internal/utils/debug.go` | 任务清理/状态检查 |

**环境变量（部署配置）**

| 变量 | 单机模式 | 集群模式 |
|------|----------|----------|
| `REDIS_MODE` | 空 / `single` | `cluster` |
| `REDIS_ADDR` | `host:port`（默认 `redis:6379`） | 不使用 |
| `REDIS_CLUSTER_ADDRS` | 不使用 | 逗号分隔，如 `node1:6379,node2:6379,node3:6379` |
| `REDIS_USERNAME` | 可选 | 可选 |
| `REDIS_PASSWORD` | 可选 | 可选 |
| `REDIS_DB` | 默认 `0` | 集群模式忽略（Redis Cluster 仅 db 0） |
| `REDIS_PREFIX` | Key 前缀 | Key 前缀 |

**部署文件（须保留集群 env 注入）**

| 文件 | 说明 |
|------|------|
| `docker-compose.yml` | 已注入 `REDIS_MODE`、`REDIS_CLUSTER_ADDRS` 等 |
| `helm/templates/app.yaml` | ⚠️ 当前仅单机 env，升级后需补充集群变量 |
| `helm/values.yaml` | 需新增 `redis.mode` / `redis.clusterAddrs` 配置项（建议） |

**配置示例**

```bash
# 生产 Redis Cluster
REDIS_MODE=cluster
REDIS_CLUSTER_ADDRS=10.0.1.1:6379,10.0.1.2:6379,10.0.1.3:6379
REDIS_PASSWORD=your-password

# 开发/测试单机
REDIS_MODE=single
REDIS_ADDR=redis:6379
REDIS_PASSWORD=redis123456
REDIS_DB=0
```

**合并原则**

1. `initRedisClient` 与 `getAsynqRedisConnOpt` **必须保持双模式**（`cluster` / `single`），禁止合并回仅单机实现
2. 所有 Redis 消费方继续使用 `redis.UniversalClient` 接口，禁止改回 `*redis.Client` 强类型
3. 上游若重构 Redis 初始化，将集群逻辑提取为 `internal/infrastructure/redis/client.go`（或 `internal/extensions/nxin/redis/`）独立函数
4. 合并后分别在 **单机** 和 **集群** 两种模式下跑通 CAS 缓存、asynq 任务、IM 限流

### 3.6 跨租户 KB 权限解析层（P0+）

> 共享知识库的「暗线」：散落在各 Handler 的 `effectiveTenantID` 解析链，任何一处回退都会导致跨租户读写失败。

| 文件 | 关键函数/逻辑 |
|------|---------------|
| `internal/handler/knowledge.go` | `validateKnowledgeBaseAccess(WithKBID)` |
| `internal/handler/chunk.go` | 注入 `sharedKBService`，成员角色校验 |
| `internal/handler/faq.go` | `effectiveCtxForKB` |
| `internal/handler/tag.go` | `effectiveCtxForKB` |
| `internal/handler/knowledgebase.go` | 共享 KB CRUD + `ErrSharedKnowledgeBasePinNotAllowed` |
| `internal/application/service/knowledgebase.go` | 注入 `sharedKBService`，共享 KB 不可 pin |
| `internal/application/service/knowledge.go` | 共享 KB 下 FAQ/文档跨租户查询 |
| `internal/agent/tools/list_knowledge_chunks.go` | Agent 工具跨租户读共享 KB |

**合并策略**：建议提取 `internal/extensions/nxin/kb_access/resolver.go`，各 Handler 统一调用。

### 3.7 检索目标构建 — 共享成员判定（P0+）

`session_knowledge_qa.go` 中 `buildSearchTargets` 在组织共享之外，增加直接成员共享判定（`5fa1767e` 修复，合并时极易丢失）：

```go
if !hasAccess && s.sharedKBService != nil {
    role, _ := s.sharedKBService.GetMemberRoleByKBAndUser(ctx, kbID, userID)
    hasAccess = role != ""
}
```

| 文件 | 职责 |
|------|------|
| `internal/application/service/session_knowledge_qa.go` | `buildSearchTargets` 成员判定 + `buildOpenSearchTargets` |

### 3.8 NXIN 域名 / CORS / API 基址（P0+）

| 文件 | 定制内容 |
|------|----------|
| `internal/router/router.go` | CORS 白名单 `zsk.nxin.com` / `zsk.t.nxin.com`；`AllowHeaders` 含 `X-Open-Retrieve-Api-Key` |
| `frontend/vite.config.ts` | `allowedHosts: ['zsk.t.nxin.com', 'zsk.nxin.com', '.nxin.com']` |
| `frontend/src/utils/request.ts` | 默认 API `zsk.t.nxin.com:8080` |
| `docker-compose.yml` | `MINIO_BUCKET_NAME` 默认 `nxinweknora` |

### 3.9 HTTPS 进程内 TLS（P0+）

| 文件 | 定制内容 |
|------|----------|
| `config/config.yaml` | `server.https.enabled: true` + `ssl/cert.pem` |
| `internal/config/config.go` | `HTTPSConfig` 校验；`CASConfig` / `NXINCASAuthConfig` / `OpenRetrieveConfig` |
| `scripts/generate-ssl-cert.ps1` / `.sh` | 自签证书生成 |

与 CAS `auth.nxin_cas_auth.require_https: true` 联动。

### 3.10 颜色统一化 theme.css（P1）

> 设计文档：[颜色统一化改造](./2026-02-05-颜色统一化改造.md)

| 文件 | 说明 |
|------|------|
| `frontend/src/assets/theme/theme.css` | 470+ 行 CSS 变量，`--wk-*` 自定义变量 |
| 51+ 业务组件 | 770+ 处 `var(--td-*)`（`Input-field.vue`、`menu.vue`、`AgentSelector.vue` 等） |

与 SVG `theme` 预设联动，合并时 **变量表优先保留本地扩展**。

### 3.11 前端权限工具与成员搜索（P1）

| 文件 | 说明 |
|------|------|
| `frontend/src/utils/kb-permission.ts` | `canEditKB` / `canDeleteKB` 等（合并后推广引用） |
| `internal/application/repository/kb_member.go` | 成员搜索：`email` / `username` / `cas_real_name` |
| `frontend/src/views/knowledge/settings/KnowledgeBaseMembers.vue` | 成员管理 UI |

### 3.12 共享知识库前端附属（P1）

| 文件 | 说明 |
|------|------|
| `frontend/src/stores/menu.ts` | 侧边栏「知识库广场」菜单项 |
| `frontend/src/router/index.ts` | `/platform/shared-knowledge-bases` 路由 |
| `frontend/src/i18n/locales/*.ts` | `menu.sharedKnowledgeBaseSquare` 多语言 |
| `frontend/src/views/knowledge/KnowledgeBaseEditorModal.vue` | 创建/编辑共享 KB（`5fa1767e` 流程） |
| `frontend/src/views/knowledge/KnowledgeBaseList.vue` | 三源列表合并 + 去重 |
| `frontend/src/components/menu.vue` | `shared-kb` 图标映射 + NXIN Logo |

### 3.13 设置页 API 信息门控（P1）

| 文件 | 说明 |
|------|------|
| `frontend/src/views/settings/Settings.vue` | `showApiInfo`：仅 `tenantId <= 10001` 显示 API 信息页 |

### 3.14 用户菜单品牌化（P1）

| 文件 | 说明 |
|------|------|
| `frontend/src/components/UserMenu.vue` | 隐藏 API 文档 / 官网 / GitHub 外链（CAS 登出逻辑见 §3.1） |

### 3.15 存储配额策略（P1）

| 文件 | 说明 |
|------|------|
| `migrations/versioned/feature/0000014_update_storage_quota_default.up.sql` | 租户默认配额 10GB → 1GB |

### 3.16 NXIN 配置结构（P1）

| 文件 | 配置段 |
|------|--------|
| `config/config.yaml` | `cas`、`auth.nxin_cas_auth`、`open_retrieve`、`server.https` |
| `internal/config/config.go` | 对应结构体与默认值校验 |

### 3.17 P2 关注项（按部署环境验证，不阻断合并）

| 类别 | 文件 | 关注时机 |
|------|------|----------|
| 测试环境编排 | `docker-compose.test.yml` | Phase 5 测试部署 |
| Windows 开发 | `internal/sandbox/local_windows.go` | 本地 Windows 构建 |
| ParadeDB | `migrations/paradedb/*`、`scripts/fix_bm25_partial_index.sql` | 使用 ParadeDB 时 |
| 合并工具 | `scripts/merge-execute.ps1`、`merge-check.ps1`、`resolve-conflicts.ps1` | 合并执行期间 |
| SSL / 部署 | `scripts/generate-ssl-cert.*`、`scripts/deploy_test.sh` | 部署阶段 |
| 用户导入 | `scripts/import_user_fix.sql`、`check_password_hash.sql` | 数据迁移时 |

### 3.18 跟随上游（无需单独保护）

| 文件/变更 | 处理方式 |
|-----------|----------|
| `knowledgebase_search_results.go` / `chat_pipeline/search.go` | 仅格式化 diff，跟随上游 |
| `GeneralSettings.vue` 主题切换 | 上游已具备 |
| `docs/**`（40+ 篇） | 随仓库保留 |
| `retriever/qdrant|weaviate` | 跟随上游 |

---

## 四、高风险冲突文件预判

基于本地 198 文件 diff 与上游 v0.6.x CHANGELOG，以下文件在合并中 **必然或极可能冲突**：

| 优先级 | 文件 | 冲突原因 | 解决策略 |
|--------|------|----------|----------|
| ⭐⭐⭐⭐⭐ | `internal/container/container.go` | 上游可能重写 DI/Redis 初始化 vs `initRedisClient` 集群分支 | 保留 `REDIS_MODE` 双模式，建议提取独立函数 |
| ⭐⭐⭐⭐⭐ | `internal/router/task.go` | asynq 连接配置变更 vs `RedisClusterClientOpt` | 保留 `getAsynqRedisConnOpt` 集群分支 |
| ⭐⭐⭐⭐ | `internal/middleware/auth.go` | 上游 RBAC + OIDC 大改 vs `tryNXINCASAuth` + Redis 缓存 | 先合上游，再追加 CAS 兜底；保留 Redis 缓存读写 |
| ⭐⭐⭐⭐⭐ | `internal/router/router.go` | 路由暴增（Wiki/RBAC/Integrations） vs CAS/open 路由 | 合并双方路由注册函数，提取 `RegisterNXINRoutes()` |
| ⭐⭐⭐⭐⭐ | `internal/application/service/session_knowledge_qa.go` | 检索管道多次重构 vs `SearchKnowledgeOpen` | 保留 `SearchKnowledgeOpen`，适配新 pipeline 插件接口 |
| ⭐⭐⭐⭐⭐ | `internal/handler/knowledgebase.go` | 上游 CRUD 大改 vs 共享 KB 扩展 | 上游结构 + 本地共享方法（join/leave/members） |
| ⭐⭐⭐⭐ | `internal/handler/knowledge.go` | `effectiveTenantID` 跨租户逻辑 | 合并权限解析，接入上游 per-KB ownership |
| ⭐⭐⭐⭐ | `internal/config/config.go` | 新增大量配置段 | 保留 `CASConfig`/`NXINCASAuthConfig`/`OpenRetrieveConfig` |
| ⭐⭐⭐⭐ | `config/config.yaml` | 配置结构变化 | 三向合并：上游默认 + NXIN 域名/CAS/open_retrieve |
| ⭐⭐⭐⭐ | `frontend/src/router/index.ts` | 上游路由重构 vs CAS 守卫 | 保留 `beforeEach` CAS 逻辑 |
| ⭐⭐⭐⭐ | `frontend/src/views/knowledge/KnowledgeBaseList.vue` | 上游 KB 列表大改 vs 三源合并 + SvgIcon | 上游 UI + 本地三源去重 + 保留 `SvgIcon` |
| ⭐⭐⭐⭐ | `frontend/src/components/menu.vue` | v0.6.3 侧边栏大改 vs SvgIcon + NXIN Logo | 上游功能/布局 + 本地 `SvgIcon` 与品牌区 |
| ⭐⭐⭐⭐ | `frontend/src/components/AgentSelector.vue` | 上游 Agent 选择器重构 vs SvgIcon | 上游交互 + 本地图标组件 |
| ⭐⭐⭐⭐ | `frontend/src/components/icons/*` | 上游无此目录，易被误删 | **整目录保留**，不做任何覆盖 |
| ⭐⭐⭐ | `frontend/src/assets/theme/theme.css` | 上游 Settings 重构 vs 本地 CSS 变量 | 合并变量表，保留 `--wk-*` 自定义变量 |
| ⭐⭐⭐ | `frontend/src/views/chat/components/AgentStreamDisplay.vue` | v0.6.3 chat 流式大改 vs SvgIcon | 上游流式布局 + 本地图标引用 |
| ⭐⭐⭐ | `migrations/versioned/*` | 编号冲突（本地 032 vs 上游 059） | **禁止**改动上游迁移编号；`feature/` 目录保持独立 |
| ⭐⭐⭐ | `go.mod` | Go 1.24 → 1.26 | 跟随上游升级 Go 工具链 |
| ⭐⭐⭐ | `docker-compose*.yml` | 服务编排变化（MCP profile、Langfuse） | 保留 NXIN 镜像源、`REDIS_MODE`/`REDIS_CLUSTER_ADDRS` 等 env |
| ⭐⭐⭐⭐ | `frontend/vite.config.ts` | 上游构建配置变更 vs `allowedHosts` | 保留 NXIN 域名白名单 |
| ⭐⭐⭐⭐ | `frontend/src/utils/request.ts` | API 基址逻辑变更 | 保留 `zsk.t.nxin.com` 默认值 |
| ⭐⭐⭐ | `frontend/src/views/settings/Settings.vue` | v0.6.1 Settings 大改 vs `showApiInfo` | 上游 UI + 本地租户门控 |

---

## 五、分阶段实施计划

### Phase 0：准备与环境（1–2 天）

#### Task 0.1：建立升级分支与备份

**操作**

```powershell
cd E:\Tencent\WeKnora-slj
git status                                    # 确保工作区干净
git branch backup/2026-07-09-v0.3.6-pre-upgrade   # 备份当前 main
git fetch upstream --tags
git checkout -b upgrade/v0.3.6-to-v0.6.3
```

**验收标准**
- [ ] 备份分支已创建
- [ ] `upstream` 已 fetch，`v0.6.3` Tag 可见（`git tag -l 'v0.6.*'`）

#### Task 0.2：NXIN 差异以升级分支为准（不再落盘 patch）

> **2026-07-17 更新**：原计划导出 `patches/2026-07-09/nxin-extensions.patch` /
> `nxin-svg-usage.patch` 已取消并删除。Windows 下 `git diff` 重定向易导致中文注释乱码，
> 且补丁不参与构建/运行；NXIN 定制以分支 `upgrade/v0.3.6-to-v0.6.3` 上的源码与提交历史为准。
>
> 若需临时对照上游差异，直接查看（勿写入仓库）：
> `git diff upstream/main...HEAD -- <paths>`

**验收标准**
- [x] 不依赖仓库内 `patches/` 目录；差异可通过 `git diff` / 分支提交复现

#### Task 0.3：建立扩展模块边界（建议提前重构）

将 NXIN 扩展收口到明确入口，降低后续冲突面：

```
internal/extensions/nxin/
├── cas/              # §3.1
├── shared_kb/        # §3.2
├── open_retrieve/    # §3.3
├── kb_access/        # §3.6 统一 effectiveTenantID 解析（新建）
├── redis/            # §3.5 initRedisClient 集群逻辑（从 container.go 提取）
└── routes.go         # §3.8 CORS + §3.1 CAS + §3.3 open 路由

frontend/src/extensions/nxin/   # 或保持现有路径
├── icons/            # §3.4
├── theme/            # §3.10 theme.css
└── deploy/           # §3.8 request.ts、vite allowedHosts
```

> Phase 0 建议完成扩展目录收口；后续合并只需维护 `extensions/nxin/` 入口。

#### Task 0.4：测试环境基线记录

| 场景 | 记录方式 |
|------|----------|
| CAS 登录（test/production） | 录屏 + 保存 JWT/Cookie 样例 |
| 共享 KB 广场 join/leave | API 请求/响应快照 |
| 组织共享 KB 访问 | 跨租户检索结果快照 |
| open_retrieve 召回 | `POST /api/v1/open/knowledge/retrieve` 基准响应 |
| 现有知识库文档检索 | 固定 query 的 top-K 结果 |
| SVG 组件化 | 侧边栏/KB 列表/Agent 列表截图（light + dark 主题） |
| Redis 集群 | 记录 `REDIS_MODE=cluster` 下 Ping、CAS 缓存、asynq 任务投递结果 |
| CORS / 跨域 | 从 `zsk.t.nxin.com` 调 API 截图 |
| 共享 KB 检索 | 加入共享 KB 后固定 query 的 top-K 快照 |

**Checkpoint-0**
- [ ] NXIN 差异可在升级分支上用 `git diff` / 提交历史复现（无需 `patches/`）
- [ ] 基线测试用例文档化（可复用 `docs/api/` 中现有接口说明）
- [ ] 测试/生产数据库已备份

---

### Phase 1：合并 Tag `v0.4.0`（2–3 天）

#### Task 1.1：阶梯合并

```powershell
git merge v0.4.0 --no-commit --no-ff
```

**重点关注**
- VectorStore 新表（`000032_vector_stores`）与本地 `000032_add_video_info_to_chunks` 编号冲突 → 以**上游编号为准**，本地 feature 迁移不受影响
- `internal/handler/knowledge.go` 附件处理新参数
- 前端 `KnowledgeBaseList` 首次大改

#### Task 1.2：解决冲突（按优先级）

1. `migrations/versioned/` — 采用上游
2. `internal/router/router.go` — 合并路由
3. `internal/container/container.go` — **保留 Redis 集群双模式**
4. `internal/router/task.go` — **保留 asynq 集群连接**
5. `internal/middleware/auth.go` — 保留 CAS 兜底
6. `config/config.yaml` — 三向合并
7. 前端知识库相关 Vue 组件
8. `frontend/src/components/icons/` — **整目录保留，不合并覆盖**

#### Task 1.3：验证

```powershell
go build ./cmd/server
cd frontend; npm run type-check; npm run build
go test ./internal/middleware/... -run OpenRetrieve
```

**Checkpoint-1（必须通过才可进入 Phase 2）**
- [ ] 后端编译通过
- [ ] 前端 type-check / build 通过
- [ ] CAS 登录流程正常
- [ ] 共享 KB 广场列表可访问
- [ ] open_retrieve 接口返回结构与基线一致（允许分数微小浮动）
- [ ] 侧边栏/KB 列表 `SvgIcon` 正常渲染，light/dark 主题下图标颜色跟随
- [ ] `REDIS_MODE=cluster` 下服务启动成功，asynq 任务可投递
- [ ] 自 Tag `v0.3.6` 迁移至 `v0.4.0` 成功

---

### Phase 2：合并 Tag `v0.5.2`（3–4 天）

#### Task 2.1：合并 `v0.5.0` → `v0.5.2`

```powershell
# 若 Phase 1 停在 v0.4.0，可先合并 v0.5.0、v0.5.1，或直接合并检查点 Tag：
git merge v0.5.2 --no-commit --no-ff
```

**本阶段最大风险：RBAC 与租户模型**

| 检查项 | 操作 |
|--------|------|
| RBAC 角色定义 | 确认 CAS AutoBind 创建用户时分配默认角色 |
| 每 KB 所有权 | `knowledge_bases.owner_id` 与上游 ownership 字段映射 |
| 审计日志 | 不影响扩展功能，但需验证写入 |
| 凭证 AES 加密 | 检查 `config.yaml` 中 model API key 加密配置 |
| 多向量库 fan-out | 验证共享 KB 跨租户检索仍指向正确向量库 |

#### Task 2.2：共享知识库适配

```go
// 权限解析伪代码 — 合并后必须保持
func resolveKBAccess(ctx, kbID, userID) (effectiveTenantID, role, error) {
    if isOwner(kbID, userID)       → return ownerTenant, "owner", nil
    if orgShare.CanAccess(kbID)   → return sourceTenant, orgRole, nil
    if directShare.GetRole(kbID)  → return sourceTenant, memberRole, nil
    if agentShare.CanAccess(kbID) → return sourceTenant, "agent", nil
    return "", "", ErrForbidden
}
```

**待修复项（合并前已知）**
- `KnowledgeBaseList` 三源列表按 `kb.id` 去重（owner 优先）
- `kb-permission.ts` 统一替换 `orgStore.canEditKB` 调用

#### Task 2.3：验证

**Checkpoint-2**
- [ ] RBAC 下 CAS 用户可正常访问被授权 KB
- [ ] 组织共享 + 直接共享双模式端到端通过
- [ ] 审计日志不影响现有 API 响应时间（< 50ms 增量）
- [ ] open_retrieve 限流（`rate_limit_qps`）仍生效

---

### Phase 3：合并 Tag `v0.6.3`（4–5 天）

#### Task 3.1：合并 `v0.6.0` → `v0.6.3`

```powershell
# 检查点可直接合并目标 Tag；亦可持续合并 v0.6.0、v0.6.1、v0.6.2 后至 v0.6.3
git merge v0.6.3 --no-commit --no-ff
```

**本阶段重点消化**

| 变更 | 扩展适配要点 |
|------|-------------|
| `process_config` 替代 `enable_multimodal` | 上传/检索配置读取路径变更；open_retrieve 读 KB 默认配置 |
| 解析 Trace Timeline | 不影响 open_retrieve，但影响文档入库监控 |
| OpenSearch 驱动 | 若环境使用 OpenSearch，需在测试环境验证 |
| 内置模型 YAML | `config/builtin_models.yaml` 新增，合并时保留 NXIN 模型配置 |
| Jaeger 移除 / Langfuse only | 更新 `docker-compose` 监控配置 |
| Go 1.26 | 升级本地 Go 工具链 |
| chat/agent 流式重构 | 不影响 open_retrieve（非流式），但影响 QA 回归 |
| HNSW 索引 `000059` | 生产迁移需低峰期执行，关注 1024 维向量 |

#### Task 3.2：session_knowledge_qa.go 适配

`SearchKnowledgeOpen` 必须重新对接 v0.6.x 检索管道：

```
buildOpenSearchTargets()
  → 按 KB/Knowledge ID 解析 tenant_id（无用户权限校验）
  → 调用新版 HybridSearch / chat_pipeline.PluginSearch
  → 适配 rerank threshold、progress events（若暴露给三方）
```

#### Task 3.3：前端 CAS 与新版 Settings 共存

- 上游 Settings 面板完全重构 → CAS 环境变量注入方式不变
- 上游新增 `auth logout clear session resource caches` → `cas.ts` logout 需调用新清理逻辑
- 保留 NXIN Logo / 主题 CSS 变量
- 合并 v0.6.3 新 `menu` 组件时，逐项检查 `SvgIcon` 引用未丢失（见 3.4 已接入组件清单）

#### Task 3.4：数据库迁移终检

```
执行顺序：
1. 上游 versioned/000000 – 000059（按序号）
2. 本地 feature/0000013 – 0000017（NXIN 定制，独立目录）
```

**验收标准**
- [ ] 新库初始化成功
- [ ] 自 Tag `v0.3.6` 生产库升级至 `v0.6.3` 成功（先在测试库演练）
- [ ] `users.cas_*` 字段保留
- [ ] `knowledge_base_members` 表保留
- [ ] `knowledge_bases.visibility` 等扩展字段保留

**Checkpoint-3**
- [ ] 全量编译通过（Go 1.26 + 前端 build）
- [ ] 全量迁移测试通过
- [ ] 全部 P0 / P0+ / P1 保护项回归通过（见第六节）

---

### Phase 4：扩展层加固与质量收敛（2–3 天）

#### Task 4.1：补充自动化测试

| 测试 | 文件 | 覆盖场景 |
|------|------|----------|
| CAS 中间件 | `internal/middleware/cas_auth_test.go`（新建） | Cookie 有效/无效/HTTPS 要求 |
| open_retrieve | 已有 `open_retrieve_auth_test.go` | 扩展：无效 KB ID、限流 |
| 共享 KB 权限 | `internal/application/service/shared_kb_test.go`（新建） | join/leave/跨租户读 |

#### Task 4.2：配置与文档同步

- 更新 `config/config.yaml` 示例（含 NXIN 三项配置）
- 更新 `docs/api/2026-04-15-用户端、模型端知识库、知识接口说明.md`
- 在 `docs/2026-02-11-项目升级文档索引.md` 追加本文档链接

#### Task 4.3：性能与兼容性验证

| 指标 | 基线 | 可接受偏差 |
|------|------|-----------|
| open_retrieve P95 延迟 | 记录基线 | < +20% |
| CAS validate P95 | 记录基线 | < +30%（含 Redis 缓存） |
| 共享 KB 广场列表 | 记录基线 | 功能等价 |
| 向量检索 top-5 重合率 | ≥ 80% | 允许排序微调 |

---

### Phase 5：部署与灰度（2–3 天）

#### Task 5.1：测试环境部署

参照 `docs/2026-01-27-测试生产环境部署方案.md`：

1. 构建新镜像（app + docreader + frontend）
2. 执行数据库迁移（先 `000033–000059`，再 `feature/`）
3. 更新 `config/config.yaml`（CAS / open_retrieve / nxin_cas_auth）
4. 冒烟测试全部 P0 / P0+ / P1 保护项

#### Task 5.2：生产灰度

```
阶段 A（1 天）：仅测试环境全量验证
阶段 B（1 天）：生产部署，CAS 登录 + 只读接口验证
阶段 C（1 天）：开放 open_retrieve 流量（监控 QPS/错误率）
阶段 D：      全量切换，保留 backup 分支可回滚
```

**回滚策略**

```powershell
# 代码回滚
git checkout backup/2026-07-09-v0.3.6-pre-upgrade
# 数据库：提前备份，迁移脚本必须配套 .down.sql
# 镜像：保留上一版 docker tag
```

---

## 六、回归测试清单

### 6.1 CAS 认证

| # | 场景 | 步骤 | 预期 |
|---|------|------|------|
| C1 | 前端 SSO 登录 | 无 Token 访问前端 → 跳转 CAS → 回调 | 获得 JWT，进入首页 |
| C2 | Cookie 兜底 API | 带 `_cas_sid/_cas_uid` 无 Bearer 调 `GET /api/v1/knowledge-bases` | 200，返回用户 KB 列表 |
| C3 | HTTPS 要求 | `require_https: true` 下 HTTP 请求 | CAS 兜底拒绝，返回 401 |
| C4 | Redis 缓存 | 连续两次 Cookie 请求 | 第二次命中缓存，延迟降低 |
| C5 | 登出 | 点击退出 | CAS Cookie 清除 + 上游 session cache 清除 |
| C6 | 新用户 AutoBind | 首次 CAS 登录未知用户 | 自动创建用户/租户，写入 `cas_user_id` |

### 6.2 共享知识库

| # | 场景 | 步骤 | 预期 |
|---|------|------|------|
| S1 | 创建共享 KB | `POST /api/v1/knowledge-bases/shared` | visibility=shared，owner 正确 |
| S2 | 广场列表 | `GET /api/v1/knowledge-bases/shared` | 分页返回公开共享 KB |
| S3 | 加入/离开 | join → 访问知识 → leave | 加入后可读，离开后 403 |
| S4 | 成员管理 | 添加/修改/删除成员角色 | 角色生效 |
| S5 | 组织共享 | 通过 KBShareSettings 共享到组织 | 组织成员可访问 |
| S6 | 跨租户检索 | 在会话中选择共享 KB 提问 | 召回共享 KB 内容 |
| S7 | 列表去重 | 同时通过组织+直接共享的 KB | 列表只显示一条 |
| S8 | IM 通道 | IM 机器人访问共享 KB（若启用） | v0.6.1 synthetic identity 生效 |

### 6.3 开放检索 open_retrieve

| # | 场景 | 步骤 | 预期 |
|---|------|------|------|
| O1 | 正常召回 | 有效 API Key + 合法 KB ID + query | 返回 SearchResult 列表 |
| O2 | 无 Key | 不带 `X-Open-Retrieve-Api-Key` | 401 |
| O3 | 错误 Key | 无效 Key | 403 |
| O4 | 限流 | 超过 `rate_limit_qps` | 429 |
| O5 | 禁用开关 | `open_retrieve.enabled: false` | 404 或 403 |
| O6 | 无用户权限校验 | 用 open API 访问他人 KB | 允许（设计行为），结果正确 |
| O7 | 多 Key 轮换 | 使用 `api_keys` 中任一 Key | 均可用 |
| O8 | 管道升级后 | v0.6.3 下相同 query | 与基线 top-5 重合率 ≥ 80% |

### 6.4 前端 SVG 组件化

| # | 场景 | 步骤 | 预期 |
|---|------|------|------|
| I1 | 侧边栏图标 | 打开首页，检查 `menu.vue` 各导航项 | 图标正常显示，无破损 `<img>` |
| I2 | 主题跟随 | 切换 light / dark / 跟随系统 | `SvgIcon` 颜色随 `--td-text-color-*` 变化 |
| I3 | theme 预设 | 检查 `docInfo` 中 `theme="brand"` | 图标显示品牌色 |
| I4 | variant 变体 | 检查 KB 列表 `variant="green"` 组织图标 | 绿色变体正确 |
| I5 | KB 列表 | 打开知识库列表页 | 组织来源 `SvgIcon` 正常 |
| I6 | Agent 选择器 | 打开对话页 Agent 选择器 tooltip | 组织/用户图标正常 |
| I7 | 流式对话 | 触发 Agent 工具调用流式展示 | `AgentStreamDisplay` 内图标正常 |
| I8 | 品牌 Logo | 检查侧边栏顶部 | NXIN Logo 显示正确 |
| I9 | 无冗余 SVG | `grep -r '\.svg"' frontend/src/views` | 业务组件无回退为 `<img src="xxx-green.svg">` |
| I10 | 注册表完整 | 遍历 `IconName` 类型各 name 渲染一次 | 13 个图标均可渲染 |

### 6.5 Redis 集群配置

| # | 场景 | 步骤 | 预期 |
|---|------|------|------|
| R1 | 集群启动 | `REDIS_MODE=cluster` + 有效 `REDIS_CLUSTER_ADDRS` 启动服务 | 启动成功，`Ping` 通过 |
| R2 | 集群缺失地址 | `REDIS_MODE=cluster` 但 `REDIS_CLUSTER_ADDRS` 为空 | 启动失败，明确报错 |
| R3 | 单机兼容 | `REDIS_MODE=single` 或空 + `REDIS_ADDR` | 与升级前行为一致 |
| R4 | CAS 缓存 | 集群模式下 Cookie 兜底鉴权两次相同请求 | 第二次命中 Redis 缓存 |
| R5 | asynq 任务 | 集群模式下上传文档触发异步解析 | 任务入队并成功执行 |
| R6 | IM 限流 | 集群模式下 IM 通道高频消息（若启用） | 分布式限流生效，无 panic |
| R7 | 上下文存储 | 多轮对话后检查 Redis `context:*` key | 会话上下文正常读写 |
| R8 | docker-compose | 检查 `docker-compose.yml` env 注入 | `REDIS_MODE`/`REDIS_CLUSTER_ADDRS` 仍存在 |
| R9 | Helm 部署 | K8s 部署后检查 Pod env | 集群变量已注入（升级后需补充） |
| R10 | 双模式切换 | 同一二进制分别用 single/cluster 配置启动 | 均正常，无需重新编译 |

### 6.6 P0+ / P1 补充回归

| # | 场景 | 步骤 | 预期 |
|---|------|------|------|
| X1 | CORS 跨域 | 从 `zsk.t.nxin.com` 调 API | CORS 通过，Cookie 可携带 |
| X2 | 共享 KB 会话检索 | 加入共享 KB 后在会话中提问 | `buildSearchTargets` 包含该 KB |
| X3 | 共享 KB pin | 对共享 KB 执行 pin | 返回 `ErrSharedKnowledgeBasePinNotAllowed` |
| X4 | 成员 CAS 姓名搜索 | 成员管理搜 `cas_real_name` | 可搜到对应成员 |
| X5 | API 信息门控 | tenantId > 10001 进入设置 | 不显示 API 信息页 |
| X6 | HTTPS + CAS | HTTPS 下 Cookie 兜底请求 | `require_https` 校验通过 |
| X7 | 主题切换 | light / dark 切换 | 侧边栏/KB 列表颜色跟随 |
| X8 | 存储配额 | 新租户默认配额 | 为 1GB（1073741824 字节） |

---

## 七、风险与缓解

| 风险 | 影响 | 概率 | 缓解措施 |
|------|------|------|----------|
| `auth.go` 合并失败导致 CAS 失效 | 全站无法登录 | 高 | 扩展逻辑独立函数 + 自动化 C1–C6 |
| `session_knowledge_qa.go` 合并后 open_retrieve 返回空 | 三方集成中断 | 高 | 基准 query 快照对比 + O8 |
| 数据库迁移编号冲突 | 部署失败 | 中 | feature/ 独立目录；先在测试库演练 |
| 共享 KB 权限回归 | 数据越权或无法访问 | 高 | S1–S8 全覆盖；权限单测 |
| 前端大改导致 CAS 路由守卫失效 | 前端白屏/死循环 | 中 | 保留 `router.beforeEach` 独立测试 |
| `menu.vue` 合并后 SvgIcon 被覆盖 | 图标破损/主题色失效 | 高 | `icons/` 整目录保护 + I1–I10 回归 |
| 上游回退 `<img src="*.svg">` | 主题适配丢失、冗余 SVG 复活 | 中 | 合并后 `grep` 检查 + 禁止引入 `-green`/`-grey` 变体 |
| `initRedisClient` 被上游覆盖 | 生产 Redis Cluster 无法连接 | 高 | 提取独立函数 + R1–R10 双模式回归 |
| Helm 缺少集群 env | K8s 部署回退单机配置 | 中 | Phase 3 补充 `helm/templates/app.yaml` |
| `effectiveTenantID` 链断裂 | 共享 KB 跨租户读写失败 | 高 | §3.6 提取 `kb_access/resolver` + X2 |
| CORS 白名单被删 | 前端无法调 API | 高 | §3.8 保护 + X1 |
| `buildSearchTargets` 成员判定丢失 | 广场加入的 KB 无法检索 | 高 | §3.7 专项对比 + X2 |
| Settings 大改冲掉 `showApiInfo` | 非授权租户可见 API 信息 | 中 | §3.13 + X5 |
| `theme.css` 变量被覆盖 | UI 颜色混乱 | 中 | §3.10 合并优先保留 `--wk-*` + X7 |
| Go 1.26 工具链不兼容 | 编译失败 | 低 | CI 先行验证 |
| HNSW 迁移锁表 | 生产检索中断 | 中 | 低峰期执行；提前评估表大小 |
| 上游继续发版 | 方案过时 | 中 | 锁定目标 tag `v0.6.3`，升级完成后再跟进 v0.6.4+ |

---

## 八、工作量估算

| 阶段 | 预估工时 | 人力 |
|------|----------|------|
| Phase 0 准备 | 1–2 天 | 1 后端 + 0.5 运维 |
| Phase 1 v0.4.0 | 2–3 天 | 1 后端 + 1 前端 |
| Phase 2 v0.5.2 | 3–4 天 | 1 后端 + 1 前端 |
| Phase 3 v0.6.3 | 4–5 天 | 1 后端 + 1 前端 |
| Phase 4 质量收敛 | 2–3 天 | 1 后端 + 1 测试 |
| Phase 5 部署灰度 | 2–3 天 | 1 运维 + 1 后端 |
| **合计** | **14–20 天** | 2–3 人 |

---

## 九、执行命令速查

```powershell
# 1. 开始升级（对齐 Tencent/WeKnora Tags）
git fetch upstream --tags
git tag -l "v0.6.*"    # 确认 v0.6.3 存在
git checkout -b upgrade/v0.3.6-to-v0.6.3
git branch backup/2026-07-09-v0.3.6-pre-upgrade

# 2. 分阶段合并（按 Tag）
git merge v0.4.0 --no-commit --no-ff   # Phase 1
# git merge v0.5.2 --no-commit --no-ff # Phase 2
# git merge v0.6.3 --no-commit --no-ff # Phase 3

# 3. 冲突解决辅助（已有脚本）
.\scripts\merge-execute.ps1 -DryRun          # 预览
.\scripts\resolve-conflicts.ps1              # 辅助解决
.\scripts\merge-check.ps1                    # 合并后检查

# 4. 构建验证
go build -o server.exe ./cmd/server
cd frontend; npm ci; npm run type-check; npm run build

# 5. 迁移测试（测试库）
# 参考 docker-compose.test.yml 启动后执行迁移

# 6. 扩展功能快速验证
go test ./internal/middleware/... -run OpenRetrieve -v
curl -X POST https://zsk.t.nxin.com:8080/api/v1/open/knowledge/retrieve `
  -H "X-Open-Retrieve-Api-Key: <key>" `
  -H "Content-Type: application/json" `
  -d '{"query":"猪口蹄疫症状","knowledge_base_ids":["<kb_id>"]}'
```

---

## 十、决策记录

| 决策 | 选择 | 理由 |
|------|------|------|
| 合并策略 | 按 Tag 阶梯 `v0.4.0` → `v0.5.2` → `v0.6.3` | Tag 跨度过大，禁止一次 merge 至 `v0.6.3` |
| 扩展代码组织 | 建议收口 `internal/extensions/nxin/` | 降低后续持续跟进上游的成本 |
| 迁移策略 | 上游 versioned + 本地 feature 双轨 | 避免编号冲突，便于独立回滚 NXIN 迁移 |
| 目标 Tag 锁定 | `v0.6.3` | 上游当前最新 Release Tag；完成后可再评估 `v0.6.4+` |
| OIDC 与 CAS | 并存 | 上游 OIDC 保留；NXIN 场景继续走 CAS |
| 共享 KB 模式 | 双模式继续共存 | 业务已依赖直接成员共享，组织共享为上游能力 |
| open_retrieve | 保持独立 API Key 体系 | 与用户 JWT / 租户 Key 安全边界分离 |
| SVG 组件化 | 保留 `icons/` 目录 + `SvgIcon` 引用 | 上游无等价方案；回退 `<img>` 会破坏主题适配 |
| 主题 CSS | 与 SVG 联动保护 | `SvgIcon` 的 `theme` 预设依赖 `theme.css` 变量 |
| Redis 集群 | 保留双模式 env + `UniversalClient` | 生产依赖 Cluster；合并禁止回退单机-only 实现 |
| Helm Redis | 升级后补充集群 env | 当前 `app.yaml` 缺 `REDIS_MODE`/`REDIS_CLUSTER_ADDRS` |
| 保护分级 | P0/P0+/P1 入主清单，P2 关注 | 见 §3.0；P2 不阻断合并 |
| kb_access 收口 | Phase 0 提取 `kb_access/resolver.go` | 降低 §3.6 跨 Handler 权限链冲突 |
| CORS / 域名 | 硬编码保留，后续可配置化 | §3.8；待确认事项 #7 |
| theme.css | P1 合并保护 | 与 SVG 联动；冲突时保留 `--wk-*` 变量 |
| 存储配额 | P1 保留 feature 迁移 0000014 | 新租户默认 1GB |
| P2 脚本 | 关注不阻断 | §3.17；合并/部署阶段按需使用 |

---

## 十一、相关文档

- [升级内容清单](./2026-07-09-WeKnora-slj-v0.3.6-to-v0.6.3升级内容清单.md)（执行勾选跟踪表）
- [项目升级与合并指南](./2026-02-11-项目升级与合并指南.md)
- [代码合并执行方案](./2026-04-01-代码合并执行方案.md)
- [CAS 单点登录集成细化方案](./2026-01-15-CAS单点登录集成细化方案.md)
- [共享知识库权限与逻辑处理方案](./2026-01-26-共享知识库权限与逻辑处理方案.md)
- [开放检索 API 文档](./api/open-knowledge-retrieve.md)
- [三方接口说明](./api/2026-04-15-用户端、模型端知识库、知识接口说明.md)
- [自定义 SVG 组件化方案](./2026-02-12-自定义SVG组件化方案.md)
- [颜色统一化改造](./2026-02-05-颜色统一化改造.md)
- [图标主题适配升级](./2026-02-13-图标主题适配升级.md)
- 上游 [CHANGELOG](https://github.com/Tencent/WeKnora/blob/main/CHANGELOG.md)

---

## 十二、待确认事项

1. **生产向量库类型**：PostgreSQL pgvector / ParadeDB / OpenSearch？影响 v0.6.1 OpenSearch 驱动是否启用及 HNSW 迁移方案。
2. **是否启用上游 RBAC 细粒度权限**：若启用，CAS AutoBind 用户的默认角色/权限组需产品确认。
3. **升级窗口**：HNSW 索引迁移（`000059`）是否可接受短暂只读？
4. **目标部署环境**：继续 `zsk.nxin.com` 私有镜像，还是同步上游官方镜像构建方式？
5. **v0.6.3 新功能取舍**：网站嵌入组件、Integrations Center、Wiki 等是否在本次升级后对外开放？
6. **Redis 部署模式**：测试环境是否也需要集群模式验证，还是仅生产 `cluster` + 测试 `single`？
7. **CORS 白名单策略**：v0.6.3 上游 CORS 若改为可配置，是否将 `zsk.nxin.com` 迁入配置文件而非硬编码？

---

## 十三、附录：保护项与合并阶段映射

> 完整保护清单见 **§三**；本节仅保留阶段对照，便于执行时快速查阅。

| 合并阶段 | P0 / P0+ 关注 | P1 关注 | P2 关注 |
|----------|---------------|---------|---------|
| Phase 0 | 扩展目录收口（§3.0 Task 0.3）；差异以升级分支为准（不再落盘 patch） | 基线截图（主题/CORS） | 合并脚本就绪 |
| Phase 1 (`v0.4.0`) | Redis 集群、CORS、CAS | `theme.css`、`KnowledgeBaseList` | — |
| Phase 2 (`v0.5.2`) | `effectiveTenantID` 链、RBAC 对齐 | `kb-permission.ts` 推广 | — |
| Phase 3 (`v0.6.3`) | `session_knowledge_qa`、HTTPS、Helm Redis | `Settings.showApiInfo`、UserMenu 品牌化 | `docker-compose.test.yml` |
| Phase 4–5 | 全量 §6 回归 | 存储配额验证 | ParadeDB 脚本（若适用） |

**分析来源**：`git diff upstream/main...HEAD`（198 文件），2026-07-09 梳理。

---

*本文档为升级规划，不包含代码修改。确认 §十二待确认事项后，可按 Phase 0 开始执行。*
