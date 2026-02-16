#!/bin/bash
# LingGuard 卸载脚本
# 用法: make uninstall 或 ./scripts/uninstall.sh

set -e

# 配置
PREFIX="${PREFIX:-/usr/local}"
BIN_NAME="lingguard"
SERVICE_NAME="lingguard"
CONFIG_DIR="${HOME}/.lingguard"

echo "=== LingGuard 卸载 ==="
echo ""

# 询问是否保留配置
KEEP_CONFIG="n"
if [ -d "${CONFIG_DIR}" ]; then
    echo "发现配置目录: ${CONFIG_DIR}"
    read -p "是否保留配置目录？(y/N): " KEEP_CONFIG
    KEEP_CONFIG=${KEEP_CONFIG:-n}
fi

echo ""
echo "开始卸载..."

# 1. 停止并禁用服务
echo "[1/4] 停止服务..."
if systemctl --user is-active ${SERVICE_NAME} &>/dev/null; then
    systemctl --user stop ${SERVICE_NAME}
    echo "  ✓ 已停止用户服务"
fi

if [ "$EUID" -eq 0 ]; then
    if systemctl is-active ${SERVICE_NAME} &>/dev/null; then
        systemctl stop ${SERVICE_NAME}
        echo "  ✓ 已停止系统服务"
    fi
fi

# 2. 禁用服务
echo "[2/4] 禁用服务..."
if systemctl --user is-enabled ${SERVICE_NAME} &>/dev/null; then
    systemctl --user disable ${SERVICE_NAME}
    echo "  ✓ 已禁用用户服务"
fi

if [ "$EUID" -eq 0 ]; then
    if systemctl is-enabled ${SERVICE_NAME} &>/dev/null; then
        systemctl disable ${SERVICE_NAME}
        echo "  ✓ 已禁用系统服务"
    fi
fi

# 3. 删除服务文件
echo "[3/4] 删除服务文件..."
if [ -f "${HOME}/.config/systemd/user/${SERVICE_NAME}.service" ]; then
    rm -f "${HOME}/.config/systemd/user/${SERVICE_NAME}.service"
    systemctl --user daemon-reload
    echo "  ✓ 已删除用户服务文件"
fi

if [ "$EUID" -eq 0 ]; then
    if [ -f "/etc/systemd/system/${SERVICE_NAME}.service" ]; then
        rm -f "/etc/systemd/system/${SERVICE_NAME}.service"
        systemctl daemon-reload
        echo "  ✓ 已删除系统服务文件"
    fi
fi

# 4. 删除二进制文件
echo "[4/4] 删除二进制文件..."
if [ -f "${PREFIX}/bin/${BIN_NAME}" ]; then
    rm -f "${PREFIX}/bin/${BIN_NAME}"
    echo "  ✓ 已删除 ${PREFIX}/bin/${BIN_NAME}"
elif [ -f "/usr/local/bin/${BIN_NAME}" ]; then
    rm -f "/usr/local/bin/${BIN_NAME}"
    echo "  ✓ 已删除 /usr/local/bin/${BIN_NAME}"
else
    echo "  ! 二进制文件不存在"
fi

# 清理符号链接
if [ -L "${HOME}/.config/systemd/user/default.target.wants/${SERVICE_NAME}.service" ]; then
    rm -f "${HOME}/.config/systemd/user/default.target.wants/${SERVICE_NAME}.service"
    echo "  ✓ 已清理服务符号链接"
fi

# 删除配置目录（可选）
if [ "${KEEP_CONFIG}" = "y" ] || [ "${KEEP_CONFIG}" = "Y" ]; then
    echo ""
    echo "配置目录已保留: ${CONFIG_DIR}"
else
    if [ -d "${CONFIG_DIR}" ]; then
        echo ""
        echo "即将删除配置目录: ${CONFIG_DIR}"
        echo "  包含: 配置文件、记忆、日志、技能等"
        read -p "确认删除？(y/N): " CONFIRM
        if [ "${CONFIRM}" = "y" ] || [ "${CONFIRM}" = "Y" ]; then
            rm -rf "${CONFIG_DIR}"
            echo "  ✓ 已删除配置目录"
        else
            echo "  ! 配置目录已保留"
        fi
    fi
fi

echo ""
echo "=== 卸载完成 ==="
echo ""
echo "如需重新安装，请运行:"
echo "  make install"
