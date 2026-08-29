package canvas

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/maoshuhua/pavo-cli/internal/api"
)

const StoryboardSchemaVersion = "pavo.storyboard/v1"

type StoryboardProfile struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Guidance    string `json:"guidance"`
}

var storyboardProfiles = []StoryboardProfile{
	{Code: "auto", Name: "自动匹配", Description: "根据 brief 判断题材，但仍强制完整镜头和连续性字段", Guidance: "从 brief 推断合适的媒介、节奏与镜头语言；不要混用互相冲突的画风。"},
	{Code: "cinematic", Name: "写实电影", Description: "面向剧情短片和连续人物表演", Guidance: "采用写实电影语言；保持角色五官、发型、服装、关键道具和空间轴线；镜头运动克制，每镜只承担一个主要叙事动作。"},
	{Code: "commercial", Name: "商业广告", Description: "面向产品、品牌和主视觉广告", Guidance: "产品结构、材质、颜色和标识必须稳定；先建立使用场景，再突出一个核心卖点，最后收束到清晰 hero shot；不要生成不可读文字或虚构品牌标识。"},
	{Code: "animation", Name: "风格动画", Description: "面向二维、三维或插画动画", Guidance: "先固定媒介、线条、材质、角色比例和色板；跨镜头保持造型语言一致；动作使用清晰关键姿势，避免同一序列突然写实化。"},
	{Code: "documentary", Name: "纪实叙事", Description: "面向人物纪录、旅行和真实事件表达", Guidance: "使用自然光和可信环境细节；运镜以稳定观察为主，避免过度戏剧化；不得在 brief 未提供时虚构事实、身份或事件。"},
}

func StoryboardProfiles() []StoryboardProfile {
	return append([]StoryboardProfile(nil), storyboardProfiles...)
}

func FindStoryboardProfile(code string) (*StoryboardProfile, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		code = "auto"
	}
	for index := range storyboardProfiles {
		if storyboardProfiles[index].Code == code {
			profile := storyboardProfiles[index]
			return &profile, nil
		}
	}
	values := make([]string, 0, len(storyboardProfiles))
	for _, profile := range storyboardProfiles {
		values = append(values, profile.Code)
	}
	return nil, fmt.Errorf("未知 storyboard profile %q；可用值：%s", code, strings.Join(values, ", "))
}

func NewStoryboardTemplate(profileCode string, shotCount int) (*Storyboard, error) {
	profile, err := FindStoryboardProfile(profileCode)
	if err != nil {
		return nil, err
	}
	if shotCount <= 0 || shotCount > 100 {
		return nil, errors.New("shot count 必须在 1 到 100 之间")
	}
	style := StoryboardStyleBible{VisualStyle: "请填写统一媒介、摄影与质感", Palette: "请填写主色、辅色和冷暖关系", AspectRatio: "16:9", Continuity: "固定角色身份、服装、关键道具和场景空间轴线", Negative: "文字、水印、身份漂移、服装漂移、结构变形、背景漂移"}
	switch profile.Code {
	case "cinematic":
		style.VisualStyle = "写实电影感，克制景深与自然材质"
	case "commercial":
		style.VisualStyle = "写实商业摄影，高质感产品广告"
		style.Continuity = "固定产品结构、材质、颜色、标识和使用场景空间"
		style.Negative = "文字、水印、虚构标识、产品结构变形、材质漂移、闪烁"
	case "animation":
		style.VisualStyle = "请填写二维、三维或插画媒介，并固定线条与材质"
		style.Continuity = "固定角色比例、造型语言、服装、色板和场景透视"
	case "documentary":
		style.VisualStyle = "自然光纪实影像，可信环境细节，克制运镜"
		style.Continuity = "固定真实人物身份、时间、地点和可核实环境信息"
	}
	storyboard := &Storyboard{
		SchemaVersion: StoryboardSchemaVersion,
		Title:         "请填写分镜标题",
		Brief:         "请填写叙事目标、受众、媒介、节奏与必须保持的元素",
		StyleBible:    style,
		Characters: []StoryboardCharacter{{
			ID: "char-01", Name: "请填写角色名", Appearance: "请填写固定五官、发型、体型和识别特征", Wardrobe: "请填写服装款式、材质与颜色", Personality: "请填写性格及表演边界", ReferenceNodeKeys: []string{},
		}},
		Scenes: []StoryboardScene{{
			ID: "scene-01", Name: "请填写场景名", Description: "请填写稳定空间结构、前中后景和关键陈设", Time: "请填写时间与天气", Lighting: "请填写固定主光方向与冷暖关系", ReferenceNodeKeys: []string{},
		}},
		Shots: []StoryboardShot{},
	}
	roles := []string{"建立人物与空间关系", "推进主要动作", "突出关键变化", "完成情绪反应", "收束到明确结束状态"}
	characterIDs := []string{"char-01"}
	subjectIDs := []string{}
	negative := "身份、服装、肢体、背景结构或运动错误"
	if profile.Code == "commercial" {
		storyboard.Characters = []StoryboardCharacter{}
		storyboard.Subjects = []StoryboardSubject{{ID: "subject-01", Name: "请填写产品或固定主体名", Description: "请填写固定结构、材质、颜色、比例和标识", Continuity: "请填写跨镜头不可改变的产品细节与使用状态", ReferenceNodeKeys: []string{}}}
		roles = []string{"建立产品与使用场景", "展示一个核心使用动作", "突出一个可见卖点", "呈现使用结果", "收束到清晰产品 hero shot"}
		characterIDs = []string{}
		subjectIDs = []string{"subject-01"}
		negative = "产品结构、材质、颜色、标识、比例、背景结构或运动错误"
	}
	for index := 0; index < shotCount; index++ {
		role := roles[index%len(roles)]
		storyboard.Shots = append(storyboard.Shots, StoryboardShot{
			ID: fmt.Sprintf("shot-%03d", index+1), Order: index + 1, DurationSeconds: 4, Plot: role,
			CharacterIDs: characterIDs, SubjectIDs: subjectIDs, SceneID: "scene-01", ShotSize: "请填写景别", CameraAngle: "请填写机位角度", CameraMotion: "请填写单一且克制的运镜", Composition: "请填写主体位置、视线和前中后景", Action: "请填写一个可观察的动作过程", Emotion: "请填写可表演的情绪变化", Lighting: "保持场景主光，并写明本镜变化", Dialogue: "", Audio: "请填写环境音、音效或音乐", Transition: "请填写本镜结束状态和转场", Negative: negative,
		})
	}
	return storyboard, nil
}

