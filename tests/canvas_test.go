package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/maoshuhua/pavo-cli/cmd"
	"github.com/maoshuhua/pavo-cli/internal/api"
	"github.com/maoshuhua/pavo-cli/internal/config"
)

func TestAPICanvasDetailAcceptsStringAndNumericIdentifiers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != api.CanvasProjectDetailPath || request.URL.Query().Get("uuid") != "project-1" || request.URL.Query().Get("canvas_uuid") != "canvas-1" {
			t.Fatalf("request URL = %s", request.URL.String())
		}
		_, _ = writer.Write([]byte(`{"code":"000000","message":"success","data":{"project_meta":{"id":338562408542949376,"project_uuid":"project-1"},"current_canvas":{"canvas_uuid":"canvas-1"},"node_list":[{"id":338562408542949377,"node_key":"i-one","node_type":"image","position":{"position_x":"100","position_y":80},"measured":{"width":"280","height":280},"data":{"node_key":"i-one","future":{"value":1}}}],"connection_list":[],"version":"7"}}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL, func() (string, error) { return "token", nil })
	detail, err := client.GetCanvasProjectDetail(context.Background(), "project-1", "canvas-1")
	if err != nil {
		t.Fatal(err)
	}
	if detail.ProjectMeta.ID != "338562408542949376" || detail.NodeList[0].ID != "338562408542949377" || detail.NodeList[0].Position.PositionY != "80" || detail.Version != 7 {
		t.Fatalf("detail = %#v", detail)
	}
}

func TestAPIUploadCanvasFileSelectsUGCPurpose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reference.jpg")
	if err := os.WriteFile(path, []byte("image bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case config.PresignedURLPath:
			var body api.PresignedURLRequest
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.Purpose != "ugc_image" {
				t.Fatalf("purpose = %q", body.Purpose)
			}
			_, _ = writer.Write([]byte(`{"code":"000000","message":"success","data":{"upload_url":"` + server.URL + `/object","public_url":"https://cdn.example.test/reference.jpg","method":"PUT","required_headers":{"Content-Type":"image/jpeg"}}}`))
		case "/object":
			_, _ = io.Copy(io.Discard, request.Body)
			writer.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("path = %q", request.URL.Path)
		}
	}))
	defer server.Close()

	client := newTestClient(server.URL, func() (string, error) { return "token", nil })
	result, err := client.UploadCanvasFile(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if result.PublicURL != "https://cdn.example.test/reference.jpg" {
		t.Fatalf("result = %#v", result)
	}
}

func TestCLICanvasNodeCreateUsesFreshVersionAndFullData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if got := request.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("Authorization = %q", got)
		}
		switch request.URL.Path {
		case api.CanvasModelOptionsPath:
			if request.URL.Query().Get("scene_code") != "canvas_image" {
				t.Fatalf("scene_code = %q", request.URL.Query().Get("scene_code"))
			}
			_, _ = writer.Write([]byte(`{"code":"000000","message":"success","data":{"items":[{"model_code":"model-x","allowed":true,"is_online":true,"constraints":{"aspect_ratios":["1:1"],"resolutions":["sd"],"mode_types":["text_to_image","image_to_image"],"max_batch_images":2}}]}}`))
		case api.CanvasProjectDetailPath:
			_, _ = writer.Write([]byte(`{"code":"000000","message":"success","data":{"current_canvas":{"canvas_uuid":"canvas-1"},"node_list":[],"connection_list":[],"version":7}}`))
		case "/api/v1/pixa/canvas/project/project-1/nodes/batch":
			var body api.CanvasBatchRequest
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.Version != 7 || body.CanvasUUID != "canvas-1" || body.SessionID == "" || !strings.HasPrefix(body.RequestID, "req-") || len(body.Nodes.Create) != 1 {
				t.Fatalf("batch body = %#v", body)
			}
			item := body.Nodes.Create[0]
			if item.Type != "image" || item.Name != "主视觉" || !strings.HasPrefix(item.NodeKey, "i-") {
				t.Fatalf("create item = %#v", item)
			}
			var data map[string]any
			if err := json.Unmarshal([]byte(item.Data), &data); err != nil {
				t.Fatal(err)
			}
			params, _ := data["params"].(map[string]any)
			settings, _ := params["settings"].(map[string]any)
			if data["future"] != "kept" || data["node_key"] != item.NodeKey || params["model"] != "model-x" || params["count"] != float64(1) || settings["ratio"] != "1:1" || settings["resolution"] != "sd" {
				t.Fatalf("node data = %#v", data)
			}
			_, _ = writer.Write([]byte(`{"code":"000000","message":"success","data":{"version":8}}`))
		default:
			t.Fatalf("path = %q", request.URL.Path)
		}
	}))
	defer server.Close()

	stdout, stderr := executeCanvasCLI(t, server.URL, []string{"canvas", "node", "create", "--project", "project-1", "--canvas", "canvas-1", "--type", "image", "--name", "主视觉", "--prompt", "一只猫", "--model", "model-x", "--data", `{"future":"kept"}`})
	if stderr != "" {
		t.Fatalf("stderr = %q", stderr)
	}
	var result struct {
		NodeKey string `json:"node_key"`
		Version int64  `json:"version"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("stdout = %q: %v", stdout, err)
	}
	if !strings.HasPrefix(result.NodeKey, "i-") || result.Version != 8 {
		t.Fatalf("result = %#v", result)
	}
}

