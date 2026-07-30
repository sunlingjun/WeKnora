# Build stage
FROM golang:1.26-bookworm AS builder

WORKDIR /app

# 通过构建参数接收敏感信息
ARG GOPRIVATE_ARG
# 国内构建机访问 proxy.golang.org 常超时；goproxy.cn 偶发 HTTP/2 GOAWAY，故默认多源 + 关闭 HTTP/2
ARG GOPROXY_ARG=https://goproxy.cn,https://mirrors.aliyun.com/goproxy/,direct
ARG GOSUMDB_ARG=off
ARG APK_MIRROR_ARG

# 设置Go环境变量
ENV GOPRIVATE=${GOPRIVATE_ARG}
ENV GOPROXY=${GOPROXY_ARG}
ENV GOSUMDB=${GOSUMDB_ARG}
# 规避 goproxy.cn 大批量下载时 http2 GOAWAY 断连
ENV GODEBUG=http2client=0

# Install dependencies
RUN if [ -n "$APK_MIRROR_ARG" ]; then \
        sed -i "s@deb.debian.org@${APK_MIRROR_ARG}@g" /etc/apt/sources.list.d/debian.sources; \
    fi && \
    apt-get update && \
    apt-get install -y git build-essential libsqlite3-dev curl ca-certificates gzip

# Install migrate tool（网络抖动时重试；避免 dash 下 $((...)) 语法问题）
RUN go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest \
    || (echo "migrate install retry 1" && sleep 5 && go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest) \
    || (echo "migrate install retry 2" && sleep 10 && go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest) \
    || (echo "migrate install retry 3" && sleep 15 && go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest) \
    || (echo "migrate install retry 4" && sleep 20 && go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest)

# Copy go mod and sum files
COPY go.mod go.sum ./
# go mod download：HTTP/2 GOAWAY / 代理抖动时重试
# 不使用 RUN --mount（需 BuildKit）；兼容生产 Jenkins DOCKER_BUILDKIT=0
RUN go mod download \
    || (echo "go mod download retry 1" && sleep 5 && go mod download) \
    || (echo "go mod download retry 2" && sleep 10 && go mod download) \
    || (echo "go mod download retry 3" && sleep 15 && go mod download) \
    || (echo "go mod download retry 4" && sleep 20 && go mod download)
COPY cmd/download cmd/download
# DuckDB 扩展：先使用 curl 预下载并解压，避免 DuckDB INSTALL 在构建时触发更短超时的远程下载。
#
# 注意：URL 与路径版本号来自 DuckDB extension release（当前失败日志为 v1.5.2）。
ARG DUCKDB_EXTENSION_VERSION=1.5.2
RUN mkdir -p "/root/.duckdb/extensions/v${DUCKDB_EXTENSION_VERSION}/linux_amd64" && \
    for ext in httpfs spatial excel; do \
      url="https://extensions.duckdb.org/v${DUCKDB_EXTENSION_VERSION}/linux_amd64/${ext}.duckdb_extension.gz"; \
      echo "Downloading DuckDB extension: ${url}"; \
      curl -fsSL \
        --retry 5 --retry-delay 10 \
        --connect-timeout 30 \
        --max-time 600 \
        -o "/root/.duckdb/extensions/v${DUCKDB_EXTENSION_VERSION}/linux_amd64/${ext}.duckdb_extension.gz" \
        "${url}"; \
      gunzip -f "/root/.duckdb/extensions/v${DUCKDB_EXTENSION_VERSION}/linux_amd64/${ext}.duckdb_extension.gz"; \
    done
RUN go run cmd/download/duckdb/duckdb.go \
    || (echo "duckdb extensions retry 1" && sleep 5 && go run cmd/download/duckdb/duckdb.go) \
    || (echo "duckdb extensions retry 2" && sleep 10 && go run cmd/download/duckdb/duckdb.go) \
    || (echo "duckdb extensions retry 3" && sleep 15 && go run cmd/download/duckdb/duckdb.go) \
    || (echo "duckdb extensions retry 4" && sleep 20 && go run cmd/download/duckdb/duckdb.go)
COPY . .

# Get version and commit info for build injection
ARG VERSION_ARG
ARG COMMIT_ID_ARG
ARG BUILD_TIME_ARG
ARG GO_VERSION_ARG

# Set build-time variables
ENV VERSION=${VERSION_ARG}
ENV COMMIT_ID=${COMMIT_ID_ARG}
ENV BUILD_TIME=${BUILD_TIME_ARG}
ENV GO_VERSION=${GO_VERSION_ARG}

# Build the application with version info（无 BuildKit cache mount，兼容 legacy builder）
RUN make build-prod
RUN cp -r /go/pkg/mod/github.com/yanyiwu/ /app/yanyiwu/

# Final stage
FROM debian:12.12-slim

WORKDIR /app

ARG APK_MIRROR_ARG

# Create a non-root user first
RUN useradd -m -s /bin/bash appuser

# First, install ca-certificates without mirror to ensure HTTPS works
RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates && \
    rm -rf /var/lib/apt/lists/*

# Then switch to mirror if specified and install other packages
RUN if [ -n "$APK_MIRROR_ARG" ]; then \
        sed -i "s@deb.debian.org@${APK_MIRROR_ARG}@g" /etc/apt/sources.list.d/debian.sources; \
    fi && \
    apt-get update && \
    apt-get install -y --no-install-recommends \
        build-essential postgresql-client default-mysql-client tzdata sed curl bash vim wget \
        libsqlite3-0 \
        python3 python3-pip python3-dev libffi-dev libssl-dev \
        nodejs npm \
        gosu \
        ffmpeg && \
    python3 -m pip install --break-system-packages --upgrade pip setuptools wheel && \
    mkdir -p /home/appuser/.local/bin && \
    curl -LsSf https://astral.sh/uv/install.sh | CARGO_HOME=/home/appuser/.cargo UV_INSTALL_DIR=/home/appuser/.local/bin sh && \
    chown -R appuser:appuser /home/appuser && \
    ln -sf /home/appuser/.local/bin/uvx /usr/local/bin/uvx && \
    chmod +x /usr/local/bin/uvx && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

# Create data directories and set permissions
RUN mkdir -p /data/files && \
    chown -R appuser:appuser /app /data/files

# Copy migrate tool from builder stage
COPY --from=builder /go/bin/migrate /usr/local/bin/
COPY --from=builder /app/yanyiwu/ /go/pkg/mod/github.com/yanyiwu/

# Copy the binary from the builder stage
COPY --from=builder /app/config ./config
COPY --from=builder /app/scripts ./scripts
COPY --from=builder /app/migrations ./migrations
COPY --from=builder /app/dataset/samples ./dataset/samples
COPY --from=builder /app/skills/preloaded ./skills/preloaded
# Keep a read-only backup so bind-mount cannot erase built-in skills
COPY --from=builder /app/skills/preloaded ./skills/_builtin
COPY --from=builder /root/.duckdb /home/appuser/.duckdb
COPY --from=builder /app/WeKnora .

# Copy and make entrypoint script executable
COPY --from=builder /app/scripts/docker-entrypoint.sh ./scripts/docker-entrypoint.sh

# Make scripts executable
RUN chmod +x ./scripts/*.sh

# Expose ports
EXPOSE 8080


ENTRYPOINT ["./scripts/docker-entrypoint.sh"]
CMD ["./WeKnora"]
