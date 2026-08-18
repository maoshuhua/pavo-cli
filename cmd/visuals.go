package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/maoshuhua/pavo-cli/internal/api"
	"github.com/maoshuhua/pavo-cli/internal/output"
	"github.com/spf13/cobra"
)

const (
	defaultVisualDownloadConcurrency = 4
	maxVisualDownloadConcurrency     = 32
)

type visualsOutput struct {
	Category    api.VisualCategory   `json:"category"`
	DownloadDir string               `json:"download_dir"`
	Downloaded  int                  `json:"downloaded"`
	Failed      int                  `json:"failed"`
	Pagination  api.VisualPagination `json:"pagination"`
	Groups      []api.VisualGroup    `json:"groups"`
}

func newVisualsCommand(stdout, stderr io.Writer, deps *dependencies) *cobra.Command {
	var category string
	var page int
	var pageSize int
	var downloadDir string
	var downloadConcurrency int
	cmd := &cobra.Command{
		Use:   "visuals",
		Short: "List and download the current user's generated images or videos",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return writeVisuals(stdout, deps, cmd, api.VisualCategory(strings.ToLower(strings.TrimSpace(category))), page, pageSize, downloadDir, downloadConcurrency)
		},
	}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	flags := cmd.Flags()
	flags.StringVar(&category, "category", "", "visual category: images or videos")
	flags.IntVar(&page, "page", 1, "page number starting from 1")
	flags.IntVar(&pageSize, "page-size", 5, "number of visual items per page")
	flags.StringVar(&downloadDir, "download-dir", "", "directory for downloaded assets; defaults to pavo_outputs/visuals/<category>")
	flags.IntVar(&downloadConcurrency, "download-concurrency", defaultVisualDownloadConcurrency, "parallel asset downloads (1-32)")
	return cmd
}

func newShortDramaListCommand(stdout, stderr io.Writer, deps *dependencies) *cobra.Command {
	var page int
	var pageSize int
	var downloadDir string
	var downloadConcurrency int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List and download the current user's completed short dramas",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return writeVisuals(stdout, deps, cmd, api.VisualCategoryShortDramaFinal, page, pageSize, downloadDir, downloadConcurrency)
		},
	}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	flags := cmd.Flags()
	flags.IntVar(&page, "page", 1, "page number starting from 1")
	flags.IntVar(&pageSize, "page-size", 5, "number of short dramas per page")
	flags.StringVar(&downloadDir, "download-dir", "", "directory for downloaded short dramas; defaults to pavo_outputs/visuals/short_drama_final")
	flags.IntVar(&downloadConcurrency, "download-concurrency", defaultVisualDownloadConcurrency, "parallel short-drama downloads (1-32)")
	return cmd
}

func writeVisuals(stdout io.Writer, deps *dependencies, cmd *cobra.Command, category api.VisualCategory, page, pageSize int, rawDownloadDir string, downloadConcurrency int) error {
	if downloadConcurrency < 1 || downloadConcurrency > maxVisualDownloadConcurrency {
		return fmt.Errorf("download_concurrency 必须在 1 到 %d 之间", maxVisualDownloadConcurrency)
	}
	downloadDir, err := visualDownloadDir(rawDownloadDir, category)
	if err != nil {
		return err
	}
	data, err := deps.api.ListVisuals(cmd.Context(), category, page, pageSize)
	if err != nil {
		return err
	}
	downloaded, failed, err := downloadVisualItems(cmd.Context(), deps, data, downloadDir, downloadConcurrency)
	if err != nil {
		return err
	}
	return output.WriteJSON(stdout, visualsOutput{
		Category:    category,
		DownloadDir: downloadDir,
		Downloaded:  downloaded,
		Failed:      failed,
		Pagination:  data.Pagination,
		Groups:      data.Groups,
	})
}

type visualDownloadMetadata struct {
	Mimetype    string `json:"mimetype"`
	URL         string `json:"url"`
	OriginalURL string `json:"original_url"`
}

type visualDownloadTask struct {
	item       *api.VisualItem
	url        string
	outputPath string
}

func visualDownloadDir(rawDir string, category api.VisualCategory) (string, error) {
	downloadDir := strings.TrimSpace(rawDir)
	if downloadDir == "" {
		workingDir, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("读取当前工作目录失败: %w", err)
		}
		downloadDir = filepath.Join(workingDir, "pavo_outputs", "visuals", string(category))
	}
	absDir, err := filepath.Abs(downloadDir)
	if err != nil {
		return "", fmt.Errorf("解析下载目录失败: %w", err)
	}
	return absDir, nil
}

