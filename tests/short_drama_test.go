package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/maoshuhua/pavo-cli/cmd"
	"github.com/maoshuhua/pavo-cli/internal/api"
	"github.com/maoshuhua/pavo-cli/internal/config"
)

func TestCLIShortDramaStartThenReplyUsesOneConversation(t *testing.T) {
	const conversationID = "340407156788563968"
	var createCalls int
	var streamCalls int
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case config.ConversationPath:
			createCalls++
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"code":"000000","message":"success","data":{"conversation_id":340407156788563968}}`))
		case config.StreamPath:
			streamCalls++
			var body api.StreamRequest
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.ConversationID != conversationID || body.Mode != string(api.StreamModeShortDrama) {
				t.Fatalf("stream body = %#v", body)
			}
			if body.ExtraContext == nil || body.ExtraContext.AgentParams == nil ||
				body.ExtraContext.AgentParams.ImageModelCode != "agnes-image" ||
				body.ExtraContext.AgentParams.VideoModelCode != "agnes-video-new" {
				t.Fatalf("extra_context = %#v", body.ExtraContext)
			}
			if streamCalls == 1 && body.Prompt != "南京宣传片" {
				t.Fatalf("first prompt = %q", body.Prompt)
			}
			if streamCalls == 2 && body.Prompt != "改成水墨风格" {
				t.Fatalf("reply prompt = %q", body.Prompt)
			}
			writer.Header().Set("Content-Type", "application/x-ndjson")
			_, _ = writer.Write([]byte(`{"data":{},"seq":1,"type":"AgentEnd"}` + "\n"))
		default:
			t.Fatalf("unexpected path %q", request.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv(config.EnvAPIBaseURL, server.URL)
	t.Setenv(config.EnvAccessToken, "test-token")
	t.Setenv(config.EnvConfigFile, filepath.Join(t.TempDir(), "config.json"))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	root, err := cmd.NewRootCommand(&stdout, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	root.SetArgs([]string{"short-drama", "start", "--prompt", "南京宣传片"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	var started api.StreamOutput
	if err := json.Unmarshal(stdout.Bytes(), &started); err != nil {
		t.Fatalf("start stdout is not valid JSON: %v; stdout=%q", err, stdout.String())
	}
	if started.ConversationID != conversationID {
		t.Fatalf("start result = %#v", started)
	}

	stdout.Reset()
	stderr.Reset()
	root, err = cmd.NewRootCommand(&stdout, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	root.SetArgs([]string{"short-drama", "reply", "--conversation-id", conversationID, "--prompt", "改成水墨风格"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if createCalls != 1 || streamCalls != 2 {
		t.Fatalf("createCalls=%d streamCalls=%d", createCalls, streamCalls)
	}
}

func TestCLIShortDramaResultDownloadsEveryPersistedImageAndVideo(t *testing.T) {
	const conversationID = "340407156788563968"
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case config.ConversationHistoryPath:
			if request.URL.Query().Get("conversation_id") != conversationID {
				t.Fatalf("conversation_id = %q", request.URL.Query().Get("conversation_id"))
			}
			_, _ = writer.Write([]byte(`{"code":"000000","message":"success","data":{"is_running":false,"turns":[{"assistant":[{"type":"GenerationArtifact","data":{"results":[{"mimetype":"image/jpeg","url":"` + serverURL + `/scene.jpg"}]}},{"type":"GenerationArtifact","data":{"results":[{"mimetype":"video/mp4","url":"` + serverURL + `/shot.mp4"}]}}]}]}}`))
		case "/scene.jpg":
			_, _ = writer.Write([]byte("scene image"))
		case "/shot.mp4":
			_, _ = writer.Write([]byte("shot video"))
		default:
			t.Fatalf("unexpected path %q", request.URL.Path)
		}
	}))
	defer server.Close()
	serverURL = server.URL

	t.Setenv(config.EnvAPIBaseURL, server.URL)
	t.Setenv(config.EnvAccessToken, "test-token")
	t.Setenv(config.EnvConfigFile, filepath.Join(t.TempDir(), "config.json"))
	downloadDir := filepath.Join(t.TempDir(), "durable-assets")
	var stdout bytes.Buffer
	root, err := cmd.NewRootCommand(&stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	root.SetArgs([]string{"short-drama", "result", "--conversation-id", conversationID, "--download-dir", downloadDir})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Results []api.GenerationResult `json:"results"`
		Assets  []api.GeneratedAsset   `json:"assets"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("stdout is not valid JSON: %v; stdout=%q", err, stdout.String())
	}
	if len(result.Results) != 2 || len(result.Assets) != 2 {
		t.Fatalf("result = %#v", result)
	}
	for _, asset := range result.Assets {
		if _, err := os.Stat(asset.Result.LocalPath); err != nil {
			t.Fatalf("asset %q was not downloaded: %v", asset.Result.LocalPath, err)
		}
	}
}

