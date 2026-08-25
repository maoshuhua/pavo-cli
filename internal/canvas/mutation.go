package canvas

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/maoshuhua/pavo-cli/internal/api"
)

type MutationClient interface {
	GetCanvasProjectDetail(ctx context.Context, projectUUID, canvasUUID string) (*api.CanvasProjectDetail, error)
	BatchCanvasProjectNodes(ctx context.Context, projectUUID string, request api.CanvasBatchRequest) (*api.CanvasBatchResult, error)
}

type MutationBuilder func(detail *api.CanvasProjectDetail) (*api.CanvasBatchRequest, error)

// ApplyMutation rebuilds a safe canvas mutation from a fresh detail snapshot.
// It retries once only when the server explicitly reports a version conflict.
func ApplyMutation(ctx context.Context, client MutationClient, scope Scope, build MutationBuilder) (*api.CanvasBatchResult, error) {
	if strings.TrimSpace(scope.ProjectUUID) == "" {
		return nil, errors.New("project_uuid 不能为空")
	}
	for attempt := 0; attempt < 2; attempt++ {
		detail, err := client.GetCanvasProjectDetail(ctx, scope.ProjectUUID, scope.CanvasUUID)
		if err != nil {
			return nil, err
		}
		request, err := build(detail)
		if err != nil {
			return nil, err
		}
		if request == nil {
			return nil, errors.New("画布变更请求为空")
		}
		canvasUUID := strings.TrimSpace(scope.CanvasUUID)
		if canvasUUID == "" {
			canvasUUID = strings.TrimSpace(detail.CurrentCanvas.CanvasUUID)
		}
		if canvasUUID == "" {
			return nil, errors.New("画布详情缺少 current_canvas.canvas_uuid")
		}
		request.CanvasUUID = canvasUUID
		request.Version = int64(detail.Version)
		request.SessionID = scope.SessionID
		request.Timestamp = time.Now().UnixMilli()
		request.RequestID, err = RequestID()
		if err != nil {
			return nil, err
		}
		normalizeBatchRequest(request)
		result, err := client.BatchCanvasProjectNodes(ctx, scope.ProjectUUID, *request)
		if err == nil {
			return result, nil
		}
		if attempt == 0 && api.IsCanvasVersionConflict(err) {
			continue
		}
		return nil, err
	}
	return nil, fmt.Errorf("画布版本持续冲突，请重新读取后再试")
}

func NewBatchRequest() *api.CanvasBatchRequest {
	request := &api.CanvasBatchRequest{}
	normalizeBatchRequest(request)
	return request
}

func normalizeBatchRequest(request *api.CanvasBatchRequest) {
	if request.Nodes.Create == nil {
		request.Nodes.Create = []api.CanvasBatchNodeWriteItem{}
	}
	if request.Nodes.Update == nil {
		request.Nodes.Update = []api.CanvasBatchNodeWriteItem{}
	}
	if request.Nodes.Delete == nil {
		request.Nodes.Delete = []string{}
	}
	if request.Connections.Create == nil {
		request.Connections.Create = []api.CanvasBatchConnectionWriteItem{}
	}
	if request.Connections.Delete == nil {
		request.Connections.Delete = []api.CanvasBatchConnectionDeleteItem{}
	}
}
