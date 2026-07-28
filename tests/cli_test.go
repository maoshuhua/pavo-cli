package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/maoshuhua/pavo-cli/cmd"
	"github.com/maoshuhua/pavo-cli/internal/api"
	"github.com/maoshuhua/pavo-cli/internal/auth"
	"github.com/maoshuhua/pavo-cli/internal/config"
)

func TestCLIBusinessCommandsAreLimitedToProvidedCapabilities(t *testing.T) {
	t.Setenv(config.EnvConfigFile, filepath.Join(t.TempDir(), "config.json"))
	root, err := cmd.NewRootCommand(&bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"login", "conversation", "stream", "short-drama", "resume", "upload", "download-result", "update"} {
		command, _, findErr := root.Find([]string{name})
		if findErr != nil || command.Name() != name {
			t.Fatalf("missing command %q: command=%v err=%v", name, command, findErr)
		}
	}
	for _, removed := range []string{"generate-image", "generate-video", "get-thread"} {
		command, _, findErr := root.Find([]string{removed})
		if findErr == nil && command.Name() == removed {
			t.Fatalf("unexpected legacy command %q", removed)
		}
	}
}

func TestCLILoginStoresTokenWithoutPrintingIt(t *testing.T) {
	const token = "header.eyJleHAiOjE3ODcxOTY4NzJ9.signature"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if got := request.Header.Get("Authorization"); got != "" {
			t.Fatalf("login Authorization = %q", got)
		}
		var body api.LoginRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.EmailPassword.Password != "secret" {
			t.Fatal("login password was not sent")
		}
		_, _ = writer.Write([]byte(`{
			"code":"000000",
			"message":"success",
			"data":{
				"access_token":"` + token + `",
				"user_info":{"id":"user-1","email":"user@example.com","is_active":true}
			}
		}`))
	}))
	defer server.Close()

	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv(config.EnvAPIBaseURL, server.URL)
	t.Setenv(config.EnvConfigFile, configPath)
	t.Setenv(config.EnvAccessToken, "")
	t.Setenv(config.EnvPassword, "")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	root, err := cmd.NewRootCommand(&stdout, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	root.SetArgs([]string{"login", "--email", "user@example.com", "--password", "secret"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}

	session, err := auth.NewFileStore(configPath).Load()
	if err != nil {
		t.Fatal(err)
	}
	if session.AccessToken != token || session.User.ID != "user-1" {
		t.Fatalf("session = %#v", session)
	}
	if strings.Contains(stdout.String(), token) || strings.Contains(stdout.String(), "secret") {
		t.Fatalf("sensitive login value leaked to stdout: %s", stdout.String())
	}
}

func TestCLIConversationCreateAcceptsNumericIDAndPrintsString(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(`{"code":"000000","message":"success","data":{"conversation_id":338562408542949376}}`))
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
	root.SetArgs([]string{"conversation", "create", "--prompt", "生成图片"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}

	var result struct {
		ConversationID string `json:"conversation_id"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("stdout is not valid JSON: %v; stdout=%q", err, stdout.String())
	}
	if result.ConversationID != "338562408542949376" {
		t.Fatalf("conversation id = %q", result.ConversationID)
	}
}

func TestCLIStreamDisplaysRawEventsAndReturnsAgentEnd(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = writer.Write([]byte(
			`{"data":{"message":"hello"},"seq":1,"type":"MessageDelta"}` + "\n" +
				`{"data":{},"seq":2,"type":"AgentEnd"}` + "\n",
		))
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
	root.SetArgs([]string{"stream", "--conversation-id", "338575800850784256", "--prompt", "生成图片"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(stderr.String(), `"type":"MessageDelta"`) ||
		!strings.Contains(stderr.String(), `"message":"hello"`) ||
		!strings.Contains(stderr.String(), `"type":"AgentEnd"`) {
		t.Fatalf("stderr does not contain raw events: %s", stderr.String())
	}
	var result api.StreamOutput
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("stdout is not valid JSON: %v; stdout=%q", err, stdout.String())
	}
	if result.TerminalType != "AgentEnd" {
		t.Fatalf("terminal type = %q", result.TerminalType)
	}
}

func TestCLIStreamDownloadsSuccessfulResultsToRequestedDirectory(t *testing.T) {
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case config.StreamPath:
			writer.Header().Set("Content-Type", "application/x-ndjson")
			_, _ = writer.Write([]byte(`{"data":{"results":[{"success":true,"url":"` + serverURL + `/generated.jpg","mimetype":"image/jpeg"}]},"seq":1,"type":"GenerationSuccess"}` + "\n"))
		case "/generated.jpg":
			if request.Method != http.MethodGet {
				t.Fatalf("download method = %s", request.Method)
			}
			_, _ = writer.Write([]byte("generated image"))
		default:
			t.Fatalf("path = %q", request.URL.Path)
		}
	}))
	defer server.Close()
	serverURL = server.URL

	t.Setenv(config.EnvAPIBaseURL, server.URL)
	t.Setenv(config.EnvAccessToken, "test-token")
	t.Setenv(config.EnvConfigFile, filepath.Join(t.TempDir(), "config.json"))
	downloadDir := filepath.Join(t.TempDir(), "generated")
	var stdout bytes.Buffer
	root, err := cmd.NewRootCommand(&stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	root.SetArgs([]string{"stream", "--conversation-id", "conversation-1", "--prompt", "生成图片", "--download-dir", downloadDir})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}

	var result api.StreamOutput
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("stdout is not valid JSON: %v; stdout=%q", err, stdout.String())
	}
	if len(result.Results) != 1 || result.Results[0].LocalPath == "" {
		t.Fatalf("result = %#v", result)
	}
	if !filepath.IsAbs(result.Results[0].LocalPath) {
		t.Fatalf("local_path is not absolute: %q", result.Results[0].LocalPath)
	}
	content, err := os.ReadFile(result.Results[0].LocalPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "generated image" {
		t.Fatalf("file = %q", content)
	}
}

func TestCLIStreamDownloadsEveryImageAndVideoArtifact(t *testing.T) {
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case config.StreamPath:
			writer.Header().Set("Content-Type", "application/x-ndjson")
			_, _ = writer.Write([]byte(
				`{"data":{"title":"场景/秦淮河·夜色","kind":"image","extra":{"short_drama_item_id":"qinhuai_river_night","short_drama_parallel_group":"scene_image"},"results":[{"url":"` + serverURL + `/qinhuai.jpg","mimetype":"image/jpeg"}]},"seq":1,"type":"GenerationArtifact"}` + "\n" +
					`{"data":{"title":"分镜视频/镜头 1","kind":"video","extra":{"short_drama_item_id":"shot-1","short_drama_parallel_group":"shot_video"},"results":[{"url":"` + serverURL + `/shot-1.mp4","mimetype":"video/mp4"}]},"seq":2,"type":"GenerationArtifact"}` + "\n" +
					`{"data":{},"seq":3,"type":"AgentEnd"}` + "\n",
			))
		case "/qinhuai.jpg":
			_, _ = writer.Write([]byte("qinhuai image"))
		case "/shot-1.mp4":
			_, _ = writer.Write([]byte("shot video"))
		default:
			t.Fatalf("path = %q", request.URL.Path)
		}
	}))
	defer server.Close()
	serverURL = server.URL

	t.Setenv(config.EnvAPIBaseURL, server.URL)
	t.Setenv(config.EnvAccessToken, "test-token")
	t.Setenv(config.EnvConfigFile, filepath.Join(t.TempDir(), "config.json"))
	downloadDir := filepath.Join(t.TempDir(), "short-drama-assets")
	var stdout bytes.Buffer
	root, err := cmd.NewRootCommand(&stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	root.SetArgs([]string{"stream", "--conversation-id", "conversation-1", "--prompt", "生成短剧素材", "--download-dir", downloadDir})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}

	var result api.StreamOutput
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("stdout is not valid JSON: %v; stdout=%q", err, stdout.String())
	}
	if len(result.Assets) != 2 || len(result.Results) != 2 {
		t.Fatalf("result = %#v", result)
	}
	for _, asset := range result.Assets {
		if !filepath.IsAbs(asset.Result.LocalPath) {
			t.Fatalf("asset local_path = %q", asset.Result.LocalPath)
		}
		if _, err := os.Stat(asset.Result.LocalPath); err != nil {
			t.Fatalf("asset %q was not downloaded: %v", asset.Result.LocalPath, err)
		}
	}
	if result.Results[0].LocalPath == "" || result.Results[1].LocalPath == "" ||
		!strings.Contains(filepath.Base(result.Assets[0].Result.LocalPath), "scene-image") ||
		!strings.Contains(filepath.Base(result.Assets[1].Result.LocalPath), "shot-video") {
		t.Fatalf("downloaded result = %#v", result)
	}
}

func TestCLIStreamResumesAnAlreadyActiveConversation(t *testing.T) {
	var streamCalls int
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case config.StreamPath:
			streamCalls++
			writer.WriteHeader(http.StatusConflict)
			_, _ = writer.Write([]byte(`{"code":"070301","message":"conversation 338575800850784256 has active stream: request-1"}`))
		case config.ResumeStreamPath:
			var body api.ResumeStreamRequest
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.ConversationID != "338575800850784256" || body.FromSeq != 0 {
				t.Fatalf("resume body = %#v", body)
			}
			writer.Header().Set("Content-Type", "application/x-ndjson")
			_, _ = writer.Write([]byte(`{"data":{"results":[{"url":"https://example.test/result.jpg","mimetype":"image/jpeg"}]},"seq":1,"type":"GenerationSuccess"}` + "\n"))
		default:
			t.Fatalf("path = %q", request.URL.Path)
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
	root.SetArgs([]string{"stream", "--conversation-id", "338575800850784256", "--prompt", "生成图片"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if streamCalls != 1 || !strings.Contains(stderr.String(), "已有生成任务在运行") {
		t.Fatalf("streamCalls=%d stderr=%s", streamCalls, stderr.String())
	}
	var result api.StreamOutput
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("stdout is not valid JSON: %v; stdout=%q", err, stdout.String())
	}
	if result.TerminalType != "GenerationSuccess" || len(result.Results) != 1 || result.Results[0].URL != "https://example.test/result.jpg" {
		t.Fatalf("result = %#v", result)
	}
}

func TestCLIStreamKeepsMessageDeltasAndAssetsAcrossReconnect(t *testing.T) {
	var streamCalls int
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case config.StreamPath:
			streamCalls++
			writer.Header().Set("Content-Type", "application/x-ndjson")
			_, _ = writer.Write([]byte(`{"data":{"content":"前半段","message_id":"assistant-1"},"message_id":"assistant-1","seq":1,"type":"MessageDelta"}` + "\n"))
		case config.ResumeStreamPath:
			var body api.ResumeStreamRequest
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.FromSeq != 1 {
				t.Fatalf("resume body = %#v", body)
			}
			writer.Header().Set("Content-Type", "application/x-ndjson")
			_, _ = writer.Write([]byte(
				`{"data":{"content":"后半段","message_id":"assistant-1"},"message_id":"assistant-1","seq":2,"type":"MessageDelta"}` + "\n" +
					`{"data":{"results":[{"url":"https://example.test/clip.mp4","mimetype":"video/mp4"}]},"seq":3,"type":"GenerationArtifact"}` + "\n" +
					`{"data":{},"message_id":"assistant-1","seq":4,"type":"AgentEnd"}` + "\n",
			))
		default:
			t.Fatalf("path = %q", request.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv(config.EnvAPIBaseURL, server.URL)
	t.Setenv(config.EnvAccessToken, "test-token")
	t.Setenv(config.EnvConfigFile, filepath.Join(t.TempDir(), "config.json"))
	var stdout bytes.Buffer
	root, err := cmd.NewRootCommand(&stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	root.SetArgs([]string{"stream", "--conversation-id", "338575800850784256", "--prompt", "生成视频"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	var result api.StreamOutput
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("stdout is not valid JSON: %v; stdout=%q", err, stdout.String())
	}
	if streamCalls != 1 || result.AssistantText != "前半段后半段" || len(result.Assets) != 1 ||
		result.Assets[0].Result.URL != "https://example.test/clip.mp4" {
		t.Fatalf("result = %#v", result)
	}
}
