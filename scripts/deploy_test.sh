#!/bin/bash
# WeKnora 测试环境一键部署脚本

set -e

# 设置颜色
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # 无颜色

# 日志函数
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

# 获取项目根目录
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_ROOT="$( cd "$SCRIPT_DIR/.." && pwd )"

echo "=========================================="
echo "WeKnora 测试环境部署脚本"
echo "=========================================="

# 1. 检查环境
log_info "[1/9] 检查环境..."
if ! command -v docker &> /dev/null; then
    log_error "Docker 未安装，请先安装 Docker"
    exit 1
fi

if ! docker compose version &> /dev/null && ! docker-compose version &> /dev/null; then
    log_error "Docker Compose 未安装，请先安装 Docker Compose"
    exit 1
fi

log_success "环境检查通过"

# 2. 创建数据目录
log_info "[2/9] 创建数据目录..."
sudo mkdir -p /data/weknora/{postgres,redis,minio,neo4j,files,logs,backup}
sudo chown -R $USER:$USER /data/weknora
log_success "数据目录创建完成"

# 3. 检查配置文件
log_info "[3/9] 检查配置文件..."
if [ ! -f "$PROJECT_ROOT/docker-compose.test.yml" ]; then
    log_warning "docker-compose.test.yml 不存在，使用 docker-compose.yml"
    COMPOSE_FILE="docker-compose.yml"
else
    COMPOSE_FILE="docker-compose.test.yml"
fi

if [ ! -f "$PROJECT_ROOT/.env" ]; then
    log_warning ".env 文件不存在，请创建并配置环境变量"
    log_info "可以复制 .env.example 或参考文档创建 .env 文件"
fi

# 4. 构建 APP 镜像
log_info "[4/9] 构建 APP 镜像..."
cd "$PROJECT_ROOT"
if [ -f "$PROJECT_ROOT/scripts/build_images.sh" ]; then
    ./scripts/build_images.sh --app || {
        log_error "APP 镜像构建失败"
        exit 1
    }
else
    log_warning "build_images.sh 不存在，跳过 APP 镜像构建"
fi
log_success "APP 镜像构建完成"

# 5. 构建 FRONTEND 镜像
log_info "[5/9] 构建 FRONTEND 镜像..."
if [ -f "$PROJECT_ROOT/scripts/build_images.sh" ]; then
    ./scripts/build_images.sh --frontend || {
        log_error "FRONTEND 镜像构建失败"
        exit 1
    }
else
    log_warning "build_images.sh 不存在，跳过 FRONTEND 镜像构建"
fi
log_success "FRONTEND 镜像构建完成"

# 6. 拉取开源镜像
log_info "[6/9] 拉取开源镜像..."
docker pull paradedb/paradedb:v0.21.4-pg17 || log_warning "PostgreSQL 镜像拉取失败"
docker pull redis:7.0-alpine || log_warning "Redis 镜像拉取失败"
docker pull minio/minio:RELEASE.2025-09-07T16-13-09Z || log_warning "MinIO 镜像拉取失败"
docker pull neo4j:2025.10.1 || log_warning "Neo4j 镜像拉取失败"
docker pull wechatopenai/weknora-docreader:latest || log_warning "Docreader 镜像拉取失败"
log_success "开源镜像拉取完成"

# 7. 启动服务
log_info "[7/9] 启动服务..."
cd "$PROJECT_ROOT"
docker compose -f "$COMPOSE_FILE" up -d || {
    log_error "服务启动失败"
    exit 1
}
log_success "服务启动完成"

# 8. 等待服务就绪
log_info "[8/9] 等待服务就绪..."
sleep 30

# 检查 PostgreSQL
log_info "检查 PostgreSQL..."
for i in {1..30}; do
    if docker compose -f "$COMPOSE_FILE" exec -T postgres pg_isready -U weknora &> /dev/null; then
        log_success "PostgreSQL 已就绪"
        break
    fi
    if [ $i -eq 30 ]; then
        log_warning "PostgreSQL 启动超时，请检查日志"
    fi
    sleep 2
done

# 检查 MinIO
log_info "检查 MinIO..."
for i in {1..30}; do
    if curl -sf http://localhost:9000/minio/health/live &> /dev/null; then
        log_success "MinIO 已就绪"
        break
    fi
    if [ $i -eq 30 ]; then
        log_warning "MinIO 启动超时，请检查日志"
    fi
    sleep 2
done

# 9. 初始化 MinIO Bucket
log_info "[9/9] 初始化 MinIO Bucket..."
sleep 5
docker compose -f "$COMPOSE_FILE" exec -T minio sh -c 'mc alias set myminio http://localhost:9000 minioadmin minioadmin 2>/dev/null && mc mb myminio/weknora-files 2>/dev/null && mc anonymous set download myminio/weknora-files 2>/dev/null' || {
    log_warning "MinIO Bucket 初始化失败，请手动创建"
    log_info "手动创建命令："
    log_info "  docker compose exec minio mc alias set myminio http://localhost:9000 minioadmin minioadmin"
    log_info "  docker compose exec minio mc mb myminio/weknora-files"
    log_info "  docker compose exec minio mc anonymous set download myminio/weknora-files"
}

# 10. 执行数据库迁移
log_info "[10/10] 执行数据库迁移..."
sleep 10
if docker compose -f "$COMPOSE_FILE" exec -T app ./scripts/migrate.sh up &> /dev/null; then
    log_success "数据库迁移完成"
else
    log_warning "数据库迁移失败，请手动执行"
    log_info "手动执行命令："
    log_info "  docker compose exec app ./scripts/migrate.sh up"
fi

echo ""
echo "=========================================="
log_success "部署完成！"
echo "=========================================="
echo ""
log_info "服务地址："
SERVER_IP=$(hostname -I | awk '{print $1}' 2>/dev/null || echo "localhost")
echo "  前端地址: http://${SERVER_IP}"
echo "  API地址: http://${SERVER_IP}:8080"
echo "  MinIO控制台: http://${SERVER_IP}:9001"
echo "  Neo4j浏览器: http://${SERVER_IP}:7474"
echo ""
log_info "常用命令："
echo "  查看服务状态: docker compose -f $COMPOSE_FILE ps"
echo "  查看日志: docker compose -f $COMPOSE_FILE logs -f"
echo "  停止服务: docker compose -f $COMPOSE_FILE down"
echo "  重启服务: docker compose -f $COMPOSE_FILE restart"
echo ""
