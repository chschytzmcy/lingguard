package main

import (
	"os"
	"path/filepath"

	"github.com/lingguard/cmd/cli"
)

func main() {
	// 设置默认配置路径
	configPath := os.Getenv("LINGGUARD_CONFIG")
	if configPath == "" {
		// 优先从本地 configs 目录加载
		localConfig := filepath.Join("configs", "config.json")
		if _, err := os.Stat(localConfig); err == nil {
			configPath = localConfig
		} else {
			// 如果本地不存在，从用户主目录加载
			home, _ := os.UserHomeDir()
			configPath = filepath.Join(home, ".lingguard", "config.json")
		}
	}

	if err := cli.Execute(configPath); err != nil {
		os.Exit(1)
	}
}
