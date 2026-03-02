---
name: clawhub
description: 搜索和安装 AI 技能。当用户说"搜索技能"、"安装技能"、"热门技能"、"trending skills"、"clawhub"时，必须先加载此 skill 了解用法
homepage: https://clawhub.ai
metadata: {"lingguard":{"emoji":"🦞"}}
---

# ClawHub 技能搜索

公共 AI 技能注册表，支持自然语言搜索（向量搜索）。

## ⚠️ 重要规则

- **必须直接调用 shell 工具**执行 npx 命令，不要使用其他工具
- 安装完成后提醒用户开始新会话以加载技能

## 触发场景

- 搜索技能、查找技能
- 热门技能、trending
- 安装新技能
- 更新已安装技能

## 搜索技能

```bash
npx --yes clawhub@latest search "web scraping" --limit 5
```

## 安装技能

**重要：使用绝对路径，不要使用 cd 命令！**

```bash
npx --yes clawhub@latest install <slug> --workdir "$HOME/.lingguard/workspace"
```

将 `<slug>` 替换为搜索结果中的技能名称。

**如果出现安全警告：** 询问用户确认后使用 `--force`：
```bash
npx --yes clawhub@latest install <slug> --workdir "$HOME/.lingguard/workspace" --force
```

## 更新技能

```bash
npx --yes clawhub@latest update --all --workdir "$HOME/.lingguard/workspace"
```

## 列出已安装

```bash
npx --yes clawhub@latest list --workdir "$HOME/.lingguard/workspace"
```

## 注意事项

- 需要 Node.js（npx 随 Node.js 安装）
- 安装后需要开始新会话才能加载技能
- ClawHub 技能会覆盖同名的内置技能
