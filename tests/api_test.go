package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/maoshuhua/pavo-cli/internal/api"
	"github.com/maoshuhua/pavo-cli/internal/config"
)

const successEvent = `{"app_standalone_card":false,"data":{"event_id":"event-1","message_id":"message-1","model_code":"agnes-image","results":[{"base64":"","height":2624,"message":"ok","mimetype":"image/jpeg","ratio":"9:16","success":true,"thumbnail_url":"https://example.test/thumb.webp","url":"https://example.test/image.jpg","width":1472}],"trace_id":"trace-1"},"seq":12,"task_id":"pavo-task-1","ts":1784785350145,"type":"GenerationSuccess"}`

func TestAPILoginDoesNotSendAuthorizationAndParsesToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != config.LoginPath {
			t.Fatalf("path = %q", request.URL.Path)
		}
		if got := request.Header.Get("Authorization"); got != "" {
			t.Fatalf("Authorization = %q", got)
		}
		if got := request.Header.Get("X-Platform"); got != "1" {
			t.Fatalf("X-Platform = %q", got)
		}
		var body api.LoginRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.EmailPassword.Email != "user@example.com" || body.EmailPassword.Password != "secret" {
			t.Fatalf("body = %#v", body)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{
			"code":"000000",
			"message":"success",
			"data":{
				"access_token":"token-value",
				"user_info":{"id":"user-1","email":"user@example.com","is_active":true}
			}
		}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL, func() (string, error) { return "unused", nil })
	result, err := client.Login(context.Background(), " user@example.com ", "secret")
	if err != nil {
		t.Fatal(err)
	}
	if result.AccessToken != "token-value" || result.UserInfo.ID != "user-1" {
		t.Fatalf("result = %#v", result)
	}
}

func TestAPICreateConversationBuildsNestedTitleAndUsesBearerToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != config.ConversationPath {
			t.Fatalf("path = %q", request.URL.Path)
		}
		if got := request.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("Authorization = %q", got)
		}
		if got := request.Header.Get("X-Platform"); got != "1" {
			t.Fatalf("X-Platform = %q", got)
		}
		var body api.CreateConversationRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.FolderID != "" || body.KBStrict {
			t.Fatalf("body = %#v", body)
		}
		var title []map[string]string
		if err := json.Unmarshal([]byte(body.Title), &title); err != nil {
			t.Fatalf("title %q is invalid: %v", body.Title, err)
		}
		if len(title) != 1 || title[0]["type"] != "text" || title[0]["content"] != `生成"美女"图` {
			t.Fatalf("title = %#v", title)
		}
		_, _ = writer.Write([]byte(`{"code":"000000","message":"success","data":{"conversation_id":"338562408542949376"}}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL, func() (string, error) { return "test-token", nil })
	id, err := client.CreateConversation(context.Background(), `生成"美女"图`)
	if err != nil {
		t.Fatal(err)
	}
	if id != "338562408542949376" {
		t.Fatalf("conversation id = %q", id)
	}
}

func TestAPICreateConversationAcceptsNumericConversationID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(`{"code":"000000","message":"success","data":{"conversation_id":338562408542949376}}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL, func() (string, error) { return "test-token", nil })
	id, err := client.CreateConversation(context.Background(), "生成图片")
	if err != nil {
		t.Fatal(err)
	}
	if id != "338562408542949376" {
		t.Fatalf("conversation id = %q", id)
	}
}

