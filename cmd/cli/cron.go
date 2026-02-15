package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/lingguard/internal/config"
	"github.com/lingguard/internal/cron"
	"github.com/spf13/cobra"
)

var cronCmd = &cobra.Command{
	Use:   "cron",
	Short: "Manage scheduled tasks",
	Long:  `Manage scheduled tasks (cron jobs) for automated message delivery.`,
}

var cronListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all scheduled tasks",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, service, err := initCronService()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		defer service.Stop()

		includeDisabled, _ := cmd.Flags().GetBool("all")
		jobs := service.ListJobs(includeDisabled)

		if len(jobs) == 0 {
			fmt.Println("No scheduled tasks found.")
			return
		}

		fmt.Println("Scheduled Tasks:")
		fmt.Println("─────────────────────────────────────────────────────────────────────────")
		for _, job := range jobs {
			status := "✓"
			if !job.Enabled {
				status = "✗"
			}
			nextRun := formatNextRun(job.State.NextRunAtMs)
			lastRun := formatLastRun(job.State.LastRunAtMs, job.State.LastStatus)

			fmt.Printf("ID: %s | Status: %s | Name: %s\n", job.ID, status, job.Name)
			fmt.Printf("    Schedule: %s | Next: %s\n", formatSchedule(job.Schedule), nextRun)
			fmt.Printf("    Message: %s\n", truncateString(job.Payload.Message, 50))
			if lastRun != "" {
				fmt.Printf("    Last Run: %s\n", lastRun)
			}
			fmt.Println("─────────────────────────────────────────────────────────────────────────")
		}

		_ = cfg
	},
}

var cronAddCmd = &cobra.Command{
	Use:   "add <name> <schedule> <message>",
	Short: "Add a new scheduled task",
	Long: `Add a new scheduled task.

Schedule formats:
  - every:<duration>  - Repeat every duration (e.g., "every:1h", "every:30m")
  - at:<datetime>     - Run once at specific time (e.g., "at:2024-12-25 09:00")
  - cron:<expr>       - Cron expression (e.g., "cron:0 9 * * *")`,
	Args: cobra.ExactArgs(3),
	Run: func(cmd *cobra.Command, args []string) {
		_, service, err := initCronService()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		defer service.Stop()

		name := args[0]
		scheduleStr := args[1]
		message := args[2]

		schedule, err := parseSchedule(scheduleStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing schedule: %v\n", err)
			os.Exit(1)
		}

		var opts []cron.JobOption

		// 处理投递选项
		deliver, _ := cmd.Flags().GetBool("deliver")
		if deliver {
			channel, _ := cmd.Flags().GetString("channel")
			to, _ := cmd.Flags().GetString("to")
			opts = append(opts, cron.WithDeliver(channel, to))
		}

		// 处理一次性任务选项
		deleteAfter, _ := cmd.Flags().GetBool("delete-after")
		if deleteAfter {
			opts = append(opts, cron.WithDeleteAfterRun())
		}

		job, err := service.AddJob(name, *schedule, message, opts...)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error adding job: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Task added successfully!\n")
		fmt.Printf("  ID: %s\n", job.ID)
		fmt.Printf("  Name: %s\n", job.Name)
		fmt.Printf("  Schedule: %s\n", formatSchedule(job.Schedule))
		fmt.Printf("  Next Run: %s\n", formatNextRun(job.State.NextRunAtMs))
	},
}

var cronRemoveCmd = &cobra.Command{
	Use:   "remove <job-id>",
	Short: "Remove a scheduled task",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		_, service, err := initCronService()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		defer service.Stop()

		jobID := args[0]
		if service.RemoveJob(jobID) {
			fmt.Printf("Task %s removed successfully.\n", jobID)
		} else {
			fmt.Fprintf(os.Stderr, "Task %s not found.\n", jobID)
			os.Exit(1)
		}
	},
}

var cronEnableCmd = &cobra.Command{
	Use:   "enable <job-id>",
	Short: "Enable a scheduled task",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		_, service, err := initCronService()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		defer service.Stop()

		jobID := args[0]
		job := service.EnableJob(jobID, true)
		if job != nil {
			fmt.Printf("Task %s enabled.\n", jobID)
			fmt.Printf("  Next Run: %s\n", formatNextRun(job.State.NextRunAtMs))
		} else {
			fmt.Fprintf(os.Stderr, "Task %s not found.\n", jobID)
			os.Exit(1)
		}
	},
}

var cronDisableCmd = &cobra.Command{
	Use:   "disable <job-id>",
	Short: "Disable a scheduled task",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		_, service, err := initCronService()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		defer service.Stop()

		jobID := args[0]
		job := service.EnableJob(jobID, false)
		if job != nil {
			fmt.Printf("Task %s disabled.\n", jobID)
		} else {
			fmt.Fprintf(os.Stderr, "Task %s not found.\n", jobID)
			os.Exit(1)
		}
	},
}