func downloadVisualItems(ctx context.Context, deps *dependencies, data *api.VisualsData, downloadDir string, concurrency int) (int, int, error) {
	tasks := visualDownloadTasks(data, downloadDir)
	if len(tasks) == 0 {
		downloaded, failed := visualDownloadCounts(data)
		return downloaded, failed, nil
	}

	jobs := make(chan visualDownloadTask, len(tasks))
	for _, task := range tasks {
		jobs <- task
	}
	close(jobs)

	workerCount := min(concurrency, len(tasks))
	var workers sync.WaitGroup

	workers.Add(workerCount)
	for worker := 0; worker < workerCount; worker++ {
		go func() {
			defer workers.Done()
			for task := range jobs {
				response, err := deps.api.DownloadResult(ctx, api.DownloadResultOptions{
					URL:        task.url,
					OutputPath: task.outputPath,
				})
				if err != nil {
					task.item.DownloadError = err.Error()
					continue
				}
				task.item.LocalPath = response.OutputPath
			}
		}()
	}

	workers.Wait()
	if err := ctx.Err(); err != nil {
		return 0, 0, fmt.Errorf("个人资产下载中断: %w", err)
	}
	downloaded, failed := visualDownloadCounts(data)
	return downloaded, failed, nil
}

func visualDownloadTasks(data *api.VisualsData, downloadDir string) []visualDownloadTask {
	if data == nil {
		return nil
	}
	tasks := make([]visualDownloadTask, 0)
	itemNumber := 0
	for groupIndex := range data.Groups {
		group := &data.Groups[groupIndex]
		dateDir := safeFilenamePart(group.Date)
		if dateDir == "" {
			dateDir = "undated"
		}
		for itemIndex := range group.List {
			itemNumber++
			item := &group.List[itemIndex]
			item.LocalPath = ""
			item.DownloadError = ""
			metadata, err := parseVisualDownloadMetadata(item.Metadata)
			if err != nil {
				item.DownloadError = fmt.Sprintf("解析 metadata 失败: %v", err)
				continue
			}
			downloadURL := firstNonEmpty(item.URL, metadata.URL, metadata.OriginalURL)
			if downloadURL == "" {
				item.DownloadError = "缺少可下载 URL"
				continue
			}
			identifier := safeFilenamePart(string(item.VisualID))
			if identifier == "" {
				identifier = safeFilenamePart(string(item.ResourceID))
			}
			if identifier == "" {
				identifier = fmt.Sprintf("item-%03d", itemNumber)
			}
			filename := "visual-" + identifier + visualFileExtension(metadata.Mimetype, item.Type, downloadURL)
			tasks = append(tasks, visualDownloadTask{
				item:       item,
				url:        downloadURL,
				outputPath: filepath.Join(downloadDir, dateDir, filename),
			})
		}
	}
	return tasks
}

func visualDownloadCounts(data *api.VisualsData) (int, int) {
	if data == nil {
		return 0, 0
	}
	downloaded := 0
	failed := 0
	for groupIndex := range data.Groups {
		for itemIndex := range data.Groups[groupIndex].List {
			item := &data.Groups[groupIndex].List[itemIndex]
			if item.LocalPath != "" {
				downloaded++
			} else if item.DownloadError != "" {
				failed++
			}
		}
	}
	return downloaded, failed
}

func parseVisualDownloadMetadata(raw json.RawMessage) (visualDownloadMetadata, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return visualDownloadMetadata{}, nil
	}
	var metadata visualDownloadMetadata
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return visualDownloadMetadata{}, err
	}
	return metadata, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func visualFileExtension(mimetype, itemType, rawURL string) string {
	switch strings.ToLower(strings.TrimSpace(strings.Split(mimetype, ";")[0])) {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	case "video/mp4":
		return ".mp4"
	case "video/webm":
		return ".webm"
	}
	if parsed, err := url.Parse(rawURL); err == nil {
		switch extension := strings.ToLower(filepath.Ext(parsed.Path)); extension {
		case ".jpg", ".jpeg", ".png", ".webp", ".gif", ".mp4", ".webm", ".mov":
			return extension
		}
	}
	if strings.EqualFold(strings.TrimSpace(itemType), "image") {
		return ".jpg"
	}
	if strings.EqualFold(strings.TrimSpace(itemType), "video") {
		return ".mp4"
	}
	return ".bin"
}
