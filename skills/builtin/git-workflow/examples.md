---
name: git-workflow
description: Git 工作流使用示例
dependencies:
  - git (>= 2.30)
---

# Git Workflow 使用示例

## 基础示例

### 示例 1：完整工作流程

```bash
# 步骤 1：切换到 master 并拉取最新代码
git checkout master
git pull origin master
# 输出：Already up to date.

# 步骤 2：创建 AI 分支（基于当前时间戳，如 ai-20260210143025）
git checkout -b ai-$(date +%Y%m%d%H%M%S)
# 输出：Switched to a new branch 'ai-20260210143025'

# 步骤 3：查看当前分支
git branch
# 输出：
#   develop
# * ai-20260210143025
#   master

# 步骤 4：进行更改后提交
git add .
git commit -m "Add new feature"
# 输出：[ai-john-doe abc1234] Add new feature

# 步骤 5：推送到远程
git push -u origin ai-20260210143025
# 输出：
# Branch 'ai-20260210143025' set up to track remote branch 'ai-20260210143025' from 'origin'.
```

### 示例 2：使用辅助脚本

```bash
# 拉取 master 最新代码
./scripts/git_pull.py

# 创建 AI 分支
./scripts/git_branch.py
# 输出：✅ 已创建并切换到分支 ai-20260210143025

# 进行开发...
git add .
git commit -m "Add new feature"

# 安全推送（自动检查保护分支）
./scripts/git_push.py
```

## 高级示例

### 示例 3：理解时间戳命名

```bash
# 不同时间创建的分支示例
git checkout master && git pull origin master
git checkout -b ai-20260210143025   # 2026年2月10日 14:30:25

git checkout master && git pull origin master
git checkout -b ai-20260210153045   # 2026年2月10日 15:30:45

# 分支名格式：ai-YYYYMMDDHHMMSS
#            ai-年月日时分秒
```

### 示例 4：处理冲突

```bash
# 拉取 master 时遇到冲突
git checkout master
git pull origin master
# 输出：CONFLICT (content): Merge conflict in src/main.py

# 解决冲突后继续
git add src/main.py
git rebase --continue

# 创建 AI 分支
git checkout -b ai-$(date +%Y%m%d%H%M%S)

# 推送
git push -u origin ai-20260210143025
```

### 示例 5：安全保护触发示例

```bash
# 当前在 master 分支
git checkout master

# 尝试推送
./scripts/git_helper.py push
# 输出：❌ 错误：禁止推送到保护分支 'master'
#       请切换到 ai-*、feature/* 或其他开发分支
```

### 示例 6：批量操作多个仓库

```bash
#!/bin/bash
# multi-repo-update.sh

REPOS=(
    "/path/to/repo1"
    "/path/to/repo2"
    "/path/to/repo3"
)

for repo in "${REPOS[@]}"; do
    echo "处理仓库: $repo"
    cd "$repo"

    # 拉取 master
    git checkout master
    git pull origin master

    # 创建 AI 分支（基于时间戳）
    BRANCH="ai-$(date +%Y%m%d%H%M%S)"
    git checkout -b "$BRANCH"

    # 推送
    git push -u origin "$BRANCH"
done
```

## 场景示例

### 场景 1：AI 辅助修复 Bug

```bash
# 1. 从 master 拉取最新代码并创建分支
git checkout master && git pull origin master
git checkout -b ai-$(date +%Y%m%d%H%M%S)

# 2. AI 分析并修复代码
# ... AI 工作过程 ...

# 3. 提交修复
git add src/bug.py
git commit -m "Fix: resolve null pointer exception"

# 4. 推送并创建 PR
git push -u origin ai-20260210143025
gh pr create --title "AI Fix: Null pointer exception" --body "Automated fix by AI"
```

### 场景 2：AI 生成新功能

```bash
# 1. 从 master 创建功能分支
git checkout master && git pull origin master
git checkout -b ai-$(date +%Y%m%d%H%M%S)

# 2. AI 生成新功能代码
# 多个文件被修改...

# 3. 查看变更
git status
# 输出：
# modified:   src/api/user.py
# modified:   src/models/user.py
# new file:   tests/test_user_api.py

# 4. 提交所有更改
git add .
git commit -m "Feat: add user authentication API

- Add JWT token validation
- Add user registration endpoint
- Add unit tests"

# 5. 推送
git push -u origin ai-john-doe
```

### 场景 3：紧急修复流程

```bash
# 快速修复流程
git checkout master && git pull origin master
git checkout -b ai-$(date +%Y%m%d%H%M%S)

# 快速修复...
vim src/critical.py

git add src/critical.py
git commit -m "Hotfix: critical security patch"
git push -u origin ai-20260210143025

# 通知团队
# ...
```

## 与 CI/CD 集成示例

### GitHub Actions 工作流

```yaml
# .github/workflows/ai-branch-check.yml
name: AI Branch Protection

on:
  pull_request:
    branches: [master]

jobs:
  check-branch:
    runs-on: ubuntu-latest
    steps:
      - name: Check if PR is from AI branch
        run: |
          if [[ "${{ github.head_ref }}" =~ ^ai\+ ]]; then
            echo "✅ PR from AI branch detected"
            echo "ai_branch=true" >> $GITHUB_OUTPUT
          else
            echo "ai_branch=false" >> $GITHUB_OUTPUT
          fi
```

## 故障排除示例

### 问题：推送被拒绝

```bash
$ git push
! [rejected]        ai-20260210143025 -> ai-20260210143025 (fetch first)
error: failed to push some refs to 'origin'

# 解决方案：先拉取远程更新
git pull --rebase origin ai-20260210143025
git push
```

### 问题：分支名格式

```bash
# 确保分支名格式正确：ai-YYYYMMDDHHMMSS
# 错误示例：
#   ai-20260210143025  (使用了 - 而不是 +)
#   ai-2026-02-10      (包含了分隔符)

# 正确示例：
BRANCH="ai-$(date +%Y%m%d%H%M%S)"
git checkout -b "$BRANCH"
```
