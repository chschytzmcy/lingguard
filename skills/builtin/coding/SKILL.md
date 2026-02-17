---
name: coding
description: MUST use opencode tool for ALL coding tasks - writing, editing, analyzing code
metadata: {"nanobot":{"emoji":"💻","always":true}}
always: true
---
# Coding Tasks - MUST Use OpenCode

**CRITICAL: For ANY coding-related task, you MUST use ONLY the `opencode` tool.**

## ABSOLUTE RULES

1. ✅ **ONLY use `opencode`** for coding tasks
2. ❌ **NEVER use `file` tool** after opencode completes
3. ❌ **NEVER use `shell` tool** to verify opencode's work
4. ❌ **NEVER use `workspace` tool** to check files created by opencode

**opencode returns complete results. Trust it. No verification needed.**

## What opencode Handles

- Creating files
- Writing code
- Running tests
- Verifying results

**You do NOT need to check or verify anything after opencode returns.**

## Usage

```json
{"action": "prompt", "task": "Write a Go function that does X", "agent": "build"}
```

## WRONG Behavior (DO NOT DO THIS)

```
User: Write a calculator in Go
You: [calls opencode]
     [calls file tool to check]  ← WRONG!
     [calls shell to run tests]   ← WRONG!
```

## CORRECT Behavior

```
User: Write a calculator in Go
You: [calls opencode]
     [returns opencode's response directly]
```

**opencode result is final. Report it directly to user. No follow-up tools.**
