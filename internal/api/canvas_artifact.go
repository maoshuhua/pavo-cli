package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type CanvasArtifact struct {
	ID                ScalarString    `json:"id,omitempty"`
	ArtifactUUID      string          `json:"artifact_uuid"`
	CanvasID          ScalarString    `json:"canvas_id,omitempty"`
	UserID            ScalarString    `json:"user_id,omitempty"`
	NodeKey           string          `json:"node_key,omitempty"`
	NodeName          string          `json:"node_name,omitempty"`
	NodeType          string          `json:"node_type,omitempty"`
	RunID             ScalarString    `json:"run_id,omitempty"`
	RunNodeID         ScalarString    `json:"run_node_id,omitempty"`
	TaskID            ScalarString    `json:"task_id,omitempty"`
	OutputIndex       int             `json:"output_index,omitempty"`
	ArtifactType      string          `json:"artifact_type,omitempty"`
	URL               string          `json:"url,omitempty"`
	ThumbnailURL      string          `json:"thumbnail_url,omitempty"`
	PosterURL         string          `json:"poster_url,omitempty"`
	TextContent       string          `json:"text_content,omitempty"`
	Width             int             `json:"width,omitempty"`
	Height            int             `json:"height,omitempty"`
	DurationMS        int64           `json:"duration_ms,omitempty"`
	MIMEType          string          `json:"mime_type,omitempty"`
	Model             string          `json:"model,omitempty"`
	Prompt            string          `json:"prompt,omitempty"`
	InputSnapshotJSON json.RawMessage `json:"input_snapshot_json,omitempty"`
	MetadataJSON      json.RawMessage `json:"metadata_json,omitempty"`
	VisibilityStatus  string          `json:"visibility_status,omitempty"`
	SavedVisualID     ScalarString    `json:"saved_visual_id,omitempty"`
	SavedAtMS         int64           `json:"saved_at_ms,omitempty"`
	CreatedAt         json.RawMessage `json:"created_at,omitempty"`
	UpdatedAt         json.RawMessage `json:"updated_at,omitempty"`
	LocalPath         string          `json:"local_path,omitempty"`
	DownloadError     string          `json:"download_error,omitempty"`
}

type CanvasArtifactDateGroup struct {
	Date string           `json:"date"`
	List []CanvasArtifact `json:"list"`
}

type CanvasArtifactPagination struct {
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
	Total    int `json:"total"`
}

type CanvasArtifactList struct {
	Groups     []CanvasArtifactDateGroup `json:"groups"`
	Pagination CanvasArtifactPagination  `json:"pagination"`
}

type CanvasMediaAssetItem struct {
	NodeKey       string `json:"nodeKey"`
	ResourceIndex int    `json:"resourceIndex,omitempty"`
	Name          string `json:"name,omitempty"`
}

type CanvasMediaAssetsRequest struct {
	CanvasUUID string                 `json:"canvasUuid"`
	Items      []CanvasMediaAssetItem `json:"items"`
}

func canvasArtifactsPath(projectUUID string) string {
	return canvasProjectPath(projectUUID) + "/artifacts"
}

func (c *Client) ListCanvasArtifacts(ctx context.Context, projectUUID, canvasUUID, category string, page, pageSize int) (*CanvasArtifactList, error) {
	projectUUID = strings.TrimSpace(projectUUID)
	if projectUUID == "" {
		return nil, errors.New("project_uuid 不能为空")
	}
	if page < 1 {
		return nil, errors.New("page 必须大于 0")
	}
	if pageSize < 1 || pageSize > 100 {
		return nil, errors.New("page_size 必须是 1 到 100")
	}
	switch strings.ToLower(strings.TrimSpace(category)) {
	case "", "all":
		category = "all"
	case "image", "images":
		category = "images"
	case "video", "videos":
		category = "videos"
	default:
		return nil, errors.New("category 必须是 all、images 或 videos")
	}
	query := url.Values{"category": {category}, "page": {strconv.Itoa(page)}, "page_size": {strconv.Itoa(pageSize)}}
	if strings.TrimSpace(canvasUUID) != "" {
		query.Set("canvas_uuid", strings.TrimSpace(canvasUUID))
	}
	var response struct {
		Code    string             `json:"code"`
		Message string             `json:"message"`
		Data    CanvasArtifactList `json:"data"`
	}
	if err := c.getJSON(ctx, canvasArtifactsPath(projectUUID), query, &response); err != nil {
		return nil, err
	}
	if err := validateEnvelope(response.Code, response.Message); err != nil {
		return nil, err
	}
	if response.Data.Groups == nil {
		response.Data.Groups = []CanvasArtifactDateGroup{}
	}
	for index := range response.Data.Groups {
		if response.Data.Groups[index].List == nil {
			response.Data.Groups[index].List = []CanvasArtifact{}
		}
	}
	return &response.Data, nil
}

