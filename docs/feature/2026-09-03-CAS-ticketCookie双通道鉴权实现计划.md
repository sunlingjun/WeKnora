# CAS Cookie 双通道鉴权 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** WeKnora Cookie 鉴权对齐网关：优先 `ticketCookie`→ZNT→`getUserArchive`，否则 `cookie_sid`→`getUserByUcTicket`→`getUserArchive`；uid 与 `user.get/3.0` 不再作为 Cookie 主路径。

**Architecture:** 在现有 `userCenterDirectoryClient`（`CAS_UC_*`）上扩展无凭证换档 + 有凭证拉档；`CASAuthService` 新增 `ResolveCASUserFromCookies`；`tryNXINCASAuth` 与 `/cas/validate` 共用该解析。JWT 冲突检测仅在存在 uid cookie 时生效。

**Tech Stack:** Go、Gin、Redis、httptest 单测、现有 `archiveToCASUserInfo` / `AutoBindUser`

**Spec:** [docs/feature/2026-09-03-CAS-ticketCookie双通道鉴权设计.md](./2026-09-03-CAS-ticketCookie双通道鉴权设计.md)

---

## 复查结论（方案无误）

| 检查项 | 结论 |
|--------|------|
| `ticketCookie` ≠ `user.get` | 实测有效 ZNT 票调 `user.get/3.0` 仍 `10011`；ZNT 换 boId 成功 |
| 全量档案接口 | `POST person/getUserArchive/{boId}` + 服务凭证，字段覆盖 AutoBind |
| 与网关优先级 | ticketCookie 优先，否则 sid；与 `UserServiceImpl` 一致 |
| 去掉 sid+uid / user.get | 双通道已够用；浏览器仅带 sid 也可 UcTicket→Archive |
| 配置 | 复用 `CAS_UC_*`；拉档必须三项齐全 |
| 残留风险 | 本计划未用「活的 `_cas_t_sid`」实网打通 UcTicket（网关现网依赖该接口）；实现后用真 sid 做联调 Task 验证 |

**实现时注意（相对网关微调，以 spec 为准）：** 若请求同时有 `ticketCookie` 与 sid，且 ticketCookie 换档失败 → **直接 401**，不回落到 sid（spec §2/§6）。

---

## File map

| File | Responsibility |
|------|----------------|
| `internal/types/interfaces/cas_member_import.go` | 扩展 `UserCenterDirectory`：ZNT / UcTicket / GetUserArchive |
| `internal/application/service/user_center_client.go` | HTTP 实现；导出/复用 `parseUserArchiveObject` |
| `internal/application/service/user_center_client_test.go` | UC 客户端单测 |
| `internal/types/interfaces/cas_auth.go` | 新增 `ResolveCASUserFromCookies`；旧 `ValidateCASSession` 保留但 Cookie 路径停用 |
| `internal/application/service/cas_auth.go` | 实现 Resolve；注入 UserCenterDirectory |
| `internal/application/service/cas_auth_resolve_test.go` | Resolve 优先级与失败单测 |
| `internal/middleware/cas_cookie.go`（新建） | 读 `ticketCookie` / `cookie_sid`（及可选 uid，仅冲突检测用） |
| `internal/middleware/auth_cas.go` | 改走 Resolve；缓存 key 用 channel+token |
| `internal/middleware/auth.go` | 注释更新；冲突检测逻辑确认 |
| `internal/handler/cas_auth.go` | validate 改走 Resolve |
| `internal/container/container.go` | 若 `NewCASAuthService` 签名变更则接线 |
| `docs/feature/2026-09-03-CAS-ticketCookie双通道鉴权设计.md` | 已存在，不重复改除非实现偏差 |

---

### Task 1: UC 客户端 — GetUserArchive + 换档 API（TDD）

**Files:**
- Modify: `internal/types/interfaces/cas_member_import.go`
- Modify: `internal/application/service/user_center_client.go`
- Modify: `internal/application/service/user_center_client_test.go`

- [ ] **Step 1: 扩展接口**

在 `UserCenterDirectory` 增加：

