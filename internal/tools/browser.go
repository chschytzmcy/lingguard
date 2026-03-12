// Package tools 工具实现 - 浏览器自动化工具
package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/lingguard/internal/config"
	"github.com/lingguard/pkg/browser"
	"github.com/lingguard/pkg/logger"
)

// BrowserTool 浏览器自动化工具
type BrowserTool struct {
	config    *config.BrowserConfig
	manager   *browser.Manager
	workspace string
	mu        sync.RWMutex
}

// NewBrowserTool 创建浏览器工具
func NewBrowserTool(cfg *config.BrowserConfig, workspace string) *BrowserTool {
	// 转换配置
	browserCfg := &browser.Config{
		Mode:           cfg.Mode,
		BrowserPath:    cfg.BrowserPath,
		RemoteURL:      cfg.RemoteURL,
		ProfileDir:     cfg.ProfileDir,
		ScreenshotDir:  cfg.ScreenshotDir,
		Args:           cfg.Args,
		DefaultTimeout: 30 * time.Second,
	}

	// 设置 headless 默认值
	if cfg.Headless != nil {
		browserCfg.Headless = *cfg.Headless
	} else {
		browserCfg.Headless = true // 默认无头模式
	}

	// 设置超时
	if cfg.DefaultTimeout > 0 {
		browserCfg.DefaultTimeout = time.Duration(cfg.DefaultTimeout) * time.Second
	}

	// 设置截图目录
	if cfg.ScreenshotDir == "" {
		browserCfg.ScreenshotDir = filepath.Join(workspace, "screenshots")
	}

	tool := &BrowserTool{
		config:    cfg,
		workspace: workspace,
		manager:   browser.NewManager(browserCfg),
	}

	logger.Info("BrowserTool created", "enabled", cfg.Enabled, "headless", browserCfg.Headless, "mode", cfg.Mode)
	return tool
}

func (t *BrowserTool) Name() string { return "browser" }

func (t *BrowserTool) Description() string {
	return `浏览器自动化工具，支持网页操作、截图、表单填写等。

**支持的 action**：
- navigate: 导航到 URL
- click: 点击元素
- type: 输入文本
- screenshot: 截取页面截图
- scroll: 滚动页面 (up/down/left/right/top/bottom)
- wait: 等待元素出现
- evaluate: 执行 JavaScript
- get_text: 获取元素文本
- get_html: 获取元素 HTML
- upload: 上传文件
- press: 按键 (Enter, Tab, Escape 等)
- go_back / go_forward: 前进/后退
- refresh: 刷新页面
- get_url / get_title: 获取当前 URL/标题
- new_page / switch_page / list_pages / close_page: 多页面管理

**注意**：
- 调用前必须先加载 browser skill 了解详细用法：skill --name browser
- 选择器使用 CSS 选择器语法`
}

func (t *BrowserTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"description": "操作类型",
				"enum": []string{
					"navigate", "click", "type", "type_clear", "screenshot",
					"scroll", "wait", "wait_hidden", "evaluate",
					"get_text", "get_html", "get_attribute", "set_value",
					"press", "upload", "go_back", "go_forward", "refresh",
					"get_url", "get_title", "select_option", "is_visible",
					"new_page", "switch_page", "list_pages", "close_page",
				},
			},
			"url": map[string]interface{}{
				"type":        "string",
				"description": "URL (navigate 时使用)",
			},
			"selector": map[string]interface{}{
				"type":        "string",
				"description": "CSS 选择器",
			},
			"text": map[string]interface{}{
				"type":        "string",
				"description": "输入文本 (type 时使用)",
			},
			"value": map[string]interface{}{
				"type":        "string",
				"description": "值 (set_value, select_option 时使用)",
			},
			"direction": map[string]interface{}{
				"type":        "string",
				"description": "滚动方向 (up/down/left/right/top/bottom)",
			},
			"amount": map[string]interface{}{
				"type":        "number",
				"description": "滚动距离 (像素)",
			},
			"script": map[string]interface{}{
				"type":        "string",
				"description": "JavaScript 代码 (evaluate 时使用)",
			},
			"key": map[string]interface{}{
				"type":        "string",
				"description": "按键名称 (Enter, Tab, Escape 等)",
			},
			"file_path": map[string]interface{}{
				"type":        "string",
				"description": "文件路径 (upload 时使用)",
			},
			"attribute": map[string]interface{}{
				"type":        "string",
				"description": "属性名 (get_attribute 时使用)",
			},
			"full_page": map[string]interface{}{
				"type":        "boolean",
				"description": "是否全页截图 (screenshot 时使用，默认 false)",
			},
			"timeout": map[string]interface{}{
				"type":        "number",
				"description": "超时时间 (秒)",
			},
			"page_id": map[string]interface{}{
				"type":        "number",
				"description": "页面 ID (switch_page, close_page 时使用)",
			},
		},
		"required": []string{"action"},
	}
}