var storyboardJSONSchema = json.RawMessage(`{
  "$schema":"https://json-schema.org/draft/2020-12/schema",
  "$id":"https://pavo-ai.cn/schemas/storyboard-v1.json",
  "title":"PAVO Structured Storyboard",
  "type":"object",
  "additionalProperties":false,
  "required":["schema_version","title","brief","style_bible","characters","scenes","shots"],
  "properties":{
    "schema_version":{"const":"pavo.storyboard/v1"},
    "title":{"type":"string","minLength":1},
    "brief":{"type":"string"},
    "style_bible":{"type":"object","additionalProperties":false,"required":["visual_style","palette","aspect_ratio","continuity","negative"],"properties":{"visual_style":{"type":"string","minLength":1},"palette":{"type":"string","minLength":1},"aspect_ratio":{"type":"string","minLength":1},"continuity":{"type":"string","minLength":1},"negative":{"type":"string","minLength":1}}},
    "characters":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["id","name","appearance","wardrobe","personality","reference_node_keys"],"properties":{"id":{"type":"string","minLength":1},"name":{"type":"string","minLength":1},"appearance":{"type":"string","minLength":1},"wardrobe":{"type":"string","minLength":1},"personality":{"type":"string","minLength":1},"reference_node_keys":{"type":"array","items":{"type":"string","minLength":1}}}}},
    "subjects":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["id","name","description","continuity","reference_node_keys"],"properties":{"id":{"type":"string","minLength":1},"name":{"type":"string","minLength":1},"description":{"type":"string","minLength":1},"continuity":{"type":"string","minLength":1},"reference_node_keys":{"type":"array","items":{"type":"string","minLength":1}}}}},
    "scenes":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["id","name","description","time","lighting","reference_node_keys"],"properties":{"id":{"type":"string","minLength":1},"name":{"type":"string","minLength":1},"description":{"type":"string","minLength":1},"time":{"type":"string","minLength":1},"lighting":{"type":"string","minLength":1},"reference_node_keys":{"type":"array","items":{"type":"string","minLength":1}}}}},
    "shots":{"type":"array","minItems":1,"items":{"type":"object","additionalProperties":false,"required":["id","order","duration_seconds","plot","character_ids","scene_id","shot_size","camera_angle","camera_motion","composition","action","emotion","lighting","dialogue","audio","transition","negative"],"properties":{"id":{"type":"string","minLength":1},"order":{"type":"integer","minimum":1},"duration_seconds":{"type":"number","minimum":1,"maximum":30},"plot":{"type":"string","minLength":1},"character_ids":{"type":"array","items":{"type":"string","minLength":1}},"subject_ids":{"type":"array","items":{"type":"string","minLength":1}},"scene_id":{"type":"string"},"shot_size":{"type":"string","minLength":1},"camera_angle":{"type":"string","minLength":1},"camera_motion":{"type":"string","minLength":1},"composition":{"type":"string","minLength":1},"action":{"type":"string","minLength":1},"emotion":{"type":"string","minLength":1},"lighting":{"type":"string","minLength":1},"dialogue":{"type":"string"},"audio":{"type":"string","minLength":1},"transition":{"type":"string","minLength":1},"negative":{"type":"string","minLength":1}}}}
  }
}`)

func StoryboardJSONSchema() json.RawMessage {
	return append(json.RawMessage(nil), storyboardJSONSchema...)
}

type Storyboard struct {
	SchemaVersion string                `json:"schema_version"`
	Title         string                `json:"title"`
	Brief         string                `json:"brief"`
	StyleBible    StoryboardStyleBible  `json:"style_bible"`
	Characters    []StoryboardCharacter `json:"characters"`
	Subjects      []StoryboardSubject   `json:"subjects,omitempty"`
	Scenes        []StoryboardScene     `json:"scenes"`
	Shots         []StoryboardShot      `json:"shots"`
}

type StoryboardStyleBible struct {
	VisualStyle string `json:"visual_style"`
	Palette     string `json:"palette"`
	AspectRatio string `json:"aspect_ratio"`
	Continuity  string `json:"continuity"`
	Negative    string `json:"negative"`
}

type StoryboardCharacter struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	Appearance        string   `json:"appearance"`
	Wardrobe          string   `json:"wardrobe"`
	Personality       string   `json:"personality"`
	ReferenceNodeKeys []string `json:"reference_node_keys"`
}

type StoryboardScene struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	Description       string   `json:"description"`
	Time              string   `json:"time"`
	Lighting          string   `json:"lighting"`
	ReferenceNodeKeys []string `json:"reference_node_keys"`
}

type StoryboardSubject struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	Description       string   `json:"description"`
	Continuity        string   `json:"continuity"`
	ReferenceNodeKeys []string `json:"reference_node_keys"`
}

type StoryboardShot struct {
	ID              string   `json:"id"`
	Order           int      `json:"order"`
	DurationSeconds float64  `json:"duration_seconds"`
	Plot            string   `json:"plot"`
	CharacterIDs    []string `json:"character_ids"`
	SubjectIDs      []string `json:"subject_ids,omitempty"`
	SceneID         string   `json:"scene_id"`
	ShotSize        string   `json:"shot_size"`
	CameraAngle     string   `json:"camera_angle"`
	CameraMotion    string   `json:"camera_motion"`
	Composition     string   `json:"composition"`
	Action          string   `json:"action"`
	Emotion         string   `json:"emotion"`
	Lighting        string   `json:"lighting"`
	Dialogue        string   `json:"dialogue"`
	Audio           string   `json:"audio"`
	Transition      string   `json:"transition"`
	Negative        string   `json:"negative"`
}

type StoryboardIssue struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

type StoryboardLintIssue struct {
	Severity   string `json:"severity"`
	Code       string `json:"code"`
	Path       string `json:"path"`
	Message    string `json:"message"`
	Suggestion string `json:"suggestion,omitempty"`
}

type StoryboardLintResult struct {
	SchemaVersion        string                `json:"schema_version"`
	Valid                bool                  `json:"valid"`
	QualityReady         bool                  `json:"quality_ready"`
	ShotCount            int                   `json:"shot_count"`
	TotalDurationSeconds float64               `json:"total_duration_seconds"`
	Errors               int                   `json:"errors"`
	Warnings             int                   `json:"warnings"`
	Advisories           int                   `json:"advisories"`
	Issues               []StoryboardLintIssue `json:"issues"`
	Storyboard           *Storyboard           `json:"-"`
}

type CompiledStoryboardShot struct {
	ShotID            string   `json:"shot_id"`
	Order             int      `json:"order"`
	DurationSeconds   float64  `json:"duration_seconds"`
	ReferenceNodeKeys []string `json:"reference_node_keys"`
	ImagePrompt       string   `json:"image_prompt,omitempty"`
	VideoPrompt       string   `json:"video_prompt,omitempty"`
}

