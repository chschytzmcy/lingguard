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

	// 视频大小限制：5MB（base64 编码后约 6.7MB）
	const maxVideoSize = 5 * 1024 * 1024

	// 添加图片/视频
	for _, path := range mediaPaths {
		// 读取媒体文件并转换为 base64
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read media %s: %w", path, err)
		}

		ext := strings.ToLower(filepath.Ext(path))

		// 如果是视频文件，使用 video_url 格式（Qwen-Omni 模型）
		if isVideoFile(ext) {
			mimeType := detectVideoMimeType(ext)
			videoPaths = append(videoPaths, path)

			// 检查视频大小，超过限制则不发送 base64
			if len(data) > maxVideoSize {
				logger.Warn("Video too large, skipping base64 encoding", "path", path, "size", len(data), "maxSize", maxVideoSize)
				// 只添加提示文本，不发送视频内容
				parts = append(parts, llm.ContentPart{
					Type: "text",
					Text: fmt.Sprintf("[视频文件: %s，大小: %.1fMB，超过限制无法直接查看]", filepath.Base(path), float64(len(data))/1024/1024),
				})
			} else {
				base64Data := encodeBase64(data)
				logger.Info("Processing video file for multimodal", "path", path, "mimeType", mimeType, "size", len(data))

				// 使用 video_url 格式（支持 Qwen-Omni 模型）
				parts = append(parts, llm.ContentPart{
					Type: "video_url",
					VideoURL: &llm.VideoURL{
						URL: fmt.Sprintf("data:%s;base64,%s", mimeType, base64Data),
					},
				})
			}
		} else {
			// 图片使用 image_url 格式
			mimeType := detectMimeType(data)

			// 压缩图片以符合 API 限制（base64 最大 10MB，但压缩到 8MB 以内更安全）
			compressedData, err := compressImageForLLM(data, 8*1024*1024)
			if err != nil {
				logger.Warn("Failed to compress image, using original", "error", err)
				compressedData = data
			}

			base64Data := base64.StdEncoding.EncodeToString(compressedData)
			logger.Info("Processing image for multimodal", "path", path, "originalSize", len(data), "compressedSize", len(compressedData), "base64Size", len(base64Data))

			parts = append(parts, llm.ContentPart{
				Type: "image_url",
				ImageURL: &llm.ImageURL{
					URL:    fmt.Sprintf("data:%s;base64,%s", mimeType, base64Data),
					Detail: "auto",
				},
			})
			imagePaths = append(imagePaths, path)
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
