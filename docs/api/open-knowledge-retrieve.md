# 开放检索 API（无登录）

[返回目录](./README.md)

## 概述

该接口用于三方系统在**无用户登录态**下，按知识库 ID 或知识 ID 检索知识分片（chunk），用于外部模型上下文拼接。  
该接口与登录态的 `/api/v1/knowledge-search` 语义不同：不做用户归属/可见性校验。

**召回策略（平台默认）**：向量检索 ∥ 关键词检索 → RRF 融合 →（可选）MODEL Rerank → 按 `match_count` 截断返回。  
配置优先级：**本次请求参数 > 系统默认（`open_retrieve` / 平台级固定默认）**。

> **不使用租户 `RetrievalConfig`。** 开放检索需支持跨用户、跨库（乃至跨租户资源 ID）一次召回；若回落到某一租户的检索配置，会导致多库行为不一致、隐性绑定「主租户」、跨调用方不可复现。租户配置仅作用于登录态 `/knowledge-search` 等用户域接口。  
> 底层仍按资源记录中的真实 `tenant_id` **访问索引与模型服务**，与「策略参数不取租户默认」互不冲突。

> 策略依据与分阶段落地见内部分析；本文为对外/对内统一的**请求体与响应契约**。  
> 端到端（chatbot / aisi 技能）范围收窄见：[`2026-09-05-open-retrieve-scope-and-e2e.md`](../../../chatbot-service/docs/sdd/knowledge/design/2026-09-05-open-retrieve-scope-and-e2e.md)。

---

## 接口信息

- **方法**: `POST`
- **路径**: `/api/v1/open/knowledge/retrieve`
- **认证**: 不要求用户登录；使用服务端预先发放的 **开放检索专用 API Key**（与用户 `X-API-Key` 区分，避免混用）
- **Content-Type**: `application/json`

### 请求头（必填）

| 头字段 | 说明 |
|--------|------|
| `X-Open-Retrieve-Api-Key` | 授权 API Key，与部署配置 `open_retrieve.api_key` 或 `open_retrieve.api_keys[]` 之一匹配即可 |

### 查询参数（可选）

| 参数 | 说明 |
|------|------|
| `resource_urls` | `handle`（默认）或 `public`。`public` 时将 `content` / `image_info` 内的 `resource://` 引用改写为可直链加载的 `https://…`，并在每条 `data[]` 上附加 `images` 数组（供会话 UI 回显）。需部署配置 `APP_EXTERNAL_URL` 等公共 URL 能力。与登录态 `knowledge-search?resource_urls=public` 语义一致。 |

---

## 请求参数

### 基础字段（已实现 / 保持兼容）

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `query` | string | 是 | 用户问题或检索语句 |
| `knowledge_base_ids` | string[] | 否 | 整库检索范围 |
| `knowledge_ids` | string[] | 否 | 指定知识检索范围 |
| `match_count` | int | 否 | **最终返回条数上限 N3**。未传时用当前 `profile` 缺省（balanced/lexical=`20`，precise=`5`）。服务端上限保护建议 ≤ 100。**不**控制向量/关键词候选池大小 |

> 约束：`knowledge_base_ids` 与 `knowledge_ids` 至少提供一个。

### 范围语义（钉死）

开放检索对范围字段的处理是 **多 target 并集检索，再全局融合 / 精排 / 按 `match_count` 截断**，**不是**「先在库内再限文档」的交集。

| 入参组合 | 服务端行为 |
|----------|------------|
| 仅 `knowledge_base_ids` | 每个有效库 → 整库 target |
| 仅 `knowledge_ids` | 按文档解析父库 → **文档级** target（仅这些知识） |
| **两者都有** | 整库 target ∪ 文档级 target（见下「折叠」规则），结果进入**同一**候选池后全局 TopK |
| 无效 ID | 跳过并继续；全部无效 → `200` + `data: []`，建议记入 `meta.skipped_ids` |

**折叠规则**：若某 `knowledge_id` 的父库已出现在本次 `knowledge_base_ids` 中，则该文档 id **不再单独建 target**（视为已被整库覆盖）。仅当父库**不在**整库列表中时，才追加文档级 target。