type CompiledStoryboard struct {
	SchemaVersion        string                   `json:"schema_version"`
	Title                string                   `json:"title"`
	ShotCount            int                      `json:"shot_count"`
	TotalDurationSeconds float64                  `json:"total_duration_seconds"`
	Kind                 string                   `json:"kind"`
	QualityReady         bool                     `json:"quality_ready"`
	Warnings             []StoryboardLintIssue    `json:"warnings"`
	Advisories           []StoryboardLintIssue    `json:"advisories"`
	Shots                []CompiledStoryboardShot `json:"shots"`
}

func extractJSONObject(raw []byte) []byte {
	trimmed := bytes.TrimSpace(raw)
	if bytes.HasPrefix(trimmed, []byte("```")) {
		if newline := bytes.IndexByte(trimmed, '\n'); newline >= 0 {
			trimmed = trimmed[newline+1:]
		}
		trimmed = bytes.TrimSuffix(bytes.TrimSpace(trimmed), []byte("```"))
	}
	start, end := bytes.IndexByte(trimmed, '{'), bytes.LastIndexByte(trimmed, '}')
	if start >= 0 && end > start {
		return trimmed[start : end+1]
	}
	return trimmed
}

func decodeStoryboard(raw []byte) (*Storyboard, error) {
	raw = extractJSONObject(raw)
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil, fmt.Errorf("解析 storyboard JSON 失败: %w", err)
	}
	if wrapped, exists := object["storyboard"]; exists && len(bytes.TrimSpace(wrapped)) > 0 {
		raw = wrapped
	}
	var storyboard Storyboard
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&storyboard); err != nil {
		return nil, fmt.Errorf("解析 storyboard Schema 失败: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, errors.New("解析 storyboard Schema 失败: JSON object 后包含多余内容")
	}
	storyboard.Normalize()
	return &storyboard, nil
}

func ParseStoryboard(raw []byte) (*Storyboard, error) {
	storyboard, err := decodeStoryboard(raw)
	if err != nil {
		return nil, err
	}
	if issues := storyboard.Validate(); len(issues) > 0 {
		return nil, storyboardIssuesError(issues)
	}
	return storyboard, nil
}

func (storyboard *Storyboard) Normalize() {
	if storyboard == nil {
		return
	}
	if strings.TrimSpace(storyboard.SchemaVersion) == "" {
		storyboard.SchemaVersion = StoryboardSchemaVersion
	}
	if strings.TrimSpace(storyboard.StyleBible.AspectRatio) == "" {
		storyboard.StyleBible.AspectRatio = "16:9"
	}
	if storyboard.Characters == nil {
		storyboard.Characters = []StoryboardCharacter{}
	}
	if storyboard.Subjects == nil {
		storyboard.Subjects = []StoryboardSubject{}
	}
	if storyboard.Scenes == nil {
		storyboard.Scenes = []StoryboardScene{}
	}
	if storyboard.Shots == nil {
		storyboard.Shots = []StoryboardShot{}
	}
	for index := range storyboard.Characters {
		if storyboard.Characters[index].ReferenceNodeKeys == nil {
			storyboard.Characters[index].ReferenceNodeKeys = []string{}
		}
	}
	for index := range storyboard.Subjects {
		if storyboard.Subjects[index].ReferenceNodeKeys == nil {
			storyboard.Subjects[index].ReferenceNodeKeys = []string{}
		}
	}
	for index := range storyboard.Scenes {
		if storyboard.Scenes[index].ReferenceNodeKeys == nil {
			storyboard.Scenes[index].ReferenceNodeKeys = []string{}
		}
	}
	for index := range storyboard.Shots {
		if storyboard.Shots[index].CharacterIDs == nil {
			storyboard.Shots[index].CharacterIDs = []string{}
		}
		if storyboard.Shots[index].SubjectIDs == nil {
			storyboard.Shots[index].SubjectIDs = []string{}
		}
		if storyboard.Shots[index].Order <= 0 {
			storyboard.Shots[index].Order = index + 1
		}
		if strings.TrimSpace(storyboard.Shots[index].ID) == "" {
			storyboard.Shots[index].ID = fmt.Sprintf("shot-%03d", storyboard.Shots[index].Order)
		}
	}
	sort.SliceStable(storyboard.Shots, func(i, j int) bool { return storyboard.Shots[i].Order < storyboard.Shots[j].Order })
}

