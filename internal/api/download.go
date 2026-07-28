package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	maxDownloadRetries = 3
	retryBaseDelay     = 500 * time.Millisecond
)

// DownloadResultOptions describes one generated asset to save locally.
// OutputPath must include the destination filename.
type DownloadResultOptions struct {
	URL        string
	OutputPath string
	UpdatedAt  int64
	Force      bool
}

// DownloadResultResponse is the JSON envelope printed by pavo download-result.
// The slice fields match the established Pippit CLI contract and leave room for
// a future multi-file command.
type DownloadResultResponse struct {
	OutputPath   string   `json:"output_path"`
	Downloaded   []string `json:"downloaded,omitempty"`
	AlreadyExist []string `json:"already_exist,omitempty"`
}

type downloadHTTPError struct {
	statusCode int
}

func (e *downloadHTTPError) Error() string {
	return fmt.Sprintf("下载返回 HTTP %d", e.statusCode)
}

type downloadRequestError struct {
	cause error
}

func (e *downloadRequestError) Error() string {
	return "下载请求失败"
}

func (e *downloadRequestError) Unwrap() error {
	return e.cause
}

// DownloadResult downloads a generated asset without forwarding the PAVO
// authorization token to the result host. It writes to a temporary file in the
// target directory and only replaces the destination once the transfer closes
// successfully.
func (c *Client) DownloadResult(ctx context.Context, opts DownloadResultOptions) (*DownloadResultResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, downloadContextError(err)
	}

	rawURL := strings.TrimSpace(opts.URL)
	if err := validateHTTPURL(rawURL, "下载 URL"); err != nil {
		return nil, err
	}
	outputPath := strings.TrimSpace(opts.OutputPath)
	if outputPath == "" {
		return nil, errors.New("输出路径不能为空")
	}
	if opts.UpdatedAt < 0 {
		return nil, errors.New("updated_at 不能小于 0")
	}

	if info, err := os.Stat(outputPath); err == nil {
		if info.IsDir() {
			return nil, fmt.Errorf("输出路径 %q 是目录，请指定文件", outputPath)
		}
		if !opts.Force && shouldSkipExistingDownload(info, opts.UpdatedAt) {
			return &DownloadResultResponse{
				OutputPath:   outputPath,
				AlreadyExist: []string{outputPath},
			}, nil
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("获取输出路径信息失败: %w", err)
	}

	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, fmt.Errorf("创建输出目录失败: %w", err)
	}
	if err := c.downloadFileWithRetry(ctx, rawURL, outputPath, opts.UpdatedAt); err != nil {
		return nil, err
	}

	return &DownloadResultResponse{
		OutputPath: outputPath,
		Downloaded: []string{outputPath},
	}, nil
}

func shouldSkipExistingDownload(info os.FileInfo, updatedAt int64) bool {
	return updatedAt <= 0 || info.ModTime().Unix() >= updatedAt
}

func (c *Client) downloadFileWithRetry(ctx context.Context, rawURL, targetPath string, updatedAt int64) error {
	var lastErr error
	for attempt := 0; attempt <= maxDownloadRetries; attempt++ {
		if attempt > 0 {
			delay := retryBaseDelay * time.Duration(1<<(attempt-1))
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return downloadContextError(ctx.Err())
			case <-timer.C:
			}
		}

		err := c.downloadFile(ctx, rawURL, targetPath, updatedAt)
		if err == nil {
			return nil
		}
		lastErr = err
		if !isRetryableDownloadError(err) {
			return err
		}
	}
	return fmt.Errorf("下载重试 %d 次后仍失败: %w", maxDownloadRetries, lastErr)
}

func isRetryableDownloadError(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var statusErr *downloadHTTPError
	if errors.As(err, &statusErr) {
		return statusErr.statusCode == http.StatusRequestTimeout || statusErr.statusCode == http.StatusTooManyRequests || statusErr.statusCode >= http.StatusInternalServerError
	}
	var urlErr *url.Error
	return errors.As(err, &urlErr) && (urlErr.Timeout() || urlErr.Temporary())
}

func (c *Client) downloadFile(ctx context.Context, rawURL, targetPath string, updatedAt int64) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return errors.New("构造下载请求失败")
	}
	// Result URLs are commonly CDN or object-store URLs. Do not call authorize
	// or set X-Platform here: neither PAVO credentials nor API-only headers
	// should leave the PAVO API host.
	req.Header.Set("User-Agent", pavoUserAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return downloadContextError(ctxErr)
		}
		return &downloadRequestError{cause: err}
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64*1024))
		return &downloadHTTPError{statusCode: resp.StatusCode}
	}

	temporary, err := os.CreateTemp(filepath.Dir(targetPath), "."+filepath.Base(targetPath)+".*.tmp")
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %w", err)
	}
	temporaryPath := temporary.Name()
	temporaryActive := true
	defer func() {
		if temporaryActive {
			_ = os.Remove(temporaryPath)
		}
	}()

	_, copyErr := io.Copy(temporary, resp.Body)
	closeErr := temporary.Close()
	if copyErr != nil {
		return fmt.Errorf("写入临时文件失败: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("关闭临时文件失败: %w", closeErr)
	}
	if err := os.Rename(temporaryPath, targetPath); err != nil {
		return fmt.Errorf("替换目标文件失败: %w", err)
	}
	temporaryActive = false
	if updatedAt > 0 {
		modTime := time.Unix(updatedAt, 0)
		_ = os.Chtimes(targetPath, time.Now(), modTime)
	}
	return nil
}

func downloadContextError(err error) error {
	if errors.Is(err, context.Canceled) {
		return errors.New("下载已取消")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return errors.New("下载超时")
	}
	return fmt.Errorf("下载上下文异常: %w", err)
}
