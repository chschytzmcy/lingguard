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
CYAN='\033[0;36m'
NC='\033[0m'

log_info()  { echo -e "${GREEN}[INFO]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }
log_step()  { echo -e "${CYAN}[$1/7]${NC} $2"; }
log_sub()   { echo -e "  → $1"; }

ssh_cmd() {
    sshpass -p "$TARGET_PASSWORD" ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR -o ConnectTimeout=10 "$TARGET_USER@$TARGET_HOST" "$@"
}

# ============================================
# 主函数
# ============================================
main() {
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
    echo "  ssh $TARGET_USER@$TARGET_HOST 'systemctl --user status lingguard'"
    echo "  ssh $TARGET_USER@$TARGET_HOST 'tail -f ~/.lingguard/logs/lingguard.log'"
    echo ""
}

main "$@"