func TestAPIBusinessError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(`{"code":"100001","message":"invalid login","data":{}}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL, nil)
	if _, err := client.Login(context.Background(), "user@example.com", "bad"); err == nil {
		t.Fatal("Login() error = nil")
	}
}

func TestAPIStreamReadsNDJSONUntilGenerationSuccess(t *testing.T) {
	var types []string
	server := newStreamServer(t, "application/x-ndjson", func(writer http.ResponseWriter) {
		_, _ = writer.Write([]byte(`{"data":{},"seq":1,"type":"GenerationStarted"}` + "\n"))
		if flusher, ok := writer.(http.Flusher); ok {
			flusher.Flush()
		}
		_, _ = writer.Write([]byte(successEvent + "\n"))
	})
	defer server.Close()

	client := newTestClient(server.URL, func() (string, error) { return "stream-token", nil })
	result, err := client.Stream(context.Background(), "conversation-1", "生成美女图", func(event *api.StreamEvent) error {
		types = append(types, event.Type)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(types, []string{"GenerationStarted", "GenerationSuccess"}) {
		t.Fatalf("types = %#v", types)
	}
	if result.TerminalType != "GenerationSuccess" || result.TaskID != "pavo-task-1" ||
		len(result.Results) != 1 || result.Results[0].Width != 1472 {
		t.Fatalf("result = %#v", result)
	}
}

func TestAPIStreamReadsSSE(t *testing.T) {
	server := newStreamServer(t, "text/event-stream", func(writer http.ResponseWriter) {
		_, _ = writer.Write([]byte("event: message\r\n"))
		_, _ = writer.Write([]byte("data: " + successEvent + "\r\n\r\n"))
	})
	defer server.Close()

	client := newTestClient(server.URL, func() (string, error) { return "stream-token", nil })
	result, err := client.Stream(context.Background(), "conversation-1", "prompt", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.EventID != "event-1" || result.Results[0].URL != "https://example.test/image.jpg" {
		t.Fatalf("result = %#v", result)
	}
}

func TestAPIStreamAcceptsAgentEndAndPreservesArtifacts(t *testing.T) {
	const artifactEvent = `{"data":{"model_code":"agnes-image","results":[{"height":1024,"mimetype":"image/png","ratio":"1:1","thumbnail_url":"https://example.test/thumb.png","url":"https://example.test/image.png","width":1024}],"task_id":"artifact-task","trace_id":"artifact-trace"},"event_id":"artifact-event","seq":2,"type":"GenerationArtifact"}`
	const agentEndEvent = `{"data":{},"seq":3,"type":"AgentEnd"}`
	server := newStreamServer(t, "application/x-ndjson", func(writer http.ResponseWriter) {
		_, _ = writer.Write([]byte(artifactEvent + "\n" + agentEndEvent + "\n"))
	})
	defer server.Close()

	client := newTestClient(server.URL, func() (string, error) { return "stream-token", nil })
	result, err := client.Stream(context.Background(), "conversation-1", "prompt", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.TerminalType != "AgentEnd" || result.EventID != "artifact-event" ||
		result.TaskID != "artifact-task" ||
		len(result.Artifacts) != 1 || len(result.Results) != 1 ||
		!result.Results[0].Success || result.Results[0].URL != "https://example.test/image.png" {
		t.Fatalf("result = %#v", result)
	}
	if !strings.Contains(string(result.Artifacts[0]), `"type":"GenerationArtifact"`) {
		t.Fatalf("artifact = %s", result.Artifacts[0])
	}
}

func TestAPIStreamSendsNumericConversationIDAsJSONNumber(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if got := request.Header.Get("X-Platform"); got != "1" {
			t.Fatalf("X-Platform = %q", got)
		}
		var body struct {
			ConversationID json.RawMessage `json:"conversation_id"`
			Prompt         string          `json:"prompt"`
			Mode           string          `json:"mode"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if string(body.ConversationID) != "338575800850784256" {
			t.Fatalf("raw conversation_id = %s", body.ConversationID)
		}
		_, _ = writer.Write([]byte(successEvent))
	}))
	defer server.Close()

	client := newTestClient(server.URL, func() (string, error) { return "stream-token", nil })
	if _, err := client.Stream(context.Background(), "338575800850784256", "prompt", nil); err != nil {
		t.Fatal(err)
	}
}

func TestAPIStreamSendsAttachments(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var body api.StreamRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if len(body.Files) != 1 || body.Files[0] != (api.ChatAttachment{
			MimeType: "image/jpeg",
			URL:      "https://public.example.test/chat/Image1.jpg",
			Filename: "Image1.jpg",
		}) {
			t.Fatalf("files = %#v", body.Files)
		}
		_, _ = writer.Write([]byte(successEvent))
	}))
	defer server.Close()

	client := newTestClient(server.URL, func() (string, error) { return "stream-token", nil })
	_, err := client.StreamWithFiles(context.Background(), "conversation-1", "prompt", []api.ChatAttachment{{
		MimeType: " image/jpeg ",
		URL:      " https://public.example.test/chat/Image1.jpg ",
		Filename: " Image1.jpg ",
	}}, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestAPIStreamRequiresGenerationSuccess(t *testing.T) {
	server := newStreamServer(t, "application/json", func(writer http.ResponseWriter) {
		_, _ = writer.Write([]byte(`{"data":{},"seq":1,"type":"GenerationStarted"}`))
	})
	defer server.Close()

	client := newTestClient(server.URL, func() (string, error) { return "stream-token", nil })
	if _, err := client.Stream(context.Background(), "conversation-1", "prompt", nil); err == nil {
		t.Fatal("Stream() error = nil")
	}
}

func newTestClient(baseURL string, provider api.TokenProvider) *api.Client {
	return api.NewClient(baseURL, 5*time.Second, &config.Paths{
		Login:        config.LoginPath,
		Conversation: config.ConversationPath,
		Stream:       config.StreamPath,
		PresignedURL: config.PresignedURLPath,
	}, provider)
}

func newStreamServer(t *testing.T, contentType string, write func(http.ResponseWriter)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != config.StreamPath {
			t.Fatalf("path = %q", request.URL.Path)
		}
		if got := request.Header.Get("Authorization"); got != "Bearer stream-token" {
			t.Fatalf("Authorization = %q", got)
		}
		if got := request.Header.Get("X-Platform"); got != "1" {
			t.Fatalf("X-Platform = %q", got)
		}
		var body api.StreamRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.ConversationID != "conversation-1" || body.Mode != "design" {
			t.Fatalf("body = %#v", body)
		}
		writer.Header().Set("Content-Type", contentType)
		write(writer)
	}))
}
