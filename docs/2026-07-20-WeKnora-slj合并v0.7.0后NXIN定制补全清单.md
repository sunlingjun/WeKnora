# 2026-07-20 WeKnora-slj 合并 v0.7.0 后：NXIN 定制补全清单

> **目的**：指导「官方 Tag → WeKnora-slj 合并」之后，如何核对并补回 NXIN 定制，避免再次出现 CAS 进 `/login`、Linux 编不过、CORS 漏端口等问题。  
> **基线分支**：`stable/2026-07-20-nxin-v0.7.0`  
> **稳定 Tag（2026-07-30 重指向）**：`nxin-v0.7.0-stable.1` → `68a800ea`（含业务回补 + `/files` CAS 放行 + 构建/compose 收口）  
> **下游模型（2026-07-30 起）**：WeKnora-slj = **唯一合并基线**（官方升级 + 自研特性 + 构建加固）。NXIN-WEKNORA **简单合入**该 Tag/分支，**不再**在 NXIN 分支回灌业务/Dockerfile/compose；NXIN 仅保留密钥 env 与机房脚本。

---

## 1. 合并后必做核对（P0）

合并冲突解决、能编译之后，**先跑此表再打 Tag**。

| # | 检查项 | 期望位置 / 行为 | 本次状态 |
|---|--------|-----------------|----------|
| 1 | **CAS 路由守卫** | `frontend/src/router/index.ts`：非 Lite 未登录 → `casStore.validateSession()`，**禁止**直接 `next('/login')` | ✅ 已补 |
| 1b | **知识库广场路由** | `shared-knowledge-bases` → `SharedKnowledgeBaseSquare.vue`；`knowledge-bases/:kbId/members` 须在 `/:kbId` 前 | ✅ 已补 |
| 1c | **网页导入标题** | `KbUploadSourceDropdown` 必填 title + URL 预填；`KnowledgeBase`/`createKnowledgeFromURL` 传 `title` | ✅ 已补 |
| 1d | **卡片品牌色取色** | `KnowledgeBaseList`/`AgentList`/`OrganizationList` 勿用硬编码 `rgba(7,192,95)`/`rgba(0,82,217)`，统一 `color-mix(... var(--td-brand-color) ...)`；共享空间 infinity 图标用 `currentColor` | ✅ 已补 |
| 1e | **品牌文案 ZSK** | `index.html`→`NXIN-ZSK`；i18n 欢迎/首页；`config/prompt_templates/*`：`You are ZSK` + `developed by Nxin`（勿改 WeKnoraCloud/API 技术标识） | ✅ 已补 |
| 1f | **共享知识库成员侧栏** | `KnowledgeBaseEditorModal` 的 `navGroups`「发布集成」必须 `pickItems(['members','share'])`（见 `kbEditorNavGroups.ts`） | ✅ 已补 |
| 1f2 | **共享知识库成员 i18n** | `knowledgeList.members.*` + messages 四语齐全（见 `kbMembersI18n.test.ts`） | ✅ 已补 |
| 1f3 | **共享知识库广场 i18n** | 四语必须有顶层 `sharedKbSquare.*`（title/subtitle/searchPlaceholder/join/noDescription/fetchFailed 等）以及 `knowledgeList.sharedTag` / `leave` / `role.*` / `sections.joinedShared` / `messages.joinedSuccess`。官方 prune 会整棵删掉 `sharedKbSquare`，且 `localeKeyAudit` 扫描正则只匹配**现有**顶层 namespace，删掉后 `$t('sharedKbSquare.*')` 扫不到、审计仍绿。合完必须跑 `sharedKbI18n.test.ts`；`CRITICAL_LOCALE_KEYS` 已列入保底 | ✅ 已补 |
| 1f4 | **创建共享知识库** | `KnowledgeBaseEditorModal` 基本信息须有可见性单选；`visibility==='shared'` 时走 `createSharedKnowledgeBase`（`POST /knowledge-bases/shared`）。官方编辑器只 `createKnowledgeBase`，合完会变成只能建个人库。锁：`kbEditorNavGroups.test.ts` + `knowledgeEditor.basic.visibilityLabel` | ✅ 已补 |
| 1m | **农信用户导入路由** | `routes_auth_tenant.go`：`POST /tenants/:id/members/cas-import[/preview]` 必须挂在 `AddMember` 之后、`/:user_id` 之前。handler 在仓内不等于路由在；官方拆 router 后这两条会丢。锁：`router_cas_import_test.go` | ✅ 已补 |
| 1g | **CreateSharedKnowledgeBase UUID** | `shared_kb.go`：空 `id` 必须 `uuid.New()` | ✅ 已补 |
| 1h | **本空间 KB 列表可见性** | 同空间成员可读全量；广场已加入勿标成「本空间·其他成员」 | ✅ 已补 |
| 1i | **CAS X-Tenant-ID 切空间** | `middleware/auth.go`：`tryNXINCASAuth` 经 `resolveNXINCASTargetTenant`，与 JWT 分支同权 | ✅ 已补（`2057b6dc`，自 NXIN 回补） |
| 1j | **CAS 放行 `/files`** | `config.yaml` `nxin_cas_auth.allowed_path_globs` 含 `/api/v1/*` 与 `/files`（聊天引用图） | ✅ 已补（`6fa5c16b`） |
| 1k | **Redis Cluster SSE** | `stream.NewStreamManager(rdb UniversalClient)`，禁止另起 `redis.NewClient(REDIS_ADDR)` | ✅ 已补（`2057b6dc`） |
| 1l | **CAS 本地密码规则工具** | `internal/utils/cas_local_password.go`（规则源进基线） | ✅ 已补 |
| 2 | **CAS 公开路径** | `request.ts`：`PUBLIC_AUTH_PATHS` 含 `/api/v1/cas/` | ✅ 已补 |
| 3 | **CAS 退出** | `UserMenu.vue`：`.nxin.com` → `casStore.logout()` | ✅ 已补 |
| 4 | **CAS 后端** | `handler/cas_auth.go`、`tryNXINCASAuth`、`RegisterCASRoutes`、`nxin_cas_auth` | ✅ 已保留 |
| 5 | **KB 静态路由顺序** | `/knowledge-bases/user`、`/shared` **在** `/:id` **之前** | ✅ 已保留 |
| 6 | **CORS** | `Hostname()` 放行 `*.nxin.com`；企联网头 | ✅ 已补强 |
| 7 | **open_retrieve** | middleware + routes + config | ✅ 已保留 |
| 8 | **跨平台 listen** | `sockopt_unix.go` / `sockopt_windows.go` | ✅ 已补 |
| 9 | **前端 API base** | `api-base.ts` 读 `VITE_APP_BASE_API` | ✅ 已补 |
| 10 | **前端 env 模板** | `env.development.example` / `env.production.example`（含 `VITE_APP_CAS`） | ✅ 已补 |
| 11 | **构建加固** | `Dockerfile.app`：GOPROXY 多源、`GODEBUG=http2client=0`、DuckDB curl 预下载、无 BuildKit mount | ✅ 已收口进 slj（`68a800ea`） |
| 12 | **DuckDB LOAD-first** | `cmd/download/duckdb/duckdb.go` | ✅ 已收口进 slj |
| 13 | **compose CAS_ENVIRONMENT** | `docker-compose.yml` 透传 `CAS_ENVIRONMENT`；测试/生产拓扑 depends_on/profile 与现网一致 | ✅ 已收口进 slj |
| 14 | **进程内 HTTPS** | `config.yaml` 默认注释掉 https（Nginx 终结 TLS） | ✅ 已收口进 slj |

