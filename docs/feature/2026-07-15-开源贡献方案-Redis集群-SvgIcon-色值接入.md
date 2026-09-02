# 开源贡献方案：Redis 集群 / SvgIcon / 色值接入

> **文档日期**：2026-07-15  
> **基线版本**：本地仓库基于 WeKnora `v0.6.3`（当前约 `v0.6.3-12-g…`），贡献基线为 `upstream/main`  
> **上游远程**：`https://github.com/Tencent/WeKnora.git`（`upstream`）  
> **Fork**：`https://github.com/sunlingjun/WeKnora.git`（`origin`）  
> **状态**：设计已确认；**执行顺序已调整**（见 §9）  
> **取代关系**：在 v0.6.3 基线上重写贡献边界；2 月文档（如 `2026-02-13-贡献流程配置与待贡献版本方案.md`）仅作历史参考，冲突以本文为准

---

## 0. 执行闸门（2026-07-15 确认）

**先本地、后贡献。** 在 `upgrade/v0.3.6-to-v0.6.3` 收口为「本地合并版」并完成构建/P0 保护验证之前，**不启动** §2 起的三条 `contrib/*` 分支与上游 PR。

本地收口计划见：[`2026-07-15-本地v0.6.3合并版收口计划.md`](./2026-07-15-本地v0.6.3合并版收口计划.md)。

---

## 1. 目标与原则

