---
name: clawhub
description: Search and install agent skills from ClawHub, the public skill registry.
homepage: https://clawhub.ai
metadata: {"lingguard":{"emoji":"🦞"}}
---

# ClawHub

ClawHub is a public skill registry for AI agents. Search by natural language (vector search).

**⚠️ CRITICAL RULES:**
1. **Use `shell` tool ONLY** - run commands directly
2. **Do NOT use opencode** - this is NOT a coding task
3. **Do NOT use git-workflow** - this is NOT a git operation
4. **Do NOT load other skills** - just execute npm commands

This task only requires running `npx` commands. Nothing else.

## When to use

Use this skill when the user asks any of:
- "find a skill for ..."
- "search for skills"
- "install a skill"
- "what skills are available?"
- "update my skills"
- "从 ClawHub 搜索/安装技能"

## Setup (One-time)

1. Get API token from https://clawhub.ai (login → Settings → API Tokens → Create Token)
2. Add to LingGuard config (`~/.lingguard/config.json`):
   ```json
   {
     "tools": {
       "clawhub": {
         "apiToken": "ch_xxxxxxxxxxxx"
       }
     }
   }
   ```

## Auto-login Command

Before running install/update, use this command to auto-login if token is configured:

```bash
# Check and auto-login
if npx --yes clawhub@latest whoami 2>&1 | grep -q "Not logged in"; then
  TOKEN=$(grep -o '"apiToken"[[:space:]]*:[[:space:]]*"[^"]*"' ~/.lingguard/config.json 2>/dev/null | sed 's/.*"\([^"]*\)"$/\1/')
  [ -n "$TOKEN" ] && npx --yes clawhub@latest login --token "$TOKEN" --no-browser
fi
```

## Commands

### Search (no login required)

```bash
npx --yes clawhub@latest search "web scraping" --limit 5
```

### Install (requires login)

```bash
# 1. Auto-login first (if configured)
npx --yes clawhub@latest whoami 2>&1 | grep -q "Not logged in" && \
  npx --yes clawhub@latest login --token "$(grep -o '"apiToken"[[:space:]]*:[[:space:]]*"[^"]*"' ~/.lingguard/config.json | sed 's/.*"\([^"]*\)"$/\1')" --no-browser

# 2. Try to install skill
npx --yes clawhub@latest install <slug> --workdir ~/.lingguard/workspace
```

**If security warning appears:**
1. Show the warning to user and ask for confirmation
2. If user confirms, run with `--force`:
   ```bash
   npx --yes clawhub@latest install <slug> --workdir ~/.lingguard/workspace --force
   ```

### Update All (requires login)

```bash
# Auto-login if needed, then update
npx --yes clawhub@latest whoami 2>&1 | grep -q "Not logged in" && \
  npx --yes clawhub@latest login --token "$(grep -o '"apiToken"[[:space:]]*:[[:space:]]*"[^"]*"' ~/.lingguard/config.json | sed 's/.*"\([^"]*\)"$/\1')" --no-browser

npx --yes clawhub@latest update --all --workdir ~/.lingguard/workspace
```

### List Installed

```bash
npx --yes clawhub@latest list --workdir ~/.lingguard/workspace
```

## Notes

- Requires Node.js (`npx` comes with it)
- After install, remind the user to start a new session to load the skill
- Skills installed via ClawHub will override builtin skills with the same name

## Example Flow

```
User: 从 ClawHub 安装 apewisdom 技能
You: [使用 shell 工具]
     # 搜索
     npx --yes clawhub@latest search "apewisdom" --limit 5

     [展示搜索结果]

     # 尝试安装
     npx --yes clawhub@latest install apewisdom --workdir ~/.lingguard/workspace

     # 如果出现安全警告：
     "⚠️ 安全警告: apewisdom 被标记为可疑，可能包含外部 API 调用等风险。
      是否继续安装？(y/n)"

User: 确认安装
You: [使用 --force 安装]
     npx --yes clawhub@latest install apewisdom --workdir ~/.lingguard/workspace --force

     [告知用户重启会话]
```

**Remember: Always use `shell` tool for clawhub commands!
