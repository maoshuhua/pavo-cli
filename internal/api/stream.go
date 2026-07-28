package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const streamMode = "design"

type EventHandler func(*StreamEvent) error

func (c *Client) Stream(ctx context.Context, conversationID, prompt string, handler EventHandler) (*StreamOutput, error) {
	return c.StreamWithFiles(ctx, conversationID, prompt, nil, handler)
}

// StreamWithFiles starts a design generation with optional uploaded chat attachments.
func (c *Client) StreamWithFiles(ctx context.Context, conversationID, prompt string, files []ChatAttachment, handler EventHandler) (*StreamOutput, error) {
	conversationID = strings.TrimSpace(conversationID)
	prompt = strings.TrimSpace(prompt)
	if conversationID == "" {
		return nil, errors.New("conversation_id 不能为空")
	}
	if prompt == "" {
		return nil, errors.New("prompt 不能为空")
	}
	files, err := normalizeChatAttachments(files)
	if err != nil {
		return nil, err
	}
	requestURL, err := c.resolveURL(c.paths.Stream)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(StreamRequest{
		ConversationID: ConversationID(conversationID),
		Prompt:         prompt,
		Mode:           streamMode,
		Files:          files,
	})
	if err != nil {
		return nil, fmt.Errorf("编码 stream 请求失败: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("构造 stream 请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream, application/x-ndjson, application/json")
	setPAVOHeaders(req)
	if err := c.authorize(req); err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("POST %s 请求失败: %w", requestURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, responseError(resp)
	}

	var terminal *StreamEvent
	var eventID string
	var messageID string
	var modelCode string
	var taskID string
	var traceID string
	var results []GenerationResult
	var artifacts []json.RawMessage
	consume := func(raw []byte) error {
		event, err := decodeEvent(raw)
		if err != nil {
			return err
		}
		if event == nil {
			return nil
		}
		if event.EventID != "" {
			eventID = event.EventID
		} else if event.Data.EventID != "" {
			eventID = event.Data.EventID
		}
		if event.MessageID != "" {
			messageID = event.MessageID
		} else if event.Data.MessageID != "" {
			messageID = event.Data.MessageID
		}
		if event.ModelCode != "" {
			modelCode = event.ModelCode
		} else if event.Data.ModelCode != "" {
			modelCode = event.Data.ModelCode
		}
		if event.TaskID != "" {
			taskID = event.TaskID
		} else if event.Data.TaskID != "" {
			taskID = event.Data.TaskID
		}
		if event.TraceID != "" {
			traceID = event.TraceID
		} else if event.Data.TraceID != "" {
			traceID = event.Data.TraceID
		}
		if len(event.Data.Results) > 0 {
			results = append(results[:0], event.Data.Results...)
			if event.Type == "GenerationArtifact" {
				for index := range results {
					results[index].Success = true
				}
			}
		}
		if event.Type == "GenerationArtifact" && len(event.Raw) > 0 {
			artifacts = append(artifacts, append(json.RawMessage(nil), event.Raw...))
		}
		if handler != nil {
			if err := handler(event); err != nil {
				return err
			}
		}
		switch event.Type {
		case "GenerationSuccess", "AgentEnd":
			terminal = event
			return io.EOF
		case "GenerationFailed", "GenerationFailure", "TaskFailed", "Error":
			message := strings.TrimSpace(event.Data.Message)
			if message == "" {
				message = event.Type
			}
			return fmt.Errorf("PAVO 生成失败: %s", message)
		default:
			return nil
		}
	}

	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	reader := bufio.NewReader(resp.Body)
	if strings.Contains(contentType, "text/event-stream") || looksLikeSSE(reader) {
		err = consumeSSE(reader, consume)
	} else {
		err = consumeJSON(reader, consume)
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	if terminal == nil {
		return nil, errors.New("stream 已结束，但没有收到 GenerationSuccess 或 AgentEnd")
	}
	return &StreamOutput{
		ConversationID: conversationID,
		TerminalType:   terminal.Type,
		EventID:        eventID,
		MessageID:      messageID,
		ModelCode:      modelCode,
		TaskID:         taskID,
		TraceID:        traceID,
		Results:        results,
		Artifacts:      artifacts,
	}, nil
}

func normalizeChatAttachments(files []ChatAttachment) ([]ChatAttachment, error) {
	if len(files) == 0 {
		return nil, nil
	}
	normalized := make([]ChatAttachment, len(files))
	for index, file := range files {
		file.MimeType = strings.TrimSpace(file.MimeType)
		file.URL = strings.TrimSpace(file.URL)
		file.Filename = strings.TrimSpace(file.Filename)
		if file.MimeType == "" || file.URL == "" || file.Filename == "" {
			return nil, fmt.Errorf("附件 files[%d] 缺少 mime_type、url 或 filename", index)
		}
		if err := validateHTTPURL(file.URL, fmt.Sprintf("附件 files[%d].url", index)); err != nil {
			return nil, err
		}
		normalized[index] = file
	}
	return normalized, nil
}

func looksLikeSSE(reader *bufio.Reader) bool {
	prefix, err := reader.Peek(5)
	return err == nil && string(prefix) == "data:"
}

func consumeJSON(reader io.Reader, consume func([]byte) error) error {
	decoder := json.NewDecoder(reader)
	for {
		var raw json.RawMessage
		err := decoder.Decode(&raw)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("解析 stream JSON 失败: %w", err)
		}
		if err := consume(raw); err != nil {
			return err
		}
	}
}

func consumeSSE(reader *bufio.Reader, consume func([]byte) error) error {
	var data bytes.Buffer
	flush := func() error {
		payload := bytes.TrimSpace(data.Bytes())
		data.Reset()
		if len(payload) == 0 || bytes.Equal(payload, []byte("[DONE]")) {
			return nil
		}
		return consume(payload)
	}
	for {
		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return fmt.Errorf("读取 SSE stream 失败: %w", err)
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			if flushErr := flush(); flushErr != nil {
				return flushErr
			}
		} else if strings.HasPrefix(line, "data:") {
			if data.Len() > 0 {
				data.WriteByte('\n')
			}
			data.WriteString(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
		if errors.Is(err, io.EOF) {
			return flush()
		}
	}
}

func decodeEvent(raw []byte) (*StreamEvent, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return nil, nil
	}
	var envelope struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("解析 stream 事件失败: %w", err)
	}
	if envelope.Code != "" {
		if err := validateEnvelope(envelope.Code, envelope.Message); err != nil {
			return nil, err
		}
		return nil, nil
	}
	var event StreamEvent
	if err := json.Unmarshal(raw, &event); err != nil {
		return nil, fmt.Errorf("解析 stream 事件失败: %w", err)
	}
	event.Raw = append(event.Raw[:0], raw...)
	return &event, nil
}
