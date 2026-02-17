package utils

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestExpandHome(t *testing.T) {
	home, _ := os.UserHomeDir()

	tests := []struct {
		input    string
		expected string
	}{
		{"~", home},
		{"~/test", filepath.Join(home, "test")},
		{"~/path/to/file", filepath.Join(home, "path/to/file")},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
		{"", ""},
	}

	for _, tt := range tests {
		result := ExpandHome(tt.input)
		if result != tt.expected {
			t.Errorf("ExpandHome(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestParseTime(t *testing.T) {
	tests := []struct {
		input    string
		hasError bool
	}{
		{"2024-12-25 09:00:00", false},
		{"2024-12-25 09:00", false},
		{"2024-12-25", false},
		{"2024-12-25T09:00:00Z", false},
		{"in 5m", false},
		{"in 1h30m", false},
		{"+5m", false},
		{"+1h", false},
		{"  in 5m  ", false}, // with spaces
		{"invalid", true},
		{"", true},
	}

	for _, tt := range tests {
		_, err := ParseTime(tt.input)
		if tt.hasError && err == nil {
			t.Errorf("ParseTime(%q) expected error, got nil", tt.input)
		}
		if !tt.hasError && err != nil {
			t.Errorf("ParseTime(%q) unexpected error: %v", tt.input, err)
		}
	}
}

func TestParseTimeFormats(t *testing.T) {
	// Test datetime format
	t1, err := ParseTime("2024-12-25 09:00:00")
	if err != nil {
		t.Fatalf("ParseTime failed: %v", err)
	}
	if t1.Year() != 2024 || t1.Month() != 12 || t1.Day() != 25 {
		t.Errorf("Unexpected date: %v", t1)
	}

	// Test date only format
	t2, err := ParseTime("2024-06-15")
	if err != nil {
		t.Fatalf("ParseTime failed: %v", err)
	}
	if t2.Year() != 2024 || t2.Month() != 6 || t2.Day() != 15 {
		t.Errorf("Unexpected date: %v", t2)
	}
}

func TestParseTimeRelative(t *testing.T) {
	now := time.Now()

	// Test "in 5m"
	t1, err := ParseTime("in 5m")
	if err != nil {
		t.Fatalf("ParseTime(in 5m) failed: %v", err)
	}
	expected1 := now.Add(5 * time.Minute)
	diff1 := t1.Sub(expected1)
	if diff1 < -time.Second || diff1 > time.Second {
		t.Errorf("ParseTime(in 5m) = %v, expected around %v (diff: %v)", t1, expected1, diff1)
	}

	// Test "+1h"
	t2, err := ParseTime("+1h")
	if err != nil {
		t.Fatalf("ParseTime(+1h) failed: %v", err)
	}
	expected2 := now.Add(1 * time.Hour)
	diff2 := t2.Sub(expected2)
	if diff2 < -time.Second || diff2 > time.Second {
		t.Errorf("ParseTime(+1h) = %v, expected around %v (diff: %v)", t2, expected2, diff2)
	}

	// Test "in 1h30m"
	t3, err := ParseTime("in 1h30m")
	if err != nil {
		t.Fatalf("ParseTime(in 1h30m) failed: %v", err)
	}
	expected3 := now.Add(90 * time.Minute)
	diff3 := t3.Sub(expected3)
	if diff3 < -time.Second || diff3 > time.Second {
		t.Errorf("ParseTime(in 1h30m) = %v, expected around %v (diff: %v)", t3, expected3, diff3)
	}
}

func TestFormatTime(t *testing.T) {
	tests := []struct {
		ms       int64
		expected string
	}{
		{0, ""},
		{1703491200000, "2023-12-25 08:00:00"}, // Approximate, depends on timezone
	}

	for _, tt := range tests {
		result := FormatTime(tt.ms)
		if tt.ms == 0 && result != "" {
			t.Errorf("FormatTime(0) = %q, want empty", result)
		}
		if tt.ms != 0 && result == "" {
			t.Errorf("FormatTime(%d) = empty, want non-empty", tt.ms)
		}
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"exactly10!", 10, "exactly10!"},
		{"too long string", 10, "too long s..."},
		{"", 5, ""},
		{"abc", 3, "abc"},
	}

	for _, tt := range tests {
		result := TruncateString(tt.input, tt.maxLen)
		if result != tt.expected {
			t.Errorf("TruncateString(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.expected)
		}
	}
}

func TestExpandHomeEdgeCases(t *testing.T) {
	// Test with only tilde
	result := ExpandHome("~")
	if len(result) == 0 {
		t.Error("ExpandHome('~') should return home directory")
	}

	// Test without tilde
	result = ExpandHome("/usr/local")
	if result != "/usr/local" {
		t.Errorf("ExpandHome('/usr/local') = %q, want '/usr/local'", result)
	}
}

func BenchmarkExpandHome(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ExpandHome("~/test/path")
	}
}

func BenchmarkParseTime(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ParseTime("2024-12-25 09:00:00")
	}
}

func BenchmarkTruncateString(b *testing.B) {
	longStr := "This is a very long string that needs to be truncated"
	for i := 0; i < b.N; i++ {
		TruncateString(longStr, 20)
	}
}
