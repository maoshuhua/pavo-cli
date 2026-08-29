package canvas

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/maoshuhua/pavo-cli/internal/api"
)

func sampleStoryboard() *Storyboard {
	return &Storyboard{
		SchemaVersion: StoryboardSchemaVersion,
		Title:         "雨夜重逢",
		Brief:         "两位旧友在车站重逢",
		StyleBible:    StoryboardStyleBible{VisualStyle: "写实电影感", Palette: "冷蓝与暖黄", AspectRatio: "16:9", Continuity: "角色服装和车站空间保持一致", Negative: "过曝"},
		Characters:    []StoryboardCharacter{{ID: "char-01", Name: "林岚", Appearance: "黑色短发，棕色眼睛", Wardrobe: "米色风衣", Personality: "克制而敏感"}},
		Subjects:      []StoryboardSubject{{ID: "subject-01", Name: "长柄伞", Description: "深蓝色长柄伞，木质弯柄", Continuity: "颜色、长度和弯柄结构固定"}},
		Scenes:        []StoryboardScene{{ID: "scene-01", Name: "旧车站", Description: "潮湿站台与老式顶棚", Time: "雨夜", Lighting: "钨丝灯逆光"}},
		Shots: []StoryboardShot{
			{ID: "shot-001", Order: 1, DurationSeconds: 4, Plot: "建立雨夜车站", CharacterIDs: []string{"char-01"}, SubjectIDs: []string{"subject-01"}, SceneID: "scene-01", ShotSize: "全景", CameraAngle: "平视", CameraMotion: "缓慢推进", Composition: "人物位于右侧三分线", Action: "林岚撑伞走入站台", Emotion: "迟疑", Lighting: "冷蓝雨幕与暖黄灯光", Audio: "雨声", Transition: "直接切入近景", Negative: "人物身份漂移"},
			{ID: "shot-002", Order: 2, DurationSeconds: 3, Plot: "看到旧友", CharacterIDs: []string{"char-01"}, SubjectIDs: []string{"subject-01"}, SceneID: "scene-01", ShotSize: "近景", CameraAngle: "平视", CameraMotion: "轻微推近", Composition: "面部居中", Action: "林岚停步并抬眼", Emotion: "惊讶后释然", Lighting: "暖黄侧光", Dialogue: "好久不见", Audio: "雨声渐弱", Transition: "淡出", Negative: "面部变形"},
		},
	}
}

func TestStoryboardRoundTripAndPromptContract(t *testing.T) {
	if !json.Valid(StoryboardJSONSchema()) {
		t.Fatal("StoryboardJSONSchema is invalid JSON")
	}
	storyboard := sampleStoryboard()
	raw, err := json.Marshal(storyboard)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseStoryboard([]byte("```json\n" + string(raw) + "\n```"))
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.Shots) != 2 || parsed.Shots[1].ID != "shot-002" {
		t.Fatalf("parsed = %#v", parsed)
	}
	prompt := CompileStoryboardImagePrompt(parsed, parsed.Shots[0])
	for _, required := range []string{"【角色一致性】", "米色风衣", "固定主体/道具", "深蓝色长柄伞", "【构图与机位】", "【统一视觉风格】", "【负面约束】"} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("prompt missing %q: %s", required, prompt)
		}
	}
	generation := StoryboardGenerationPrompt(storyboard.Title, storyboard.Brief, 2)
	if !strings.Contains(generation, `shots 必须恰好 2 条`) || !strings.Contains(generation, StoryboardSchemaVersion) {
		t.Fatalf("generation prompt = %s", generation)
	}
}

