---
name: meos-file
description: MEOS/MeBox 文件管理 HTTP API skill - 提供远程设备文件管理接口，包括创建、删除、元数据、标签、搜索、版本控制等操作；触发关键字：MEOS、meos、密盒、小密盒、会议助手、小密盒智会通、MEBOX、mebox、MeBox、盒子文件、盒子搜索
---

# MEOS Drive 对象服务 API

本 skill 提供 MEOS drive 对象服务 (`drive.object.v1.ObjectService`) 的完整 HTTP API 规范。

## 前置约束 ⚠️

在使用本 skill 之前，必须确保以下参数可用：

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `BOX_IP` | 盒子服务 IP 地址 | `127.0.0.1` |
| `BOX_PORT` | 盒子服务端口 | `8888` |

### 参数检查逻辑

1. **默认值**：
   - `BOX_IP` = `127.0.0.1`
   - `BOX_PORT` = `8080`

2. **环境变量覆盖**：
   - `MEBOX_BOX_IP` - 覆盖默认 IP
   - `MEBOX_BOX_PORT` - 覆盖默认端口

3. **基础 URL 格式**：
   ```
   http://{BOX_IP}:{BOX_PORT}/v1/files/
   ```

### 完整调用示例

```bash
# 默认访问本地服务
curl -X POST "http://127.0.0.1:8888/v1/files/search" \
  -H "Authorization: Bearer {token}" \
  -H "Content-Type: application/json" \
  -d '{"query": [{"term": {"name": "文档"}}]}'
```

---

## 可用接口

### 文件创建

| 方法 | 端点 | 说明 |
|------|------|------|
| POST | `/v1/files/create_link` | 创建链接文件 |

**请求 (CreateLinkFileRequest):**
```json
{
  "name": "string (最长255字符)",
  "target_uri": "string",
  "meta": { ... }
}
```

**响应 (CreateLinkFileResponse):**
```json
{
  "item": { FileInfo }
}
```

---

### 文件删除

| 方法 | 端点 | 说明 |
|------|------|------|
| DELETE | `/v1/files/delete` | 删除单个文件 |
| POST | `/v1/files/batch/delete` | 批量删除文件 |

**请求 (DeleteFileRequest):**
```json
{
  "file_id": "string",
  "delete_type": 1  // 1=回收站, 2=永久删除
}
```

**请求 (DeleteFilesRequest):**
```json
{
  "items": ["file_id_1", "file_id_2", ...],
  "delete_type": 1
}
```

**响应 (DeleteFilesResponse):**
```json
{
  "task_code": 0,
  "items": ["file_id_1", "file_id_2", ...]
}
```

---

### 文件元数据

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | `/v1/files/meta` | 获取文件元数据 |
| POST | `/v1/files/metadata` | 设置文件元数据 |
| PATCH | `/v1/files/rename` | 重命名文件 |

**请求 (SetFileMetadataRequest):**
```json
{
  "file_id": "string",
  "meta": { "key": "value", ... }
}
```

**请求 (RenameFileRequest):**
```json
{
  "file_id": "string",
  "name": "string (1-255字符)"
}
```

**响应 (RenameFileResponse):**
```json
{
  "item": { FileInfo }
}
```

---

### 文件收藏/星级

| 方法 | 端点 | 说明 |
|------|------|------|
| PATCH | `/v1/files/sc` | 设置单个文件星级 |
| POST | `/v1/files/batch/sc` | 批量设置文件星级 |

**请求 (SetFileSCRequest):**
```json
{
  "file_id": "string",
  "sc": 1  // 1=sc1x, 2=sc2x
}
```

**请求 (SetFilesScRequest):**
```json
{
  "items": [
    { "file_id": "string", "sc": 1 },
    ...
  ]
}
```

---

### 文件资源（封面/图标/缩略图/歌词）

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | `/v1/files/assets` | 获取文件资源 |
| POST | `/v1/files/assets` | 设置文件资源 |

**请求 (GetFileAssetRequest):**
```json
{
  "file_id": "string",
  "type": "cover|icon|thumb|transcoding|lyric"
}
```

**请求 (SetFileAssetsRequest):**
```json
{
  "file_id": "string",
  "assets": [
    { "type": "string", "spec": "string", "file_id": "string" },
    ...
  ]
}
```

---

### 用户标签

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | `/v1/files/tagging` | 获取文件标签 |
| POST | `/v1/files/tagging` | 更新文件标签 |
| POST | `/v1/files/tagging/remove` | 移除文件标签 |

**标签类型：**
- `1` = AI 标签 (AiTag)
- `2` = 用户标签 (UserTag)

**请求 (UpdateTaggingRequest):**
```json
{
  "file_id": "string",
  "tag_type": 1,  // FileTagType 枚举
  "tags": ["标签1", "标签2", ...]
}
```