**文档级阈值**：文档级（及显式收窄）target 应对齐登录态检索，**关闭召回阈值**（等价 `DisableRecallThresholds`），避免显式指定的文档因向量/关键词阈值被滤空后，只剩整库噪声占满 TopK。

**调用方推荐（产品语义）**：

| 场景 | 推荐传参 |
|------|----------|
| 本轮已明确指定文档（如会话 `@` 文档） | **只传** `knowledge_ids`，不要把绑定大库并进同一次请求 |
| 本轮只指定库（`@` 知识库） | **只传**该库的 `knowledge_base_ids` |
| 无本轮指定、仅数字人绑定库 | 传绑定 `knowledge_base_ids` |
| 确需「绑定库 + 本轮文档」同搜 | 可同时传两字段（并集）；须接受大库可能挤占 TopK。优先改调用方分两次请求或先文档后兜底 |

> **禁止**将「两字段同时出现」理解为交集或「在库内过滤文档」。跨系统对接（AI 大脑 / 技能）的范围收窄约定见同级仓库 chatbot-service：[`docs/sdd/knowledge/design/2026-09-05-open-retrieve-scope-and-e2e.md`](../../../chatbot-service/docs/sdd/knowledge/design/2026-09-05-open-retrieve-scope-and-e2e.md)。

### 召回策略字段（升级契约）

以下字段均为**可选**。未传时一律使用下方**系统默认**（部署配置 `open_retrieve.*` 或平台固定值），**不读取**任一目标库所属租户的 `RetrievalConfig`。实现可按阶段分批上线；客户端可先按契约联调。

