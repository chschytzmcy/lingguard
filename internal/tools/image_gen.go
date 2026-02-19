// Package tools 工具实现 - 图像/视频生成工具
package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// ImageGenTool 图像/视频生成工具
type ImageGenTool struct {
	apiKey    string
	apiBase   string
	model     string
	outputDir string
}

// ImageGenConfig 图像生成配置
type ImageGenConfig struct {
	APIKey    string
	APIBase   string
	Model     string
	OutputDir string
}

// DefaultImageGenConfig 默认配置
func DefaultImageGenConfig() *ImageGenConfig {
	home, _ := os.UserHomeDir()
	return &ImageGenConfig{
		APIBase:   "https://dashscope.aliyuncs.com/api/v1/services/aigc",
		Model:     "wanx2.1-t2i-turbo", // 通义万相默认模型
		OutputDir: filepath.Join(home, ".lingguard", "workspace", "generated"),
	}
}

// NewImageGenTool 创建图像生成工具
func NewImageGenTool(cfg *ImageGenConfig) *ImageGenTool {
	if cfg.APIBase == "" {
		cfg.APIBase = "https://dashscope.aliyuncs.com/api/v1/services/aigc"
	}
	if cfg.Model == "" {
		cfg.Model = "wanx2.1-t2i-turbo"
	}
	if cfg.OutputDir == "" {
		home, _ := os.UserHomeDir()
		cfg.OutputDir = filepath.Join(home, ".lingguard", "generated")
	}

	return &ImageGenTool{
		apiKey:    cfg.APIKey,
		apiBase:   cfg.APIBase,
		model:     cfg.Model,
		outputDir: cfg.OutputDir,
	}
}

// Name 返回工具名称
func (t *ImageGenTool) Name() string {
	return "image_gen"
}

// Description 返回工具描述
func (t *ImageGenTool) Description() string {
	return `Image and video generation tool using Alibaba Cloud Tongyi Wanxiang.

Actions:
- generate_image: Generate an image from text description
- generate_video: Generate a video from text description

Usage:
{"action": "generate_image", "prompt": "A cute cat sitting on a chair", "size": "1024x1024"}
{"action": "generate_video", "prompt": "A cat walking in a garden", "duration": 4}

Available image models:
- wanx2.1-t2i-turbo: Fast generation, good quality (default)
- wanx2.1-t2i-plus: Higher quality, slower
- wanx-v1: Legacy model

Video generation:
- Default duration: 4 seconds
- Max duration: 10 seconds`
}

// Parameters 返回参数定义
func (t *ImageGenTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"generate_image", "generate_video"},
				"description": "The generation action to perform",
			},
			"prompt": map[string]interface{}{
				"type":        "string",
				"description": "Text description of the image or video to generate",
			},
			"model": map[string]interface{}{
				"type":        "string",
				"description": "Model to use (optional, defaults to wanx2.1-t2i-turbo)",
			},
			"size": map[string]interface{}{
				"type":        "string",
				"description": "Image size for generation (e.g., 1024x1024, 720x1280)",
			},
			"duration": map[string]interface{}{
				"type":        "integer",
				"description": "Video duration in seconds (default: 4, max: 10)",
			},
			"style": map[string]interface{}{
				"type":        "string",
				"description": "Style preset (optional, e.g., 'anime', 'realistic', '3d')",
			},
		},
		"required": []string{"action", "prompt"},
	}
}

