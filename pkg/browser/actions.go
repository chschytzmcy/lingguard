package browser

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/lingguard/pkg/logger"
)

// Navigate 导航到 URL
func (p *Page) Navigate(ctx context.Context, url string, timeout time.Duration) error {
	err := chromedp.Run(p.baseCtx,
		chromedp.Navigate(url),
		chromedp.WaitReady("body"),
	)
	if err != nil {
		return fmt.Errorf("navigate to %s: %w", url, err)
	}

	// 更新页面 URL
	var currentURL string
	if err := chromedp.Run(p.baseCtx, chromedp.Location(&currentURL)); err == nil {
		p.URL = currentURL
	}

	logger.Info("Navigated to", "url", url, "page_id", p.ID)
	return nil
}

// Click 点击元素
func (p *Page) Click(ctx context.Context, selector string, timeout time.Duration) error {
	err := chromedp.Run(p.baseCtx,
		chromedp.WaitVisible(selector),
		chromedp.Click(selector),
	)
	if err != nil {
		return fmt.Errorf("click %s: %w", selector, err)
	}

	logger.Info("Clicked element", "selector", selector, "page_id", p.ID)
	return nil
}

// Type 在元素中输入文本
func (p *Page) Type(ctx context.Context, selector, text string, timeout time.Duration) error {
	err := chromedp.Run(p.baseCtx,
		chromedp.WaitVisible(selector),
		chromedp.Focus(selector),
		chromedp.SendKeys(selector, text),
	)
	if err != nil {
		return fmt.Errorf("type into %s: %w", selector, err)
	}

	logger.Info("Typed text", "selector", selector, "length", len(text), "page_id", p.ID)
	return nil
}

// TypeWithClear 清空并输入文本
func (p *Page) TypeWithClear(ctx context.Context, selector, text string, timeout time.Duration) error {
	err := chromedp.Run(p.baseCtx,
		chromedp.WaitVisible(selector),
		chromedp.Focus(selector),
		chromedp.Clear(selector),
		chromedp.SendKeys(selector, text),
	)
	if err != nil {
		return fmt.Errorf("type with clear into %s: %w", selector, err)
	}

	logger.Info("Typed text (cleared first)", "selector", selector, "length", len(text), "page_id", p.ID)
	return nil
}

// Screenshot 截取页面截图
func (p *Page) Screenshot(ctx context.Context, fullPage bool) ([]byte, error) {
	var buf []byte

	var action chromedp.Action
	if fullPage {
		action = chromedp.FullScreenshot(&buf, 90)
	} else {
		action = chromedp.CaptureScreenshot(&buf)
	}

	err := chromedp.Run(p.baseCtx, action)
	if err != nil {
		return nil, fmt.Errorf("screenshot: %w", err)
	}

	logger.Info("Screenshot taken", "fullPage", fullPage, "size", len(buf), "page_id", p.ID)
	return buf, nil
}

// ScreenshotElement 截取指定元素的截图
func (p *Page) ScreenshotElement(ctx context.Context, selector string) ([]byte, error) {
	var buf []byte

	err := chromedp.Run(p.baseCtx,
		chromedp.WaitVisible(selector),
		chromedp.Screenshot(selector, &buf),
	)
	if err != nil {
		return nil, fmt.Errorf("screenshot element %s: %w", selector, err)
	}

	logger.Info("Element screenshot taken", "selector", selector, "size", len(buf), "page_id", p.ID)
	return buf, nil
}

