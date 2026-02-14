#!/usr/bin/env python3
"""
Git Branch 脚本
用途：从 master 分支创建新的 AI 分支
代码路径：/home/etsme/code
"""

import subprocess
import sys
import os
from datetime import datetime


# 代码路径（支持环境变量覆盖）
CODE_PATH = os.getenv('CODE_PATH', '/home/etsme/code')


# 颜色输出
class Colors:
    RED = '\033[0;31m'
    GREEN = '\033[0;32m'
    YELLOW = '\033[1;33m'
    NC = '\033[0m'


def run_command_quiet(cmd: list) -> subprocess.CompletedProcess:
    """执行 shell 命令（静默，返回结果）"""
    try:
        return subprocess.run(cmd, check=True, capture_output=True, text=True, cwd=CODE_PATH)
    except subprocess.CalledProcessError:
        print(f"{Colors.RED}命令执行失败: {' '.join(cmd)}{Colors.NC}")
        sys.exit(1)


def print_success(msg: str) -> None:
    print(f"{Colors.GREEN}✅ {msg}{Colors.NC}")


def print_info(msg: str) -> None:
    print(f"{Colors.YELLOW}📌 {msg}{Colors.NC}")


def get_ai_branch_name() -> str:
    """生成基于时间戳的 AI 分支名"""
    timestamp = datetime.now().strftime('%Y%m%d%H%M%S')
    return f"ai-{timestamp}"


def main() -> None:
    # 检查是否是 Git 仓库
    try:
        run_command_quiet(['git', 'rev-parse', '--git-dir'])
    except subprocess.CalledProcessError:
        print(f"{Colors.RED}❌ {CODE_PATH} 不是 Git 仓库{Colors.NC}")
        sys.exit(1)

    # 生成分支名
    branch_name = get_ai_branch_name()
    print_info(f"正在从 master 创建 AI 分支: {branch_name}")

    # 切换到 master
    print_info("切换到 master 分支...")
    try:
        run_command_quiet(['git', 'checkout', 'master'])
    except subprocess.CalledProcessError:
        try:
            run_command_quiet(['git', 'switch', 'master'])
        except subprocess.CalledProcessError:
            print(f"{Colors.RED}❌ 切换到 master 分支失败{Colors.NC}")
            sys.exit(1)

    # 创建新分支
    try:
        run_command_quiet(['git', 'checkout', '-b', branch_name])
        print_success(f"已创建并切换到分支 {branch_name}")
    except subprocess.CalledProcessError:
        try:
            run_command_quiet(['git', 'switch', '-c', branch_name])
            print_success(f"已创建并切换到分支 {branch_name}")
        except subprocess.CalledProcessError:
            print(f"{Colors.RED}❌ 创建分支失败{Colors.NC}")
            sys.exit(1)


if __name__ == '__main__':
    main()