---

## 2. 建议一并补齐（P1）

| # | 检查项 | 说明 | 本次状态 |
|---|--------|------|----------|
| 1 | 品牌外链隐藏 | `UserMenu`：`showUpstreamMenuLinks = false` | ✅ 已补 |
| 2 | Redis 集群 asynq | 勿仅依赖 `REDIS_ADDR` | ✅ 已有 |
| 3 | ParadeDB | `docker-compose.test.yml` 对齐 `v0.22.2-pg17` | ✅ 已收口 |
| 4 | 品牌/配额脚本 | `scripts/2026-07-21-verify-zsk-brand.py` 等 | ✅ 已收口 |

---

## 3. 仅 NXIN 部署仓保留（密钥与机房脚本）

升 0.7.1 起：**禁止**再把业务/Dockerfile/compose 差分只留在 NXIN。

| 项 | 原因 |
|----|------|
| `.env.test` / `.env.prod` / `frontend/.env.*` 真实密钥 | 密钥不出库 |
| Jenkins / 服务器 `deploy.sh` fail-fast | 机房运维脚本，可不进 Git 或仅 NXIN 私有 |
| 一次性运维 SQL / `migrate-cas-user-password` | 运维工具 |
| 环境专属运维笔记（可选） | 部署知识库；**合并基线说明仍写在 slj `docs/`** |

