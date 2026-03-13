# LingGuard Browser 功能增强设计

> 从 OpenClaw 借鉴的 browser 功能改进建议

## 概述

本文档记录 LingGuard 从 OpenClaw 的 browser 机制中借鉴的功能增强建议，分为三个优先级：

- **Phase 1**: 高优先级，快速见效
- **Phase 2**: 中优先级，体验提升
- **Phase 3**: 锦上添花，可选功能

---

## 功能对比表

| 功能 | 当前 LingGuard | OpenClaw | 建议优先级 |
|------|---------------|----------|-----------|
| 基础操作 (navigate/click/type/screenshot) | ✅ | ✅ | - |
| 多标签页管理 | ✅ | ✅ | - |
| **AI 友好快照 (snapshot)** | ❌ | ✅ | **Phase 1** |
| **Ref 引用系统** | ❌ | ✅ | **Phase 1** |
| **Profile 持久化** | 部分 | ✅ | **Phase 1** |
| 对话框自动处理 | ❌ | ✅ | Phase 2 |
| 表单批量填充 | ❌ | ✅ | Phase 2 |
| PDF 导出 | ❌ | ✅ | Phase 2 |
| 智能等待机制 | 基础 | ✅ | Phase 2 |
| **文件选择器预置** | ❌ | ✅ | **Phase 2** ⚡新增 |
| **下载管理** | ❌ | ✅ | **Phase 2** ⚡新增 |
| **视口调整** | ❌ | ✅ | **Phase 2** ⚡新增 |
| **标签截图 (带 ref 标注)** | ❌ | ✅ | **Phase 2** ⚡新增 |
| Chrome 扩展中继 | ❌ | ✅ | Phase 3 |
| 拖拽操作 | ❌ | ✅ | Phase 3 |
| 控制台日志 | ❌ | ✅ | Phase 3 |
| 网络请求监控 | ❌ | ✅ | Phase 3 |
| **SSRF 防护** | ❌ | ✅ | **Phase 3** ⚡新增 |

> ⚡ 标记为本次对比分析新增的功能

---

## Phase 1: 高优先级功能

### 1.1 AI 友好页面快照 (Snapshot)

**问题**: 当前 LingGuard 返回原始 HTML，LLM 难以理解和定位元素。

**解决方案**: 基于 ARIA 角色的结构化快照，专为 AI 优化。

#### 数据结构

```go
// pkg/browser/snapshot.go

// SnapshotNode 快照节点
type SnapshotNode struct {
    Ref      string          `json:"ref"`      // 短引用 ID: "e1", "e2"
    Role     string          `json:"role"`     // button, link, textbox, etc.
    Name     string          `json:"name"`     // 可访问性名称
    Value    string          `json:"value,omitempty"`
    Children []*SnapshotNode `json:"children,omitempty"`
}

// SnapshotResult 快照结果
type SnapshotResult struct {
    Format    string          `json:"format"`    // "aria" | "ai"
    TargetID  string          `json:"targetId"`
    URL       string          `json:"url"`
    Nodes     []*SnapshotNode `json:"nodes"`
    Refs      map[string]RefInfo `json:"refs,omitempty"`
}

// RefInfo 引用信息
type RefInfo struct {
    Role string `json:"role"`
    Name string `json:"name,omitempty"`
    Nth  int    `json:"nth,omitempty"`
}
```

#### 角色分类

```go
// 交互元素 - 可点击/可输入
var InteractiveRoles = map[string]bool{
    "button": true, "link": true, "textbox": true, "checkbox": true,
    "radio": true, "combobox": true, "listbox": true, "menuitem": true,
    "searchbox": true, "slider": true, "switch": true, "tab": true,
}

// 内容元素 - 包含文本信息
var ContentRoles = map[string]bool{
    "heading": true, "paragraph": true, "article": true, "cell": true, "listitem": true,
}
```

#### 实现代码

```go
// pkg/browser/snapshot.go

package browser

import (
    "context"
    "fmt"
    "strings"
    "github.com/chromedp/chromedp"
)

type SnapshotOptions struct {
    InteractiveOnly bool   // 仅返回交互元素
    MaxDepth        int    // 最大深度，默认 10
    Compact         bool   // 紧凑模式
}

func (p *Page) Snapshot(ctx context.Context, opts SnapshotOptions) (*SnapshotResult, error) {
    if opts.MaxDepth == 0 {
        opts.MaxDepth = 10
    }

    script := fmt.Sprintf(`
        (function() {
            const interactiveOnly = %v;
            const maxDepth = %d;
            const interactiveRoles = new Set(['button', 'link', 'textbox', 'checkbox', 'radio', 'combobox', 'listbox', 'menuitem', 'searchbox', 'slider', 'switch', 'tab']);

            let refCounter = 0;
            const nodes = [];
            const refMap = {};

            function getRole(el) {
                const role = el.getAttribute('role');
                if (role) return role;
                const tag = el.tagName.toLowerCase();
                const roleMap = {
                    'button': 'button', 'a': 'link',
                    'input': el.type === 'checkbox' ? 'checkbox' : el.type === 'radio' ? 'radio' : 'textbox',
                    'textarea': 'textbox', 'select': 'combobox', 'img': 'img',
                    'h1': 'heading', 'h2': 'heading', 'h3': 'heading',
                    'p': 'paragraph', 'li': 'listitem', 'td': 'cell',
                };
                return roleMap[tag] || 'generic';
            }

            function getName(el) {
                let name = el.getAttribute('aria-label') || el.getAttribute('title') ||
                          el.getAttribute('alt') || el.getAttribute('placeholder');
                if (!name && el.innerText) {
                    name = el.innerText.trim().slice(0, 100);
                }
                if (!name && el.value) {
                    name = el.value.slice(0, 50);
                }
                return name || '';
            }

            function walk(el, depth) {
                if (!el || depth > maxDepth || el.hidden) return null;

                const role = getRole(el);
                const isInteractive = interactiveRoles.has(role);

                if (interactiveOnly && !isInteractive) {
                    const children = [];
                    for (const child of el.children) {
                        const result = walk(child, depth + 1);
                        if (result) children.push(...(Array.isArray(result) ? result : [result]));
                    }
                    return children.length > 0 ? children : null;
                }

                if (role === 'generic') {
                    const children = [];
                    for (const child of el.children) {
                        const result = walk(child, depth + 1);
                        if (result) children.push(...(Array.isArray(result) ? result : [result]));
                    }
                    return children.length > 0 ? children : null;
                }

                const ref = 'e' + (++refCounter);
                const node = { ref, role, name: getName(el) };
                refMap[ref] = { role, name: node.name };
                nodes.push(node);

                return node;
            }

            walk(document.body, 0);
            return { format: 'aria', nodes, refMap };
        })();
    `, opts.InteractiveOnly, opts.MaxDepth)

    var result SnapshotResult
    err := chromedp.Run(p.baseCtx, chromedp.Evaluate(script, &result))
    if err != nil {
        return nil, fmt.Errorf("snapshot: %w", err)
    }

    return &result, nil
}

// 格式化为 AI 友好的文本
func (r *SnapshotResult) ToText() string {
    var sb strings.Builder
    for _, node := range r.Nodes {
        sb.WriteString(fmt.Sprintf("[%s] %s", node.Ref, node.Role))
        if node.Name != "" {
            sb.WriteString(fmt.Sprintf(" \"%s\"", node.Name))
        }
        sb.WriteString("\n")
    }
    return sb.String()
}
```

