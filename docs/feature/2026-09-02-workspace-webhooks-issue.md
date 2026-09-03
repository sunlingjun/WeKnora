## 背景

本设计已与 WeKnora 维护者沟通，确定开发。

自托管 WeKnora 目前没有工作空间级的知识生命周期**出站** Webhook。对接方只能轮询列表 API，会漏掉 worker 侧的 `parse_completed`。嵌入渠道的聊天 Webhook 是另一套协议（会话消息、只签 body、不重试）。

## 方案

由 Owner 配置 HTTPS 回调（每空间最多 5 个）：

- outbox + 独立 asynq `webhook` 池（隔离方式对齐 Wiki）
- P0 事件：`knowledge.created|parse_completed|parse_failed|deleted|batch_deleted`、`kb.created|deleted`、`rbac.member_added|removed`、`webhook.test`
- HMAC-SHA256 签 `timestamp + "." + raw_body`
- 短时下载票：`GET /api/v1/files/knowledge-download/:id`（现网 `GET /knowledge/:id/download` 不改）
- 设置 → 空间 → 事件回调（仅 Owner）

设计文档随 PR 提交：`docs/workspace-webhooks.md`

一个功能 PR（后端 + 设置页 + 文档）。

## 本 PR 不做

- 组织共享扇出 / `kb.share_*`
- 改 `/knowledge/:id/download`
- 在载荷里放文件正文、JWT 或空间 API Key
- 与 Embed 或 IM 入站 webhook 合并

## 安全

SSRF、HTTPS、密钥不回显、票绑定 purpose、outbox 保证至少一次，接收方按包络 `id` 去重。
