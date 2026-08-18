package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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

type visualCLIOutput struct {
	Category    string               `json:"category"`
	DownloadDir string               `json:"download_dir"`
	Downloaded  int                  `json:"downloaded"`
	Failed      int                  `json:"failed"`
	Pagination  api.VisualPagination `json:"pagination"`
	Groups      []api.VisualGroup    `json:"groups"`
}

func visualsResponse(assetType, mimetype string, assetURLs ...string) string {
	items := make([]string, 0, len(assetURLs))
	for index, assetURL := range assetURLs {
		items = append(items, fmt.Sprintf(`{
      "visual_id":%d,
      "source":"ai_creation",
      "resource_id":%d,
      "type":%q,
      "url":"",
      "thumbnail_url":"",
      "created_at":"2026-08-18T01:52:39Z",
      "metadata":{"mimetype":%q,"prompt":"南京大学海报","url":%q}
    }`, 347920702751436800+index, 347920598527291392+index, assetType, mimetype, assetURL))
	}
	return fmt.Sprintf(`{
  "code":"000000",
  "message":"success",
  "data":{
    "pagination":{"page":2,"page_size":3,"total":%d},
    "groups":[{"date":"2026-08-18","list":[%s]}]
  }
}`, len(items), strings.Join(items, ","))
}

func TestAPIListVisualsUsesCurrentLoginAndPreservesMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != config.VisualsPath {
			t.Fatalf("request = %s %s", request.Method, request.URL.Path)
		}
		if got := request.Header.Get("Authorization"); got != "Bearer current-user-token" {
			t.Fatalf("Authorization = %q", got)
		}
		query := request.URL.Query()
		if query.Get("category") != "images" || query.Get("page") != "2" || query.Get("page_size") != "3" {
			t.Fatalf("query = %q", request.URL.RawQuery)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(visualsResponse("image", "image/jpeg", "https://example.test/image.jpg")))
	}))
	defer server.Close()

	client := api.NewClient(server.URL, 5*time.Second, &config.Paths{Visuals: config.VisualsPath}, func() (string, error) {
		return "current-user-token", nil
	})
	data, err := client.ListVisuals(context.Background(), api.VisualCategoryImages, 2, 3)
	if err != nil {
		t.Fatal(err)
	}
	if data.Pagination.Total != 1 || len(data.Groups) != 1 || len(data.Groups[0].List) != 1 {
		t.Fatalf("data = %#v", data)
	}
	item := data.Groups[0].List[0]
	if string(item.VisualID) != "347920702751436800" || !strings.Contains(string(item.Metadata), "南京大学海报") {
		t.Fatalf("item = %#v", item)
	}
}

func TestCLIVisualsListsImagesAndVideos(t *testing.T) {
	var categories []string
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case config.VisualsPath:
			category := request.URL.Query().Get("category")
			categories = append(categories, category)
			assetType, mimetype, assetPath := "image", "image/jpeg", "/asset.jpg"
			if category == "videos" {
				assetType, mimetype, assetPath = "video", "video/mp4", "/asset.mp4"
			}
			_, _ = writer.Write([]byte(visualsResponse(assetType, mimetype, serverURL+assetPath)))
		case "/asset.jpg":
			_, _ = writer.Write([]byte("image asset"))
		case "/asset.mp4":
			_, _ = writer.Write([]byte("video asset"))
		default:
			t.Fatalf("path = %q", request.URL.Path)
		}
	}))
	defer server.Close()
	serverURL = server.URL

	t.Setenv(config.EnvAPIBaseURL, server.URL)
	t.Setenv(config.EnvAccessToken, "current-user-token")
	t.Setenv(config.EnvConfigFile, filepath.Join(t.TempDir(), "config.json"))
	for _, category := range []string{"images", "videos"} {
		var stdout bytes.Buffer
		downloadDir := filepath.Join(t.TempDir(), category)
		root, err := cmd.NewRootCommand(&stdout, &bytes.Buffer{})
		if err != nil {
			t.Fatal(err)
		}
		root.SetArgs([]string{"visuals", "--category", category, "--page", "2", "--page-size", "3", "--download-dir", downloadDir})
		if err := root.Execute(); err != nil {
			t.Fatal(err)
		}
		var result visualCLIOutput
		if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
			t.Fatalf("stdout = %q: %v", stdout.String(), err)
		}
		if result.Category != category || result.Pagination.Total != 1 || result.Downloaded != 1 || len(result.Groups) != 1 || len(result.Groups[0].List) != 1 {
			t.Fatalf("result = %#v", result)
		}
		localPath := result.Groups[0].List[0].LocalPath
		if !filepath.IsAbs(localPath) || !strings.HasPrefix(localPath, result.DownloadDir) {
			t.Fatalf("local_path = %q download_dir=%q", localPath, result.DownloadDir)
		}
		if _, err := os.Stat(localPath); err != nil {
			t.Fatalf("downloaded asset missing: %v", err)
		}
	}
	if len(categories) != 2 || categories[0] != "images" || categories[1] != "videos" {
		t.Fatalf("categories = %#v", categories)
	}
}

