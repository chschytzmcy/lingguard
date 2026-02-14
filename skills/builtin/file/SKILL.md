---
name: file
description: File operations - read, write, edit, and list files
metadata: {"nanobot":{"emoji":"📁"}}
---
# File Operations

Use the `file` tool to work with files and directories.

## Operations

### Read a File

```json
{
  "operation": "read",
  "path": "/path/to/file.txt"
}
```

Returns the file contents as a string.

### Write a File

```json
{
  "operation": "write",
  "path": "/path/to/file.txt",
  "content": "File contents here"
}
```

Creates the file (and parent directories) if it doesn't exist.

### Edit a File

```json
{
  "operation": "edit",
  "path": "/path/to/file.txt",
  "old_string": "text to replace",
  "new_string": "replacement text"
}
```

Replaces all occurrences of `old_string` with `new_string`.

### List Directory

```json
{
  "operation": "list",
  "path": "/path/to/directory"
}
```

Returns entries with type prefix: `dir: dirname` or `file: filename`.

## Best Practices

1. **Read before edit**: Always read a file first to understand its structure
2. **Precise old_string**: Make `old_string` unique to avoid unintended replacements
3. **Check paths**: Verify paths exist before operations
4. **Backup important files**: Consider copying before major edits

## Safety

When sandbox mode is enabled, operations are restricted to the configured workspace directory.