func TestCLIShortDramaLiveAssetsDownloadsAndEmitsEachArtifactBeforeTerminal(t *testing.T) {
	const conversationID = "340407156788563968"
	var serverURL string
	var phase atomic.Int32
	var imageDownloads atomic.Int32
	var videoDownloads atomic.Int32
	imageDownloaded := make(chan struct{}, 1)
	videoDownloaded := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case config.ConversationPath:
			_, _ = writer.Write([]byte(`{"code":"000000","message":"success","data":{"conversation_id":340407156788563968}}`))
		case config.StreamPath:
			writer.Header().Set("Content-Type", "application/x-ndjson")
			flusher, ok := writer.(http.Flusher)
			if !ok {
				t.Fatal("stream response does not support flushing")
			}
			phase.Store(1)
			_, _ = writer.Write([]byte(`{"data":{"title":"场景图","kind":"image","results":[{"mimetype":"image/jpeg","url":"` + serverURL + `/scene.jpg"}]},"seq":1,"type":"GenerationArtifact"}` + "\n"))
			flusher.Flush()
			select {
			case <-imageDownloaded:
			case <-time.After(2 * time.Second):
				t.Error("first image was not downloaded while the stream was still open")
				return
			}
			phase.Store(2)
			_, _ = writer.Write([]byte(`{"data":{"title":"分镜视频","kind":"video","results":[{"mimetype":"video/mp4","url":"` + serverURL + `/shot.mp4"}]},"seq":2,"type":"GenerationArtifact"}` + "\n"))
			flusher.Flush()
			select {
			case <-videoDownloaded:
			case <-time.After(2 * time.Second):
				t.Error("video was not downloaded while the stream was still open")
				return
			}
			phase.Store(3)
			_, _ = writer.Write([]byte(`{"data":{},"seq":3,"type":"AgentEnd"}` + "\n"))
		case "/scene.jpg":
			if phase.Load() != 1 {
				t.Errorf("image was downloaded after phase %d", phase.Load())
			}
			imageDownloads.Add(1)
			_, _ = writer.Write([]byte("scene image"))
			imageDownloaded <- struct{}{}
		case "/shot.mp4":
			if phase.Load() != 2 {
				t.Errorf("video was downloaded after phase %d", phase.Load())
			}
			videoDownloads.Add(1)
			_, _ = writer.Write([]byte("shot video"))
			videoDownloaded <- struct{}{}
		default:
			t.Fatalf("unexpected path %q", request.URL.Path)
		}
	}))
	defer server.Close()
	serverURL = server.URL

	t.Setenv(config.EnvAPIBaseURL, server.URL)
	t.Setenv(config.EnvAccessToken, "test-token")
	t.Setenv(config.EnvConfigFile, filepath.Join(t.TempDir(), "config.json"))
	downloadDir := filepath.Join(t.TempDir(), "live-assets")
	var stdout bytes.Buffer
	root, err := cmd.NewRootCommand(&stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	root.SetArgs([]string{"short-drama", "start", "--prompt", "南京宣传片", "--live-assets", "--download-dir", downloadDir})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected two asset events and one complete event, stdout=%q", stdout.String())
	}
	var imageEvent, videoEvent struct {
		Type  string             `json:"type"`
		Asset api.GeneratedAsset `json:"asset"`
	}
	if err := json.Unmarshal([]byte(lines[0]), &imageEvent); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(lines[1]), &videoEvent); err != nil {
		t.Fatal(err)
	}
	var complete struct {
		Type   string            `json:"type"`
		Result *api.StreamOutput `json:"result"`
	}
	if err := json.Unmarshal([]byte(lines[2]), &complete); err != nil {
		t.Fatal(err)
	}
	if imageEvent.Type != "asset_ready" || videoEvent.Type != "asset_ready" || complete.Type != "complete" || complete.Result == nil {
		t.Fatalf("live output = %#v %#v %#v", imageEvent, videoEvent, complete)
	}
	if imageEvent.Asset.Result.LocalPath == "" || videoEvent.Asset.Result.LocalPath == "" ||
		len(complete.Result.Assets) != 2 || complete.Result.Assets[0].Result.LocalPath == "" || complete.Result.Assets[1].Result.LocalPath == "" {
		t.Fatalf("live output is missing local paths: %#v %#v %#v", imageEvent, videoEvent, complete.Result)
	}
	if imageDownloads.Load() != 1 || videoDownloads.Load() != 1 {
		t.Fatalf("imageDownloads=%d videoDownloads=%d", imageDownloads.Load(), videoDownloads.Load())
	}
}
