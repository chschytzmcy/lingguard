// Package tools 媒体扫描工具
package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/lingguard/internal/providers"
	"github.com/lingguard/pkg/llm"
	"github.com/lingguard/pkg/logger"
	"github.com/lingguard/pkg/speech"
)

// MediaScanTool 媒体扫描工具
type MediaScanTool struct {
	multimodalProvider providers.Provider
	speechService      speech.Service
	workspaceMgr       *WorkspaceManager
	sandboxed          bool
	maxConcurrent      int
	maxVideoSize       int64 // 视频大小限制（字节）
}

// NewMediaScanTool 创建媒体扫描工具
func NewMediaScanTool(multimodalProvider providers.Provider, speechService speech.Service, workspaceMgr *WorkspaceManager, sandboxed bool) *MediaScanTool {
	return &MediaScanTool{
		multimodalProvider: multimodalProvider,
		speechService:      speechService,
		workspaceMgr:       workspaceMgr,
		sandboxed:          sandboxed,
		maxConcurrent:      3,                // 并发数，避免 API 限流
		maxVideoSize:       10 * 1024 * 1024, // 10MB
	}
}

func (t *MediaScanTool) Name() string { return "media_scan" }

func (t *MediaScanTool) Description() string {
	return `扫描工作目录中的媒体文件（图片/视频/音频），识别特定内容。

**支持类型**：
- 图片：jpg, jpeg, png, gif, webp, bmp
- 视频：mp4, mov, avi, mkv, webm（限制 10MB）
- 音频：mp3, wav, m4a, opus, flac, aac

**参数**：
- directory: 要扫描的目录路径（必填，相对于工作目录）
- target: 要查找的内容描述（必填）
- media_types: 媒体类型 ["image", "video", "audio"]，默认全部
- recursive: 是否递归扫描子目录（默认 true）
- max_files: 每种类型最大扫描数（默认 50）

**触发场景**：
- "找出包含香烟的图片和视频"
- "搜索提到'戒烟'的音频"
- "查找有猫的照片"

**注意**：只能扫描工作目录内的文件。`
}

func (t *MediaScanTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"directory": map[string]interface{}{
				"type":        "string",
				"description": "要扫描的目录路径（相对于工作目录）",
			},
			"target": map[string]interface{}{
				"type":        "string",
				"description": "要查找的内容描述",
			},
			"media_types": map[string]interface{}{
				"type":        "array",
				"items":       map[string]interface{}{"type": "string", "enum": []string{"image", "video", "audio"}},
				"description": "媒体类型过滤",
			},
			"recursive": map[string]interface{}{
				"type":        "boolean",
				"description": "是否递归扫描子目录",
				"default":     true,
			},
			"max_files": map[string]interface{}{
				"type":        "integer",
				"description": "每种类型最大扫描数",
				"default":     50,
			},
		},
		"required": []string{"directory", "target"},
	}
}

