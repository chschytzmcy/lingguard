#!/bin/bash
# LingGuard 部署脚本 - 部署到 Firefly 设备

set -e

# ============================================
# 配置
# ============================================
DEFAULT_HOST="192.168.1.103"
DEFAULT_USER="firefly"
DEFAULT_PASSWORD="firefly"
DEFAULT_PLATFORM="linux-arm64"
DEFAULT_PATH="/tmp"

TARGET_HOST="${TARGET_HOST:-$DEFAULT_HOST}"
TARGET_USER="${TARGET_USER:-$DEFAULT_USER}"
TARGET_PASSWORD="${TARGET_PASSWORD:-$DEFAULT_PASSWORD}"
TARGET_PLATFORM="${TARGET_PLATFORM:-$DEFAULT_PLATFORM}"
TARGET_PATH="${TARGET_PATH:-$DEFAULT_PATH}"
PROJECT_ROOT="$(cd "$(dirname "$0")" && pwd)"

# 颜色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
NC='\033[0m'

log_info()  { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }
log_step()  { echo -e "${CYAN}[$1/7]${NC} $2"; }
log_sub()   { echo -e "  → $1"; }

ssh_cmd() {
    sshpass -p "$TARGET_PASSWORD" ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR -o ConnectTimeout=10 "$TARGET_USER@$TARGET_HOST" "$@"
}

# ============================================
# 帮助信息
# ============================================
show_help() {
    echo ""
    echo -e "${CYAN}LingGuard 部署工具 - Firefly 设备${NC}"
    echo ""
    echo "用法: $0 <命令> [选项]"
    echo ""
    echo "命令:"
    echo "  deploy    部署 LingGuard 到目标设备 (默认)"
    echo "  uninstall 从目标设备卸载 LingGuard"
    echo "  status    查看目标设备上的服务状态"
    echo "  logs      查看目标设备上的日志"
    echo "  help      显示此帮助信息"
    echo ""
    echo "环境变量:"
    echo "  TARGET_HOST     目标设备 IP (默认: $DEFAULT_HOST)"
    echo "  TARGET_USER     SSH 用户名 (默认: $DEFAULT_USER)"
    echo "  TARGET_PASSWORD SSH 密码 (默认: $DEFAULT_PASSWORD)"
    echo "  TARGET_PLATFORM 目标平台 (默认: $DEFAULT_PLATFORM)"
    echo ""
    echo "示例:"
    echo "  $0 deploy                    # 部署到默认设备"
    echo "  $0 uninstall                 # 卸载"
    echo "  $0 status                    # 查看状态"
    echo "  TARGET_HOST=192.168.1.100 $0 deploy  # 指定设备部署"
    echo ""
}

# ============================================
# 卸载功能
# ============================================
do_uninstall() {
    echo ""
    echo -e "${CYAN}╔════════════════════════════════════════╗${NC}"
    echo -e "${CYAN}║   LingGuard 卸载工具 - Firefly 设备   ║${NC}"
    echo -e "${CYAN}╚════════════════════════════════════════╝${NC}"
    echo ""
    log_info "目标: $TARGET_USER@$TARGET_HOST"
    echo ""

    # 测试连接
    log_info "测试 SSH 连接..."
    if ! ssh_cmd "echo '连接成功'" 2>/dev/null; then
        log_error "无法连接到 $TARGET_USER@$TARGET_HOST"
        exit 1
    fi

    # 1. 停止服务
    log_step 1 "停止服务"
    ssh_cmd "pkill -9 lingguard 2>/dev/null || true"
    ssh_cmd "systemctl --user stop lingguard 2>/dev/null || true"
    ssh_cmd "systemctl --user disable lingguard 2>/dev/null || true"
    sleep 1
    log_info "服务已停止"

    # 2. 删除 systemd 服务文件
    log_step 2 "删除系统服务"
    ssh_cmd "rm -f ~/.config/systemd/user/lingguard.service 2>/dev/null || true"
    ssh_cmd "systemctl --user daemon-reload 2>/dev/null || true"
    log_info "系统服务已删除"

    # 3. 删除二进制文件
    log_step 3 "删除程序文件"
    ssh_cmd "rm -f ~/.local/bin/lingguard 2>/dev/null || true"
    log_info "程序文件已删除"

    # 4. 清理数据目录（询问）
    log_step 4 "清理数据目录"
    local data_dir=$(ssh_cmd "echo ~/.lingguard")
    local dir_exists=$(ssh_cmd "test -d ~/.lingguard && echo 'yes' || echo 'no'")

    if [ "$dir_exists" = "yes" ]; then
        if [ "${KEEP_DATA:-}" = "true" ]; then
            log_info "保留数据目录: $data_dir"
        elif [ "${KEEP_DATA:-}" = "false" ]; then
            log_warn "删除数据目录: $data_dir"
            ssh_cmd "rm -rf ~/.lingguard"
        else
            echo ""
            read -p "是否删除数据目录 ~/.lingguard？(y/N): " -n 1 -r
            echo ""
            if [[ $REPLY =~ ^[Yy]$ ]]; then
                log_warn "删除数据目录: $data_dir"
                ssh_cmd "rm -rf ~/.lingguard"
            else
                log_info "保留数据目录: $data_dir"
            fi
        fi
    else
        log_info "数据目录不存在，跳过"
    fi

    # 5. 验证卸载
    log_step 5 "验证卸载"
    local binary=$(ssh_cmd "test -f ~/.local/bin/lingguard && echo 'exists' || echo 'gone'")
    local service=$(ssh_cmd "test -f ~/.config/systemd/user/lingguard.service && echo 'exists' || echo 'gone'")

    if [ "$binary" = "gone" ] && [ "$service" = "gone" ]; then
        log_info "✓ 卸载成功"
    else
        log_warn "部分文件可能未完全删除"
    fi

    # 摘要
    echo ""
    echo -e "${GREEN}╔════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║          卸载完成！                    ║${NC}"
    echo -e "${GREEN}╚════════════════════════════════════════╝${NC}"
    echo ""
}

