// Package tools 不可用工具占位
package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// UnavailableTool 不可用工具（占位提示）
type UnavailableTool struct {
	name        string
	description string
	reason      string
}

// NewUnavailableTool 创建不可用工具
func NewUnavailableTool(name, reason string) *UnavailableTool {
	return &UnavailableTool{
		name:   name,
		reason: reason,
	}
}

func (t *UnavailableTool) Name() string { return t.name }

func (t *UnavailableTool) Description() string {
	return fmt.Sprintf("此工具当前不可用。原因: %s", t.reason)
}

func (t *UnavailableTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}

func (t *UnavailableTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	return "", fmt.Errorf("工具 %s 不可用: %s", t.name, t.reason)
}

func (t *UnavailableTool) IsDangerous() bool         { return false }
func (t *UnavailableTool) ShouldLoadByDefault() bool { return false }
