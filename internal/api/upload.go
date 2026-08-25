package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const chatAttachmentPurpose = "chat_attachment"

type uploadFileMetadata struct {
	path        string
	filename    string
	contentType string
	size        int64
}

// UploadFile uploads a local file through PAVO's presigned URL flow and returns
// the stable public URL. The temporary signed URL is never returned to callers.
func (c *Client) UploadFile(ctx context.Context, path string) (*FileUploadResult, error) {
	return c.UploadFileWithPurpose(ctx, path, chatAttachmentPurpose)
}

// UploadCanvasFile uploads an image, video, or audio file using the UGC
// purpose expected by canvas nodes.
func (c *Client) UploadCanvasFile(ctx context.Context, path string) (*FileUploadResult, error) {
	metadata, err := inspectUploadFile(path)
	if err != nil {
		return nil, err
	}
	purpose, err := canvasUploadPurpose(metadata.contentType)
	if err != nil {
		return nil, err
	}
	return c.uploadFileWithMetadata(ctx, metadata, purpose)
}

// UploadFileWithPurpose exposes the presigned upload flow to API features that
// require a purpose other than chat_attachment.
func (c *Client) UploadFileWithPurpose(ctx context.Context, path, purpose string) (*FileUploadResult, error) {
	metadata, err := inspectUploadFile(path)
	if err != nil {
		return nil, err
	}
	purpose = strings.TrimSpace(purpose)
	if purpose == "" {
		return nil, errors.New("上传 purpose 不能为空")
	}
	return c.uploadFileWithMetadata(ctx, metadata, purpose)
}

func canvasUploadPurpose(contentType string) (string, error) {
	switch {
	case strings.HasPrefix(contentType, "image/"):
		return "ugc_image", nil
	case strings.HasPrefix(contentType, "video/"):
		return "ugc_video", nil
	case strings.HasPrefix(contentType, "audio/"):
		return "ugc_audio", nil
	default:
		return "", fmt.Errorf("画布仅支持图片、视频或音频文件，实际 Content-Type 为 %q", contentType)
	}
}

func (c *Client) uploadFileWithMetadata(ctx context.Context, metadata uploadFileMetadata, purpose string) (*FileUploadResult, error) {

	var response struct {
		Code    string           `json:"code"`
		Message string           `json:"message"`
		Data    PresignedURLData `json:"data"`
	}
	if err := c.doJSON(ctx, http.MethodPost, c.paths.PresignedURL, PresignedURLRequest{
		Purpose:     purpose,
		ContentType: metadata.contentType,
		Filename:    metadata.filename,
	}, true, &response); err != nil {
		return nil, fmt.Errorf("获取文件预上传地址失败: %w", err)
	}
	if err := validateEnvelope(response.Code, response.Message); err != nil {
		return nil, fmt.Errorf("获取文件预上传地址失败: %w", err)
	}
	if err := validatePresignedURLData(response.Data); err != nil {
		return nil, err
	}
	if err := c.putPresignedFile(ctx, response.Data, metadata); err != nil {
		return nil, err
	}

	return &FileUploadResult{
		PublicURL:   strings.TrimSpace(response.Data.PublicURL),
		ContentType: metadata.contentType,
		Filename:    metadata.filename,
	}, nil
}

func inspectUploadFile(path string) (uploadFileMetadata, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return uploadFileMetadata{}, errors.New("上传文件路径不能为空")
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return uploadFileMetadata{}, fmt.Errorf("上传文件不存在: %s", path)
		}
		if os.IsPermission(err) {
			return uploadFileMetadata{}, fmt.Errorf("没有权限读取上传文件: %s", path)
		}
		return uploadFileMetadata{}, fmt.Errorf("获取上传文件信息失败: %w", err)
	}
	if info.IsDir() {
		return uploadFileMetadata{}, fmt.Errorf("上传路径 %q 是目录，请指定文件", path)
	}

	contentType, err := detectFileContentType(path)
	if err != nil {
		return uploadFileMetadata{}, err
	}
	return uploadFileMetadata{
		path:        path,
		filename:    filepath.Base(path),
		contentType: contentType,
		size:        info.Size(),
	}, nil
}

func detectFileContentType(path string) (string, error) {
	if contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(path))); contentType != "" {
		return contentType, nil
	}

	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("打开上传文件失败: %w", err)
	}
	defer file.Close()

	buffer := make([]byte, 512)
	n, readErr := file.Read(buffer)
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return "", fmt.Errorf("读取上传文件失败: %w", readErr)
	}
	if n == 0 {
		return "application/octet-stream", nil
	}
	return http.DetectContentType(buffer[:n]), nil
}

func validatePresignedURLData(data PresignedURLData) error {
	if err := validateHTTPURL(data.UploadURL, "data.upload_url"); err != nil {
		return err
	}
	if err := validateHTTPURL(data.PublicURL, "data.public_url"); err != nil {
		return err
	}
	if strings.ToUpper(strings.TrimSpace(data.Method)) != http.MethodPut {
		return fmt.Errorf("预上传响应 data.method 必须为 PUT，实际为 %q", data.Method)
	}
	if !hasRequiredHeader(data.RequiredHeaders, "Content-Type") {
		return errors.New("预上传响应缺少 data.required_headers.Content-Type")
	}
	return nil
}

func hasRequiredHeader(headers map[string]string, target string) bool {
	for key, value := range headers {
		if strings.EqualFold(strings.TrimSpace(key), target) && strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}

func validateHTTPURL(raw, field string) error {
	parsed, err := url.ParseRequestURI(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("%s 不是有效的 HTTP URL", field)
	}
	return nil
}

func (c *Client) putPresignedFile(ctx context.Context, data PresignedURLData, metadata uploadFileMetadata) error {
	file, err := os.Open(metadata.path)
	if err != nil {
		return fmt.Errorf("打开上传文件失败: %w", err)
	}
	defer file.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, strings.TrimSpace(data.UploadURL), file)
	if err != nil {
		return fmt.Errorf("构造文件直传请求失败: %w", err)
	}
	req.ContentLength = metadata.size
	for key, value := range data.RequiredHeaders {
		key = strings.TrimSpace(key)
		if key == "" {
			return errors.New("预上传响应包含空的 required_headers 键")
		}
		req.Header.Set(key, value)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return newPresignedUploadRequestError(err)
	}
	defer resp.Body.Close()
	if _, err := io.Copy(io.Discard, io.LimitReader(resp.Body, 64*1024)); err != nil {
		return fmt.Errorf("读取文件直传响应失败: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("文件直传返回 HTTP %d", resp.StatusCode)
	}
	return nil
}

func newPresignedUploadRequestError(err error) error {
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return fmt.Errorf("文件直传请求失败: %w", urlError.Err)
	}
	return errors.New("文件直传请求失败")
}