将农信本地 fork 中已验证、且适合上游的三项能力，以**三个独立 PR**回馈 [Tencent/WeKnora](https://github.com/Tencent/WeKnora)：

| 顺序 | PR | 一句话 |
|------|-----|--------|
| 1 | Redis 集群配置（部署完整集） | 可选 Cluster，默认单机行为不变，并保留上游 Redis TLS |
| 2 | SvgIcon 本地抽象（仅核心） | 业务自定义 SVG 组件化 + 首批替换 |
| 3 | 统一品牌色（仅色值接入方式） | 硬编码 → `var(--td-*)`，**保留上游绿系色板** |

**总原则**

1. **干净分支**：一律从 `upstream/main` 新建分支，手工移植/cherry-pick 相关 hunk，不从带 CAS/农信蓝板的本地合并分支直接拆分。
2. **串行合入**：Redis → SvgIcon → 色值接入，降低冲突与 review 负担。
3. **公私边界**：不贡献农信 CAS 认证、农信蓝品牌色板、`tdesign-icon-offline` 离线护栏（可另开 Issue/PR）。
4. **色板事实**：上游默认**绿系**（如 `#07c05f`）；农信本地为**蓝系**（主色 `#366ef4` / 6 阶，hover `#618dff` / 5 阶，[2026-09-02 已采用](./2026-09-02-品牌色配色决策.md)）。贡献色值接入时 fallback 用绿，不得把蓝板推上游。

---

## 2. 总流程

```text
git fetch upstream
  │
  ├─ contrib/redis-cluster        → PR #1 → 合入 main
  │
  ├─ contrib/svg-icon             → PR #2（可 rebase 最新 main）→ 合入
  │
  └─ contrib/td-color-tokens      → PR #3（建议在 #2 之后）→ 合入
```

**日常同步 Fork**

```powershell
git checkout main
git fetch upstream
git merge upstream/main
git push origin main
```

**单条功能分支模板**

```powershell
git fetch upstream
git checkout -b contrib/<name> upstream/main
# 手工移植改动 → 本地验证 → push origin → 对 Tencent/WeKnora 开 PR
```

---

## 3. PR-1：Redis 集群配置（部署完整集）

### 3.1 目标

在保持上游默认单机（及 Lite：无 `REDIS_ADDR`）行为的前提下，增加可选 Redis Cluster；应用连接与 asynq 使用同一套开关；部署侧（compose / Helm / `.env.example`）可配置。

### 3.2 分支与文件

- **分支**：`contrib/redis-cluster`
- **主要触及**：
  - `internal/container/container.go` — `initRedisClient` 双模式
  - `internal/router/task.go` — asynq `RedisClusterClientOpt` / 单机 Opt
  - `.env.example`
  - `docker-compose.yml`
  - `helm/templates/app.yaml`、`helm/values.yaml`
  - 简短使用说明（README 小节或 `docs/` 一篇）

### 3.3 环境变量契约

| 变量 | 单机（默认） | 集群 |
|------|--------------|------|
| `REDIS_MODE` | 空 / `single` | `cluster` |
| `REDIS_ADDR` | 必填（空则 Lite） | 忽略 |
| `REDIS_CLUSTER_ADDRS` | 不用 | 必填，逗号分隔 `host:port` |
| `REDIS_USERNAME` / `REDIS_PASSWORD` | 共用 | 共用 |
| `REDIS_DB` | 可用 | 集群下忽略 |

### 3.4 上游 Redis TLS（必须保留）

上游已支持客户端 TLS（约 PR #1930），通过 `common.RedisTLSConfig()` 挂到 `redis.Options.TLSConfig` / asynq Opt：

| 变量 | 含义 |
|------|------|
| `REDIS_USE_TLS=true` | 开启 TLS（默认关，明文） |
| `REDIS_TLS_SERVER_NAME` | 证书校验 / SNI |
| `REDIS_TLS_INSECURE_SKIP_VERIFY=true` | 跳过校验（仅开发） |

**贡献要求**：单机与集群路径均设置 `TLSConfig: common.RedisTLSConfig()`。本地现有集群实现若缺 TLS，移植时不得丢弃上游能力。

### 3.5 实现要点

- `initRedisClient` 返回 `redis.UniversalClient`；`cluster` → `NewClusterClient`，否则单机 `NewClient`。
- 仍需 `*redis.Client` 的 DI 点用断言/unwrap，禁止无关全仓类型抖动。
- Helm `values` 注释去掉「NXIN」等私有表述。
- **不做**：从 `container.go` 拆独立包、CAS 专用 Redis 逻辑。

### 3.6 验收

- [ ] 空 `REDIS_MODE` + `REDIS_ADDR`：行为与上游一致
- [ ] `REDIS_MODE=cluster` + 有效 `REDIS_CLUSTER_ADDRS`：Ping 通，asynq 可投递
- [ ] `cluster` 且地址为空：启动失败且错误明确
- [ ] `REDIS_USE_TLS=true` 时单机/集群均带上 TLSConfig
- [ ] compose / Helm / `.env.example` 可配置且文档可读

### 3.7 建议 PR 标题

`feat(redis): optional Redis Cluster mode with compose/Helm support`

---

## 4. PR-2：SvgIcon 本地抽象（仅核心）

### 4.1 目标

业务自定义 SVG：统一组件 + 注册表，`currentColor` 跟主题色，消除多份同图标不同色静态文件。

### 4.2 分支与交付物

- **分支**：`contrib/svg-icon`
- **新增**：
  - `frontend/src/components/icons/SvgIcon.vue`
  - `frontend/src/components/icons/registry.ts`
  - `frontend/src/components/icons/index.ts`
- **首批替换（控制 diff）**：优先 `menu.vue`、`AgentSelector.vue`、`AgentStreamDisplay.vue`、`deepThink.vue`、`Input-field.vue` 等与上游可对比的页面；其余调用点可后续 PR。

### 4.3 组件契约

| Prop | 说明 |
|------|------|
| `name` | 注册表图标名 |
| `size` | 默认 20 |
| `color` | 可选覆盖色 |
| `theme` | `default` / `brand` / `secondary` / `placeholder` / `anti` → 对应 `--td-text-color-*` / `--td-brand-color` |
| `variant` | 线宽等样式变体；颜色仍走 theme/color |

路径数据保持中性，不写死农信蓝。

### 4.4 明确不做

- `tdesign-icon-offline.ts`、本地 `public/tdesign-icons/` sprite、`index.html` CDN 阻断  
  （解决的是 TDesign `<t-icon>` 拉 `tdesign.gtimg.com` 的问题，与业务 SvgIcon 正交，另议）
- 全仓 SVG 必换
- 本 PR 内大范围硬编码色替换（属 PR-3）

### 4.5 验收

- [ ] 亮/暗主题下首批页面图标颜色跟随 token
- [ ] 无缺失图标、无明显布局错位
- [ ] PR 描述注明「不含 TDesign CDN 离线护栏」

### 4.6 建议 PR 标题

`feat(frontend): add SvgIcon registry for theme-aware custom icons`

---

## 5. PR-3：统一品牌色（仅色值接入方式）

### 5.1 目标

组件硬编码颜色 → TDesign CSS 变量，亮/暗主题可走通；**不替换上游绿系色板为农信蓝**。

### 5.2 色板约定

| | 上游 | 农信本地（不贡献） |
|--|------|-------------------|
| 品牌主色 | 绿 `#07c05f` 阶 | 蓝 `#366ef4` 阶（hover `#618dff`） |
| `theme.css` 色板 token 数值 | 原则上保持上游 | 继续本地蓝板 |

Fallback 示例：`var(--td-brand-color, #07c05f)`，禁止贡献蓝 fallback。

### 5.3 分支与范围

- **分支**：`contrib/td-color-tokens`（建议 SvgIcon 合入后再开）
- **映射**：沿用 `docs/2026-02-05-颜色统一化改造.md` 中文本/背景/边框/品牌/功能色/透明度对照，面向上游绿
- **对象**：业务 `.vue` / `.less` / `.css`；按 Settings / Chat / Knowledge / 公共组件分批 commit
- **`theme.css`**：默认不提交农信蓝板；仅当上游缺变量或暗色明显有洞时做最小增量

### 5.4 保留不硬改

- Agent 模式区分色、Avatar 渐变等非主题语义色
- 农信蓝整板切换
- 离线 CDN 相关改动

### 5.5 移植纪律

- 只挑「硬编码 → `var(--td-*)`」hunk
- 本地蓝 fallback 改为绿或删除
- 已含 SvgIcon 的文件只动样式色值块

### 5.6 验收

- [ ] 亮/暗切换观感仍为上游**绿系**
- [ ] 抽查 Settings / 菜单 / 知识库无大面积遗漏硬编码
- [ ] `theme.css` diff 无农信蓝 1–10 色阶替换

### 5.7 建议 PR 标题

`refactor(frontend): replace hardcoded colors with TDesign CSS variables`

---

## 6. 不在本次贡献范围

| 项 | 原因 |
|----|------|
| 农信 CAS / 组织权限私有逻辑 | 非上游通用产品能力 |
| 农信蓝 `theme.css` 色板 | 品牌定制，非开源默认 |
| `tdesign-icon-offline` + 本地 sprite | 与 SvgIcon 正交；可单独对上游提 Issue/PR |
| 从本地「大合并分支」直接切三条 feature | 易夹带无关 diff |

---

## 7. 与历史文档关系

| 文档 | 关系 |
|------|------|
| `2026-02-12-自定义SVG组件化方案.md` | SvgIcon API 仍可用；本方案收窄为「仅核心 + 首批页面」 |
| `2026-02-05-颜色统一化改造.md` | 映射表可用；明确**绿 fallback**、不上蓝板 |
| `2026-02-13-贡献流程配置与待贡献版本方案.md` | remote 角色仍适用；版本拆分与顺序以本文为准 |
| `2026-07-09-WeKnora-slj-v0.3.6-to-v0.6.3升级方案.md` | Redis 双模式与合并风险说明仍有效；贡献时另叠加上游 TLS |

---

## 8. 决策记录（摘要）

| 决策点 | 选择 |
|--------|------|
| PR 拆分 | 三个独立 PR |
| 合入顺序 | Redis → SvgIcon → 色值接入 |
| 色值贡献内容 | 仅接入方式，保留上游绿系 |
| Redis 范围 | 部署完整集（代码 + compose/Helm/env 文档） |
| SvgIcon 范围 | 仅核心，不含离线护栏 |
| 分支来源 | 从 `upstream/main` 干净分支手工移植 |

---

## 9. 下一步

1. ~~评审人确认本文无异议~~（§1–§3 已确认）。  
2. **闸门**：完成[本地 v0.6.3 合并版收口](./2026-07-15-本地v0.6.3合并版收口计划.md)（WIP 落地、编译、P0 保护、可选打 Tag）。  
3. 收口通过后：编写三条 PR 的《贡献实施计划》，再开 `contrib/redis-cluster` 等分支。