func (c *Client) DeleteCanvasArtifact(ctx context.Context, projectUUID, artifactUUID string) (json.RawMessage, error) {
	artifactUUID = strings.TrimSpace(artifactUUID)
	if artifactUUID == "" {
		return nil, errors.New("artifact_uuid 不能为空")
	}
	if len(artifactUUID) > 64 {
		return nil, errors.New("artifact_uuid 最长 64 字符")
	}
	var response struct {
		Code    string          `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := c.doJSON(ctx, http.MethodDelete, canvasArtifactsPath(projectUUID)+"/"+url.PathEscape(artifactUUID), nil, true, &response); err != nil {
		return nil, err
	}
	if err := validateEnvelope(response.Code, response.Message); err != nil {
		return nil, err
	}
	return response.Data, nil
}

func (c *Client) BatchDeleteCanvasArtifacts(ctx context.Context, projectUUID string, artifactUUIDs []string) (json.RawMessage, error) {
	if len(artifactUUIDs) == 0 || len(artifactUUIDs) > 100 {
		return nil, errors.New("artifact_uuids 数量必须是 1 到 100")
	}
	normalized := make([]string, 0, len(artifactUUIDs))
	seen := map[string]bool{}
	for _, value := range artifactUUIDs {
		value = strings.TrimSpace(value)
		if value == "" || len(value) > 64 {
			return nil, fmt.Errorf("artifact_uuid %q 无效", value)
		}
		if !seen[value] {
			normalized = append(normalized, value)
			seen[value] = true
		}
	}
	var response struct {
		Code    string          `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := c.doJSON(ctx, http.MethodPost, canvasArtifactsPath(projectUUID)+"/batch-delete", map[string]any{"artifact_uuids": normalized}, true, &response); err != nil {
		return nil, err
	}
	if err := validateEnvelope(response.Code, response.Message); err != nil {
		return nil, err
	}
	return response.Data, nil
}

func (c *Client) CreateCanvasMediaAssets(ctx context.Context, projectUUID string, request CanvasMediaAssetsRequest) (json.RawMessage, error) {
	if strings.TrimSpace(request.CanvasUUID) == "" {
		return nil, errors.New("canvasUuid 不能为空")
	}
	if len(request.Items) == 0 || len(request.Items) > 100 {
		return nil, errors.New("items 数量必须是 1 到 100")
	}
	for index := range request.Items {
		request.Items[index].NodeKey = strings.TrimSpace(request.Items[index].NodeKey)
		request.Items[index].Name = strings.TrimSpace(request.Items[index].Name)
		if request.Items[index].NodeKey == "" {
			return nil, fmt.Errorf("items[%d].nodeKey 不能为空", index)
		}
		if request.Items[index].ResourceIndex < 0 {
			return nil, fmt.Errorf("items[%d].resourceIndex 不能小于 0", index)
		}
	}
	var response struct {
		Code    string          `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := c.doJSON(ctx, http.MethodPost, canvasProjectPath(projectUUID)+"/media-assets/batch-create", request, true, &response); err != nil {
		return nil, err
	}
	if err := validateEnvelope(response.Code, response.Message); err != nil {
		return nil, err
	}
	return response.Data, nil
}
