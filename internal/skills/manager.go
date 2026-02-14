// Package skills 技能系统
package skills

import (
	"fmt"
	"strings"
	"sync"

	"github.com/lingguard/pkg/logger"
)

// Manager 技能管理器
type Manager struct {
	loader *Loader
	cache  map[string]*Skill
	mu     sync.RWMutex
}

// NewManager 创建技能管理器
func NewManager(loader *Loader) *Manager {
	return &Manager{
		loader: loader,
		cache:  make(map[string]*Skill),
	}
}

// GetSkill 获取技能（优先从缓存）
func (m *Manager) GetSkill(name string) (*Skill, error) {
	m.mu.RLock()
	if skill, ok := m.cache[name]; ok {
		m.mu.RUnlock()
		return skill, nil
	}
	m.mu.RUnlock()

	// 加载技能
	skill, err := m.loader.LoadSkill(name)
	if err != nil {
		return nil, err
	}

	// 缓存技能
	m.mu.Lock()
	m.cache[name] = skill
	m.mu.Unlock()

	return skill, nil
}

// ListSkills 列出所有技能
func (m *Manager) ListSkills() ([]*Skill, error) {
	return m.loader.ListSkills()
}

// GetSkillsContext 获取技能上下文（用于注入到系统提示）
func (m *Manager) GetSkillsContext() string {
	return m.loader.BuildSkillsSummary()
}

// GetSkillInstruction 获取技能的详细指令
func (m *Manager) GetSkillInstruction(name string) (string, error) {
	skill, err := m.GetSkill(name)
	if err != nil {
		return "", err
	}

	if !skill.Available {
		return "", fmt.Errorf("skill %s is not available: %s", name, skill.Unavailable)
	}

	// 构建技能指令
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Skill: %s\n\n", skill.Name))
	sb.WriteString(fmt.Sprintf("%s\n", skill.Description))

	if skill.Homepage != "" {
		sb.WriteString(fmt.Sprintf("\nHomepage: %s\n", skill.Homepage))
	}

	sb.WriteString(fmt.Sprintf("\n## Instructions\n\n%s", skill.Content))

	return sb.String(), nil
}

// RefreshSkills 刷新技能缓存
func (m *Manager) RefreshSkills() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 清空缓存
	m.cache = make(map[string]*Skill)

	logger.Info("skills cache refreshed")
	return nil
}

// FindMatchingSkill 根据关键词查找匹配的技能
func (m *Manager) FindMatchingSkill(query string) (*Skill, error) {
	skills, err := m.ListSkills()
	if err != nil {
		return nil, err
	}

	query = strings.ToLower(query)

	for _, skill := range skills {
		if strings.Contains(strings.ToLower(skill.Name), query) ||
			strings.Contains(strings.ToLower(skill.Description), query) {
			if skill.Available {
				return skill, nil
			}
		}
	}

	return nil, fmt.Errorf("no matching skill found for: %s", query)
}

// GetAvailableSkillNames 获取所有可用技能的名称
func (m *Manager) GetAvailableSkillNames() []string {
	skills, err := m.ListSkills()
	if err != nil {
		return nil
	}

	names := make([]string, 0)
	for _, s := range skills {
		if s.Available {
			names = append(names, s.Name)
		}
	}

	return names
}
