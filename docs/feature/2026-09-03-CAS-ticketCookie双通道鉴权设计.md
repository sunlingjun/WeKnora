# 2026-09-03 CAS Cookie 双通道鉴权设计（ticketCookie + _cas_sid）

> **状态**：已拍板并已实现（方案 A 修订版；代码在 WeKnora-slj，待活票联调）  
> **仓库**：WeKnora-slj  
> **实现计划**：[2026-09-03-CAS-ticketCookie双通道鉴权实现计划.md](./2026-09-03-CAS-ticketCookie双通道鉴权实现计划.md)  
> **对照**：网关 `UserServiceImpl`（`ticketCookie` → ZNT；`_cas_sid` → UC ticket）+ manage 依赖 `UserCenterClient.getUserArchive`

---

## 1. 背景与目标

### 1.1 问题

- 调用方（APP / BFF）可通过 Header 传 `Cookie: ticketCookie=<token>`，网关优先用其换档案；WeKnora 现网 `nxin_cas_auth` 只认环境 `cookie_sid` + `cookie_uid`，且走 `user.get/3.0`。
- 实测（测试环境）：有效 `ticketCookie` **不能**被 `user.get/3.0` 识别（`10011`）；同一票可走 `login/get-boId-by-znt-token/{token}` 得到 boId。
- 仅档案 ID 不足以支撑 `AutoBindUser`；须拉齐登录名、实名、手机等字段。

### 1.2 目标

1. 业务 Cookie 鉴权与 `GET /api/v1/cas/validate` **统一**为网关同款双通道，且 **uid 非必须**。
2. 换档后通过 UC `person/getUserArchive/{boId}` 获取完整档案，再 `AutoBindUser` / `AutoBindTenant`。
3. **移除** Cookie 主路径上对 `user.get/3.0`（sid+uid）的依赖与优先分支。

### 1.3 非目标

- 不改编网关 Java 代码。
- 不引入 BFF JWT 换票；仍 Cookie（+ 可选 `X-Tenant-ID`）。
- 不强制删除 `CASClient.ValidateSession` 源码（可留作废弃/内部兼容，但 Cookie 鉴权不再调用）。

---

## 2. 鉴权顺序（唯一真源）

中间件 `tryNXINCASAuth` 与 `CASAuthHandler.ValidateCASSession` **共用**下列顺序：

```
1. Cookie「ticketCookie」非空
   → GET  {CAS_UC_URL}login/get-boId-by-znt-token/{token}
   → POST {CAS_UC_URL}person/getUserArchive/{boId}   // 须 CAS_UC_* 服务凭证
   → CASUserInfo → AutoBindUser → AutoBindTenant → 建会话

2. 否则 Cookie「{cookie_sid}」非空（生产 _cas_sid / 测试 _cas_t_sid）
   → POST {CAS_UC_URL}login/getUserByUcTicket   form: ticket=<sid>
   → POST {CAS_UC_URL}person/getUserArchive/{boId}
   → CASUserInfo → AutoBind…（同上）

3. 二者皆无
   → 中间件：返回 false（继续「missing authentication」）
   → validate：现有未登录 JSON（code 10011 风格保持兼容）
```

**明确删除：**

- 读取 / 依赖 `cookie_uid`（`_cas_uid` / `_cas_t_uid`）作为 Cookie 鉴权前置条件。
- 「有 sid+uid 则优先 `ValidateCASSession` / `nxin.usercenter.user.get/3.0`」逻辑。

**JWT 冲突检测（`jwtIdentityConflictsWithCASCookie`）：**

- 仅当请求仍携带 uid cookie 时，与 JWT 用户的 `cas_user_id` 比对。
- **无 uid cookie → 不视为冲突**（与现网「无 cookie 不冲突」一致，并覆盖仅 ticketCookie / 仅 sid 场景）。

---

## 3. UC 接口约定

| 步骤 | 方法 | 路径 | 凭证 | 成功语义 |
|------|------|------|------|----------|
| ZNT 换 ID | GET | `login/get-boId-by-znt-token/{token}` | 无服务凭证 | `code=0`，`data` 为档案 ID（number/string） |
| UC ticket 换 ID | POST | `login/getUserByUcTicket` | 无服务凭证 | `code=0`，`data.id` 为档案 ID |
| 拉全量档案 | POST | `person/getUserArchive/{boId}` | `systemId` + `timestamp` + `accessToken=md5(cert+timestamp)` | `code=0`，`data` 为 UserArchive |

`getUserArchive` 字段映射沿用现有 `archiveToCASUserInfo`（与农信导入一致），至少覆盖 AutoBind 使用字段：

