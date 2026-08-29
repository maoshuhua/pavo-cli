package canvas

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type CanvasModelConstraints struct {
	AspectRatios             []string  `json:"aspect_ratios"`
	Resolutions              []string  `json:"resolutions"`
	ModeTypes                []string  `json:"mode_types"`
	SupportedDurationSeconds []float64 `json:"supported_duration_seconds"`
	MinDurationSeconds       float64   `json:"min_duration_seconds"`
	MaxBatchImages           int       `json:"max_batch_images"`
	SupportsAudioGeneration  bool      `json:"supports_audio_generation"`
}

type CanvasModelOption struct {
	ModelCode   string                 `json:"model_code"`
	ModelName   string                 `json:"model_name,omitempty"`
	Description string                 `json:"description,omitempty"`
	Allowed     *bool                  `json:"allowed"`
	IsOnline    *bool                  `json:"is_online"`
	Constraints CanvasModelConstraints `json:"constraints"`
}

type CanvasModelExplanation struct {
	Scene             string            `json:"scene"`
	NodeType          string            `json:"node_type"`
	Available         bool              `json:"available"`
	UnavailableReason string            `json:"unavailable_reason,omitempty"`
	Model             CanvasModelOption `json:"model"`
	Raw               json.RawMessage   `json:"raw"`
	EffectiveDefaults map[string]any    `json:"effective_defaults"`
	Guidance          []string          `json:"guidance"`
}

func ParseCanvasModelOptions(raw json.RawMessage) ([]CanvasModelOption, error) {
	var root struct {
		Items []CanvasModelOption `json:"items"`
	}
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, fmt.Errorf("解析画布模型配置失败: %w", err)
	}
	if root.Items == nil {
		root.Items = []CanvasModelOption{}
	}
	return root.Items, nil
}

func ExplainCanvasModel(raw json.RawMessage, scene, modelCode string) (*CanvasModelExplanation, error) {
	items, err := ParseCanvasModelOptions(raw)
	if err != nil {
		return nil, err
	}
	modelCode = strings.TrimSpace(modelCode)
	var selected *CanvasModelOption
	for index := range items {
		if strings.TrimSpace(items[index].ModelCode) == modelCode {
			selected = &items[index]
			break
		}
	}
	if selected == nil {
		return nil, fmt.Errorf("实时 %s 模型列表中找不到 %q", strings.TrimSpace(scene), modelCode)
	}
	var rawRoot struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(raw, &rawRoot); err != nil {
		return nil, fmt.Errorf("解析画布模型配置失败: %w", err)
	}
	var selectedRaw json.RawMessage
	for _, itemRaw := range rawRoot.Items {
		var identity struct {
			ModelCode string `json:"model_code"`
		}
		if json.Unmarshal(itemRaw, &identity) == nil && strings.TrimSpace(identity.ModelCode) == modelCode {
			selectedRaw = append(json.RawMessage(nil), itemRaw...)
			break
		}
	}
	nodeType := strings.TrimPrefix(strings.TrimSpace(scene), "canvas_")
	explanation := &CanvasModelExplanation{
		Scene:             strings.TrimSpace(scene),
		NodeType:          nodeType,
		Available:         true,
		Model:             *selected,
		Raw:               selectedRaw,
		EffectiveDefaults: map[string]any{},
		Guidance:          []string{},
	}
	if selected.Allowed != nil && !*selected.Allowed {
		explanation.Available = false
		explanation.UnavailableReason = "当前账号不可用"
	} else if selected.IsOnline != nil && !*selected.IsOnline {
		explanation.Available = false
		explanation.UnavailableReason = "模型当前未上线"
	}
	constraints := selected.Constraints
	if len(constraints.ModeTypes) > 0 {
		explanation.EffectiveDefaults["modeType"] = constraints.ModeTypes
		explanation.Guidance = append(explanation.Guidance, "只使用 constraints.mode_types 中列出的输入模式")
	}
	settings := map[string]any{}
	if len(constraints.AspectRatios) > 0 {
		settings["ratio"] = constraints.AspectRatios[0]
	}
	if len(constraints.Resolutions) > 0 {
		settings["resolution"] = constraints.Resolutions[0]
	}
	settings["generateAudio"] = false
	explanation.EffectiveDefaults["settings"] = settings
	explanation.EffectiveDefaults["model"] = selected.ModelCode
	explanation.EffectiveDefaults["power"] = 0
	switch nodeType {
	case "image":
		explanation.EffectiveDefaults["count"] = 1
		if constraints.MaxBatchImages > 0 {
			explanation.Guidance = append(explanation.Guidance, fmt.Sprintf("单次图片数量不得超过 %d", constraints.MaxBatchImages))
		}
	case "video":
		durations := constraints.SupportedDurationSeconds
		if len(durations) == 0 && constraints.MinDurationSeconds > 0 {
			durations = []float64{constraints.MinDurationSeconds}
		}
		if len(durations) > 0 {
			explanation.EffectiveDefaults["duration"] = durations[0]
			explanation.Guidance = append(explanation.Guidance, "视频时长必须使用 constraints.supported_duration_seconds 中的值")
		}
		if constraints.SupportsAudioGeneration {
			explanation.Guidance = append(explanation.Guidance, "模型支持生成音频；默认关闭，只有用户需要声音时再启用")
		}
	}
	return explanation, nil
}

