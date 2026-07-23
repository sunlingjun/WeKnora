# 2026-07-20 WeKnora-slj 合并 v0.7.0 后：NXIN 定制补全清单

> **目的**：指导「官方 Tag → WeKnora-slj 合并」之后，如何核对并补回 NXIN 定制，避免再次出现 CAS 进 `/login`、Linux 编不过、CORS 漏端口等问题。  
> **基线分支**：`stable/2026-07-20-nxin-v0.7.0`（merge 官方 `v0.7.0` + 本清单补丁）  
> **下游**：NXIN-WEKNORA 只引用 slj 的 `nxin-v0.7.0-stable.*`，再叠部署差分（Dockerfile GOPROXY / DuckDB 预下载 / Jenkins / compose）。

---

## 1. 合并后必做核对（P0）

合并冲突解决、能编译之后，**先跑此表再打 Tag**。

| # | 检查项 | 期望位置 / 行为 | 本次状态 |
|---|--------|-----------------|----------|
| 1 | **CAS 路由守卫** | `frontend/src/router/index.ts`：非 Lite 未登录 → `casStore.validateSession()`，**禁止**直接 `next('/login')` | ✅ 已补 |
| 1b | **知识库广场路由** | `shared-knowledge-bases` → `SharedKnowledgeBaseSquare.vue`；`knowledge-bases/:kbId/members` 须在 `/:kbId` 前 | ✅ 已补 |
| 1c | **网页导入标题** | `KbUploadSourceDropdown` 必填 title + URL 预填；`KnowledgeBase`/`createKnowledgeFromURL` 传 `title` | ✅ 已补 |
| 1d | **卡片品牌色取色** | `KnowledgeBaseList`/`AgentList`/`OrganizationList` 勿用硬编码 `rgba(7,192,95)`/`rgba(0,82,217)`，统一 `color-mix(... var(--td-brand-color) ...)`；共享空间 infinity 图标用 `currentColor` | ✅ 已补 |
| 1e | **品牌文案 ZSK** | `index.html`→`NXIN-ZSK`；i18n 欢迎/首页；`config/prompt_templates/*`：`You are ZSK` + `developed by Nxin`（勿改 WeKnoraCloud/API 技术标识） | ✅ 已补（`24b7d999`；slj `975f98fd`） |
| 1f | **共享知识库成员侧栏** | `KnowledgeBaseEditorModal` 的 `navGroups`「发布集成」必须 `pickItems(['members','share'])`（见 `kbEditorNavGroups.ts`）；仅写 `share` 会导致成员入口静默丢失 | ✅ 已补（2026-07-23） |
| 1f2 | **共享知识库成员 i18n** | `knowledgeList.members.*` + `knowledgeList.messages.fetchMembersFailed/roleUpdated/...` 四语齐全；合并官方 locale 时勿冲掉（见 `kbMembersI18n.test.ts`） | ✅ 已补（2026-07-23） |
| 1g | **CreateSharedKnowledgeBase UUID** | `shared_kb.go`：空 `id` 必须 `uuid.New()`，并写 `TenantID`/`CreatorID`/时间戳；否则第二次创建撞 `knowledge_bases_pkey` | ✅ 已补（2026-07-23，`dev`） |
| 1h | **本空间 KB 列表可见性** | 对齐官方 v0.7.0：同空间成员可读全量；广场已加入勿标成「本空间·其他成员」 | ✅ 已补（`b60abf70` / `311c0893`） |
| 2 | **CAS 公开路径** | `frontend/src/utils/request.ts`：`PUBLIC_AUTH_PATHS` 含 `/api/v1/cas/`；`.nxin.com` 上 401 回 `/` 触发守卫 | ✅ 已补 |
| 3 | **CAS 退出** | `UserMenu.vue`：`.nxin.com` → `casStore.logout()` | ✅ 已补 |
| 4 | **CAS 后端** | `handler/cas_auth.go`、`middleware.tryNXINCASAuth`、`RegisterCASRoutes`、`config nxin_cas_auth` | ✅ merge 已保留 |
| 5 | **KB 静态路由顺序** | `/knowledge-bases/user`、`/shared` **在** `/:id` **之前** | ✅ merge 已保留 |
| 6 | **CORS** | `url.Parse` + `Hostname()` 放行 `*.nxin.com`；`AllowHeaders` 含 `pd`/`systemid` 等 | ✅ 已补强 |
| 7 | **open_retrieve** | middleware + routes + config | ✅ merge 已保留 |
| 8 | **跨平台 listen** | `cmd/server/sockopt_unix.go` + `sockopt_windows.go`，`listen.go` 调 `setReuseAddr`（勿在共用文件写 `syscall.Handle`） | ✅ 已补 |
| 9 | **前端 API base** | `api-base.ts` 读 `VITE_APP_BASE_API`，**不硬编码** `zsk.t.nxin.com:8080` | ✅ 已补 |
| 10 | **前端 env 模板** | `frontend/env.development.example` / `env.production.example`（含 `VITE_APP_CAS`） | ✅ 已补 |

