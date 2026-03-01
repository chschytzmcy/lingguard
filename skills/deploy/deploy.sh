#!/bin/bash
# LingGuard 部署脚本 - 部署到 Firefly 设备
# 用法: ./deploy.sh [options]

set -e

# 默认配置
TARGET_HOST="${TARGET_HOST:-}"
TARGET_USER="${TARGET_USER:-firefly}"
TARGET_PASSWORD="${TARGET_PASSWORD:-firefly}"
TARGET_PLATFORM="${TARGET_PLATFORM:-linux-arm64}"
TARGET_PATH="${TARGET_PATH:-/tmp}"
RESTART="${RESTART:-true}"
VERBOSE="${VERBOSE:-false}"
PROJECT_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 日志函数
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_debug() {
    if [ "$VERBOSE" = "true" ]; then
        echo -e "${BLUE}[DEBUG]${NC} $1"
    fi
}

# 显示帮助
show_help() {
    cat << EOF
LingGuard 部署工具 - 部署到 Firefly 设备

用法: $0 [选项]

选项:
  -h, --host HOST         目标设备 IP 或主机名 (必需)
  -u, --user USER         SSH 用户名 (默认: firefly)
  -p, --password PASS     SSH 密码 (默认: firefly)
  -P, --platform PLATFORM 目标平台 (默认: linux-arm64)
                          可选: linux-arm64, linux-amd64, darwin-arm64, darwin-amd64
  -d, --path PATH         上传路径 (默认: /tmp)
  -n, --no-restart        不重启服务
  -v, --verbose           显示详细输出
  --help                  显示帮助信息

示例:
  $0 -h 192.168.1.103
  $0 -h 192.168.1.103 -u firefly -p firefly
  $0 -h 192.168.1.100 -P linux-amd64 -v

环境变量:
  TARGET_HOST       目标设备 IP
  TARGET_USER       SSH 用户名
  TARGET_PASSWORD   SSH 密码
  TARGET_PLATFORM   目标平台
  TARGET_PATH       上传路径
  RESTART           是否重启 (true/false)
  VERBOSE           详细输出 (true/false)
EOF
}

# 解析参数
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--host)
                TARGET_HOST="$2"
                shift 2
                ;;
            -u|--user)
                TARGET_USER="$2"
                shift 2
                ;;
            -p|--password)
                TARGET_PASSWORD="$2"
                shift 2
                ;;
            -P|--platform)
                TARGET_PLATFORM="$2"
                shift 2
                ;;
            -d|--path)
                TARGET_PATH="$2"
                shift 2
                ;;
            -n|--no-restart)
                RESTART="false"
                shift
                ;;
            -v|--verbose)
                VERBOSE="true"
                shift
                ;;
            --help)
                show_help
                exit 0
                ;;
            *)
                log_error "未知参数: $1"
                show_help
                exit 1
                ;;
        esac
    done
}

# 检查必需参数
check_args() {
    if [ -z "$TARGET_HOST" ]; then
        log_error "缺少必需参数: 目标设备 IP (--host)"
        log_info "使用方法: $0 -h <IP地址>"
        exit 1
    fi
}

# SSH 命令包装器
ssh_cmd() {
    if [ -n "$TARGET_PASSWORD" ] && [ "$TARGET_PASSWORD" != "" ]; then
        sshpass -p "$TARGET_PASSWORD" ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null "$TARGET_USER@$TARGET_HOST" "$@"
    else
        ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null "$TARGET_USER@$TARGET_HOST" "$@"
    fi
}

# SCP 命令包装器
scp_cmd() {
    local src="$1"
    local dst="$2"
    if [ -n "$TARGET_PASSWORD" ] && [ "$TARGET_PASSWORD" != "" ]; then
        sshpass -p "$TARGET_PASSWORD" scp -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null "$src" "$dst"
    else
        scp -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null "$src" "$dst"
    fi
}

