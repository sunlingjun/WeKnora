# ParadeDB Docker 镜像升级操作说明

本文说明在本地 Docker（含 Docker Desktop）环境下，将 **ParadeDB** 数据容器从 **`paradedb/paradedb:v0.21.4-pg17`** 升级至 **`paradedb/paradedb:v0.22.2-pg17`** 的步骤与注意事项。源、目标均为 **PostgreSQL 17**，沿用现有数据卷即可，无需跨大版本 `pg_upgrade`。

---

## 1. 适用范围

| 场景 | Compose 文件 | 容器名（示例） | 数据卷（示例） |
|------|--------------|----------------|----------------|
| 本地开发（仅基础设施） | `docker-compose.dev.yml` | `WeKnora-postgres-dev` | `postgres-data-dev` |
| 完整栈 / 生产式本地 | `docker-compose.yml` | `WeKnora-postgres` | `postgres-data` |
| 测试栈 | `docker-compose.test.yml` | 以该文件内 `container_name` 为准 | 以该文件内 `volumes` 为准 |

升级前请确认当前运行的容器与卷与上表一致，避免误操作其它环境。

**仓库内镜像标签现状（便于对齐）：**

- `docker-compose.dev.yml`、`docker-compose.yml`：`v0.22.2-pg17`
- `docker-compose.test.yml`：当前为 `v0.21.4-pg17`；若测试环境需与开发一致，可单独将测试 compose 中的镜像改为 `v0.22.2-pg17` 并按本文流程对**对应容器与卷**执行升级

---

## 2. 原理简述

- ParadeDB 检索能力通过 PostgreSQL 扩展 **`pg_search`** 提供。
- 更换 Docker 镜像后，必须在**已安装 `pg_search` 的每个数据库**中执行 **`ALTER EXTENSION pg_search UPDATE TO '…'`**，版本号以新镜像内为准。
- 官方升级说明：<https://docs.paradedb.com/deploy/upgrading>

---

## 3. 升级前准备

### 3.1 环境变量

在仓库根目录使用与启动 Compose **相同的** `.env`（或 `--env-file`），确认：

- `DB_USER`、`DB_PASSWORD`、`DB_NAME`
- `DB_PORT`（默认 `5432`）

下文用 `<DB_USER>`、`<DB_PASSWORD>`、`<DB_NAME>` 表示占位符。

### 3.2 建议操作

- 暂停依赖本机 Postgres 的应用进程（如本地运行的 WeKnora `app`），避免升级瞬间连接失败。
- 在仓库根目录执行后续命令，例如：

  ```powershell
  cd E:\Tencent\WeKnora-slj
  ```

---

## 4. 备份（必做）

### 4.1 全库逻辑备份（推荐）

**PowerShell 示例**（开发容器 `WeKnora-postgres-dev`）：

```powershell
docker exec WeKnora-postgres-dev pg_dumpall -U <DB_USER> -f /tmp/backup_all.sql
docker cp WeKnora-postgres-dev:/tmp/backup_all.sql E:\backup-paradedb-dev-$(Get-Date -Format yyyyMMdd-HHmmss).sql
```

若使用 `docker-compose.yml` 中的容器 `WeKnora-postgres`，将容器名替换为 `WeKnora-postgres`。

### 4.2 确认数据卷

```powershell
docker volume ls | findstr postgres
```

**禁止**在未完成备份的情况下删除 `postgres-data-dev` 或 `postgres-data` 等卷。

---

## 5. 升级镜像并重建容器

以下以**开发环境**为例；其它环境将 `-f docker-compose.dev.yml` 与容器名改为对应文件及容器即可。

### 5.1 确认 Compose 中镜像

`docker-compose.dev.yml` 中 `postgres` 服务应为：

```yaml
image: paradedb/paradedb:v0.22.2-pg17
```

### 5.2 拉取镜像并仅重建 Postgres

```powershell
docker compose -f docker-compose.dev.yml --env-file .env pull postgres
docker compose -f docker-compose.dev.yml --env-file .env stop postgres
docker compose -f docker-compose.dev.yml --env-file .env up -d postgres
```

### 5.3 确认运行镜像与健康检查

