package canvas

import (
	"testing"

	"github.com/maoshuhua/pavo-cli/internal/api"
)

func TestBuildURLUsesFrontendCanvasRoute(t *testing.T) {
	got := BuildURL("https://app-test.pavo-ai.cn/", "338562408542949376", "project one", "canvas/two")
	want := "https://app-test.pavo-ai.cn/canvas/338562408542949376?canvas_uuid=canvas%2Ftwo&project_uuid=project+one"
	if got != want {
		t.Fatalf("BuildURL() = %q, want %q", got, want)
	}
}

func TestCanvasIdentifiersUseDocumentedFallbacks(t *testing.T) {
	detail := &api.CanvasProjectDetail{
		ProjectMeta:   api.CanvasProjectMeta{ID: "11"},
		CurrentCanvas: api.CanvasInfo{ProjectID: "22"},
	}
	if got := ProjectIDFromDetail(detail); got != "22" {
		t.Fatalf("ProjectIDFromDetail() = %q", got)
	}
	entry := api.CanvasProjectEntry{ID: "33", LatestCanvasUUID: "canvas-latest"}
	if got := ProjectIDFromEntry(entry); got != "33" {
		t.Fatalf("ProjectIDFromEntry() = %q", got)
	}
	if got := CanvasUUIDFromEntry(entry); got != "canvas-latest" {
		t.Fatalf("CanvasUUIDFromEntry() = %q", got)
	}
}
