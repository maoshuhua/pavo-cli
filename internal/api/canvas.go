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

const (
	CanvasProjectCreatePath      = "/api/v1/pixa/canvas/project/create"
	CanvasProjectDetailPath      = "/api/v1/pixa/canvas/project/detail"
	CanvasFolderEntriesPath      = "/api/v1/pixa/canvas/folder/entries"
	CanvasToolSpecsPath          = "/api/v1/pixa/canvas/tool-specs"
	CanvasModelOptionsPath       = "/api/v1/pixa/model_options"
	CanvasGenerationProgressPath = "/api/v1/pixa/canvas/generation/progress"
	CanvasGenerationCancelPath   = "/api/v1/pixa/canvas/generation/cancel"
)

// ScalarString accepts either a JSON string or number. Pixa uses both forms
// for identifiers in older and newer canvas records.
type ScalarString string

func (value *ScalarString) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*value = ""
		return nil
	}
	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		*value = ScalarString(text)
		return nil
	}
	var number json.Number
	if err := json.Unmarshal(data, &number); err != nil {
		return fmt.Errorf("值必须是字符串或数字: %w", err)
	}
	*value = ScalarString(number.String())
	return nil
}

func (value ScalarString) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(value))
}

// FlexibleInt64 accepts an integer encoded as either a number or a string.
type FlexibleInt64 int64

func (value *FlexibleInt64) UnmarshalJSON(data []byte) error {
	if string(data) == "null" || len(data) == 0 {
		*value = 0
		return nil
	}
	var number json.Number
	if err := json.Unmarshal(data, &number); err == nil {
		parsed, parseErr := strconv.ParseInt(number.String(), 10, 64)
		if parseErr == nil {
			*value = FlexibleInt64(parsed)
			return nil
		}
	}
	var text string
	if err := json.Unmarshal(data, &text); err != nil {
		return fmt.Errorf("值必须是整数或整数字符串: %w", err)
	}
	parsed, err := strconv.ParseInt(strings.TrimSpace(text), 10, 64)
	if err != nil {
		return fmt.Errorf("解析整数 %q 失败: %w", text, err)
	}
	*value = FlexibleInt64(parsed)
	return nil
}

type CanvasProjectEntry struct {
	ProjectID        ScalarString `json:"project_id,omitempty"`
	ID               ScalarString `json:"id,omitempty"`
	Title            string       `json:"title"`
	CoverURL         string       `json:"cover_url"`
	CreatedAt        string       `json:"created_at"`
	UpdatedAt        string       `json:"updated_at"`
	CanvasUUID       string       `json:"canvas_uuid,omitempty"`
	LatestCanvasUUID string       `json:"latest_canvas_uuid,omitempty"`
	ProjectUUID      string       `json:"project_uuid,omitempty"`
	Status           string       `json:"status,omitempty"`
	CanvasCount      int          `json:"canvas_count,omitempty"`
	CanvasURL        string       `json:"canvas_url,omitempty"`
}

type CanvasFolderEntries struct {
	Items []CanvasProjectEntry `json:"items"`
	Total int                  `json:"total"`
}

type CanvasProjectMeta struct {
	ID          ScalarString `json:"id,omitempty"`
	ProjectUUID string       `json:"project_uuid,omitempty"`
	Title       string       `json:"title,omitempty"`
	CoverURL    string       `json:"cover_url,omitempty"`
}

type CanvasInfo struct {
	ID         ScalarString `json:"id,omitempty"`
	CanvasUUID string       `json:"canvas_uuid,omitempty"`
	ProjectID  ScalarString `json:"project_id,omitempty"`
	Title      string       `json:"title,omitempty"`
}

type CanvasNodePosition struct {
	PositionX ScalarString `json:"position_x"`
	PositionY ScalarString `json:"position_y"`
}

type CanvasNodeMeasured struct {
	Width  ScalarString `json:"width,omitempty"`
	Height ScalarString `json:"height,omitempty"`
}

