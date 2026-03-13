---
name: media-scan
description: 扫描**用户指定的目录**中的媒体文件，找出包含特定内容的目标。触发场景：用户要"搜索/查找/找出"某个目录下的图片/视频/音频，如"找出小密盒里包含雷锋的图片"、"搜索 photos 文件夹里的猫"。**注意**：如果用户只是发送单张图片让你分析，不需要此 skill，直接回答即可。
metadata: {"nanobot":{"emoji":"🔍"}}
---

# 媒体扫描

扫描用户**指定的目录**中的媒体文件，识别特定内容。

## ⚠️ 重要：直接调用工具，不是 shell 命令

**直接调用 `media_scan` 工具**，不要用 shell 调用！

## 理解用户的目录描述

用户可能用自然语言描述目录：

| 用户说 | 目录参数 |
|--------|----------|
| "photos 文件夹" | `photos` |
| "下载的图片" | `downloads` 或 `Downloads` |
| "工作目录下的所有图片" | `.` |
| "media 目录里的" | `media` |

**如果不确定目录路径**，先用 `file` 工具列出工作目录确认。

## 工具参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| directory | string | ✅ | 要扫描的**相对目录**路径 |
| target | string | ✅ | 要查找的内容描述 |
| media_types | array | ❌ | `["image"]`、`["video"]`、`["audio"]`，默认全部 |
| recursive | bool | ❌ | 递归扫描子目录，默认 true |
| max_files | int | ❌ | 每种类型最大扫描数，默认 50 |

## 调用示例

### 示例 1：扫描小密盒目录

用户："找出小密盒里包含雷锋的图片"

```json
{
  "directory": "xiaomihe",
  "target": "雷锋头像"
}
```

### 示例 2：扫描 photos 目录

用户："在 photos 文件夹里找出包含猫的图片"

```json
{
  "directory": "photos",
  "target": "猫"
}
```

### 示例 3：扫描整个工作目录

用户："找出工作目录下包含人脸的图片"

```json
{
  "directory": ".",
  "target": "人脸"
}
```

### 示例 4：只扫描视频

用户："找出 videos 里包含人脸的视频"

```json
{
  "directory": "videos",
  "target": "人脸",
  "media_types": ["video"]
}
```

## 支持的媒体类型

| 类型 | 扩展名 |
|------|--------|
| 图片 | jpg, jpeg, png, gif, webp, bmp |
| 视频 | mp4, mov, avi, mkv, webm（限制 10MB） |
| 音频 | mp3, wav, m4a, opus, flac, aac |

## 前置条件

需要配置 `multimodalProvider`：

```json
{
  "agents": {
    "multimodalProvider": "qwen-vl"
  }
}
```