func TestCLICanvasEdgeAddSynchronizesConnectionData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case api.CanvasProjectDetailPath:
			_, _ = writer.Write([]byte(`{"code":"000000","message":"success","data":{"current_canvas":{"canvas_uuid":"canvas-1"},"node_list":[{"id":"1","node_key":"i-source","node_type":"upload","name":"参考图","position":{"position_x":"100","position_y":"80"},"data":{"node_key":"i-source","title":"参考图","mediaType":"image","url":["https://cdn.example.test/reference.png"],"isExecutable":false}},{"id":"2","node_key":"i-target","node_type":"image","name":"主视觉","position":{"position_x":"450","position_y":"80"},"data":{"node_key":"i-target","title":"主视觉","isExecutable":true}}],"connection_list":[],"version":3}}`))
		case "/api/v1/pixa/canvas/project/project-1/nodes/batch":
			var body api.CanvasBatchRequest
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if len(body.Connections.Create) != 1 || len(body.Nodes.Update) != 2 {
				t.Fatalf("body = %#v", body)
			}
			connection := body.Connections.Create[0]
			if connection.Source != "i-source" || connection.Target != "i-target" || !strings.HasPrefix(connection.ConnectionID, "e-i-i-") {
				t.Fatalf("connection = %#v", connection)
			}
			dataByKey := map[string]map[string]any{}
			for _, item := range body.Nodes.Update {
				var data map[string]any
				if err := json.Unmarshal([]byte(item.Data), &data); err != nil {
					t.Fatal(err)
				}
				dataByKey[item.NodeKey] = data
			}
			if dataByKey["i-source"]["target"].([]any)[0] != "i-target" || dataByKey["i-target"]["source"].([]any)[0] != "i-source" {
				t.Fatalf("dataByKey = %#v", dataByKey)
			}
			_, _ = writer.Write([]byte(`{"code":"000000","message":"success","data":{"version":4}}`))
		default:
			t.Fatalf("path = %q", request.URL.Path)
		}
	}))
	defer server.Close()

	stdout, stderr := executeCanvasCLI(t, server.URL, []string{"canvas", "edge", "add", "--source", "参考图", "--target", "主视觉", "--project", "project-1", "--canvas", "canvas-1"})
	if stderr != "" || !strings.Contains(stdout, `"operation":"edge.add"`) || !strings.Contains(stdout, `"version":4`) {
		t.Fatalf("stdout=%q stderr=%q", stdout, stderr)
	}
}