// CanvasNode keeps data as raw JSON so CLI mutations do not discard fields
// introduced by newer versions of pavo-app-front.
type CanvasNode struct {
	ID             ScalarString       `json:"id,omitempty"`
	NodeKey        string             `json:"node_key"`
	NodeType       ScalarString       `json:"node_type"`
	Name           string             `json:"name,omitempty"`
	Position       CanvasNodePosition `json:"position"`
	Measured       CanvasNodeMeasured `json:"measured,omitempty"`
	ParentKey      string             `json:"parent_key,omitempty"`
	Data           json.RawMessage    `json:"data"`
	ContentVersion FlexibleInt64      `json:"content_version,omitempty"`
	ZIndex         json.RawMessage    `json:"z_index,omitempty"`
}

type CanvasConnection struct {
	ID             ScalarString    `json:"id,omitempty"`
	ConnectionID   string          `json:"connection_id"`
	SourceNodeKey  string          `json:"source_node_key"`
	TargetNodeKey  string          `json:"target_node_key"`
	ConnectionType string          `json:"connection_type,omitempty"`
	Type           string          `json:"type,omitempty"`
	SourceHandle   string          `json:"source_handle,omitempty"`
	TargetHandle   string          `json:"target_handle,omitempty"`
	SourcePortType string          `json:"source_port_type,omitempty"`
	TargetPortType string          `json:"target_port_type,omitempty"`
	Role           string          `json:"role,omitempty"`
	MediaOrder     json.RawMessage `json:"media_order,omitempty"`
	ColorKey       string          `json:"color_key,omitempty"`
	Selectable     *bool           `json:"selectable,omitempty"`
	Deletable      *bool           `json:"deletable,omitempty"`
	Style          json.RawMessage `json:"style,omitempty"`
	StyleJSON      json.RawMessage `json:"style_json,omitempty"`
}

type CanvasProjectDetail struct {
	ProjectMeta    CanvasProjectMeta  `json:"project_meta"`
	CurrentCanvas  CanvasInfo         `json:"current_canvas"`
	CanvasList     json.RawMessage    `json:"canvas_list,omitempty"`
	NodeList       []CanvasNode       `json:"node_list"`
	ConnectionList []CanvasConnection `json:"connection_list"`
	ProjectDraft   json.RawMessage    `json:"project_draft,omitempty"`
	Version        FlexibleInt64      `json:"version"`
	GraphVersion   FlexibleInt64      `json:"graph_version,omitempty"`
	LayoutVersion  FlexibleInt64      `json:"layout_version,omitempty"`
}

type CreateCanvasProjectRequest struct {
	Title    string `json:"title"`
	CoverURL string `json:"cover_url"`
}

type CanvasProjectCreated struct {
	CanvasUUID  string            `json:"canvas_uuid"`
	ProjectUUID string            `json:"project_uuid"`
	Canvas      CanvasInfo        `json:"canvas"`
	Project     CanvasProjectMeta `json:"project"`
}

type UpdateCanvasProjectRequest struct {
	Title    *string `json:"title,omitempty"`
	CoverURL *string `json:"cover_url,omitempty"`
}

type CanvasBatchNodeWriteItem struct {
	NodeKey  string `json:"nodeKey"`
	Type     string `json:"type"`
	Name     string `json:"name"`
	Position struct {
		PositionX string `json:"positionX"`
		PositionY string `json:"positionY"`
	} `json:"position"`
	Measured struct {
		Width  string `json:"width"`
		Height string `json:"height"`
	} `json:"measured"`
	ParentKey string `json:"parentKey"`
	Data      string `json:"data"`
}

type CanvasBatchNodes struct {
	Create []CanvasBatchNodeWriteItem `json:"create"`
	Update []CanvasBatchNodeWriteItem `json:"update"`
	Delete []string                   `json:"delete"`
}

type CanvasBatchConnectionWriteItem struct {
	ConnectionID   string          `json:"connection_id"`
	Source         string          `json:"source"`
	Target         string          `json:"target"`
	SourceHandle   string          `json:"source_handle"`
	TargetHandle   string          `json:"target_handle"`
	SourcePortType string          `json:"source_port_type"`
	TargetPortType string          `json:"target_port_type"`
	Role           string          `json:"role"`
	MediaOrder     int             `json:"media_order"`
	ConnectionType string          `json:"connection_type"`
	ColorKey       string          `json:"color_key"`
	Selectable     bool            `json:"selectable"`
	Deletable      bool            `json:"deletable"`
	Style          json.RawMessage `json:"style_json,omitempty"`
}

type CanvasBatchConnectionDeleteItem struct {
	ConnectionID string `json:"connection_id"`
}

