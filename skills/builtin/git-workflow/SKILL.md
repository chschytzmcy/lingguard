---
name: git-workflow
description: Git 工作流自动化技能，包含拉取代码、创建 AI 分支、推送代码等操作，内置安全检查防止误推送到 master 分支
dependencies:
  - git (>= 2.30)
---

# Git Workflow

## 功能概述

自动化 Git 工作流程，包括：
- **拉取最新代码**：从远程仓库获取最新变更
- **创建 AI 分支**：自动创建名为 `ai-年月日时分秒` 的分支（如 `ai-20260210143025`）
- **从 master 创建**：每次都从 master 分支新建分支
- **推送代码**：安全地将代码推送到远程仓库
- **安全保护**：内置检查，禁止推送到 master 分支

## 使用场景

1. **日常开发流程**：快速拉取代码并创建个人开发分支
2. **AI 辅助开发**：为 AI 代码生成任务创建专属分支
3. **团队协作**：规范化的分支命名，便于识别 AI 贡献的代码
4. **安全防护**：防止意外推送代码到保护分支

## 前置条件

1. **Git 环境**：系统已安装 Git 2.30 或更高版本
2. **已配置 Git 用户**：
   ```bash
   git config --global user.name "Your Name"
   git config --global user.email "your.email@example.com"
   ```
3. **在 Git 仓库中**：当前目录必须是 Git 仓库的子目录
4. **远程仓库配置**：已配置 origin 远程仓库

## 核心功能

### 1. 拉取代码 (git_pull.py)

切换到 master 分支并拉取最新代码

```bash
./scripts/git_pull.py
```

### 2. 创建分支 (git_branch.py)

从 master 创建 AI 分支，命名格式：`ai-年月日时分秒`

```bash
./scripts/git_branch.py
# 输出：✅ 已创建并切换到分支 ai-20260210143025
```

### 3. 推送代码 (git_push.py)

推送代码到远程仓库（内置安全检查）

- ✅ 允许推送到：`ai-*`、`feature/*`、`bugfix/*`、`develop` 等非保护分支
- ❌ 禁止推送到：`master` 分支

```bash
./scripts/git_push.py
```

## 参考文件

- [reference.md](./reference.md) - 详细技术参考和 API 说明
- [examples.md](./examples.md) - 完整使用示例
- [scripts/git_pull.py](./scripts/git_pull.py) - 拉取代码脚本
- [scripts/git_branch.py](./scripts/git_branch.py) - 创建分支脚本
- [scripts/git_push.py](./scripts/git_push.py) - 推送代码脚本

## 安全特性

| 检查项 | 说明 |
|--------|------|
| 分支保护 | 拒绝推送到 master 分支 |
| 分支命名 | 自动使用 `ai-年月日时分秒` 格式 |
| 基准分支 | 每次从 master 分支创建 |
| 状态检查 | 推送前检查工作区状态 |
| 远程验证 | 确认远程仓库可访问 |
