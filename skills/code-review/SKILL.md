---
name: code-review
description: 代码审查。当用户说"review代码"、"代码审查"、"检查代码"、"审查代码"、"代码质量检查"时使用
metadata: {"nanobot":{"emoji":"🔍"}}
---

# 代码审查

## 🚨 核心指令 - 必须立即执行

**你是一个执行者，不是指导者！加载此 skill 后，必须立即使用工具审查代码，不要只是返回文本说明！**

```
❌ 错误：返回 "建议使用以下方式审查..."
✅ 正确：直接调用工具读取代码并分析
```

---

## 审查流程

### 1. 确定审查范围

**⚡ 立即执行（使用 shell 工具）：**

```bash
# 查看 workspace 下的仓库
ls ~/.lingguard/workspace/

# 查看目标仓库的改动
cd ~/.lingguard/workspace/<repo> && git status
cd ~/.lingguard/workspace/<repo> && git diff
```

### 2. 读取关键文件

**⚡ 使用 shell 工具读取文件：**

```bash
# 读取单个文件
cat ~/.lingguard/workspace/<repo>/path/to/file.go

# 查看目录结构
find ~/.lingguard/workspace/<repo> -type f -name "*.go" | head -20
```

### 3. 执行审查并输出结果

---

## 审查清单

### 🔴 安全问题（必须修复）
- SQL 注入漏洞
- XSS（跨站脚本攻击）
- 命令注入
- 敏感数据泄露
- 认证/授权问题
- 硬编码的密钥或凭证

### 🟡 代码质量（建议修复）
- 代码重复
- 复杂的条件逻辑
- 缺少错误处理
- 资源泄露（文件句柄、连接）
- 并发代码中的竞态条件
- 空指针解引用

### 🟢 最佳实践（可选改进）
- 命名规范
- 必要的文档/注释
- 函数/方法长度
- 测试覆盖率
- SOLID 原则遵循

---

## 输出格式

```
## 📋 审查报告

### 概述
[代码功能简介]

### 🔴 关键问题（必须修复）
1. [文件:行号] 问题描述
   - 问题代码: `xxx`
   - 修复建议: xxx

### 🟡 警告（建议修复）
1. [文件:行号] 问题描述

### 🟢 改进建议
1. [建议内容]

### ✅ 优点
- [发现的良好实践]
```

---

## 使用示例

```
User: 审查 etsconfig 的代码

Agent 行为：
1. [调用 skill 工具] skill --name code-review
2. [立即调用 shell] cd ~/.lingguard/workspace/etsconfig && git status
3. [调用 shell] cd ~/.lingguard/workspace/etsconfig && git diff
4. [调用 shell] cat ~/.lingguard/workspace/etsconfig/internal/xxx/xxx.go
5. [分析并输出审查报告]
```

---

## 注意事项

- **纯审查任务不需要 git-sync**：只在需要下载/上传代码时才使用 git-sync
- **聚焦改动**：优先审查 git diff 显示的变更
- **提供可操作建议**：给出具体的修复代码，不只是泛泛而谈
