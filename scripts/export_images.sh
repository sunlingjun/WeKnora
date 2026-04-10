#!/bin/bash
# 该脚本用于导出 WeKnora 的 Docker 镜像为压缩包

# 设置颜色
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # 无颜色

# 获取项目根目录（脚本所在目录的上一级）
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_ROOT="$( cd "$SCRIPT_DIR/.." && pwd )"

# 版本信息
VERSION="1.0.0"
SCRIPT_NAME=$(basename "$0")

# 默认导出目录
EXPORT_DIR="${PROJECT_ROOT}/docker-images"

# 显示帮助信息
show_help() {
    echo -e "${GREEN}WeKnora 镜像导出脚本 v${VERSION}${NC}"
    echo -e "${GREEN}用法:${NC} $0 [选项]"
    echo "选项:"
    echo "  -h, --help         显示帮助信息"
    echo "  -a, --all          导出所有镜像（默认）"
    echo "  -p, --app          仅导出应用镜像"
    echo "  -d, --docreader    仅导出文档读取器镜像"
    echo "  -f, --frontend     仅导出前端镜像"
    echo "  -o, --output DIR   指定导出目录（默认: docker-images）"
    echo "  -v, --version      显示版本信息"
    echo ""
    echo "示例:"
    echo "  $0                      # 导出所有镜像到 docker-images 目录"
    echo "  $0 -a -o /tmp/images    # 导出所有镜像到 /tmp/images 目录"
    echo "  $0 -p                   # 仅导出应用镜像"
    exit 0
}

# 显示版本信息
show_version() {
    echo -e "${GREEN}WeKnora 镜像导出脚本 v${VERSION}${NC}"
    exit 0
}

# 日志函数
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

# 检查Docker是否已安装
check_docker() {
    log_info "检查Docker环境..."
    
    if ! command -v docker &> /dev/null; then
        log_error "未安装Docker，请先安装Docker"
        return 1
    fi
    
    # 检查Docker服务运行状态
    if ! docker info &> /dev/null; then
        log_error "Docker服务未运行，请启动Docker服务"
        return 1
    fi
    
    log_success "Docker环境检查通过"
    return 0
}

# 检查镜像是否存在
check_image_exists() {
    local image_name=$1
    if docker images --format "{{.Repository}}:{{.Tag}}" | grep -q "^${image_name}$"; then
        return 0
    else
        return 1
    fi
}

# 导出应用镜像
export_app_image() {
    local image_name="wechatopenai/weknora-app:latest"
    local output_file="${EXPORT_DIR}/weknora-app-latest.tar"
    
    log_info "导出应用镜像: ${image_name}"
    
    # 检查镜像是否存在
    if ! check_image_exists "$image_name"; then
        log_error "镜像不存在: ${image_name}"
        log_info "提示: 请先构建镜像，运行: ./scripts/build_images.sh --app"
        return 1
    fi
    
    # 创建导出目录
    mkdir -p "$EXPORT_DIR"
    
    # 导出镜像
    log_info "正在导出镜像到: ${output_file}"
    docker save -o "$output_file" "$image_name"
    
    if [ $? -eq 0 ]; then
        # 显示文件大小
        local file_size=$(du -h "$output_file" | cut -f1)
        log_success "应用镜像导出成功: ${output_file} (${file_size})"
        return 0
    else
        log_error "应用镜像导出失败"
        return 1
    fi
}

# 导出文档读取器镜像
export_docreader_image() {
    local image_name="wechatopenai/weknora-docreader:latest"
    local output_file="${EXPORT_DIR}/weknora-docreader-latest.tar"
    
    log_info "导出文档读取器镜像: ${image_name}"
    
    # 检查镜像是否存在
    if ! check_image_exists "$image_name"; then
        log_error "镜像不存在: ${image_name}"
        log_info "提示: 请先构建镜像，运行: ./scripts/build_images.sh --docreader"
        return 1
    fi
    
    # 创建导出目录
    mkdir -p "$EXPORT_DIR"
    
    # 导出镜像
    log_info "正在导出镜像到: ${output_file}"
    docker save -o "$output_file" "$image_name"
    
    if [ $? -eq 0 ]; then
        # 显示文件大小
        local file_size=$(du -h "$output_file" | cut -f1)
        log_success "文档读取器镜像导出成功: ${output_file} (${file_size})"
        return 0
    else
        log_error "文档读取器镜像导出失败"
        return 1
    fi
}