```powershell
docker ps --filter name=WeKnora-postgres-dev
```

应显示镜像 **`paradedb/paradedb:v0.22.2-pg17`**，状态在数秒内变为 `healthy`。

---

## 6. 升级扩展 `pg_search`（必做）

### 6.1 查询可用版本

```powershell
docker exec -e PGPASSWORD=<DB_PASSWORD> -it WeKnora-postgres-dev psql -U <DB_USER> -d <DB_NAME> -c "SELECT * FROM pg_available_extension_versions WHERE name = 'pg_search';"
```

在 实际连接的应用库 上执行（不是只看书面配置）：
SELECT extname, extversion FROM pg_extension WHERE extname = 'pg_search';
-- 若 ParadeDB 提供：
SELECT * FROM paradedb.version_info();
若 extversion 明显低于镜像标签中的 pg_search 版本，或文档要求的版本：
ALTER EXTENSION pg_search UPDATE TO '<目标版本>';  -- 版本号以官方 upgrading 文档为准

记录目标版本字符串（通常与镜像小版本一致，如 **`0.22.2`**，以查询结果为准）。

### 6.2 执行升级

将 `<VERSION>` 替换为上一步结果中的版本：

```powershell
docker exec -e PGPASSWORD=<DB_PASSWORD> -it WeKnora-postgres-dev psql -U <DB_USER> -d <DB_NAME> -c "ALTER EXTENSION pg_search UPDATE TO '<VERSION>';"
```

若其它数据库也安装了 `pg_search`，需对每个库分别执行。可在 `psql` 中 `\l` 列出库，再 `\connect <库名>` 后检查 `pg_extension`。

---

## 7. 验证

```powershell
docker exec -e PGPASSWORD=<DB_PASSWORD> -it WeKnora-postgres-dev psql -U <DB_USER> -d <DB_NAME> -c "SELECT extversion FROM pg_extension WHERE extname = 'pg_search'; SELECT * FROM paradedb.version_info();"
```

- `extversion` 与 `paradedb.version_info()` 应对齐；若不一致，可先重启容器后再查：

  ```powershell
  docker restart WeKnora-postgres-dev
  ```

应用侧：启动后端，验证登录及依赖检索/BM25 的功能。

---

## 8. Collation 版本不一致告警处理（重要）

升级到新镜像后，若日志持续出现如下内容：

- `database "...\" has a collation version mismatch`
- `The database was created using collation version 2.36, but the operating system provides version 2.41`

这是因为旧数据目录在旧 glibc/locale 版本下创建，新镜像内系统库版本更高。该告警常由健康检查与应用连接反复触发（每次新连接都会打印）。

### 8.1 先确认哪些库受影响

```powershell
docker exec -e PGPASSWORD=<DB_PASSWORD> -it WeKnora-postgres-dev psql -U <DB_USER> -d postgres -c "SELECT datname, datcollversion FROM pg_database ORDER BY datname;"
```

常见会看到 `WeKnora`、`postgres`、`template1` 显示旧版本（如 `2.36`）。

### 8.2 刷新数据库 collation 版本标记

```powershell
docker exec -e PGPASSWORD=<DB_PASSWORD> -it WeKnora-postgres-dev psql -U <DB_USER> -d postgres -c "ALTER DATABASE \"WeKnora\" REFRESH COLLATION VERSION; ALTER DATABASE postgres REFRESH COLLATION VERSION; ALTER DATABASE template1 REFRESH COLLATION VERSION;"
```

再次确认：

```powershell
docker exec -e PGPASSWORD=<DB_PASSWORD> -it WeKnora-postgres-dev psql -U <DB_USER> -d postgres -c "SELECT datname, datcollversion FROM pg_database ORDER BY datname;"
```

### 8.3 重建受影响文本索引（建议在低峰执行）

仅刷新版本标记不一定覆盖历史索引；建议在业务库执行 `REINDEX`：

```powershell
docker exec -e PGPASSWORD=<DB_PASSWORD> -it WeKnora-postgres-dev psql -U <DB_USER> -d postgres -c "REINDEX DATABASE \"WeKnora\";"
```

