package canvas

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/maoshuhua/pavo-cli/internal/api"
)

func FindProgress(data *api.CanvasGenerationProgressData, taskID string) *api.CanvasGenerationProgress {
	if data == nil {
		return nil
	}
	for index := range data.Progresses {
		if strings.TrimSpace(string(data.Progresses[index].TaskID)) == strings.TrimSpace(taskID) {
			return &data.Progresses[index]
		}
	}
	return nil
}

// DecodeTaskResult accepts Pixa's JSON-string taskResult and the object form
// occasionally returned by test or compatibility deployments.
func DecodeTaskResult(raw json.RawMessage) json.RawMessage {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil
	}
	if trimmed[0] != '"' {
		return append(json.RawMessage(nil), trimmed...)
	}
	var text string
	if json.Unmarshal(trimmed, &text) != nil || strings.TrimSpace(text) == "" {
		return nil
	}
	if !json.Valid([]byte(text)) {
		return json.RawMessage(strconvQuote(text))
	}
	return json.RawMessage(text)
}

func strconvQuote(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func ApplyGenerationStart(data map[string]any, created api.CanvasGenerationCreated) {
	taskID := created.EffectiveTaskID()
	data["status"] = "generating"
	data["progress"] = 0
	data["task_id"] = taskID
	data["copyTaskId"] = taskID
	data["generation_status"] = 0
	delete(data, "generation_error_code")
	delete(data, "generation_completed_acknowledged")
	if created.EstimatedGenerationTime > 0 {
		data["estimated_generation_time"] = created.EstimatedGenerationTime
	}
}

func ApplyGenerationTerminal(data map[string]any, nodeType string, progress api.CanvasGenerationProgress) {
	delete(data, "status")
	data["progress"] = -1
	data["task_id"] = "-1"
	data["generation_status"] = progress.Status
	delete(data, "generation_completed_acknowledged")
	if progress.Failed() {
		if strings.TrimSpace(progress.ErrorCode) != "" {
			data["generation_error_code"] = progress.ErrorCode
		}
		return
	}
	delete(data, "generation_error_code")
	result := DecodeTaskResult(progress.TaskResult)
	if len(result) == 0 {
		return
	}
	var payload struct {
		Results []map[string]any `json:"results"`
	}
	decoder := json.NewDecoder(bytes.NewReader(result))
	decoder.UseNumber()
	if decoder.Decode(&payload) != nil {
		return
	}
	if nodeType == "text" {
		for _, item := range payload.Results {
			mimeType, _ := firstTaskField(item, "mime_type", "mimeType").(string)
			if mimeType != "text/markdown" {
				continue
			}
			if content, ok := firstTaskField(item, "text_content", "textContent").(string); ok {
				data["content"] = content
				data["mode"] = "authoring"
			}
			return
		}
		return
	}
	urls := make([]any, 0, len(payload.Results))
	for _, item := range payload.Results {
		url, _ := item["url"].(string)
		if strings.TrimSpace(url) == "" {
			continue
		}
		urls = append(urls, strings.TrimSpace(url))
		if _, exists := data["duration"]; !exists {
			if duration := firstTaskField(item, "duration"); duration != nil {
				data["duration"] = duration
			}
		}
	}
	if len(urls) > 0 {
		data["url"] = urls
	}
}

func firstTaskField(item map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, exists := item[key]; exists {
			return value
		}
	}
	return nil
}