# ============================================
# 查看状态
# ============================================
do_status() {
    echo ""
    echo -e "${CYAN}LingGuard 服务状态 - $TARGET_HOST${NC}"
    echo ""

    # 测试连接
    if ! ssh_cmd "echo ''" 2>/dev/null; then
        log_error "无法连接到 $TARGET_USER@$TARGET_HOST"
        exit 1
    fi

    echo "=== Systemd 服务 ==="
    ssh_cmd "systemctl --user status lingguard 2>/dev/null || echo '服务未安装'"

    echo ""
    echo "=== 进程状态 ==="
    local pid=$(ssh_cmd "pgrep -f 'lingguard' || echo ''")
    if [ -n "$pid" ]; then
        ssh_cmd "ps aux | grep lingguard | grep -v grep"
    else
        echo "无运行进程"
    fi

    echo ""
    echo "=== 端口监听 ==="
    ssh_cmd "ss -tlnp 2>/dev/null | grep 18989 || echo '端口 18989 未监听'"

    echo ""
    echo "=== 安装信息 ==="
    local bin_ver=$(ssh_cmd "~/.local/bin/lingguard version 2>/dev/null || echo '未安装'")
    echo "版本: $bin_ver"
    local config=$(ssh_cmd "test -f ~/.lingguard/config.json && echo '已配置' || echo '未配置'")
    echo "配置: $config"
}

# ============================================
# 查看日志
# ============================================
do_logs() {
    echo ""
    echo -e "${CYAN}LingGuard 日志 - $TARGET_HOST${NC}"
    echo ""

    # 测试连接
    if ! ssh_cmd "echo ''" 2>/dev/null; then
        log_error "无法连接到 $TARGET_USER@$TARGET_HOST"
        exit 1
    fi

    local lines="${LOG_LINES:-50}"
    echo "最近 $lines 行日志:"
    echo "----------------------------------------"
    ssh_cmd "tail -$lines ~/.lingguard/logs/lingguard.log 2>/dev/null || echo '日志文件不存在'"
}

