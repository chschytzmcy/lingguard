---
name: media-generate
description: 媒体生成。AI 创作图片和视频。触发场景：用户要"画"、"生成"、"创建"视觉内容，如"画一只猫"、"生成美女图片"、"做个视频"、"让这张图动起来"。**注意**：如果用户只是发送图片让你**看**、**识别**、**分析**、**描述**内容（如"这是什么"、"图片里有什么"），不需要此 skill，直接用多模态能力回答即可。
metadata: {"nanobot":{"emoji":"🎨"}}
---

# 媒体生成

## 🚨 必须遵守的规则

1. **直接调用 `aigc` 工具**，不要调用其他任何工具
2. **不要创建目录**：aigc 工具会自动创建输出目录
3. **不要使用 shell/file/MCP 工具**：只调用 aigc 工具
4. **每次调用生成新内容**：不要返回历史路径

## 立即调用示例

| 用户说 | 立即调用 |
|--------|----------|
| "画一只猫" | `{"action": "generate_image", "prompt": "一只可爱的猫咪"}` |
| "生成美女图片" | `{"action": "generate_image", "prompt": "美女描述"}` |
| "生成一个猫咪视频" | `{"action": "generate_video", "prompt": "猫咪在花园里玩耍"}` |
| "让这张图动起来" | `{"action": "generate_video_from_image", "prompt": "画面开始动起来", "image_path": "图片路径"}` |
| "用这个视频生成新视频" | `{"action": "generate_video_from_video", "prompt": "人物开始跳舞", "video_path": "视频路径"}` |

---

## 四种生成模式

### 1️⃣ 文生图 (generate_image)

根据文字描述生成静态图片。

**触发场景**：
- "画一张"、"生成图片"、"帮我画"
- "文生图"、"生成一张图"

**用法**：
```json
{
  "action": "generate_image",
  "prompt": "一只可爱的猫咪坐在椅子上，卡通风格",
  "size": "1024x1024",
  "style": "anime"
}
```

**参数**：
| 参数 | 说明 | 必填 |
|------|------|------|
| action | 固定值 `generate_image` | ✅ |
| prompt | 图片描述 | ✅ |
| size | 尺寸：1024x1024, 720x1280 等 | ❌ |
| style | 风格：anime, realistic, 3d | ❌ |

**示例对话**：
- 用户："画一只可爱的小猫"
- 调用：`{"action": "generate_image", "prompt": "一只可爱的猫咪，卡通风格"}`

---

### 2️⃣ 文生视频 (generate_video)

根据文字描述生成视频。

**触发场景**：
- "生成视频"、"做个视频"
- "文生视频"、"生成一个视频"

**用法**：
```json
{
  "action": "generate_video",
  "prompt": "一只猫在花园里散步，阳光明媚",
  "duration": 5
}
```

**参数**：
| 参数 | 说明 | 必填 |
|------|------|------|
| action | 固定值 `generate_video` | ✅ |
| prompt | 视频内容描述 | ✅ |
| duration | 时长（秒），默认5，最大10 | ❌ |

**示例对话**：
- 用户："生成一个猫咪在花园里玩耍的视频"
- 调用：`{"action": "generate_video", "prompt": "一只猫咪在花园里欢快地玩耍", "duration": 5}`

---

### 3️⃣ 图生视频 (generate_video_from_image)

让静态图片动起来，生成视频。

**触发场景**：
- "让这张图动起来"、"把图片变成视频"
- "图生视频"、"让这张图片动一动"

**用法**：
```json
{
  "action": "generate_video_from_image",
  "prompt": "人物开始微笑并挥手",
  "image_path": "~/.lingguard/workspace/generated/image-xxx.png",
  "duration": 5
}
```

**参数**：
| 参数 | 说明 | 必填 |
|------|------|------|
| action | 固定值 `generate_video_from_image` | ✅ |
| prompt | 动作描述（描述图片中的内容如何动） | ✅ |
| image_path | 图片的绝对路径 | ✅ |
| duration | 时长（秒），默认5，最大15 | ❌ |