func (t *MediaScanTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Directory  string   `json:"directory"`
		Target     string   `json:"target"`
		MediaTypes []string `json:"media_types"`
		Recursive  bool     `json:"recursive"`
		MaxFiles   int      `json:"max_files"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}

	// 默认值
	if len(params.MediaTypes) == 0 {
		params.MediaTypes = []string{"image", "video", "audio"}
	}
	if params.MaxFiles <= 0 {
		params.MaxFiles = 50
	}

	// 验证目录路径
	absDir, err := t.validateAndResolvePath(params.Directory)
	if err != nil {
		return "", err
	}

	// 检查多模态能力
	if t.multimodalProvider == nil {
		return "", fmt.Errorf("multimodal provider not configured")
	}

	// 1. 收集媒体文件
	mediaFiles, err := t.collectMediaFiles(absDir, params.MediaTypes, params.Recursive, params.MaxFiles)
	if err != nil {
		return "", fmt.Errorf("collect media files: %w", err)
	}

	if len(mediaFiles.Images) == 0 && len(mediaFiles.Videos) == 0 && len(mediaFiles.Audios) == 0 {
		return "未找到媒体文件", nil
	}

	logger.Info("Media scan started", "directory", absDir, "target", params.Target,
		"images", len(mediaFiles.Images), "videos", len(mediaFiles.Videos), "audios", len(mediaFiles.Audios))

	startTime := time.Now()

	// 2. 并行分析各类媒体
	result := &ScanResult{
		Target: params.Target,
	}

	var wg sync.WaitGroup

	// 图片扫描
	if len(mediaFiles.Images) > 0 && t.containsType(params.MediaTypes, "image") {
		wg.Add(1)
		go func() {
			defer wg.Done()
			matches := t.scanImages(ctx, mediaFiles.Images, params.Target)
			result.ImageMatches = matches
		}()
	}

	// 视频扫描
	if len(mediaFiles.Videos) > 0 && t.containsType(params.MediaTypes, "video") {
		wg.Add(1)
		go func() {
			defer wg.Done()
			matches := t.scanVideos(ctx, mediaFiles.Videos, params.Target)
			result.VideoMatches = matches
		}()
	}

	// 音频扫描
	if len(mediaFiles.Audios) > 0 && t.containsType(params.MediaTypes, "audio") && t.speechService != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			matches := t.scanAudios(ctx, mediaFiles.Audios, params.Target)
			result.AudioMatches = matches
		}()
	}

	wg.Wait()

	// 3. 格式化结果
	result.Duration = time.Since(startTime)
	return t.formatResult(result, mediaFiles, absDir), nil
}

// MediaFiles 媒体文件集合
type MediaFiles struct {
	Images []string
	Videos []string
	Audios []string
}

// ScanResult 扫描结果
type ScanResult struct {
	Target       string
	ImageMatches []MediaMatch
	VideoMatches []MediaMatch
	AudioMatches []MediaMatch
	Duration     time.Duration
}

// MediaMatch 匹配的媒体
type MediaMatch struct {
	Path        string
	Description string
	Confidence  string // high/medium/low
}

// validateAndResolvePath 验证并解析路径
func (t *MediaScanTool) validateAndResolvePath(dir string) (string, error) {
	// 展开路径
	dir = t.expandPath(dir)

	// 获取绝对路径
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	// 沙箱检查
	if t.sandboxed && t.workspaceMgr != nil {
		workspace := t.workspaceMgr.Get()
		absWorkspace, _ := filepath.Abs(workspace)

		// 检查是否在工作目录内
		rel, err := filepath.Rel(absWorkspace, absDir)
		if err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
			return "", fmt.Errorf("directory '%s' is outside workspace '%s'", dir, workspace)
		}
	}

	// 检查目录是否存在
	info, err := os.Stat(absDir)
	if err != nil {
		return "", fmt.Errorf("cannot access directory: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("'%s' is not a directory", dir)
	}

	return absDir, nil
}

// expandPath 展开路径中的 ~ 和环境变量
func (t *MediaScanTool) expandPath(path string) string {
	// 如果是相对路径且工作目录存在，相对于工作目录解析
	if !filepath.IsAbs(path) && t.workspaceMgr != nil {
		workspace := t.workspaceMgr.Get()
		return filepath.Join(workspace, path)
	}
	return path
}

// collectMediaFiles 收集媒体文件
func (t *MediaScanTool) collectMediaFiles(dir string, types []string, recursive bool, maxPerType int) (*MediaFiles, error) {
	files := &MediaFiles{}

	imageExts := map[string]bool{
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true, ".bmp": true,
	}
	videoExts := map[string]bool{
		".mp4": true, ".mov": true, ".avi": true, ".mkv": true, ".webm": true, ".m4v": true,
	}
	audioExts := map[string]bool{
		".mp3": true, ".wav": true, ".m4a": true, ".opus": true, ".flac": true, ".aac": true, ".ogg": true,
	}

	scanImage := t.containsType(types, "image")
	scanVideo := t.containsType(types, "video")
	scanAudio := t.containsType(types, "audio")

	imageCount := 0
	videoCount := 0
	audioCount := 0

	walkFn := func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))

		if scanImage && imageExts[ext] && imageCount < maxPerType {
			files.Images = append(files.Images, path)
			imageCount++
		} else if scanVideo && videoExts[ext] && videoCount < maxPerType {
			files.Videos = append(files.Videos, path)
			videoCount++
		} else if scanAudio && audioExts[ext] && audioCount < maxPerType {
			files.Audios = append(files.Audios, path)
			audioCount++
		}

		return nil
	}

	if recursive {
		filepath.Walk(dir, walkFn)
	} else {
		entries, _ := os.ReadDir(dir)
		for _, entry := range entries {
			path := filepath.Join(dir, entry.Name())
			info, err := os.Stat(path)
			if err == nil {
				walkFn(path, info, nil)
			}
		}
	}

	return files, nil
}

// scanImages 扫描图片
func (t *MediaScanTool) scanImages(ctx context.Context, images []string, target string) []MediaMatch {
	var matches []MediaMatch
	var mu sync.Mutex

	sem := make(chan struct{}, t.maxConcurrent)
	var wg sync.WaitGroup

	for _, imgPath := range images {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			match, err := t.analyzeImage(ctx, path, target)
			if err != nil {
				logger.Debug("Image analysis failed", "path", path, "error", err)
				return
			}
			if match != nil {
				mu.Lock()
				matches = append(matches, *match)
				mu.Unlock()
				logger.Debug("Image matched", "path", path, "description", match.Description)
			}
		}(imgPath)
	}

	wg.Wait()
	return matches
}

// analyzeImage 分析单张图片
func (t *MediaScanTool) analyzeImage(ctx context.Context, imgPath, target string) (*MediaMatch, error) {
	// 读取图片
	data, err := os.ReadFile(imgPath)
	if err != nil {
		return nil, err
	}

	// 压缩大图片
	if len(data) > 2*1024*1024 { // 超过 2MB 压缩
		compressed, err := compressImage(data, 1024*1024) // 压缩到 1MB 以下
		if err == nil {
			data = compressed
		}
	}

	// 构建 base64 data URL
	mimeType := detectImageMimeType(data)
	dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(data))

	// 构建消息
	prompt := fmt.Sprintf(`分析这张图片，判断是否包含"%s"。

请严格按照以下 JSON 格式回复，不要添加任何其他内容：
{"contains":true或false,"description":"简要描述图片中与%s相关的内容","confidence":"high或medium或low"}`, target, target)

	content := []llm.ContentPart{
		{Type: "image_url", ImageURL: &llm.ImageURL{URL: dataURL, Detail: "low"}},
		{Type: "text", Text: prompt},
	}

	// 调用多模态 LLM
	req := &llm.Request{
		Messages: []llm.Message{
			{Role: "user", ContentParts: content},
		},
	}

	resp, err := t.multimodalProvider.Complete(ctx, req)
	if err != nil {
		return nil, err
	}

	response := resp.GetContent()
	return t.parseMediaResponse(response, imgPath)
}

// scanVideos 扫描视频
func (t *MediaScanTool) scanVideos(ctx context.Context, videos []string, target string) []MediaMatch {
	var matches []MediaMatch
	var mu sync.Mutex

	sem := make(chan struct{}, t.maxConcurrent)
	var wg sync.WaitGroup

	for _, videoPath := range videos {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			match, err := t.analyzeVideo(ctx, path, target)
			if err != nil {
				logger.Debug("Video analysis failed", "path", path, "error", err)
				return
			}
			if match != nil {
				mu.Lock()
				matches = append(matches, *match)
				mu.Unlock()
				logger.Debug("Video matched", "path", path, "description", match.Description)
			}
		}(videoPath)
	}

	wg.Wait()
	return matches
}

// analyzeVideo 分析视频
func (t *MediaScanTool) analyzeVideo(ctx context.Context, videoPath, target string) (*MediaMatch, error) {
	// 检查文件大小
	info, err := os.Stat(videoPath)
	if err != nil {
		return nil, err
	}

	if info.Size() > t.maxVideoSize {
		// 大视频暂不支持
		return &MediaMatch{
			Path:        videoPath,
			Description: fmt.Sprintf("视频太大 (%.1fMB)，跳过分析", float64(info.Size())/1024/1024),
			Confidence:  "low",
		}, nil
	}

	// 小视频：直接 base64 发送
	data, err := os.ReadFile(videoPath)
	if err != nil {
		return nil, err
	}

	mimeType := detectVideoMimeType(filepath.Ext(videoPath))
	dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(data))

	prompt := fmt.Sprintf(`分析这个视频，判断是否包含"%s"。

请严格按照以下 JSON 格式回复，不要添加任何其他内容：
{"contains":true或false,"description":"描述视频中与%s相关的内容","confidence":"high或medium或low"}`, target, target)

	content := []llm.ContentPart{
		{Type: "video_url", VideoURL: &llm.VideoURL{URL: dataURL}},
		{Type: "text", Text: prompt},
	}

	req := &llm.Request{
		Messages: []llm.Message{
			{Role: "user", ContentParts: content},
		},
	}

	resp, err := t.multimodalProvider.Complete(ctx, req)
	if err != nil {
		return nil, err
	}

	return t.parseMediaResponse(resp.GetContent(), videoPath)
}

// scanAudios 扫描音频
func (t *MediaScanTool) scanAudios(ctx context.Context, audios []string, target string) []MediaMatch {
	var matches []MediaMatch
	var mu sync.Mutex

	sem := make(chan struct{}, t.maxConcurrent)
	var wg sync.WaitGroup

	for _, audioPath := range audios {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			match, err := t.analyzeAudio(ctx, path, target)
			if err != nil {
				logger.Debug("Audio analysis failed", "path", path, "error", err)
				return
			}
			if match != nil {
				mu.Lock()
				matches = append(matches, *match)
				mu.Unlock()
				logger.Debug("Audio matched", "path", path, "description", match.Description)
			}
		}(audioPath)
	}

	wg.Wait()
	return matches
}

// analyzeAudio 分析音频
func (t *MediaScanTool) analyzeAudio(ctx context.Context, audioPath, target string) (*MediaMatch, error) {
	// 1. ASR 转写
	result, err := t.speechService.Transcribe(ctx, audioPath)
	if err != nil {
		return nil, err
	}

	transcript := result.Text
	if transcript == "" {
		return nil, nil
	}

	// 2. 检查文本是否包含目标内容
	prompt := fmt.Sprintf(`判断以下音频转写文本是否与"%s"相关。

转写文本：
%s

请严格按照以下 JSON 格式回复，不要添加任何其他内容：
{"relevant":true或false,"reason":"判断理由"}`, target, truncateText(transcript, 1000))

	req := &llm.Request{
		Messages: []llm.Message{
			{Role: "user", Content: prompt},
		},
	}

	resp, err := t.multimodalProvider.Complete(ctx, req)
	if err != nil {
		// 降级为简单字符串匹配
		if strings.Contains(transcript, target) {
			return &MediaMatch{
				Path:        audioPath,
				Description: fmt.Sprintf("转写文本包含关键词: %s", target),
				Confidence:  "medium",
			}, nil
		}
		return nil, nil
	}

	response := resp.GetContent()
	var judgeResult struct {
		Relevant bool   `json:"relevant"`
		Reason   string `json:"reason"`
	}

	// 尝试解析 JSON
	if err := parseJSONFromResponse(response, &judgeResult); err == nil && judgeResult.Relevant {
		return &MediaMatch{
			Path:        audioPath,
			Description: fmt.Sprintf("转写: %s\n判断: %s", truncateText(transcript, 200), judgeResult.Reason),
			Confidence:  "high",
		}, nil
	}

	return nil, nil
}

// parseMediaResponse 解析媒体分析响应
func (t *MediaScanTool) parseMediaResponse(response, path string) (*MediaMatch, error) {
	var result struct {
		Contains    bool   `json:"contains"`
		Description string `json:"description"`
		Confidence  string `json:"confidence"`
	}

	if err := parseJSONFromResponse(response, &result); err != nil {
		// 尝试简单关键词匹配
		responseLower := strings.ToLower(response)
		if strings.Contains(responseLower, "true") || strings.Contains(responseLower, "包含") || strings.Contains(responseLower, "yes") {
			return &MediaMatch{
				Path:        path,
				Description: truncateText(response, 200),
				Confidence:  "low",
			}, nil
		}
		return nil, nil
	}

	if result.Contains {
		if result.Confidence == "" {
			result.Confidence = "medium"
		}
		return &MediaMatch{
			Path:        path,
			Description: result.Description,
			Confidence:  result.Confidence,
		}, nil
	}

	return nil, nil
}

// formatResult 格式化结果
func (t *MediaScanTool) formatResult(result *ScanResult, files *MediaFiles, baseDir string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("📊 媒体扫描结果\n"))
	sb.WriteString(fmt.Sprintf("目标: %s\n", result.Target))
	sb.WriteString(fmt.Sprintf("扫描目录: %s\n", baseDir))
	sb.WriteString(fmt.Sprintf("耗时: %.1f 秒\n\n", result.Duration.Seconds()))

	// 图片结果
	if len(files.Images) > 0 {
		sb.WriteString(fmt.Sprintf("📷 图片 (%d/%d 匹配):\n", len(result.ImageMatches), len(files.Images)))
		if len(result.ImageMatches) > 0 {
			for _, m := range result.ImageMatches {
				relPath := t.getRelativePath(m.Path, baseDir)
				sb.WriteString(fmt.Sprintf("  ✓ %s [%s]\n", relPath, m.Confidence))
				if m.Description != "" {
					sb.WriteString(fmt.Sprintf("    %s\n", m.Description))
				}
			}
		} else {
			sb.WriteString("  未找到匹配的图片\n")
		}
		sb.WriteString("\n")
	}

	// 视频结果
	if len(files.Videos) > 0 {
		sb.WriteString(fmt.Sprintf("🎬 视频 (%d/%d 匹配):\n", len(result.VideoMatches), len(files.Videos)))
		if len(result.VideoMatches) > 0 {
			for _, m := range result.VideoMatches {
				relPath := t.getRelativePath(m.Path, baseDir)
				sb.WriteString(fmt.Sprintf("  ✓ %s [%s]\n", relPath, m.Confidence))
				if m.Description != "" {
					sb.WriteString(fmt.Sprintf("    %s\n", m.Description))
				}
			}
		} else {
			sb.WriteString("  未找到匹配的视频\n")
		}
		sb.WriteString("\n")
	}

	// 音频结果
	if len(files.Audios) > 0 {
		sb.WriteString(fmt.Sprintf("🎵 音频 (%d/%d 匹配):\n", len(result.AudioMatches), len(files.Audios)))
		if len(result.AudioMatches) > 0 {
			for _, m := range result.AudioMatches {
				relPath := t.getRelativePath(m.Path, baseDir)
				sb.WriteString(fmt.Sprintf("  ✓ %s [%s]\n", relPath, m.Confidence))
				if m.Description != "" {
					sb.WriteString(fmt.Sprintf("    %s\n", m.Description))
				}
			}
		} else {
			sb.WriteString("  未找到匹配的音频\n")
		}
		sb.WriteString("\n")
	}

	// 汇总
	totalMatches := len(result.ImageMatches) + len(result.VideoMatches) + len(result.AudioMatches)
	sb.WriteString(fmt.Sprintf("总计: %d 个匹配", totalMatches))

	return sb.String()
}

// getRelativePath 获取相对路径
func (t *MediaScanTool) getRelativePath(path, baseDir string) string {
	rel, err := filepath.Rel(baseDir, path)
	if err != nil {
		return path
	}
	return rel
}

// containsType 检查是否包含指定类型
func (t *MediaScanTool) containsType(types []string, typ string) bool {
	for _, t := range types {
		if t == typ {
			return true
		}
	}
	return false
}

// compressImage 压缩图片
func compressImage(data []byte, maxSize int) ([]byte, error) {
	// 简单实现：如果已经小于目标大小，直接返回
	if len(data) <= maxSize {
		return data, nil
	}

	// TODO: 使用图像库进行压缩
	// 目前先返回原图，后续可以添加实际的压缩逻辑
	return data, nil
}

// detectImageMimeType 检测图片 MIME 类型
func detectImageMimeType(data []byte) string {
	if len(data) < 8 {
		return "image/jpeg"
	}
	// JPEG
	if data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return "image/jpeg"
	}
	// PNG
	if data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 {
		return "image/png"
	}
	// GIF
	if data[0] == 0x47 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x38 {
		return "image/gif"
	}
	// WebP
	if data[0] == 0x52 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x46 {
		if len(data) > 11 && data[8] == 0x57 && data[9] == 0x45 && data[10] == 0x42 && data[11] == 0x50 {
			return "image/webp"
		}
	}
	return "image/jpeg"
}

// detectVideoMimeType 检测视频 MIME 类型
func detectVideoMimeType(ext string) string {
	mimes := map[string]string{
		".mp4":  "video/mp4",
		".mov":  "video/quicktime",
		".webm": "video/webm",
		".avi":  "video/x-msvideo",
		".mkv":  "video/x-matroska",
		".m4v":  "video/mp4",
	}
	ext = strings.ToLower(ext)
	if m, ok := mimes[ext]; ok {
		return m
	}
	return "video/mp4"
}

// parseJSONFromResponse 从响应中提取 JSON
func parseJSONFromResponse(response string, target interface{}) error {
	// 尝试直接解析
	if err := json.Unmarshal([]byte(response), target); err == nil {
		return nil
	}

	// 尝试提取 JSON 块
	start := strings.Index(response, "{")
	end := strings.LastIndex(response, "}")
	if start >= 0 && end > start {
		jsonStr := response[start : end+1]
		return json.Unmarshal([]byte(jsonStr), target)
	}

	return fmt.Errorf("no valid JSON found")
}

func (t *MediaScanTool) IsDangerous() bool         { return false }
func (t *MediaScanTool) ShouldLoadByDefault() bool { return false }
