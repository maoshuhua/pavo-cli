package canvas

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type canvasModelOption struct {
	ModelCode   string `json:"model_code"`
	Allowed     *bool  `json:"allowed"`
	IsOnline    *bool  `json:"is_online"`
	Constraints struct {
		AspectRatios             []string  `json:"aspect_ratios"`
		Resolutions              []string  `json:"resolutions"`
		ModeTypes                []string  `json:"mode_types"`
		SupportedDurationSeconds []float64 `json:"supported_duration_seconds"`
		MinDurationSeconds       float64   `json:"min_duration_seconds"`
		MaxBatchImages           int       `json:"max_batch_images"`
		SupportsAudioGeneration  bool      `json:"supports_audio_generation"`
	} `json:"constraints"`
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
	var root struct {
		Items []canvasModelOption `json:"items"`
	}
	if err := json.Unmarshal(raw, &root); err != nil {
		return fmt.Errorf("解析画布模型配置失败: %w", err)
	}
	var selected *canvasModelOption
	for index := range root.Items {
		if strings.TrimSpace(root.Items[index].ModelCode) == modelCode {
			selected = &root.Items[index]
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