// Scroll 滚动页面
func (p *Page) Scroll(ctx context.Context, direction string, amount float64) error {
	var scrollScript string
	switch strings.ToLower(direction) {
	case "up":
		scrollScript = fmt.Sprintf("window.scrollBy(0, -%f)", amount)
	case "down":
		scrollScript = fmt.Sprintf("window.scrollBy(0, %f)", amount)
	case "left":
		scrollScript = fmt.Sprintf("window.scrollBy(-%f, 0)", amount)
	case "right":
		scrollScript = fmt.Sprintf("window.scrollBy(%f, 0)", amount)
	case "top":
		scrollScript = "window.scrollTo(0, 0)"
	case "bottom":
		scrollScript = "window.scrollTo(0, document.body.scrollHeight)"
	default:
		return fmt.Errorf("invalid scroll direction: %s", direction)
	}

	err := chromedp.Run(p.baseCtx,
		chromedp.Evaluate(scrollScript, nil),
	)
	if err != nil {
		return fmt.Errorf("scroll %s: %w", direction, err)
	}

	logger.Info("Scrolled", "direction", direction, "amount", amount, "page_id", p.ID)
	return nil
}

// Wait 等待元素出现
func (p *Page) Wait(ctx context.Context, selector string, timeout time.Duration) error {
	err := chromedp.Run(p.baseCtx,
		chromedp.WaitVisible(selector),
	)
	if err != nil {
		return fmt.Errorf("wait for %s: %w", selector, err)
	}

	logger.Info("Element appeared", "selector", selector, "page_id", p.ID)
	return nil
}

// WaitHidden 等待元素消失
func (p *Page) WaitHidden(ctx context.Context, selector string, timeout time.Duration) error {
	err := chromedp.Run(p.baseCtx,
		chromedp.WaitNotVisible(selector),
	)
	if err != nil {
		return fmt.Errorf("wait hidden for %s: %w", selector, err)
	}

	logger.Info("Element disappeared", "selector", selector, "page_id", p.ID)
	return nil
}

// Evaluate 执行 JavaScript
func (p *Page) Evaluate(ctx context.Context, script string, result interface{}) error {
	err := chromedp.Run(p.baseCtx,
		chromedp.Evaluate(script, result),
	)
	if err != nil {
		return fmt.Errorf("evaluate script: %w", err)
	}

	logger.Info("JavaScript executed", "script_length", len(script), "page_id", p.ID)
	return nil
}

// GetText 获取元素文本
func (p *Page) GetText(ctx context.Context, selector string) (string, error) {
	var text string

	err := chromedp.Run(p.baseCtx,
		chromedp.Text(selector, &text),
	)
	if err != nil {
		return "", fmt.Errorf("get text from %s: %w", selector, err)
	}

	return text, nil
}

// GetHTML 获取元素 HTML
func (p *Page) GetHTML(ctx context.Context, selector string) (string, error) {
	var html string

	err := chromedp.Run(p.baseCtx,
		chromedp.OuterHTML(selector, &html),
	)
	if err != nil {
		return "", fmt.Errorf("get HTML from %s: %w", selector, err)
	}

	return html, nil
}

// GetAttribute 获取元素属性
func (p *Page) GetAttribute(ctx context.Context, selector, attribute string) (string, error) {
	var value string
	var ok bool

	err := chromedp.Run(p.baseCtx,
		chromedp.AttributeValue(selector, attribute, &value, &ok),
	)
	if err != nil {
		return "", fmt.Errorf("get attribute %s from %s: %w", attribute, selector, err)
	}

	if !ok {
		return "", fmt.Errorf("attribute %s not found on %s", attribute, selector)
	}

	return value, nil
}

// SetValue 设置表单元素的值
func (p *Page) SetValue(ctx context.Context, selector, value string, timeout time.Duration) error {
	err := chromedp.Run(p.baseCtx,
		chromedp.WaitVisible(selector),
		chromedp.SetValue(selector, value),
	)
	if err != nil {
		return fmt.Errorf("set value on %s: %w", selector, err)
	}

	logger.Info("Value set", "selector", selector, "page_id", p.ID)
	return nil
}

// Press 按键
func (p *Page) Press(ctx context.Context, key string) error {
	err := chromedp.Run(p.baseCtx,
		chromedp.KeyEvent(key),
	)
	if err != nil {
		return fmt.Errorf("press key %s: %w", key, err)
	}

	logger.Info("Key pressed", "key", key, "page_id", p.ID)
	return nil
}

