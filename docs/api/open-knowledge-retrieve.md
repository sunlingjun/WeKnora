# 开放检索 API（无登录）

[返回目录](./README.md)

## 概述

该接口用于三方系统在**无用户登录态**下，按知识库 ID 或知识 ID 检索知识分片（chunk），用于外部模型上下文拼接。  
该接口与登录态的 `/api/v1/knowledge-search` 语义不同：不做用户归属/可见性校验。

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

---

## 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `query` | string | 是 | 用户问题或检索语句 |
| `knowledge_base_ids` | string[] | 否 | 整库检索范围 |
| `knowledge_ids` | string[] | 否 | 指定知识检索范围 |
| `match_count` | int | 否 | 返回分片条数上限（服务端可做上限保护） |

> 约束：`knowledge_base_ids` 与 `knowledge_ids` 至少提供一个。

### 请求示例

```json
{
  "query": "退款规则与处理时效",
  "knowledge_base_ids": ["kb-001", "kb-101"],
  "knowledge_ids": ["doc-88"],
  "match_count": 10
}
```

```bash
curl -sS -X POST "https://{host}/api/v1/open/knowledge/retrieve" \
  -H "Content-Type: application/json" \
  -H "X-Open-Retrieve-Api-Key: {your_open_retrieve_api_key}" \
  -d "{\"query\":\"退款规则与处理时效\",\"knowledge_base_ids\":[\"kb-001\"]}"
```

---

## 响应结构

### 成功响应

```json
{
  "success": true,
  "data": [
    {
      "knowledge_base_id": "kb-001",
      "knowledge_id": "doc-88",
      "content": "......",
      "score": 0.92,
      "match_type": "embedding"
    }
  ]
}
```

### 字段说明（data[]）

| 字段 | 说明 |
|------|------|
| `knowledge_base_id` | 命中的知识库 ID |
| `knowledge_id` | 命中的知识 ID |
| `content` | 命中的分片内容 |
| `score` | 匹配分数 |
| `match_type` | 匹配类型（如 embedding / keyword / hybrid） |

---

## 错误码建议

| HTTP 状态码 | 场景 | 说明 |
|------------|------|------|
| `400` | 参数错误 | `query` 为空，或 `knowledge_base_ids`/`knowledge_ids` 同时为空 |
| `401` | API Key 无效 | 未携带 `X-Open-Retrieve-Api-Key` 或与配置不一致 |
| `403` | 功能关闭 | `open_retrieve.enabled=false` |
| `429` | 触发限流 | 若部署启用限流且 QPS 超限 |
| `500` | 内部错误 | 检索服务执行失败 |

---

## 行为约定

- 不做用户登录校验。
- 不做用户归属/可见性校验。
- 按资源记录中的真实 `tenant_id` 执行底层检索。
- 无效 ID 可按实现策略处理为“跳过并返回空结果”或“返回参数错误”，建议对外固定一种行为。

---

## 安全建议（精简）

- **HTTPS**：传输层保护 API Key。
- **密钥轮换**：定期更换 `X-Open-Retrieve-Api-Key` 对应配置值。
- **可选**：网关 IP 白名单、简单 QPS 限流、结构化审计（不落 query 明文）。
