package tests

import (
	"bytes"
	"context"
	"encoding/json"
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

func TestAPIUploadFilePresignsThenPUTsWithoutPAVOHeaders(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Image1.jpg")
	contents := []byte("image bytes")
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case config.PresignedURLPath:
			if request.Method != http.MethodPost {
				t.Fatalf("presign method = %s", request.Method)
			}
			if got := request.Header.Get("Authorization"); got != "Bearer test-token" {
				t.Fatalf("presign Authorization = %q", got)
			}
			if got := request.Header.Get("X-Platform"); got != "1" {
				t.Fatalf("presign X-Platform = %q", got)
			}
			var body api.PresignedURLRequest
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.Purpose != "chat_attachment" || body.ContentType != "image/jpeg" || body.Filename != "Image1.jpg" {
				t.Fatalf("presign body = %#v", body)
			}
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"code":"000000","message":"success","data":{"upload_url":"` + server.URL + `/upload/object?signature=temporary","public_url":"https://public.example.test/chat/Image1.jpg","method":"PUT","expires_in":600,"required_headers":{"Content-Type":"image/jpeg","X-Upload-Test":"required"}}}`))
		case "/upload/object":
			if request.Method != http.MethodPut {
				t.Fatalf("upload method = %s", request.Method)
			}
			if got := request.URL.Query().Get("signature"); got != "temporary" {
				t.Fatalf("upload signature = %q", got)
			}
			if got := request.Header.Get("Authorization"); got != "" {
				t.Fatalf("upload Authorization = %q", got)
			}
			if got := request.Header.Get("X-Platform"); got != "" {
				t.Fatalf("upload X-Platform = %q", got)
			}
			if got := request.Header.Get("User-Agent"); got == "PAVO-CLI/1.0" {
				t.Fatalf("upload leaked PAVO User-Agent")
			}
			if got := request.Header.Get("Content-Type"); got != "image/jpeg" {
				t.Fatalf("upload Content-Type = %q", got)
			}
			if got := request.Header.Get("X-Upload-Test"); got != "required" {
				t.Fatalf("upload required header = %q", got)
			}
			data, err := io.ReadAll(request.Body)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(data, contents) {
				t.Fatalf("upload body = %q", data)
			}
			writer.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected path %q", request.URL.Path)
		}
	}))
	defer server.Close()

	client := newTestClient(server.URL, func() (string, error) { return "test-token", nil })
	result, err := client.UploadFile(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if result.PublicURL != "https://public.example.test/chat/Image1.jpg" || result.ContentType != "image/jpeg" || result.Filename != "Image1.jpg" {
		t.Fatalf("result = %#v", result)
	}
}

func TestAPIUploadFileRejectsInvalidInputBeforeNetwork(t *testing.T) {
	client := newTestClient("http://127.0.0.1:1", func() (string, error) { return "test-token", nil })
	if _, err := client.UploadFile(context.Background(), filepath.Join(t.TempDir(), "missing.jpg")); err == nil || !strings.Contains(err.Error(), "上传文件不存在") {
		t.Fatalf("missing file error = %v", err)
	}
	if _, err := client.UploadFile(context.Background(), t.TempDir()); err == nil || !strings.Contains(err.Error(), "是目录") {
		t.Fatalf("directory error = %v", err)
	}
}

func TestAPIUploadFileDoesNotLeakSignedURLOnPUTFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Image1.jpg")
	if err := os.WriteFile(path, []byte("image bytes"), 0o600); err != nil {
		t.Fatal(err)
	}

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == config.PresignedURLPath {
			_, _ = writer.Write([]byte(`{"code":"000000","message":"success","data":{"upload_url":"` + server.URL + `/upload/object?signature=private-value","public_url":"https://public.example.test/chat/Image1.jpg","method":"PUT","required_headers":{"Content-Type":"image/jpeg"}}}`))
			return
		}
		if request.URL.Path == "/upload/object" {
			writer.WriteHeader(http.StatusForbidden)
			return
		}
		t.Fatalf("unexpected path %q", request.URL.Path)
	}))
	defer server.Close()

	client := newTestClient(server.URL, func() (string, error) { return "test-token", nil })
	_, err := client.UploadFile(context.Background(), path)
	if err == nil || !strings.Contains(err.Error(), "HTTP 403") || strings.Contains(err.Error(), "private-value") {
		t.Fatalf("upload error = %v", err)
	}
}

func TestCLIUploadPrintsOnlyPublicResult(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Image1.jpg")
	if err := os.WriteFile(path, []byte("image bytes"), 0o600); err != nil {
		t.Fatal(err)
	}

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == config.PresignedURLPath {
			_, _ = writer.Write([]byte(`{"code":"000000","message":"success","data":{"upload_url":"` + server.URL + `/upload/object?signature=private-value","public_url":"https://public.example.test/chat/Image1.jpg","method":"PUT","required_headers":{"Content-Type":"image/jpeg"}}}`))
			return
		}
		if request.URL.Path == "/upload/object" {
			writer.WriteHeader(http.StatusNoContent)
			return
		}
		t.Fatalf("unexpected path %q", request.URL.Path)
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
	root.SetArgs([]string{"upload", "--file", path})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stdout.String(), "private-value") {
		t.Fatalf("stdout leaked signed URL: %s", stdout.String())
	}
	var result api.FileUploadResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("stdout is not valid JSON: %v; stdout=%q", err, stdout.String())
	}
	if result.PublicURL != "https://public.example.test/chat/Image1.jpg" {
		t.Fatalf("result = %#v", result)
	}
}