#### 输出示例

```
URL: https://example.com/login

[e1] heading "用户登录"
[e2] textbox "用户名"
[e3] textbox "密码"
[e4] checkbox "记住我"
[e5] button "登录"
[e6] link "忘记密码?"
```

---

### 1.2 Ref 引用系统

**问题**: 每次操作都要写完整的 CSS 选择器，冗长且不稳定。

**解决方案**: 先快照获取 ref，后续操作使用短 ref。

```go
// internal/tools/browser.go

type BrowserTool struct {
    // ...existing fields...
    refMap   map[string]string  // ref -> CSS selector 映射
    refMapMu sync.RWMutex
}

// 支持使用 ref 或 selector
func (t *BrowserTool) resolveSelector(ref, selector string) (string, error) {
    if ref != "" {
        t.refMapMu.RLock()
        s, ok := t.refMap[ref]
        t.refMapMu.RUnlock()
        if !ok {
            return "", fmt.Errorf("ref not found: %s (run snapshot first)", ref)
        }
        return s, nil
    }
    return selector, nil
}

// 修改 click 支持 ref
func (t *BrowserTool) click(ctx context.Context, ref, selector string, timeout time.Duration) (string, error) {
    sel, err := t.resolveSelector(ref, selector)
    if err != nil {
        return "", err
    }
    // ... 原有逻辑
}
```

#### 使用示例

```bash
# 1. 获取快照
browser --action snapshot

# 2. 使用 ref 操作
browser --action click --ref e1
browser --action type --ref e2 --text "hello"
```

---

### 1.3 Profile 持久化

**问题**: 每次重启浏览器登录状态丢失。

**解决方案**: 支持 Profile 持久化和多 Profile 切换。

#### 配置结构

```go
type BrowserProfileConfig struct {
    Name       string `json:"name"`
    ProfileDir string `json:"profileDir"`
    Color      string `json:"color,omitempty"`
}

type BrowserConfig struct {
    // ...existing fields...
    DefaultProfile string                          `json:"defaultProfile"`
    Profiles       map[string]BrowserProfileConfig `json:"profiles"`
}
```

#### 配置示例

```json
{
  "tools": {
    "browser": {
      "enabled": true,
      "defaultProfile": "default",
      "profiles": {
        "default": {
          "name": "默认",
          "profileDir": "~/.lingguard/browser/default"
        },
        "work": {
          "name": "工作",
          "profileDir": "~/.lingguard/browser/work"
        },
        "shopping": {
          "name": "购物",
          "profileDir": "~/.lingguard/browser/shopping"
        }
      }
    }
  }
}
```

---

## Phase 2: 中优先级功能

### 2.1 对话框自动处理 (dialog)

**问题**: alert/confirm/prompt 弹窗会阻塞页面。

```go
// pkg/browser/dialog.go

type DialogAction struct {
    Accept     bool   `json:"accept"`      // true=确定, false=取消
    PromptText string `json:"promptText"`  // prompt 输入文本
}

func (p *Page) ArmDialog(ctx context.Context, action DialogAction, timeout time.Duration) error {
    script := fmt.Sprintf(`
        window.__lingguardDialogHandler = %v;
        window.__lingguardDialogPromptText = %q;

        window.alert = function(msg) { console.log('[LingGuard] alert blocked:', msg); };
        window.confirm = function(msg) { return window.__lingguardDialogHandler; };
        window.prompt = function(msg, defaultText) {
            return window.__lingguardDialogHandler ? window.__lingguardDialogPromptText : null;
        };
    `, action.Accept, action.PromptText)

    return chromedp.Run(p.baseCtx, chromedp.Evaluate(script, nil))
}
```

#### 使用示例

```bash
# 预设所有对话框点击"确定"
browser --action arm_dialog --accept true

# 预设 prompt 输入
browser --action arm_dialog --accept true --prompt_text "hello"
```

---

### 2.2 表单批量填充 (fill)

**问题**: 多字段表单需要多次调用 type。

```go
// pkg/browser/form.go

type FormField struct {
    Ref    string `json:"ref"`
    Value  string `json:"value"`
    Clear  bool   `json:"clear"`
}

func (p *Page) FillForm(ctx context.Context, fields []FormField, timeout time.Duration) error {
    for _, field := range fields {
        selector := p.refToSelector(field.Ref)
        actions := []chromedp.Action{chromedp.WaitVisible(selector), chromedp.Focus(selector)}
        if field.Clear {
            actions = append(actions, chromedp.Clear(selector))
        }
        actions = append(actions, chromedp.SendKeys(selector, field.Value))
        if err := chromedp.Run(p.baseCtx, actions...); err != nil {
            return fmt.Errorf("fill field %s: %w", field.Ref, err)
        }
    }
    return nil
}
```

#### 使用示例

```bash
browser --action fill --fields '[
    {"ref": "e1", "value": "user@example.com", "clear": true},
    {"ref": "e2", "value": "mypassword", "clear": true}
]'
```

---

### 2.3 PDF 导出

```go
// pkg/browser/pdf.go

func (p *Page) PrintToPDF(ctx context.Context, opts PDFOptions) (string, error) {
    var buf []byte

    action := chromedp.ActionFunc(func(ctx context.Context) error {
        result, err := page.PrintToPDF(&page.PrintToPDFArgs{
            PrintBackground: opts.PrintBackground,
            PaperWidth:      opts.PaperWidth,
            PaperHeight:     opts.PaperHeight,
        }).Do(ctx)
        if err != nil {
            return err
        }
        buf = result.Data
        return nil
    })

    if err := chromedp.Run(p.baseCtx, action); err != nil {
        return "", err
    }

    filename := fmt.Sprintf("page_%s.pdf", time.Now().Format("20060102_150405"))
    filepath := filepath.Join(p.manager.GetConfig().ScreenshotDir, filename)
    os.WriteFile(filepath, buf, 0644)
    return filepath, nil
}
```

