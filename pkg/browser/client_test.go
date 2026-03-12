package browser

import (
	"context"
	"testing"
	"time"
)

func TestManager_NewManager(t *testing.T) {
	cfg := &Config{
		Headless:       true,
		DefaultTimeout: 10 * time.Second,
	}

	m := NewManager(cfg)
	if m == nil {
		t.Fatal("NewManager returned nil")
	}

	if m.config != cfg {
		t.Error("Config not set correctly")
	}

	if m.pages == nil {
		t.Error("pages map not initialized")
	}
}

func TestManager_IsConnected(t *testing.T) {
	cfg := &Config{
		Headless:       true,
		DefaultTimeout: 10 * time.Second,
	}

	m := NewManager(cfg)

	if m.IsConnected() {
		t.Error("New manager should not be connected")
	}
}

func TestManager_ConnectAndClose(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := &Config{
		Mode:           "managed",
		Headless:       true,
		DefaultTimeout: 30 * time.Second,
	}

	m := NewManager(cfg)
	ctx := context.Background()

	// Connect
	if err := m.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	if !m.IsConnected() {
		t.Error("Manager should be connected after Connect")
	}

	// Should have one page
	pages := m.ListPages()
	if len(pages) != 1 {
		t.Errorf("Expected 1 page, got %d", len(pages))
	}

	// Close
	if err := m.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	if m.IsConnected() {
		t.Error("Manager should not be connected after Close")
	}
}

func TestManager_NewPage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := &Config{
		Mode:           "managed",
		Headless:       true,
		DefaultTimeout: 30 * time.Second,
	}

	m := NewManager(cfg)
	ctx := context.Background()

	if err := m.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer m.Close()

	// Create new page
	page, err := m.NewPage(ctx)
	if err != nil {
		t.Fatalf("NewPage failed: %v", err)
	}

	if page.ID == 0 {
		t.Error("Page ID should not be 0")
	}

	// Check pages list
	pages := m.ListPages()
	if len(pages) != 2 {
		t.Errorf("Expected 2 pages, got %d", len(pages))
	}
}

func TestPage_NavigateAndGetURL(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := &Config{
		Mode:           "managed",
		Headless:       true,
		DefaultTimeout: 30 * time.Second,
	}

	m := NewManager(cfg)
	ctx := context.Background()

	if err := m.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer m.Close()

	page, err := m.GetCurrentPage()
	if err != nil {
		t.Fatalf("GetCurrentPage failed: %v", err)
	}

	// Navigate to example.com
	if err := page.Navigate(ctx, "https://example.com", 30*time.Second); err != nil {
		t.Fatalf("Navigate failed: %v", err)
	}

	// Get URL using the same page's context (not passing a new context)
	url := page.URL
	if url == "" {
		t.Error("URL should not be empty after navigation")
	}

	t.Logf("Navigated to: %s", url)

	// Get title
	title, err := page.GetTitle(ctx)
	if err != nil {
		t.Fatalf("GetTitle failed: %v", err)
	}
	t.Logf("Title: %s", title)
}

func TestPage_Screenshot(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := &Config{
		Mode:           "managed",
		Headless:       true,
		DefaultTimeout: 30 * time.Second,
	}

	m := NewManager(cfg)
	ctx := context.Background()

	if err := m.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer m.Close()

	page, err := m.GetCurrentPage()
	if err != nil {
		t.Fatalf("GetCurrentPage failed: %v", err)
	}

	// Navigate first
	if err := page.Navigate(ctx, "https://example.com", 30*time.Second); err != nil {
		t.Fatalf("Navigate failed: %v", err)
	}

	// Take screenshot
	buf, err := page.Screenshot(ctx, false)
	if err != nil {
		t.Fatalf("Screenshot failed: %v", err)
	}

	if len(buf) == 0 {
		t.Error("Screenshot buffer should not be empty")
	}

	t.Logf("Screenshot size: %d bytes", len(buf))
}

func TestPage_GetText(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := &Config{
		Mode:           "managed",
		Headless:       true,
		DefaultTimeout: 30 * time.Second,
	}

	m := NewManager(cfg)
	ctx := context.Background()

	if err := m.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer m.Close()

	page, err := m.GetCurrentPage()
	if err != nil {
		t.Fatalf("GetCurrentPage failed: %v", err)
	}

	// Navigate first
	if err := page.Navigate(ctx, "https://example.com", 30*time.Second); err != nil {
		t.Fatalf("Navigate failed: %v", err)
	}

	// Get text from h1
	text, err := page.GetText(ctx, "h1")
	if err != nil {
		t.Fatalf("GetText failed: %v", err)
	}

	if text == "" {
		t.Error("Text should not be empty for h1 on example.com")
	}

	t.Logf("Got text: %s", text)
}
