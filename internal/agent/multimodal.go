// Package agent 多模态内容处理
package agent

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"

	"github.com/lingguard/pkg/llm"
	"github.com/lingguard/pkg/logger"
)

// buildMultimodalContent 构建多模态内容
func (a *Agent) buildMultimodalContent(text string, mediaPaths []string) ([]llm.ContentPart, error) {
	parts := make([]llm.ContentPart, 0)

	// 收集视频文件路径，用于后续添加路径提示
	var videoPaths []string
	var imagePaths []string
	var documentPaths []string

	// 视频大小限制：5MB（base64 编码后约 6.7MB）
	const maxVideoSize = 5 * 1024 * 1024

	// 统计图片数量，用于动态计算压缩限制
	imageCount := 0
	for _, path := range mediaPaths {
		// 跳过 base64 data URL 格式的媒体
		if isBase64DataURL(path) {
			imageCount++
			continue
		}
		ext := strings.ToLower(filepath.Ext(path))
		if !isVideoFile(ext) && !isDocumentFile(ext) {
			imageCount++
		}
	}

	// 动态计算每张图片的最大 base64 大小
	// 目标：所有图片的总 base64 大小不超过 8MB
	maxBase64PerImage := 8 * 1024 * 1024 // 默认 8MB
	if imageCount > 1 {
		maxBase64PerImage = (8 * 1024 * 1024) / imageCount
		logger.Info("Multiple images detected, adjusting compression limit", "imageCount", imageCount, "maxBase64PerImage", maxBase64PerImage)
	}

	// 添加图片/视频
	for _, path := range mediaPaths {
		var data []byte
		var mimeType string
		var isDataURL bool

		// 检查是否为 base64 data URL 格式（如来自 QQ 渠道的图片）
		if isBase64DataURL(path) {
			// 直接解析 base64 data URL
			var err error
			data, mimeType, err = parseBase64DataURL(path)
			if err != nil {
				return nil, fmt.Errorf("parse base64 data URL (%s): %w", truncateMediaPath(path), err)
			}
			isDataURL = true
			logger.Info("Parsed base64 data URL", "mimeType", mimeType, "size", len(data))
		} else {
			// 从文件读取
			var err error
			data, err = os.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("read media %s: %w", path, err)
			}
		}

		// 如果是视频文件，使用 video_url 格式（Qwen-Omni 模型）
		// 注意：base64 data URL 格式不支持视频，只支持图片
		if !isDataURL && isVideoFile(filepath.Ext(path)) {
			videoMimeType := detectVideoMimeType(filepath.Ext(path))
			videoPaths = append(videoPaths, path)

			// 检查视频大小，超过限制则不发送 base64
			if len(data) > maxVideoSize {
				logger.Warn("Video too large, skipping base64 encoding", "path", truncateMediaPath(path), "size", len(data), "maxSize", maxVideoSize)
				// 只添加提示文本，不发送视频内容
				parts = append(parts, llm.ContentPart{
					Type: "text",
					Text: fmt.Sprintf("[视频文件: %s，大小: %.1fMB，超过限制无法直接查看]", filepath.Base(path), float64(len(data))/1024/1024),
				})
			} else {
				base64Data := encodeBase64(data)
				logger.Info("Processing video file for multimodal", "path", truncateMediaPath(path), "mimeType", videoMimeType, "size", len(data))

				// 使用 video_url 格式（支持 Qwen-Omni 模型）
				parts = append(parts, llm.ContentPart{
					Type: "video_url",
					VideoURL: &llm.VideoURL{
						URL: fmt.Sprintf("data:%s;base64,%s", videoMimeType, base64Data),
					},
				})
			}
		} else if !isDataURL && isDocumentFile(filepath.Ext(path)) {
			// 文档文件：不发送内容，只提供文件路径提示
			documentPaths = append(documentPaths, path)
			fileName := filepath.Base(path)
			fileSize := float64(len(data)) / 1024 / 1024
			logger.Info("Processing document file", "path", truncateMediaPath(path), "size", len(data))

			// 添加文档提示文本
			parts = append(parts, llm.ContentPart{
				Type: "text",
				Text: fmt.Sprintf("[文档文件: %s，大小: %.1fMB，路径: %s]", fileName, fileSize, path),
			})
		} else {
			// 图片使用 image_url 格式
			// 如果是 base64 data URL，mimeType 已经在上面的 parseBase64DataURL 中解析出来了
			imageMimeType := mimeType
			if imageMimeType == "" {
				imageMimeType = detectMimeType(data)
			}

			// 压缩图片以符合动态计算的 API 限制
			compressedData, err := compressImageForLLM(data, maxBase64PerImage)
			if err != nil {
				logger.Warn("Failed to compress image, using original", "error", err)
				compressedData = data
			}

			base64Data := base64.StdEncoding.EncodeToString(compressedData)
			logger.Info("Processing image for multimodal", "path", truncateMediaPath(path), "originalSize", len(data), "compressedSize", len(compressedData), "base64Size", len(base64Data))

			parts = append(parts, llm.ContentPart{
				Type: "image_url",
				ImageURL: &llm.ImageURL{
					URL:    fmt.Sprintf("data:%s;base64,%s", imageMimeType, base64Data),
					Detail: "auto",
				},
			})
			// 只记录文件路径，不记录 base64 data URL
			if !isDataURL {
				imagePaths = append(imagePaths, path)
			}
		}
	}

	// 添加媒体路径提示（让 LLM 知道文件位置以便使用工具处理）
	var mediaHints []string
	if len(videoPaths) > 0 {
		mediaHints = append(mediaHints, fmt.Sprintf("[视频文件路径: %s]", strings.Join(videoPaths, ", ")))
	}
	if len(imagePaths) > 0 {
		mediaHints = append(mediaHints, fmt.Sprintf("[图片文件路径: %s]", strings.Join(imagePaths, ", ")))
	}
	if len(documentPaths) > 0 {
		mediaHints = append(mediaHints, fmt.Sprintf("[文档文件路径: %s]", strings.Join(documentPaths, ", ")))
	}

	// 添加文本（放在最后）
	if text != "" {
		parts = append(parts, llm.ContentPart{
			Type: "text",
			Text: text,
		})
	}

	// 添加媒体路径提示
	if len(mediaHints) > 0 {
		parts = append(parts, llm.ContentPart{
			Type: "text",
			Text: strings.Join(mediaHints, "\n"),
		})
	}

	return parts, nil
}

