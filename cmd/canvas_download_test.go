package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCanvasTaskOutputDirDefaultsToWorkspaceTaskDirectory(t *testing.T) {
	workingDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	got, err := canvasTaskOutputDir("", "canvas-image/task:1")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(workingDir, "pavo_outputs", "canvas", "canvas-image-task-1")
	if got != want {
		t.Fatalf("output dir = %q, want %q", got, want)
	}
}

func TestCanvasResultExtensionPrefersURLThenMimetype(t *testing.T) {
	if got := canvasResultExtension(map[string]any{"mimetype": "image/png"}, "https://cdn.example.test/result.jpg?token=x"); got != ".jpg" {
		t.Fatalf("URL extension = %q", got)
	}
	if got := canvasResultExtension(map[string]any{"mimetype": "video/mp4"}, "https://cdn.example.test/result"); got != ".mp4" {
		t.Fatalf("mimetype extension = %q", got)
	}
}
