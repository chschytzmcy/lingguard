// Package skills 技能系统
package skills

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/lingguard/pkg/logger"
)

// Skill 技能定义
type Skill struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Homepage    string                 `json:"homepage,omitempty"`
	Emoji       string                 `json:"emoji,omitempty"`
	Content     string                 `json:"content,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Requires    *Requirements          `json:"requires,omitempty"`
	Path        string                 `json:"path,omitempty"`
	Available   bool                   `json:"available"`
	Unavailable string                 `json:"unavailable,omitempty"`
}

// Requirements 技能依赖要求
type Requirements struct {
	Bins []string `json:"bins,omitempty"`
	Env  []string `json:"env,omitempty"`
}

// Loader 技能加载器
type Loader struct {
	builtinDir string
	workspace  string
}

// NewLoader 创建技能加载器
func NewLoader(builtinDir, workspace string) *Loader {
	return &Loader{
		builtinDir: builtinDir,
		workspace:  workspace,
	}
}

// ListSkills 列出所有可用技能
func (l *Loader) ListSkills() ([]*Skill, error) {
	skills := make([]*Skill, 0)

	// 加载内置技能
	if l.builtinDir != "" {
		builtinSkills, err := l.loadFromDir(l.builtinDir)
		if err != nil {
			logger.Warn("failed to load builtin skills", "error", err)
		}
		skills = append(skills, builtinSkills...)
	}

	// 加载工作区技能
	if l.workspace != "" {
		workspaceSkills, err := l.loadFromDir(l.workspace)
		if err != nil {
			logger.Warn("failed to load workspace skills", "error", err)
		}
		skills = append(skills, workspaceSkills...)
	}

	return skills, nil
}

// LoadSkill 加载指定技能的完整内容
func (l *Loader) LoadSkill(name string) (*Skill, error) {
	// 先在 builtin 目录查找
	if l.builtinDir != "" {
		skill, err := l.loadSkillByName(l.builtinDir, name)
		if err == nil {
			return skill, nil
		}
	}

	// 再在 workspace 目录查找
	if l.workspace != "" {
		skill, err := l.loadSkillByName(l.workspace, name)
		if err == nil {
			return skill, nil
		}
	}

	return nil, fmt.Errorf("skill not found: %s", name)
}

// loadSkillByName 从指定目录加载技能
func (l *Loader) loadSkillByName(dir, name string) (*Skill, error) {
	skillPath := filepath.Join(dir, name, "SKILL.md")
	content, err := os.ReadFile(skillPath)
	if err != nil {
		return nil, err
	}

	skill, err := parseSkill(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse skill %s: %w", name, err)
	}

	skill.Path = skillPath
	// Content 已在 parseSkill 中设置为去掉 frontmatter 的正文
	skill.Available, skill.Unavailable = l.checkRequirements(skill.Requires)

	return skill, nil
}

// loadFromDir 从目录加载所有技能（仅元数据，不加载内容）
func (l *Loader) loadFromDir(dir string) ([]*Skill, error) {
	skills := make([]*Skill, 0)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillPath := filepath.Join(dir, entry.Name(), "SKILL.md")
		content, err := os.ReadFile(skillPath)
		if err != nil {
			logger.Debug("skipping skill directory, no SKILL.md", "dir", entry.Name())
			continue
		}

		skill, err := parseSkill(content)
		if err != nil {
			logger.Warn("failed to parse skill", "name", entry.Name(), "error", err)
			continue
		}

		skill.Path = skillPath
		skill.Available, skill.Unavailable = l.checkRequirements(skill.Requires)
		skills = append(skills, skill)
	}

	return skills, nil
}

// parseSkill 解析 SKILL.md 文件
func parseSkill(content []byte) (*Skill, error) {
	skill := &Skill{
		Metadata: make(map[string]interface{}),
	}

	// 解析 YAML frontmatter
	text := string(content)

	// 检查是否有 frontmatter
	if !strings.HasPrefix(text, "---") {
		return nil, fmt.Errorf("skill file must start with YAML frontmatter")
	}

	// 找到 frontmatter 结束位置
	endIndex := bytes.Index(content[3:], []byte("---"))
	if endIndex == -1 {
		return nil, fmt.Errorf("invalid frontmatter: missing closing ---")
	}

	frontmatter := content[4 : endIndex+3]
	body := content[endIndex+6:]

	// 解析 frontmatter
	lines := strings.Split(string(frontmatter), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "name":
			skill.Name = value
		case "description":
			skill.Description = value
		case "homepage":
			skill.Homepage = value
		case "metadata":
			// 解析 JSON metadata
			var metadata struct {
				Nanobot struct {
					Emoji    string        `json:"emoji"`
					Requires *Requirements `json:"requires"`
				} `json:"nanobot"`
			}
			if err := json.Unmarshal([]byte(value), &metadata); err == nil {
				skill.Emoji = metadata.Nanobot.Emoji
				skill.Requires = metadata.Nanobot.Requires
			}
		}
	}

	// 存储完整内容
	skill.Content = string(body)

	return skill, nil
}

// checkRequirements 检查技能依赖是否满足
func (l *Loader) checkRequirements(req *Requirements) (bool, string) {
	if req == nil {
		return true, ""
	}

	// 检查二进制依赖
	for _, bin := range req.Bins {
		if _, err := exec.LookPath(bin); err != nil {
			return false, fmt.Sprintf("missing binary: %s", bin)
		}
	}

	// 检查环境变量依赖
	for _, env := range req.Env {
		if os.Getenv(env) == "" {
			return false, fmt.Sprintf("missing environment variable: %s", env)
		}
	}

	return true, ""
}

// BuildSkillsSummary 构建技能摘要（用于注入到系统提示）
func (l *Loader) BuildSkillsSummary() string {
	skills, err := l.ListSkills()
	if err != nil || len(skills) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("<skills>\n")

	for _, skill := range skills {
		sb.WriteString(fmt.Sprintf("  <skill name=\"%s\"", skill.Name))
		if skill.Emoji != "" {
			sb.WriteString(fmt.Sprintf(" emoji=\"%s\"", skill.Emoji))
		}
		sb.WriteString(">\n")
		sb.WriteString(fmt.Sprintf("    <description>%s</description>\n", skill.Description))
		if !skill.Available {
			sb.WriteString(fmt.Sprintf("    <unavailable>%s</unavailable>\n", skill.Unavailable))
		}
		sb.WriteString("  </skill>\n")
	}

	sb.WriteString("</skills>")

	return sb.String()
}

// GetAvailableSkills 获取可用的技能列表
func (l *Loader) GetAvailableSkills() ([]*Skill, error) {
	skills, err := l.ListSkills()
	if err != nil {
		return nil, err
	}

	available := make([]*Skill, 0)
	for _, s := range skills {
		if s.Available {
			available = append(available, s)
		}
	}

	return available, nil
}