| 字段 | 类型 | 系统默认 | 说明 |
|------|------|----------|------|
| `profile` | string | `balanced` | 一键参数包，见 [Profile](#profile-参数包)。与细粒度字段同时出现时：**细粒度字段覆盖 profile 展开值** |
| `retrieval_mode` | string | `hybrid` | `hybrid` \| `vector` \| `keyword` \| `auto`（`auto` = `hybrid` 别名） |
| `dense_top_k` | int | `40` | **N1** 向量候选池大小（建议 1–100） |
| `keyword_top_k` | int | `40` | **N2** 关键词候选池大小（建议 1–100） |
| `fusion` | object | 见下 | 多路融合配置；仅 `retrieval_mode=hybrid` 时生效 |
| `enable_rerank` | bool | `false` | 是否对融合后候选做 MODEL Rerank。开放面**默认关闭**（控延迟与费用） |
| `rerank_model_id` | string | 系统配置 | `enable_rerank=true` 时生效；未传则用 `open_retrieve.rerank_model_id`（平台级）。**禁止**回落到某一租户的 Rerank 模型，以免跨库结果依赖「首个 ID 的租户」 |
| `score_threshold` | number | 见 [生效条件](#score_threshold-生效条件) | 仅在 Rerank 成功且 `score_type=rerank` 时过滤；否则忽略。`precise` 未传时系统缺省 `0.2` |
| `tag_ids` | string[] | `[]` | 标签预过滤（检索前收窄候选） |
| `metadata_filter` | object | 无 | 元数据预过滤，见 [metadata_filter](#metadata_filter) |
| `expand_parent` | bool | `true`（系统默认） | 是否将命中子块扩展为父块内容；可由 `open_retrieve.expand_parent` 覆盖 |
| `expand_nearby` | bool | `false`（系统默认） | 是否附带邻接块；可由 `open_retrieve.expand_nearby` 覆盖 |
| `rewrite` | bool | `false` | 是否对 `query` 做改写。开放 API **默认关**（无会话多轮上下文） |

#### `fusion` 对象

| 字段 | 类型 | 默认 | 说明 |
|------|------|------|------|
| `type` | string | `rrf` | `rrf` \| `weight` |
| `k` | int | `60` | RRF 平滑常数，必须 `> 0` |
| `vector_weight` | number | `1.0` | 向量路权重（RRF / weight 均可用） |
| `keyword_weight` | number | `1.0` | 关键词路权重 |

- **`rrf`**：按排名融合，`score = Σ w_i / (k + rank_i)`，无需跨路分数归一化，**推荐默认**。
- **`weight`**：需先将各路分数归一化到可比区间再线性加权；适合要显式偏置语义/关键词比例的场景。

#### 分层参数关系（调优核心）

```
dense_top_k     = N1  ← 向量召回候选数
keyword_top_k   = N2  ← 关键词召回候选数
fusion / rerank     ← 融合与可选精排
match_count     = N3  ← 最终返回条数（通常 N3 < N1 且 N3 < N2）
```

推荐起步：`N1 = N2 = 40`，`N3 = 20`。加大 N1/N2 提高召回率，同时增加延迟与 Rerank 成本。

### Profile 参数包

`profile` 展开为**固定常量**（可被同请求细粒度字段覆盖）。实现与联调必须使用下表数值，禁止「适当放大」等模糊语义。

| `profile` | 适用场景 | `retrieval_mode` | `dense_top_k` (N1) | `keyword_top_k` (N2) | `fusion` | `enable_rerank` | `match_count` 缺省¹ |
|-----------|----------|------------------|--------------------|----------------------|----------|-----------------|---------------------|
| `balanced` | 平台默认、通用问答 | `hybrid` | `40` | `40` | `rrf`, k=`60`, w_v=`1.0`, w_k=`1.0` | `false` | `20` |
| `precise` | 跨多库、答案必须准 | `hybrid` | `40` | `40` | `rrf`, k=`60`, w_v=`1.0`, w_k=`1.0` | `true` | `5` |
| `lexical` | 编号、专有词、政策条文 | `hybrid` | `20` | `40` | `rrf`, k=`60`, w_v=`0.4`, w_k=`0.6` | `false` | `20` |

¹ 仅当请求未传 `match_count`（或 `≤0`）时使用该缺省；显式传入则覆盖。服务端上限仍适用（建议 ≤ 100）。

覆盖规则：

1. 先展开 `profile`（未传则等同 `balanced`）。
2. 再应用请求中显式出现的细粒度字段（`dense_top_k`、`fusion.vector_weight` 等）。
3. `enable_rerank`：请求显式传 `true`/`false` 时覆盖 profile；未传则用 profile 展开值。

未识别的 `profile` → `400`。

`retrieval_mode=auto`：保留为 **`hybrid` 的别名**（无额外启发式），新客户端请直接传 `hybrid`。

### `score_threshold` 生效条件

| 条件 | 行为 |
|------|------|
| `enable_rerank=true` 且精排成功，`score_type=rerank` | **生效**：过滤 `score < score_threshold` 的条目（Rerank 分通常 ∈ `[0,1]`） |
| 未开 Rerank，或 `score_type` 为 `rrf` / `vector` / `keyword` / `weight` | **忽略** `score_threshold`（可在 `meta.warnings` 提示），避免用「相似度直觉」误砍 RRF 小分 |

`precise` 未传 `score_threshold` 时：系统默认阈值 **`0.2`**（仅 Rerank 路径）。`balanced` / `lexical` 默认不启用阈值。

`normalized_score`（可选）：仅作展示/调试；**不作为** `score_threshold` 比较对象，除非后续版本单独声明公式并锁定测试。当前不规定 min-max / softmax，调用方勿依赖。

### `metadata_filter`

在检索前按元数据收窄范围。字段须在知识侧已定义；逻辑组合建议支持 `and` / `or`。

**跨库语义（钉死）**：

- 多库混检时，仅对**本次命中范围内各库均存在定义**的字段施加过滤（共有字段交集）。
- 请求中出现某库未定义的字段：该条件对未定义库**不生效**，并记入 `meta.warnings`（如 `metadata_filter_field_skipped`）；**不**因此整单 `400`。
- 若过滤后候选为空：返回 `200` + `data: []`。

```json
{
  "metadata_filter": {
    "and": [
      { "field": "category", "op": "eq", "value": "产品文档" },
      { "field": "status", "op": "in", "value": ["published", "active"] }
    ]
  }
}
```

| `op` | 说明 |
|------|------|
| `eq` / `neq` | 等于 / 不等于 |
| `in` / `nin` | 属于 / 不属于集合 |
| `contains` | 字符串包含 |
| `gt` / `gte` / `lt` / `lte` | 数值或时间比较 |

---

## 请求示例

### 最小请求（兼容现网）

```json
{
  "query": "退款规则与处理时效",
  "knowledge_base_ids": ["kb-001"],
  "match_count": 10
}
```

等价于 `profile=balanced`，但 `match_count=10` 覆盖缺省 N3=`20`。

### 推荐：balanced（显式）

```json
{
  "query": "退款规则与处理时效",
  "knowledge_base_ids": ["kb-001", "kb-101"],
  "profile": "balanced",
  "match_count": 20
}
```

### 本轮指定文档（推荐只传 knowledge_ids）

```json
{
  "query": "安字怎么理解 安和定",
  "knowledge_ids": ["doc-88"],
  "profile": "balanced",
  "match_count": 20
}
```

> 若同时传入无关大库的 `knowledge_base_ids` 与 `knowledge_ids`，语义为**并集**，大库分片可能占满 TopK；本轮 `@` 文档场景请勿并入绑定库。

### 并集示例（明确接受多 target 同池时）

```json
{
  "query": "退款规则与处理时效",
  "knowledge_base_ids": ["kb-001"],
  "knowledge_ids": ["doc-88"],
  "profile": "balanced",
  "match_count": 20
}
```

若 `doc-88` 属于 `kb-001`，文档 target 被折叠，等价于只搜整库 `kb-001`。若 `doc-88` 属于其他库，则为「整库 kb-001 ∪ 该文档」并集。

### 质量档：precise（使用表内常量）

```json
{
  "query": "退款规则与处理时效",
  "knowledge_base_ids": ["kb-001", "kb-101"],
  "profile": "precise",
  "score_threshold": 0.3
}
```

展开后：N1=N2=`40`，RRF(k=60, 1:1)，`enable_rerank=true`，N3 缺省=`5`；本例阈值 `0.3` 覆盖 precise 系统缺省 `0.2`。

### 术语偏置：lexical（表内常量，可再覆盖）

```json
{
  "query": "SOP-退款-03 处理时效",
  "knowledge_base_ids": ["kb-001"],
  "profile": "lexical",
  "metadata_filter": {
    "and": [{ "field": "category", "op": "eq", "value": "售后政策" }]
  },
  "match_count": 5
}
```

展开后：N1=`20`，N2=`40`，RRF(k=60, w_v=`0.4`, w_k=`0.6`)，`enable_rerank=false`。若需微调权重，再显式传 `fusion` 覆盖即可。

```bash
curl -sS -X POST "https://{host}/api/v1/open/knowledge/retrieve" \
  -H "Content-Type: application/json" \
  -H "X-Open-Retrieve-Api-Key: {your_open_retrieve_api_key}" \
  -d '{"query":"退款规则与处理时效","knowledge_base_ids":["kb-001"],"profile":"balanced","match_count":20}'
```

---

## 响应结构

### 成功响应

```json
{
  "success": true,
  "meta": {
    "profile": "balanced",
    "retrieval_mode": "hybrid",
    "fusion_type": "rrf",
    "rerank_enabled": false,
    "score_type": "rrf",
    "dense_top_k": 40,
    "keyword_top_k": 40,
    "match_count": 20,
    "skipped_ids": [],
    "target_hits": [
      { "type": "knowledge_base", "knowledge_base_id": "kb-001", "hit_count": 12 },
      { "type": "knowledge", "knowledge_base_id": "kb-101", "knowledge_ids": ["doc-88"], "hit_count": 3 }
    ],
    "warnings": []
  },
  "data": [
    {
      "id": "chunk-001",
      "knowledge_base_id": "kb-001",
      "knowledge_id": "doc-88",
      "knowledge_title": "退款政策说明",
      "content": "......",
      "score": 0.0312,
      "normalized_score": 0.86,
      "score_type": "rrf",
      "match_type": "hybrid",
      "chunk_index": 3,
      "metadata": {}
    }
  ]
}
```

> `meta` 为升级契约字段：便于调用方日志对齐实际生效策略；实现未就绪时可暂不返回，但不改变 `data[]` 兼容字段。  
> `meta.target_hits`（推荐，P0.5）：各检索 target 在融合前的命中条数，用于区分「未入库 / 阈值滤空 / 被全局 TopK 挤掉」。字段未就绪时可用结构化日志等价输出。

### 字段说明（data[]）

| 字段 | 说明 |
|------|------|
| `id` | 分片 ID（与登录态 knowledge-search 对齐，便于二次拉取） |
| `knowledge_base_id` | 命中的知识库 ID |
| `knowledge_id` | 命中的知识 ID |
| `knowledge_title` | 来源知识标题（可选但推荐） |
| `content` | 命中的分片内容 |
| `score` | 原始排序分（含义见 `score_type`） |
| `normalized_score` | 可选，映射到约 `[0, 1]`，便于统一设 `score_threshold` |
| `score_type` | `rrf` \| `vector` \| `keyword` \| `rerank` \| `weight` |
| `match_type` | 命中通道：`vector` \| `keyword` \| `hybrid` \| `faq` 等（实现枚举以代码为准） |
| `chunk_index` | 分片在文档中的序号 |
| `metadata` | 自定义元数据 |
| `images` | 可选。当 `resource_urls=public` 且分片含图时，浏览器可加载的图片列表（从 Markdown 图片与 `image_info` 去重汇总） |

`images[]` 元素：

| 字段 | 说明 |
|------|------|
| `url` | `https://…` 直链（仅 `public` 模式下为 http(s)；`handle` 模式下通常为空数组） |
| `alt` | 可选，来自 Markdown alt 或 `image_info.caption` |
| `chunk_id` | 与当前分片 `id` 一致 |

### 分数契约

| `score_type` | `score` 含义 | 与 `score_threshold` |
|--------------|--------------|----------------------|
| `rrf` | 排名融合分，常见量级约 `0 ~ 0.03`（双路都靠前时），**不是**余弦相似度 | **不参与**阈值过滤 |
| `vector` / `keyword` | 单路原始分，量纲可能不同 | **不参与**阈值过滤 |
| `rerank` | Cross-Encoder / 排序模型分，通常 ∈ `[0, 1]` | **参与**：`score < threshold` 丢弃 |
| `weight` | 归一化后加权分 | **不参与**（避免与未钉死的归一化耦合） |

**禁止**在文档或示例中暗示「未归一化的 RRF 分 ≈ 0.92 相似度」。  
**禁止**对非 `rerank` 的 `score` 套用「0.3 / 0.7」类相似度阈值。

---

## 错误码建议

| HTTP 状态码 | 场景 | 说明 |
|------------|------|------|
| `400` | 参数错误 | `query` 为空；`knowledge_base_ids`/`knowledge_ids` 同时为空；非法 `profile`/`retrieval_mode`/`fusion`；多库 Embedding 模型不一致且无法合并 |
| `401` | API Key 无效 | 未携带 `X-Open-Retrieve-Api-Key` 或与配置不一致 |
| `403` | 功能关闭 | `open_retrieve.enabled=false` |
| `429` | 触发限流 | 若部署启用限流且 QPS 超限 |
| `500` | 内部错误 | 检索服务执行失败 |
| `503` | 依赖不可用 | 向量库 / 关键词引擎 / Rerank 服务不可用（若实现区分） |

---

## 行为约定

- 不做用户登录校验。
- 不做用户归属/可见性校验。
- 按资源记录中的真实 `tenant_id` 执行底层索引/模型访问（支持跨用户、跨库 ID 混检）。
- **召回策略参数不取租户默认**：N1/N2/N3、fusion、阈值、Rerank 开关与模型、expand 等仅来自「请求 > 系统默认」；禁止用 `primaryTenant` 的 `RetrievalConfig` 填默认，避免跨库行为随 ID 顺序漂移。
- **范围 = 并集而非交集**：见 [范围语义](#范围语义钉死)；文档级 target 关闭召回阈值。
- **无效 ID**：跳过并继续（最终可能 `data: []`）；不因单个无效 ID 整单失败。若全部 ID 无效 → 空结果（`200` + 空数组）。建议在 `meta.skipped_ids`（或 `meta.warnings`）列出被跳过的 ID，便于与「真无命中」区分。
- **多库 Embedding 不一致**：返回 `400`（明确错误），禁止静默空结果或跨空间比分。
- **FAQ 库**：仍走现有 FAQ 检索分支；通用策略字段在不影响 FAQ 正确性的前提下尽量复用。
- **`match_count` 与候选池解耦**：缩小 `match_count` **不得**同步缩小 N1/N2（否则损害 Rerank / 融合质量）。
- **验收（策略）**：同一组 `knowledge_base_ids` 仅打乱顺序时，展开后的策略参数（N1/N2/fusion/rerank）必须不变（不得随「首个租户」漂移）。
- **验收（范围）**：仅传 `knowledge_ids` 时，命中须全部来自这些知识（在已入库且可检索前提下）；「绑定大库 + 本轮文档」并集时，若调用方未收窄，允许大库占满 TopK（属调用方责任，非交集语义）。

---

## 场景 → 推荐调用

| 场景 | 推荐 | 要点 |
|------|------|------|
| 通用开放接入 | `profile: balanced` | N1=N2=40，关 Rerank，N3 缺省 20 |
| 本轮 `@` 文档 | 只传 `knowledge_ids` + `balanced` | **不要**并绑定大库；query 锚定文档标题 |
| 本轮 `@` 知识库 | 只传该库 `knowledge_base_ids` | 同左 |
| 跨多库且要准 | `profile: precise` | N1=N2=40，开 Rerank，默认阈值 0.2；库需同 Embedding |
| 编号 / 专有词 | `profile: lexical` | N1=20 / N2=40，w_v:w_k=0.4:0.6，关 Rerank |
| 低延迟实时 | `balanced` + 显式更小 N1/N2/N3 | 保持 `enable_rerank=false` |
| 带类目/标签语料 | 加 `tag_ids` 或 `metadata_filter` | 过滤放在检索前；跨库仅共有字段 |

---

## 实现落地阶段（与契约对齐）

| 阶段 | 内容 | 契约字段 |
|------|------|----------|
| **P0** | 钉死 profile 常量；去掉租户 `RetrievalConfig` 回落；`profile` / `retrieval_mode` / N1/N2/N3；分数契约；**文档级 target 关召回阈值**；范围并集语义写入契约 | 上表「召回策略字段」基础项 + 系统默认 + [范围语义](#范围语义钉死) |
| **P0.5** | `meta.skipped_ids`；`meta.target_hits`（或等价日志） | 排障：未入库 / 滤空 / TopK 挤掉 |
| **P1** | `fusion` 可覆盖；`enable_rerank`；`score_threshold`；`tag_ids` / `metadata_filter`；响应 `meta` + `score_type` | 质量与过滤 |
| **P2** | `rewrite`、`expand_parent` / `expand_nearby`；评测集（Recall@5 / MRR / p95） | 扩展与可观测 |
| **P3** | 按库能力探测开启图谱一路（非默认） | 不进入默认 `profile` |

当前代码路径：基础字段已通；策略细字段按上表分批实现。**客户端应以「未传 = 默认 balanced 行为」编写**，以便平滑升级。  
范围收窄由调用方（chatbot / 技能）在 P0 即可落地，**不依赖**新 `scope_mode` 字段。

---

## 批量分片图片换签（无登录）

用于 chatbot 历史会话：按稳定 `chunk_id` 批量换取 `resource_urls=public` 的可加载图片 URL，**不做**向量召回。

- **方法**: `POST`
- **路径**: `/api/v1/open/knowledge/chunks/media`
- **认证**: 同开放检索，`X-Open-Retrieve-Api-Key`
- **查询参数**: `resource_urls=public`（推荐；与 retrieve 一致）
- **请求体**:

```json
{ "chunk_ids": ["61a843af-0d16-41aa-aa80-50741028636f"] }
```

- **响应**: `{ "success": true, "data": [{ "chunk_id": "…", "knowledge_base_id": "…", "images": [{ "url": "https://…", "alt": "…", "chunk_id": "…" }] }] }`
- **限制**: 单次最多 64 个 `chunk_id`（去重后）
- **父子分块**: 请求 ID 为 `text` 子块且自身无图时，自动回退读取其 `parent_text` 的 markdown / `image_info` 换签；响应 `chunk_id` 仍为请求的叶子 ID（与 chatbot 落库的 `kbMedia.items[].chunkId` 对齐）

---

## 安全建议（精简）

- **HTTPS**：传输层保护 API Key。
- **密钥轮换**：定期更换 `X-Open-Retrieve-Api-Key` 对应配置值。
- **可选**：网关 IP 白名单、简单 QPS 限流、结构化审计（不落 query 明文；可记 `query_hash`）。
- **Rerank**：开放面默认关闭，避免调用方在不知情时产生模型费用。
