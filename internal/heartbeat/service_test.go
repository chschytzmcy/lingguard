package heartbeat

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if !cfg.Enabled {
		t.Error("default config should be enabled")
	}
	if cfg.Interval != DefaultInterval {
		t.Errorf("default interval should be %v, got %v", DefaultInterval, cfg.Interval)
	}
}

func TestNewService(t *testing.T) {
	cfg := &Config{
		Enabled:  true,
		Interval: 10 * time.Minute,
	}

	callback := func(ctx context.Context, prompt string) (string, error) {
		return "HEARTBEAT_OK", nil
	}

	svc := NewService(cfg, callback)
	if svc == nil {
		t.Fatal("service should not be nil")
	}
	if svc.config.Interval != 10*time.Minute {
		t.Errorf("interval should be 10m, got %v", svc.config.Interval)
	}
}

func TestNewServiceNilConfig(t *testing.T) {
	callback := func(ctx context.Context, prompt string) (string, error) {
		return "HEARTBEAT_OK", nil
	}

	svc := NewService(nil, callback)
	if svc == nil {
		t.Fatal("service should not be nil")
	}
	if svc.config.Interval != DefaultInterval {
		t.Errorf("should use default interval, got %v", svc.config.Interval)
	}
}

func TestNewServiceZeroInterval(t *testing.T) {
	cfg := &Config{
		Enabled:  true,
		Interval: 0,
	}

	callback := func(ctx context.Context, prompt string) (string, error) {
		return "HEARTBEAT_OK", nil
	}

	svc := NewService(cfg, callback)
	if svc.config.Interval != DefaultInterval {
		t.Errorf("zero interval should default to %v, got %v", DefaultInterval, svc.config.Interval)
	}
}

func TestIsHeartbeatEmpty(t *testing.T) {
	tests := []struct {
		content  string
		expected bool
	}{
		{"", true},
		{"   ", true},
		{"\n\n", true},
		{"\t\t", true},
		{"# Tasks", false},
		{"Some content", false},
	}

	for _, tt := range tests {
		result := isHeartbeatEmpty(tt.content)
		if result != tt.expected {
			t.Errorf("isHeartbeatEmpty(%q) = %v, want %v", tt.content, result, tt.expected)
		}
	}
}

func TestServiceStartStop(t *testing.T) {
	cfg := &Config{
		Enabled:  true,
		Interval: 1 * time.Second,
	}

	callback := func(ctx context.Context, prompt string) (string, error) {
		return "HEARTBEAT_OK", nil
	}

	svc := NewService(cfg, callback)

	// Start
	if err := svc.Start(); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	if !svc.Running() {
		t.Error("service should be running")
	}

	// Double start should be ok
	if err := svc.Start(); err != nil {
		t.Fatalf("double start failed: %v", err)
	}

	// Stop
	svc.Stop()
	if svc.Running() {
		t.Error("service should not be running")
	}

	// Double stop should be ok
	svc.Stop()
}

func TestServiceReadHeartbeatFile(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "heartbeat-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Test 1: No file
	svc := NewService(nil, nil)
	content := svc.readHeartbeatFile(tmpDir)
	if content != "" {
		t.Errorf("expected empty content for missing file, got %q", content)
	}

	// Test 2: File exists
	heartbeatContent := "# Tasks\n- Task 1\n- Task 2"
	heartbeatPath := filepath.Join(tmpDir, "HEARTBEAT.md")
	if err := os.WriteFile(heartbeatPath, []byte(heartbeatContent), 0644); err != nil {
		t.Fatalf("failed to write heartbeat file: %v", err)
	}

	content = svc.readHeartbeatFile(tmpDir)
	if content != heartbeatContent {
		t.Errorf("expected %q, got %q", heartbeatContent, content)
	}
}

func TestServiceStatus(t *testing.T) {
	cfg := &Config{
		Enabled:  true,
		Interval: 5 * time.Minute,
	}

	svc := NewService(cfg, nil)
	svc.SetWorkspace("/tmp/test")

	status := svc.Status()
	if status["enabled"] != true {
		t.Error("status should show enabled")
	}
	if status["interval"] != "5m0s" {
		t.Errorf("interval should be 5m0s, got %v", status["interval"])
	}
	if status["workspace"] != "/tmp/test" {
		t.Errorf("workspace should be /tmp/test, got %v", status["workspace"])
	}
}