func FirstAvailableModelCode(raw json.RawMessage) (string, error) {
	items, err := ParseCanvasModelOptions(raw)
	if err != nil {
		return "", err
	}
	for _, item := range items {
		if strings.TrimSpace(item.ModelCode) == "" {
			continue
		}
		if item.Allowed != nil && !*item.Allowed {
			continue
		}
		if item.IsOnline != nil && !*item.IsOnline {
			continue
		}
		return strings.TrimSpace(item.ModelCode), nil
	}
	return "", errors.New("实时模型列表中没有当前账号可用的在线模型")
}

func ModelScene(nodeType string) string {
	switch strings.TrimSpace(nodeType) {
	case "image":
		return "canvas_image"
	case "video":
		return "canvas_video"
	case "audio":
		return "canvas_audio"
	default:
		return ""
	}
}

// ApplyModelConfiguration validates a live model option and fills the params
// fields that pavo-app-front normally writes when a model is selected. User
// values are preserved when still supported by the selected model.
func ApplyModelConfiguration(data map[string]any, nodeType, modelCode string, raw json.RawMessage) error {
	modelCode = strings.TrimSpace(modelCode)
	if modelCode == "" {
		return errors.New("model code 不能为空")
	}
	items, err := ParseCanvasModelOptions(raw)
	if err != nil {
		return err
	}
	var selected *CanvasModelOption
	for index := range items {
		if strings.TrimSpace(items[index].ModelCode) == modelCode {
			selected = &items[index]
			break
		}
	}
	if selected == nil {
		return fmt.Errorf("实时 %s 模型列表中找不到 %q", ModelScene(nodeType), modelCode)
	}
	if selected.Allowed != nil && !*selected.Allowed {
		return fmt.Errorf("模型 %q 当前账号不可用", modelCode)
	}
	if selected.IsOnline != nil && !*selected.IsOnline {
		return fmt.Errorf("模型 %q 当前未上线", modelCode)
	}

	params := objectField(data, "params")
	params["model"] = modelCode
	if len(selected.Constraints.ModeTypes) > 0 {
		params["modeType"] = stringValues(selected.Constraints.ModeTypes)
	}
	if _, exists := params["power"]; !exists {
		params["power"] = 0
	}
	settings := objectField(params, "settings")
	ensureSupportedString(settings, "ratio", selected.Constraints.AspectRatios)
	ensureSupportedString(settings, "resolution", selected.Constraints.Resolutions)
	if _, exists := settings["generateAudio"]; !exists || !selected.Constraints.SupportsAudioGeneration {
		settings["generateAudio"] = false
	}
	params["settings"] = settings

	switch strings.TrimSpace(nodeType) {
	case "image":
		count := numberValue(params["count"])
		maxCount := selected.Constraints.MaxBatchImages
		if maxCount <= 0 {
			maxCount = 1
		}
		if count < 1 || count > float64(maxCount) {
			params["count"] = 1
		}
	case "video":
		durations := selected.Constraints.SupportedDurationSeconds
		if len(durations) == 0 && selected.Constraints.MinDurationSeconds > 0 {
			durations = []float64{selected.Constraints.MinDurationSeconds}
		}
		if len(durations) > 0 && !containsNumber(durations, numberValue(params["duration"])) {
			params["duration"] = durations[0]
		}
	}
	data["params"] = params
	return nil
}

func stringValues(values []string) []any {
	result := make([]any, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			result = append(result, value)
		}
	}
	return result
}

func ensureSupportedString(target map[string]any, key string, supported []string) {
	if len(supported) == 0 {
		return
	}
	current, _ := target[key].(string)
	for _, value := range supported {
		if strings.EqualFold(strings.TrimSpace(current), strings.TrimSpace(value)) {
			target[key] = value
			return
		}
	}
	target[key] = supported[0]
}

func numberValue(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case json.Number:
		number, _ := typed.Float64()
		return number
	case string:
		number := json.Number(strings.TrimSpace(typed))
		result, _ := number.Float64()
		return result
	default:
		return 0
	}
}

func containsNumber(values []float64, target float64) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
