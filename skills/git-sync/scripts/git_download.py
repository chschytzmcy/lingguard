#!/usr/bin/env python3
"""
Git Download 脚本
用途：克隆新仓库 或 切换到 ai-test 分支并拉取最新代码

用法：
  # 克隆新仓库
  python3 git_download.py --clone ssh://git@gitlab.example.com:9022/group/repo.git --dir repo_name

  # 下载已有仓库
  cd /path/to/repo && python3 git_download.py
  # 或
  python3 git_download.py --dir /path/to/repo

环境变量：
  CODE_PATH: 仓库目录路径
  AI_BRANCH: AI 分支名称（默认 ai-test）
  MAIN_BRANCH: 主分支名称（自动检测）
"""

import subprocess
import sys
import os
import argparse

# 默认配置
WORKSPACE = os.path.expanduser('~/.lingguard/workspace')
AI_BRANCH = os.getenv('AI_BRANCH', 'ai-test')
MAIN_BRANCH = os.getenv('MAIN_BRANCH', '')  # 留空自动检测

class Colors:
    RED = '\033[0;31m'
    GREEN = '\033[0;32m'
    YELLOW = '\033[1;33m'
    BLUE = '\033[0;34m'
    NC = '\033[0m'

def run_cmd(cmd, cwd=None, check=True):
    """执行命令"""
    result = subprocess.run(cmd, capture_output=True, text=True, cwd=cwd)
    if check and result.returncode != 0:
        print(f"{Colors.RED}❌ 命令失败: {' '.join(cmd)}{Colors.NC}")
        print(result.stderr)
        sys.exit(1)
    return result

def get_main_branch(cwd):
    """自动检测主分支（优先 master，其次 main）"""
    result = run_cmd(['git', 'symbolic-ref', 'refs/remotes/origin/HEAD'],
                    cwd=cwd, check=False)
    if result.returncode == 0:
        return result.stdout.strip().split('/')[-1]

    for branch in ['master', 'main', 'develop']:
        result = run_cmd(['git', 'rev-parse', f'origin/{branch}'],
                        cwd=cwd, check=False)
        if result.returncode == 0:
            return branch

    return 'master'

def remote_branch_exists(branch, cwd):
    """检查远程分支是否存在"""
    result = run_cmd(['git', 'rev-parse', f'origin/{branch}'],
                    cwd=cwd, check=False)
    return result.returncode == 0

def clone_repo(repo_url, target_dir):
    """克隆新仓库"""
    print(f"{Colors.BLUE}📥 克隆仓库: {repo_url}{Colors.NC}")

    # 确保目标目录的父目录存在
    parent_dir = os.path.dirname(target_dir)
    if parent_dir and not os.path.exists(parent_dir):
        os.makedirs(parent_dir, exist_ok=True)

    # 克隆仓库
    result = run_cmd(['git', 'clone', repo_url, target_dir],
                    cwd=WORKSPACE, check=False)

    if result.returncode != 0:
        print(f"{Colors.RED}❌ 克隆失败{Colors.NC}")
        print(result.stderr)
        sys.exit(1)

    print(f"{Colors.GREEN}✅ 仓库已克隆到: {target_dir}{Colors.NC}")
    return target_dir