// isVideoFile 检查文件扩展名是否为视频格式
func isVideoFile(ext string) bool {
	videoExts := map[string]bool{
		".mp4":  true,
		".mov":  true,
		".avi":  true,
		".mkv":  true,
		".webm": true,
		".flv":  true,
		".wmv":  true,
		".m4v":  true,
		".3gp":  true,
	}
	return videoExts[ext]
}

// isDocumentFile 判断是否为文档文件
func isDocumentFile(ext string) bool {
	docExts := map[string]bool{
		".pdf":  true,
		".doc":  true,
		".docx": true,
		".xls":  true,
		".xlsx": true,
		".ppt":  true,
		".pptx": true,
		".txt":  true,
		".md":   true,
		".csv":  true,
		".rtf":  true,
	}
	return docExts[ext]
}

// detectVideoMimeType 根据扩展名检测视频 MIME 类型
func detectVideoMimeType(ext string) string {
	videoMimes := map[string]string{
		".mp4":  "video/mp4",
		".mov":  "video/quicktime",
		".avi":  "video/x-msvideo",
		".mkv":  "video/x-matroska",
		".webm": "video/webm",
		".flv":  "video/x-flv",
		".wmv":  "video/x-ms-wmv",
		".m4v":  "video/mp4",
		".3gp":  "video/3gpp",
	}
	if mime, ok := videoMimes[ext]; ok {
		return mime
	}
	return "video/mp4"
}

// detectMimeType 检测图片 MIME 类型
func detectMimeType(data []byte) string {
	if len(data) < 8 {
		return "image/jpeg"
	}

	// JPEG: FF D8 FF
	if data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return "image/jpeg"
	}
	// PNG: 89 50 4E 47 0D 0A 1A 0A
	if data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 {
		return "image/png"
	}
	// GIF: 47 49 46 38
	if data[0] == 0x47 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x38 {
		return "image/gif"
	}
	// WebP: 52 49 46 46 ... 57 45 42 50
	if data[0] == 0x52 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x46 {
		if len(data) > 11 && data[8] == 0x57 && data[9] == 0x45 && data[10] == 0x42 && data[11] == 0x50 {
			return "image/webp"
		}
	}

	return "image/jpeg"
}