// Execute 执行工具
func (t *ImageGenTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	if t.apiKey == "" {
		return "", fmt.Errorf("image generation API key not configured")
	}

	var params struct {
		Action   string `json:"action"`
		Prompt   string `json:"prompt"`
		Model    string `json:"model"`
		Size     string `json:"size"`
		Duration int    `json:"duration"`
		Style    string `json:"style"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}

	switch params.Action {
	case "generate_image":
		return t.generateImage(ctx, params.Prompt, params.Model, params.Size, params.Style)
	case "generate_video":
		return t.generateVideo(ctx, params.Prompt, params.Duration)
	default:
		return "", fmt.Errorf("unknown action: %s", params.Action)
	}
}

// generateImage 生成图片
func (t *ImageGenTool) generateImage(ctx context.Context, prompt, model, size, style string) (string, error) {
	if prompt == "" {
		return "", fmt.Errorf("prompt is required")
	}

	if model == "" {
		model = t.model
	}

	// 构建请求
	parameters := map[string]interface{}{
		"n": 1,
	}

	// 注意: wanx2.1 系列模型不支持 size 参数，使用模型默认尺寸
	// size 参数已弃用，忽略用户传入的 size 值

	if style != "" {
		parameters["style"] = style
	}

	reqBody := map[string]interface{}{
		"model": model,
		"input": map[string]string{
			"prompt": prompt,
		},
		"parameters": parameters,
	}

	// 调用 API
	result, err := t.callImageAPI(ctx, reqBody)
	if err != nil {
		return "", err
	}

	// 下载并保存图片
	if len(result.Output.Results) == 0 {
		return "", fmt.Errorf("no image generated")
	}

	// 确保输出目录存在
	if err := os.MkdirAll(t.outputDir, 0755); err != nil {
		return "", fmt.Errorf("create output directory: %w", err)
	}

	// 下载图片
	imageURL := result.Output.Results[0].URL
	localPath, err := t.downloadFile(ctx, imageURL, "image", ".png")
	if err != nil {
		return "", fmt.Errorf("download image: %w", err)
	}

	// 返回特殊格式，让飞书 channel 自动发送图片
	return fmt.Sprintf("图片生成成功！\n描述: %s\n\n[GENERATED_IMAGE:%s]", prompt, localPath), nil
}

// generateVideo 生成视频
func (t *ImageGenTool) generateVideo(ctx context.Context, prompt string, duration int) (string, error) {
	if prompt == "" {
		return "", fmt.Errorf("prompt is required")
	}

	if duration <= 0 {
		duration = 4
	}
	if duration > 10 {
		duration = 10
	}

	// 构建请求 - 使用 multimodal-generation API
	reqBody := map[string]interface{}{
		"model": "wanx2.1-t2v-plus",
		"input": map[string]interface{}{
			"prompt": prompt,
		},
		"parameters": map[string]interface{}{},
	}

	// 调用视频生成 API（异步）
	taskID, err := t.submitVideoTask(ctx, reqBody)
	if err != nil {
		return "", err
	}

	// 等待生成完成
	result, err := t.waitForVideoResult(ctx, taskID)
	if err != nil {
		return "", err
	}

	// 下载视频
	if result.Output.VideoURL == "" {
		return "", fmt.Errorf("no video URL in result")
	}

	// 确保输出目录存在
	if err := os.MkdirAll(t.outputDir, 0755); err != nil {
		return "", fmt.Errorf("create output directory: %w", err)
	}

	localPath, err := t.downloadFile(ctx, result.Output.VideoURL, "video", ".mp4")
	if err != nil {
		return "", fmt.Errorf("download video: %w", err)
	}

	// 返回特殊格式，让飞书 channel 自动发送视频
	return fmt.Sprintf("视频生成成功！\n描述: %s\n时长: %d 秒\n\n[GENERATED_VIDEO:%s]", prompt, duration, localPath), nil
}

// imageAPIResponse 图片 API 响应
type imageAPIResponse struct {
	RequestId string `json:"request_id"`
	Output    struct {
		TaskID     string `json:"task_id"`
		TaskStatus string `json:"task_status"`
		Results    []struct {
			URL string `json:"url"`
		} `json:"results"`
		Code    string `json:"code,omitempty"`
		Message string `json:"message,omitempty"`
	} `json:"output"`
}

// videoAPIResponse 视频 API 响应
type videoAPIResponse struct {
	RequestId string `json:"request_id"`
	Output    struct {
		TaskID     string `json:"task_id"`
		TaskStatus string `json:"task_status"`
		VideoURL   string `json:"video_url"`
		Code       string `json:"code,omitempty"`
		Message    string `json:"message,omitempty"`
	} `json:"output"`
}

// callImageAPI 调用图片生成 API
func (t *ImageGenTool) callImageAPI(ctx context.Context, reqBody interface{}) (*imageAPIResponse, error) {
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/text2image/image-synthesis", t.apiBase)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", t.apiKey))
	req.Header.Set("X-DashScope-Async", "enable")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: status=%d body=%s", resp.StatusCode, string(body))
	}

	var result imageAPIResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	// 如果是异步任务，等待结果
	if result.Output.TaskID != "" && result.Output.TaskStatus != "SUCCEEDED" {
		return t.waitForImageResult(ctx, result.Output.TaskID)
	}

	return &result, nil
}

// waitForImageResult 等待图片生成结果
func (t *ImageGenTool) waitForImageResult(ctx context.Context, taskID string) (*imageAPIResponse, error) {
	// 阿里云任务查询 URL
	url := fmt.Sprintf("https://dashscope.aliyuncs.com/api/v1/tasks/%s", taskID)

	client := &http.Client{Timeout: 30 * time.Second}
	maxAttempts := 60 // 最多等待 5 分钟

	for i := 0; i < maxAttempts; i++ {
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, err
		}

		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", t.apiKey))

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var result imageAPIResponse
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, err
		}

		status := result.Output.TaskStatus
		// 处理可能的状态值（大小写兼容）
		switch {
		case status == "SUCCEEDED" || status == "succeeded":
			return &result, nil
		case status == "FAILED" || status == "failed":
			errMsg := result.Output.Message
			if errMsg == "" {
				errMsg = "unknown error"
			}
			return nil, fmt.Errorf("image generation failed: %s", errMsg)
		case status == "PENDING" || status == "pending" || status == "":
			// 空状态或 PENDING，继续等待
			time.Sleep(3 * time.Second)
		case status == "RUNNING" || status == "running" || status == "SUBMITTED" || status == "submitted":
			time.Sleep(3 * time.Second)
		default:
			// 未知状态，记录但继续等待
			time.Sleep(3 * time.Second)
		}
	}

	return nil, fmt.Errorf("timeout waiting for image generation")
}

// submitVideoTask 提交视频生成任务
func (t *ImageGenTool) submitVideoTask(ctx context.Context, reqBody interface{}) (string, error) {
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	// 使用 video-generation API 端点
	url := "https://dashscope.aliyuncs.com/api/v1/services/aigc/video-generation/video-synthesis"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", t.apiKey))
	req.Header.Set("X-DashScope-Async", "enable")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error: status=%d body=%s", resp.StatusCode, string(body))
	}

	var result videoAPIResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}

	return result.Output.TaskID, nil
}

// waitForVideoResult 等待视频生成结果
func (t *ImageGenTool) waitForVideoResult(ctx context.Context, taskID string) (*videoAPIResponse, error) {
	// 使用统一的任务查询 URL
	url := fmt.Sprintf("https://dashscope.aliyuncs.com/api/v1/tasks/%s", taskID)

	client := &http.Client{Timeout: 30 * time.Second}
	maxAttempts := 120 // 最多等待 10 分钟（视频生成较慢）

	for i := 0; i < maxAttempts; i++ {
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, err
		}

		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", t.apiKey))

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var result videoAPIResponse
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, err
		}

		switch result.Output.TaskStatus {
		case "SUCCEEDED":
			return &result, nil
		case "FAILED":
			return nil, fmt.Errorf("video generation failed: %s", result.Output.Message)
		case "PENDING", "RUNNING":
			time.Sleep(5 * time.Second)
		default:
			return nil, fmt.Errorf("unknown task status: %s", result.Output.TaskStatus)
		}
	}

	return nil, fmt.Errorf("timeout waiting for video generation")
}

// downloadFile 下载文件
func (t *ImageGenTool) downloadFile(ctx context.Context, url, prefix, ext string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed: status=%d", resp.StatusCode)
	}

	// 生成文件名
	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("%s-%s%s", prefix, timestamp, ext)
	filepath := filepath.Join(t.outputDir, filename)

	// 创建文件
	file, err := os.Create(filepath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// 写入文件
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return "", err
	}

	return filepath, nil
}

// IsDangerous 返回是否为危险操作
func (t *ImageGenTool) IsDangerous() bool {
	return false
}

// SetAPIKey 设置 API Key
func (t *ImageGenTool) SetAPIKey(key string) {
	t.apiKey = key
}
