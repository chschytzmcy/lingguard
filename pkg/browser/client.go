// Package browser 浏览器自动化包，基于 Chrome DevTools Protocol
package browser

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/lingguard/pkg/logger"
)

// Config 浏览器配置
type Config struct {
	Mode           string        // "managed" | "connect"
	BrowserPath    string        // 浏览器可执行文件路径
	RemoteURL      string        // 远程 CDP URL
	Headless       bool          // 无头模式
	ProfileDir     string        // Profile 目录
	DefaultTimeout time.Duration // 默认超时
	ScreenshotDir  string        // 截图目录
	Args           []string      // 额外启动参数
}

// Manager 浏览器管理器
type Manager struct {
	config      *Config
	allocCtx    context.Context
	allocCancel context.CancelFunc
	pages       map[int]*Page
	currentPage int
	nextPageID  int
	mu          sync.RWMutex
}

// Page 表示一个浏览器页面/标签
type Page struct {
	ID      int
	Context context.Context
	Cancel  context.CancelFunc
	URL     string
	Title   string
	baseCtx context.Context // 基础 context，不受操作取消影响
}

// NewManager 创建浏览器管理器
func NewManager(cfg *Config) *Manager {
	if cfg.DefaultTimeout == 0 {
		cfg.DefaultTimeout = 30 * time.Second
	}
	return &Manager{
		config: cfg,
		pages:  make(map[int]*Page),
	}
}

// Connect 启动或连接浏览器
func (m *Manager) Connect(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.allocCtx != nil {
		return nil // Already connected
	}

	// 创建 allocator 上下文
	if m.config.Mode == "connect" && m.config.RemoteURL != "" {
		// 连接远程浏览器
		allocatorCtx, cancel := chromedp.NewRemoteAllocator(ctx, m.config.RemoteURL)
		m.allocCtx = allocatorCtx
		m.allocCancel = cancel
		logger.Info("Browser connected to remote", "url", m.config.RemoteURL)
	} else {
		// 启动本地浏览器
		allocOpts := []chromedp.ExecAllocatorOption{
			chromedp.NoDefaultBrowserCheck,
			chromedp.NoFirstRun,
		}

		if m.config.Headless {
			allocOpts = append(allocOpts, chromedp.Flag("headless", true))
		}

		if m.config.BrowserPath != "" {
			allocOpts = append(allocOpts, chromedp.ExecPath(m.config.BrowserPath))
		}

		if m.config.ProfileDir != "" {
			// 确保目录存在
			if err := os.MkdirAll(m.config.ProfileDir, 0755); err != nil {
				return fmt.Errorf("create profile dir: %w", err)
			}
			allocOpts = append(allocOpts, chromedp.UserDataDir(m.config.ProfileDir))
		}

		// 添加额外参数
		for _, arg := range m.config.Args {
			allocOpts = append(allocOpts, chromedp.Flag(arg, true))
		}

		// 创建 allocator context
		allocCtx, cancel := chromedp.NewExecAllocator(ctx, allocOpts...)
		m.allocCtx = allocCtx
		m.allocCancel = cancel
		logger.Info("Browser started", "headless", m.config.Headless)
	}

	// 创建第一个页面
	_, err := m.newPageLocked()
	if err != nil {
		return fmt.Errorf("create initial page: %w", err)
	}

	return nil
}

// NewPage 创建新页面
func (m *Manager) NewPage(ctx context.Context) (*Page, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.allocCtx == nil {
		return nil, fmt.Errorf("browser not connected")
	}

	return m.newPageLocked()
}

// newPageLocked 创建新页面（需要持有锁）
func (m *Manager) newPageLocked() (*Page, error) {
	pageCtx, cancel := chromedp.NewContext(m.allocCtx)

	page := &Page{
		ID:      m.nextPageID,
		Context: pageCtx,
		Cancel:  cancel,
		baseCtx: pageCtx, // 保存基础 context
	}
	m.nextPageID++
	m.pages[page.ID] = page
	m.currentPage = page.ID

	logger.Info("New browser page created", "id", page.ID)
	return page, nil
}

// GetCurrentPage 获取当前页面
func (m *Manager) GetCurrentPage() (*Page, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.allocCtx == nil {
		return nil, fmt.Errorf("browser not connected")
	}

	page, ok := m.pages[m.currentPage]
	if !ok {
		return nil, fmt.Errorf("no current page")
	}
	return page, nil
}

// GetPage 获取指定 ID 的页面
func (m *Manager) GetPage(id int) (*Page, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	page, ok := m.pages[id]
	if !ok {
		return nil, fmt.Errorf("page not found: %d", id)
	}
	return page, nil
}

// SwitchPage 切换当前页面
func (m *Manager) SwitchPage(id int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.pages[id]; !ok {
		return fmt.Errorf("page not found: %d", id)
	}
	m.currentPage = id
	logger.Info("Switched to page", "id", id)
	return nil
}

// ClosePage 关闭指定页面
func (m *Manager) ClosePage(id int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	page, ok := m.pages[id]
	if !ok {
		return fmt.Errorf("page not found: %d", id)
	}

	// 关闭页面上下文
	page.Cancel()
	delete(m.pages, id)

	// 如果关闭的是当前页面，切换到其他页面
	if m.currentPage == id {
		m.currentPage = 0
		for pid := range m.pages {
			m.currentPage = pid
			break
		}
	}

	logger.Info("Page closed", "id", id)
	return nil
}

// ListPages 列出所有页面
func (m *Manager) ListPages() []PageInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var list []PageInfo
	for id, page := range m.pages {
		list = append(list, PageInfo{
			ID:        id,
			URL:       page.URL,
			Title:     page.Title,
			IsCurrent: id == m.currentPage,
		})
	}
	return list
}

// PageInfo 页面信息
type PageInfo struct {
	ID        int    `json:"id"`
	URL       string `json:"url"`
	Title     string `json:"title"`
	IsCurrent bool   `json:"isCurrent"`
}

// Close 关闭浏览器
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 关闭所有页面
	for _, page := range m.pages {
		page.Cancel()
	}
	m.pages = make(map[int]*Page)

	// 关闭 allocator
	if m.allocCancel != nil {
		m.allocCancel()
	}
	m.allocCtx = nil
	m.allocCancel = nil

	logger.Info("Browser closed")
	return nil
}

// IsConnected 检查是否已连接
func (m *Manager) IsConnected() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.allocCtx != nil
}

// GetConfig 获取配置
func (m *Manager) GetConfig() *Config {
	return m.config
}

// EnsureScreenshotDir 确保截图目录存在
func (m *Manager) EnsureScreenshotDir() (string, error) {
	dir := m.config.ScreenshotDir
	if dir == "" {
		// 默认使用 workspace 下的 screenshots 目录
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".lingguard", "workspace", "screenshots")
	}

	// 展开 ~ 路径
	if strings.HasPrefix(dir, "~/") {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, dir[2:])
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create screenshot dir: %w", err)
	}
	return dir, nil
}