```go
// UserCenterDirectory looks up 农信 users (directory + session ticket resolve).
type UserCenterDirectory interface {
	Configured() bool
	// HasBaseURL reports whether CAS_UC_URL (or alias) is set. ZNT/UcTicket need URL only.
	HasBaseURL() bool
	FindByAuthorizedPhone(ctx context.Context, phone string) (*types.CASUserInfo, error)
	SearchByNameOrPhone(ctx context.Context, keyword string) ([]*types.CASUserInfo, error)
	// GetBoIDByZNTToken GET login/get-boId-by-znt-token/{token} → archive id string.
	GetBoIDByZNTToken(ctx context.Context, token string) (string, error)
	// GetBoIDByUcTicket POST login/getUserByUcTicket ticket= → archive id string.
	GetBoIDByUcTicket(ctx context.Context, ticket string) (string, error)
	// GetUserArchive POST person/getUserArchive/{boID} with service credentials.
	GetUserArchive(ctx context.Context, boID string) (*types.CASUserInfo, error)
}
```

- [ ] **Step 2: 写失败单测（先红）**

在 `user_center_client_test.go` 追加：

```go
func TestGetBoIDByZNTToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/login/get-boId-by-znt-token/tok-1", r.URL.Path)
		require.Empty(t, r.Header.Get("systemId")) // no service creds
		_, _ = w.Write([]byte(`{"code":0,"data":1787161,"error":""}`))
	}))
	t.Cleanup(srv.Close)
	c := &userCenterDirectoryClient{
		cfg:        config.CASUserCenterConfig{URL: srv.URL + "/"},
		httpClient: srv.Client(),
	}
	id, err := c.GetBoIDByZNTToken(context.Background(), "tok-1")
	require.NoError(t, err)
	require.Equal(t, "1787161", id)
}

func TestGetBoIDByUcTicket(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/login/getUserByUcTicket", r.URL.Path)
		body, _ := io.ReadAll(r.Body)
		vals, _ := url.ParseQuery(string(body))
		require.Equal(t, "sid-1", vals.Get("ticket"))
		_, _ = w.Write([]byte(`{"code":0,"data":{"id":1787161}}`))
	}))
	t.Cleanup(srv.Close)
	c := &userCenterDirectoryClient{
		cfg:        config.CASUserCenterConfig{URL: srv.URL + "/"},
		httpClient: srv.Client(),
	}
	id, err := c.GetBoIDByUcTicket(context.Background(), "sid-1")
	require.NoError(t, err)
	require.Equal(t, "1787161", id)
}

func TestGetUserArchiveByBoID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/person/getUserArchive/1787161", r.URL.Path)
		require.Equal(t, "sys-1", r.Header.Get("systemId"))
		_, _ = w.Write([]byte(`{"code":0,"data":{"id":1787161,"idStr":"1787161","loginName":"u1","realName":"刘二","mobilePhone":"182****2222","unionId":"ff1bb4e8-85e9"}}`))
	}))
	t.Cleanup(srv.Close)
	c := &userCenterDirectoryClient{
		cfg: config.CASUserCenterConfig{
			URL: srv.URL + "/", SystemID: "sys-1", Cert: "cert-value",
		},
		httpClient: srv.Client(),
	}
	info, err := c.GetUserArchive(context.Background(), "1787161")
	require.NoError(t, err)
	require.Equal(t, "1787161", info.ID)
	require.Equal(t, "刘二", info.RealName)
	require.Equal(t, "u1", info.LoginName)
}
```

- [ ] **Step 3: 跑测确认失败**

Run:

```bash
cd e:/Tencent/WeKnora-slj
go test ./internal/application/service/ -run "TestGetBoIDByZNTToken|TestGetBoIDByUcTicket|TestGetUserArchiveByBoID" -count=1
```

Expected: 编译失败或 undefined method（接口/实现尚未写完）。

- [ ] **Step 4: 实现**

在 `user_center_client.go`：

