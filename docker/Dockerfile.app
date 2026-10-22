# Build extension and daemon from the same pinned source on the runtime architecture.
FROM --platform=$TARGETPLATFORM node:24-bookworm-slim@sha256:ba849c60be29959425b8734d57b8b4b7d56f98edd9504c9af091d5281095a71e AS browserskill
WORKDIR /build
# Use the same optional Debian mirror as the Go builder and runtime stages.
ARG APK_MIRROR_ARG
# This base image has no CA certificates. Install them from the original HTTP
# Debian source before swapping in a mirror: mirrors.tencent.com redirects
# apt to HTTPS, and that handshake fails without a trust store.
RUN if [ -n "$APK_MIRROR_ARG" ]; then \
        apt-get update && \
        apt-get install -y --no-install-recommends ca-certificates && \
        rm -rf /var/lib/apt/lists/* && \
        sed -i "s@deb.debian.org@${APK_MIRROR_ARG}@g" /etc/apt/sources.list.d/debian.sources; \
    fi && \
    apt-get update && \
    apt-get install -y --no-install-recommends git python3 ca-certificates curl build-essential cmake pkg-config && \
    rm -rf /var/lib/apt/lists/*
ENV RUSTUP_HOME=/usr/local/rustup CARGO_HOME=/usr/local/cargo
ENV PATH=/usr/local/cargo/bin:$PATH
RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y --profile minimal --default-toolchain stable
COPY scripts/build_browserskill.sh scripts/browserskill-release.json ./scripts/
ARG TARGETOS
ARG TARGETARCH
RUN bash scripts/build_browserskill.sh /opt/weknora/browserskill "${TARGETOS}/${TARGETARCH}"

# Anydoc static library. Kept out of the app source layer so a gallery or
# handler edit does not reinstall Rust or recompile this archive. The script
# resolves the crate from its own location, so the copy layout must stay
# /src/scripts and /src/third_party/anydoc-go.
FROM golang:1.26-bookworm AS anydoc
WORKDIR /src
ENV RUSTUP_HOME=/usr/local/rustup CARGO_HOME=/usr/local/cargo
ENV PATH=/usr/local/cargo/bin:$PATH
RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs \
    | sh -s -- -y --profile minimal --default-toolchain 1.99.1
COPY scripts/build-anydoc-lib.sh scripts/build-anydoc-lib.sh
COPY third_party/anydoc-go third_party/anydoc-go
RUN --mount=type=cache,target=/usr/local/cargo/registry \
    --mount=type=cache,target=/usr/local/cargo/git \
    --mount=type=cache,target=/src/third_party/anydoc-go/target \
    bash scripts/build-anydoc-lib.sh

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

# Copy go mod files. go.mod replace-points anydoc at ./third_party/anydoc-go,
# so that module's go.mod must exist before `go mod download`.
COPY go.mod go.sum ./
# KEEP-OPS-01: anydoc go.mod required before download. Do not cache-mount
# /go/pkg/mod over this layer; an empty mount would hide the modules just downloaded.
COPY third_party/anydoc-go/go.mod third_party/anydoc-go/go.mod
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
RUN --mount=type=cache,target=/go/pkg/mod bash ./scripts/copy-licenses.sh /license-bundle

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

# UP-DOC-01 anydoc. The archive comes from the anydoc stage, after COPY . .
# so a source checkout cannot replace it with a stale or missing file.
# cgo looks for lib/linux_amd64_gnu on the Debian builder (linux && !musl).
ARG WITH_ANYDOC=1
COPY --from=anydoc /src/third_party/anydoc-go/lib/linux_amd64_gnu/libanydoc_go.a \
    /app/third_party/anydoc-go/lib/linux_amd64_gnu/libanydoc_go.a
RUN --mount=type=cache,target=/root/.cache/go-build \
    if [ "$WITH_ANYDOC" = "1" ]; then \
        make build-prod GO_BUILD_TAGS=anydoc; \
    else \
        make build-prod; \
    fi
RUN cp -r /go/pkg/mod/github.com/yanyiwu/ /app/yanyiwu/

# Final stage
FROM debian:12.12-slim

WORKDIR /app

ARG APK_MIRROR_ARG

# Pairing derives the gateway URL from the user's page origin by default.
ENV BROWSERSKILL_BINARY=/opt/weknora/browserskill/bsk \
    BROWSERSKILL_EXTENSION_PATH=/opt/weknora/browserskill/browser-skill-weknora-0.3.1.zip
COPY --from=browserskill /opt/weknora/browserskill /opt/weknora/browserskill

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
COPY --from=builder /root/.duckdb /home/appuser/.duckdb
COPY --from=builder /app/WeKnora .
COPY --from=builder /license-bundle/ ./

# Copy and make entrypoint script executable
COPY --from=builder /app/scripts/docker-entrypoint.sh ./scripts/docker-entrypoint.sh

# Make scripts executable
RUN chmod +x ./scripts/*.sh

# Expose ports
EXPOSE 8080


ENTRYPOINT ["./scripts/docker-entrypoint.sh"]
CMD ["./WeKnora"]
