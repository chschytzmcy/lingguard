---
name: media-recognize
description: 媒体识别。扫描目录中的媒体文件，识别特定内容。触发场景：用户要"找出/搜索/查找"某个目录下的图片/视频/音频，如"找出包含雷锋的图片"。**注意**：如果用户只是发送单张图片让你分析，直接回答即可，无需此 skill。
metadata: {"nanobot":{"emoji":"🔍"}}
---

# 媒体识别

## ⚠️ 重要：直接调用工具

**直接调用 `media_scan` 工具**，不要用 shell 命令调用！

## 理解用户的目录描述

用户可能用自然语言描述目录：

| 用户说 | 目录参数 |
|--------|----------|
| "photos 文件夹" | `photos` |
| "下载的图片" | `downloads` |
| "工作目录下的所有图片" | `.` |
| "media 目录里的" | `media` |

**如果不确定目录路径**，先用 `file` 工具列出工作目录确认。

## 调用示例

### 识别小密盒目录

用户："找出小密盒里包含雷锋的图片"

```json
{
  "directory": "xiaomihe",
  "target": "雷锋头像"
}
```

### 识别整个工作目录

用户："找出工作目录下包含人脸的图片"

```json
{
  "directory": ".",
  "target": "人脸"
}
```

### 只识别视频

用户："找出 videos 里包含人脸的视频"

```json
{
  "directory": "videos",
  "target": "人脸",
  "media_types": ["video"]
}
```

## 前置条件

需要配置 `multimodalProvider`：

```json
{
  "agents": {
    "multimodalProvider": "qwen-vl"
  }
}
```
