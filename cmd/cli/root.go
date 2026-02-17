// Package cli CLI 命令
package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lingguard/internal/config"
	"github.com/lingguard/internal/providers"
	"github.com/lingguard/pkg/logger"
	"github.com/spf13/cobra"
)

var (
	cfgPath string
)

var rootCmd = &cobra.Command{
	Use:   "lingguard",
	Short: "LingGuard - Personal AI Assistant",
	Long:  `A lightweight personal AI assistant written in Go.`,
}

// Execute 执行 CLI
func Execute(configPath string) error {
	cfgPath = configPath

	// 尝试加载配置并初始化日志
	if cfg, err := config.Load(cfgPath); err == nil {
		logger.InitWithConfig(logger.Config{
			Level:      cfg.Logging.Level,
			Format:     cfg.Logging.Format,
			Output:     cfg.Logging.Output,
			MaxSize:    cfg.Logging.MaxSize,
			MaxAge:     cfg.Logging.MaxAge,
			MaxBackups: cfg.Logging.MaxBackups,
			Compress:   cfg.Logging.Compress,
		})
	}

	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(agentCmd)
	rootCmd.AddCommand(statusCmd)
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize configuration",
	Run: func(cmd *cobra.Command, args []string) {
		if err := runInit(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func runInit() error {
	// 创建配置目录
	dir := filepath.Dir(cfgPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// 检查配置文件是否已存在
	if _, err := os.Stat(cfgPath); err == nil {
		fmt.Printf("Config file already exists: %s\n", cfgPath)
		return nil
	}

	// 创建默认配置
	cfg := config.DefaultConfig()
	if err := cfg.Save(cfgPath); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Created config file: %s\n", cfgPath)
	fmt.Println("Please edit the config file to add your API keys.")
	return nil
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status",
	Run: func(cmd *cobra.Command, args []string) {
		runStatus()
	},
}

func runStatus() {
	fmt.Println("LingGuard Status")
	fmt.Println("================")

	// 检查配置文件
	if _, err := os.Stat(cfgPath); err != nil {
		fmt.Printf("Config: Not found (%s)\n", cfgPath)
		fmt.Println("Run 'lingguard init' to create a config file.")
		return
	}

	fmt.Printf("Config: %s\n", cfgPath)

	// 加载配置
	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		return
	}

	// 显示 Provider 信息（使用 ProviderSpec 显示友好名称）
	fmt.Println("\nProviders:")
	for name, pc := range cfg.Providers {
		status := "not configured"
		if pc.APIKey != "" && pc.APIKey != "sk-xxx" && pc.APIKey != "xxx" {
			status = "configured"
		}
		model := pc.Model
		if model == "" {
			// 使用规范中的默认模型
			if spec := providers.FindSpecByName(name); spec != nil {
				model = spec.DefaultModel
			}
			if model == "" {
				model = "default"
			}
		}

		// 获取显示名称
		displayName := name
		if spec := providers.FindSpecByName(name); spec != nil {
			displayName = spec.DisplayName
		}

		fmt.Printf("  - %s (%s): %s [%s]\n", displayName, name, model, status)
	}

	// 显示 Agent 信息
	fmt.Println("\nAgent:")
	providerDisplayName := cfg.Agents.Provider
	if spec := providers.FindSpecByName(cfg.Agents.Provider); spec != nil {
		providerDisplayName = fmt.Sprintf("%s (%s)", spec.DisplayName, cfg.Agents.Provider)
	}
	fmt.Printf("  Provider: %s\n", providerDisplayName)
	fmt.Printf("  Workspace: %s\n", cfg.Agents.Workspace)
	fmt.Printf("  Max Iterations: %d\n", cfg.Agents.MaxToolIterations)
	fmt.Printf("  Memory Window: %d\n", cfg.Agents.MemoryWindow)

	// 显示渠道信息
	fmt.Println("\nChannels:")
	if cfg.Channels.Feishu != nil {
		status := "disabled"
		if cfg.Channels.Feishu.Enabled {
			status = "enabled"
		}
		fmt.Printf("  - Feishu: %s\n", status)
	} else {
		fmt.Println("  - Feishu: not configured")
	}
}
