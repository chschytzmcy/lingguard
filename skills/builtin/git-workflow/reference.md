---
name: git-workflow
description: Git 工作流详细技术参考
dependencies:
  - git (>= 2.30)
---

# Git Workflow 技术参考

## 功能概述

本文档提供 Git Workflow 技能的详细技术实现说明，包括命令参数、错误处理和安全机制。

## 命令详解

### 拉取代码 (git pull/fetch)

```bash
# 获取远程更新，不合并
git fetch origin

# 拉取并合并到当前分支
git pull origin <branch>

# 拉取使用 rebase 策略（推荐）
git pull --rebase origin <branch>
```

**参数说明**：
| 参数 | 说明 |
|------|------|
| `--all` | 获取所有远程分支 |
| `--rebase` | 使用 rebase 而非 merge |

### 创建分支 (git checkout/switch)

```bash
# 从 master 分支创建 AI 分支
# 分支名格式：ai-YYYYMMDDHHMMSS
git checkout master
git pull origin master
git checkout -b ai-$(date +%Y%m%d%H%M%S)

# 示例分支名：
# ai-20260210143025
# ai-20260210153045

# 新方式（Git >= 2.23）
git switch master
git pull origin master
git switch -c ai-$(date +%Y%m%d%H%M%S)
```

### 推送代码 (git push)

```bash
# 推送当前分支到远程同名分支
git push

# 推送并设置上游分支
git push -u origin ai-20260210143025

# 注意：不建议使用强制推送，可能导致数据丢失
```

## 安全检查机制

### 分支名称验证

```bash
# 生成基于时间戳的分支名
TIMESTAMP=$(date +%Y%m%d%H%M%S)
BRANCH_NAME="ai-${TIMESTAMP}"

# 示例输出：ai-20260210143025
```

### 保护分支列表

| 分支模式 | 说明 | 是否允许推送 |
|----------|------|--------------|
| `master` | 主分支 | ❌ 禁止 |
| `ai-*` | AI 开发分支（时间戳格式） | ✅ 允许 |
| `feature/*` | 功能分支 | ✅ 允许 |
| `bugfix/*` | 修复分支 | ✅ 允许 |
| `develop` | 开发分支 | ✅ 允许 |

### 推送前检查流程

```bash
# 1. 检查当前分支
CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)

# 2. 检查是否为保护分支
if [[ "$CURRENT_BRANCH" == "master" ]]; then
    echo "错误：禁止推送到保护分支 $CURRENT_BRANCH"
    exit 1
fi

# 3. 检查工作区状态
if [[ -n $(git status --porcelain) ]]; then
    echo "警告：工作区有未提交的更改"
fi

# 4. 执行推送
git push -u origin "$CURRENT_BRANCH"
```

## 错误处理

### 常见错误及解决方案

| 错误 | 原因 | 解决方案 |
|------|------|----------|
| `fatal: not a git repository` | 不在 Git 仓库中 | 切换到仓库目录 |
| `fatal: 'origin' does not appear to be a git repository` | 远程仓库未配置 | 添加 `git remote add origin <url>` |
| `error: failed to push some refs` | 远程有新提交 | 先执行 `git pull --rebase` |
| ` Updates were rejected because the tip of your current branch is behind` | 本地落后于远程 | 执行 `git pull` 合并远程更新 |

### 脚本退出码

| 代码 | 含义 |
|------|------|
| 0 | 成功 |
| 1 | 一般错误 |
| 2 | 误用（命令用法错误） |
| 128 | Git 严重错误 |

## Git 配置

### 查看当前配置

```bash
git config --list
git config user.name
git config user.email
```

### 设置用户信息

```bash
# 全局配置
git config --global user.name "Your Name"
git config --global user.email "your.email@example.com"

# 仓库级别配置（优先级更高）
git config --local user.name "Your Name"
git config --local user.email "your.email@example.com"
```

## 完整工作流程示例

```bash
# 1. 进入项目目录
cd /path/to/project

# 2. 拉取 master 最新代码
./scripts/git_pull.py

# 3. 创建 AI 分支（基于时间戳）
./scripts/git_branch.py
# 输出：✅ 已创建并切换到分支 ai-20260210143025

# 4. 进行开发工作...
# ... 编写代码 ...

# 5. 提交更改
git add .
git commit -m "AI generated changes"

# 6. 推送到远程（带安全检查）
./scripts/git_push.py
```