**NXIN 合入方式**：`fetch` slj 稳定 Tag → merge/ff 到 `dev`/`master` → 保留本地未跟踪的 `.env*` → 构建部署。无业务冲突预期。

---

## 4. 冲突解决时的「保底口诀」

```
官方改了 Login / router.beforeEach？
  → 合完后立刻把 CAS 分支接回去（未登录 ≠ /login）

官方改了 CORS AllowOriginFunc？
  → 用 Hostname()，不要 HasSuffix(origin, ".nxin.com")

官方改了 listen / SO_REUSEADDR？
  → 拆 sockopt_unix.go / sockopt_windows.go

官方改了 api-base / 环境变量名？
  → 保留 VITE_APP_CAS / VITE_APP_APP / VITE_APP_BASE_API

出现 /knowledge-bases/:id 吃掉 user|shared？
  → 静态路由永远写在参数路由前面

官方改了 prompt_templates 或 i18n 欢迎语？
  → ZSK/Nxin 品牌；勿动 WeKnoraCloud

官方 prune i18n？
  → zh-CN 必须保留 sharedKbSquare 整树 + knowledgeList.sharedTag/leave/role/sections.joinedShared/messages.joinedSuccess
  → 合完跑 kbMembersI18n.test.ts 与 sharedKbI18n.test.ts
  → 勿只信 localeKeyAudit：命名空间被删后扫描正则扫不到 $t('sharedKbSquare.*')

官方改了知识库编辑器创建流程？
  → 保留可见性单选；shared 走 createSharedKnowledgeBase，不要只 POST /knowledge-bases

官方拆了 routes_auth_tenant.go？
  → 立刻把 POST /members/cas-import 与 /preview 挂回 Owner+ 组

官方改了 StreamManager / Redis 初始化？
  → 保持 NewStreamManager(UniversalClient)，Cluster 勿直连 REDIS_ADDR

官方改了 nxin_cas_auth / Auth 中间件？
  → 保留 X-Tenant-ID resolve；allowed_path_globs 含 /files

官方改了 Dockerfile / duckdb 下载？
  → 保留 GOPROXY 多源、curl 预下载、LOAD-first、legacy builder 兼容
```

---

## 5. 建议的本地验收（打 Tag 前）

```bash
go build -o server.exe ./cmd/server
go test ./internal/stream/ ./internal/utils/ -count=1

# 前端（复制 env 模板）
cp frontend/env.development.example frontend/.env.development
cd frontend && npm run build_dev
npx tsx --test src/views/knowledge/kbMembersI18n.test.ts src/views/knowledge/sharedKbI18n.test.ts src/i18n/localeKeyAudit.test.ts

# 浏览器：CAS 首页、切空间、聊天引用图、共享 KB、图谱（Neo4j）
```

---

## 6. 与升级主方案的关系

- 0.7.0 主流程：`docs/2026-07-20-升级至v0.7.0-WeKnora-slj与NXIN先后方案.md`
- **0.7.1 主流程**：`docs/2026-07-30-升级至v0.7.1-WeKnora-slj与NXIN先后方案.md`
- **0.7.2 主流程**：`docs/2026-08-14-升级至v0.7.2-WeKnora-slj与NXIN先后方案.md`
- 本文件是 **合并基线核对表**：merge 完成 ≠ 可打 Tag；过完 §1 P0 再 `nxin-vX.Y.Z-stable.*`

---

## 7. 2026-07-30 Tag 收口记录

| 提交 | 说明 |
|------|------|
| `2057b6dc` | 自 NXIN 回补：CAS X-Tenant-ID、Stream Cluster、cas_local_password |
| `6fa5c16b` | CAS `allowed_path_globs` 增加 `/files` |
| `68a800ea` | Dockerfile/DuckDB/compose/https-off/脚本收口进 slj |
| Tag | `git tag -f nxin-v0.7.0-stable.1` → `68a800ea`（若远端已有旧 Tag，需 `git push --force origin nxin-v0.7.0-stable.1`） |