// encodeBase64 编码为 base64（使用标准库）
func encodeBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// compressImageForLLM 压缩图片以符合 LLM API 大小限制
// maxSize 是 base64 编码后的最大字节数
func compressImageForLLM(imageData []byte, maxBase64Size int) ([]byte, error) {
	// 先检查原始大小（base64 编码后）
	base64Size := (len(imageData)+2)/3*4 + len("data:image/jpeg;base64,")
	if base64Size <= maxBase64Size {
		return imageData, nil
	}

	// 解码图片
	img, _, err := image.Decode(bytes.NewReader(imageData))
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}

	// 尝试不同质量级别压缩
	qualities := []int{85, 70, 55, 40, 25}
	for _, quality := range qualities {
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
			continue
		}

		// 检查 base64 编码后的大小
		encodedSize := (buf.Len()+2)/3*4 + len("data:image/jpeg;base64,")
		if encodedSize <= maxBase64Size {
			logger.Info("Image compressed for LLM", "quality", quality, "originalSize", len(imageData), "compressedSize", buf.Len())
			return buf.Bytes(), nil
		}
	}

	// 如果仍然太大，尝试缩小尺寸
	bounds := img.Bounds()
	maxDimension := 1024
	for maxDimension >= 256 {
		ratio := float64(maxDimension) / float64(max(bounds.Dx(), bounds.Dy()))
		if ratio >= 1 {
			maxDimension -= 128
			continue
		}

		newWidth := int(float64(bounds.Dx()) * ratio)
		newHeight := int(float64(bounds.Dy()) * ratio)

		// 简单的最近邻缩放
		resized := resizeImage(img, newWidth, newHeight)

		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, resized, &jpeg.Options{Quality: 70}); err != nil {
			maxDimension -= 128
			continue
		}

		encodedSize := (buf.Len()+2)/3*4 + len("data:image/jpeg;base64,")
		if encodedSize <= maxBase64Size {
			logger.Info("Image resized and compressed for LLM", "maxDimension", maxDimension, "compressedSize", buf.Len())
			return buf.Bytes(), nil
		}

		maxDimension -= 128
	}

	return nil, fmt.Errorf("image too large after compression (base64 size still exceeds %d)", maxBase64Size)
}

// resizeImage 简单的图片缩放
func resizeImage(img image.Image, newWidth, newHeight int) image.Image {
	bounds := img.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))

	xRatio := float64(bounds.Dx()) / float64(newWidth)
	yRatio := float64(bounds.Dy()) / float64(newHeight)

	for y := 0; y < newHeight; y++ {
		for x := 0; x < newWidth; x++ {
			srcX := int(float64(x) * xRatio)
			srcY := int(float64(y) * yRatio)
			dst.Set(x, y, img.At(srcX+bounds.Min.X, srcY+bounds.Min.Y))
		}
	}

	return dst
}

// isBase64DataURL 检查字符串是否为 base64 data URL 格式
// 格式: data:<mimeType>;base64,<base64Data>
func isBase64DataURL(s string) bool {
	return strings.HasPrefix(s, "data:") && strings.Contains(s, ";base64,")
}

// truncateMediaPath 截断媒体路径显示，避免在日志中打印完整的 base64 数据
func truncateMediaPath(path string) string {
	if isBase64DataURL(path) {
		// 对于 base64 data URL，只显示摘要
		base64Index := strings.Index(path, ";base64,")
		if base64Index > 0 {
			mimeType := path[5:base64Index]        // "data:" 长度为 5
			dataLen := len(path) - base64Index - 8 // ";base64," 长度为 8
			return fmt.Sprintf("[base64:%s,len=%d]", mimeType, dataLen)
		}
		return "[base64 data URL]"
	}
	return path
}

// parseBase64DataURL 解析 base64 data URL，返回解码后的数据和 MIME 类型
// 格式: data:<mimeType>;base64,<base64Data>
func parseBase64DataURL(dataURL string) ([]byte, string, error) {
	// 检查格式
	if !strings.HasPrefix(dataURL, "data:") {
		return nil, "", fmt.Errorf("invalid data URL: missing 'data:' prefix")
	}

	// 查找 base64 标记
	base64Index := strings.Index(dataURL, ";base64,")
	if base64Index == -1 {
		return nil, "", fmt.Errorf("invalid data URL: missing ';base64,' marker")
	}

	// 提取 MIME 类型
	mimeType := dataURL[5:base64Index] // "data:" 长度为 5

	// 提取 base64 数据
	base64Data := dataURL[base64Index+8:] // ";base64," 长度为 8
	if base64Data == "" {
		return nil, "", fmt.Errorf("invalid data URL: empty base64 data")
	}

	// 解码 base64
	data, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return nil, "", fmt.Errorf("decode base64: %w", err)
	}

	return data, mimeType, nil
}
