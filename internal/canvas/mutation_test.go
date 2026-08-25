package canvas

import (
	"context"
	"testing"

	"github.com/maoshuhua/pavo-cli/internal/api"
)

type conflictOnceClient struct {
	detailCalls int
	batchCalls  int
	versions    []int64
}

func (client *conflictOnceClient) GetCanvasProjectDetail(context.Context, string, string) (*api.CanvasProjectDetail, error) {
	client.detailCalls++
	return &api.CanvasProjectDetail{CurrentCanvas: api.CanvasInfo{CanvasUUID: "canvas-1"}, Version: api.FlexibleInt64(6 + client.detailCalls)}, nil
}

func (client *conflictOnceClient) BatchCanvasProjectNodes(_ context.Context, _ string, request api.CanvasBatchRequest) (*api.CanvasBatchResult, error) {
	client.batchCalls++
	client.versions = append(client.versions, request.Version)
	if client.batchCalls == 1 {
		return nil, &api.APIError{HTTPStatus: 409, Message: "version conflict"}
	}
	return &api.CanvasBatchResult{Version: 9}, nil
}

func TestApplyMutationRebuildsOnceAfterExplicitConflict(t *testing.T) {
	client := &conflictOnceClient{}
	buildCalls := 0
	result, err := ApplyMutation(context.Background(), client, Scope{ProjectUUID: "project-1", SessionID: "session-1"}, func(_ *api.CanvasProjectDetail) (*api.CanvasBatchRequest, error) {
		buildCalls++
		request := NewBatchRequest()
		request.Nodes.Delete = append(request.Nodes.Delete, "i-one")
		return request, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Version != 9 || client.detailCalls != 2 || client.batchCalls != 2 || buildCalls != 2 {
		t.Fatalf("result=%#v detail=%d batch=%d build=%d", result, client.detailCalls, client.batchCalls, buildCalls)
	}
	if len(client.versions) != 2 || client.versions[0] != 7 || client.versions[1] != 8 {
		t.Fatalf("versions = %#v", client.versions)
	}
}
