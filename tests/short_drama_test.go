package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