func TestStoryboardProfilesLintAndOfflineCompile(t *testing.T) {
	generation := StoryboardGenerationPromptWithProfile("产品雨夜广告", "固定产品结构并完成主视觉收束", 3, "commercial")
	for _, required := range []string{"商业广告", "产品结构", "每镜只安排一个主要叙事动作"} {
		if !strings.Contains(generation, required) {
			t.Fatalf("generation prompt missing %q: %s", required, generation)
		}
	}
	template, err := NewStoryboardTemplate("commercial", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(template.Characters) != 0 || len(template.Subjects) != 1 || len(template.Shots[0].SubjectIDs) != 1 {
		t.Fatalf("commercial template = %#v", template)
	}
	raw, _ := json.Marshal(template)
	lint := LintStoryboard(raw)
	if !lint.Valid || lint.QualityReady || lint.Warnings == 0 || lint.ShotCount != 3 {
		t.Fatalf("template lint = %#v", lint)
	}
	storyboard := sampleStoryboard()
	compiled, err := CompileStoryboard(storyboard, "all")
	if err != nil {
		t.Fatal(err)
	}
	if compiled.ShotCount != 2 || len(compiled.Shots) != 2 || compiled.Shots[0].ImagePrompt == "" || compiled.Shots[0].VideoPrompt == "" {
		t.Fatalf("compiled = %#v", compiled)
	}
	bad := append([]byte(nil), raw...)
	bad = []byte(strings.Replace(string(bad), `"title":"请填写分镜标题"`, `"title":"标题","unknown":true`, 1))
	invalid := LintStoryboard(bad)
	if invalid.Valid || invalid.Errors != 1 || invalid.Issues[0].Code != "schema.parse_failed" {
		t.Fatalf("invalid lint = %#v", invalid)
	}
}

func TestBuildStoryboardGraphCreatesStableAssetsAndGroup(t *testing.T) {
	storyboard := sampleStoryboard()
	data := map[string]any{}
	if err := SetStoryboardNodeData(data, storyboard); err != nil {
		t.Fatal(err)
	}
	detail := &api.CanvasProjectDetail{}
	item, err := NewNode(detail, NewNodeOptions{NodeKey: "t-story", Type: "text", Name: "Storyboard", X: floatPointer(100), Y: floatPointer(80), Data: data})
	if err != nil {
		t.Fatal(err)
	}
	detail.NodeList = append(detail.NodeList, NodeFromWriteItem(*item))
	build, err := BuildStoryboardGraph(detail, detail.NodeList[0], storyboard, StoryboardBuildOptions{ImageModel: "image-x", VideoModel: "video-x", WithVideo: true}, func(nodeType, model string, data map[string]any) error {
		SetModel(data, model)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !build.Changed || len(build.Assets) != 2 || build.GroupKey == "" || len(build.Request.Nodes.Create) != 5 || len(build.Request.Connections.Create) != 2 {
		t.Fatalf("build = %#v", build)
	}
	if build.Assets[0].ImageNodeKey == "" || build.Assets[0].VideoNodeKey == "" {
		t.Fatalf("assets = %#v", build.Assets)
	}
	for _, created := range build.Request.Nodes.Create {
		detail.NodeList = append(detail.NodeList, NodeFromWriteItem(created))
	}
	for _, created := range build.Request.Connections.Create {
		detail.ConnectionList = append(detail.ConnectionList, api.CanvasConnection{ConnectionID: created.ConnectionID, SourceNodeKey: created.Source, TargetNodeKey: created.Target, Role: created.Role, MediaOrder: json.RawMessage(`0`)})
	}
	second, err := BuildStoryboardGraph(detail, detail.NodeList[0], storyboard, StoryboardBuildOptions{ImageModel: "image-x", VideoModel: "video-x", WithVideo: true}, func(nodeType, model string, data map[string]any) error {
		SetModel(data, model)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if second.Changed || !BatchRequestEmpty(second.Request) {
		t.Fatalf("second build should be idempotent: %#v", second)
	}
}

func floatPointer(value float64) *float64 { return &value }

func TestValidateCanvasNodeChecksLiveSkill(t *testing.T) {
	data := map[string]any{}
	ReplaceTextPrompt(data, "角色立绘")
	if err := AddSkillPrompt(data, "missing-skill"); err != nil {
		t.Fatal(err)
	}
	SetModel(data, "image-x")
	raw, _ := EncodeObject(data)
	node := api.CanvasNode{NodeKey: "i-1", NodeType: "image", Name: "角色", Data: json.RawMessage(raw)}
	specs := &ToolSpecs{NodeConfigs: ToolNodeConfigs{Skill: map[string][]ToolConfig{"image": {{Code: "character_setting"}}}}}
	validation := ValidateCanvasNode(node, specs, func(nodeType, model string) error { return nil })
	if validation.Valid || len(validation.Issues) == 0 || !strings.Contains(validation.Issues[0].Message, "missing-skill") {
		t.Fatalf("validation = %#v", validation)
	}
}

func TestFirstAvailableModelCodeUsesAllowedOnlineModel(t *testing.T) {
	raw := json.RawMessage(`{"items":[{"model_code":"offline","allowed":true,"is_online":false},{"model_code":"denied","allowed":false,"is_online":true},{"model_code":"image-x","allowed":true,"is_online":true}]}`)
	model, err := FirstAvailableModelCode(raw)
	if err != nil || model != "image-x" {
		t.Fatalf("FirstAvailableModelCode = %q, %v", model, err)
	}
}

func TestStoryboardRequestCannotFinalizeBeforeSuccessfulGeneration(t *testing.T) {
	data := map[string]any{"pavo_storyboard_request": map[string]any{"shots": 2}, "content": StoryboardGenerationPrompt("标题", "需求", 2)}
	raw, _ := EncodeObject(data)
	node := api.CanvasNode{NodeKey: "t-request", NodeType: "text", Data: json.RawMessage(raw)}
	if _, err := StoryboardFromNode(node); err == nil || !strings.Contains(err.Error(), "尚未成功") {
		t.Fatalf("error = %v", err)
	}
}
