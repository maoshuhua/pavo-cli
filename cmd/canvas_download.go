package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/maoshuhua/pavo-cli/internal/api"
)

func downloadCanvasTaskResults(ctx context.Context, deps *dependencies, task *canvasTaskOutput, taskID, nodeName, rawOutputDir string) error {
	if task == nil || len(bytes.TrimSpace(task.TaskResult)) == 0 {
		return nil
	}

	var payload map[string]any
	decoder := json.NewDecoder(bytes.NewReader(task.TaskResult))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return fmt.Errorf("解析画布任务结果以下载资源: %w", err)
	}
	results, ok := payload["results"].([]any)
	if !ok || len(results) == 0 {
		return nil
	}

	outputDir, err := canvasTaskOutputDir(rawOutputDir, taskID)
	if err != nil {
		return err
	}
	filenameBase := safeFilenamePart(nodeName)
	if filenameBase == "" {
		filenameBase = "result"
	}

	for index, rawItem := range results {
		item, ok := rawItem.(map[string]any)
		if !ok {
			continue
		}
		rawURL, _ := item["url"].(string)
		rawURL = strings.TrimSpace(rawURL)
		if rawURL == "" {
			continue
		}

		delete(item, "local_path")
		delete(item, "download_error")
		filename := fmt.Sprintf("%02d-%s%s", index+1, filenameBase, canvasResultExtension(item, rawURL))
		response, downloadErr := deps.api.DownloadResult(ctx, api.DownloadResultOptions{
			URL:        rawURL,
			OutputPath: filepath.Join(outputDir, filename),
		})
		if downloadErr != nil {
			item["download_error"] = downloadErr.Error()
			continue
		}
		item["local_path"] = response.OutputPath
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("编码带本地路径的画布任务结果: %w", err)
	}
	task.TaskResult = encoded
	return nil
}

func canvasTaskOutputDir(rawOutputDir, taskID string) (string, error) {
	outputDir := strings.TrimSpace(rawOutputDir)
	if outputDir == "" {
		workingDir, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("读取当前工作目录失败: %w", err)
		}
		taskPart := safeFilenamePart(taskID)
		if taskPart == "" {
			taskPart = "task"
		}
		outputDir = filepath.Join(workingDir, "pavo_outputs", "canvas", taskPart)
	}
	absDir, err := filepath.Abs(outputDir)
	if err != nil {
		return "", fmt.Errorf("解析画布结果下载目录失败: %w", err)
	}
	return absDir, nil
}

func canvasResultExtension(item map[string]any, rawURL string) string {
	if parsed, err := url.Parse(rawURL); err == nil {
		if extension := safeResultExtension(path.Ext(parsed.Path)); extension != "" {
			return extension
		}
	}
	mimetype, _ := item["mimetype"].(string)
	if strings.TrimSpace(mimetype) == "" {
		mimetype, _ = item["mime_type"].(string)
	}
	if extension := preferredCanvasMIMEExtension(mimetype); extension != "" {
		return extension
	}
	return generatedResultExtension(mimetype)
}

func preferredCanvasMIMEExtension(mimetype string) string {
	switch strings.ToLower(strings.TrimSpace(strings.SplitN(mimetype, ";", 2)[0])) {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	case "image/heic":
		return ".heic"
	case "image/heif":
		return ".heif"
	case "video/mp4":
		return ".mp4"
	case "video/quicktime":
		return ".mov"
	case "video/webm":
		return ".webm"
	case "audio/mpeg":
		return ".mp3"
	case "audio/mp4":
		return ".m4a"
	case "audio/wav", "audio/x-wav":
		return ".wav"
	case "audio/ogg":
		return ".ogg"
	default:
		return ""
	}
}

func safeResultExtension(extension string) string {
	extension = strings.ToLower(strings.TrimSpace(extension))
	if len(extension) < 2 || len(extension) > 10 || extension[0] != '.' {
		return ""
	}
	for _, char := range extension[1:] {
		if !unicode.IsLetter(char) && !unicode.IsDigit(char) {
			return ""
		}
	}
	return extension
}