func (t *BrowserTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p struct {
		Action    string  `json:"action"`
		URL       string  `json:"url"`
		Selector  string  `json:"selector"`
		Text      string  `json:"text"`
		Value     string  `json:"value"`
		Direction string  `json:"direction"`
		Amount    float64 `json:"amount"`
		Script    string  `json:"script"`
		Key       string  `json:"key"`
		FilePath  string  `json:"file_path"`
		Attribute string  `json:"attribute"`
		FullPage  bool    `json:"full_page"`
		Timeout   int     `json:"timeout"`
		PageID    int     `json:"page_id"`
	}

	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("invalid parameters: %w", err)
	}

	// 确保浏览器已连接
	if p.Action != "list_pages" {
		if err := t.ensureConnected(ctx); err != nil {
			return "", err
		}
	}

	timeout := time.Duration(p.Timeout) * time.Second
	if timeout == 0 {
		timeout = time.Duration(t.config.DefaultTimeout) * time.Second
		if timeout == 0 {
			timeout = 30 * time.Second
		}
	}

	switch p.Action {
	case "navigate":
		return t.navigate(ctx, p.URL, timeout)
	case "click":
		return t.click(ctx, p.Selector, timeout)
	case "type":
		return t.typeText(ctx, p.Selector, p.Text, timeout, false)
	case "type_clear":
		return t.typeText(ctx, p.Selector, p.Text, timeout, true)
	case "screenshot":
		return t.screenshot(ctx, p.Selector, p.FullPage)
	case "scroll":
		return t.scroll(ctx, p.Direction, p.Amount)
	case "wait":
		return t.wait(ctx, p.Selector, timeout)
	case "wait_hidden":
		return t.waitHidden(ctx, p.Selector, timeout)
	case "evaluate":
		return t.evaluate(ctx, p.Script)
	case "get_text":
		return t.getText(ctx, p.Selector)
	case "get_html":
		return t.getHTML(ctx, p.Selector)
	case "get_attribute":
		return t.getAttribute(ctx, p.Selector, p.Attribute)
	case "set_value":
		return t.setValue(ctx, p.Selector, p.Value, timeout)
	case "press":
		return t.press(ctx, p.Key)
	case "upload":
		return t.upload(ctx, p.Selector, p.FilePath, timeout)
	case "go_back":
		return t.goBack(ctx, timeout)
	case "go_forward":
		return t.goForward(ctx, timeout)
	case "refresh":
		return t.refresh(ctx, timeout)
	case "get_url":
		return t.getURL(ctx)
	case "get_title":
		return t.getTitle(ctx)
	case "select_option":
		return t.selectOption(ctx, p.Selector, p.Value, timeout)
	case "is_visible":
		return t.isVisible(ctx, p.Selector)
	case "new_page":
		return t.newPage(ctx)
	case "switch_page":
		return t.switchPage(p.PageID)
	case "list_pages":
		return t.listPages()
	case "close_page":
		return t.closePage(p.PageID)
	default:
		return "", fmt.Errorf("unknown action: %s", p.Action)
	}
}