---

## 2. 建议一并补齐（P1）

| # | 检查项 | 说明 | 本次状态 |
|---|--------|------|----------|
| 1 | 品牌外链隐藏 | `UserMenu`：`showUpstreamMenuLinks = false` | ✅ 已补 |
| 2 | Redis 集群 asynq | 勿仅依赖 `REDIS_ADDR` | ✅ 已有 |
| 3 | ParadeDB | compose 主文件 `v0.22.2-pg17`；`docker-compose.test.yml` 合并时对照 | ⚠ 合并后复核 test compose |
| 4 | DuckDB / GOPROXY | **优先留在 NXIN 部署仓** Dockerfile；slj 可不强制默认国内代理 | 部署侧 |

---

## 3. 仅 NXIN 部署仓保留（不要强行并进 slj 业务线）

| 项 | 原因 |
|----|------|
| Jenkins / `deploy.sh` fail-fast | 环境运维 |
| Dockerfile `GOPROXY` + DuckDB curl 预下载 | 国内构建机 |
| `.env.test` / `.env.prod` 真实密钥 | 密钥不出库 |
| `cmd/migrate-cas-user-password` | 一次性运维工具 |
| 大量 `docs/2026-*` 运维笔记 | 部署知识库 |

---

## 4. 冲突解决时的「保底口诀」

```
官方改了 Login / router.beforeEach？
  → 合完后立刻对照 NXIN 旧版，把 CAS 分支接回去（未登录 ≠ /login）

官方改了 CORS AllowOriginFunc？
  → 用 Hostname()，不要 HasSuffix(origin, ".nxin.com")（带端口会漏）

官方改了 listen / SO_REUSEADDR？
  → 拆 sockopt_unix.go / sockopt_windows.go，Linux 构建不能出现 syscall.Handle

官方改了 api-base / 环境变量名？
  → 保留 VITE_APP_CAS / VITE_APP_APP / VITE_APP_BASE_API

出现 /knowledge-bases/:id 吃掉 user|shared？
  → 静态路由永远写在参数路由前面

官方改了 prompt_templates 或 i18n 欢迎语？
  → 跑 scripts/2026-07-21-restore-zsk-brand-copy.py：WeKnora/Tencent → ZSK/Nxin；勿动 WeKnoraCloud

官方改了知识库/智能体/共享空间卡片样式？
  → 禁止硬编码 rgba(7,192,95)/rgba(0,82,217)；统一 color-mix(--td-brand-color)
  → OrganizationList infinity 图标用 inline SVG + currentColor（勿用 organization-green.svg 硬绿）
  → 可跑 scripts/2026-07-21-fix-org-brand-color.py / fix-card-brand-color.py

官方改了 KnowledgeBaseEditorModal 侧栏为 navGroups？
  → 「发布集成」必须含 members（在 share 前）；用 kbEditorNavGroups.ts
  → 验收：共享库 + is_owner → 侧栏出现「成员」；npx tsx --test kbEditorNavGroups.test.ts
  → 同时核对 knowledgeList.members.* 四语文案（npx tsx --test kbMembersI18n.test.ts）

官方改了 CreateSharedKnowledgeBase / shared_kb？
  → 空 id 必须生成 UUID + TenantID；勿再空字符串落主键
```

---

## 5. 建议的本地验收（打 Tag 前）

```bash
# 后端
go build -o server.exe ./cmd/server

# 前端（复制 env 模板）
cp frontend/env.development.example frontend/.env.development
# 按需改 VITE_IS_DOCKER / proxy
cd frontend && npm run build_dev   # 或现有脚本

# 浏览器
# 访问业务首页（非 /login）→ 应跳转 cas.t.nxin.com
# 日志无 tenant_api_keys_key_hash_key（见占位 hash 修正文档）
```

---

## 6. 与升级主方案的关系

主流程仍见：`docs/2026-07-20-升级至v0.7.0-WeKnora-slj与NXIN先后方案.md`。  
本文件是 **Phase A 收尾检查表**：merge 完成 ≠ 可打 Tag；过完 §1 P0 再 `nxin-v0.7.0-stable.*`。

库侧占位 hash：`docs/2026-07-20-tenant_api_keys占位hash修正-本机测试生产.md`。
