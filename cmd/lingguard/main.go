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
		home, _ := os.UserHomeDir()
		configPath = filepath.Join(home, ".lingguard", "config.json")
	}

	if err := cli.Execute(configPath); err != nil {
		os.Exit(1)
	}
}
