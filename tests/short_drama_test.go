package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

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
				body.ExtraContext.AgentParams.VideoModelCode != "agnes-video" {
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