type CanvasBatchConnections struct {
	Create []CanvasBatchConnectionWriteItem  `json:"create"`
	Delete []CanvasBatchConnectionDeleteItem `json:"delete"`
}

type CanvasBatchRequest struct {
	CanvasUUID  string                 `json:"canvasUuid"`
	Nodes       CanvasBatchNodes       `json:"nodes"`
	Connections CanvasBatchConnections `json:"connections"`
	Version     int64                  `json:"version"`
	RequestID   string                 `json:"requestId"`
	SessionID   string                 `json:"sessionId"`
	Timestamp   int64                  `json:"timestamp"`
}

type CanvasBatchResult struct {
	Version FlexibleInt64 `json:"version,omitempty"`
}

type CreateCanvasGenerationRequest struct {
	NodeKey          string `json:"nodeKey"`
	RequestID        string `json:"requestId"`
	CanvasUUID       string `json:"canvasUuid,omitempty"`
	ExecutionBatchID string `json:"executionBatchId,omitempty"`
	BatchOrder       int    `json:"batchOrder,omitempty"`
	BatchTotal       int    `json:"batchTotal,omitempty"`
}

type CanvasGenerationCreated struct {
	TaskID                  ScalarString  `json:"task_id"`
	TaskIDCamel             ScalarString  `json:"taskId,omitempty"`
	EstimatedGenerationTime float64       `json:"estimated_generation_time,omitempty"`
	Version                 FlexibleInt64 `json:"version,omitempty"`
	Status                  string        `json:"status,omitempty"`
	ErrorCode               string        `json:"errorCode,omitempty"`
	TraceID                 string        `json:"trace_id,omitempty"`
}

func (created CanvasGenerationCreated) EffectiveTaskID() string {
	if strings.TrimSpace(string(created.TaskID)) != "" {
		return strings.TrimSpace(string(created.TaskID))
	}
	return strings.TrimSpace(string(created.TaskIDCamel))
}

type CanvasGenerationProgress struct {
	EndTimeMS       int64           `json:"endTimeMs,omitempty"`
	ErrorCode       string          `json:"errorCode,omitempty"`
	ErrorMessage    string          `json:"errorMessage,omitempty"`
	Power           float64         `json:"power,omitempty"`
	ProgressPercent float64         `json:"progressPercent"`
	StartTimeMS     int64           `json:"startTimeMs,omitempty"`
	Status          int             `json:"status"`
	StatusText      string          `json:"statusText,omitempty"`
	TaskID          ScalarString    `json:"taskId"`
	TaskResult      json.RawMessage `json:"taskResult,omitempty"`
	Version         FlexibleInt64   `json:"version,omitempty"`
}

func (progress CanvasGenerationProgress) Terminal() bool {
	if progress.Status == 2 || progress.Status == 3 {
		return true
	}
	status := strings.ToLower(strings.TrimSpace(progress.StatusText))
	return status == "failed" || status == "error" || status == "cancelled" || status == "canceled"
}

func (progress CanvasGenerationProgress) Failed() bool {
	if progress.Status == 3 || strings.TrimSpace(progress.ErrorCode) != "" {
		return true
	}
	status := strings.ToLower(strings.TrimSpace(progress.StatusText))
	return status == "failed" || status == "error" || status == "cancelled" || status == "canceled"
}

type CanvasGenerationProgressData struct {
	Progresses []CanvasGenerationProgress `json:"progresses"`
}

type CanvasGenerationCancelResult struct {
	Cancelled bool          `json:"cancelled"`
	Message   string        `json:"message,omitempty"`
	Version   FlexibleInt64 `json:"version,omitempty"`
}

func canvasProjectPath(projectUUID string) string {
	return "/api/v1/pixa/canvas/project/" + url.PathEscape(strings.TrimSpace(projectUUID))
}

func (c *Client) ListCanvasProjects(ctx context.Context) (*CanvasFolderEntries, error) {
	var response struct {
		Code    string              `json:"code"`
		Message string              `json:"message"`
		Data    CanvasFolderEntries `json:"data"`
	}
	if err := c.getJSON(ctx, CanvasFolderEntriesPath, nil, &response); err != nil {
		return nil, err
	}
	if err := validateEnvelope(response.Code, response.Message); err != nil {
		return nil, err
	}
	if response.Data.Items == nil {
		response.Data.Items = []CanvasProjectEntry{}
	}
	return &response.Data, nil
}

