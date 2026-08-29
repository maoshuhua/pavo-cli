package canvas

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// PromptSegment is the extensible Pixa params.prompt item. Keeping the raw
// object shape lets newer frontend segment fields survive CLI updates.
type PromptSegment map[string]any

func promptSegments(params map[string]any) []PromptSegment {
	value, exists := params["prompt"]
	if !exists || value == nil {
		return []PromptSegment{}
	}
	if text, ok := value.(string); ok {
		text = strings.TrimSpace(text)
		if text == "" {
			return []PromptSegment{}
		}
		return []PromptSegment{{"type": "text", "content": text}}
	}
	items, ok := value.([]any)
	if !ok {
		if typed, typedOK := value.([]PromptSegment); typedOK {
			return append([]PromptSegment(nil), typed...)
		}
		return []PromptSegment{}
	}
	result := make([]PromptSegment, 0, len(items))
	for _, item := range items {
		switch typed := item.(type) {
		case map[string]any:
			result = append(result, PromptSegment(typed))
		case PromptSegment:
			result = append(result, typed)
		}
	}
	return result
}

func setPromptSegments(data map[string]any, segments []PromptSegment) {
	params := objectField(data, "params")
	items := make([]any, 0, len(segments))
	for _, segment := range segments {
		items = append(items, map[string]any(segment))
	}
	params["prompt"] = items
	data["params"] = params
}

// ReplaceTextPrompt replaces only text segments and preserves skill/media
// segments. This matches the composable prompt contract used by pavo-app-front.
func ReplaceTextPrompt(data map[string]any, prompt string) {
	prompt = strings.TrimSpace(prompt)
	params := objectField(data, "params")
	existing := promptSegments(params)
	result := make([]PromptSegment, 0, len(existing)+1)
	for _, segment := range existing {
		kind, _ := segment["type"].(string)
		if strings.EqualFold(strings.TrimSpace(kind), "text") {
			continue
		}
		result = append(result, segment)
	}
	if prompt != "" {
		result = append(result, PromptSegment{"type": "text", "content": prompt})
	}
	setPromptSegments(data, result)
}

// AddSkillPrompt prepends one skill segment and removes duplicate occurrences
// of the same skill code while preserving other prompt segments.
func AddSkillPrompt(data map[string]any, code string) error {
	code = strings.TrimSpace(code)
	if code == "" {
		return errors.New("skill code 不能为空")
	}
	params := objectField(data, "params")
	existing := promptSegments(params)
	result := []PromptSegment{{"type": "skill", "code": code}}
	for _, segment := range existing {
		kind, _ := segment["type"].(string)
		segmentCode, _ := segment["code"].(string)
		if strings.EqualFold(strings.TrimSpace(kind), "skill") && strings.TrimSpace(segmentCode) == code {
			continue
		}
		result = append(result, segment)
	}
	setPromptSegments(data, result)
	return nil
}

func ReplacePromptSegments(data map[string]any, raw string) error {
	if strings.TrimSpace(raw) == "" {
		setPromptSegments(data, []PromptSegment{})
		return nil
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	var items []map[string]any
	if err := decoder.Decode(&items); err != nil {
		return fmt.Errorf("prompt segments 必须是 JSON array: %w", err)
	}
	segments := make([]PromptSegment, 0, len(items))
	for index, item := range items {
		kind, _ := item["type"].(string)
		kind = strings.TrimSpace(kind)
		if kind == "" {
			return fmt.Errorf("prompt segment[%d] 缺少 type", index)
		}
		switch kind {
		case "text":
			content, _ := item["content"].(string)
			if strings.TrimSpace(content) == "" {
				return fmt.Errorf("prompt segment[%d] text 缺少 content", index)
			}
		case "skill":
			code, _ := item["code"].(string)
			if strings.TrimSpace(code) == "" {
				return fmt.Errorf("prompt segment[%d] skill 缺少 code", index)
			}
		}
		segments = append(segments, PromptSegment(item))
	}
	setPromptSegments(data, segments)
	return nil
}

func PromptSegments(data map[string]any) []PromptSegment {
	return promptSegments(objectField(data, "params"))
}