# 导出前端镜像
export_frontend_image() {
    local image_name="wechatopenai/weknora-ui:latest"
    local output_file="${EXPORT_DIR}/weknora-ui-latest.tar"
    
    log_info "导出前端镜像: ${image_name}"
    
    # 检查镜像是否存在
    if ! check_image_exists "$image_name"; then
        log_error "镜像不存在: ${image_name}"
        log_info "提示: 请先构建镜像，运行: ./scripts/build_images.sh --frontend"
        return 1
    fi
    
    # 创建导出目录
    mkdir -p "$EXPORT_DIR"
    
    # 导出镜像
    log_info "正在导出镜像到: ${output_file}"
    docker save -o "$output_file" "$image_name"
    
    if [ $? -eq 0 ]; then
        # 显示文件大小
        local file_size=$(du -h "$output_file" | cut -f1)
        log_success "前端镜像导出成功: ${output_file} (${file_size})"
        return 0
    else
        log_error "前端镜像导出失败"
        return 1
    fi
}

# 导出所有镜像
export_all_images() {
    log_info "开始导出所有镜像..."
    
    local app_result=0
    local docreader_result=0
    local frontend_result=0
    
    # 创建导出目录
    mkdir -p "$EXPORT_DIR"
    
    # 导出应用镜像
    export_app_image
    app_result=$?
    
    # 导出文档读取器镜像
    export_docreader_image
    docreader_result=$?
    
    # 导出前端镜像
    export_frontend_image
    frontend_result=$?
    
    # 显示导出结果
    echo ""
    log_info "=== 导出结果 ==="
    if [ $app_result -eq 0 ]; then
        log_success "✓ 应用镜像导出成功"
    else
        log_error "✗ 应用镜像导出失败"
    fi
    
    if [ $docreader_result -eq 0 ]; then
        log_success "✓ 文档读取器镜像导出成功"
    else
        log_error "✗ 文档读取器镜像导出失败"
    fi
    
    if [ $frontend_result -eq 0 ]; then
        log_success "✓ 前端镜像导出成功"
    else
        log_error "✗ 前端镜像导出失败"
    fi
    
    # 显示导出目录信息
    if [ -d "$EXPORT_DIR" ]; then
        echo ""
        log_info "导出目录: ${EXPORT_DIR}"
        log_info "导出文件列表:"
        ls -lh "$EXPORT_DIR"/*.tar 2>/dev/null | awk '{print "  " $9 " (" $5 ")"}'
        
        # 计算总大小
        local total_size=$(du -sh "$EXPORT_DIR" | cut -f1)
        log_info "总大小: ${total_size}"
    fi
    
    if [ $app_result -eq 0 ] && [ $docreader_result -eq 0 ] && [ $frontend_result -eq 0 ]; then
        log_success "所有镜像导出完成！"
        return 0
    else
        log_error "部分镜像导出失败"
        return 1
    fi
}

# 解析命令行参数
EXPORT_ALL=false
EXPORT_APP=false
EXPORT_DOCREADER=false
EXPORT_FRONTEND=false

# 没有参数时默认导出所有镜像
if [ $# -eq 0 ]; then
    EXPORT_ALL=true
fi

while [ "$1" != "" ]; do
    case $1 in
        -h | --help )       show_help
                            ;;
        -a | --all )        EXPORT_ALL=true
                            ;;
        -p | --app )        EXPORT_APP=true
                            ;;
        -d | --docreader )  EXPORT_DOCREADER=true
                            ;;
        -f | --frontend )   EXPORT_FRONTEND=true
                            ;;
        -o | --output )     shift
                            EXPORT_DIR="$1"
                            ;;
        -v | --version )    show_version
                            ;;
        * )                 log_error "未知选项: $1"
                            show_help
                            ;;
    esac
    shift
done

# 检查Docker环境
check_docker
if [ $? -ne 0 ]; then
    exit 1
fi

# 执行导出操作
if [ "$EXPORT_ALL" = true ]; then
    export_all_images
    exit $?
fi

if [ "$EXPORT_APP" = true ]; then
    export_app_image
    exit $?
fi

if [ "$EXPORT_DOCREADER" = true ]; then
    export_docreader_image
    exit $?
fi

if [ "$EXPORT_FRONTEND" = true ]; then
    export_frontend_image
    exit $?
fi

exit 0
