package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseSkill(t *testing.T) {
	content := `---
name: weather
description: Get current weather and forecasts (no API key required).
homepage: https://wttr.in/:help
metadata: {"nanobot":{"emoji":"🌤️","requires":{"bins":["curl"]}}}
---
# Weather

Two free services, no API keys needed.
`

	skill, err := parseSkill([]byte(content))
	if err != nil {
		t.Fatalf("parseSkill failed: %v", err)
	}

	if skill.Name != "weather" {
		t.Errorf("Expected name=weather, got %s", skill.Name)
	}

	if skill.Description != "Get current weather and forecasts (no API key required)." {
		t.Errorf("Unexpected description: %s", skill.Description)
	}

	if skill.Homepage != "https://wttr.in/:help" {
		t.Errorf("Unexpected homepage: %s", skill.Homepage)
	}

	if skill.Emoji != "🌤️" {
		t.Errorf("Expected emoji=🌤️, got %s", skill.Emoji)
	}

	if skill.Requires == nil || len(skill.Requires.Bins) != 1 || skill.Requires.Bins[0] != "curl" {
		t.Errorf("Expected requires.bins=[curl], got %v", skill.Requires)
	}
}

func TestParseSkillNoFrontmatter(t *testing.T) {
	content := `# Weather
No frontmatter here.`

	_, err := parseSkill([]byte(content))
	if err == nil {
		t.Error("Expected error for missing frontmatter")
	}
}

func TestLoaderListSkills(t *testing.T) {
	// 创建临时目录
	tmpDir := t.TempDir()

	// 创建测试技能目录
	skillDir := filepath.Join(tmpDir, "test-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill dir: %v", err)
	}

	// 创建 SKILL.md 文件
	skillContent := `---
name: test-skill
description: A test skill
---
# Test Skill
This is a test.
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}

	loader := NewLoader(tmpDir, "")
	skills, err := loader.ListSkills()
	if err != nil {
		t.Fatalf("ListSkills failed: %v", err)
	}

	if len(skills) != 1 {
		t.Errorf("Expected 1 skill, got %d", len(skills))
	}

	if skills[0].Name != "test-skill" {
		t.Errorf("Expected name=test-skill, got %s", skills[0].Name)
	}
}

func TestLoaderLoadSkill(t *testing.T) {
	tmpDir := t.TempDir()

	skillDir := filepath.Join(tmpDir, "my-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill dir: %v", err)
	}

	skillContent := `---
name: my-skill
description: My custom skill
---
# My Skill
Detailed instructions here.
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}

	loader := NewLoader(tmpDir, "")
	skill, err := loader.LoadSkill("my-skill")
	if err != nil {
		t.Fatalf("LoadSkill failed: %v", err)
	}

	if skill.Name != "my-skill" {
		t.Errorf("Expected name=my-skill, got %s", skill.Name)
	}

	if skill.Content == "" {
		t.Error("Skill content should not be empty")
	}
}

func TestLoaderLoadSkillNotFound(t *testing.T) {
	loader := NewLoader(t.TempDir(), "")
	_, err := loader.LoadSkill("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent skill")
	}
}

func TestBuildSkillsSummary(t *testing.T) {
	tmpDir := t.TempDir()

	skillDir := filepath.Join(tmpDir, "weather")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill dir: %v", err)
	}

	skillContent := `---
name: weather
description: Get weather info
metadata: {"nanobot":{"emoji":"🌤️"}}
---
# Weather
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}

	loader := NewLoader(tmpDir, "")
	summary := loader.BuildSkillsSummary()

	if summary == "" {
		t.Error("Expected non-empty summary")
	}

	// 验证 XML 格式
	if !contains(summary, `<skills>`) || !contains(summary, `</skills>`) {
		t.Errorf("Summary should contain <skills> tags: %s", summary)
	}

	if !contains(summary, `name="weather"`) {
		t.Errorf("Summary should contain skill name: %s", summary)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