func TestCLIShortDramaListUsesFinalCategory(t *testing.T) {
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case config.VisualsPath:
			query := request.URL.Query()
			if query.Get("category") != "short_drama_final" || query.Get("page") != "1" || query.Get("page_size") != "5" {
				t.Fatalf("query = %q", request.URL.RawQuery)
			}
			_, _ = writer.Write([]byte(visualsResponse("video", "video/mp4", serverURL+"/short-drama.mp4")))
		case "/short-drama.mp4":
			_, _ = writer.Write([]byte("short drama"))
		default:
			t.Fatalf("path = %q", request.URL.Path)
		}
	}))
	defer server.Close()
	serverURL = server.URL

	t.Setenv(config.EnvAPIBaseURL, server.URL)
	t.Setenv(config.EnvAccessToken, "current-user-token")
	t.Setenv(config.EnvConfigFile, filepath.Join(t.TempDir(), "config.json"))
	var stdout bytes.Buffer
	root, err := cmd.NewRootCommand(&stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	root.SetArgs([]string{"short-drama", "list", "--download-dir", t.TempDir()})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), `"category":"short_drama_final"`) {
		t.Fatalf("stdout = %q", stdout.String())
	}
	var result visualCLIOutput
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil || result.Downloaded != 1 || result.Groups[0].List[0].LocalPath == "" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func TestCLIVisualsDownloadsAssetsInParallel(t *testing.T) {
	var active int32
	var maxActive int32
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == config.VisualsPath {
			urls := []string{serverURL + "/asset-1.jpg", serverURL + "/asset-2.jpg", serverURL + "/asset-3.jpg", serverURL + "/asset-4.jpg"}
			_, _ = writer.Write([]byte(visualsResponse("image", "image/jpeg", urls...)))
			return
		}
		if !strings.HasPrefix(request.URL.Path, "/asset-") {
			t.Fatalf("path = %q", request.URL.Path)
		}
		current := atomic.AddInt32(&active, 1)
		for {
			observed := atomic.LoadInt32(&maxActive)
			if current <= observed || atomic.CompareAndSwapInt32(&maxActive, observed, current) {
				break
			}
		}
		defer atomic.AddInt32(&active, -1)
		time.Sleep(100 * time.Millisecond)
		_, _ = writer.Write([]byte("asset"))
	}))
	defer server.Close()
	serverURL = server.URL

	t.Setenv(config.EnvAPIBaseURL, server.URL)
	t.Setenv(config.EnvAccessToken, "current-user-token")
	t.Setenv(config.EnvConfigFile, filepath.Join(t.TempDir(), "config.json"))
	var stdout bytes.Buffer
	root, err := cmd.NewRootCommand(&stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	root.SetArgs([]string{"visuals", "--category", "images", "--download-dir", t.TempDir(), "--download-concurrency", "4"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	var result visualCLIOutput
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Downloaded != 4 || atomic.LoadInt32(&maxActive) < 2 {
		t.Fatalf("downloaded=%d maxActive=%d", result.Downloaded, maxActive)
	}
	for _, item := range result.Groups[0].List {
		if !filepath.IsAbs(item.LocalPath) {
			t.Fatalf("local_path = %q", item.LocalPath)
		}
	}
}

func TestCLIVisualsUsesDefaultWorkspaceDownloadDirectory(t *testing.T) {
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case config.VisualsPath:
			_, _ = writer.Write([]byte(visualsResponse("image", "image/jpeg", serverURL+"/default.jpg")))
		case "/default.jpg":
			_, _ = writer.Write([]byte("default asset"))
		default:
			t.Fatalf("path = %q", request.URL.Path)
		}
	}))
	defer server.Close()
	serverURL = server.URL

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	workspace := t.TempDir()
	if err := os.Chdir(workspace); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})

	t.Setenv(config.EnvAPIBaseURL, server.URL)
	t.Setenv(config.EnvAccessToken, "current-user-token")
	t.Setenv(config.EnvConfigFile, filepath.Join(t.TempDir(), "config.json"))
	var stdout bytes.Buffer
	root, err := cmd.NewRootCommand(&stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	root.SetArgs([]string{"visuals", "--category", "images"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	var result visualCLIOutput
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	wantDir := filepath.Join(workspace, "pavo_outputs", "visuals", "images")
	if result.DownloadDir != wantDir || !strings.HasPrefix(result.Groups[0].List[0].LocalPath, wantDir) {
		t.Fatalf("download_dir=%q local_path=%q want prefix=%q", result.DownloadDir, result.Groups[0].List[0].LocalPath, wantDir)
	}
}

func TestCLIVisualsReportsMissingDownloadURLWithoutFailingQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(visualsResponse("image", "image/jpeg", "")))
	}))
	defer server.Close()

	t.Setenv(config.EnvAPIBaseURL, server.URL)
	t.Setenv(config.EnvAccessToken, "current-user-token")
	t.Setenv(config.EnvConfigFile, filepath.Join(t.TempDir(), "config.json"))
	var stdout bytes.Buffer
	root, err := cmd.NewRootCommand(&stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	root.SetArgs([]string{"visuals", "--category", "images", "--download-dir", t.TempDir()})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	var result visualCLIOutput
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	item := result.Groups[0].List[0]
	if result.Downloaded != 0 || result.Failed != 1 || item.LocalPath != "" || item.DownloadError != "缺少可下载 URL" {
		t.Fatalf("result = %#v", result)
	}
}

