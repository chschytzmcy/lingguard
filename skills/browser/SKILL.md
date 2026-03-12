---
name: browser
description: 浏览器自动化工具。当用户说"打开网页"、"截图"、"网页截图"、"浏览器"、"访问网站"、"点击"、"填写表单"时，必须先加载此 skill 了解用法
metadata: {"nanobot":{"emoji":"🌐"}}
---

# Browser 浏览器自动化技能

浏览器自动化工具，基于 Chrome DevTools Protocol 实现网页操作、截图、表单填写等功能。

## 功能概述

- 网页导航与控制
- 元素查找与操作（点击、输入、获取文本）
- 页面截图
- JavaScript 执行
- 多标签页管理
- 文件上传

## 配置

在 `config.json` 中添加：

```json
{
  "tools": {
    "browser": {
      "enabled": true,
      "mode": "managed",
      "headless": true,
      "defaultTimeout": 30
    }
  }
}
```

### 配置项说明

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `enabled` | bool | false | 是否启用浏览器工具 |
| `mode` | string | "managed" | 模式：managed(自动启动浏览器) 或 connect(连接已有浏览器) |
| `headless` | bool | true | 无头模式（不显示浏览器窗口） |
| `browserPath` | string | 自动检测 | 浏览器可执行文件路径 |
| `remoteUrl` | string | - | 远程 CDP URL（connect 模式） |
| `profileDir` | string | - | 浏览器 Profile 目录（保持登录状态） |
| `defaultTimeout` | int | 30 | 默认超时时间（秒） |
| `screenshotDir` | string | workspace/screenshots | 截图保存目录 |
| `args` | []string | - | 额外浏览器启动参数 |

## 支持的操作

### 页面导航

| Action | 参数 | 说明 |
|--------|------|------|
| `navigate` | url | 导航到指定 URL |
| `go_back` | - | 后退 |
| `go_forward` | - | 前进 |
| `refresh` | - | 刷新页面 |
| `get_url` | - | 获取当前页面 URL |
| `get_title` | - | 获取页面标题 |

### 元素操作

| Action | 参数 | 说明 |
|--------|------|------|
| `click` | selector | 点击元素 |
| `type` | selector, text | 输入文本（追加） |
| `type_clear` | selector, text | 清空后输入文本 |
| `set_value` | selector, value | 设置表单值 |
| `select_option` | selector, value | 选择下拉选项 |
| `press` | key | 按键（Enter, Tab, Escape 等） |
| `upload` | selector, file_path | 上传文件 |

### 获取信息

| Action | 参数 | 说明 |
|--------|------|------|
| `get_text` | selector | 获取元素文本内容 |
| `get_html` | selector | 获取元素 HTML |
| `get_attribute` | selector, attribute | 获取元素属性 |
| `is_visible` | selector | 检查元素是否可见 |

### 等待与截图

| Action | 参数 | 说明 |
|--------|------|------|
| `wait` | selector, timeout | 等待元素出现 |
| `wait_hidden` | selector, timeout | 等待元素消失 |
| `screenshot` | full_page, selector | 截图（全页或指定元素） |
| `scroll` | direction, amount | 滚动页面 |

### JavaScript

| Action | 参数 | 说明 |
|--------|------|------|
| `evaluate` | script | 执行 JavaScript |

### 多页面管理

| Action | 参数 | 说明 |
|--------|------|------|
| `new_page` | - | 创建新标签页 |
| `switch_page` | page_id | 切换到指定页面 |
| `list_pages` | - | 列出所有页面 |
| `close_page` | page_id | 关闭指定页面 |

## 使用示例

### 示例 1：打开网页并截图

```
browser --action navigate --url https://example.com
browser --action screenshot --full_page true
```

### 示例 2：表单填写

```
browser --action navigate --url https://example.com/login
browser --action type --selector "#username" --text "user@example.com"
browser --action type --selector "#password" --text "mypassword"
browser --action click --selector "button[type=submit]"
```

### 示例 3：获取页面内容

```
browser --action navigate --url https://example.com/article
browser --action get_text --selector ".article-content"
```

### 示例 4：执行 JavaScript

```
browser --action evaluate --script "document.querySelector('h1').innerText"
```

### 示例 5：等待动态内容

```
browser --action navigate --url https://example.com
browser --action wait --selector ".dynamic-content" --timeout 10
browser --action get_text --selector ".dynamic-content"
```

### 示例 6：多标签页操作

```
browser --action navigate --url https://example.com
browser --action new_page
browser --action switch_page --page_id 1
browser --action navigate --url https://google.com
browser --action list_pages
```

## 选择器语法

使用标准 CSS 选择器：

- `#id` - ID 选择器
- `.class` - 类选择器
- `tag` - 标签选择器
- `[attr="value"]` - 属性选择器
- `parent > child` - 子元素选择器
- `ancestor descendant` - 后代选择器

## 注意事项

1. **首次使用**：首次调用时会自动启动浏览器实例，可能需要几秒钟
2. **超时处理**：长时间操作建议设置 `timeout` 参数
3. **截图保存**：截图默认保存到 `~/.lingguard/workspace/screenshots/`
4. **资源清理**：工具会在应用退出时自动关闭浏览器
5. **安全性**：此工具标记为危险工具，需要显式启用配置

## 常见问题

### Q: 浏览器启动失败？

检查系统是否安装了 Chrome、Chromium 或 Edge 浏览器。可以通过 `browserPath` 指定浏览器路径。

### Q: 元素找不到？

- 确认选择器是否正确
- 使用 `wait` 等待元素加载
- 检查是否需要滚动页面

### Q: 如何保持登录状态？

设置 `profileDir` 配置项，浏览器会使用持久化的 Profile：

```json
{
  "tools": {
    "browser": {
      "enabled": true,
      "profileDir": "~/.lingguard/browser-profile"
    }
  }
}
```

### Q: 如何连接远程浏览器？

使用 `connect` 模式：

```json
{
  "tools": {
    "browser": {
      "enabled": true,
      "mode": "connect",
      "remoteUrl": "http://localhost:9222"
    }
  }
}
```

远程浏览器需要以调试模式启动：
```bash
chrome --remote-debugging-port=9222
```