// ensureConnected 确保浏览器已连接
func (t *BrowserTool) ensureConnected(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.manager.IsConnected() {
		return nil
	}

	logger.Info("Connecting to browser...")
	if err := t.manager.Connect(ctx); err != nil {
		return fmt.Errorf("connect browser: %w", err)
	}

	return nil
}

// getCurrentPage 获取当前页面
func (t *BrowserTool) getCurrentPage() (*browser.Page, error) {
	page, err := t.manager.GetCurrentPage()
	if err != nil {
		return nil, fmt.Errorf("no active page. Use navigate action first")
	}
	return page, nil
}

func (t *BrowserTool) navigate(ctx context.Context, url string, timeout time.Duration) (string, error) {
	if url == "" {
		return "", fmt.Errorf("url is required")
	}

	// 确保连接
	if err := t.ensureConnected(ctx); err != nil {
		return "", err
	}

	page, err := t.getCurrentPage()
	if err != nil {
		return "", err
	}

	if err := page.Navigate(ctx, url, timeout); err != nil {
		return "", err
	}

	// 获取标题
	title, _ := page.GetTitle(ctx)

	return fmt.Sprintf("Navigated to: %s\nTitle: %s", url, title), nil
}

func (t *BrowserTool) click(ctx context.Context, selector string, timeout time.Duration) (string, error) {
	if selector == "" {
		return "", fmt.Errorf("selector is required")
	}

	page, err := t.getCurrentPage()
	if err != nil {
		return "", err
	}

	if err := page.Click(ctx, selector, timeout); err != nil {
		return "", err
	}

	return fmt.Sprintf("Clicked: %s", selector), nil
}

func (t *BrowserTool) typeText(ctx context.Context, selector, text string, timeout time.Duration, clear bool) (string, error) {
	if selector == "" {
		return "", fmt.Errorf("selector is required")
	}

	page, err := t.getCurrentPage()
	if err != nil {
		return "", err
	}

	if clear {
		if err := page.TypeWithClear(ctx, selector, text, timeout); err != nil {
			return "", err
		}
	} else {
		if err := page.Type(ctx, selector, text, timeout); err != nil {
			return "", err
		}
	}

	return fmt.Sprintf("Typed %d characters into: %s", len(text), selector), nil
}

func (t *BrowserTool) screenshot(ctx context.Context, selector string, fullPage bool) (string, error) {
	page, err := t.getCurrentPage()
	if err != nil {
		return "", err
	}

	var buf []byte
	if selector != "" {
		buf, err = page.ScreenshotElement(ctx, selector)
	} else {
		buf, err = page.Screenshot(ctx, fullPage)
	}
	if err != nil {
		return "", err
	}

	// 保存截图
	dir, err := t.manager.EnsureScreenshotDir()
	if err != nil {
		return "", err
	}

	filename := fmt.Sprintf("screenshot_%s.png", time.Now().Format("20060102_150405"))
	filePath := filepath.Join(dir, filename)

	if err := os.WriteFile(filePath, buf, 0644); err != nil {
		return "", fmt.Errorf("save screenshot: %w", err)
	}

	// 返回 base64 编码和文件路径
	b64 := base64.StdEncoding.EncodeToString(buf)

	result := fmt.Sprintf("Screenshot saved to: %s\nSize: %d bytes\nBase64 length: %d",
		filePath, len(buf), len(b64))

	// 如果是图片元素，添加 base64 数据以便显示
	if selector != "" || len(buf) < 100*1024 { // 小于 100KB 的图片返回 base64
		result += fmt.Sprintf("\n\n![screenshot](data:image/png;base64,%s)", b64)
	}

	return result, nil
}

func (t *BrowserTool) scroll(ctx context.Context, direction string, amount float64) (string, error) {
	if direction == "" {
		direction = "down"
	}
	if amount == 0 {
		amount = 300
	}

	page, err := t.getCurrentPage()
	if err != nil {
		return "", err
	}

	if err := page.Scroll(ctx, direction, amount); err != nil {
		return "", err
	}

	return fmt.Sprintf("Scrolled %s by %.0f pixels", direction, amount), nil
}

