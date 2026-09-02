# 2026-09-02 website-docs 部署方案（方案 A）

> **结论**：同源 `/docs/`，独立 docs 镜像，由 **frontend nginx 反代**；本地/测试单机一份，生产三机每台一份。  
> **访问**：`/docs/` **允许未登录**（CAS 若以后要挡，放网关，不放文档容器）。  
> **修订**：取代 [2026-08-14](./2026-08-14-升级至v0.7.2-WeKnora-slj与NXIN先后方案.md) §4.5「website-docs 容器不部署 / 仅内网预览」的默认。

---

## 1. 为什么是方案 A（不是更少容器的 C）

`website-docs` 是 VitePress 静态站（`base: /docs/`，`cleanUrls: true`）。要让用户打开 `https://zsk.nxin.com/docs/`，**nginx 配置不可避免**：frontend 现有 `location /` 会把未知路径 fallback 到 Vue `index.html`，不单独写 `/docs/` 就会把文档站吃掉。

| 方案 | 独立镜像 | 改 nginx | 适用 |
|------|----------|----------|------|
| **A（本方案）** | 要 | **frontend 只加反代**；VitePress 的 alias / cleanUrls / 资源长缓存已经在 `website-docs/nginx.conf` | 同源 `/docs/`，文档与前端可分开发版 |
| B 独立端口 `:8081` | 要 | 可不改 frontend | 入口分裂，生产还要单独负载，否决 |
| C 打进 frontend 镜像 | 不要 | 要在 **同一份** frontend nginx 里同时写 Vue SPA 和 VitePress 两套 `try_files` | 少一个容器，但发版绑死、配置易踩 alias 坑 |

**A 仍是最优**，原因就三条：

1. **两套静态站规则分开**：Vue 是 `try_files … /index.html`；VitePress 是 `/docs` + `alias` + `$uri.html`。官方镜像已经写好后者，frontend 只 `proxy_pass`，不必把两套 fallback 揉进一个 conf。
2. **和现有交付物对齐**：仓库已有 `website-docs/Dockerfile`，官方注释就是 `location /docs/ { proxy_pass http://weknora-docs:80/docs/; }`。
3. **无状态、副本便宜**：文档不碰 DB/Redis/卷。本地/测试 1 个容器，生产 3 台各 1 个，编排同一套；文档改版不必重打 frontend。

独立镜像 **不是**再搭一套业务服务，只是把「已构建的静态文件 + 专用 nginx」打成可在三机上 `docker compose up` 的单元。不走独立镜像就只能走 C（拷进 frontend）或手工 rsync dist（三机易漂）。

**不要**再在宿主机/LB 上配第三层 `/docs/`（除非公司统一入口禁止改 frontend）。入口已经是 frontend `:80`，反代放这里就能随镜像走三机，不用另开基础设施单。

---

## 2. 目标形态

```
浏览器  未登录即可
  │  GET /docs/…
  ▼
[入口 80/443]  本地/测试：单机 frontend
               生产：LB → 三台机器上的 frontend（各一份）
  │
  ▼
frontend nginx :80
  ├─ /api/  /files  /r/  → app
  ├─ /docs/              → docs :80     ← 新增，^~ 优先于 SPA
  └─ /                   → Vue SPA
```

| 环境 | 拓扑 | 访问 |
|------|------|------|
| 本地写作 | 不走 Docker：`cd website-docs && npm run dev` | VitePress 开发端口 |
| 本地验收镜像 | 单机 compose，profile `docs` + `frontend` | `http://localhost/docs/` |
| 测试 | 单机 Docker，与生产同一编排 | `https://zsk.t.nxin.com/docs/` |
| 生产 | 三机，**跑 frontend 的节点各起一份 docs** | `https://zsk.nxin.com/docs/` |

文档容器不做鉴权、不挂卷、不依赖 app 健康。frontend 挂了 SPA 不可用，docs 仍可在排障时用宿主机 `8081:80` 直连（可选，默认不对外映射）。

---

## 3. 落地清单（实现时按此改）

### 3.1 独立镜像 `weknora-docs`

- 构建必须在**仓库根**（读取 `VERSION`）：
  `docker build --platform linux/amd64 -f website-docs/Dockerfile -t weknora-docs:${WEKNORA_VERSION} .`
- 与 app/frontend 一样：**Jenkins/本机构建，禁止 pull Hub `latest`**。
- 生产阶段基础镜像与 frontend **同一 digest**（`nginx:1.30.3-alpine@sha256:0d3b80406a13a767339fbe2f41406d6c7da727ab89cf8fae399e81f780f814d1`），不要用浮动 `nginx:stable-alpine`（旧内核已踩过坑）。
- **端口收口**：容器内 nginx **listen 80**（对齐 `EXPOSE 80` 与官方 gateway 注释）。现状 `website-docs/nginx.conf` 的 `listen 8081` 与 Dockerfile 矛盾，实现时改掉。宿主机调试才 `-p 8081:80`。
- 探活：`wget -qO- http://127.0.0.1/docs/`（或 `curl -f`），期望 200。

