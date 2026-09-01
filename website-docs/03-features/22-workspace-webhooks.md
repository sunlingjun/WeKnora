# 工作空间知识事件回调

把本空间**自己拥有的**知识库、知识、成员变化，推到 Owner 配置的 HTTPS 地址。这和「发布集成 → 网页嵌入」里的聊天 Webhook 不是同一套：这里是知识生命周期，那边是会话消息。

配置路径：设置 → 空间 → **事件回调**（仅 Owner）。每个空间最多 5 个 URL；生产须 https。密钥用于 HMAC，创建后不再回显。

## 投递什么

P0 事件：

- `knowledge.created` / `parse_completed` / `parse_failed` / `deleted` / `batch_deleted`
- `kb.created` / `kb.deleted`（删库只发这一条，不刷 N 条文档事件）
- `rbac.member_added` / `rbac.member_removed`（本工作空间用户加入/移除，不是共享空间）
- `webhook.test`（设置页点测试）

组织共享的 `kb.share_*` 和扇出到成员空间是后续能力，本版不做。全量底账用[知识目录拉取](../04-api/01-api-overview.md)，不要拿 Webhook 当全量同步。

## 怎么验签

WeKnora 作为客户端 POST 到你填的 URL：

```http
POST /hooks/weknora HTTP/1.1
Content-Type: application/json
User-Agent: WeKnora-Workspace-Webhook/1.0
X-WeKnora-Event: knowledge.created
X-WeKnora-Delivery: dlv_...
X-WeKnora-Timestamp: 1756624860
X-WeKnora-Signature: sha256=...
```

签名是 `HMAC-SHA256(secret, timestamp + "." + raw_body)`，**不是**只签 body（嵌入渠道聊天 Webhook 才是只签 body）。按包络 `id` 去重；投递至少一次。

## 源文件下载票

`knowledge.created` / `parse_completed` 里若有源文件，`data.download.ticket` 是 5 分钟票：

```http
GET /api/v1/files/knowledge-download/:id
X-WeKnora-Download-Ticket: wdt1....
```

过期后 1 小时内可 `POST /api/v1/files/knowledge-download/:id/renew` 换新票。这与现网 `GET /api/v1/knowledge/:id/download`（需要 Contributor+）是两条路，不要把知识 API 加进匿名白名单。

图片与其它渠道的文件访问见[图片与文件的对外访问](21-file-access.md)。