**示例对话**：
- 用户："让这张美女图片动起来"
- 调用：`{"action": "generate_video_from_image", "prompt": "美女开始微笑并向镜头挥手", "image_path": "之前生成的图片路径"}`

**注意事项**：
- image_path 必须是**绝对路径**
- 可以使用之前生成的图片路径
- prompt 描述的是**动作**，不是场景

---

### 4️⃣ 视频生视频 (generate_video_from_video)

基于已有视频生成新视频，保持角色一致性。

**触发场景**：
- "用这个视频生成新视频"
- "视频生视频"、"让视频里的人做其他动作"

**用法**：
```json
{
  "action": "generate_video_from_video",
  "prompt": "人物开始跳舞",
  "video_path": "~/.lingguard/workspace/generated/video-xxx.mp4",
  "duration": 5
}
```

**参数**：
| 参数 | 说明 | 必填 |
|------|------|------|
| action | 固定值 `generate_video_from_video` | ✅ |
| prompt | 新动作描述 | ✅ |
| video_path | 原视频的绝对路径 | ✅ |
| duration | 时长（秒），5 或 10 | ❌ |

**示例对话**：
- 用户："让视频里的人开始跳舞"
- 调用：`{"action": "generate_video_from_video", "prompt": "人物开始跳街舞", "video_path": "原视频路径"}`

**注意事项**：
- 视频生视频**保持角色一致性**
- prompt 描述**新的动作**
- 时长只能是 5 或 10 秒

---

## 参数汇总

| 参数 | 说明 | 文生图 | 文生视频 | 图生视频 | 视频生视频 |
|------|------|:------:|:--------:|:--------:|:----------:|
| action | 动作类型 | ✅ | ✅ | ✅ | ✅ |
| prompt | 描述 | ✅ | ✅ | ✅ | ✅ |
| image_path | 图片路径 | - | - | ✅ | - |
| video_path | 视频路径 | - | - | - | ✅ |
| duration | 时长(秒) | - | 5-10 | 5-15 | 5/10 |
| size | 图片尺寸 | ❌ | - | - | - |
| style | 风格 | ❌ | - | - | - |

---

## 文件路径

| 类型 | 路径 |
|------|------|
| 生成的图片/视频 | `~/.lingguard/workspace/generated/` |
| 聊天下载的媒体 | `~/.lingguard/workspace/media/` |

**获取路径**：生成的文件会返回完整路径，可直接用于后续的图生视频/视频生视频。

---

## 可用模型

| 模型 | 用途 |
|------|------|
| wan2.6-t2i | 文生图（默认） |
| wan2.6-t2v | 文生视频 |
| wan2.6-i2v-flash | 图生视频 |
| wan2.6-r2v-flash | 视频生视频 |

---

## 完整示例流程

### 场景1：连续生成图片和视频

```
用户: "画一张美女图片"
→ {"action": "generate_image", "prompt": "美女描述..."}
→ 返回: 图片保存到 /home/xxx/.lingguard/workspace/generated/image-xxx.png

用户: "让这张图动起来，让她微笑"
→ {"action": "generate_video_from_image", "prompt": "美女开始微笑", "image_path": "/home/xxx/.lingguard/workspace/generated/image-xxx.png"}
→ 返回: 视频保存到 /home/xxx/.lingguard/workspace/generated/video-xxx.mp4

用户: "让视频里的人开始跳舞"
→ {"action": "generate_video_from_video", "prompt": "美女开始跳舞", "video_path": "/home/xxx/.lingguard/workspace/generated/video-xxx.mp4"}
→ 返回: 新视频保存到 ...
```

### 场景2：直接生成视频

```
用户: "生成一个猫咪在花园里玩的视频"
→ {"action": "generate_video", "prompt": "一只可爱的猫咪在花园里欢快地玩耍，追逐蝴蝶", "duration": 5}
→ 返回: 视频保存到 /home/xxx/.lingguard/workspace/generated/video-xxx.mp4
```