func (storyboard *Storyboard) Validate() []StoryboardIssue {
	issues := []StoryboardIssue{}
	add := func(path, message string) { issues = append(issues, StoryboardIssue{Path: path, Message: message}) }
	if storyboard == nil {
		return []StoryboardIssue{{Path: "$", Message: "storyboard 不能为空"}}
	}
	if storyboard.SchemaVersion != StoryboardSchemaVersion {
		add("schema_version", fmt.Sprintf("必须是 %s", StoryboardSchemaVersion))
	}
	if strings.TrimSpace(storyboard.Title) == "" {
		add("title", "不能为空")
	}
	if strings.TrimSpace(storyboard.StyleBible.VisualStyle) == "" || strings.TrimSpace(storyboard.StyleBible.Palette) == "" || strings.TrimSpace(storyboard.StyleBible.AspectRatio) == "" || strings.TrimSpace(storyboard.StyleBible.Continuity) == "" || strings.TrimSpace(storyboard.StyleBible.Negative) == "" {
		add("style_bible", "visual_style、palette、aspect_ratio、continuity 和 negative 不能为空")
	}
	if len(storyboard.Shots) == 0 {
		add("shots", "至少需要一个 shot")
	}
	characters := map[string]bool{}
	for index, character := range storyboard.Characters {
		path := fmt.Sprintf("characters[%d]", index)
		if strings.TrimSpace(character.ID) == "" {
			add(path+".id", "不能为空")
		} else if characters[character.ID] {
			add(path+".id", "重复")
		} else {
			characters[character.ID] = true
		}
		if strings.TrimSpace(character.Name) == "" || strings.TrimSpace(character.Appearance) == "" || strings.TrimSpace(character.Wardrobe) == "" || strings.TrimSpace(character.Personality) == "" {
			add(path, "name、appearance、wardrobe 和 personality 不能为空")
		}
	}
	subjects := map[string]bool{}
	for index, subject := range storyboard.Subjects {
		path := fmt.Sprintf("subjects[%d]", index)
		if strings.TrimSpace(subject.ID) == "" {
			add(path+".id", "不能为空")
		} else if subjects[subject.ID] {
			add(path+".id", "重复")
		} else {
			subjects[subject.ID] = true
		}
		if strings.TrimSpace(subject.Name) == "" || strings.TrimSpace(subject.Description) == "" || strings.TrimSpace(subject.Continuity) == "" {
			add(path, "name、description 和 continuity 不能为空")
		}
	}
	scenes := map[string]bool{}
	for index, scene := range storyboard.Scenes {
		path := fmt.Sprintf("scenes[%d]", index)
		if strings.TrimSpace(scene.ID) == "" {
			add(path+".id", "不能为空")
		} else if scenes[scene.ID] {
			add(path+".id", "重复")
		} else {
			scenes[scene.ID] = true
		}
		if strings.TrimSpace(scene.Name) == "" || strings.TrimSpace(scene.Description) == "" || strings.TrimSpace(scene.Time) == "" || strings.TrimSpace(scene.Lighting) == "" {
			add(path, "name、description、time 和 lighting 不能为空")
		}
	}
	shotIDs, orders := map[string]bool{}, map[int]bool{}
	for index, shot := range storyboard.Shots {
		path := fmt.Sprintf("shots[%d]", index)
		if strings.TrimSpace(shot.ID) == "" {
			add(path+".id", "不能为空")
		} else if shotIDs[shot.ID] {
			add(path+".id", "重复")
		} else {
			shotIDs[shot.ID] = true
		}
		if shot.Order <= 0 || orders[shot.Order] {
			add(path+".order", "必须是唯一正整数")
		}
		orders[shot.Order] = true
		if shot.DurationSeconds < 1 || shot.DurationSeconds > 30 {
			add(path+".duration_seconds", "必须在 1 到 30 秒之间")
		}
		if strings.TrimSpace(shot.Plot) == "" || strings.TrimSpace(shot.ShotSize) == "" || strings.TrimSpace(shot.Action) == "" {
			add(path, "plot、shot_size 和 action 不能为空")
		}
		if strings.TrimSpace(shot.CameraAngle) == "" || strings.TrimSpace(shot.CameraMotion) == "" || strings.TrimSpace(shot.Composition) == "" || strings.TrimSpace(shot.Emotion) == "" || strings.TrimSpace(shot.Lighting) == "" || strings.TrimSpace(shot.Audio) == "" || strings.TrimSpace(shot.Transition) == "" || strings.TrimSpace(shot.Negative) == "" {
			add(path, "camera_angle、camera_motion、composition、emotion、lighting、audio、transition 和 negative 不能为空")
		}
		if shot.SceneID != "" && !scenes[shot.SceneID] {
			add(path+".scene_id", fmt.Sprintf("引用了不存在的 scene %q", shot.SceneID))
		}
		for charIndex, characterID := range shot.CharacterIDs {
			if !characters[characterID] {
				add(fmt.Sprintf("%s.character_ids[%d]", path, charIndex), fmt.Sprintf("引用了不存在的 character %q", characterID))
			}
		}
		for subjectIndex, subjectID := range shot.SubjectIDs {
			if !subjects[subjectID] {
				add(fmt.Sprintf("%s.subject_ids[%d]", path, subjectIndex), fmt.Sprintf("引用了不存在的 subject %q", subjectID))
			}
		}
	}
	for index, shot := range storyboard.Shots {
		if shot.Order != index+1 {
			add(fmt.Sprintf("shots[%d].order", index), "必须从 1 开始连续递增")
		}
	}
	return issues
}

func storyboardIssuesError(issues []StoryboardIssue) error {
	parts := make([]string, 0, len(issues))
	for _, issue := range issues {
		parts = append(parts, issue.Path+": "+issue.Message)
	}
	return errors.New("storyboard Schema 校验失败: " + strings.Join(parts, "; "))
}

func storyboardIssueCode(issue StoryboardIssue) string {
	switch {
	case strings.Contains(issue.Message, "不存在"):
		return "schema.reference_missing"
	case strings.Contains(issue.Message, "重复") || strings.Contains(issue.Message, "唯一"):
		return "schema.duplicate"
	case strings.Contains(issue.Path, "duration_seconds"):
		return "schema.duration_invalid"
	case strings.Contains(issue.Path, ".order"):
		return "schema.order_invalid"
	case strings.Contains(issue.Message, "不能为空") || strings.Contains(issue.Message, "至少"):
		return "schema.required"
	default:
		return "schema.invalid"
	}
}

