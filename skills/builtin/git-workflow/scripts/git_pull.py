#!/usr/bin/env python3
"""
Git Pull 脚本
用途：切换到 master 分支并拉取最新代码
代码路径：/home/etsme/code
"""

import subprocess
import sys
import os


# 代码路径（支持环境变量覆盖）
CODE_PATH = os.getenv('CODE_PATH', '/home/etsme/code')


# 颜色输出
class Colors:
    RED = '\033[0;31m'
    GREEN = '\033[0;32m'
    YELLOW = '\033[1;33m'
    NC = '\033[0m'


def run_command(cmd: list) -> None:
    """执行 shell 命令"""
    try:
        subprocess.run(cmd, check=True, text=True, cwd=CODE_PATH)
    except subprocess.CalledProcessError:
        print(f"{Colors.RED}命令执行失败: {' '.join(cmd)}{Colors.NC}")
        sys.exit(1)


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


def main() -> None:
    # 检查是否是 Git 仓库
    try:
        run_command_quiet(['git', 'rev-parse', '--git-dir'])
    except subprocess.CalledProcessError:
        print(f"{Colors.RED}❌ {CODE_PATH} 不是 Git 仓库{Colors.NC}")
        sys.exit(1)

    # 切换到 master 分支
    print_info("正在切换到 master 分支...")
    try:
        run_command_quiet(['git', 'checkout', 'master'])
    except subprocess.CalledProcessError:
        try:
            run_command_quiet(['git', 'switch', 'master'])
        except subprocess.CalledProcessError:
            print(f"{Colors.RED}❌ 切换到 master 分支失败{Colors.NC}")
            sys.exit(1)

    # 拉取最新代码
    print_info("正在拉取 master 最新代码...")
    subprocess.run(['git', 'pull', 'origin', 'master'], check=False, cwd=CODE_PATH)
    print_success("master 代码已更新")


if __name__ == '__main__':
    main()