def setup_ai_branch(cwd):
    """设置 ai-test 分支"""
    main_branch = MAIN_BRANCH or get_main_branch(cwd)
    print(f"{Colors.YELLOW}📌 检测到主分支: {main_branch}{Colors.NC}")

    # 获取远程信息
    print(f"{Colors.YELLOW}📌 获取远程仓库信息...{Colors.NC}")
    run_cmd(['git', 'fetch', 'origin'], cwd=cwd)

    # 检查本地 ai-test 分支是否存在
    local_result = run_cmd(['git', 'rev-parse', '--verify', AI_BRANCH],
                          cwd=cwd, check=False)

    if local_result.returncode != 0:
        # 本地分支不存在
        if remote_branch_exists(AI_BRANCH, cwd):
            # 远程 ai-test 存在，从远程创建本地分支
            print(f"{Colors.YELLOW}📌 本地分支 {AI_BRANCH} 不存在，从远程 {AI_BRANCH} 创建...{Colors.NC}")
            run_cmd(['git', 'checkout', '-b', AI_BRANCH, f'origin/{AI_BRANCH}'], cwd=cwd)
            print(f"{Colors.GREEN}✅ 已创建并切换到分支 {AI_BRANCH}（跟踪远程）{Colors.NC}")
        else:
            # 远程 ai-test 也不存在，从主分支创建
            print(f"{Colors.YELLOW}📌 分支 {AI_BRANCH} 不存在，从 {main_branch} 创建...{Colors.NC}")
            run_cmd(['git', 'checkout', '-b', AI_BRANCH, f'origin/{main_branch}'], cwd=cwd)
            print(f"{Colors.GREEN}✅ 已创建并切换到分支 {AI_BRANCH}（从 {main_branch}）{Colors.NC}")
    else:
        # 本地分支存在，切换并拉取
        print(f"{Colors.YELLOW}📌 切换到分支 {AI_BRANCH}...{Colors.NC}")
        run_cmd(['git', 'checkout', AI_BRANCH], cwd=cwd)

        if remote_branch_exists(AI_BRANCH, cwd):
            print(f"{Colors.YELLOW}📌 拉取最新代码...{Colors.NC}")
            result = run_cmd(['git', 'pull', 'origin', AI_BRANCH], cwd=cwd, check=False)
            if result.returncode == 0:
                print(f"{Colors.GREEN}✅ 代码已更新{Colors.NC}")
            else:
                print(f"{Colors.YELLOW}⚠️ 拉取有冲突，尝试 rebase...{Colors.NC}")
                result = run_cmd(['git', 'pull', '--rebase', 'origin', AI_BRANCH],
                               cwd=cwd, check=False)
                if result.returncode == 0:
                    print(f"{Colors.GREEN}✅ 代码已更新（rebase）{Colors.NC}")
                else:
                    print(f"{Colors.YELLOW}⚠️ rebase 也有问题，请手动处理{Colors.NC}")
        else:
            # 远程没有 ai-test，需要推送
            print(f"{Colors.YELLOW}📌 推送 {AI_BRANCH} 到远程...{Colors.NC}")
            run_cmd(['git', 'push', '-u', 'origin', AI_BRANCH], cwd=cwd)
            print(f"{Colors.GREEN}✅ 已切换到分支 {AI_BRANCH} 并推送到远程{Colors.NC}")

def main():
    parser = argparse.ArgumentParser(description='Git 下载脚本')
    parser.add_argument('--clone', '-c', metavar='URL', help='克隆新仓库的 URL')
    parser.add_argument('--dir', '-d', metavar='DIR', help='仓库目录路径')

    args = parser.parse_args()

    # 确定工作目录
    if args.clone:
        # 克隆模式
        repo_url = args.clone

        if args.dir:
            if os.path.isabs(args.dir):
                target_dir = args.dir
            else:
                target_dir = os.path.join(WORKSPACE, args.dir)
        else:
            # 从 URL 提取目录名
            repo_name = repo_url.rstrip('/').split('/')[-1]
            if repo_name.endswith('.git'):
                repo_name = repo_name[:-4]
            target_dir = os.path.join(WORKSPACE, repo_name)

        # 检查目录是否已存在
        if os.path.exists(target_dir) and os.path.exists(os.path.join(target_dir, '.git')):
            print(f"{Colors.YELLOW}📌 目录已存在: {target_dir}{Colors.NC}")
            code_path = target_dir
        else:
            code_path = clone_repo(repo_url, target_dir)
    else:
        # 下载模式
        code_path = args.dir or os.getenv('CODE_PATH', os.getcwd())

        if not os.path.isabs(code_path):
            code_path = os.path.join(WORKSPACE, code_path)

    # 检查是否是 Git 仓库
    if run_cmd(['git', 'rev-parse', '--git-dir'], cwd=code_path, check=False).returncode != 0:
        print(f"{Colors.RED}❌ {code_path} 不是 Git 仓库{Colors.NC}")
        sys.exit(1)

    # 设置 ai-test 分支
    setup_ai_branch(code_path)

    print(f"\n{Colors.GREEN}🎉 下载完成！{Colors.NC}")
    print(f"{Colors.BLUE}📂 目录: {code_path}{Colors.NC}")
    print(f"{Colors.BLUE}🌿 分支: {AI_BRANCH}{Colors.NC}")

if __name__ == '__main__':
    main()
