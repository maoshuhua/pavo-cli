package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/maoshuhua/pavo-cli/cmd"
	"github.com/maoshuhua/pavo-cli/internal/api"
	"github.com/maoshuhua/pavo-cli/internal/config"
)

func TestCLIDownloadResultWritesFileWithoutCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/result.jpg" {
			t.Fatalf("request = %s %s", request.Method, request.URL.Path)
		}
		if got := request.Header.Get("Authorization"); got != "" {
			t.Fatalf("download Authorization = %q", got)
		}
		if got := request.Header.Get("X-Platform"); got != "" {
			t.Fatalf("download X-Platform = %q", got)
		}
		if got := request.Header.Get("User-Agent"); got != "PAVO-CLI/1.0" {
			t.Fatalf("download User-Agent = %q", got)
		}
		_, _ = writer.Write([]byte("generated image"))
	}))
	defer server.Close()

	t.Setenv(config.EnvAPIBaseURL, server.URL)
	t.Setenv(config.EnvAccessToken, "test-token")
	t.Setenv(config.EnvConfigFile, filepath.Join(t.TempDir(), "config.json"))
	outputPath := filepath.Join(t.TempDir(), "results", "image.jpg")
	var stdout, stderr bytes.Buffer
	root, err := cmd.NewRootCommand(&stdout, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	root.SetArgs([]string{"download-result", "--url", server.URL + "/result.jpg", "--output-path", outputPath})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v; stderr = %s", err, stderr.String())
	}

	var result api.DownloadResultResponse
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("stdout is not valid JSON: %v; stdout=%q", err, stdout.String())
	}
	if result.OutputPath != outputPath || len(result.Downloaded) != 1 || result.Downloaded[0] != outputPath {
		t.Fatalf("result = %#v", result)
	}
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "generated image" {
		t.Fatalf("file = %q", content)
	}
}

func TestCLIDownloadResultSkipsExistingUnlessForced(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests++
		_, _ = writer.Write([]byte("new asset"))
	}))
	defer server.Close()

	outputPath := filepath.Join(t.TempDir(), "image.jpg")
	if err := os.WriteFile(outputPath, []byte("old asset"), 0o600); err != nil {
		t.Fatal(err)
	}
	client := newTestClient(server.URL, func() (string, error) { return "test-token", nil })
	result, err := client.DownloadResult(context.Background(), api.DownloadResultOptions{URL: server.URL, OutputPath: outputPath})
	if err != nil {
		t.Fatal(err)
	}
	if requests != 0 || len(result.AlreadyExist) != 1 || result.AlreadyExist[0] != outputPath {
		t.Fatalf("result = %#v, requests = %d", result, requests)
	}

	result, err = client.DownloadResult(context.Background(), api.DownloadResultOptions{URL: server.URL, OutputPath: outputPath, Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if requests != 1 || len(result.Downloaded) != 1 {
		t.Fatalf("result = %#v, requests = %d", result, requests)
	}
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "new asset" {
		t.Fatalf("file = %q", content)
	}
}

func TestAPIDownloadResultRefreshesStaleFileAndRetries(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests++
		if requests == 1 {
			writer.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = writer.Write([]byte("new asset"))
	}))
	defer server.Close()

	outputPath := filepath.Join(t.TempDir(), "image.jpg")
	if err := os.WriteFile(outputPath, []byte("old asset"), 0o600); err != nil {
		t.Fatal(err)
	}
	updatedAt := time.Now().Add(-time.Minute).Unix()
	staleTime := time.Unix(updatedAt-10, 0)
	if err := os.Chtimes(outputPath, staleTime, staleTime); err != nil {
		t.Fatal(err)
	}

	client := newTestClient(server.URL, nil)
	result, err := client.DownloadResult(context.Background(), api.DownloadResultOptions{
		URL:        server.URL,
		OutputPath: outputPath,
		UpdatedAt:  updatedAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	if requests != 2 || len(result.Downloaded) != 1 {
		t.Fatalf("result = %#v, requests = %d", result, requests)
	}
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "new asset" {
		t.Fatalf("file = %q", content)
	}
	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.ModTime().Unix() != updatedAt {
		t.Fatalf("mtime = %d, want %d", info.ModTime().Unix(), updatedAt)
	}
}

func TestAPIDownloadResultRejectsInvalidURL(t *testing.T) {
	client := newTestClient("http://127.0.0.1:1", nil)
	_, err := client.DownloadResult(context.Background(), api.DownloadResultOptions{
		URL:        "file:///etc/passwd",
		OutputPath: filepath.Join(t.TempDir(), "result"),
	})
	if err == nil || !strings.Contains(err.Error(), "不是有效的 HTTP URL") {
		t.Fatalf("error = %v", err)
	}
}