### 3.2 compose：profile `docs`

在 `docker-compose.yml`（测试/生产共用）增加服务，**默认不随无 profile 的 `up` 起来**，与 `frontend` 一样显式打开：

```yaml
docs:
  image: weknora-docs:${WEKNORA_VERSION:-latest}
  build:
    context: .
    dockerfile: website-docs/Dockerfile
  container_name: WeKnora-docs
  # 不默认映射宿主机端口；走 frontend 反代即可
  # ports:
  #   - "${DOCS_PORT:-8081}:80"
  networks:
    - WeKnora-network
  restart: unless-stopped
  profiles:
    - docs
  healthcheck:
    test: ["CMD", "wget", "-qO-", "http://127.0.0.1/docs/"]
    interval: 30s
    timeout: 5s
    retries: 3
    start_period: 10s
```

本地/测试/生产启动 frontend 时同时加 `--profile docs`（或 `COMPOSE_PROFILES` 含 `frontend,docs`）。

`docker-compose.dev.yml` **不**加 docs：开发机用 `npm run dev` 写文档，避免和宿主机前端抢资源。

### 3.3 frontend nginx：只加反代

在 `frontend/nginx.conf` 里、`location /` **之前**增加（`^~` 避免被 SPA 吃掉）：

```nginx
location ^~ /docs/ {
    proxy_pass http://${DOCS_HOST}:80/docs/;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
}

location = /docs {
    return 302 /docs/;
}
```

`frontend/docker-entrypoint.sh` 的 `envsubst` 增加 `DOCS_HOST`；compose 里 frontend：

```yaml
environment:
  - DOCS_HOST=${DOCS_HOST:-docs}
```

`proxy_pass` 的目标是 **compose 服务名** `docs`（Docker 网络 DNS），不是 `container_name`。

frontend **不要** `depends_on: docs` 做成硬依赖：docs 挂了只影响 `/docs/` 502，不能拖垮登录和问答。打开 profile 后两者独立 `restart: unless-stopped` 即可。

### 3.4 生产三机

| 项 | 做法 |
|----|------|
| 放哪 | 每台已跑 `frontend` 的机器各一份 `docs`，镜像 tag 相同 |
| LB | **不用**为 `/docs/` 单独分流；打到哪台 frontend，由那台反代本机 docs |
| 卷/共享存储 | 无 |
| 滚动 | 构建 → node1 `up -d --no-deps docs` → 打开 `/docs/` → node2 → node3 |
| 回滚 | 三机把 `weknora-docs` 拨回上一 tag，同样 `--no-deps` 重建 |

测试环境单机：同一套命令，只做一次。

---

## 4. 权限与安全

- 文档容器、frontend `/docs/` 反代 **都不做 CAS**。
- 现网 `zsk.nxin.com` 的 CAS 在 Vue 与 `/api/`，静态 `/docs/` 未登录可打开（与本方案确认一致）。
- 若以后要登录才看文档：在公司网关对 `/docs/` 加 CAS，不改 VitePress、不改 docs 镜像。
- 安全头沿用 docs 容器 nginx 已有的 `X-Frame-Options` 等；反代不要用 `proxy_hide_header` 剥掉。

文档内容含架构与 API，等同对内产品说明公开在现网域名下。接受这一点再部署；不接受就不要解析公网，只留内网 LB。

---

## 5. 验收

| # | 环境 | 标准 |
|---|------|------|
| A1 | 本地 compose | `http://localhost/docs/` 首页 200，侧栏可进任意一章；`/docs/assets/` 有长期缓存头 |
| A2 | 本地 | 直接打开 `http://localhost/docs/03-features/01-tenant-auth`（无 `.html`）不 404、不掉进 Vue 登录页 |
| A3 | 测试 | 未登录浏览器隐身窗口打开 `https://zsk.t.nxin.com/docs/` 可完整阅读 |
| A4 | 测试 | 登录后的产品站与 `/docs/` 同宿主，无跨域 |
| A5 | 生产 | 三机各自 `wget -qO- http://127.0.0.1/docs/`（容器内或经 frontend）200；经 LB 抽查三次，落到不同节点仍可用 |
| A6 | 生产 | docs 容器停掉后，`/` 与 `/api/` 仍可用，仅 `/docs/` 为 502 |

---

## 6. 明确不做

- 不把 dist 打进 frontend 镜像（方案 C）。
- 不对外默认暴露 `:8081`（避免和第二入口并存）。
- 不在 docs 里接 CAS、Redis、数据库。
- 不改 VitePress `base`（保持 `/docs/`）。

---

## 7. 实现顺序

1. 收口 `website-docs/nginx.conf` 为 `listen 80`；生产阶段钉 nginx digest。
2. `docker-compose.yml` 增加 `docs` 服务（profile `docs`）。
3. frontend nginx + `envsubst` + `DOCS_HOST`。
4. 本地 `--profile frontend --profile docs` 跑通 A1/A2。
5. 测试单机部署，隐身窗口验收 A3。
6. 生产三机滚动，验收 A5/A6。