| UC / CASUserInfo | AutoBind 用途 |
|------------------|---------------|
| `id` / `idStr` | `cas_user_id`、查重 |
| `loginName` | `username` / `cas_login_name` |
| `realName` | 默认空间名 / `cas_real_name` |
| `email` | 查重 / `users.email` |
| `mobilePhone` | 占位密码 / `cas_mobile_phone` |
| `unionId` / `nickName` / `image` / `phoneSigned` | 解析保留；Avatar 仍可不写本地（与现 AutoBind 行为一致，本需求不强制改 Avatar） |

---

## 4. 配置

| 配置 | 用途 |
|------|------|
| `CAS_UC_URL` | UC base（须尾斜杠），如 `http://uc.t.nxin.com/` |
| `CAS_UC_SYSTEM_ID` / `CAS_UC_CERT` | **仅** `getUserArchive`（及已有导入目录接口） |
| `cas.*.cookie_sid` | 通道 2 的 Cookie 名 |
| `auth.nxin_cas_auth.*` | enabled、cache_ttl、require_https、allowed_path_globs 不变 |

未配置 `CAS_UC_*`（`Configured()==false`）时：双通道无法拉全量档案 → 按鉴权失败处理（401 / validate 未登录），并打明确日志（避免静默退回已删除的 user.get 路径）。

---

## 5. 缓存

- Redis key：对 `channel + token`（或 boId）做摘要，前缀保持 `auth:nxin_cas_auth:` 风格，避免明文 ticket 入库。
- Value：`{ user_id, tenant_id }`（与现网一致）。
- TTL：`nxin_cas_auth.cache_ttl_seconds`。
- 命中缓存后仍走 `authenticateJWTUser` / validate 发 JWT，不跳过用户激活检查。

---

## 6. 失败语义

| 情况 | 行为 |
|------|------|
| 有 ticket/sid，ZNT 或 UcTicket 失败 | 401（invalid session）/ validate 未登录 |
| 换 ID 成功，`getUserArchive` 失败或未配置凭证 | 401 或 503（凭证缺失建议 503「Auth provider unavailable」；票无效 401） |
| AutoBind 失败 | 503（与现网一致） |
| 无 ticket 且无 sid | 中间件 false；validate 未登录 |

---

## 7. 代码落点（WeKnora-slj）

| 区域 | 改动 |
|------|------|
| `user_center_client.go`（或并列 login 客户端） | 增加无凭证的 ZNT / UcTicket；增加 `GetUserArchive(boId)`（服务凭证 POST） |
| Cookie 解析 helper | 读 `ticketCookie`、环境 `cookie_sid`；**不强制** uid |
| `tryNXINCASAuth` | 按 §2 换档；删除 uid 前置与 user.get 优先 |
| `handler/cas_auth.go` | validate 改走同一解析 + 换档 |
| `jwtIdentityConflictsWithCASCookie` | 无 uid 不冲突（保持/对齐） |
| 单测 | 优先级 ticketCookie > sid；仅 ticket；仅 sid；无票；getUserArchive 映射；缓存 |

`CASClient.ValidateSession`：本需求范围内 Cookie 路径不再调用；是否删代码留给后续清理，不阻塞本功能。

---

## 8. 验证计划

1. 测试环境：有效 `Cookie: ticketCookie=...` 调受保护 API → 200，用户 `cas_user_id` 与 ZNT boId 一致。
2. 有效 `_cas_t_sid`（无 uid）→ 同上。
3. `GET /api/v1/cas/validate` 仅 ticketCookie 或仅 sid → 返回 JWT + user。
4. 两 Cookie 皆无 → 401 / 未登录。
5. 故意错误票 → 401，不误绑。
6. 清空 `CAS_UC_CERT` → getUserArchive 失败路径符合 §6。

---

## 9. 决策记录

| 项 | 决定 |
|----|------|
| 落点 | WeKnora-slj only |
| 通道 | ticketCookie → ZNT；否则 cookie_sid → UcTicket；再统一 getUserArchive |
| uid | 非必须；Cookie 鉴权不再读 uid |
| user.get/3.0 | Cookie 主路径移除 |
| validate | 与中间件同一套双通道 |
| 全量档案 | `POST person/getUserArchive/{boId}`（网关 manage `UserCenterClient.getUserArchive` 同源） |

---

## 10. 参考

- 网关：`UserServiceImpl`（`APP_TOKEY_COOKIE_NAME=ticketCookie`，`USER_TOKEY_COOKIE_NAME=_cas_sid`）
- 网关 manage：`UserCenterClientConfig` + `getUserArchive`
- 现网映射：`internal/application/service/user_center_client.go` → `archiveToCASUserInfo`
- 绑定：`internal/application/service/cas_auth.go` → `AutoBindUser`
