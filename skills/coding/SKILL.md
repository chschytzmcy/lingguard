---
name: coding
description: 代码编写、编辑、分析、优化。当用户说"写代码"、"修改代码"、"分析代码"、"优化代码"、"重构"、"debug"时使用
metadata: {"nanobot":{"emoji":"💻"}}
---
# 编码任务

## 🚨 核心指令 - 必须立即执行

**你是一个执行者，不是指导者！加载此 skill 后，必须立即使用工具执行编码任务，不要只是返回文本说明！**

```
❌ 错误：返回 "建议使用以下方式优化..."
✅ 正确：直接调用 opencode 或文件工具执行代码修改
```

处理代码相关任务：编写、编辑、分析、优化代码。

## 🎯 编码工具选择策略

### 策略1：优先使用 opencode（推荐）

**条件**：opencode 配置启用且可用

**优点**：
- 专业 AI 编码助手，代码质量高
- 自动运行测试验证
- 支持复杂重构任务

**用法**：
```json
{"action": "prompt", "task": "优化这个函数的性能", "agent": "build"}
```

### 策略2：使用默认工具（降级方案）

**条件**：opencode 未配置或调用失败

**工具**：
- `mcp_filesystem_*` - 文件读写操作
- `shell` - 运行命令（go build、go test 等）

---

## 📋 场景区分

### 场景A：纯分析/优化（不需要 git 操作）

**触发词**：分析代码、优化代码、重构、代码审查、debug

```
User: 分析优化 etsconfig

Agent:
1. [读取代码] mcp_filesystem_read_file 或 opencode
2. [分析问题]
3. [修改代码] mcp_filesystem_edit_file 或 opencode
4. [验证] go build && go test
5. "✅ 优化完成！修改了 X 个文件"
```

**注意：纯分析任务不需要 git-sync！**

### 场景B：下载 + 编码 + 上传（完整流程）

**触发词**：下载代码、上传代码、上库、git clone、git push

```
User: 下载 xxx，优化，并上库

Agent:
1. [calls skill --name git-sync] → 下载代码
2. [编码任务] opencode 或 mcp_filesystem_*
3. [验证] go build && go test
4. [calls skill --name git-sync] → 上传代码
5. "✅ 完成！代码已优化并推送"
```

---

## 🚨 git-sync 使用规则

**git-sync skill 只用于：**
- 下载代码（git clone / git pull）
- 上传代码（git push）

**git-sync skill 不用于：**
- ❌ 纯代码分析
- ❌ 代码优化（本地已有代码）
- ❌ 代码审查

**判断标准**：
| 用户需求 | 是否需要 git-sync |
|----------|-------------------|
| "分析优化 xxx" | ❌ 不需要 |
| "下载代码并优化" | ✅ 需要下载 |
| "优化后上库" | ✅ 需要上传 |
| "下载代码，优化，并上库" | ✅ 需要下载+上传 |

---

## 🔧 opencode 使用指南

### 小任务（单次调用）
- 修复单个 bug
- 添加一个函数
- 重构一个模块
- 编写简单功能

### 大任务（拆分多次调用）
- 分析整个项目结构
- 优化多个文件
- 跨模块重构
- 完整代码审查

**每个 opencode 调用控制在 20 分钟内**

### opencode 用法

```json
{
  "action": "prompt",
  "task": "任务描述",
  "agent": "build"
}
```

**agent 类型**：
- `build` - 编码任务（默认）
- `ask` - 代码分析

---

## 🔧 默认工具使用指南（降级方案）

当 opencode 不可用时，使用以下工具：

### 文件操作
```json
// 读取文件
{"tool": "mcp_filesystem_read_file", "path": "/path/to/file.go"}

// 写入文件
{"tool": "mcp_filesystem_write_file", "path": "/path/to/file.go", "content": "..."}

// 编辑文件
{"tool": "mcp_filesystem_edit_file", "path": "/path/to/file.go", "edits": [...]}
```

### 运行命令
```json
// 构建项目
{"tool": "shell", "command": "cd /path/to/project && go build ./..."}

// 运行测试
{"tool": "shell", "command": "cd /path/to/project && go test ./..."}
```

---

## ⚠️ 任务完成检查清单

### 纯分析任务
1. ✅ 代码已分析完成
2. ✅ 优化方案已实施
3. ✅ 代码已验证（编译通过/测试通过）
4. ✅ 向用户报告结果

### 涉及上传的任务
1. ✅ 代码已修改完成
2. ✅ 代码已验证
3. ✅ **代码已 git commit**
4. ✅ **代码已 git push**
5. ✅ 向用户报告结果

---

## 错误行为 vs 正确行为

### ❌ 错误行为
```
- "分析优化 xxx" 时调用 git-sync
- 直接用 shell git clone / git push（应该用 git-sync）
- opencode 返回后又用 file 工具验证
- 涉及上传任务时没 commit/push 就说完成
```

### ✅ 正确行为
```
- "分析优化 xxx" 时直接用 opencode 或 file 工具
- 只有需要下载/上传时才调用 git-sync
- 信任 opencode 的结果，不重复验证
- 根据用户需求决定是否需要 git 操作
```
