package canvas

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestBindingRoundTripAndParentDiscovery(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "one", "two")
	if err := os.MkdirAll(nested, 0o700); err != nil {
		t.Fatal(err)
	}
	path, err := WriteBinding(root, Binding{ProjectUUID: "project-1", CanvasUUID: "canvas-1", SessionID: "session-1"})
	if err != nil {
		t.Fatal(err)
	}
	binding, foundPath, err := FindBinding(nested)
	if err != nil {
		t.Fatal(err)
	}
	if foundPath != path || binding.ProjectUUID != "project-1" || binding.CanvasUUID != "canvas-1" || binding.SessionID != "session-1" {
		t.Fatalf("binding = %#v, path = %q", binding, foundPath)
	}
	if err := RemoveBinding(path); err != nil {
		t.Fatal(err)
	}
	if _, _, err := FindBinding(nested); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("FindBinding() error = %v", err)
	}
}

func TestRandomUUIDAndNodeKeyContracts(t *testing.T) {
	uuid, err := RandomUUID()
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`).MatchString(uuid) {
		t.Fatalf("uuid = %q", uuid)
	}
	key, err := NewNodeKey("upload", "video")
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^v-[A-Za-z0-9_-]{12}$`).MatchString(key) {
		t.Fatalf("node key = %q", key)
	}
}