func TestCLICanvasRunWaitsAndReturnsDecodedTaskResult(t *testing.T) {
	detailVersion := int64(7)
	batchCalls := 0
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/result.png":
			if got := request.Header.Get("Authorization"); got != "" {
				t.Fatalf("download Authorization = %q", got)
			}
			writer.Header().Set("Content-Type", "image/png")
			_, _ = writer.Write([]byte("generated canvas image"))
		case "/missing.png":
			writer.WriteHeader(http.StatusNotFound)
		case api.CanvasProjectDetailPath:
			_, _ = writer.Write([]byte(fmt.Sprintf(`{"code":"000000","message":"success","data":{"project_meta":{"id":"338562408542949376","project_uuid":"project-1"},"current_canvas":{"canvas_uuid":"canvas-1"},"node_list":[{"id":"1","node_key":"i-one","node_type":"image","name":"图片节点1","position":{"position_x":"100","position_y":"80"},"data":{"node_key":"i-one","title":"图片节点1","isExecutable":true,"task_id":"-1"}}],"connection_list":[],"version":%d}}`, detailVersion)))
		case "/api/v1/pixa/canvas/project/project-1/generation/create":
			var body api.CreateCanvasGenerationRequest
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.NodeKey != "i-one" || body.CanvasUUID != "canvas-1" || !strings.HasPrefix(body.RequestID, "req-") {
				t.Fatalf("create body = %#v", body)
			}
			detailVersion = 8
			_, _ = writer.Write([]byte(`{"code":"000000","message":"success","data":{"task_id":"task-1","version":8}}`))
		case "/api/v1/pixa/canvas/project/project-1/nodes/batch":
			var body api.CanvasBatchRequest
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.Version != detailVersion || len(body.Nodes.Update) != 1 {
				t.Fatalf("sync batch = %#v, detailVersion=%d", body, detailVersion)
			}
			var data map[string]any
			if err := json.Unmarshal([]byte(body.Nodes.Update[0].Data), &data); err != nil {
				t.Fatal(err)
			}
			batchCalls++
			if batchCalls == 1 && (data["task_id"] != "task-1" || data["status"] != "generating") {
				t.Fatalf("start data = %#v", data)
			}
			if batchCalls == 2 {
				urls, _ := data["url"].([]any)
				if data["task_id"] != "-1" || len(urls) != 2 || urls[0] != server.URL+"/result.png" || urls[1] != server.URL+"/missing.png" {
					t.Fatalf("terminal data = %#v", data)
				}
			}
			detailVersion++
			_, _ = writer.Write([]byte(fmt.Sprintf(`{"code":"000000","message":"success","data":{"version":%d}}`, detailVersion)))
		case api.CanvasGenerationProgressPath:
			taskResult, err := json.Marshal(fmt.Sprintf(`{"results":[{"url":%q,"mimetype":"image/png"},{"url":%q,"mimetype":"image/png"}]}`, server.URL+"/result.png", server.URL+"/missing.png"))
			if err != nil {
				t.Fatal(err)
			}
			_, _ = fmt.Fprintf(writer, `{"code":"000000","message":"success","data":{"progresses":[{"taskId":"task-1","status":2,"statusText":"succeeded","progressPercent":100,"version":9,"taskResult":%s}]}}`, taskResult)
		default:
			t.Fatalf("path = %q", request.URL.Path)
		}
	}))
	defer server.Close()

	downloadDir := t.TempDir()
	stdout, stderr := executeCanvasCLI(t, server.URL, []string{"canvas", "run", "图片节点1", "--project", "project-1", "--canvas", "canvas-1", "--interval", "1ms", "--timeout", "1s", "--download", "--output-dir", downloadDir})
	if !strings.Contains(stderr, "status=2 progress=100%") {
		t.Fatalf("stderr = %q", stderr)
	}
	var result struct {
		ProjectID  string `json:"project_id"`
		CanvasUUID string `json:"canvas_uuid"`
		CanvasURL  string `json:"canvas_url"`
		Task       struct {
			TaskID     string `json:"task_id"`
			Terminal   bool   `json:"terminal"`
			TaskResult struct {
				Results []struct {
					URL           string `json:"url"`
					LocalPath     string `json:"local_path"`
					DownloadError string `json:"download_error"`
				} `json:"results"`
			} `json:"task_result"`
		} `json:"task"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("stdout = %q: %v", stdout, err)
	}
	wantCanvasURL := "https://app-test.pavo-ai.cn/canvas/338562408542949376?canvas_uuid=canvas-1&project_uuid=project-1"
	if result.ProjectID != "338562408542949376" || result.CanvasUUID != "canvas-1" || result.CanvasURL != wantCanvasURL || result.Task.TaskID != "task-1" || !result.Task.Terminal || len(result.Task.TaskResult.Results) != 2 {
		t.Fatalf("result = %#v", result)
	}
	first := result.Task.TaskResult.Results[0]
	if first.URL != server.URL+"/result.png" || first.LocalPath != filepath.Join(downloadDir, "01-图片节点1.png") || first.DownloadError != "" {
		t.Fatalf("first result = %#v", first)
	}
	content, err := os.ReadFile(first.LocalPath)
	if err != nil || string(content) != "generated canvas image" {
		t.Fatalf("downloaded content=%q err=%v", content, err)
	}
	second := result.Task.TaskResult.Results[1]
	if second.URL != server.URL+"/missing.png" || second.LocalPath != "" || !strings.Contains(second.DownloadError, "HTTP 404") {
		t.Fatalf("second result = %#v", second)
	}
	if batchCalls != 2 {
		t.Fatalf("batch calls = %d", batchCalls)
	}
}

func TestCLICanvasRunRejectsDownloadWithoutWaiting(t *testing.T) {
	t.Setenv(config.EnvAccessToken, "test-token")
	t.Setenv(config.EnvConfigFile, filepath.Join(t.TempDir(), "config.json"))
	var stdout, stderr bytes.Buffer
	root, err := cmd.NewRootCommand(&stdout, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	root.SetArgs([]string{"canvas", "run", "i-one", "--project", "project-1", "--wait=false", "--download"})
	err = root.Execute()
	if err == nil || !strings.Contains(err.Error(), "不能与 --wait=false 同时使用") {
		t.Fatalf("error = %v", err)
	}
}

func TestCLICanvasStatusReturnsFrontendURL(t *testing.T) {
	t.Setenv(config.EnvAppBaseURL, "https://app-test.pavo-ai.cn")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != api.CanvasProjectDetailPath {
			t.Fatalf("path = %q", request.URL.Path)
		}
		_, _ = writer.Write([]byte(`{"code":"000000","message":"success","data":{"project_meta":{"id":"338562408542949376","project_uuid":"project-1"},"current_canvas":{"canvas_uuid":"canvas-1"},"node_list":[],"connection_list":[],"version":1}}`))
	}))
	defer server.Close()

	stdout, stderr := executeCanvasCLI(t, server.URL, []string{"canvas", "status", "--project", "project-1", "--canvas", "canvas-1"})
	if stderr != "" {
		t.Fatalf("stderr = %q", stderr)
	}
	var result struct {
		ProjectID  string `json:"project_id"`
		CanvasUUID string `json:"canvas_uuid"`
		CanvasURL  string `json:"canvas_url"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("stdout = %q: %v", stdout, err)
	}
	want := "https://app-test.pavo-ai.cn/canvas/338562408542949376?canvas_uuid=canvas-1&project_uuid=project-1"
	if result.ProjectID != "338562408542949376" || result.CanvasUUID != "canvas-1" || result.CanvasURL != want {
		t.Fatalf("result = %#v", result)
	}
}