// Upload 上传文件
func (p *Page) Upload(ctx context.Context, selector, filePath string, timeout time.Duration) error {
	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", filePath)
	}

	// 获取绝对路径
	absPath, _ := filepath.Abs(filePath)

	err := chromedp.Run(p.baseCtx,
		chromedp.WaitVisible(selector),
		chromedp.SetUploadFiles(selector, []string{absPath}),
	)
	if err != nil {
		return fmt.Errorf("upload file to %s: %w", selector, err)
	}

	logger.Info("File uploaded", "selector", selector, "file", filePath, "page_id", p.ID)
	return nil
}

// GoBack 后退
func (p *Page) GoBack(ctx context.Context, timeout time.Duration) error {
	err := chromedp.Run(p.baseCtx,
		chromedp.NavigateBack(),
	)
	if err != nil {
		return fmt.Errorf("go back: %w", err)
	}

	// 更新页面 URL
	var currentURL string
	if err := chromedp.Run(p.baseCtx, chromedp.Location(&currentURL)); err == nil {
		p.URL = currentURL
	}

	logger.Info("Navigated back", "url", currentURL, "page_id", p.ID)
	return nil
}

// GoForward 前进
func (p *Page) GoForward(ctx context.Context, timeout time.Duration) error {
	err := chromedp.Run(p.baseCtx,
		chromedp.NavigateForward(),
	)
	if err != nil {
		return fmt.Errorf("go forward: %w", err)
	}

	// 更新页面 URL
	var currentURL string
	if err := chromedp.Run(p.baseCtx, chromedp.Location(&currentURL)); err == nil {
		p.URL = currentURL
	}

	logger.Info("Navigated forward", "url", currentURL, "page_id", p.ID)
	return nil
}

// Refresh 刷新页面
func (p *Page) Refresh(ctx context.Context, timeout time.Duration) error {
	err := chromedp.Run(p.baseCtx,
		chromedp.Reload(),
	)
	if err != nil {
		return fmt.Errorf("refresh: %w", err)
	}

	logger.Info("Page refreshed", "page_id", p.ID)
	return nil
}

// GetURL 获取当前页面 URL
func (p *Page) GetURL(ctx context.Context) (string, error) {
	var url string
	err := chromedp.Run(p.baseCtx, chromedp.Location(&url))
	if err != nil {
		return "", fmt.Errorf("get URL: %w", err)
	}
	p.URL = url
	return url, nil
}

// GetTitle 获取页面标题
func (p *Page) GetTitle(ctx context.Context) (string, error) {
	var title string
	err := chromedp.Run(p.baseCtx, chromedp.Title(&title))
	if err != nil {
		return "", fmt.Errorf("get title: %w", err)
	}
	p.Title = title
	return title, nil
}

// SelectOption 选择下拉框选项
func (p *Page) SelectOption(ctx context.Context, selector, value string, timeout time.Duration) error {
	err := chromedp.Run(p.baseCtx,
		chromedp.WaitVisible(selector),
		chromedp.SetAttributeValue(selector, "value", value),
	)
	if err != nil {
		return fmt.Errorf("select option %s on %s: %w", value, selector, err)
	}

	logger.Info("Option selected", "selector", selector, "value", value, "page_id", p.ID)
	return nil
}

// IsVisible 检查元素是否可见
func (p *Page) IsVisible(ctx context.Context, selector string) (bool, error) {
	var visible bool
	err := chromedp.Run(p.baseCtx,
		chromedp.Evaluate(fmt.Sprintf(
			`(() => { const el = document.querySelector('%s'); return el && el.offsetParent !== null; })()`,
			selector,
		), &visible),
	)
	if err != nil {
		return false, fmt.Errorf("check visibility of %s: %w", selector, err)
	}
	return visible, nil
}

// WaitForNavigation 等待导航完成
func (p *Page) WaitForNavigation(ctx context.Context, timeout time.Duration) error {
	// 等待页面加载完成
	err := chromedp.Run(p.baseCtx,
		chromedp.WaitReady("body"),
	)
	if err != nil {
		return fmt.Errorf("wait for navigation: %w", err)
	}

	return nil
}
