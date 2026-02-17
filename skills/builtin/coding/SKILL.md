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
5. ✅ **SPLIT large tasks** into smaller steps (each opencode call < 20 minutes)

**opencode returns complete results. Trust it. No verification needed.**

## Task Size Guidelines

### Small Tasks (single opencode call)
- Fix a bug in one file
- Add a single function
- Refactor one module
- Write a simple feature

### Large Tasks (MUST split into multiple calls)
- Analyze entire project structure
- Optimize multiple files
- Major refactoring across modules
- Full project code review

**Split Pattern:**
```
Step 1: opencode - "Analyze project structure, list main files and their purposes"
Step 2: opencode - "Review file X for optimization opportunities"
Step 3: opencode - "Apply optimizations to file X"
Step 4: opencode - "Review and optimize file Y"
...continue as needed...
```

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
