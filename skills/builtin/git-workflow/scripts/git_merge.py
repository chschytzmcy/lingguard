#!/usr/bin/env python3
"""
Git Merge 脚本
用途：将当前分支合并到 ai-test 分支
代码路径：/home/etsme/code
"""

import subprocess
import sys
import argparse


# 代码路径
CODE_PATH = "/home/etsme/code"


# 颜色输出
class Colors:
    RED = '\033[0;31m'
    GREEN = '\033[0;32m'
    YELLOW = '\033[1;33m'
    CYAN = '\033[0;36m'
    NC = '\033[0m'


def run_command_quiet(cmd: list) -> subprocess.CompletedProcess:
    """执行 shell 命令（静默，返回结果）"""
    try:
        return subprocess.run(cmd, check=True, capture_output=True, text=True, cwd=CODE_PATH)
    except subprocess.CalledProcessError as e:
        print(f"{Colors.RED}❌ 命令执行失败: {' '.join(cmd)}{Colors.NC}")
        if e.stderr:
            print(f"{Colors.RED}错误信息: {e.stderr.strip()}{Colors.NC}")
        sys.exit(1)


def run_command(cmd: list) -> subprocess.CompletedProcess:
    """执行 shell 命令（显示输出）"""
    try:
        return subprocess.run(cmd, check=True, text=True, cwd=CODE_PATH)
    except subprocess.CalledProcessError as e:
        print(f"{Colors.RED}❌ 命令执行失败: {' '.join(cmd)}{Colors.NC}")
        if e.stderr:
            print(f"{Colors.RED}错误信息: {e.stderr.strip()}{Colors.NC}")
        sys.exit(1)


def get_current_branch() -> str:
    """获取当前分支名"""
    result = run_command_quiet(['git', 'rev-parse', '--abbrev-ref', 'HEAD'])
    return result.stdout.strip()


def get_local_branches() -> list:
    """获取所有本地分支"""
    result = run_command_quiet(['git', 'branch', '--format=%(refname:short)'])
    return result.stdout.strip().split('\n')


def print_success(msg: str) -> None:
    print(f"{Colors.GREEN}✅ {msg}{Colors.NC}")


def print_info(msg: str) -> None:
    print(f"{Colors.YELLOW}📌 {msg}{Colors.NC}")


def print_step(msg: str) -> None:
    print(f"{Colors.CYAN}▶ {msg}{Colors.NC}")


def main() -> None:
    parser = argparse.ArgumentParser(description='合并当前分支到 ai-test')
    parser.add_argument('--source', '-s', help='源分支名称（默认：当前分支）')
    parser.add_argument('--target', '-t', default='ai-test',
                       help='目标分支名称（默认：ai-test）')
    parser.add_argument('--push', '-p', action='store_true',
                       help='合并后推送到远程')

    args = parser.parse_args()

    # 检查是否是 Git 仓库
    try:
        run_command_quiet(['git', 'rev-parse', '--git-dir'])
    except subprocess.CalledProcessError:
        print(f"{Colors.RED}❌ {CODE_PATH} 不是 Git 仓库{Colors.NC}")
        sys.exit(1)

    # 获取源分支
    if args.source:
        source_branch = args.source
    else:
        source_branch = get_current_branch()

    # 安全检查：禁止合并 master
    if source_branch == 'master':
        print(f"{Colors.RED}❌ 禁止从 master 分支合并{Colors.NC}")
        sys.exit(1)

    target_branch = args.target

    print(f"{Colors.CYAN}{'='*60}{Colors.NC}")
    print(f"{Colors.CYAN}Git Merge: {source_branch} -> {target_branch}{Colors.NC}")
    print(f"{Colors.CYAN}{'='*60}{Colors.NC}")

    # 显示源分支信息
    print_step(f"源分支: {source_branch}")
    print_step(f"目标分支: {target_branch}")

    # 切换到目标分支
    local_branches = get_local_branches()
    if target_branch in local_branches:
        print_step(f"切换到已存在的 {target_branch} 分支...")
        run_command(['git', 'checkout', target_branch])
    else:
        print_step(f"从远程创建 {target_branch} 分支...")
        try:
            run_command_quiet(['git', 'checkout', '-b', target_branch, f'origin/{target_branch}'])
        except subprocess.CalledProcessError:
            print(f"{Colors.RED}❌ 远程分支 origin/{target_branch} 不存在{Colors.NC}")
            print(f"{Colors.YELLOW}💡 提示: 先在远程创建 {target_branch} 分支{Colors.NC}")
            sys.exit(1)

    # 拉取目标分支最新代码
    print_step(f"拉取 {target_branch} 最新代码...")
    run_command(['git', 'pull', 'origin', target_branch])

    # 合并源分支
    print_step(f"合并 {source_branch} 到 {target_branch}...")
    try:
        run_command(['git', 'merge', source_branch, '--no-ff', '-m',
                     f'Merge branch {source_branch} into {target_branch}'])
        print_success(f"已合并 {source_branch} 到 {target_branch}")
    except subprocess.CalledProcessError as e:
        print(f"{Colors.RED}❌ 合并失败，可能存在冲突{Colors.NC}")
        print(f"{Colors.YELLOW}💡 请手动解决冲突后继续{Colors.NC}")
        sys.exit(1)

    # 推送到远程
    if args.push:
        print_step(f"推送 {target_branch} 到远程...")
        run_command(['git', 'push', 'origin', target_branch])
        print_success(f"已推送 {target_branch} 到远程")

    print(f"{Colors.CYAN}{'='*60}{Colors.NC}")
    print_success(f"合并完成！当前分支: {get_current_branch()}")
    print(f"{Colors.CYAN}{'='*60}{Colors.NC}")


if __name__ == '__main__':
    main()