---

### 2.4 智能等待机制

```go
// pkg/browser/wait.go

type WaitCondition string

const (
    WaitVisible      WaitCondition = "visible"
    WaitHidden       WaitCondition = "hidden"
    WaitNetworkIdle  WaitCondition = "networkIdle"
    WaitURL          WaitCondition = "url"
    WaitText         WaitCondition = "text"
)

type WaitOptions struct {
    Condition WaitCondition
    Selector  string
    URL       string
    Text      string
    Timeout   time.Duration
}

func (p *Page) WaitFor(ctx context.Context, opts WaitOptions) error {
    switch opts.Condition {
    case WaitVisible:
        return chromedp.Run(p.baseCtx, chromedp.WaitVisible(opts.Selector))
    case WaitHidden:
        return chromedp.Run(p.baseCtx, chromedp.WaitNotVisible(opts.Selector))
    case WaitNetworkIdle:
        return p.waitForNetworkIdle(ctx, opts.Timeout)
    case WaitURL:
        return p.waitForURL(ctx, opts.URL, opts.Timeout)
    }
    return nil
}
```

---

### 2.5 文件选择器预置 (arm_file_chooser) 🆕

**问题**: 文件上传对话框需要手动处理，无法自动化。

**解决方案**: 预置文件选择器响应，当页面触发文件选择时自动填充预设文件。

```go
// pkg/browser/filechooser.go

type FileChooserOptions struct {
    Paths   []string      `json:"paths"`     // 要上传的文件路径列表
    Timeout time.Duration `json:"timeout"`   // 等待超时
}

// ArmFileChooser 预置文件选择器响应
func (p *Page) ArmFileChooser(ctx context.Context, opts FileChooserOptions) error {
    // 使用 CDP 的 Page.setInterceptFileChooser
    // 当文件选择对话框出现时自动填充预设文件
    return nil
}
```

#### 使用示例

```bash
# 预置文件选择器，当用户点击上传按钮时自动填充文件
browser --action arm_file_chooser --paths '["/path/to/file1.pdf", "/path/to/file2.jpg"]'
```

---

### 2.6 下载管理 (download) 🆕

**问题**: 无法处理文件下载。

**解决方案**: 等待下载完成并返回下载结果。

```go
// pkg/browser/download.go

type DownloadResult struct {
    URL               string `json:"url"`               // 下载 URL
    SuggestedFilename string `json:"suggestedFilename"` // 建议的文件名
    Path              string `json:"path"`              // 保存路径
}

// WaitForDownload 等待下载完成
func (p *Page) WaitForDownload(ctx context.Context, savePath string, timeout time.Duration) (*DownloadResult, error) {
    // 监听 Page.downloadWillBegin 和 Page.downloadProgress 事件
    // 使用 CDP 的 Browser.setDownloadBehavior 控制下载行为
    return nil, nil
}

// DownloadViaClick 点击链接并等待下载
func (p *Page) DownloadViaClick(ctx context.Context, ref, savePath string, timeout time.Duration) (*DownloadResult, error) {
    // 1. 先设置下载行为
    // 2. 点击下载链接
    // 3. 等待下载完成
    return nil, nil
}
```

#### 使用示例

```bash
# 等待下载完成
browser --action wait_for_download --path "/downloads/"

# 点击链接并下载
browser --action download --ref e5 --path "/downloads/report.pdf"
```

---

### 2.7 视口调整 (viewport) 🆕

**问题**: 无法调整浏览器窗口大小，影响响应式页面测试。

```go
// pkg/browser/viewport.go

type ViewportSize struct {
    Width  int `json:"width"`
    Height int `json:"height"`
}

// SetViewportSize 设置视口大小
func (p *Page) SetViewportSize(ctx context.Context, width, height int) error {
    // 使用 CDP 的 Emulation.setDeviceMetricsOverride
    action := chromedp.ActionFunc(func(ctx context.Context) error {
        _, err := page.SetDeviceMetricsOverride(
            &page.SetDeviceMetricsOverrideArgs{
                Width:  width,
                Height: height,
                Scale:  1,
            },
        ).Do(ctx)
        return err
    })
    return chromedp.Run(p.baseCtx, action)
}

// GetViewportSize 获取当前视口大小
func (p *Page) GetViewportSize(ctx context.Context) (*ViewportSize, error) {
    var size ViewportSize
    script := `
        (function() {
            return {
                width: window.innerWidth,
                height: window.innerHeight
            };
        })();
    `
    err := chromedp.Run(p.baseCtx, chromedp.Evaluate(script, &size))
    return &size, err
}
```

#### 使用示例

```bash
# 设置视口大小为 1920x1080
browser --action set_viewport --width 1920 --height 1080

# 获取当前视口大小
browser --action get_viewport
```

---

### 2.8 标签截图 (screenshot_with_labels) 🆕 ⭐ 推荐

**问题**: LLM 难以理解截图中的元素对应关系。

**解决方案**: 在截图上绘制 ref 标签，帮助 LLM 定位元素。

```go
// pkg/browser/screenshot_labels.go

type ScreenshotWithLabelsOptions struct {
    Refs      map[string]RefInfo `json:"refs"`      // ref -> 元素信息
    MaxLabels int                `json:"maxLabels"`  // 最大标签数，默认 150
}

type ScreenshotWithLabelsResult struct {
    Buffer  []byte `json:"buffer"`   // 图片数据
    Labels  int    `json:"labels"`   // 实际标注数量
    Skipped int    `json:"skipped"`  // 跳过的元素数
}

// ScreenshotWithLabels 截图并绘制元素标签
func (p *Page) ScreenshotWithLabels(ctx context.Context, opts ScreenshotWithLabelsOptions) (*ScreenshotWithLabelsResult, error) {
    // 1. 获取视口信息
    // 2. 遍历 refs，获取每个元素的边界框
    // 3. 在页面上绘制标签 overlay（橙色边框 + ref 标签）
    // 4. 截图
    // 5. 移除 overlay
    return nil, nil
}
```

#### 输出示例

