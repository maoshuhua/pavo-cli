package canvas

import (
	"encoding/json"
	"testing"
)

func TestReplaceTextPromptPreservesSkillAndMediaSegments(t *testing.T) {
	data := map[string]any{"params": map[string]any{"prompt": []any{
		map[string]any{"type": "skill", "code": "character_setting"},
		map[string]any{"type": "image", "url": "https://example.test/ref.png"},
		map[string]any{"type": "text", "content": "旧提示词"},
	}}}
	ReplaceTextPrompt(data, "新提示词")
	segments := PromptSegments(data)
	if len(segments) != 3 || segments[0]["code"] != "character_setting" || segments[1]["url"] != "https://example.test/ref.png" || segments[2]["content"] != "新提示词" {
		t.Fatalf("segments = %#v", segments)
	}
}

func TestAddSkillPromptDeduplicatesAndPrepends(t *testing.T) {
	data := map[string]any{}
	ReplaceTextPrompt(data, "主体描述")
	if err := AddSkillPrompt(data, "scene_setting"); err != nil {
		t.Fatal(err)
	}
	if err := AddSkillPrompt(data, "scene_setting"); err != nil {
		t.Fatal(err)
	}
	segments := PromptSegments(data)
	if len(segments) != 2 || segments[0]["type"] != "skill" || segments[0]["code"] != "scene_setting" {
		t.Fatalf("segments = %#v", segments)
	}
}

func TestParseToolSpecsNormalizesLiveShortcuts(t *testing.T) {
	raw := json.RawMessage(`{
		"version":"2026-08-25",
		"nodeConfigs":{
			"guide":{"video":[{"code":"guide_first_last_frames","name":"首尾帧","node_type":"video","extra":{"node_list":[
				{"node_key":"first_frame","action_type":"input","input_type":"image","node_type":"image","name":"首帧","data":{"url":"https://example.test/first.png"}},
				{"node_key":"self_node","node_type":"video","node_status":"edit","data":{"params":{"model":"video-x"}}},
				{"node_key":"output","action_type":"output","node_type":"video","name":"生成视频","data":{}}
			]}}]},
			"skill":{"image":[{"code":"character_setting","name":"角色设定","node_type":"image","extra":{}}]},
			"mode":{},
			"node":[{"code":"plain_image","node_type":"image"}]
		},
		"textModes":[{"modeCode":"text_common","modelCodes":["text-x"]}],
		"textModels":[{"modelCode":"text-x","isOnline":true}]
	}`)
	specs, err := ParseToolSpecs(raw)
	if err != nil {
		t.Fatal(err)
	}
	guide, err := specs.FindShortcut("guide_first_last_frames")
	if err != nil {
		t.Fatal(err)
	}
	if guide.Kind != "guide" || !guide.CreatesOutput || len(guide.RequiredInputs) != 1 || !guide.RequiredInputs[0].HasExample {
		t.Fatalf("guide = %#v", guide)
	}
	if model, err := specs.DefaultTextModel("text_common"); err != nil || model != "text-x" {
		t.Fatalf("DefaultTextModel = %q, %v", model, err)
	}
}

func TestBuildSkillShortcutPlanPreservesPresetAndSource(t *testing.T) {
	shortcut := Shortcut{Kind: "skill", Code: "character_setting", Name: "角色设定", NodeType: "image", ToolSpecVersion: "v1"}
	plan, err := BuildShortcutPlan(shortcut, ShortcutApplyOptions{Source: "u-source", Prompt: "固定角色特征"}, func(nodeType, configured string) (string, error) {
		return "image-model", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Operations) != 2 || plan.Operations[0].Op != "node.create" || plan.Operations[1].Source != "u-source" || plan.RunRef != "$shortcut_output" {
		t.Fatalf("plan = %#v", plan)
	}
	data, err := DecodeObject(plan.Operations[0].Data)
	if err != nil {
		t.Fatal(err)
	}
	segments := PromptSegments(data)
	if len(segments) != 2 || segments[0]["code"] != "character_setting" || segments[1]["content"] != "固定角色特征" {
		t.Fatalf("segments = %#v", segments)
	}
}

func TestBuildGuideShortcutPlanRequiresAndBindsInputs(t *testing.T) {
	shortcut := Shortcut{Kind: "guide", Code: "guide_first_last_frames", Name: "首尾帧", NodeType: "video", Config: ToolConfig{Extra: ToolConfigExtra{NodeList: []ToolConfigNode{
		{NodeKey: "first_frame", ActionType: "input", NodeType: "image", Name: "首帧"},
		{NodeKey: "self_node", NodeType: "video", NodeStatus: "edit", Data: ToolConfigNodeData{Params: map[string]any{}}},
		{NodeKey: "output", ActionType: "output", NodeType: "video", Name: "结果"},
	}}}}
	if _, err := BuildShortcutPlan(shortcut, ShortcutApplyOptions{}, nil); err == nil {
		t.Fatal("expected missing input error")
	}
	plan, err := BuildShortcutPlan(shortcut, ShortcutApplyOptions{Inputs: map[string]string{"first_frame": "i-ref"}}, func(nodeType, configured string) (string, error) {
		return "video-model", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Operations) != 5 || plan.Operations[2].Source != "i-ref" || plan.Operations[2].Target != "$shortcut_self" || plan.Operations[4].Op != "group.create" {
		t.Fatalf("operations = %#v", plan.Operations)
	}
}

func TestBuildGuideShortcutPlanUsesLiveConnectionList(t *testing.T) {
	shortcut := Shortcut{Kind: "guide", Code: "guide-live", Name: "实时模板", NodeType: "image", Config: ToolConfig{Extra: ToolConfigExtra{
		NodeList: []ToolConfigNode{
			{NodeKey: "material", ActionType: "input", NodeType: "image", Name: "素材"},
			{NodeKey: "self_node", NodeType: "text", Data: ToolConfigNodeData{Params: map[string]any{}}},
			{NodeKey: "result", ActionType: "output", NodeType: "image", Name: "结果"},
		},
		ConnectionList: []ToolConfigConnection{{SourceNodeKey: "material", TargetNodeKey: "result", Role: "reference"}, {Source: "self_node", Target: "result", Role: "prompt"}},
	}}}
	plan, err := BuildShortcutPlan(shortcut, ShortcutApplyOptions{Inputs: map[string]string{"material": "i-material"}}, func(nodeType, configured string) (string, error) {
		return map[string]string{"text": "text-x", "image": "image-x"}[nodeType], nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Operations) != 5 || plan.Operations[2].Source != "i-material" || plan.Operations[2].Target != "$shortcut_output_1" || plan.Operations[3].Role != "prompt" {
		t.Fatalf("operations = %#v", plan.Operations)
	}
}