func LintStoryboard(raw []byte) StoryboardLintResult {
	result := StoryboardLintResult{SchemaVersion: StoryboardSchemaVersion, Issues: []StoryboardLintIssue{}}
	storyboard, err := decodeStoryboard(raw)
	if err != nil {
		result.Errors = 1
		result.Issues = append(result.Issues, StoryboardLintIssue{Severity: "error", Code: "schema.parse_failed", Path: "$", Message: err.Error(), Suggestion: "先运行 `pavo canvas storyboard schema` 对照字段，并移除未知字段或 JSON 前后缀"})
		return result
	}
	result.Storyboard = storyboard
	result.ShotCount = len(storyboard.Shots)
	for _, shot := range storyboard.Shots {
		result.TotalDurationSeconds += shot.DurationSeconds
	}
	for _, issue := range storyboard.Validate() {
		result.Errors++
		result.Issues = append(result.Issues, StoryboardLintIssue{Severity: "error", Code: storyboardIssueCode(issue), Path: issue.Path, Message: issue.Message})
	}
	addWarning := func(code, path, message, suggestion string) {
		result.Warnings++
		result.Issues = append(result.Issues, StoryboardLintIssue{Severity: "warning", Code: code, Path: path, Message: message, Suggestion: suggestion})
	}
	addAdvisory := func(code, path, message, suggestion string) {
		result.Advisories++
		result.Issues = append(result.Issues, StoryboardLintIssue{Severity: "advisory", Code: code, Path: path, Message: message, Suggestion: suggestion})
	}
	placeholderTerms := []string{"todo", "tbd", "待补充", "待填写", "请填写", "同上", "同前", "延续前镜", "same as above"}
	checkQualityText := func(path, value string, minRunes int, suggestion string) {
		trimmed := strings.TrimSpace(value)
		lower := strings.ToLower(trimmed)
		for _, term := range placeholderTerms {
			if strings.Contains(lower, term) {
				addWarning("quality.placeholder", path, fmt.Sprintf("包含省略或占位表达 %q", term), suggestion)
				break
			}
		}
		if trimmed != "" && utf8.RuneCountInString(trimmed) < minRunes {
			addWarning("quality.description_too_short", path, "描述过短，模型难以稳定复现", suggestion)
		}
	}
	checkQualityText("brief", storyboard.Brief, 8, "写清叙事目标、受众、媒介、节奏和必须保持的元素")
	checkQualityText("style_bible.visual_style", storyboard.StyleBible.VisualStyle, 4, "指定媒介、质感和摄影或造型语言")
	checkQualityText("style_bible.palette", storyboard.StyleBible.Palette, 4, "给出主色、辅色及冷暖关系")
	checkQualityText("style_bible.continuity", storyboard.StyleBible.Continuity, 8, "列出跨镜头不可变化的身份、服装、道具和空间锚点")
	checkQualityText("style_bible.negative", storyboard.StyleBible.Negative, 6, "写明身份、结构、文字、闪烁等全局负面约束")
	characterUsage := map[string]int{}
	for _, shot := range storyboard.Shots {
		for _, id := range shot.CharacterIDs {
			characterUsage[id]++
		}
	}
	for index, character := range storyboard.Characters {
		base := fmt.Sprintf("characters[%d]", index)
		checkQualityText(base+".appearance", character.Appearance, 6, "写出五官、发型、体型和稳定识别特征")
		checkQualityText(base+".wardrobe", character.Wardrobe, 4, "写出服装款式、材质和颜色")
		if characterUsage[character.ID] > 1 && len(uniqueNonEmptyStrings(character.ReferenceNodeKeys)) == 0 {
			addAdvisory("continuity.character_reference_missing", base+".reference_node_keys", "角色跨多个镜头出现，但没有画布参考节点", "需要更强身份一致性时，先用 upload 或角色设定 shortcut 创建参考资产，再填入真实 node_key")
		}
	}
	subjectUsage := map[string]int{}
	for _, shot := range storyboard.Shots {
		for _, id := range shot.SubjectIDs {
			subjectUsage[id]++
		}
	}
	for index, subject := range storyboard.Subjects {
		base := fmt.Sprintf("subjects[%d]", index)
		checkQualityText(base+".description", subject.Description, 8, "写出结构、材质、颜色、比例和稳定标识")
		checkQualityText(base+".continuity", subject.Continuity, 6, "列出跨镜头不可改变的主体细节和使用状态")
		if subjectUsage[subject.ID] > 1 && len(uniqueNonEmptyStrings(subject.ReferenceNodeKeys)) == 0 {
			addAdvisory("continuity.subject_reference_missing", base+".reference_node_keys", "固定主体或道具跨多个镜头出现，但没有画布参考节点", "需要更强结构一致性时，先 upload 真实参考资产，再填入 node_key")
		}
	}
	sceneUsage := map[string]int{}
	for _, shot := range storyboard.Shots {
		if shot.SceneID != "" {
			sceneUsage[shot.SceneID]++
		}
	}
	for index, scene := range storyboard.Scenes {
		base := fmt.Sprintf("scenes[%d]", index)
		checkQualityText(base+".description", scene.Description, 8, "写出稳定空间结构、前中后景和关键陈设")
		if sceneUsage[scene.ID] > 1 && len(uniqueNonEmptyStrings(scene.ReferenceNodeKeys)) == 0 {
			addAdvisory("continuity.scene_reference_missing", base+".reference_node_keys", "场景跨多个镜头出现，但没有画布参考节点", "需要更强场景一致性时，先用 upload 或场景设定 shortcut 创建参考资产，再填入真实 node_key")
		}
	}
	for index, shot := range storyboard.Shots {
		base := fmt.Sprintf("shots[%d]", index)
		checkQualityText(base+".plot", shot.Plot, 4, "说明该镜头唯一的叙事作用")
		checkQualityText(base+".composition", shot.Composition, 4, "说明主体位置、视线方向和前中后景")
		checkQualityText(base+".action", shot.Action, 4, "写成可观察的动作过程，避免抽象情绪词")
		checkQualityText(base+".lighting", shot.Lighting, 4, "说明主光方向、冷暖和与场景光线的关系")
		checkQualityText(base+".negative", shot.Negative, 4, "补充本镜容易发生的身份、肢体、结构或运动错误")
	}
	result.Valid = result.Errors == 0
	result.QualityReady = result.Valid && result.Warnings == 0
	return result
}

func CompileStoryboard(storyboard *Storyboard, kind string) (*CompiledStoryboard, error) {
	if storyboard == nil {
		return nil, errors.New("storyboard 不能为空")
	}
	kind = strings.ToLower(strings.TrimSpace(kind))
	if kind == "" {
		kind = "all"
	}
	if kind != "all" && kind != "image" && kind != "video" {
		return nil, fmt.Errorf("--kind 必须是 all、image 或 video，当前为 %q", kind)
	}
	raw, err := json.Marshal(storyboard)
	if err != nil {
		return nil, err
	}
	lint := LintStoryboard(raw)
	if !lint.Valid {
		issues := storyboard.Validate()
		return nil, storyboardIssuesError(issues)
	}
	compiled := &CompiledStoryboard{SchemaVersion: StoryboardSchemaVersion, Title: storyboard.Title, ShotCount: len(storyboard.Shots), TotalDurationSeconds: lint.TotalDurationSeconds, Kind: kind, QualityReady: lint.QualityReady, Warnings: []StoryboardLintIssue{}, Advisories: []StoryboardLintIssue{}, Shots: []CompiledStoryboardShot{}}
	for _, issue := range lint.Issues {
		if issue.Severity == "warning" {
			compiled.Warnings = append(compiled.Warnings, issue)
		} else if issue.Severity == "advisory" {
			compiled.Advisories = append(compiled.Advisories, issue)
		}
	}
	for _, shot := range storyboard.Shots {
		_, _, references := shotContext(storyboard, shot)
		item := CompiledStoryboardShot{ShotID: shot.ID, Order: shot.Order, DurationSeconds: shot.DurationSeconds, ReferenceNodeKeys: references}
		if kind == "all" || kind == "image" {
			item.ImagePrompt = CompileStoryboardImagePrompt(storyboard, shot)
		}
		if kind == "all" || kind == "video" {
			item.VideoPrompt = CompileStoryboardVideoPrompt(storyboard, shot)
		}
		compiled.Shots = append(compiled.Shots, item)
	}
	return compiled, nil
}

func StoryboardGenerationPrompt(title, brief string, shotCount int) string {
	return StoryboardGenerationPromptWithProfile(title, brief, shotCount, "auto")
}