func TestCLICanvasStoryboardOfflineQualityCommands(t *testing.T) {
	directory := t.TempDir()
	templatePath := filepath.Join(directory, "storyboard.json")
	stdout, stderr := executeCanvasCLI(t, "http://unused.example.test", []string{"canvas", "storyboard", "template", "--profile", "commercial", "--shots", "3", "--output", templatePath})
	if stderr != "" || !strings.Contains(stdout, `"operation":"storyboard.template"`) {
		t.Fatalf("template stdout=%q stderr=%q", stdout, stderr)
	}
	if _, err := os.Stat(templatePath); err != nil {
		t.Fatal(err)
	}
	stdout, stderr = executeCanvasCLI(t, "http://unused.example.test", []string{"canvas", "storyboard", "lint", templatePath})
	if stderr != "" {
		t.Fatalf("lint stderr=%q", stderr)
	}
	var lint struct {
		Valid        bool `json:"valid"`
		QualityReady bool `json:"quality_ready"`
		ShotCount    int  `json:"shot_count"`
		Warnings     int  `json:"warnings"`
	}
	if err := json.Unmarshal([]byte(stdout), &lint); err != nil {
		t.Fatalf("lint stdout=%q: %v", stdout, err)
	}
	if !lint.Valid || lint.QualityReady || lint.ShotCount != 3 || lint.Warnings == 0 {
		t.Fatalf("lint=%#v", lint)
	}
	stdout, stderr = executeCanvasCLI(t, "http://unused.example.test", []string{"canvas", "storyboard", "compile", templatePath, "--kind", "image"})
	if stderr != "" || !strings.Contains(stdout, `"kind":"image"`) || !strings.Contains(stdout, `"image_prompt":"`) || strings.Contains(stdout, `"video_prompt":"`) {
		t.Fatalf("compile stdout=%q stderr=%q", stdout, stderr)
	}
}