**请求 (RemoveTaggingRequest):**
```json
{
  "file_id": "string",
  "tag_type": 1,
  "tags": ["标签1", ...]
}
```

**响应 (GetTaggingResponse):**
```json
{
  "tags": ["标签1", "标签2", ...]
}
```

---

### 描述标签

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | `/v1/files/tagging/descs` | 获取所有描述标签列表 |
| POST | `/v1/files/tagging/descs` | 更新描述标签 |
| GET | `/v1/files/tagging/desc` | 获取描述标签 |
| POST | `/v1/files/tagging/desc` | 设置描述标签（内部接口）|
| DELETE | `/v1/files/tagging/desc` | 删除描述标签 |

**请求 (UpdateDescTaggingRequest):**
```json
{
  "file_id": "string",
  "tags": { "key": ["value1", "value2"], ... }
}
```

**请求 (SetDescTaggingRequest):**
```json
{
  "file_id": "string",
  "key": "艺人",
  "tags": ["周杰伦", "林俊杰"]
}
```

**请求 (DeleteDescTaggingRequest):**
```json
{
  "file_id": "string",
  "key": "艺人",
  "values": ["周杰伦"]
}
```

---

### 文件版本

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | `/v1/files/version` | 获取文件版本列表 |
| POST | `/v1/files/version/restore` | 恢复文件版本 |

**请求 (ListVersionFilesRequest):**
```json
{
  "file_id": "string"
}
```

**响应 (ListVersionFilesResponse):**
```json
{
  "files": [ { fileVersionInfo }, ... ]
}
```

**请求 (RestoreVersionFilesRequest):**
```json
{
  "file_id": "string"
}
```

---

### 文件搜索

| 方法 | 端点 | 说明 |
|------|------|------|
| POST | `/v1/files/search` | 搜索文件 |
| POST | `/v1/files/search_count` | 统计搜索结果数量 |
| POST | `/v1/files/object_search` | 对象搜索（内部接口）|

**请求 (SearchFilesRequest):**

```json
{
  "match": { "字段": "模式" },
  "query": [ { "term": { "字段": "值" } } ],
  "category": "image|video|audio|doc|archive",
  "page": { "page": 1, "size": 20 },
  "order": "asc|desc"
}
```

**响应 (SearchFilesResponse):**

```json
{
  "files": [ { FileInfo }, ... ],
  "page_result": { "total": 100, "page": 1, "size": 20 }
}
```

**请求 (SearchCountRequest):**
```json
{
  "key": "搜索关键词",
  "query": [ ... ]
}
```

**响应 (SearchCountResponse):**
```json
{
  "total": 100
}
```

---

### 文件详情

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | `/v1/files/detail` | 获取文件详情 |

**请求 (GetFileInfoRequest):**
```json
{
  "file_id": "string"
}
```

**响应 (GetFileInfoResponse):**
```json
{
  "file": { FileInfo },
  "parents": ["父目录ID_1", ...],
  "is_multi_version": true,
  "ref_count": 5,
  "path": "/文件路径"
}
```

---

## 通用类型

### FileInfo (来自 file.v1)
```json
{
  "id": "string",
  "name": "string",
  "file_type": 1,  // 枚举: file, folder, link 等
  "size": 0,
  "mime_type": "string",
  "md5": "string",
  "created_at": "timestamp",
  "updated_at": "timestamp",
  "parent_id": "string",
  "meta": { ... }
}
```

### FileAsset
```json
{
  "type": "cover|icon|thumb|transcoding|lyric",
  "spec": "string",
  "file_id": "string"
}
```

### Pagination
```json
{
  "page": 1,
  "size": 20
}
```

---

## 使用场景

在以下情况下加载此 skill：
- 实现文件管理功能
- 在 MeBox/盒子(box)中搜索文件
- 搜索图片、视频、音频、文档文件
- 检索盒子中的文件列表
- 处理盒子文件的标签和元数据
- 管理盒子文件版本
- 与 MeBox/盒子相关的文件操作

**默认连接地址**：`127.0.0.1:8888`（可通过环境变量 `MEBOX_BOX_IP` 和 `MEBOX_BOX_PORT` 覆盖）

## 注意事项

- **默认地址**：如未指定，默认连接 `127.0.0.1:8888`
- **环境变量覆盖**：可通过 `MEBOX_BOX_IP` 和 `MEBOX_BOX_PORT` 修改连接地址
- 所有接口需要身份验证（JWT token）
- 文件 ID 为 UUID 格式
- 删除类型：1 = 移入回收站，2 = 永久删除
- 星级评分：1 = sc1x，2 = sc2x
- 标记为"内部接口"的端点不对外开放