截图上会显示：
- 橙色边框标注每个元素
- 元素上方显示 ref 标签（如 `e1`, `e2`）

#### 使用示例

```bash
# 先获取快照
browser --action snapshot

# 截图并标注元素
browser --action screenshot_with_labels --max_labels 50
```

---

## Phase 3: Chrome 扩展中继

让 LingGuard 控制用户已打开的 Chrome 标签页，复用登录状态。

### 工作原理

```
┌──────────────────────────────────────────────────────────────────┐
│                        用户浏览器 (已打开)                         │
│  ┌─────────────────┐                                              │
│  │ Tab: 淘宝购物车   │ ← 已登录，有 cookies                         │
│  │ Tab: GitHub      │ ← 已登录                                    │
│  └─────────────────┘                                              │
│         ↑                                                          │
│    chrome.debugger API                                            │
│         │                                                          │
│  ┌──────┴──────────┐                                              │
│  │ Chrome Extension │ ← WebSocket 连接到 LingGuard               │
│  │ (LingGuard Relay)│                                             │
│  └─────────────────┘                                              │
└──────────────────────────────────────────────────────────────────┘
          │
          │ WebSocket (ws://127.0.0.1:PORT/extension)
          ↓
┌──────────────────────────────────────────────────────────────────┐
│                      LingGuard (Relay Server)                     │
│  ┌─────────────────────────────────────────────────────────────┐ │
│  │ 模拟 CDP 服务端                                               │ │
│  │ - /json/version → 返回 webSocketDebuggerUrl                  │ │
│  │ - /json/list    → 返回已连接的标签页列表                       │ │
│  │ - /cdp (WS)     → 接收 CDP 命令，转发给扩展                    │ │
│  └─────────────────────────────────────────────────────────────┘ │
│         ↑                                                          │
│    chromedp 连接 (以为连的是真实浏览器)                            │
└──────────────────────────────────────────────────────────────────┘
```

### 核心流程

1. **Chrome 扩展** 使用 `chrome.debugger` API 附加到标签页
2. **扩展** 通过 WebSocket 连接到 LingGuard 的 Relay Server
3. **Relay Server** 模拟一个 CDP 服务端
4. **chromedp** 连接到 Relay Server
5. **所有 CDP 命令** 通过扩展转发到真实的 Chrome 标签页

### 组件 1: Chrome 扩展 (manifest.json)

```json
{
  "manifest_version": 3,
  "name": "LingGuard Browser Relay",
  "version": "1.0.0",
  "description": "Allow LingGuard to control your browser tabs",
  "permissions": ["debugger", "activeTab", "tabs", "storage"],
  "background": {
    "service_worker": "background.js"
  },
  "action": {
    "default_icon": {
      "16": "icons/icon16.png",
      "48": "icons/icon48.png",
      "128": "icons/icon128.png"
    },
    "default_title": "LingGuard Relay"
  }
}
```

### 组件 2: 扩展后台脚本 (background.js)

```javascript
// LingGuard Browser Relay - Background Script

const RELAY_SERVER_URL = "ws://127.0.0.1:9092/extension";
let ws = null;
let attachedTabs = new Map();

// 用户点击扩展图标
chrome.action.onClicked.addListener(async (tab) => {
  if (ws && ws.readyState === WebSocket.OPEN) {
    if (attachedTabs.has(tab.id)) {
      await detachTab(tab.id);
    } else {
      await attachTab(tab);
    }
  } else {
    await connectRelay();
    await attachTab(tab);
  }
});

// 连接到 Relay Server
async function connectRelay() {
  return new Promise((resolve, reject) => {
    chrome.storage.local.get(['relayToken'], (result) => {
      const token = result.relayToken || '';
      const url = token ? `${RELAY_SERVER_URL}?token=${token}` : RELAY_SERVER_URL;
      ws = new WebSocket(url);

      ws.onopen = () => {
        console.log('Connected to LingGuard Relay');
        updateBadge(true);
        resolve();
      };

      ws.onmessage = handleRelayMessage;
      ws.onclose = () => { updateBadge(false); setTimeout(() => connectRelay().catch(() => {}), 5000); };
    });
  });
}

// 处理来自 Relay Server 的消息
async function handleRelayMessage(event) {
  const msg = JSON.parse(event.data);

  if (msg.method === 'ping') {
    ws.send(JSON.stringify({ method: 'pong' }));
    return;
  }

  if (msg.method === 'forwardCDPCommand') {
    const { id, params } = msg;
    let tabId = null;
    for (const [tid, info] of attachedTabs) {
      if (info.sessionId === params.sessionId) { tabId = tid; break; }
    }

    if (!tabId) {
      ws.send(JSON.stringify({ id, error: 'Session not found' }));
      return;
    }

    try {
      const result = await chrome.debugger.sendCommand({ tabId }, params.method, params.params);
      ws.send(JSON.stringify({ id, result }));
    } catch (err) {
      ws.send(JSON.stringify({ id, error: err.message }));
    }
  }
}

// 附加到标签页
async function attachTab(tab) {
  await chrome.debugger.attach({ tabId: tab.id }, '1.3');

  const sessionId = 'session_' + Math.random().toString(36).substr(2, 9);
  const targetInfo = { targetId: String(tab.id), type: 'page', title: tab.title, url: tab.url, attached: true };
  attachedTabs.set(tab.id, { sessionId, targetInfo });

  if (ws && ws.readyState === WebSocket.OPEN) {
    ws.send(JSON.stringify({
      method: 'forwardCDPEvent',
      params: { method: 'Target.attachedToTarget', params: { sessionId, targetInfo, waitingForDebugger: false } }
    }));
  }

  chrome.action.setBadgeText({ text: 'ON', tabId: tab.id });
  chrome.action.setBadgeBackgroundColor({ color: '#4CAF50' });
}

// 分离标签页
async function detachTab(tabId) {
  const info = attachedTabs.get(tabId);
  if (info) {
    await chrome.debugger.detach({ tabId });
    attachedTabs.delete(tabId);
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({
        method: 'forwardCDPEvent',
        params: { method: 'Target.detachedFromTarget', params: { sessionId: info.sessionId } }
      }));
    }
    chrome.action.setBadgeText({ text: '', tabId });
  }
}

// 监听 debugger 事件
chrome.debugger.onEvent.addListener((source, method, params) => {
  if (!ws || ws.readyState !== WebSocket.OPEN) return;
  const info = attachedTabs.get(source.tabId);
  if (!info) return;
  ws.send(JSON.stringify({ method: 'forwardCDPEvent', params: { method, params, sessionId: info.sessionId } }));
});

function updateBadge(connected) {
  chrome.action.setBadgeText({ text: connected ? '●' : '○' });
  chrome.action.setBadgeBackgroundColor({ color: connected ? '#4CAF50' : '#9E9E9E' });
}
```