func StoryboardGenerationPromptWithProfile(title, brief string, shotCount int, profileCode string) string {
	if shotCount <= 0 {
		shotCount = 8
	}
	profile, err := FindStoryboardProfile(profileCode)
	if err != nil {
		profile, _ = FindStoryboardProfile("auto")
	}
	return fmt.Sprintf(`你是专业影视分镜导演。根据创作需求生成严格结构化分镜。

硬性要求：
1. 只输出一个 JSON object，不要 Markdown 代码块、解释或前后缀。
2. schema_version 必须是 %q；shots 必须恰好 %d 条。
3. character/scene/shot 的 id 稳定、唯一；shot.order 从 1 连续递增。
4. 每条 shot 都必须写明 plot、duration_seconds、shot_size、camera_angle、camera_motion、composition、action、emotion、lighting、dialogue、audio、transition、negative。
5. 角色外观、服装、固定主体/产品/道具、场景空间、色彩和光线必须跨镜头连续；不要用“同上”“延续前镜”等省略表达。
6. duration_seconds 在 1 到 30 之间。
7. 每镜只安排一个主要叙事动作；动作、构图和运镜必须具体可见，禁止“很有感觉”“同上”“延续前镜”等省略表达。
8. negative 必须针对本镜可能出现的身份、肢体、产品结构、背景结构或时序错误，不能只写“低质量”。
9. 只有人物放入 characters；产品、道具、车辆和其他固定非人物主体放入 subjects，并在每镜用 subject_ids 引用；没有对应内容时使用空数组。

创作 Profile：%s（%s）
Profile 约束：%s

JSON Schema 形状：
{"schema_version":"pavo.storyboard/v1","title":"标题","brief":"需求","style_bible":{"visual_style":"视觉风格","palette":"色板","aspect_ratio":"16:9","continuity":"连续性规则","negative":"全局负面约束"},"characters":[{"id":"char-01","name":"角色名","appearance":"完整固定外观","wardrobe":"固定服装","personality":"性格","reference_node_keys":[]}],"subjects":[{"id":"subject-01","name":"产品/道具名","description":"固定结构、材质、颜色和标识","continuity":"跨镜头不可改变的细节","reference_node_keys":[]}],"scenes":[{"id":"scene-01","name":"场景名","description":"完整空间描述","time":"时间","lighting":"固定光线","reference_node_keys":[]}],"shots":[{"id":"shot-001","order":1,"duration_seconds":4,"plot":"剧情作用","character_ids":["char-01"],"subject_ids":["subject-01"],"scene_id":"scene-01","shot_size":"中景","camera_angle":"平视","camera_motion":"缓慢推进","composition":"主体和空间构图","action":"明确动作过程","emotion":"情绪和表情","lighting":"本镜光线","dialogue":"台词，无则空字符串","audio":"环境音/音效/音乐","transition":"转场","negative":"本镜负面约束"}]}

项目标题：%s
创作需求：%s`, StoryboardSchemaVersion, shotCount, profile.Name, profile.Code, profile.Guidance, strings.TrimSpace(title), strings.TrimSpace(brief))
}

func RenderStoryboardMarkdown(storyboard *Storyboard) string {
	if storyboard == nil {
		return ""
	}
	var result strings.Builder
	fmt.Fprintf(&result, "# %s\n\n", storyboard.Title)
	if storyboard.Brief != "" {
		fmt.Fprintf(&result, "%s\n\n", storyboard.Brief)
	}
	fmt.Fprintf(&result, "**视觉规范：** %s；色板：%s；画幅：%s；连续性：%s\n\n", storyboard.StyleBible.VisualStyle, storyboard.StyleBible.Palette, storyboard.StyleBible.AspectRatio, storyboard.StyleBible.Continuity)
	if len(storyboard.Characters) > 0 {
		result.WriteString("## 角色设定\n\n")
		for _, character := range storyboard.Characters {
			fmt.Fprintf(&result, "- `%s` %s：%s；服装：%s；性格：%s\n", character.ID, character.Name, character.Appearance, character.Wardrobe, character.Personality)
		}
		result.WriteString("\n")
	}
	if len(storyboard.Subjects) > 0 {
		result.WriteString("## 固定主体与道具\n\n")
		for _, subject := range storyboard.Subjects {
			fmt.Fprintf(&result, "- `%s` %s：%s；连续性：%s\n", subject.ID, subject.Name, subject.Description, subject.Continuity)
		}
		result.WriteString("\n")
	}
	if len(storyboard.Scenes) > 0 {
		result.WriteString("## 场景设定\n\n")
		for _, scene := range storyboard.Scenes {
			fmt.Fprintf(&result, "- `%s` %s：%s；时间：%s；光线：%s\n", scene.ID, scene.Name, scene.Description, scene.Time, scene.Lighting)
		}
		result.WriteString("\n")
	}
	for _, shot := range storyboard.Shots {
		fmt.Fprintf(&result, "## %02d · %s（%.1fs）\n\n- 剧情：%s\n- 角色：%s\n- 固定主体：%s\n- 场景：%s\n- 镜头：%s / %s / %s\n- 构图：%s\n- 动作：%s\n- 情绪：%s\n- 光线：%s\n- 台词：%s\n- 声音：%s\n\n", shot.Order, shot.ID, shot.DurationSeconds, shot.Plot, strings.Join(shot.CharacterIDs, ", "), strings.Join(shot.SubjectIDs, ", "), shot.SceneID, shot.ShotSize, shot.CameraAngle, shot.CameraMotion, shot.Composition, shot.Action, shot.Emotion, shot.Lighting, shot.Dialogue, shot.Audio)
	}
	return strings.TrimSpace(result.String())
}

func findStoryboardCharacter(storyboard *Storyboard, id string) *StoryboardCharacter {
	for index := range storyboard.Characters {
		if storyboard.Characters[index].ID == id {
			return &storyboard.Characters[index]
		}
	}
	return nil
}

func findStoryboardScene(storyboard *Storyboard, id string) *StoryboardScene {
	for index := range storyboard.Scenes {
		if storyboard.Scenes[index].ID == id {
			return &storyboard.Scenes[index]
		}
	}
	return nil
}

func findStoryboardSubject(storyboard *Storyboard, id string) *StoryboardSubject {
	for index := range storyboard.Subjects {
		if storyboard.Subjects[index].ID == id {
			return &storyboard.Subjects[index]
		}
	}
	return nil
}