# 检测目标平台
detect_platform() {
    if [ "$TARGET_PLATFORM" = "auto" ]; then
        log_info "自动检测目标平台..."
        local arch=$(ssh_cmd "uname -m")
        local os=$(ssh_cmd "uname -s")
        
        case "$os" in
            Linux)
                case "$arch" in
                    aarch64|arm64)
                        TARGET_PLATFORM="linux-arm64"
                        ;;
                    x86_64|amd64)
                        TARGET_PLATFORM="linux-amd64"
                        ;;
                    *)
                        log_error "不支持的架构: $arch"
                        exit 1
                        ;;
                esac
                ;;
            Darwin)
                case "$arch" in
                    arm64)
                        TARGET_PLATFORM="darwin-arm64"
                        ;;
                    x86_64)
                        TARGET_PLATFORM="darwin-amd64"
                        ;;
                    *)
                        log_error "不支持的架构: $arch"
                        exit 1
                        ;;
                esac
                ;;
            *)
                log_error "不支持的操作系统: $os"
                exit 1
                ;;
        esac
        log_info "检测到平台: $TARGET_PLATFORM"
    fi
}

# 构建安装包
build_package() {
    log_info "=========================================="
    log_info "步骤 1/6: 构建安装包"
    log_info "=========================================="
    
    cd "$PROJECT_ROOT"
    
    local make_target="package-$TARGET_PLATFORM"
    
    log_info "执行: make clean && make $make_target"
    
    make clean
    
    if ! make "$make_target" 2>&1 | tee /tmp/lingguard-build.log; then
        log_error "构建失败"
        exit 1
    fi
    
    # 查找生成的包
    PACKAGE_FILE=$(ls -t dist/lingguard-$TARGET_PLATFORM-*.tar.gz 2>/dev/null | head -1)
    
    if [ -z "$PACKAGE_FILE" ]; then
        log_error "找不到构建产物"
        exit 1
    fi
    
    log_info "构建完成: $PACKAGE_FILE"
    log_debug "包大小: $(du -h "$PACKAGE_FILE" | cut -f1)"
}

# 上传到目标设备
upload_package() {
    log_info "=========================================="
    log_info "步骤 2/6: 上传到目标设备"
    log_info "=========================================="
    
    local remote_file="$TARGET_PATH/$(basename "$PACKAGE_FILE")"
    
    log_info "上传: $PACKAGE_FILE -> $TARGET_USER@$TARGET_HOST:$remote_file"
    
    if ! scp_cmd "$PACKAGE_FILE" "$TARGET_USER@$TARGET_HOST:$remote_file"; then
        log_error "上传失败"
        exit 1
    fi
    
    log_info "上传完成"
    REMOTE_PACKAGE="$remote_file"
}

# 停止旧服务
stop_service() {
    log_info "=========================================="
    log_info "步骤 3/6: 停止旧服务"
    log_info "=========================================="
    
    log_info "停止 lingguard 服务..."
    ssh_cmd "pkill -9 lingguard 2>/dev/null || true"
    ssh_cmd "pkill -9 node 2>/dev/null || true"
    ssh_cmd "rm -rf ~/.lingguard/locks/*.lock 2>/dev/null || true"
    ssh_cmd "systemctl --user stop lingguard 2>/dev/null || true"
    
    sleep 2
    log_info "旧服务已停止"
}

# 解压安装包
extract_package() {
    log_info "=========================================="
    log_info "步骤 4/6: 解压安装包"
    log_info "=========================================="
    
    local extract_dir="$TARGET_PATH/lingguard-$TARGET_PLATFORM"
    
    log_info "解压: $REMOTE_PACKAGE"
    
    ssh_cmd "cd $TARGET_PATH && rm -rf $extract_dir && tar -xzf $REMOTE_PACKAGE"
    
    if ! ssh_cmd "test -d $extract_dir"; then
        log_error "解压失败"
        exit 1
    fi
    
    REMOTE_EXTRACT_DIR="$extract_dir"
    log_info "解压完成: $REMOTE_EXTRACT_DIR"
}