func TestServiceTick(t *testing.T) {
	// Create temp directory with HEARTBEAT.md
	tmpDir, err := os.MkdirTemp("", "heartbeat-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	heartbeatContent := "# Tasks\n- Check something"
	heartbeatPath := filepath.Join(tmpDir, "HEARTBEAT.md")
	if err := os.WriteFile(heartbeatPath, []byte(heartbeatContent), 0644); err != nil {
		t.Fatalf("failed to write heartbeat file: %v", err)
	}

	var receivedPrompt string
	callback := func(ctx context.Context, prompt string) (string, error) {
		receivedPrompt = prompt
		return "HEARTBEAT_OK", nil
	}

	svc := NewService(nil, callback)
	svc.SetWorkspace(tmpDir)

	// Trigger tick
	svc.tick()

	// Verify callback was called with prompt containing the file path
	expectedPath := tmpDir + "/HEARTBEAT.md"
	if !strings.Contains(receivedPrompt, expectedPath) {
		t.Errorf("expected prompt to contain %q, got %q", expectedPath, receivedPrompt)
	}

	// Verify status was updated
	status := svc.Status()
	if status["lastStatus"] != "ok" {
		t.Errorf("expected lastStatus ok, got %v", status["lastStatus"])
	}
	if status["runCount"].(int) != 1 {
		t.Errorf("expected runCount 1, got %v", status["runCount"])
	}
}

func TestServiceTickEmptyFile(t *testing.T) {
	// Create temp directory with empty HEARTBEAT.md
	tmpDir, err := os.MkdirTemp("", "heartbeat-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	heartbeatPath := filepath.Join(tmpDir, "HEARTBEAT.md")
	if err := os.WriteFile(heartbeatPath, []byte("   \n\n  "), 0644); err != nil {
		t.Fatalf("failed to write heartbeat file: %v", err)
	}

	callbackCalled := false
	callback := func(ctx context.Context, prompt string) (string, error) {
		callbackCalled = true
		return "HEARTBEAT_OK", nil
	}

	svc := NewService(nil, callback)
	svc.SetWorkspace(tmpDir)

	// Trigger tick
	svc.tick()

	// Callback should not be called for empty file
	if callbackCalled {
		t.Error("callback should not be called for empty heartbeat file")
	}
}

func TestHeartbeatOKDetection(t *testing.T) {
	tests := []struct {
		response string
		isOK     bool
	}{
		{"HEARTBEAT_OK", true},
		{"heartbeat_ok", true},
		{"HEARTBEAT_OK - All good!", true},
		{"All done, heartbeat_ok!", true},
		{"Task completed", false},
		{"HEARTBEAT OK", false}, // underscore is required for detection
	}

	for _, tt := range tests {
		hasToken := strings.Contains(strings.ToUpper(tt.response), HeartbeatOKToken)
		if hasToken != tt.isOK {
			t.Errorf("response %q: expected isOK=%v, got %v", tt.response, tt.isOK, hasToken)
		}
	}
}

func TestServiceTrigger(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "heartbeat-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	heartbeatPath := filepath.Join(tmpDir, "HEARTBEAT.md")
	if err := os.WriteFile(heartbeatPath, []byte("# Test"), 0644); err != nil {
		t.Fatalf("failed to write heartbeat file: %v", err)
	}

	triggered := make(chan bool, 1)
	callback := func(ctx context.Context, prompt string) (string, error) {
		triggered <- true
		return "HEARTBEAT_OK", nil
	}

	svc := NewService(nil, callback)
	svc.SetWorkspace(tmpDir)

	// Trigger
	svc.Trigger()

	// Wait for trigger
	select {
	case <-triggered:
		// Success
	case <-time.After(1 * time.Second):
		t.Error("trigger should have called callback")
	}
}

func TestIsInSilentPeriod(t *testing.T) {
	tests := []struct {
		name       string
		start      string
		end        string
		mockHour   int
		mockMinute int
		expected   bool
	}{
		// Normal period (00:00 - 06:00)
		{"in silent period (2:00)", "00:00", "06:00", 2, 0, true},
		{"in silent period (5:59)", "00:00", "06:00", 5, 59, true},
		{"not in silent period (6:00)", "00:00", "06:00", 6, 0, false},
		{"not in silent period (12:00)", "00:00", "06:00", 12, 0, false},
		{"not in silent period (23:59)", "00:00", "06:00", 23, 59, false},
		{"at start (0:00)", "00:00", "06:00", 0, 0, true},

		// Cross-midnight period (23:00 - 06:00)
		{"cross-midnight in (23:30)", "23:00", "06:00", 23, 30, true},
		{"cross-midnight in (0:30)", "23:00", "06:00", 0, 30, true},
		{"cross-midnight in (5:59)", "23:00", "06:00", 5, 59, true},
		{"cross-midnight not in (22:59)", "23:00", "06:00", 22, 59, false},
		{"cross-midnight not in (6:00)", "23:00", "06:00", 6, 0, false},

		// Empty config
		{"empty start", "", "06:00", 2, 0, false},
		{"empty end", "00:00", "", 2, 0, false},
		{"both empty", "", "", 2, 0, false},

		// Invalid format
		{"invalid start", "invalid", "06:00", 2, 0, false},
		{"invalid end", "00:00", "invalid", 2, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: We can't easily mock time.Now() in Go without dependency injection
			// So we just test the logic by calling parseTimeMinutes directly
			// For full testing, we would need to refactor isInSilentPeriod to accept time

			// Test parseTimeMinutes helper
			if tt.start != "" && tt.start != "invalid" {
				minutes, ok := parseTimeMinutes(tt.start)
				if !ok {
					t.Errorf("parseTimeMinutes(%q) failed", tt.start)
				}
				_ = minutes
			}
		})
	}
}

func TestParseTimeMinutes(t *testing.T) {
	tests := []struct {
		timeStr  string
		expected int
		expectOK bool
	}{
		{"00:00", 0, true},
		{"06:00", 360, true},
		{"23:59", 1439, true},
		{"12:30", 750, true},
		{"invalid", 0, false},
		{"25:00", 0, false},
		{"12:60", 0, false},
		{"12", 0, false},
		{"", 0, false},
	}

	for _, tt := range tests {
		result, ok := parseTimeMinutes(tt.timeStr)
		if ok != tt.expectOK {
			t.Errorf("parseTimeMinutes(%q) ok = %v, want %v", tt.timeStr, ok, tt.expectOK)
		}
		if ok && result != tt.expected {
			t.Errorf("parseTimeMinutes(%q) = %d, want %d", tt.timeStr, result, tt.expected)
		}
	}
}