### 组件 3: Go Relay Server (pkg/browser/relay.go)

```go
// Package browser 提供浏览器自动化功能
package browser

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "sync"
    "time"

    "github.com/gorilla/websocket"
)

// ExtensionRelay Chrome 扩展中继服务器
type ExtensionRelay struct {
    port          int
    authToken     string
    server        *http.Server
    extensionConn *websocket.Conn
    cdpClients    map[*websocket.Conn]bool
    targets       map[string]*TargetInfo
    pendingReqs   map[int]chan *ExtensionResponse
    nextReqID     int
    mu            sync.RWMutex
}

// TargetInfo 标签页信息
type TargetInfo struct {
    TargetID  string `json:"targetId"`
    Type      string `json:"type"`
    Title     string `json:"title"`
    URL       string `json:"url"`
    Attached  bool   `json:"attached"`
    SessionID string `json:"sessionId,omitempty"`
}

// CDPCommand CDP 命令
type CDPCommand struct {
    ID        int             `json:"id"`
    Method    string          `json:"method"`
    Params    json.RawMessage `json:"params,omitempty"`
    SessionID string          `json:"sessionId,omitempty"`
}

// CDPResponse CDP 响应
type CDPResponse struct {
    ID        int             `json:"id"`
    Result    json.RawMessage `json:"result,omitempty"`
    Error     *CDPError       `json:"error,omitempty"`
    SessionID string          `json:"sessionId,omitempty"`
}

type CDPError struct { Message string `json:"message"` }

type ExtensionResponse struct {
    ID     int             `json:"id"`
    Result json.RawMessage `json:"result,omitempty"`
    Error  string          `json:"error,omitempty"`
}

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

// NewExtensionRelay 创建扩展中继服务器
func NewExtensionRelay(port int, authToken string) *ExtensionRelay {
    return &ExtensionRelay{
        port:        port,
        authToken:   authToken,
        cdpClients:  make(map[*websocket.Conn]bool),
        targets:     make(map[string]*TargetInfo),
        pendingReqs: make(map[int]chan *ExtensionResponse),
    }
}

// Start 启动服务器
func (r *ExtensionRelay) Start(ctx context.Context) error {
    mux := http.NewServeMux()
    mux.HandleFunc("/json/version", r.handleJSONVersion)
    mux.HandleFunc("/json", r.handleJSONList)
    mux.HandleFunc("/json/list", r.handleJSONList)
    mux.HandleFunc("/extension", r.handleExtensionWS)
    mux.HandleFunc("/cdp", r.handleCDPWS)
    mux.HandleFunc("/extension/status", r.handleExtensionStatus)

    r.server = &http.Server{Addr: fmt.Sprintf("127.0.0.1:%d", r.port), Handler: mux}
    return r.server.ListenAndServe()
}

// Stop 停止服务器
func (r *ExtensionRelay) Stop() error {
    if r.server != nil { return r.server.Close() }
    return nil
}

func (r *ExtensionRelay) handleJSONVersion(w http.ResponseWriter, req *http.Request) {
    if !r.checkAuth(w, req) { return }
    response := map[string]interface{}{
        "Browser": "LingGuard/extension-relay",
        "Protocol-Version": "1.3",
        "webSocketDebuggerUrl": fmt.Sprintf("ws://127.0.0.1:%d/cdp", r.port),
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

func (r *ExtensionRelay) handleJSONList(w http.ResponseWriter, req *http.Request) {
    if !r.checkAuth(w, req) { return }
    r.mu.RLock()
    targets := make([]map[string]interface{}, 0, len(r.targets))
    for _, t := range r.targets {
        targets = append(targets, map[string]interface{}{
            "id": t.TargetID, "type": t.Type, "title": t.Title, "url": t.URL,
            "webSocketDebuggerUrl": fmt.Sprintf("ws://127.0.0.1:%d/cdp", r.port),
        })
    }
    r.mu.RUnlock()
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(targets)
}

func (r *ExtensionRelay) handleExtensionStatus(w http.ResponseWriter, req *http.Request) {
    r.mu.RLock()
    connected := r.extensionConn != nil
    targetCount := len(r.targets)
    r.mu.RUnlock()
    json.NewEncoder(w).Encode(map[string]interface{}{"connected": connected, "targets": targetCount})
}

func (r *ExtensionRelay) handleExtensionWS(w http.ResponseWriter, req *http.Request) {
    if !r.checkAuth(w, req) { return }
    conn, err := upgrader.Upgrade(w, req, nil)
    if err != nil { return }

    r.mu.Lock()
    if r.extensionConn != nil { r.extensionConn.Close() }
    r.extensionConn = conn
    r.mu.Unlock()

    // Ping 保持连接
    go func() {
        ticker := time.NewTicker(5 * time.Second)
        defer ticker.Stop()
        for range ticker.C {
            r.mu.RLock()
            c := r.extensionConn
            r.mu.RUnlock()
            if c == nil { return }
            c.WriteJSON(map[string]string{"method": "ping"})
        }
    }()

    // 读取消息
    for {
        _, msg, err := conn.ReadMessage()
        if err != nil {
            r.mu.Lock()
            r.extensionConn = nil
            r.mu.Unlock()
            return
        }

        var extMsg struct {
            ID     int             `json:"id"`
            Method string          `json:"method"`
            Params json.RawMessage `json:"params"`
            Result json.RawMessage `json:"result"`
            Error  string          `json:"error"`
        }
        json.Unmarshal(msg, &extMsg)

        if extMsg.Method == "pong" { continue }
        if extMsg.Method == "forwardCDPEvent" { r.handleForwardedEvent(&extMsg); continue }
        if extMsg.ID != 0 {
            r.mu.RLock()
            ch, ok := r.pendingReqs[extMsg.ID]
            r.mu.RUnlock()
            if ok { ch <- &ExtensionResponse{ID: extMsg.ID, Result: extMsg.Result, Error: extMsg.Error} }
        }
    }
}

func (r *ExtensionRelay) handleCDPWS(w http.ResponseWriter, req *http.Request) {
    if !r.checkAuth(w, req) { return }
    conn, err := upgrader.Upgrade(w, req, nil)
    if err != nil { return }

    r.mu.Lock()
    r.cdpClients[conn] = true
    r.mu.Unlock()

    defer func() {
        r.mu.Lock()
        delete(r.cdpClients, conn)
        r.mu.Unlock()
        conn.Close()
    }()

    for {
        _, msg, err := conn.ReadMessage()
        if err != nil { return }
        var cmd CDPCommand
        json.Unmarshal(msg, &cmd)
        response := r.handleCDPCommand(&cmd)
        conn.WriteJSON(response)
    }
}

func (r *ExtensionRelay) handleCDPCommand(cmd *CDPCommand) *CDPResponse {
    switch cmd.Method {
    case "Browser.getVersion":
        return &CDPResponse{ID: cmd.ID, Result: json.RawMessage(`{"protocolVersion":"1.3","product":"LingGuard"}`)}
    case "Target.setDiscoverTargets", "Target.setAutoAttach":
        return &CDPResponse{ID: cmd.ID, Result: json.RawMessage(`{}`)}
    case "Target.getTargets":
        r.mu.RLock()
        targets := make([]map[string]interface{}, 0)
        for _, t := range r.targets {
            targets = append(targets, map[string]interface{}{"targetId": t.TargetID, "type": t.Type, "title": t.Title, "url": t.URL, "attached": t.Attached})
        }
        r.mu.RUnlock()
        result, _ := json.Marshal(map[string]interface{}{"targetInfos": targets})
        return &CDPResponse{ID: cmd.ID, Result: result}
    }

    r.mu.RLock()
    extConn := r.extensionConn
    r.mu.RUnlock()

    if extConn == nil {
        return &CDPResponse{ID: cmd.ID, Error: &CDPError{Message: "Extension not connected"}}
    }

    r.mu.Lock()
    reqID := r.nextReqID
    r.nextReqID++
    respCh := make(chan *ExtensionResponse, 1)
    r.pendingReqs[reqID] = respCh
    r.mu.Unlock()

    defer func() {
        r.mu.Lock()
        delete(r.pendingReqs, reqID)
        r.mu.Unlock()
    }()

    extConn.WriteJSON(map[string]interface{}{"id": reqID, "method": "forwardCDPCommand", "params": cmd})

    select {
    case resp := <-respCh:
        if resp.Error != "" {
            return &CDPResponse{ID: cmd.ID, Error: &CDPError{Message: resp.Error}}
        }
        return &CDPResponse{ID: cmd.ID, Result: resp.Result}
    case <-time.After(30 * time.Second):
        return &CDPResponse{ID: cmd.ID, Error: &CDPError{Message: "Timeout"}}
    }
}

func (r *ExtensionRelay) handleForwardedEvent(msg *ExtensionMessage) {
    var event struct {
        Method    string          `json:"method"`
        Params    json.RawMessage `json:"params"`
        SessionID string          `json:"sessionId"`
    }
    json.Unmarshal(msg.Params, &event)

    if event.Method == "Target.attachedToTarget" {
        var params struct {
            SessionID  string       `json:"sessionId"`
            TargetInfo *TargetInfo  `json:"targetInfo"`
        }
        if err := json.Unmarshal(event.Params, &params); err == nil && params.TargetInfo != nil {
            r.mu.Lock()
            params.TargetInfo.SessionID = params.SessionID
            r.targets[params.SessionID] = params.TargetInfo
            r.mu.Unlock()
        }
    }

    if event.Method == "Target.detachedFromTarget" {
        var params struct { SessionID string `json:"sessionId"` }
        if err := json.Unmarshal(event.Params, &params); err == nil {
            r.mu.Lock()
            delete(r.targets, params.SessionID)
            r.mu.Unlock()
        }
    }

    r.mu.RLock()
    clients := make([]*websocket.Conn, 0, len(r.cdpClients))
    for c := range r.cdpClients { clients = append(clients, c) }
    r.mu.RUnlock()

    for _, c := range clients { c.WriteMessage(websocket.TextMessage, msg) }
}

func (r *ExtensionRelay) checkAuth(w http.ResponseWriter, req *http.Request) bool {
    if r.authToken == "" { return true }
    token := req.Header.Get("X-LingGuard-Relay-Token")
    if token == "" { token = req.URL.Query().Get("token") }
    if token != r.authToken {
        w.WriteHeader(http.StatusUnauthorized)
        w.Write([]byte("Unauthorized"))
        return false
    }
    return true
}
```

