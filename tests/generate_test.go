package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/maoshuhua/pavo-cli/cmd"
	"github.com/maoshuhua/pavo-cli/internal/api"
	"github.com/maoshuhua/pavo-cli/internal/config"
)

func TestAPIListModeSupportModelsPreservesDynamicMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != config.ModeSupportModelsPath {
			t.Fatalf("path = %q", request.URL.Path)
		}
		if got := request.URL.Query().Get("mode_code"); got != "generate_video" {
			t.Fatalf("mode_code = %q", got)
		}
		if got := request.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("Authorization = %q", got)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"code":"000000","message":"success","data":{"models":[{"code":"agnes-video-new","name":"Agnes Video","model_intro":"intro","icon_url":"https://example.test/icon.png","is_online":true,"subscription_level":0,"tags":[{"code":"free","i18n_code":"zh-Hans","label":"免费"}],"modes":["frames_to_video"]}]}}`))
	}))
	defer server.Close()

	client := api.NewClient(server.URL, 5*time.Second, &config.Paths{
		ModeSupportModels: config.ModeSupportModelsPath,
	}, func() (string, error) { return "test-token", nil })
	models, err := client.ListModeSupportModels(context.Background(), api.ModeCodeGenerateVideo)
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 1 || models[0].Code != "agnes-video-new" || len(models[0].Modes) != 1 ||
		len(models[0].Tags) != 1 || models[0].Tags[0].Code != "free" {
		t.Fatalf("models = %#v", models)
	}
}

func TestCLIModelsFiltersShortDramaTypeAndOnlineState(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != config.ModeSupportModelsPath || request.URL.Query().Get("mode_code") != "short_drama" {
			t.Fatalf("request = %s?%s", request.URL.Path, request.URL.RawQuery)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"code":"000000","message":"success","data":{"models":[{"code":"agnes-image","name":"Agnes Image","is_online":true,"type":"image","subscription_level":0,"tags":[{"code":"free","label":"免费"}]},{"code":"offline-image","name":"Offline","is_online":false,"type":"image","subscription_level":0,"tags":[]},{"code":"agnes-video-new","name":"Agnes Video","is_online":true,"type":"video","subscription_level":0,"tags":[{"code":"free","label":"免费"}]}]}}`))
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
	root.SetArgs([]string{"models", "--mode", "short_drama", "--type", "image", "--online-only"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	var output struct {
		Mode   string               `json:"mode"`
		Models []api.SupportedModel `json:"models"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatalf("stdout = %q: %v", stdout.String(), err)
	}
	if output.Mode != "short_drama" || len(output.Models) != 1 || output.Models[0].Code != "agnes-image" {
		t.Fatalf("output = %#v", output)
	}
}

func TestCLIGenerateImageUsesCreativePixaRequestContract(t *testing.T) {
	const conversationID = "346482729455452160"
	var streamCalls int
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case config.ModeSupportModelsPath:
			if request.URL.Query().Get("mode_code") != "generate_image" {
				t.Fatalf("mode_code = %q", request.URL.Query().Get("mode_code"))
			}
			_, _ = writer.Write([]byte(`{"code":"000000","message":"success","data":{"models":[{"code":"agnes-image","name":"Agnes Image","is_online":true,"subscription_level":0,"tags":[{"code":"free","label":"免费"}]}]}}`))
		case config.StreamPath:
			streamCalls++
			var body api.StreamRequest
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.ConversationID != conversationID || body.Prompt != "美颜下" || body.Mode != "generate_image" ||
				body.Model != "agnes-image" || body.Ratio != "9:16" || body.Resolution != "SD" {
				t.Fatalf("body = %#v", body)
			}
			if string(body.Count) != "1" || len(body.Images) != 1 || body.Images[0].URL != "https://example.test/portrait.jpg" {
				t.Fatalf("creative fields = %#v", body)
			}
			var blocks []map[string]string
			if err := json.Unmarshal([]byte(body.CreativePromptJSON), &blocks); err != nil {
				t.Fatalf("creative_prompt_json = %q: %v", body.CreativePromptJSON, err)
			}
			if len(blocks) != 1 || blocks[0]["type"] != "text" || blocks[0]["content"] != "美颜下" {
				t.Fatalf("blocks = %#v", blocks)
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
	root, err := cmd.NewRootCommand(&stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	root.SetArgs([]string{
		"generate", "image",
		"--conversation-id", conversationID,
		"--prompt", "美颜下",
		"--ratio", "9:16",
		"--resolution", "sd",
		"--image", "https://example.test/portrait.jpg",
	})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if streamCalls != 1 {
		t.Fatalf("streamCalls = %d", streamCalls)
	}
	var result api.StreamOutput
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil || result.ConversationID != conversationID {
		t.Fatalf("stdout = %q result=%#v err=%v", stdout.String(), result, err)
	}
}

func TestCLIGenerateVideoSendsAutoParametersAndRejectsAgnesOutOfRange(t *testing.T) {
	const conversationID = "346482729455452160"
	var streamCalls int
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case config.ModeSupportModelsPath:
			_, _ = writer.Write([]byte(`{"code":"000000","message":"success","data":{"models":[{"code":"agnes-video-new","name":"Agnes Video","is_online":true,"subscription_level":0,"tags":[{"code":"free","label":"免费"}],"modes":["frames_to_video"]}]}}`))
		case config.StreamPath:
			streamCalls++
			var body api.StreamRequest
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.Mode != "frames_to_video" || body.Model != "agnes-video-new" || string(body.Duration) != `"auto"` ||
				string(body.Sound) != `"auto"` || string(body.Count) != "1" {
				t.Fatalf("body = %#v", body)
			}
			if len(body.Images) != 1 || body.Images[0].URL != "https://example.test/frame.jpg" || body.CreativePromptJSON != "" {
				t.Fatalf("frame fields = %#v", body)
			}
			writer.Header().Set("Content-Type", "application/x-ndjson")
			_, _ = writer.Write([]byte(`{"data":{},"seq":1,"type":"AgentEnd"}` + "\n"))
		default:
			t.Fatalf("unexpected path %q", request.URL.Path)
		}
	}))
	defer server.Close()

	newRoot := func(stdout *bytes.Buffer) *cobraCommandAdapter {
		root, err := cmd.NewRootCommand(stdout, &bytes.Buffer{})
		if err != nil {
			t.Fatal(err)
		}
		return &cobraCommandAdapter{setArgs: root.SetArgs, execute: root.Execute}
	}
	t.Setenv(config.EnvAPIBaseURL, server.URL)
	t.Setenv(config.EnvAccessToken, "test-token")
	t.Setenv(config.EnvConfigFile, filepath.Join(t.TempDir(), "config.json"))
	var stdout bytes.Buffer
	valid := newRoot(&stdout)
	valid.setArgs([]string{"generate", "video", "--conversation-id", conversationID, "--prompt", "海边日落", "--image", "https://example.test/frame.jpg"})
	if err := valid.execute(); err != nil {
		t.Fatal(err)
	}
	invalid := newRoot(&bytes.Buffer{})
	invalid.setArgs([]string{"generate", "video", "--conversation-id", conversationID, "--prompt", "海边日落", "--image", "https://example.test/frame.jpg", "--duration", "4"})
	if err := invalid.execute(); err == nil || !strings.Contains(err.Error(), "5 到 15 秒") {
		t.Fatalf("invalid duration error = %v", err)
	}
	if streamCalls != 1 {
		t.Fatalf("streamCalls = %d", streamCalls)
	}
}

func TestCLIGenerateVideoUsesFramesModeForTextToVideo(t *testing.T) {
	const conversationID = "346482729455452160"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case config.ModeSupportModelsPath:
			_, _ = writer.Write([]byte(`{"code":"000000","message":"success","data":{"models":[{"code":"dual-video","name":"Dual Video","is_online":true,"subscription_level":0,"tags":[{"code":"free","label":"免费"}],"modes":["omni_to_video","frames_to_video"]}]}}`))
		case config.StreamPath:
			var body api.StreamRequest
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.Mode != "frames_to_video" || body.Model != "dual-video" || len(body.Images) != 0 ||
				len(body.Videos) != 0 || len(body.Audios) != 0 || body.CreativePromptJSON != "" {
				t.Fatalf("body = %#v", body)
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
	root, err := cmd.NewRootCommand(&bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	root.SetArgs([]string{
		"generate", "video",
		"--conversation-id", conversationID,
		"--prompt", "海边日落的延时摄影",
		"--model", "dual-video",
	})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestCLIGenerateVideoUsesCreativeModeForOmniModel(t *testing.T) {
	const conversationID = "346482729455452160"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case config.ModeSupportModelsPath:
			_, _ = writer.Write([]byte(`{"code":"000000","message":"success","data":{"models":[{"code":"seedance-2-0","name":"Seedance 2.0","is_online":true,"subscription_level":1.5,"tags":[],"modes":["omni_to_video","frames_to_video"]}]}}`))
		case config.StreamPath:
			var body api.StreamRequest
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.Mode != "generate_video" || body.Model != "seedance-2-0" || body.CreativePromptJSON == "" ||
				len(body.Videos) != 1 || body.Videos[0].URL != "https://example.test/reference.mp4" {
				t.Fatalf("body = %#v", body)
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
	root, err := cmd.NewRootCommand(&bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	root.SetArgs([]string{
		"generate", "video",
		"--conversation-id", conversationID,
		"--prompt", "模仿参考视频动作",
		"--model", "seedance-2-0",
		"--video", "https://example.test/reference.mp4",
	})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
}

// cobraCommandAdapter keeps this test helper local without exporting cmd internals.
type cobraCommandAdapter struct {
	setArgs func([]string)
	execute func() error
}