```go
func (c *userCenterDirectoryClient) HasBaseURL() bool {
	return c != nil && strings.TrimSpace(c.cfg.URL) != ""
}

func (c *userCenterDirectoryClient) GetBoIDByZNTToken(ctx context.Context, token string) (string, error) {
	if !c.HasBaseURL() {
		return "", fmt.Errorf("user center url is not configured")
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return "", fmt.Errorf("empty znt token")
	}
	path := "login/get-boId-by-znt-token/" + url.PathEscape(token)
	raw, err := c.getNoAuth(ctx, path)
	if err != nil {
		return "", err
	}
	return parseBoIDFromDataField(raw) // code==0, data number|string
}

func (c *userCenterDirectoryClient) GetBoIDByUcTicket(ctx context.Context, ticket string) (string, error) {
	if !c.HasBaseURL() {
		return "", fmt.Errorf("user center url is not configured")
	}
	ticket = strings.TrimSpace(ticket)
	if ticket == "" {
		return "", fmt.Errorf("empty uc ticket")
	}
	raw, err := c.postFormNoAuth(ctx, "login/getUserByUcTicket", url.Values{"ticket": {ticket}})
	if err != nil {
		return "", err
	}
	return parseBoIDFromUcTicket(raw) // code==0, data.id
}

func (c *userCenterDirectoryClient) GetUserArchive(ctx context.Context, boID string) (*types.CASUserInfo, error) {
	if !c.Configured() {
		return nil, fmt.Errorf("user center is not configured")
	}
	boID = strings.TrimSpace(boID)
	if boID == "" {
		return nil, fmt.Errorf("empty bo id")
	}
	// POST with empty body + service headers (gateway client style)
	raw, err := c.postForm(ctx, "person/getUserArchive/"+url.PathEscape(boID), url.Values{})
	if err != nil {
		return nil, err
	}
	info, err := parseUserArchiveObject(raw)
	if err != nil {
		return nil, err
	}
	if info == nil || info.ID == "" {
		return nil, fmt.Errorf("user archive empty for boId=%s", boID)
	}
	return info, nil
}
```

实现 `getNoAuth` / `postFormNoAuth`（无 systemId 头）、`parseBoIDFromDataField` / `parseBoIDFromUcTicket`（复用 `userCenterCodeOK`、`stringifyJSONID`）。  
非 0 code → 返回 error（含 code），供上层映射 401。

更新所有 `UserCenterDirectory` 的 test stub（`cas_member_import_test.go` 的 `stubUCDir`）补空实现，保证编译通过。

- [ ] **Step 5: 跑测确认通过**

```bash
go test ./internal/application/service/ -run "TestGetBoIDByZNTToken|TestGetBoIDByUcTicket|TestGetUserArchiveByBoID|TestUserCenter" -count=1
```

Expected: PASS

- [ ] **Step 6: Commit（仅当用户要求提交时执行）**

```bash
git add internal/types/interfaces/cas_member_import.go internal/application/service/user_center_client.go internal/application/service/user_center_client_test.go internal/application/service/cas_member_import_test.go
git commit -m "feat(cas): add UC ZNT/UcTicket and getUserArchive client methods"
```

---

### Task 2: CASAuthService.ResolveCASUserFromCookies

**Files:**
- Modify: `internal/types/interfaces/cas_auth.go`
- Modify: `internal/application/service/cas_auth.go`
- Create: `internal/application/service/cas_auth_resolve_test.go`
- Modify: `internal/container/container.go`（若构造函数增参）

- [ ] **Step 1: 接口**

```go
type CASAuthService interface {
	// ResolveCASUserFromCookies follows gateway priority:
	// ticketCookie → ZNT+Archive; else casSid → UcTicket+Archive.
	// Empty both → (nil, ErrCASCredentialsMissing).
	ResolveCASUserFromCookies(ctx context.Context, ticketCookie, casSid string) (*types.CASUserInfo, error)

	ValidateCASSession(ctx context.Context, casSid, casUid string, referer string) (*types.CASUserInfo, error)
	AutoBindUser(ctx context.Context, casUserInfo *types.CASUserInfo) (*types.User, error)
	AutoBindTenant(ctx context.Context, casUserInfo *types.CASUserInfo, user *types.User) (*types.Tenant, error)
}
```

在 `cas_auth.go` 包级：

```go
var (
	ErrCASCredentialsMissing = errors.New("cas credentials missing")
	ErrCASTicketInvalid      = errors.New("cas ticket invalid")
	ErrCASUserCenterUnavailable = errors.New("cas user center unavailable")
)
```

- [ ] **Step 2: 写 Resolve 单测（先红）**

```go
func TestResolvePrefersTicketCookieOverSid(t *testing.T) {
	dir := &fakeUCDir{
		znt: map[string]string{"tk": "100"},
		sid: map[string]string{"sid": "200"},
		arch: map[string]*types.CASUserInfo{
			"100": {ID: "100", LoginName: "from-znt"},
			"200": {ID: "200", LoginName: "from-sid"},
		},
	}
	svc := newCASAuthWithDir(dir)
	info, err := svc.ResolveCASUserFromCookies(context.Background(), "tk", "sid")
	require.NoError(t, err)
	require.Equal(t, "100", info.ID)
}

func TestResolveFallsToSidWhenNoTicketCookie(t *testing.T) { /* sid only → 200 */ }

func TestResolveMissingBoth(t *testing.T) {
	_, err := svc.ResolveCASUserFromCookies(context.Background(), "", "")
	require.ErrorIs(t, err, ErrCASCredentialsMissing)
}
```