func shotContext(storyboard *Storyboard, shot StoryboardShot) (string, string, []string) {
	continuity := []string{}
	references := []string{}
	for _, id := range shot.CharacterIDs {
		if character := findStoryboardCharacter(storyboard, id); character != nil {
			continuity = append(continuity, fmt.Sprintf("角色 %s（%s；服装：%s）", character.Name, character.Appearance, character.Wardrobe))
			references = append(references, character.ReferenceNodeKeys...)
		}
	}
	for _, id := range shot.SubjectIDs {
		if subject := findStoryboardSubject(storyboard, id); subject != nil {
			continuity = append(continuity, fmt.Sprintf("固定主体/道具 %s（%s；连续性：%s）", subject.Name, subject.Description, subject.Continuity))
			references = append(references, subject.ReferenceNodeKeys...)
		}
	}
	sceneText := ""
	if scene := findStoryboardScene(storyboard, shot.SceneID); scene != nil {
		sceneText = fmt.Sprintf("%s（%s；时间：%s；场景光线：%s）", scene.Name, scene.Description, scene.Time, scene.Lighting)
		references = append(references, scene.ReferenceNodeKeys...)
	}
	return strings.Join(continuity, "；"), sceneText, uniqueNonEmptyStrings(references)
}

func CompileStoryboardImagePrompt(storyboard *Storyboard, shot StoryboardShot) string {
	characters, scene, _ := shotContext(storyboard, shot)
	return strings.Join([]string{
		"【分镜目标】" + shot.Plot,
		"【角色一致性】" + characters,
		"【场景一致性】" + scene,
		"【构图与机位】" + shot.ShotSize + "，" + shot.CameraAngle + "，" + shot.Composition,
		"【动作与表情】" + shot.Action + "；" + shot.Emotion,
		"【光线与色彩】" + shot.Lighting + "；" + storyboard.StyleBible.Palette,
		"【统一视觉风格】" + storyboard.StyleBible.VisualStyle + "；画幅 " + storyboard.StyleBible.AspectRatio + "；" + storyboard.StyleBible.Continuity,
		"【负面约束】" + strings.Join(uniqueNonEmptyStrings([]string{storyboard.StyleBible.Negative, shot.Negative, "文字、水印、logo、肢体错误、人物身份漂移、服装漂移、场景结构漂移"}), "；"),
	}, "\n")
}

func CompileStoryboardVideoPrompt(storyboard *Storyboard, shot StoryboardShot) string {
	characters, scene, _ := shotContext(storyboard, shot)
	return strings.Join([]string{
		fmt.Sprintf("【时长】%.1f 秒", shot.DurationSeconds),
		"【叙事目标】" + shot.Plot,
		"【角色与场景锁定】" + characters + "；" + scene,
		"【起始画面】" + shot.ShotSize + "，" + shot.CameraAngle + "，" + shot.Composition,
		"【动作时间线】" + shot.Action,
		"【运镜】" + shot.CameraMotion,
		"【表演与情绪】" + shot.Emotion,
		"【光线与视觉风格】" + shot.Lighting + "；" + storyboard.StyleBible.VisualStyle + "；" + storyboard.StyleBible.Continuity,
		"【台词与声音】" + shot.Dialogue + "；" + shot.Audio,
		"【结尾与转场】" + shot.Transition,
		"【负面约束】" + strings.Join(uniqueNonEmptyStrings([]string{storyboard.StyleBible.Negative, shot.Negative, "闪烁、跳切、身份漂移、服装漂移、背景漂移、肢体错误、镜头抖动"}), "；"),
	}, "\n")
}

func uniqueNonEmptyStrings(values []string) []string {
	result, seen := []string{}, map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}

func StoryboardFromNode(node api.CanvasNode) (*Storyboard, error) {
	data, err := NodeData(node)
	if err != nil {
		return nil, err
	}
	if value, exists := data["pavo_storyboard"]; exists {
		raw, marshalErr := json.Marshal(value)
		if marshalErr != nil {
			return nil, marshalErr
		}
		return ParseStoryboard(raw)
	}
	if _, isRequestNode := data["pavo_storyboard_request"]; isRequestNode && numberValue(data["generation_status"]) != 2 {
		return nil, fmt.Errorf("节点 %s 的 storyboard 文本任务尚未成功；请先运行 storyboard generate，失败任务不能 finalize", node.NodeKey)
	}
	content, _ := data["content"].(string)
	if strings.TrimSpace(content) == "" {
		return nil, fmt.Errorf("节点 %s 没有 pavo_storyboard 或可解析 content", node.NodeKey)
	}
	return ParseStoryboard([]byte(content))
}

func SetStoryboardNodeData(data map[string]any, storyboard *Storyboard) error {
	if storyboard == nil {
		return errors.New("storyboard 不能为空")
	}
	storyboard.Normalize()
	if issues := storyboard.Validate(); len(issues) > 0 {
		return storyboardIssuesError(issues)
	}
	raw, err := json.Marshal(storyboard)
	if err != nil {
		return err
	}
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		return err
	}
	data["pavo_storyboard"] = value
	data["pavo_storyboard_schema"] = StoryboardSchemaVersion
	data["content"] = RenderStoryboardMarkdown(storyboard)
	data["mode"] = "authoring"
	data["isExecutable"] = false
	return nil
}

type StoryboardBuildOptions struct {
	ImageModel string
	VideoModel string
	WithVideo  bool
}

type StoryboardAsset struct {
	ShotID       string `json:"shot_id"`
	ImageNodeKey string `json:"image_node_key"`
	VideoNodeKey string `json:"video_node_key,omitempty"`
}

type StoryboardBuildResult struct {
	Request  *api.CanvasBatchRequest `json:"request"`
	Assets   []StoryboardAsset       `json:"assets"`
	GroupKey string                  `json:"group_key,omitempty"`
	Changed  bool                    `json:"changed"`
}

func storyboardAssetIdentity(data map[string]any) (string, string, string) {
	metadata, _ := data["pavo_storyboard_asset"].(map[string]any)
	storyboardNode, _ := metadata["storyboard_node"].(string)
	shotID, _ := metadata["shot_id"].(string)
	kind, _ := metadata["kind"].(string)
	return storyboardNode, shotID, kind
}

func findStoryboardAsset(detail *api.CanvasProjectDetail, storyboardNode, shotID, kind string) *api.CanvasNode {
	for index := range detail.NodeList {
		data, err := NodeData(detail.NodeList[index])
		if err != nil {
			continue
		}
		owner, shot, assetKind := storyboardAssetIdentity(data)
		if owner == storyboardNode && shot == shotID && assetKind == kind {
			return &detail.NodeList[index]
		}
	}
	return nil
}

func findStoryboardGroup(detail *api.CanvasProjectDetail, storyboardNode string) *api.CanvasNode {
	for index := range detail.NodeList {
		if string(detail.NodeList[index].NodeType) != "group" {
			continue
		}
		data, err := NodeData(detail.NodeList[index])
		if err != nil {
			continue
		}
		metadata, _ := data["pavo_storyboard_group"].(map[string]any)
		if owner, _ := metadata["storyboard_node"].(string); owner == storyboardNode {
			return &detail.NodeList[index]
		}
	}
	return nil
}