func TestCLICanvasModelShowExplainsLiveDefaults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != api.CanvasModelOptionsPath || request.URL.Query().Get("scene_code") != "canvas_video" {
			t.Fatalf("request=%s", request.URL.String())
		}
		_, _ = writer.Write([]byte(`{"code":"000000","message":"success","data":{"items":[{"model_code":"video-x","allowed":true,"is_online":true,"constraints":{"aspect_ratios":["16:9"],"resolutions":["hd"],"mode_types":["image_to_video"],"supported_duration_seconds":[4,8]}}]}}`))
	}))
	defer server.Close()
	stdout, stderr := executeCanvasCLI(t, server.URL, []string{"canvas", "model", "show", "video-x", "--scene", "canvas_video"})
	if stderr != "" || !strings.Contains(stdout, `"available":true`) || !strings.Contains(stdout, `"duration":4`) || !strings.Contains(stdout, `"ratio":"16:9"`) {
		t.Fatalf("stdout=%q stderr=%q", stdout, stderr)
	}
}

func TestCLICanvasApplyReadsNDJSONFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "workflow.ndjson")
	if err := os.WriteFile(file, []byte("{\"op\":\"node.create\",\"as\":\"copy\",\"type\":\"text\",\"name\":\"文案\",\"prompt\":\"一句克制旁白\"}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case api.CanvasProjectDetailPath:
			_, _ = writer.Write([]byte(`{"code":"000000","message":"success","data":{"current_canvas":{"canvas_uuid":"canvas-1"},"node_list":[],"connection_list":[],"version":2}}`))
		case "/api/v1/pixa/canvas/project/project-1/nodes/batch":
			var body api.CanvasBatchRequest
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if len(body.Nodes.Create) != 1 || body.Nodes.Create[0].Type != "text" {
				t.Fatalf("body=%#v", body)
			}
			_, _ = writer.Write([]byte(`{"code":"000000","message":"success","data":{"version":3}}`))
		default:
			t.Fatalf("path=%q", request.URL.Path)
		}
	}))
	defer server.Close()
	stdout, stderr := executeCanvasCLI(t, server.URL, []string{"canvas", "apply", "--file", file, "--project", "project-1", "--canvas", "canvas-1"})
	if stderr != "" || !strings.Contains(stdout, `"node.create":1`) || !strings.Contains(stdout, `"version":3`) {
		t.Fatalf("stdout=%q stderr=%q", stdout, stderr)
	}
}

func executeCanvasCLI(t *testing.T, baseURL string, args []string) (string, string) {
	t.Helper()
	t.Setenv(config.EnvAPIBaseURL, baseURL)
	t.Setenv(config.EnvAccessToken, "test-token")
	t.Setenv(config.EnvConfigFile, filepath.Join(t.TempDir(), "config.json"))
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	root, err := cmd.NewRootCommand(&stdout, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, stderr=%q", err, stderr.String())
	}
	return stdout.String(), stderr.String()
}