var cronRunCmd = &cobra.Command{
	Use:   "run <job-id>",
	Short: "Manually run a scheduled task",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		_, service, err := initCronService()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		defer service.Stop()

		jobID := args[0]
		force, _ := cmd.Flags().GetBool("force")

		job, err := service.RunJob(jobID, force)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error running task: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Task %s executed.\n", jobID)
		fmt.Printf("  Status: %s\n", job.State.LastStatus)
		if job.State.LastError != "" {
			fmt.Printf("  Error: %s\n", job.State.LastError)
		}
		if job.State.LastResponse != "" {
			fmt.Printf("  Response: %s\n", truncateString(job.State.LastResponse, 200))
		}
	},
}

var cronStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show cron service status",
	Run: func(cmd *cobra.Command, args []string) {
		_, service, err := initCronService()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		defer service.Stop()

		status := service.Status()
		data, _ := json.MarshalIndent(status, "", "  ")
		fmt.Println(string(data))
	},
}

func init() {
	rootCmd.AddCommand(cronCmd)
	cronCmd.AddCommand(cronListCmd)
	cronCmd.AddCommand(cronAddCmd)
	cronCmd.AddCommand(cronRemoveCmd)
	cronCmd.AddCommand(cronEnableCmd)
	cronCmd.AddCommand(cronDisableCmd)
	cronCmd.AddCommand(cronRunCmd)
	cronCmd.AddCommand(cronStatusCmd)

	// cron list flags
	cronListCmd.Flags().BoolP("all", "a", false, "Include disabled tasks")

	// cron add flags
	cronAddCmd.Flags().BoolP("deliver", "d", false, "Deliver response to channel")
	cronAddCmd.Flags().StringP("channel", "c", "", "Target channel (e.g., feishu)")
	cronAddCmd.Flags().StringP("to", "t", "", "Target user/group ID")
	cronAddCmd.Flags().BoolP("delete-after", "", false, "Delete after execution (for one-shot tasks)")

	// cron run flags
	cronRunCmd.Flags().BoolP("force", "f", false, "Force run even if disabled")
}

// initCronService 初始化定时任务服务
func initCronService() (*config.Config, *cron.Service, error) {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, nil, fmt.Errorf("load config: %w", err)
	}

	// 获取存储路径
	storePath := expandTilde("~/.lingguard/cron/jobs.json")
	if cfg.Cron != nil && cfg.Cron.StorePath != "" {
		storePath = expandTilde(cfg.Cron.StorePath)
	}

	// 创建服务（不设置回调，CLI 模式下手动执行）
	service := cron.NewService(storePath, nil)

	if err := service.Start(); err != nil {
		return nil, nil, fmt.Errorf("start cron service: %w", err)
	}

	return cfg, service, nil
}

// parseSchedule 解析调度字符串
func parseSchedule(s string) (*cron.CronSchedule, error) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid schedule format, use: every:<duration>, at:<datetime>, or cron:<expr>")
	}

	kind := strings.ToLower(parts[0])
	value := parts[1]

	switch kind {
	case "every":
		duration, err := time.ParseDuration(value)
		if err != nil {
			return nil, fmt.Errorf("invalid duration: %w", err)
		}
		return &cron.CronSchedule{
			Kind:    cron.ScheduleKindEvery,
			EveryMs: duration.Milliseconds(),
		}, nil

	case "at":
		// 支持多种时间格式
		t, err := parseTime(value)
		if err != nil {
			return nil, fmt.Errorf("invalid datetime: %w", err)
		}
		return &cron.CronSchedule{
			Kind: cron.ScheduleKindAt,
			AtMs: t.UnixMilli(),
		}, nil

	case "cron":
		return &cron.CronSchedule{
			Kind: cron.ScheduleKindCron,
			Expr: value,
		}, nil

	default:
		return nil, fmt.Errorf("unknown schedule kind: %s", kind)
	}
}

// parseTime 解析时间字符串
func parseTime(s string) (time.Time, error) {
	formats := []string{
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
		time.RFC3339,
	}

	for _, format := range formats {
		if t, err := time.ParseInLocation(format, s, time.Local); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("cannot parse time: %s", s)
}

// formatSchedule 格式化调度信息
func formatSchedule(s cron.CronSchedule) string {
	switch s.Kind {
	case cron.ScheduleKindEvery:
		return fmt.Sprintf("every %s", time.Duration(s.EveryMs)*time.Millisecond)
	case cron.ScheduleKindAt:
		return fmt.Sprintf("at %s", time.UnixMilli(s.AtMs).Format("2006-01-02 15:04:05"))
	case cron.ScheduleKindCron:
		return fmt.Sprintf("cron: %s", s.Expr)
	default:
		return string(s.Kind)
	}
}

// formatNextRun 格式化下次执行时间
func formatNextRun(ms int64) string {
	if ms == 0 {
		return "not scheduled"
	}
	t := time.UnixMilli(ms)
	return t.Format("2006-01-02 15:04:05")
}

// formatLastRun 格式化上次执行信息
func formatLastRun(ms int64, status cron.JobStatus) string {
	if ms == 0 {
		return ""
	}
	t := time.UnixMilli(ms)
	return fmt.Sprintf("%s (%s)", t.Format("2006-01-02 15:04:05"), status)
}

// truncateString 截断字符串
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// expandTilde 展开 ~ 为用户主目录
func expandTilde(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, _ := os.UserHomeDir()
		return home + path[1:]
	}
	return path
}

// 实现 strconv 包的引用（避免未使用导入）
var _ = strconv.Itoa