### 配置示例

```json
{
  "tools": {
    "browser": {
      "enabled": true,
      "mode": "relay",
      "relayURL": "http://127.0.0.1:9092",
      "relayToken": "your-secret-token",
      "relayPort": 9092
    }
  }
}
```

### 使用流程

```bash
# 1. 安装 Chrome 扩展 (开发者模式加载)

# 2. 在 Chrome 中打开网页 (已登录)

# 3. 点击扩展图标附加到标签页 (显示 ON)

# 4. 向 LingGuard 发送指令
browser --action snapshot
browser --action click --ref "e1"
```

---

## Phase 3 (续): 其他增强功能

### 拖拽操作 (drag)

```go
func (p *Page) Drag(ctx context.Context, startRef, endRef string) error {
    return chromedp.Run(p.baseCtx,
        chromedp.MouseMoveNode(p.refToSelector(startRef)),
        chromedp.MouseDown(),
        chromedp.MouseMoveNode(p.refToSelector(endRef)),
        chromedp.MouseUp(),
    )
}
```

### 控制台日志

```go
type ConsoleMessage struct {
    Type    string `json:"type"`
    Message string `json:"message"`
    Line    int    `json:"line"`
}

func (p *Page) GetConsoleMessages(ctx context.Context, level string) ([]ConsoleMessage, error) {
    // 启用 Runtime 域，收集 console 消息
}
```

### 网络请求监控

```go
type NetworkRequest struct {
    URL    string `json:"url"`
    Method string `json:"method"`
    Status int    `json:"status"`
}

func (p *Page) GetRequests(ctx context.Context, filter string) ([]NetworkRequest, error) {
    // 启用 Network 域，收集请求
}
```

---

### 3.1 SSRF 防护 (ssrf_policy) 🆕

**问题**: 浏览器可能被利用访问内网敏感资源（如 AWS 元数据服务）。

**解决方案**: URL 白名单/黑名单过滤。

