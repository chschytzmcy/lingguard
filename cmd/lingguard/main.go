package main

import (
	"os"
	"path/filepath"

	"github.com/lingguard/cmd/cli"
)

func main() {
	// 设置配置路径（按优先级）
	configPath := os.Getenv("LINGGUARD_CONFIG")
	if configPath == "" {
		// 1. 优先检查项目目录下的 configs/config.json
		if _, err := os.Stat("configs/config.json"); err == nil {
			configPath = "configs/config.json"
		} else if _, err := os.Stat("config.json"); err == nil {
			// 2. 检查当前工作目录下的 config.json
			configPath = "config.json"
		} else {
			// 3. 默认使用 ~/.lingguard/config.json
			home, _ := os.UserHomeDir()
			configPath = filepath.Join(home, ".lingguard", "config.json")
		}
	}

	if err := cli.Execute(configPath); err != nil {
		os.Exit(1)
	}
}