# ============================================
# 主函数 - 部署
# ============================================
do_deploy() {
    echo ""
    echo -e "${CYAN}╔════════════════════════════════════════╗${NC}"
    echo -e "${CYAN}║   LingGuard 部署工具 - Firefly 设备   ║${NC}"
    echo -e "${CYAN}╚════════════════════════════════════════╝${NC}"
    echo ""
    log_info "目标: $TARGET_USER@$TARGET_HOST"
    log_info "平台: $TARGET_PLATFORM"
    echo ""
    
    # 测试连接
    log_info "测试 SSH 连接..."
    if ! ssh_cmd "echo '连接成功'" 2>/dev/null; then
        log_error "无法连接到 $TARGET_USER@$TARGET_HOST"
        exit 1
    fi
    
    # 1. 构建
    log_step 1 "构建安装包 ($TARGET_HOST)"
    cd "$PROJECT_ROOT"
    log_sub "项目目录: $PROJECT_ROOT"
    
    make clean 2>/dev/null || true
    make "package-$TARGET_PLATFORM"
    
    PACKAGE_FILE=$(ls -t dist/*.tar.gz 2>/dev/null | head -1)
    if [ -z "$PACKAGE_FILE" ]; then
        log_error "找不到构建产物"
        exit 1
    fi
    log_info "构建完成: $PACKAGE_FILE"
    
    # 2. 上传
    log_step 2 "上传到目标设备 ($TARGET_HOST)"
    REMOTE_FILE="$TARGET_PATH/$(basename "$PACKAGE_FILE")"
    log_sub "上传: $PACKAGE_FILE → $TARGET_HOST:$REMOTE_FILE"
    
    # 使用 sshpass scp 上传
    sshpass -p "$TARGET_PASSWORD" scp -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null "$PACKAGE_FILE" "$TARGET_USER@$TARGET_HOST:$REMOTE_FILE"
    
    # 验证上传
    if ! ssh_cmd "test -f $REMOTE_FILE"; then
        log_error "上传失败: 文件不存在"
        exit 1
    fi
    log_info "上传完成"
    
    # 3. 停止服务
    log_step 3 "停止旧服务 ($TARGET_HOST)"
    ssh_cmd "pkill -9 lingguard 2>/dev/null || true"
    ssh_cmd "systemctl --user stop lingguard 2>/dev/null || true"
    ssh_cmd "rm -rf ~/.lingguard/locks/*.lock 2>/dev/null || true"
    sleep 1
    log_info "服务已停止"
    
    # 4. 解压
    log_step 4 "解压安装包 ($TARGET_HOST)"
    EXTRACT_DIR="$TARGET_PATH/lingguard-$TARGET_PLATFORM"
    ssh_cmd "rm -rf $EXTRACT_DIR 2>/dev/null || true"
    ssh_cmd "cd $TARGET_PATH && tar -xzf $REMOTE_FILE"
    
    if ! ssh_cmd "test -d $EXTRACT_DIR"; then
        log_error "解压失败: 目录不存在"
        exit 1
    fi
    log_info "解压完成"
    
    # 5. 安装
    log_step 5 "执行安装 ($TARGET_HOST)"
    ssh_cmd "cd $EXTRACT_DIR && chmod +x scripts/install.sh && bash scripts/install.sh" | sed 's/^/  /'
    log_info "安装完成"
    
    # 6. 验证
    log_step 6 "验证服务 ($TARGET_HOST)"
    sleep 2
    
    ssh_cmd "systemctl --user start lingguard 2>/dev/null || true"
    sleep 2
    
    # 检查服务状态
    local status=$(ssh_cmd "systemctl --user is-active lingguard 2>/dev/null || echo 'inactive'")
    if [ "$status" = "active" ]; then
        log_info "✓ 服务状态: $status"
    else
        log_info "尝试直接启动..."
        ssh_cmd "nohup ~/.local/bin/lingguard gateway > ~/.lingguard/logs/lingguard.log 2>&1 &"
        sleep 2
    fi
    
    # 检查进程
    local pid=$(ssh_cmd "pgrep -f 'lingguard gateway' | head -1")
    if [ -n "$pid" ]; then
        log_info "✓ 进程 PID: $pid"
    fi
    
    # 检查端口
    local port=$(ssh_cmd "ss -tlnp 2>/dev/null | grep 18989 || echo ''")
    if [ -n "$port" ]; then
        log_info "✓ Web UI: http://$TARGET_HOST:18989"
    fi
    
    # 7. 清理临时文件
    log_step 7 "清理临时文件 ($TARGET_HOST)"
    log_sub "删除: $REMOTE_FILE"
    ssh_cmd "rm -f $REMOTE_FILE"
    log_sub "删除: $EXTRACT_DIR"
    ssh_cmd "rm -rf $EXTRACT_DIR"
    log_info "清理完成"
    # 显示最新日志
    log_info "最新日志:"
    ssh_cmd "tail -5 ~/.lingguard/logs/lingguard.log 2>/dev/null" | sed 's/^/  /'
    
    # 摘要
    echo ""
    echo -e "${GREEN}╔════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║          部署完成！                    ║${NC}"
    echo -e "${GREEN}╚════════════════════════════════════════╝${NC}"
    echo ""
    echo "目标设备: $TARGET_USER@$TARGET_HOST"
    echo ""
    echo "常用命令:"
    echo "  $0 status                    # 查看服务状态"
    echo "  $0 logs                      # 查看日志"
    echo "  $0 uninstall                 # 卸载"
    echo ""
}

# ============================================
# 入口
# ============================================
case "${1:-deploy}" in
    deploy)
        do_deploy
        ;;
    uninstall|remove)
        do_uninstall
        ;;
    status)
        do_status
        ;;
    logs)
        do_logs
        ;;
    help|--help|-h)
        show_help
        ;;
    *)
        log_error "未知命令: $1"
        show_help
        exit 1
        ;;
esac