func (c *Client) CreateCanvasProject(ctx context.Context, title, coverURL string) (*CanvasProjectCreated, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, errors.New("title 不能为空")
	}
	var response struct {
		Code    string               `json:"code"`
		Message string               `json:"message"`
		Data    CanvasProjectCreated `json:"data"`
	}
	err := c.doJSON(ctx, http.MethodPost, CanvasProjectCreatePath, CreateCanvasProjectRequest{Title: title, CoverURL: strings.TrimSpace(coverURL)}, true, &response)
	if err != nil {
		return nil, err
	}
	if err := validateEnvelope(response.Code, response.Message); err != nil {
		return nil, err
	}
	if strings.TrimSpace(response.Data.ProjectUUID) == "" || strings.TrimSpace(response.Data.CanvasUUID) == "" {
		return nil, errors.New("创建画布项目响应缺少 project_uuid 或 canvas_uuid")
	}
	return &response.Data, nil
}

func (c *Client) GetCanvasProjectDetail(ctx context.Context, projectUUID, canvasUUID string) (*CanvasProjectDetail, error) {
	projectUUID = strings.TrimSpace(projectUUID)
	if projectUUID == "" {
		return nil, errors.New("project_uuid 不能为空")
	}
	query := url.Values{"uuid": {projectUUID}}
	if canvasUUID = strings.TrimSpace(canvasUUID); canvasUUID != "" {
		query.Set("canvas_uuid", canvasUUID)
	}
	var response struct {
		Code    string              `json:"code"`
		Message string              `json:"message"`
		Data    CanvasProjectDetail `json:"data"`
	}
	if err := c.getJSON(ctx, CanvasProjectDetailPath, query, &response); err != nil {
		return nil, err
	}
	if err := validateEnvelope(response.Code, response.Message); err != nil {
		return nil, err
	}
	if response.Data.NodeList == nil {
		response.Data.NodeList = []CanvasNode{}
	}
	if response.Data.ConnectionList == nil {
		response.Data.ConnectionList = []CanvasConnection{}
	}
	return &response.Data, nil
}

