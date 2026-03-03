#!/usr/bin/env python3
"""
Git Upload 脚本
用途：提交所有更改并推送到 ai-test 分支

用法：
  python3 git_upload.py --dir /path/to/repo
  或
  cd /path/to/repo && python3 git_upload.py
"""

import subprocess
import sys
import os
import argparse
from datetime import datetime

# 默认配置
WORKSPACE = os.path.expanduser('~/.lingguard/workspace')
AI_BRANCH = os.getenv('AI_BRANCH', 'ai-test')

class Colors:
    RED = '\033[0;31m'
    GREEN = '\033[0;32m'
    YELLOW = '\033[1;33m'
    NC = '\033[0m'

def run_cmd(cmd, cwd, check=True):
    """执行命令"""
    result = subprocess.run(cmd, capture_output=True, text=True, cwd=cwd)
    if check and result.returncode != 0:
        print(f"{Colors.RED}❌ 命令失败: {' '.join(cmd)}{Colors.NC}")
        print(result.stderr)
        sys.exit(1)
    return result

def find_repo_dir(specified_dir):
    """查找仓库目录"""
    if specified_dir:
        if os.path.isabs(specified_dir):
            return specified_dir
        return os.path.join(WORKSPACE, specified_dir)

    # 优先查找 workspace 下的唯一仓库
    if os.path.exists(WORKSPACE):
        repos = [d for d in os.listdir(WORKSPACE)
                 if os.path.isdir(os.path.join(WORKSPACE, d))
                 and os.path.exists(os.path.join(WORKSPACE, d, '.git'))]
        if len(repos) == 1:
            return os.path.join(WORKSPACE, repos[0])
        elif len(repos) > 1:
            print(f"{Colors.YELLOW}⚠️ 发现多个仓库，请指定目录:{Colors.NC}")
            for r in repos:
                print(f"  - {r}")
            sys.exit(1)

    # 如果没有指定，尝试从环境变量
    code_path = os.getenv('CODE_PATH')
    if code_path:
        return code_path

    # 最后检查当前目录是否是 git 仓库
    if os.path.exists('.git'):
        return os.getcwd()

    print(f"{Colors.RED}❌ 未找到 Git 仓库{Colors.NC}")
    sys.exit(1)

def main():
    parser = argparse.ArgumentParser(description='Git 上传脚本')
    parser.add_argument('--dir', '-d', metavar='DIR', help='仓库目录路径')
    args = parser.parse_args()

    code_path = find_repo_dir(args.dir)

    print(f"{Colors.YELLOW}📂 工作目录: {code_path}{Colors.NC}")

    # 检查是否是 Git 仓库
    if run_cmd(['git', 'rev-parse', '--git-dir'], code_path, check=False).returncode != 0:
        print(f"{Colors.RED}❌ {code_path} 不是 Git 仓库{Colors.NC}")
        sys.exit(1)

    # 获取当前分支
    result = run_cmd(['git', 'rev-parse', '--abbrev-ref', 'HEAD'], code_path)
    current_branch = result.stdout.strip()

    # 检查是否在 ai-test 分支
    if current_branch != AI_BRANCH:
        print(f"{Colors.RED}❌ 当前在 {current_branch} 分支，请先切换到 {AI_BRANCH} 分支{Colors.NC}")
        print(f"{Colors.YELLOW}💡 运行: python3 git_download.py{Colors.NC}")
        sys.exit(1)

    # 检查是否有更改
    result = run_cmd(['git', 'status', '--porcelain'], code_path)
    if not result.stdout.strip():
        print(f"{Colors.YELLOW}⚠️ 没有需要提交的更改{Colors.NC}")
        sys.exit(0)

    # 显示更改
    print(f"{Colors.YELLOW}📌 检测到以下更改:{Colors.NC}")
    result = run_cmd(['git', 'status', '--short'], code_path)
    print(result.stdout)

    # 添加所有更改
    print(f"{Colors.YELLOW}📌 添加更改到暂存区...{Colors.NC}")
    run_cmd(['git', 'add', '-A'], code_path)

    # 生成提交信息
    timestamp = datetime.now().strftime('%Y-%m-%d %H:%M:%S')
    commit_msg = f"AI update: {timestamp}"

    # 提交
    print(f"{Colors.YELLOW}📌 提交更改...{Colors.NC}")
    run_cmd(['git', 'commit', '-m', commit_msg], code_path)
    print(f"{Colors.GREEN}✅ 已提交: {commit_msg}{Colors.NC}")

    # 推送
    print(f"{Colors.YELLOW}📌 推送到 {AI_BRANCH}...{Colors.NC}")
    result = run_cmd(['git', 'push', '-u', 'origin', AI_BRANCH], code_path, check=False)
    if result.returncode == 0:
        print(f"{Colors.GREEN}✅ 推送成功{Colors.NC}")
    else:
        # 可能是远程分支不存在，尝试推送
        print(f"{Colors.YELLOW}⚠️ 尝试推送新分支...{Colors.NC}")
        result = run_cmd(['git', 'push', 'origin', AI_BRANCH], code_path, check=False)
        if result.returncode == 0:
            print(f"{Colors.GREEN}✅ 推送成功{Colors.NC}")
        else:
            print(f"{Colors.RED}❌ 推送失败{Colors.NC}")
            print(result.stderr)
            sys.exit(1)

if __name__ == '__main__':
    main()
