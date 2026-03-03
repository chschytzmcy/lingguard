---
name: git-sync
description: Git 代码同步（下载/上传）。当用户说"下载代码"、"上传代码"、"同步代码"、"拉取代码"、"推送代码"、"提交代码"、"git clone"、"git push"、"上库"、"克隆仓库"、"push代码"、"commit代码"、"上传优化后的代码"时使用，固定使用 ai-test 分支
metadata: {"nanobot":{"emoji":"🔄","requires":{"bins":["git","python3"]}}}
---

# Git 代码同步

## 🚨 核心指令 - 必须立即执行

**你是一个执行者，不是指导者！加载此 skill 后，必须立即使用 shell 工具执行脚本，不要只是返回文本说明！**

```
❌ 错误：返回 "请使用以下命令..." 或 "执行步骤如下..."
✅ 正确：直接调用 shell 工具执行 python3 ~/.lingguard/skills/git-sync/scripts/xxx.py
```

## ⚠️ 重要规则

- **立即执行脚本**：加载此 skill 后，必须立即用 shell 工具执行对应脚本
- **禁止只返回文本**：不要向用户展示命令，而是直接执行命令
- 固定使用 `ai-test` 分支进行所有操作
- 工作目录：`~/.lingguard/workspace`

## 🚨 上传代码必做清单

**在任务结束前，必须执行以下操作：**

1. ✅ `git status` 检查修改
2. ✅ `git add -A` 添加所有修改
3. ✅ `git commit -m "描述"` 提交
4. ✅ `git push origin ai-test` 推送

**❌ 禁止在未执行 git push 的情况下说"任务完成"！**

---

## 用法

### 🟢 克隆新仓库（推荐）

**⚡ 立即执行以下命令（使用 shell 工具）：**

```bash
python3 ~/.lingguard/skills/git-sync/scripts/git_download.py --clone <仓库URL>
```

**执行示例（复制并修改 URL）：**
```bash
python3 ~/.lingguard/skills/git-sync/scripts/git_download.py --clone ssh://git@gitlab.etsme.com:9022/oam/etsconfig.git
```

**脚本自动完成**：
1. 克隆仓库到 `~/.lingguard/workspace/`
2. 创建 `ai-test` 分支
3. 推送到远程 `ai-test`

---

### 🟢 下载已有仓库

**⚡ 立即执行以下命令（使用 shell 工具）：**

```bash
cd ~/.lingguard/workspace/<repo> && python3 ~/.lingguard/skills/git-sync/scripts/git_download.py
```

**脚本自动完成**：
1. 检测主分支（master/main）
2. 切换到 `ai-test` 分支（不存在则创建）
3. 拉取最新代码

---

### 🟢 上传代码

**⚡ 立即执行以下命令（使用 shell 工具）：**

```bash
python3 ~/.lingguard/skills/git-sync/scripts/git_upload.py
```

**脚本自动完成**：
1. 自动检测 workspace 下的仓库目录
2. 检查是否在 `ai-test` 分支
3. 检查是否有更改
4. 添加所有更改到暂存区
5. 提交（自动生成提交信息）
6. 推送到远程 `ai-test` 分支

---

## ❌ 错误行为（禁止）

```
❌ 返回文本说明："请使用以下命令克隆仓库..."
❌ 不执行 shell 命令就说任务完成
❌ 只加载 skill 不执行脚本
```

## ✅ 正确行为

```
✅ 加载 skill 后，立即调用 shell 工具执行脚本
✅ 等待脚本执行结果
✅ 向用户报告执行结果
```

---

## 完整流程示例

### 下载代码
```
User: 下载 ssh://git@gitlab.etsme.com:9022/oam/etsconfig.git

Agent 行为：
1. [调用 skill 工具] skill --name git-sync
2. [收到 skill 指令后]
3. [立即调用 shell 工具] python3 ~/.lingguard/skills/git-sync/scripts/git_download.py --clone ssh://git@gitlab.etsme.com:9022/oam/etsconfig.git
4. [等待脚本执行完成]
5. [返回结果] "✅ 仓库已克隆到 ~/.lingguard/workspace/etsconfig，分支 ai-test"
```

### 上传代码
```
User: 上传代码 / 推送代码 / 提交代码

Agent 行为：
1. [调用 skill 工具] skill --name git-sync
2. [收到 skill 指令后]
3. [立即调用 shell 工具] python3 ~/.lingguard/skills/git-sync/scripts/git_upload.py
4. [等待脚本执行完成]
5. [返回结果] "✅ 代码已提交并推送到 ai-test 分支"
```

---

## 脚本文件

| 脚本 | 功能 |
|------|------|
| `git_download.py` | 克隆新仓库 / 下载已有仓库 |
| `git_upload.py` | 上传代码（commit + push）|