func (c *Client) UpdateCanvasProject(ctx context.Context, projectUUID string, request UpdateCanvasProjectRequest) (json.RawMessage, error) {
	var response struct {
		Code    string          `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := c.doJSON(ctx, http.MethodPut, canvasProjectPath(projectUUID), request, true, &response); err != nil {
		return nil, err
	}
	if err := validateEnvelope(response.Code, response.Message); err != nil {
		return nil, err
	}
	return response.Data, nil
}

func (c *Client) DeleteCanvasProject(ctx context.Context, projectUUID string) (json.RawMessage, error) {
	var response struct {
		Code    string          `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := c.doJSON(ctx, http.MethodDelete, canvasProjectPath(projectUUID), nil, true, &response); err != nil {
		return nil, err
	}
	if err := validateEnvelope(response.Code, response.Message); err != nil {
		return nil, err
	}
	return response.Data, nil
}

func (c *Client) DuplicateCanvasProject(ctx context.Context, projectUUID string) (*CanvasProjectCreated, error) {
	var response struct {
		Code    string               `json:"code"`
		Message string               `json:"message"`
		Data    CanvasProjectCreated `json:"data"`
	}
	if err := c.doJSON(ctx, http.MethodPost, canvasProjectPath(projectUUID)+"/duplicate", nil, true, &response); err != nil {
		return nil, err
	}
	if err := validateEnvelope(response.Code, response.Message); err != nil {
		return nil, err
	}
	return &response.Data, nil
}

func (c *Client) BatchCanvasProjectNodes(ctx context.Context, projectUUID string, request CanvasBatchRequest) (*CanvasBatchResult, error) {
	var response struct {
		Code    string            `json:"code"`
		Message string            `json:"message"`
		Data    CanvasBatchResult `json:"data"`
	}
	if err := c.doJSON(ctx, http.MethodPost, canvasProjectPath(projectUUID)+"/nodes/batch", request, true, &response); err != nil {
		return nil, err
	}
	if err := validateEnvelope(response.Code, response.Message); err != nil {
		return nil, err
	}
	return &response.Data, nil
}

func (c *Client) GetCanvasModelOptions(ctx context.Context, sceneCode string) (json.RawMessage, error) {
	sceneCode = strings.TrimSpace(sceneCode)
	if sceneCode == "" {
		return nil, errors.New("scene_code 不能为空")
	}
	var response struct {
		Code    string          `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := c.getJSON(ctx, CanvasModelOptionsPath, url.Values{"scene_code": {sceneCode}}, &response); err != nil {
		return nil, err
	}
	if err := validateEnvelope(response.Code, response.Message); err != nil {
		return nil, err
	}
	return response.Data, nil
}

func (c *Client) GetCanvasToolSpecs(ctx context.Context) (json.RawMessage, error) {
	var response struct {
		Code    string          `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := c.getJSON(ctx, CanvasToolSpecsPath, nil, &response); err != nil {
		return nil, err
	}
	if err := validateEnvelope(response.Code, response.Message); err != nil {
		return nil, err
	}
	return response.Data, nil
}

func (c *Client) CreateCanvasGeneration(ctx context.Context, projectUUID string, request CreateCanvasGenerationRequest) (*CanvasGenerationCreated, error) {
	var response struct {
		Code    string                  `json:"code"`
		Message string                  `json:"message"`
		Data    CanvasGenerationCreated `json:"data"`
	}
	if err := c.doJSON(ctx, http.MethodPost, canvasProjectPath(projectUUID)+"/generation/create", request, true, &response); err != nil {
		return nil, err
	}
	if err := validateEnvelope(response.Code, response.Message); err != nil {
		return nil, err
	}
	if response.Data.Status == "failed" || strings.TrimSpace(response.Data.ErrorCode) != "" {
		return nil, &APIError{Code: response.Data.ErrorCode, Message: firstNonEmpty(response.Message, "创建画布生成任务失败")}
	}
	if response.Data.EffectiveTaskID() == "" {
		return nil, errors.New("创建画布生成任务响应缺少 task_id")
	}
	return &response.Data, nil
}

func (c *Client) GetCanvasGenerationProgress(ctx context.Context, taskIDs []string) (*CanvasGenerationProgressData, error) {
	normalized := make([]string, 0, len(taskIDs))
	for _, taskID := range taskIDs {
		if taskID = strings.TrimSpace(taskID); taskID != "" {
			normalized = append(normalized, taskID)
		}
	}
	if len(normalized) == 0 {
		return nil, errors.New("taskIds 不能为空")
	}
	var response struct {
		Code    string                       `json:"code"`
		Message string                       `json:"message"`
		Data    CanvasGenerationProgressData `json:"data"`
	}
	if err := c.doJSON(ctx, http.MethodPost, CanvasGenerationProgressPath, map[string]any{"taskIds": normalized}, true, &response); err != nil {
		return nil, err
	}
	if err := validateEnvelope(response.Code, response.Message); err != nil {
		return nil, err
	}
	if response.Data.Progresses == nil {
		response.Data.Progresses = []CanvasGenerationProgress{}
	}
	return &response.Data, nil
}

func (c *Client) CancelCanvasGeneration(ctx context.Context, taskID string) (*CanvasGenerationCancelResult, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, errors.New("taskId 不能为空")
	}
	var response struct {
		Code    string                       `json:"code"`
		Message string                       `json:"message"`
		Data    CanvasGenerationCancelResult `json:"data"`
	}
	if err := c.doJSON(ctx, http.MethodPost, CanvasGenerationCancelPath, map[string]string{"taskId": taskID}, true, &response); err != nil {
		return nil, err
	}
	if err := validateEnvelope(response.Code, response.Message); err != nil {
		return nil, err
	}
	return &response.Data, nil
}

func IsCanvasVersionConflict(err error) bool {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	if apiErr.HTTPStatus == http.StatusConflict {
		return true
	}
	text := strings.ToLower(apiErr.Code + " " + apiErr.Message)
	hasVersion := strings.Contains(text, "version") || strings.Contains(text, "版本")
	hasConflict := strings.Contains(text, "conflict") || strings.Contains(text, "stale") || strings.Contains(text, "outdated") || strings.Contains(text, "冲突") || strings.Contains(text, "过期")
	return hasVersion && hasConflict
}