如有需要，再对 `postgres`、`template1` 执行同样操作（通常对象较少，可按需处理）。

### 8.4 重启并观察日志

```powershell
docker restart WeKnora-postgres-dev
docker logs -f WeKnora-postgres-dev
```

若后续新连接不再触发旧版本告警，说明处理完成。

---

## 9. 回滚说明

- 优先使用 **第 4 节** 的 `pg_dumpall` 在干净实例或新卷上恢复（需单独规划恢复步骤与停机窗口）。
- 若有卷级备份，可按 Docker / 运维规范恢复对应 volume。

---

## 10. 常见问题

| 现象 | 处理 |
|------|------|
| `docker ps` 仍为 `v0.21.4` | 确认已执行 `pull`、`stop`、`up -d`，且使用的 compose 文件与启动命令一致。 |
| `ALTER EXTENSION` 报版本不存在 | 必须以 `pg_available_extension_versions` 中的版本为准。 |
| 日志持续刷 `collation version mismatch` | 先执行 `ALTER DATABASE ... REFRESH COLLATION VERSION`，再对业务库执行 `REINDEX DATABASE`。 |
| 应用无法连接 | 检查容器 `healthy`、主机端口、`DB_*` 与容器内 `POSTGRES_*` 是否一致。 |

---

## 11. 专项排障：`pdb.score(anyelement) does not exist`

### 11.1 现象

典型报错：

```text
ERROR: function "pdb.score(anyelement)" does not exist (SQLSTATE 42883)
```

常见触发语句：

- `DELETE FROM embeddings WHERE ...`
- `UPDATE embeddings SET is_enabled = ... WHERE ...`

### 11.2 根因（与官方文档对齐）

该报错通常不是业务 SQL 写错，而是 **`pg_search` 扩展版本状态不一致** 导致：

- 已升级/替换了 ParadeDB 镜像；
- 但未在对应数据库执行 `ALTER EXTENSION pg_search UPDATE TO '...';`；
- 或者同一数据卷在不同镜像版本间切换，导致扩展 SQL 对象与当前二进制不匹配。

在这种状态下，BM25 索引维护路径（写入/更新/删除时）会调用内部函数（如 `pdb.score`），若函数签名在当前库中不可用，就会抛出 `... does not exist`。

> 说明：官方升级文档重点在升级步骤，不会在功能章节逐条罗列该类异常。  
> 参考：<https://docs.paradedb.com/deploy/upgrading>

### 11.3 快速诊断

在业务库执行：

```powershell
docker exec -e PGPASSWORD=<DB_PASSWORD> -it WeKnora-postgres-dev psql -U <DB_USER> -d <DB_NAME> -c "SELECT extname, extversion FROM pg_extension WHERE extname='pg_search'; SELECT * FROM paradedb.version_info();"
```

重点检查：

- `extversion` 是否与镜像目标版本一致；
- `paradedb.version_info()` 与 `pg_extension` 结果是否一致；
- 是否所有安装 `pg_search` 的数据库都已执行过升级。

### 11.4 推荐处理顺序

1. 按 **第 6 节** 在每个安装 `pg_search` 的数据库执行 `ALTER EXTENSION ... UPDATE TO ...`。  
2. 重启容器并复查版本（见 **第 7 节**）。  
3. 重新执行触发失败的 `DELETE/UPDATE` 流程验证。  
4. 若仍失败，再检查是否存在跨环境复用旧卷、连接错库、或多套 compose 版本不一致（`dev`/`default`/`test`）。

### 11.5 应急方案（仅临时兜底）

如果业务必须立即删除数据，可临时采用：

- 删除 BM25 索引 `embeddings_search_idx`；
- 执行 `DELETE/UPDATE`；
- 再重建索引。

注意：索引删除到重建期间，关键词/BM25 检索会短暂不可用；该方案用于应急，不应替代扩展升级。

---

## 12. 相关文件

- 开发：`docker-compose.dev.yml`
- 默认完整栈：`docker-compose.yml`
- 测试：`docker-compose.test.yml`
- 库表与扩展初始化参考：`migrations/paradedb/00-init-db.sql`

---

*文档创建日期：2026-04-07*
