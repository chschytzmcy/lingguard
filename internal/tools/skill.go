// Package tools 工具系统
package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/lingguard/internal/skills"
)

// SkillTool 技能加载工具
type SkillTool struct {
	skillsMgr *skills.Manager
}

// NewSkillTool 创建技能工具
func NewSkillTool(mgr *skills.Manager) *SkillTool {
	return &SkillTool{
		skillsMgr: mgr,
	}
}

// Name 返回工具名称
func (t *SkillTool) Name() string {
	return "skill"
}

// Description 返回工具描述
func (t *SkillTool) Description() string {
	return `Load detailed instructions for a specific skill. Use this when you need to:
- Get complete guidance on how to perform a complex task
- Access expert knowledge and best practices
- Follow standardized workflows

Returns the full skill content including usage instructions, examples, and reference information.`
}

// Parameters 返回工具参数定义
func (t *SkillTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Name of the skill to load (e.g., 'git-workflow', 'code-review')",
			},
		},
		"required": []string{"name"},
	}
}

// Execute 执行工具
func (t *SkillTool) Execute(ctx context.Context, argsJSON json.RawMessage) (string, error) {
	if t.skillsMgr == nil {
		return "", fmt.Errorf("skills manager not initialized")
	}

	// 解析参数
	var args struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	if args.Name == "" {
		return "", fmt.Errorf("skill name is required")
	}

	// 获取技能指令
	instruction, err := t.skillsMgr.GetSkillInstruction(args.Name)
	if err != nil {
		return "", fmt.Errorf("failed to load skill '%s': %w", args.Name, err)
	}

	return instruction, nil
}

// IsDangerous 返回是否为危险操作
func (t *SkillTool) IsDangerous() bool {
	return false
}