func connectionExists(detail *api.CanvasProjectDetail, source, target string) bool {
	for _, connection := range detail.ConnectionList {
		if connection.SourceNodeKey == source && connection.TargetNodeKey == target {
			return true
		}
	}
	return false
}

func upsertStoryboardAsset(detail *api.CanvasProjectDetail, storyboardNode api.CanvasNode, shot StoryboardShot, kind, prompt, model string, x, y float64, configure NDJSONModelConfigurator) (string, bool, error) {
	data := map[string]any{}
	existing := findStoryboardAsset(detail, storyboardNode.NodeKey, shot.ID, kind)
	if existing != nil {
		var err error
		data, err = NodeData(*existing)
		if err != nil {
			return "", false, err
		}
	}
	data["pavo_storyboard_asset"] = map[string]any{"storyboard_node": storyboardNode.NodeKey, "shot_id": shot.ID, "order": shot.Order, "kind": kind, "schema_version": StoryboardSchemaVersion}
	data["pavo_storyboard_shot"] = shot
	ReplaceTextPrompt(data, prompt)
	if strings.TrimSpace(model) == "" {
		return "", false, fmt.Errorf("%s model 不能为空", kind)
	}
	if configure != nil {
		if err := configure(kind, model, data); err != nil {
			return "", false, err
		}
	} else {
		SetModel(data, model)
	}
	name := fmt.Sprintf("%02d_%s_%s", shot.Order, shot.ID, map[string]string{"image": "关键帧", "video": "视频"}[kind])
	if existing == nil {
		item, err := NewNode(detail, NewNodeOptions{Type: kind, Name: name, X: &x, Y: &y, Width: 320, Height: 300, Data: data})
		if err != nil {
			return "", false, err
		}
		detail.NodeList = append(detail.NodeList, NodeFromWriteItem(*item))
		return item.NodeKey, true, nil
	}
	existing.Name = name
	data["title"], data["name"] = name, name
	encoded, err := EncodeObject(data)
	if err != nil {
		return "", false, err
	}
	existing.Data = json.RawMessage(encoded)
	return existing.NodeKey, false, nil
}

func BuildStoryboardGraph(detail *api.CanvasProjectDetail, storyboardNode api.CanvasNode, storyboard *Storyboard, options StoryboardBuildOptions, configure NDJSONModelConfigurator) (*StoryboardBuildResult, error) {
	if detail == nil {
		return nil, errors.New("画布详情为空")
	}
	if issues := storyboard.Validate(); len(issues) > 0 {
		return nil, storyboardIssuesError(issues)
	}
	working, err := CloneDetail(detail)
	if err != nil {
		return nil, err
	}
	storyNode, err := FindNode(working, storyboardNode.NodeKey)
	if err != nil {
		return nil, err
	}
	baseX, baseY, err := AbsolutePosition(working, storyNode.NodeKey)
	if err != nil {
		return nil, err
	}
	oldGroup := findStoryboardGroup(working, storyNode.NodeKey)
	newKeys, allKeys := []string{}, []string{}
	result := &StoryboardBuildResult{Assets: []StoryboardAsset{}}
	for index, shot := range storyboard.Shots {
		y := baseY + float64(index)*370
		imagePrompt := CompileStoryboardImagePrompt(storyboard, shot)
		imageKey, imageCreated, err := upsertStoryboardAsset(working, *storyNode, shot, "image", imagePrompt, options.ImageModel, baseX+380, y, configure)
		if err != nil {
			return nil, err
		}
		if imageCreated {
			newKeys = append(newKeys, imageKey)
		}
		allKeys = append(allKeys, imageKey)
		asset := StoryboardAsset{ShotID: shot.ID, ImageNodeKey: imageKey}
		_, _, references := shotContext(storyboard, shot)
		for order, reference := range references {
			source, findErr := FindNode(working, reference)
			if findErr != nil {
				return nil, fmt.Errorf("shot %s reference_node_keys: %w", shot.ID, findErr)
			}
			if !connectionExists(working, source.NodeKey, imageKey) {
				if _, err := AddConnection(working, source.NodeKey, imageKey, EdgeOptions{Role: "reference", MediaOrder: order}); err != nil {
					return nil, err
				}
			}
		}
		if options.WithVideo {
			videoPrompt := CompileStoryboardVideoPrompt(storyboard, shot)
			videoKey, videoCreated, err := upsertStoryboardAsset(working, *storyNode, shot, "video", videoPrompt, options.VideoModel, baseX+750, y, configure)
			if err != nil {
				return nil, err
			}
			if videoCreated {
				newKeys = append(newKeys, videoKey)
			}
			allKeys = append(allKeys, videoKey)
			asset.VideoNodeKey = videoKey
			if !connectionExists(working, imageKey, videoKey) {
				if _, err := AddConnection(working, imageKey, videoKey, EdgeOptions{Role: "reference", MediaOrder: 0}); err != nil {
					return nil, err
				}
			}
		}
		result.Assets = append(result.Assets, asset)
	}
	if oldGroup == nil && len(allKeys) >= 2 {
		groupKey, _, err := GroupNodes(working, allKeys, GroupOptions{Name: storyboard.Title + " · Storyboard", ModeCode: "storyboard"})
		if err != nil {
			return nil, err
		}
		result.GroupKey = groupKey
	} else if oldGroup != nil {
		result.GroupKey = oldGroup.NodeKey
		if len(newKeys) > 0 {
			references := append([]string{oldGroup.NodeKey}, newKeys...)
			groupKey, _, err := GroupNodes(working, references, GroupOptions{NodeKey: oldGroup.NodeKey, Name: storyboard.Title + " · Storyboard", ModeCode: "storyboard"})
			if err != nil {
				return nil, err
			}
			result.GroupKey = groupKey
		}
	}
	if result.GroupKey != "" {
		group, err := FindNode(working, result.GroupKey)
		if err != nil {
			return nil, err
		}
		data, err := NodeData(*group)
		if err != nil {
			return nil, err
		}
		data["pavo_storyboard_group"] = map[string]any{"storyboard_node": storyNode.NodeKey, "schema_version": StoryboardSchemaVersion}
		encoded, _ := EncodeObject(data)
		group.Data = json.RawMessage(encoded)
	}
	request, err := DiffDetails(detail, working)
	if err != nil {
		return nil, err
	}
	result.Request = request
	result.Changed = !BatchRequestEmpty(request)
	return result, nil
}