```go
// pkg/browser/ssrf.go

type SSRFPolicy struct {
    AllowPrivateNetwork    bool     `json:"allowPrivateNetwork"`    // 允许私有网络，默认 true
    AllowedHostnames       []string `json:"allowedHostnames"`       // 明确允许的主机名
    HostnameAllowlist      []string `json:"hostnameAllowlist"`      // 支持通配符 *.example.com
}

// CheckNavigationAllowed 检查 URL 是否允许导航
func (m *Manager) CheckNavigationAllowed(urlStr string, policy *SSRFPolicy) error {
    if policy == nil {
        policy = &SSRFPolicy{AllowPrivateNetwork: true}
    }

    parsed, err := url.Parse(urlStr)
    if err != nil {
        return fmt.Errorf("invalid URL: %w", err)
    }

    // 检查私有网络
    if !policy.AllowPrivateNetwork {
        if isPrivateIP(parsed.Hostname()) {
            return fmt.Errorf("private network access denied: %s", parsed.Hostname())
        }
    }

    // 检查主机名白名单
    if len(policy.HostnameAllowlist) > 0 {
        if !matchHostnamePattern(parsed.Hostname(), policy.HostnameAllowlist) {
            return fmt.Errorf("hostname not in allowlist: %s", parsed.Hostname())
        }
    }

    return nil
}

// isPrivateIP 检查是否为私有 IP
func isPrivateIP(hostname string) bool {
    ip := net.ParseIP(hostname)
    if ip == nil {
        return false // 不是 IP，可能是域名
    }
    // 检查 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16, 127.0.0.0/8, 169.254.0.0/16
    privateBlocks := []string{
        "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16",
        "127.0.0.0/8", "169.254.0.0/16", "::1/128",
    }
    for _, block := range privateBlocks {
        _, cidr, _ := net.ParseCIDR(block)
        if cidr != nil && cidr.Contains(ip) {
            return true
        }
    }
    return false
}

// matchHostnamePattern 匹配主机名模式（支持通配符）
func matchHostnamePattern(hostname string, patterns []string) bool {
    for _, pattern := range patterns {
        if strings.HasPrefix(pattern, "*.") {
            suffix := pattern[2:]
            if strings.HasSuffix(hostname, suffix) || hostname == suffix[1:] {
                return true
            }
        } else {
            if hostname == pattern {
                return true
            }
        }
    }
    return false
}
```

#### 配置示例

```json
{
  "tools": {
    "browser": {
      "enabled": true,
      "ssrfPolicy": {
        "allowPrivateNetwork": true,
        "allowedHostnames": ["localhost", "metadata.internal"],
        "hostnameAllowlist": ["*.example.com", "trusted.com"]
      }
    }
  }
}
```

---

## 实现计划

| 阶段 | 功能 | 时间 | 价值 | 备注 |
|------|------|------|------|------|
| **Phase 1.1** | AI 友好快照 | 2 天 | 高 | |
| **Phase 1.2** | Ref 引用系统 | 1 天 | 高 | |
| **Phase 1.3** | Profile 持久化 | 1 天 | 高 | |
| **Phase 2.1** | 对话框处理 | 1 天 | 中 | |
| **Phase 2.2** | 表单批量填充 | 1 天 | 中 | |
| **Phase 2.3** | PDF 导出 | 0.5 天 | 中 | |
| **Phase 2.4** | 智能等待 | 1 天 | 中 | |
| **Phase 2.5** | 文件选择器预置 | 0.5 天 | 中 | 🆕 |
| **Phase 2.6** | 下载管理 | 1 天 | 中 | 🆕 |
| **Phase 2.7** | 视口调整 | 0.5 天 | 低 | 🆕 |
| **Phase 2.8** | 标签截图 | 1 天 | **高** | 🆕 ⭐ 推荐优先 |
| **Phase 3.1** | Chrome 扩展中继 | 1-2 周 | 可选 | |
| **Phase 3.2** | SSRF 防护 | 1 天 | 中 | 🆕 安全相关 |
| **Phase 3.3** | 拖拽操作 | 0.5 天 | 低 | |
| **Phase 3.4** | 控制台日志 | 0.5 天 | 低 | |
| **Phase 3.5** | 网络请求监控 | 1 天 | 低 | |

---

## 与 OpenClaw 的最终差异

| 特性 | LingGuard (增强后) | OpenClaw | 备注 |
|------|-------------------|----------|------|
| **核心功能** | ✅ 完整 | ✅ 完整 | |
| **AI 优化** | ✅ 快照 + Ref + 标签截图 | ✅ 快照 + Ref + 标签截图 | |
| **Profile** | ✅ 多 Profile | ✅ 多 Profile | |
| **下载管理** | ✅ | ✅ | 🆕 |
| **文件上传** | ✅ 预置 | ✅ 预置 | 🆕 |
| **视口控制** | ✅ | ✅ | 🆕 |
| **Chrome 扩展中继** | ⚠️ 可选 | ✅ 内置 | 个人使用可选 |
| **SSRF 防护** | ⚠️ 可选 | ✅ | 🆕 安全功能 |
| **Docker 沙箱** | ❌ | ✅ | 多租户必需，个人不需要 |
| **Node 远程代理** | ❌ | ✅ | 企业级，个人不需要 |
| **noVNC** | ❌ | ✅ | 沙箱必需，个人不需要 |

**结论**: 实现上述功能后，LingGuard 对**个人助手**场景功能完善，同时可选配置 SSRF 防护提升安全性。

---

## 附录 A: LingGuard vs OpenClaw 架构深度对比

### A.1 技术栈对比

| 维度 | LingGuard | OpenClaw |
|------|-----------|----------|
| **语言** | Go | TypeScript |
| **底层库** | chromedp (直接 CDP) | Playwright (高级封装) |
| **元素定位** | CSS 选择器 | Role-based + Ref 系统 |
| **连接模式** | 本地启动 / 远程连接 | CDP 连接 (支持扩展中继) |

### A.2 架构对比

#### LingGuard 架构

