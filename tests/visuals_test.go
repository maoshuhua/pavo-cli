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

const visualsResponse = `{
  "code":"000000",
  "message":"success",
  "data":{
    "pagination":{"page":2,"page_size":3,"total":4},
    "groups":[{"date":"2026-08-18","list":[{
      "visual_id":347920702751436800,
      "source":"ai_creation",
      "resource_id":347920598527291392,
      "type":"image",
      "url":"",
      "thumbnail_url":"",
      "created_at":"2026-08-18T01:52:39Z",
      "metadata":{"mimetype":"image/jpeg","prompt":"南京大学海报","url":"https://example.test/image.jpg"}
    }]}]
  }
}`

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
		_, _ = writer.Write([]byte(visualsResponse))
	}))
	defer server.Close()

	client := api.NewClient(server.URL, 5*time.Second, &config.Paths{Visuals: config.VisualsPath}, func() (string, error) {
		return "current-user-token", nil
	})
	data, err := client.ListVisuals(context.Background(), api.VisualCategoryImages, 2, 3)
	if err != nil {
		t.Fatal(err)
	}
	if data.Pagination.Total != 4 || len(data.Groups) != 1 || len(data.Groups[0].List) != 1 {
		t.Fatalf("data = %#v", data)
	}
	item := data.Groups[0].List[0]
	if string(item.VisualID) != "347920702751436800" || !strings.Contains(string(item.Metadata), "南京大学海报") {
		t.Fatalf("item = %#v", item)
	}
}

func TestCLIVisualsListsImagesAndVideos(t *testing.T) {
	var categories []string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		categories = append(categories, request.URL.Query().Get("category"))
		_, _ = writer.Write([]byte(visualsResponse))
	}))
	defer server.Close()

	t.Setenv(config.EnvAPIBaseURL, server.URL)
	t.Setenv(config.EnvAccessToken, "current-user-token")
	t.Setenv(config.EnvConfigFile, filepath.Join(t.TempDir(), "config.json"))
	for _, category := range []string{"images", "videos"} {
		var stdout bytes.Buffer
		root, err := cmd.NewRootCommand(&stdout, &bytes.Buffer{})
		if err != nil {
			t.Fatal(err)
		}
		root.SetArgs([]string{"visuals", "--category", category, "--page", "2", "--page-size", "3"})
		if err := root.Execute(); err != nil {
			t.Fatal(err)
		}
		var result struct {
			Category   string               `json:"category"`
			Pagination api.VisualPagination `json:"pagination"`
			Groups     []api.VisualGroup    `json:"groups"`
		}
		if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
			t.Fatalf("stdout = %q: %v", stdout.String(), err)
		}
		if result.Category != category || result.Pagination.Total != 4 || len(result.Groups) != 1 {
			t.Fatalf("result = %#v", result)
		}
	}
	if len(categories) != 2 || categories[0] != "images" || categories[1] != "videos" {
		t.Fatalf("categories = %#v", categories)
	}
}

func TestCLIShortDramaListUsesFinalCategory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		query := request.URL.Query()
		if query.Get("category") != "short_drama_final" || query.Get("page") != "1" || query.Get("page_size") != "5" {
			t.Fatalf("query = %q", request.URL.RawQuery)
		}
		_, _ = writer.Write([]byte(visualsResponse))
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
	root.SetArgs([]string{"short-drama", "list"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), `"category":"short_drama_final"`) {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestCLIVisualsRejectsInvalidCategoryAndPagination(t *testing.T) {
	t.Setenv(config.EnvConfigFile, filepath.Join(t.TempDir(), "config.json"))
	for _, args := range [][]string{
		{"visuals", "--category", "shorts"},
		{"visuals", "--category", "images", "--page", "0"},
		{"short-drama", "list", "--page-size", "0"},
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
