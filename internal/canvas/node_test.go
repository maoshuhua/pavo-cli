package canvas

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/maoshuhua/pavo-cli/internal/api"
)

func TestFindNodePrefersKeyAndRejectsAmbiguousTitle(t *testing.T) {
	detail := &api.CanvasProjectDetail{NodeList: []api.CanvasNode{
		{NodeKey: "i-one", Name: "同名", Data: json.RawMessage(`{"title":"同名"}`)},
		{NodeKey: "i-two", Name: "同名", Data: json.RawMessage(`{"title":"同名"}`)},
	}}
	node, err := FindNode(detail, "i-two")
	if err != nil || node.NodeKey != "i-two" {
		t.Fatalf("FindNode by key = %#v, %v", node, err)
	}
	if _, err := FindNode(detail, "同名"); err == nil || !strings.Contains(err.Error(), "不唯一") {
		t.Fatalf("ambiguous error = %v", err)
	}
}

func TestNewNodeBuildsFrontendCompatibleData(t *testing.T) {
	detail := &api.CanvasProjectDetail{}
	item, err := NewNode(detail, NewNodeOptions{Type: "image", Name: "主视觉", Prompt: "一只猫", Model: "model-x", Data: map[string]any{"future_field": json.Number("338562408542949376")}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(item.NodeKey, "i-") || item.Name != "主视觉" || item.Measured.Width != "280" {
		t.Fatalf("item = %#v", item)
	}
	data, err := ParseObject(item.Data)
	if err != nil {
		t.Fatal(err)
	}
	params, ok := data["params"].(map[string]any)
	if !ok || params["model"] != "model-x" || data["node_key"] != item.NodeKey || data["isExecutable"] != true {
		t.Fatalf("data = %#v", data)
	}
	if number, ok := data["future_field"].(json.Number); !ok || number.String() != "338562408542949376" {
		t.Fatalf("future_field = %#v", data["future_field"])
	}
}

func TestApplyModelConfigurationUsesLiveConstraints(t *testing.T) {
	data := map[string]any{
		"params": map[string]any{
			"duration": 99,
			"settings": map[string]any{"ratio": "unsupported", "resolution": "qhd", "generateAudio": true},
		},
	}
	raw := json.RawMessage(`{"items":[{"model_code":"video-x","allowed":true,"is_online":true,"constraints":{"aspect_ratios":["16:9","9:16"],"resolutions":["hd","fhd"],"mode_types":["text_to_video"],"supported_duration_seconds":[4,8],"supports_audio_generation":false}}]}`)
	if err := ApplyModelConfiguration(data, "video", "video-x", raw); err != nil {
		t.Fatal(err)
	}
	params := data["params"].(map[string]any)
	settings := params["settings"].(map[string]any)
	if params["model"] != "video-x" || params["duration"] != float64(4) || settings["ratio"] != "16:9" || settings["resolution"] != "hd" || settings["generateAudio"] != false {
		t.Fatalf("params = %#v", params)
	}
}