# 执行安装
install_package() {
    log_info "=========================================="
    log_info "步骤 5/6: 执行安装"
    log_info "=========================================="
    
    log_info "运行安装脚本..."
    
    # 执行安装并捕获输出
    local install_output=$(ssh_cmd "cd $REMOTE_EXTRACT_DIR && chmod +x scripts/install.sh && bash scripts/install.sh 2>&1")
    
    if [ "$VERBOSE" = "true" ]; then
        echo "$install_output"
    fi
    
    # 检查安装结果
    if echo "$install_output" | grep -q "安装完成\|更新完成"; then
        log_info "安装成功"
    else
        log_warn "安装可能未完成，请检查输出"
    fi
}

# 验证并启动服务
verify_service() {
    log_info "=========================================="
    log_info "步骤 6/6: 验证服务"
    log_info "=========================================="
    
    sleep 3
    
    # 检查服务状态
    local status=$(ssh_cmd "systemctl --user is-active lingguard 2>/dev/null || echo 'inactive'")
    
    if [ "$status" = "active" ]; then
        log_info "✓ 服务状态: $status"
    else
        log_warn "服务状态: $status"
    fi
    
    # 检查进程
    local pid=$(ssh_cmd "pgrep -f 'lingguard gateway' | head -1")
    if [ -n "$pid" ]; then
        log_info "✓ 进程 PID: $pid"
    else
        log_warn "未找到运行中的进程"
    fi
    
    # 检查端口
    local port=$(ssh_cmd "ss -tlnp 2>/dev/null | grep 18989 || netstat -tlnp 2>/dev/null | grep 18989")
    if [ -n "$port" ]; then
        log_info "✓ Web UI: http://127.0.0.1:18989"
    else
        log_warn "Web UI 端口未监听"
    fi
    
    # 显示最新日志
    log_info "最新日志:"
    ssh_cmd "tail -10 ~/.lingguard/logs/lingguard.log 2>/dev/null" | sed 's/^/  /'
}

# 显示部署摘要
show_summary() {
    echo ""
    log_info "=========================================="
    log_info "部署完成！"
    log_info "=========================================="
    echo ""
    echo "目标设备: $TARGET_USER@$TARGET_HOST"
    echo "平台: $TARGET_PLATFORM"
    echo ""
    echo "服务管理:"
    echo "  ssh $TARGET_USER@$TARGET_HOST"
    echo "  systemctl --user status lingguard"
    echo "  systemctl --user restart lingguard"
    echo "  journalctl --user -u lingguard -f"
    echo ""
    echo "查看日志:"
    echo "  ssh $TARGET_USER@$TARGET_HOST 'tail -f ~/.lingguard/logs/lingguard.log'"
    echo ""
    echo "Web UI: http://$TARGET_HOST:18989 (如果端口已映射)"
    echo ""
}

# 主函数
main() {
    parse_args "$@"
    check_args
    
    echo ""
    echo "╔════════════════════════════════════════╗"
    echo "║   LingGuard 部署工具 - Firefly 设备   ║"
    echo "╚════════════════════════════════════════╝"
    echo ""
    
    log_info "目标: $TARGET_USER@$TARGET_HOST"
    log_info "平台: $TARGET_PLATFORM"
    log_info "路径: $TARGET_PATH"
    echo ""
    
    # 检查 sshpass
    if [ -n "$TARGET_PASSWORD" ] && ! command -v sshpass &> /dev/null; then
        log_warn "未安装 sshpass，将使用密钥认证"
        log_info "安装: sudo apt install sshpass"
    fi
    
    # 执行部署步骤
    detect_platform
    build_package
    upload_package
    stop_service
    extract_package
    install_package
    
    if [ "$RESTART" = "true" ]; then
        verify_service
    fi
    
    show_summary
}

# 运行
main "$@"
