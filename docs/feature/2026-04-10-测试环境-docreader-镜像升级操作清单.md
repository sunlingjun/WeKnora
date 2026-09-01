# 测试环境 Docreader 镜像升级操作清单（直接拉取最新版）

## 1. 适用范围

- 环境：测试环境
- 编排文件：`docker-compose.test.yml`
- 目标服务：`docreader`
- 镜像：`wechatopenai/weknora-docreader:latest`
- 容器名（按 compose 定义）：`WeKnora-docreader-test`

---

## 2. 升级目标

- 不使用本地构建与导入 tar 包
- 在测试机直接 `docker pull` 拉取 `latest`
- 仅重建 `docreader` 服务，不重启其他服务
- 提供可快速回滚的标准流程

---

## 3. 前置检查

在测试机执行：

```bash
docker version
docker compose version
docker ps --format "table {{.Names}}\t{{.Image}}\t{{.Status}}" | grep -E "WeKnora-docreader-test|WeKnora-app-test|WeKnora-minio-test"
```

预期：
- `docker` 与 `docker compose` 命令可用
- `WeKnora-docreader-test`、`WeKnora-app-test`、`WeKnora-minio-test` 在运行

---

## 4. 标准升级步骤（可复制执行）

### 4.1 进入项目目录并确认 compose 文件

```bash
cd /opt/weknora
ls -l docker-compose.test.yml
```

### 4.2 记录当前版本并打回滚标签

```bash
cd /opt/weknora

# 记录当前容器镜像与镜像ID（留档）
docker inspect WeKnora-docreader-test --format 'before image={{.Config.Image}} image_id={{.Image}}'

# 给当前 latest 打回滚标签（务必保留）
ROLLBACK_TAG=rollback-$(date +%Y%m%d-%H%M%S)
docker image tag wechatopenai/weknora-docreader:latest wechatopenai/weknora-docreader:${ROLLBACK_TAG}
echo "ROLLBACK_TAG=${ROLLBACK_TAG}"
```

### 4.3 拉取最新版镜像

```bash
docker pull wechatopenai/weknora-docreader:latest
docker image ls | grep weknora-docreader
docker inspect wechatopenai/weknora-docreader:latest --format 'latest digest={{index .RepoDigests 0}}'
```

### 4.4 仅重建 docreader 服务

```bash
cd /opt/weknora
docker compose -f docker-compose.test.yml up -d --no-deps --force-recreate docreader
```

---

## 5. 升级后验证

```bash
# 1) 容器状态与健康
docker ps --format "table {{.Names}}\t{{.Image}}\t{{.Status}}" | grep WeKnora-docreader-test

# 2) 核对容器当前镜像ID
docker inspect WeKnora-docreader-test --format 'after image={{.Config.Image}} image_id={{.Image}}'

# 3) 查看 docreader 启动日志
docker logs --tail=200 WeKnora-docreader-test

# 4) 查看 app 日志，确认无 docreader 调用异常
docker logs --tail=200 WeKnora-app-test
```

如服务端已启用系统重连接口，可执行：

```bash
curl -X POST "http://127.0.0.1:8080/api/v1/system/docreader/reconnect" \
  -H "Content-Type: application/json" \
  -d '{}'
```

建议再做一次功能回归：
- 上传并解析 1 个 PDF
- 上传并解析 1 个 DOCX
- 执行 1 次 URL 导入解析

---

## 6. 一键执行脚本（含自动回滚）

说明：该脚本会先打回滚标签，拉取最新镜像，重建 `docreader`，并做基础健康验证；若失败会自动回滚。

```bash
#!/usr/bin/env bash
set -euo pipefail

WORKDIR="/opt/weknora"
COMPOSE_FILE="docker-compose.test.yml"
SERVICE="docreader"
CONTAINER="WeKnora-docreader-test"
IMAGE="wechatopenai/weknora-docreader:latest"

cd "${WORKDIR}"

echo "[1/6] 记录升级前信息..."
BEFORE_IMAGE_ID=$(docker inspect "${CONTAINER}" --format '{{.Image}}')
echo "BEFORE_IMAGE_ID=${BEFORE_IMAGE_ID}"

echo "[2/6] 打回滚标签..."
ROLLBACK_TAG="rollback-$(date +%Y%m%d-%H%M%S)"
docker image tag "${IMAGE}" "wechatopenai/weknora-docreader:${ROLLBACK_TAG}"
echo "ROLLBACK_TAG=${ROLLBACK_TAG}"

rollback() {
  echo "[ROLLBACK] 检测到异常，开始回滚..."
  docker image tag "wechatopenai/weknora-docreader:${ROLLBACK_TAG}" "${IMAGE}"
  docker compose -f "${COMPOSE_FILE}" up -d --no-deps --force-recreate "${SERVICE}"
  docker ps --format "table {{.Names}}\t{{.Image}}\t{{.Status}}" | grep "${CONTAINER}" || true
  echo "[ROLLBACK] 回滚完成。"
}

trap 'rollback' ERR

echo "[3/6] 拉取最新镜像..."
docker pull "${IMAGE}"

echo "[4/6] 重建 docreader..."
docker compose -f "${COMPOSE_FILE}" up -d --no-deps --force-recreate "${SERVICE}"

echo "[5/6] 基础校验..."
sleep 5
docker ps --format "table {{.Names}}\t{{.Image}}\t{{.Status}}" | grep "${CONTAINER}"
docker logs --tail=100 "${CONTAINER}" >/dev/null

echo "[6/6] 输出升级后信息..."
AFTER_IMAGE_ID=$(docker inspect "${CONTAINER}" --format '{{.Image}}')
echo "AFTER_IMAGE_ID=${AFTER_IMAGE_ID}"
echo "升级完成。若 AFTER_IMAGE_ID 与 BEFORE_IMAGE_ID 不同，表示镜像已更新。"

trap - ERR
```

---

## 7. 手工回滚步骤

若升级后出现异常，可使用上面记录的 `ROLLBACK_TAG` 回滚：

```bash
cd /opt/weknora

# 将回滚标签重新打回 latest
docker image tag wechatopenai/weknora-docreader:${ROLLBACK_TAG} wechatopenai/weknora-docreader:latest

# 仅重建 docreader
docker compose -f docker-compose.test.yml up -d --no-deps --force-recreate docreader

# 验证
docker ps --format "table {{.Names}}\t{{.Image}}\t{{.Status}}" | grep WeKnora-docreader-test
docker logs --tail=200 WeKnora-docreader-test
```

---

## 8. 执行记录模板（建议）

- 执行人：
- 执行时间：
- 升级前镜像ID：
- 升级后镜像ID：
- 回滚标签：
- 结果（成功/失败）：
- 备注：