func TestCLIVisualsContinuesAfterOneAssetDownloadFails(t *testing.T) {
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case config.VisualsPath:
			_, _ = writer.Write([]byte(visualsResponse("image", "image/jpeg", serverURL+"/missing.jpg", serverURL+"/ok.jpg")))
		case "/missing.jpg":
			http.NotFound(writer, request)
		case "/ok.jpg":
			_, _ = writer.Write([]byte("available asset"))
		default:
			t.Fatalf("path = %q", request.URL.Path)
		}
	}))
	defer server.Close()
	serverURL = server.URL

	t.Setenv(config.EnvAPIBaseURL, server.URL)
	t.Setenv(config.EnvAccessToken, "current-user-token")
	t.Setenv(config.EnvConfigFile, filepath.Join(t.TempDir(), "config.json"))
	var stdout bytes.Buffer
	root, err := cmd.NewRootCommand(&stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	root.SetArgs([]string{"visuals", "--category", "images", "--download-dir", t.TempDir(), "--download-concurrency", "1"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	var result visualCLIOutput
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	failedItem := result.Groups[0].List[0]
	successfulItem := result.Groups[0].List[1]
	if result.Downloaded != 1 || result.Failed != 1 {
		t.Fatalf("downloaded=%d failed=%d", result.Downloaded, result.Failed)
	}
	if failedItem.LocalPath != "" || failedItem.DownloadError != "下载返回 HTTP 404" {
		t.Fatalf("failed item = %#v", failedItem)
	}
	if successfulItem.DownloadError != "" || !filepath.IsAbs(successfulItem.LocalPath) {
		t.Fatalf("successful item = %#v", successfulItem)
	}
	if _, err := os.Stat(successfulItem.LocalPath); err != nil {
		t.Fatalf("successful asset missing: %v", err)
	}
}

func TestCLIVisualsRejectsInvalidCategoryAndPagination(t *testing.T) {
	t.Setenv(config.EnvConfigFile, filepath.Join(t.TempDir(), "config.json"))
	for _, args := range [][]string{
		{"visuals", "--category", "shorts"},
		{"visuals", "--category", "images", "--page", "0"},
		{"short-drama", "list", "--download-concurrency", "0"},
		{"visuals", "--category", "images", "--download-concurrency", "33"},
	} {
		root, err := cmd.NewRootCommand(&bytes.Buffer{}, &bytes.Buffer{})
		if err != nil {
			t.Fatal(err)
		}
		root.SetArgs(args)
		if err := root.Execute(); err == nil {
			t.Fatalf("args %q unexpectedly succeeded", args)
		}
	}
}