`fakeUCDir` 实现完整 `UserCenterDirectory`。

- [ ] **Step 3: 实现 Resolve + 改 NewCASAuthService 注入 dir**

```go
func (s *casAuthService) ResolveCASUserFromCookies(ctx context.Context, ticketCookie, casSid string) (*types.CASUserInfo, error) {
	if s.userCenter == nil || !s.userCenter.HasBaseURL() {
		return nil, ErrCASUserCenterUnavailable
	}
	ticketCookie = strings.TrimSpace(ticketCookie)
	casSid = strings.TrimSpace(casSid)

	var boID string
	var err error
	switch {
	case ticketCookie != "":
		boID, err = s.userCenter.GetBoIDByZNTToken(ctx, ticketCookie)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrCASTicketInvalid, err)
		}
	case casSid != "":
		boID, err = s.userCenter.GetBoIDByUcTicket(ctx, casSid)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrCASTicketInvalid, err)
		}
	default:
		return nil, ErrCASCredentialsMissing
	}
	if !s.userCenter.Configured() {
		return nil, ErrCASUserCenterUnavailable
	}
	info, err := s.userCenter.GetUserArchive(ctx, boID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCASUserCenterUnavailable, err)
	}
	return info, nil
}
```

`ValidateCASSession` 实现可保留，但注释标明 Cookie 主路径已废弃；**中间件/handler 不再调用**。

更新 `NewCASAuthService` 增加 `userCenter interfaces.UserCenterDirectory` 参数；`container.go` Provide 自动按类型注入（确认已有 `NewUserCenterDirectoryClient`）。

更新 `spyCASAuth` / fake 实现补 `ResolveCASUserFromCookies`。

- [ ] **Step 4: 跑测**

```bash
go test ./internal/application/service/ -run "TestResolve|TestCASAuth|TestUserCenter" -count=1
```

Expected: PASS

- [ ] **Step 5: Commit（用户要求时）**

```bash
git commit -m "feat(cas): resolve CAS user via ticketCookie/ZNT or sid/UcTicket then getUserArchive"
```

---

### Task 3: Cookie helper + 中间件 + validate

**Files:**
- Create: `internal/middleware/cas_cookie.go`
- Create: `internal/middleware/cas_cookie_test.go`
- Modify: `internal/middleware/auth_cas.go`
- Modify: `internal/middleware/auth.go`（注释）
- Modify: `internal/handler/cas_auth.go`

- [ ] **Step 1: Cookie helper**

```go
// package middleware
const ticketCookieName = "ticketCookie"

func readCASAuthCookies(c *gin.Context, cfg *config.Config) (ticketCookie, casSid, casUID string) {
	if c == nil {
		return "", "", ""
	}
	ticketCookie, _ = c.Cookie(ticketCookieName)
	if cfg != nil && cfg.CAS != nil {
		if env := cfg.CAS.GetCurrentConfig(); env != nil {
			casSid, _ = c.Cookie(env.CookieSID)
			casUID, _ = c.Cookie(env.CookieUID)
		}
	}
	return strings.TrimSpace(ticketCookie), strings.TrimSpace(casSid), strings.TrimSpace(casUID)
}
```

单测：用 `httptest` + gin 注入 Cookie。

- [ ] **Step 2: 改 `tryNXINCASAuth`**

替换「读 sid+uid → ValidateCASSession」为：

```go
ticketCookie, casSid, _ := readCASAuthCookies(c, cfg)
if ticketCookie == "" && casSid == "" {
	return false
}
// cache key: channel|token
cacheKey := buildNXINCASAuthCacheKey(casEnv.APIHost, cacheChannel(ticketCookie, casSid), cacheToken(ticketCookie, casSid))
// ... cache hit unchanged ...

casUserInfo, err := casAuthService.ResolveCASUserFromCookies(c.Request.Context(), ticketCookie, casSid)
if errors.Is(err, service.ErrCASCredentialsMissing) {
	return false
}
if errors.Is(err, service.ErrCASTicketInvalid) {
	c.JSON(401, gin.H{"error": "Unauthorized: invalid CAS session"})
	c.Abort()
	return true
}
if err != nil { // unavailable / archive
	c.JSON(503, gin.H{"error": "Auth provider unavailable"})
	c.Abort()
	return true
}
// AutoBindUser / AutoBindTenant / authenticateJWTUser / set cache — same as today
```

