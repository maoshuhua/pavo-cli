package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/maoshuhua/pavo-cli/internal/api"
	canvascore "github.com/maoshuhua/pavo-cli/internal/canvas"
)

func TestAPICanvasArtifactsPreserveSnowflakeIDsAndSaveContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/v1/pixa/canvas/project/project-1/artifacts":
			if request.URL.Query().Get("canvas_uuid") != "canvas-1" || request.URL.Query().Get("category") != "videos" || request.URL.Query().Get("page") != "2" || request.URL.Query().Get("page_size") != "5" {
				t.Fatalf("query=%v", request.URL.Query())
			}
			_, _ = writer.Write([]byte(`{"code":"000000","message":"success","data":{"groups":[{"date":"2026-08-24","list":[{"id":1908100000000001001,"artifact_uuid":"artifact-1","node_key":"v-one","artifact_type":"video","url":"https://cdn.example.test/one.mp4"}]}],"pagination":{"page":2,"page_size":5,"total":9}}}`))
		case "/api/v1/pixa/canvas/project/project-1/media-assets/batch-create":
			var body api.CanvasMediaAssetsRequest
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.CanvasUUID != "canvas-1" || len(body.Items) != 1 || body.Items[0].NodeKey != "v-one" || body.Items[0].ResourceIndex != 1 {
				t.Fatalf("body=%#v", body)
			}
			_, _ = writer.Write([]byte(`{"code":"000000","message":"success","data":{"items":[{"visual_id":1908100000000001002,"node_key":"v-one","resource_index":1,"created":true}]}}`))
		default:
			t.Fatalf("path=%s", request.URL.Path)
		}
	}))
	defer server.Close()
	client := newTestClient(server.URL, func() (string, error) { return "token", nil })
	list, err := client.ListCanvasArtifacts(context.Background(), "project-1", "canvas-1", "video", 2, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Groups) != 1 || len(list.Groups[0].List) != 1 || string(list.Groups[0].List[0].ID) != "1908100000000001001" || list.Pagination.Total != 9 {
		t.Fatalf("list=%#v", list)
	}
	result, err := client.CreateCanvasMediaAssets(context.Background(), "project-1", api.CanvasMediaAssetsRequest{CanvasUUID: "canvas-1", Items: []api.CanvasMediaAssetItem{{NodeKey: "v-one", ResourceIndex: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(result) {
		t.Fatalf("result=%s", result)
	}
}

func TestCLICanvasDAGRunHonorsTopologyAndPersistsManifest(t *testing.T) {
	workspace := t.TempDir()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(workspace); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })
	nodeData := func(key, name string) json.RawMessage {
		data, _ := json.Marshal(map[string]any{"node_key": key, "title": name, "isExecutable": true, "params": map[string]any{"model": "model-x", "count": 1}})
		return data
	}
	detail := &api.CanvasProjectDetail{CurrentCanvas: api.CanvasInfo{CanvasUUID: "canvas-1"}, Version: 7, NodeList: []api.CanvasNode{
		{NodeKey: "i-a", NodeType: "image", Name: "A", Position: api.CanvasNodePosition{PositionX: "0", PositionY: "0"}, Measured: api.CanvasNodeMeasured{Width: "280", Height: "280"}, Data: nodeData("i-a", "A")},
		{NodeKey: "i-b", NodeType: "image", Name: "B", Position: api.CanvasNodePosition{PositionX: "350", PositionY: "0"}, Measured: api.CanvasNodeMeasured{Width: "280", Height: "280"}, Data: nodeData("i-b", "B")},
	}, ConnectionList: []api.CanvasConnection{{ConnectionID: "e-ab", SourceNodeKey: "i-a", TargetNodeKey: "i-b"}}}
	plan, err := canvascore.BuildDAGPlan(detail, "project-1", "canvas-1", canvascore.DAGScope{Mode: "all"})
	if err != nil {
		t.Fatal(err)
	}
	pavoDirectory := filepath.Join(workspace, ".pavo")
	if _, err := canvascore.SaveDAGPlan(pavoDirectory, plan); err != nil {
		t.Fatal(err)
	}
	var mutex sync.Mutex
	createOrder := []string{}
	aSucceeded := false
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case api.CanvasProjectDetailPath:
			_, _ = writer.Write([]byte(`{"code":"000000","message":"success","data":{"current_canvas":{"canvas_uuid":"canvas-1"},"node_list":[{"node_key":"i-a","node_type":"image","name":"A","position":{"position_x":"0","position_y":"0"},"measured":{"width":"280","height":"280"},"data":{"node_key":"i-a","title":"A","isExecutable":true,"params":{"model":"model-x","count":1}}},{"node_key":"i-b","node_type":"image","name":"B","position":{"position_x":"350","position_y":"0"},"measured":{"width":"280","height":"280"},"data":{"node_key":"i-b","title":"B","isExecutable":true,"params":{"model":"model-x","count":1}}}],"connection_list":[{"connection_id":"e-ab","source_node_key":"i-a","target_node_key":"i-b"}],"version":7}}`))
		case "/api/v1/pixa/canvas/project/project-1/generation/create":
			var body api.CreateCanvasGenerationRequest
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			mutex.Lock()
			if body.NodeKey == "i-b" && !aSucceeded {
				mutex.Unlock()
				t.Fatalf("B submitted before A succeeded")
			}
			createOrder = append(createOrder, body.NodeKey)
			mutex.Unlock()
			expectedOrder := 1
			if body.NodeKey == "i-b" {
				expectedOrder = 2
			}
			if body.BatchOrder != expectedOrder || body.BatchTotal != 2 || !strings.HasPrefix(body.ExecutionBatchID, "run-") {
				t.Fatalf("body=%#v", body)
			}
			_, _ = fmt.Fprintf(writer, `{"code":"000000","message":"success","data":{"task_id":"task-%s"}}`, body.NodeKey)
		case api.CanvasGenerationProgressPath:
			var body struct {
				TaskIDs []string `json:"taskIds"`
			}
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if len(body.TaskIDs) != 1 {
				t.Fatalf("progress body=%#v", body)
			}
			if body.TaskIDs[0] == "task-i-a" {
				mutex.Lock()
				aSucceeded = true
				mutex.Unlock()
			}
			_, _ = fmt.Fprintf(writer, `{"code":"000000","message":"success","data":{"progresses":[{"taskId":%q,"status":2,"statusText":"succeeded","progressPercent":100,"taskResult":{"results":[]}}]}}`, body.TaskIDs[0])
		default:
			t.Fatalf("path=%s", request.URL.Path)
		}
	}))
	defer server.Close()
	stdout, stderr := executeCanvasCLI(t, server.URL, []string{"canvas", "dag", "run", "--plan", plan.PlanID, "--max-parallel", "2", "--interval", "1ms", "--timeout", "2s"})
	if !strings.Contains(stderr, "task task-i-a") || !strings.Contains(stderr, "task task-i-b") {
		t.Fatalf("stderr=%q", stderr)
	}
	var run canvascore.DAGRun
	if err := json.Unmarshal([]byte(stdout), &run); err != nil {
		t.Fatalf("stdout=%q err=%v", stdout, err)
	}
	if run.Status != "succeeded" || len(run.Nodes) != 2 || run.Nodes[0].Status != "succeeded" || run.Nodes[1].Status != "succeeded" || strings.Join(createOrder, ",") != "i-a,i-b" {
		t.Fatalf("run=%#v order=%v", run, createOrder)
	}
	if _, err := os.Stat(filepath.Join(pavoDirectory, "canvas-runs", run.ExecutionBatchID+".json")); err != nil {
		t.Fatal(err)
	}
}
