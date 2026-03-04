---
name: coding
description: 代码分析、优化。触发词：分析代码、优化代码、重构、debug、写代码
metadata: {"nanobot":{"emoji":"💻"}}
---
# 编码任务

## ⚡ 优先使用 opencode

**如果 opencode 可用**，调用 opencode 工具：
```json
{"action": "prompt", "task": "分析并优化代码", "agent": "build"}
```

---

## ⚡ 降级方案（opencode 不可用时）

**直接使用 file + shell 工具**，不要创建子任务！

### 步骤1： 了解项目结构
```json
{"command": "cd ~/.lingguard/workspace/<项目名> && find . -type f -name '*.go' | head -20"}
```

### 步骤2： 读取核心文件
```json
{"operation": "read", "path": "~/.lingguard/workspace/<项目名>/main.go"}
```

### 步骤3： 分析并修改代码
```json
{"operation": "edit", "path": "~/.lingguard/workspace/<项目名>/xxx.go", "old_string": "旧代码", "new_string": "新代码"}
```

### 步骤4： 验证修改
```json
{"command": "cd ~/.lingguard/workspace/<项目名> && go build ./..."}
```