注意：middleware 引用 `service.Err*` 会形成依赖；**更干净**做法是在 `interfaces` 或 `middleware` 用 `errors.Is` 对包装错误字符串，或把 sentinel error 放到 `internal/types` / 小包 `caserr`。推荐：`internal/types/cas_errors.go` 放三个 sentinel，service 与 middleware 共用。

- [ ] **Step 3: 改 `handler/cas_auth.go`**

```go
ticketCookie, casSid, _ := /* 同 helper：可抽到 internal/cascookie 避免 handler→middleware 依赖，或把 helper 放到 internal/cascookie 包 */

casUserInfo, err := h.casAuthService.ResolveCASUserFromCookies(ctx, ticketCookie, casSid)
if errors.Is(err, types.ErrCASCredentialsMissing) {
	// 现有 401 JSON code 10011
}
if errors.Is(err, types.ErrCASTicketInvalid) {
	// 401 会话失败
}
if err != nil {
	c.Error(errors.NewInternalServerError("CAS 用户中心不可用"))
	return
}
// AutoBind + GenerateTokens — unchanged
```

为避免 `handler` import `middleware`，把 `readCASAuthCookies` 放到新包：

- Create: `internal/cascookie/cookies.go`（`Read(c *gin.Context, cfg) (ticket, sid, uid string)`）
- middleware 与 handler 都依赖 `cascookie`

- [ ] **Step 4: 确认 `jwtIdentityConflictsWithCASCookie`**

保持：仅 `CookieUID` 非空时比对 `user.CASUserID`；无 uid 返回 false。补一条单测：仅有 ticketCookie、无 uid → 不冲突。

- [ ] **Step 5: 跑测**

```bash
go test ./internal/middleware/ ./internal/handler/ ./internal/cascookie/ ./internal/application/service/ -count=1
```

Expected: PASS（handler 若无测可只编过 `go build ./...`）

```bash
go build ./...
```

- [ ] **Step 6: Commit（用户要求时）**

```bash
git commit -m "feat(cas): wire ticketCookie/sid dual-channel into middleware and /cas/validate"
```

---

### Task 4: 文档注释与联调清单

**Files:**
- Modify: `internal/middleware/auth.go` 顶部 Auth 通道注释
- Modify: `internal/handler/cas_auth.go` swagger 描述（Cookie：ticketCookie 或 cookie_sid，uid 非必须）

- [ ] **Step 1: 更新注释/Swagger 文本**，与 spec §2 一致。

- [ ] **Step 2: 本地联调（手动，需活票）**

```bash
# 1) ticketCookie
curl -sk "https://zsk.t.nxin.com/api/v1/cas/validate" -H "Cookie: ticketCookie=<LIVE>"

# 2) sid only
curl -sk "https://zsk.t.nxin.com/api/v1/cas/validate" -H "Cookie: _cas_t_sid=<LIVE_SID>"

# 3) 业务 API
curl -sk "https://zsk.t.nxin.com/api/v1/knowledge-bases" -H "Cookie: ticketCookie=<LIVE>"
```

Expected: validate `code=0` 且含 token；业务 API 200。

- [ ] **Step 3: 确认 NXIN 环境 `CAS_UC_*` 已注入**（测试/生产 compose）；缺 CERT 时 validate 应 503 而非静默成功。

---

## Spec coverage checklist

| Spec 要求 | Task |
|-----------|------|
| ticketCookie → ZNT → getUserArchive | 1, 2, 3 |
| cookie_sid → UcTicket → getUserArchive | 1, 2, 3 |
| 共用中间件 + validate | 3 |
| 移除 uid 前置 / user.get 主路径 | 3 |
| JWT 无 uid 不冲突 | 3 |
| CAS_UC_* / 未配置失败 | 1, 2 |
| 缓存 channel+token | 3 |
| 失败 401/503 | 2, 3 |
| 单测优先级 | 1, 2 |

---

## 执行方式

Plan 已保存。实现可选：

1. **Subagent-Driven（推荐）** — 每 Task 独立子代理 + 间次审查  
2. **Inline Execution** — 本会话按 Task 连续改代码并设检查点  

回复 **1** 或 **2** 开始实现（默认不自动 commit，除非你明确要求提交）。