func (t *BrowserTool) wait(ctx context.Context, selector string, timeout time.Duration) (string, error) {
	if selector == "" {
		return "", fmt.Errorf("selector is required")
	}

	page, err := t.getCurrentPage()
	if err != nil {
		return "", err
	}

	if err := page.Wait(ctx, selector, timeout); err != nil {
		return "", err
	}

	return fmt.Sprintf("Element appeared: %s", selector), nil
}

func (t *BrowserTool) waitHidden(ctx context.Context, selector string, timeout time.Duration) (string, error) {
	if selector == "" {
		return "", fmt.Errorf("selector is required")
	}

	page, err := t.getCurrentPage()
	if err != nil {
		return "", err
	}

	if err := page.WaitHidden(ctx, selector, timeout); err != nil {
		return "", err
	}

	return fmt.Sprintf("Element disappeared: %s", selector), nil
}

func (t *BrowserTool) evaluate(ctx context.Context, script string) (string, error) {
	if script == "" {
		return "", fmt.Errorf("script is required")
	}

	page, err := t.getCurrentPage()
	if err != nil {
		return "", err
	}

	var result interface{}
	if err := page.Evaluate(ctx, script, &result); err != nil {
		return "", err
	}

	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	return fmt.Sprintf("Result:\n%s", string(resultJSON)), nil
}

func (t *BrowserTool) getText(ctx context.Context, selector string) (string, error) {
	if selector == "" {
		return "", fmt.Errorf("selector is required")
	}

	page, err := t.getCurrentPage()
	if err != nil {
		return "", err
	}

	text, err := page.GetText(ctx, selector)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Text from %s:\n%s", selector, text), nil
}

func (t *BrowserTool) getHTML(ctx context.Context, selector string) (string, error) {
	if selector == "" {
		return "", fmt.Errorf("selector is required")
	}

	page, err := t.getCurrentPage()
	if err != nil {
		return "", err
	}

	html, err := page.GetHTML(ctx, selector)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("HTML from %s:\n%s", selector, html), nil
}

func (t *BrowserTool) getAttribute(ctx context.Context, selector, attribute string) (string, error) {
	if selector == "" {
		return "", fmt.Errorf("selector is required")
	}
	if attribute == "" {
		return "", fmt.Errorf("attribute is required")
	}

	page, err := t.getCurrentPage()
	if err != nil {
		return "", err
	}

	value, err := page.GetAttribute(ctx, selector, attribute)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Attribute '%s' from %s: %s", attribute, selector, value), nil
}

func (t *BrowserTool) setValue(ctx context.Context, selector, value string, timeout time.Duration) (string, error) {
	if selector == "" {
		return "", fmt.Errorf("selector is required")
	}

	page, err := t.getCurrentPage()
	if err != nil {
		return "", err
	}

	if err := page.SetValue(ctx, selector, value, timeout); err != nil {
		return "", err
	}

	return fmt.Sprintf("Set value on %s: %s", selector, value), nil
}

func (t *BrowserTool) press(ctx context.Context, key string) (string, error) {
	if key == "" {
		return "", fmt.Errorf("key is required")
	}

	page, err := t.getCurrentPage()
	if err != nil {
		return "", err
	}

	if err := page.Press(ctx, key); err != nil {
		return "", err
	}

	return fmt.Sprintf("Pressed key: %s", key), nil
}

func (t *BrowserTool) upload(ctx context.Context, selector, filePath string, timeout time.Duration) (string, error) {
	if selector == "" {
		return "", fmt.Errorf("selector is required")
	}
	if filePath == "" {
		return "", fmt.Errorf("file_path is required")
	}

	// 展开路径
	if strings.HasPrefix(filePath, "~/") {
		home, _ := os.UserHomeDir()
		filePath = filepath.Join(home, filePath[2:])
	}

	page, err := t.getCurrentPage()
	if err != nil {
		return "", err
	}

	if err := page.Upload(ctx, selector, filePath, timeout); err != nil {
		return "", err
	}

	return fmt.Sprintf("Uploaded file to %s: %s", selector, filePath), nil
}