```
┌─────────────────────────────────────────────────────────────┐
│                      LingGuard 架构                          │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│   BrowserTool (internal/tools/browser.go)                   │
│         │                                                   │
│         ▼                                                   │
│   Manager (pkg/browser/client.go)                          │
│         │                                                   │
│         ├── 启动本地 Chrome (chromedp.NewExecAllocator)      │
│         │                                                   │
│         └── 连接远程 Chrome (chromedp.NewRemoteAllocator)    │
│                     │                                       │
│                     ▼                                       │
│   Page (pkg/browser/actions.go)                            │
│         │                                                   │
│         └── 直接调用 chromedp API                           │
│              chromedp.Click(selector)                       │
│              chromedp.SendKeys(selector, text)              │
│              chromedp.Evaluate(script, &result)             │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

#### OpenClaw 架构

```
┌─────────────────────────────────────────────────────────────┐
│                      OpenClaw 架构                           │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│   BrowserTool (src/browser/client-actions-core.ts)          │
│         │                                                   │
│         ▼                                                   │
│   Session Manager (src/browser/pw-session.ts)               │
│         │                                                   │
│         ├── 连接 CDP URL (chromium.connectOverCDP)          │
│         │                                                   │
│         ├── 支持 Chrome 扩展中继                             │
│         │                                                   │
│         └── Page State 管理                                 │
│              ├── roleRefs (ref -> element 映射)              │
│              ├── console (控制台日志)                        │
│              ├── requests (网络请求)                         │
│              └── arm handlers (预置响应)                     │
│                     │                                       │
│                     ▼                                       │
│   Playwright Page API                                       │
│         │                                                   │
│         ├── page.getByRole(role, {name: "xxx"})             │
│         ├── locator.click() / locator.fill()                │
│         └── page._snapshotForAI()                           │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### A.3 元素定位方式对比

#### LingGuard: CSS 选择器

```go
// 必须写完整的 CSS 选择器
func (p *Page) Click(ctx context.Context, selector string, timeout time.Duration) error {
    err := chromedp.Run(p.baseCtx,
        chromedp.WaitVisible(selector),  // 等待元素可见
        chromedp.Click(selector),        // 点击
    )
    // ...
}

// 使用示例
page.Click(ctx, "#login-form > div.form-group > input#username", timeout)
page.Click(ctx, "button.btn-primary[type='submit']", timeout)
```

**问题**:
- ❌ 选择器冗长、脆弱
- ❌ 页面结构变化易失效
- ❌ LLM 需要理解 HTML 结构

#### OpenClaw: Role + Ref 系统

```typescript
// 1. 先获取快照，生成 ref
const snapshot = await snapshotRoleViaPlaywright({ cdpUrl, targetId });
// 返回: [e1] textbox "用户名"
//       [e2] textbox "密码"
//       [e3] button "登录"

// 2. 使用 ref 定位元素 (通过 role + name)
export function refLocator(page: Page, ref: string) {
    const info = state?.roleRefs?.[normalized];  // {role: "textbox", name: "用户名"}

    // 使用 Playwright 的 getByRole API
    const locator = info.name
        ? page.getByRole(info.role, { name: info.name, exact: true })
        : page.getByRole(info.role);

    return info.nth !== undefined ? locator.nth(info.nth) : locator;
}

// 3. 操作元素
await clickViaPlaywright({ cdpUrl, ref: "e3" });  // 点击登录按钮
```

**优势**:
- ✅ 基于语义角色，稳定
- ✅ 短 ref 引用，简洁
- ✅ LLM 友好，易理解

### A.4 页面状态管理对比

#### LingGuard: 无状态

```go
type Page struct {
    ID      int
    Context context.Context
    Cancel  context.CancelFunc
    URL     string
    Title   string
    baseCtx context.Context
}
// 仅存储基本信息，无页面状态追踪
```

#### OpenClaw: 完整状态管理

```typescript
type PageState = {
    console: BrowserConsoleMessage[];      // 控制台日志
    errors: BrowserPageError[];            // 页面错误
    requests: BrowserNetworkRequest[];     // 网络请求
    requestIds: WeakMap<Request, string>;
    nextRequestId: number;
    armIdUpload: number;                   // 文件上传预置 ID
    armIdDialog: number;                   // 对话框预置 ID
    armIdDownload: number;                 // 下载预置 ID
    roleRefs?: Record<string, {...}>;      // Ref 映射表
    roleRefsMode?: "role" | "aria";
    roleRefsFrameSelector?: string;
};

// 自动监听页面事件
page.on("console", (msg) => { /* 收集日志 */ });
page.on("pageerror", (err) => { /* 收集错误 */ });
page.on("request", (req) => { /* 收集请求 */ });
page.on("response", (resp) => { /* 更新请求状态 */ });
```

### A.5 连接模式对比

#### LingGuard

```go
// 模式 1: 启动本地浏览器
allocCtx, cancel := chromedp.NewExecAllocator(ctx,
    chromedp.ExecPath("/usr/bin/google-chrome"),
    chromedp.Flag("headless", true),
    chromedp.UserDataDir(profileDir),
)

// 模式 2: 连接远程浏览器
allocCtx, cancel := chromedp.NewRemoteAllocator(ctx, "http://127.0.0.1:9222")
```

#### OpenClaw

```typescript
// 连接 CDP 端点 (支持多种模式)
const browser = await chromium.connectOverCDP(endpoint, { timeout, headers });

// 支持:
// 1. 直连 Chrome CDP
// 2. 连接 Chrome 扩展中继服务器
// 3. 连接 Docker 沙箱
```

### A.6 核心差距总结

| 维度 | LingGuard | OpenClaw | 差距说明 |
|------|-----------|----------|---------|
| **设计目标** | 通用浏览器自动化 | LLM 专用浏览器工具 | OpenClaw 为 LLM 优化 |
| **元素定位** | CSS 选择器 | Role + Ref | Ref 系统更稳定简洁 |
| **页面理解** | 原始 HTML | AI 友好快照 | 快照为 LLM 优化 |
| **状态追踪** | 无 | 完整 | 可追踪日志/请求/错误 |
| **预置机制** | 无 | arm 系列 | 可预置对话框/上传/下载 |
| **安全性** | 无 | SSRF 防护 | 企业级需要 |

**一句话总结**: OpenClaw 的 **Ref 系统 + 页面状态管理 + AI 快照** 是为 LLM 操作浏览器专门设计的，而 LingGuard 目前只是传统的浏览器自动化库封装。

---

## 参考资源

- OpenClaw 源码: `src/browser/` 目录
  - `pw-role-snapshot.ts` - AI 友好快照
  - `pw-tools-core.interactions.ts` - 交互操作
  - `pw-tools-core.downloads.ts` - 下载管理
  - `navigation-guard.js` - SSRF 防护
- chromedp 文档: https://github.com/chromedp/chromedp
- Chrome DevTools Protocol: https://chromedevtools.github.io/devtools-protocol/
- ARIA 角色规范: https://www.w3.org/TR/wai-aria/#role_definitions
- Chrome Debugger API: https://developer.chrome.com/docs/extensions/reference/api/debugger