func (t *BrowserTool) goBack(ctx context.Context, timeout time.Duration) (string, error) {
	page, err := t.getCurrentPage()
	if err != nil {
		return "", err
	}

	if err := page.GoBack(ctx, timeout); err != nil {
		return "", err
	}

	url, _ := page.GetURL(ctx)
	return fmt.Sprintf("Navigated back to: %s", url), nil
}

func (t *BrowserTool) goForward(ctx context.Context, timeout time.Duration) (string, error) {
	page, err := t.getCurrentPage()
	if err != nil {
		return "", err
	}

	if err := page.GoForward(ctx, timeout); err != nil {
		return "", err
	}

	url, _ := page.GetURL(ctx)
	return fmt.Sprintf("Navigated forward to: %s", url), nil
}

func (t *BrowserTool) refresh(ctx context.Context, timeout time.Duration) (string, error) {
	page, err := t.getCurrentPage()
	if err != nil {
		return "", err
	}

	if err := page.Refresh(ctx, timeout); err != nil {
		return "", err
	}

	return "Page refreshed", nil
}

func (t *BrowserTool) getURL(ctx context.Context) (string, error) {
	page, err := t.getCurrentPage()
	if err != nil {
		return "", err
	}

	url, err := page.GetURL(ctx)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Current URL: %s", url), nil
}

func (t *BrowserTool) getTitle(ctx context.Context) (string, error) {
	page, err := t.getCurrentPage()
	if err != nil {
		return "", err
	}

	title, err := page.GetTitle(ctx)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Page title: %s", title), nil
}

func (t *BrowserTool) selectOption(ctx context.Context, selector, value string, timeout time.Duration) (string, error) {
	if selector == "" {
		return "", fmt.Errorf("selector is required")
	}
	if value == "" {
		return "", fmt.Errorf("value is required")
	}

	page, err := t.getCurrentPage()
	if err != nil {
		return "", err
	}

	if err := page.SelectOption(ctx, selector, value, timeout); err != nil {
		return "", err
	}

	return fmt.Sprintf("Selected option %s on %s", value, selector), nil
}

func (t *BrowserTool) isVisible(ctx context.Context, selector string) (string, error) {
	if selector == "" {
		return "", fmt.Errorf("selector is required")
	}

	page, err := t.getCurrentPage()
	if err != nil {
		return "", err
	}

	visible, err := page.IsVisible(ctx, selector)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Element %s visible: %v", selector, visible), nil
}

func (t *BrowserTool) newPage(ctx context.Context) (string, error) {
	if err := t.ensureConnected(ctx); err != nil {
		return "", err
	}

	page, err := t.manager.NewPage(ctx)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("New page created with ID: %d", page.ID), nil
}

func (t *BrowserTool) switchPage(pageID int) (string, error) {
	if pageID == 0 {
		return "", fmt.Errorf("page_id is required")
	}

	if err := t.manager.SwitchPage(pageID); err != nil {
		return "", err
	}

	return fmt.Sprintf("Switched to page: %d", pageID), nil
}

func (t *BrowserTool) listPages() (string, error) {
	pages := t.manager.ListPages()

	if len(pages) == 0 {
		return "No pages open. Use navigate to open a page.", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Open pages (%d):\n", len(pages)))
	for _, p := range pages {
		current := ""
		if p.IsCurrent {
			current = " (current)"
		}
		sb.WriteString(fmt.Sprintf("  [%d]%s %s\n", p.ID, current, p.URL))
	}

	return sb.String(), nil
}

func (t *BrowserTool) closePage(pageID int) (string, error) {
	if pageID == 0 {
		return "", fmt.Errorf("page_id is required")
	}

	if err := t.manager.ClosePage(pageID); err != nil {
		return "", err
	}

	return fmt.Sprintf("Page %d closed", pageID), nil
}

// Close 关闭浏览器
func (t *BrowserTool) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.manager != nil {
		return t.manager.Close()
	}
	return nil
}

func (t *BrowserTool) IsDangerous() bool { return true }

func (t *BrowserTool) ShouldLoadByDefault() bool { return false }
